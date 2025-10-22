package controllers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/olaoluwavincent/go-microservice/services"
)

func ConnectUserWebsocket(c *gin.Context, upgrader *websocket.Upgrader) {
	userID := c.Query("user_id")
	if userID == "" {
		c.String(http.StatusBadRequest, "user_id is required")
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}

	services.RegisterUser(userID, conn)
	defer services.RemoveUser(userID) // Cleanup when connection closes

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Println("WebSocket disconnected:", err)
			break
		}
	}
}

func ConnectAdminWebsocket(c *gin.Context, upgrader *websocket.Upgrader) {

	adminID := c.Query("user_id") // Optional, or generate a unique ID
	if adminID == "" {
		adminID = "admin_" + c.ClientIP() // Simple fallback ID
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("Admin WebSocket upgrade error:", err)
		return
	}

	services.RegisterAdmin(adminID, conn)
	defer services.RemoveAdmin(adminID) // Cleanup when connection closes

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			log.Println("Admin WebSocket disconnected:", err)
			break
		}
	}
}
