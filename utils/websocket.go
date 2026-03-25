package utils

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"cron-job-manager/models"

	"github.com/gorilla/websocket"
)

var (
	Clients    map[*websocket.Conn]bool
	ClientsMux sync.RWMutex
	Upgrader   = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

// InitWebSocket initializes WebSocket clients map
func InitWebSocket() {
	Clients = make(map[*websocket.Conn]bool)
	log.Println("✅ WebSocket server initialized!")
}

// BroadcastToClients broadcasts a message to all connected WebSocket clients
func BroadcastToClients(message models.WSMessage) {
	ClientsMux.RLock()
	defer ClientsMux.RUnlock()

	messageBytes, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling WebSocket message: %v", err)
		return
	}

	for client := range Clients {
		err := client.WriteMessage(websocket.TextMessage, messageBytes)
		if err != nil {
			log.Printf("WebSocket write error: %v", err)
			client.Close()
			delete(Clients, client)
		}
	}
}

// AddClient adds a new WebSocket client
func AddClient(conn *websocket.Conn) {
	ClientsMux.Lock()
	Clients[conn] = true
	clientCount := len(Clients)
	ClientsMux.Unlock()

	log.Printf("✅ New WebSocket client connected. Total clients: %d", clientCount)
}

// RemoveClient removes a WebSocket client
func RemoveClient(conn *websocket.Conn) {
	ClientsMux.Lock()
	delete(Clients, conn)
	clientCount := len(Clients)
	ClientsMux.Unlock()

	log.Printf("❌ WebSocket client disconnected. Total clients: %d", clientCount)
}

// GetClientCount returns the number of connected clients
func GetClientCount() int {
	ClientsMux.RLock()
	defer ClientsMux.RUnlock()
	return len(Clients)
}
