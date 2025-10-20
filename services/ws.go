package services

import (
	"sync"

	"github.com/gorilla/websocket"
)

type Client struct {
	ID   string
	Conn *websocket.Conn
}

var (
	clients      = make(map[string]*Client)
	adminClients = make(map[string]*Client)
	mu           sync.Mutex
)

// RegisterUser adds a user WebSocket
func RegisterUser(userID string, conn *websocket.Conn) {
	mu.Lock()
	clients[userID] = &Client{ID: userID, Conn: conn}
	mu.Unlock()
	println("user registered")
}

// RegisterAdmin adds an admin WebSocket
func RegisterAdmin(adminID string, conn *websocket.Conn) {
	mu.Lock()
	adminClients[adminID] = &Client{ID: adminID, Conn: conn}
	mu.Unlock()
	println("admin registered")
}

// SendToUser sends a message to a specific user
func SendToUser(userID string, payload map[string]interface{}) {
	mu.Lock()
	client, exists := clients[userID]
	mu.Unlock()

	if exists {
		client.Conn.WriteJSON(payload)
	}
}

// BroadcastToAdmins sends a message to all admin clients
func BroadcastToAdmins(payload map[string]interface{}) {
	mu.Lock()
	for _, client := range adminClients {
		client.Conn.WriteJSON(payload)
	}
	mu.Unlock()
}

// RemoveUser removes a user from the clients map
func RemoveUser(userID string) {
	mu.Lock()
	delete(clients, userID)
	mu.Unlock()
}

// RemoveAdmin removes an admin from the adminClients map
func RemoveAdmin(adminID string) {
	mu.Lock()
	delete(adminClients, adminID)
	mu.Unlock()
}
