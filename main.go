package main

import (
	"encoding/json"
	"log"

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
	Clients    map[string]*Client
	Register   chan *Client
	Unregister chan *Client
}

// SignalingMessage represents WebRTC signaling data
type SignalingMessage struct {
	Type string      `json:"type"` // "offer", "answer", "candidate", "call-request", "call-accept", "call-reject", "hangup"
	To   string      `json:"to"`
	From string      `json:"from"`
	Data interface{} `json:"data"`
}

// NewHub creates a new Hub instance
func NewHub() *Hub {
	return &Hub{
		Clients:    make(map[string]*Client),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
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

			// Forward message to target client
			if msg.To != "" {
				msgBytes, err := json.Marshal(msg)
				if err != nil {
					log.Printf("JSON marshal error: %v", err)
					continue
				}

				if err := hub.SendToClient(msg.To, msgBytes); err != nil {
					log.Printf("Error sending to client %s: %v", msg.To, err)
				}
			}
		}
	}))

	// ICE servers endpoint
	app.Get("/ice-servers", func(c *fiber.Ctx) error {
		iceServers := fiber.Map{
			"iceServers": []fiber.Map{
				// STUN servers for NAT discovery (multiple for redundancy)
				{"urls": "stun:stun.l.google.com:19302"},
				{"urls": "stun:stun1.l.google.com:19302"},

				// TURN server (now running via Docker)
				{
					"urls":       "turn:localhost:3478",
					"username":   "testuser",
					"credential": "testpass",
				},

				// Uncomment and configure these when you have actual TURN servers:
				// {
				// 	"urls":       "turn:your-turn-server.com:3478",
				// 	"username":   "your-username",
				// 	"credential": "your-password",
				// },
				// For TURNS (secure TURN):
				// {
				// 	"urls":       "turns:your-turn-server.com:5349",
				// 	"username":   "your-username",
				// 	"credential": "your-password",
				// },
			},
		}
		return c.JSON(iceServers)
	})

	// Serve static files
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendFile("./callerA.html")
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
	log.Println("🌐 Web interface: http://0.0.0.0:8080")
	log.Println("📱 For mobile testing: Use VS Code port forwarding or your local IP")
	log.Println("🔧 Server listening on all interfaces (0.0.0.0:8080)")

	log.Fatal(app.Listen(":8080"))
}
