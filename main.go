package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
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
	Broadcast  chan []byte
}

// SignalingMessage represents WebRTC signaling data
type SignalingMessage struct {
	Type string      `json:"type"` // "offer", "answer", "candidate", "call-request", "call-accept", "call-reject", "hangup"
	To   string      `json:"to"`
	From string      `json:"from"`
	Data interface{} `json:"data"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

// NewHub creates a new Hub instance
func NewHub() *Hub {
	return &Hub{
		Clients:    make(map[string]*Client),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Broadcast:  make(chan []byte),
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
	return fmt.Errorf("client %s not found", clientID)
}

// CORS middleware
func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

// WebSocket handler for signaling
func handleWebSocket(hub *Hub, w http.ResponseWriter, r *http.Request) {
	enableCORS(w)

	clientID := r.URL.Query().Get("id")
	if clientID == "" {
		http.Error(w, "Client ID required", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &Client{
		ID:   clientID,
		Conn: conn,
		Hub:  hub,
	}

	hub.Register <- client

	// Handle incoming messages
	go func() {
		defer func() {
			hub.Unregister <- client
			conn.Close()
		}()

		for {
			var msg SignalingMessage
			err := conn.ReadJSON(&msg)
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
	}()
}

// Get ICE servers configuration
func handleICEServers(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)

	if r.Method == http.MethodOptions {
		return
	}

	iceServers := map[string]interface{}{
		"iceServers": []map[string]interface{}{
			{"urls": "stun:stun.l.google.com:19302"},
			// Add TURN servers here if needed for production
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(iceServers)
}

func main() {
	hub := NewHub()
	go hub.Run()

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		handleWebSocket(hub, w, r)
	})

	http.HandleFunc("/ice-servers", handleICEServers)

	// Serve static files
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFile(w, r, "callerA.html")
			return
		}
		http.ServeFile(w, r, r.URL.Path[1:])
	})

	fmt.Println("P2P WebRTC Signaling Server started at :8080")
	fmt.Println("WebSocket endpoint: ws://localhost:8080/ws?id=<client_id>")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
