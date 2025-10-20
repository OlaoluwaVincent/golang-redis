package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	"github.com/olaoluwavincent/go-microservice/services"
	"github.com/olaoluwavincent/go-microservice/subscriber"
)

var ctx = context.Background()
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func main() {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "vincent1428",
		DB:       0,
	})

	pong, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Redis connected:", pong)

	go subscriber.StartRedisSubscriber(rdb)

	r := gin.Default()

	// User WebSocket endpoint
	r.GET("/ws", func(c *gin.Context) {
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
	})

	// Admin WebSocket endpoint (using Alternative 1)
	r.GET("/ws/admin", func(c *gin.Context) {
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
	})

	r.Run(":8080")
}
