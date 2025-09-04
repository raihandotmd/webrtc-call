package main

import (
	"log"
	"net/http"
	"net/url"

	fastws "github.com/fasthttp/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/websocket/v2"
)

const (
	BackendWSURL = "ws://localhost:8082/communication/v1/ws"
	ProxyPort    = ":8081"
	DebugMode    = true // Set to false to reduce message logging
)

type ProxyServer struct {
	app *fiber.App
}

func NewProxyServer() *ProxyServer {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: false,
	})

	// Enable CORS
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization, X-User-Id",
		AllowMethods: "GET, POST, PUT, DELETE, OPTIONS",
	}))

	return &ProxyServer{
		app: app,
	}
}

func (ps *ProxyServer) setupRoutes() {
	// Health check endpoint
	ps.app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "healthy",
			"service": "ws-proxy",
		})
	})

	// WebSocket upgrade check middleware
	ps.app.Use("/ws", func(c *fiber.Ctx) error {
		// Check for websocket upgrade
		if websocket.IsWebSocketUpgrade(c) {
			// Extract and store user ID from query params or headers
			userID := c.Query("userId")
			if userID == "" {
				userID = c.Get("X-User-Id")
			}
			if userID == "" {
				return c.Status(400).SendString("Missing userId parameter or X-User-Id header")
			}

			c.Locals("userID", userID)
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	// WebSocket proxy endpoint
	ps.app.Get("/ws", websocket.New(ps.handleWebSocketProxy))
}

func (ps *ProxyServer) handleWebSocketProxy(clientConn *websocket.Conn) {
	defer clientConn.Close()

	// Get user ID from locals (set in middleware)
	userID, ok := clientConn.Locals("userID").(string)
	if !ok || userID == "" {
		log.Printf("Missing userID in connection locals")
		clientConn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseUnsupportedData, "Missing user ID"))
		return
	}

	log.Printf("👤 Proxying WebSocket connection for user: %s", userID)
	log.Printf("🎯 Target backend: %s", BackendWSURL)

	// Parse backend URL
	backendURL, err := url.Parse(BackendWSURL)
	if err != nil {
		log.Printf("Failed to parse backend URL: %v", err)
		return
	}

	// Create headers for backend connection
	headers := http.Header{}
	headers.Set("X-User-Id", userID)
	headers.Set("Origin", "http://localhost:8081") // Set origin for proxy

	log.Printf("📋 Sending headers to backend: X-User-Id=%s, Origin=%s", userID, "http://localhost:8081")

	// Connect to backend WebSocket server using fasthttp websocket dialer
	dialer := &fastws.Dialer{}
	backendConn, _, err := dialer.Dial(backendURL.String(), headers)
	if err != nil {
		log.Printf("Failed to connect to backend: %v", err)
		clientConn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "Backend connection failed"))
		return
	}
	defer backendConn.Close()

	log.Printf("✅ Successfully connected to backend for user: %s", userID)
	log.Printf("🔄 Starting bidirectional message forwarding...")

	// Create channels for coordinating goroutines
	done := make(chan struct{})

	// Forward messages from client to backend
	go func() {
		defer func() {
			select {
			case <-done:
			default:
				close(done)
			}
		}()
		for {
			messageType, message, err := clientConn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("[USER: %s] Client read error: %v", userID, err)
				}
				return
			}

			// Log incoming message from client
			if DebugMode {
				log.Printf("[USER: %s] ➡️ Client → Backend: %s", userID, string(message))
			}

			err = backendConn.WriteMessage(messageType, message)
			if err != nil {
				log.Printf("[USER: %s] Backend write error: %v", userID, err)
				return
			}
		}
	}()

	// Forward messages from backend to client
	go func() {
		defer func() {
			select {
			case <-done:
			default:
				close(done)
			}
		}()
		for {
			messageType, message, err := backendConn.ReadMessage()
			if err != nil {
				if fastws.IsUnexpectedCloseError(err, fastws.CloseGoingAway, fastws.CloseAbnormalClosure) {
					log.Printf("[USER: %s] Backend read error: %v", userID, err)
				}
				return
			}

			// Log incoming message from backend
			if DebugMode {
				log.Printf("[USER: %s] ⬅️ Backend → Client: %s", userID, string(message))
			}

			err = clientConn.WriteMessage(messageType, message)
			if err != nil {
				log.Printf("[USER: %s] Client write error: %v", userID, err)
				return
			}
		}
	}()

	// Wait for either goroutine to finish
	<-done
	log.Printf("🔌 WebSocket proxy connection closed for user: %s", userID)
}

func (ps *ProxyServer) Start() error {
	ps.setupRoutes()

	log.Printf("Starting WebSocket proxy server on port %s", ProxyPort)
	log.Printf("Proxying to backend: %s", BackendWSURL)

	return ps.app.Listen(ProxyPort)
}

func main() {
	server := NewProxyServer()

	log.Fatal(server.Start())
}
