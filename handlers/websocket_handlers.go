package handlers

import (
	"log"
	"net/http"

	"cron-job-manager/services"
	"cron-job-manager/utils"
)

// WebSocketHandler handles WebSocket connections
func WebSocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := utils.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	utils.AddClient(conn)
	defer utils.RemoveClient(conn)

	go services.BroadcastSystemStats()

	// Keep connection alive and handle messages
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}
	}
}
