package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	"github.com/olaoluwavincent/go-microservice/controllers"
	"github.com/olaoluwavincent/go-microservice/subscriber"
)

var ctx = context.Background()
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func main() {
	rdb := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // Replace with you own docker or local redis
		Password: "vincent1428",    // Replace with you own docker or local redis :lol you don't need to be told yeah
		DB:       0,
	})

	pong, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Redis connected:", pong)

	// Start mail worker in background
	go func() {
		if err := subscriber.StartMailWorker(rdb); err != nil {
			log.Fatal("Mail worker failed:", err)
		}
	}()

	go func() {
		err := subscriber.StartRedisSubscriber(ctx, rdb)
		if err != nil {

		}
	}()

	r := gin.Default()

	// User WebSocket endpoint
	r.GET("/ws", func(c *gin.Context) {
		controllers.ConnectUserWebsocket(c, &upgrader)
	})

	// Admin WebSocket endpoint (using Alternative 1)
	r.GET("/ws/admin", func(c *gin.Context) {
		controllers.ConnectAdminWebsocket(c, &upgrader)
	})

	err = r.Run(":8080")
	if err != nil {
		return
	}
}
