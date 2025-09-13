package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/websocket/v2"
)

// Client represents a connected WebSocket client
type Client struct {
	ID   string
	Conn *websocket.Conn
	Hub  *Hub
}

// Hub maintains active clients and handles message broadcasting
type Hub struct {
	Clients           map[string]*Client
	Register          chan *Client
	Unregister        chan *Client
	ICERequestLimiter map[string]time.Time
	ICELimiterMutex   sync.RWMutex
}

// SignalingMessage represents WebRTC signaling data
type SignalingMessage struct {
	Type string      `json:"type"` // "offer", "answer", "candidate", "call-request", "call-accept", "call-reject", "hangup", "ice-servers-request", "ice-servers-response"
	To   string      `json:"to"`
	From string      `json:"from"`
	Data interface{} `json:"data"`
}

// NewHub creates a new Hub instance
func NewHub() *Hub {
	return &Hub{
		Clients:           make(map[string]*Client),
		Register:          make(chan *Client),
		Unregister:        make(chan *Client),
		ICERequestLimiter: make(map[string]time.Time),
		ICELimiterMutex:   sync.RWMutex{},
	}
}

// Run starts the hub and handles client registration/unregistration
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.Clients[client.ID] = client
			log.Printf("Client %s connected. Total clients: %d", client.ID, len(h.Clients))

		case client := <-h.Unregister:
			if _, ok := h.Clients[client.ID]; ok {
				delete(h.Clients, client.ID)
				client.Conn.Close()
				log.Printf("Client %s disconnected. Total clients: %d", client.ID, len(h.Clients))
			}
		}
	}
}

// SendToClient sends a message to a specific client
func (h *Hub) SendToClient(clientID string, message []byte) error {
	if client, exists := h.Clients[clientID]; exists {
		return client.Conn.WriteMessage(websocket.TextMessage, message)
	}
	log.Printf("Client %s not found", clientID)
	return nil
}

// CheckICERequestRateLimit checks if client can request ICE servers
func (h *Hub) CheckICERequestRateLimit(clientID string) bool {
	h.ICELimiterMutex.Lock()
	defer h.ICELimiterMutex.Unlock()

	lastRequest, exists := h.ICERequestLimiter[clientID]
	if exists && time.Since(lastRequest) < 30*time.Second {
		return false // Rate limited
	}

	h.ICERequestLimiter[clientID] = time.Now()
	return true
}

func main() {
	// Create Fiber app
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			return c.Status(500).SendString(err.Error())
		},
	})

	// CORS middleware
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,HEAD,PUT,DELETE,PATCH,OPTIONS",
		AllowHeaders: "Origin, Content-Type, Accept",
	}))

	// Create and start hub
	hub := NewHub()
	go hub.Run()

	// WebSocket upgrade middleware
	app.Use("/ws", func(c *fiber.Ctx) error {
		// Check if it's a WebSocket upgrade request
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	// WebSocket handler
	app.Get("/ws", websocket.New(func(c *websocket.Conn) {
		// Get client ID from query parameter
		clientID := c.Query("id")
		if clientID == "" {
			log.Println("Client ID required")
			c.Close()
			return
		}

		// Create client
		client := &Client{
			ID:   clientID,
			Conn: c,
			Hub:  hub,
		}

		// Register client
		hub.Register <- client

		// Handle client disconnect
		defer func() {
			hub.Unregister <- client
			c.Close()
		}()

		// Handle incoming messages
		for {
			var msg SignalingMessage
			err := c.ReadJSON(&msg)
			if err != nil {
				log.Printf("ReadJSON error for client %s: %v", clientID, err)
				break
			}

			// Set the sender
			msg.From = clientID

			log.Printf("Received %s message from %s to %s", msg.Type, msg.From, msg.To)
		}
	}))

	// TURN credentials endpoint - proxy to actual TURN credentials API with role support
	app.Get("/turn-credentials/:role", func(c *fiber.Ctx) error {
		role := c.Params("role")
		if role != "customer" && role != "driver" {
			return c.Status(400).JSON(fiber.Map{
				"success": false,
				"error":   "Invalid role. Must be 'customer' or 'driver'",
				"data":    nil,
			})
		}

		// Set CORS headers
		c.Set("Access-Control-Allow-Origin", "*")
		c.Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		c.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Device-Id")

		// Get Authorization token from header
		authToken := c.Get("Authorization")
		if authToken == "" {
			return c.Status(401).JSON(fiber.Map{
				"success": false,
				"error":   "Missing Authorization header",
				"data":    nil,
			})
		}

		log.Printf("Making request to TURN credentials API for role: %s", role)

		// Create HTTP client
		client := &http.Client{
			Timeout: 10 * time.Second,
		}

		// Construct the backend URL based on role
		backendURL := fmt.Sprintf("https://api-stag.superapi.my.id/communication/v1/%s/turn-credentials", role)

		// Create request
		req, err := http.NewRequest("GET", backendURL, nil)
		if err != nil {
			log.Printf("Error creating request: %v", err)
			return c.Status(500).JSON(fiber.Map{
				"success": false,
				"error":   "Failed to create request",
				"data":    nil,
			})
		}

		// Add headers that the backend expects
		req.Header.Set("Accept", "*/*")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", authToken)
		req.Header.Set("X-Device-Id", c.Get("X-Device-Id"))

		log.Printf("Request URL: %s", backendURL)
		log.Printf("Request headers: %+v", req.Header)

		// Make the request
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Error fetching TURN credentials: %v", err)
			return c.Status(500).JSON(fiber.Map{
				"success": false,
				"error":   "Failed to fetch TURN credentials",
				"data":    nil,
			})
		}
		defer resp.Body.Close()

		// Read response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("Error reading TURN credentials response: %v", err)
			return c.Status(500).JSON(fiber.Map{
				"success": false,
				"error":   "Failed to read TURN credentials response",
				"data":    nil,
			})
		}

		log.Printf("✅ TURN credentials response: %d bytes, status: %d", len(body), resp.StatusCode)
		log.Printf("Response body: %s", string(body))

		// Forward the response with the same status code
		c.Status(resp.StatusCode)

		// Set content type
		contentType := resp.Header.Get("Content-Type")
		if contentType != "" {
			c.Set("Content-Type", contentType)
		} else {
			c.Set("Content-Type", "application/json")
		}

		return c.Send(body)
	})

	// Serve static files
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendFile("./client.html")
	})

	app.Get("/client.html", func(c *fiber.Ctx) error {
		return c.SendFile("./client.html")
	})

	app.Get("/callerA.html", func(c *fiber.Ctx) error {
		return c.SendFile("./callerA.html")
	})

	app.Get("/callerB.html", func(c *fiber.Ctx) error {
		return c.SendFile("./callerB.html")
	})

	app.Get("/diagnostics.html", func(c *fiber.Ctx) error {
		return c.SendFile("./diagnostics.html")
	})

	log.Println("🚀 GoFiber WebRTC Signaling Server started at :8080")
	log.Println("📡 WebSocket endpoint: ws://0.0.0.0:8080/ws?id=<client_id>")
	log.Println("🔧 TURN credentials endpoints:")
	log.Println("   Customer: http://0.0.0.0:8080/turn-credentials/customer")
	log.Println("   Driver: http://0.0.0.0:8080/turn-credentials/driver")
	log.Println("🌐 Dynamic client interface: http://0.0.0.0:8080/client.html")
	log.Println("🌐 Legacy CallerA: http://0.0.0.0:8080/callerA.html")
	log.Println("🌐 Legacy CallerB: http://0.0.0.0:8080/callerB.html")
	log.Println("📱 For mobile testing: Use VS Code port forwarding or your local IP")
	log.Println("🔧 Server listening on all interfaces (0.0.0.0:8080)")

	log.Fatal(app.Listen(":8080"))
}
