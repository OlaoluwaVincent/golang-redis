package subscriber

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/go-redis/redis/v8"
	"github.com/olaoluwavincent/go-microservice/services"
)

func StartRedisSubscriber(parentCtx context.Context, rdb *redis.Client) error {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	pubsub := rdb.PSubscribe(ctx, "notification:user:*")
	adminPubsub := rdb.Subscribe(ctx, "notification:admin")
	defer pubsub.Close()
	defer adminPubsub.Close()

	// Wait for subscription confirmation
	if _, err := pubsub.Receive(ctx); err != nil {
		return err
	}
	if _, err := adminPubsub.Receive(ctx); err != nil {
		return err
	}

	log.Println("Redis subscriber started")

	userCh := pubsub.Channel()
	adminCh := adminPubsub.Channel()

	for {
		select {
		case msg, ok := <-userCh:
			if !ok {
				return nil
			}
			handleUserMessage(msg)

		case msg, ok := <-adminCh:
			if !ok {
				return nil
			}
			if msg.Channel == "notification:admin" {
				handleAdminMessage(msg.Payload)
			}

		case <-ctx.Done():
			log.Println("Redis subscriber shutting down")
			return ctx.Err()
		}
	}
}

func handleUserMessage(msg *redis.Message) {
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(msg.Payload), &payload); err != nil {
		log.Printf("Invalid JSON: %v", err)
		return
	}

	userID := extractUserID(msg.Channel)
	if userID == "" {
		return
	}

	log.Printf("Sending to user %s: %+v", userID, payload)
	services.SendToUser(userID, payload)
}

func handleAdminMessage(payloadStr string) {
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
		log.Printf("Invalid admin JSON: %v", err)
		return
	}
	services.BroadcastToAdmins(payload)
}

func extractUserID(channel string) string {
	const prefix = "notification:user:"
	if strings.HasPrefix(channel, prefix) {
		return channel[len(prefix):]
	}
	return ""
}
