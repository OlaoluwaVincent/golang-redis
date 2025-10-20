package subscriber

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/go-redis/redis/v8"
	"github.com/olaoluwavincent/go-microservice/services"
)

var ctx = context.Background()

func StartRedisSubscriber(rdb *redis.Client) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Subscribe to user-specific channels (pattern) and admin channel
	pubsub := rdb.PSubscribe(ctx, "notification:user:*")
	adminPubsub := rdb.Subscribe(ctx, "notification:admin")
	defer pubsub.Close()
	defer adminPubsub.Close()

	// Check if subscriptions were successful
	_, err := pubsub.Receive(ctx)
	if err != nil {
		log.Printf("Failed to subscribe to Redis channel pattern 'notification:user:*': %v", err)
		return err
	}
	_, err = adminPubsub.Receive(ctx)
	if err != nil {
		log.Printf("Failed to subscribe to Redis channel 'notification:admin': %v", err)
		return err
	}

	log.Println("Redis subscriber started for channels 'notification:user:*' and 'notification:admin'")

	// Receive messages from both channels
	userCh := pubsub.Channel()
	adminCh := adminPubsub.Channel()

	for {
		select {
		case msg, ok := <-userCh:
			if !ok {
				log.Println("User subscription channel closed")
				return nil
			}

			log.Printf("Received message on channel %s: %s", msg.Channel, msg.Payload)

			// Parse the JSON payload
			var payload map[string]interface{}
			if err := json.Unmarshal([]byte(msg.Payload), &payload); err != nil {
				log.Printf("Failed to parse Redis message on channel %s: %v", msg.Channel, err)
				continue
			}

			// Log the parsed payload for debugging
			log.Printf("Parsed payload on channel %s: %+v", msg.Channel, payload)

			// Check for expected fields in the payload
			id, idOk := payload["id"].(float64)
			caption, captionOk := payload["caption"].(string)
			message, messageOk := payload["message"].(string)

			if !idOk || !captionOk || !messageOk {
				log.Printf("Invalid payload structure on channel %s: missing id, caption, or message", msg.Channel)
				continue
			}

			// Extract user ID from the channel name
			userID := extractUserID(msg.Channel)
			if userID != "" {
				log.Printf("Sending notification to user %s: id=%.0f, caption=%s, message=%s", userID, id, caption, message)
				services.SendToUser(userID, payload)
			} else {
				log.Printf("No valid user ID extracted from channel %s, treating as admin message", msg.Channel)
				services.BroadcastToAdmins(payload)
			}

		case msg, ok := <-adminCh:
			if !ok {
				log.Println("Admin subscription channel closed")
				return nil
			}

			log.Printf("Received message on admin channel %s: %s", msg.Channel, msg.Payload)

			// Parse the JSON payload
			var payload map[string]interface{}
			if err := json.Unmarshal([]byte(msg.Payload), &payload); err != nil {
				log.Printf("Failed to parse Redis message on admin channel %s: %v", msg.Channel, err)
				continue
			}

			// Log the parsed payload for debugging
			log.Printf("Parsed payload on admin channel %s: %+v", msg.Channel, payload)

			// Check for expected fields in the payload
			id, idOk := payload["id"].(float64)
			caption, captionOk := payload["caption"].(string)
			message, messageOk := payload["message"].(string)

			if !idOk || !captionOk || !messageOk {
				log.Printf("Invalid payload structure on admin channel %s: missing id, caption, or message", msg.Channel)
				continue
			}

			log.Printf("Broadcasting admin notification: id=%.0f, caption=%s, message=%s", id, caption, message)
			services.BroadcastToAdmins(payload)

		case <-ctx.Done():
			log.Println("Context cancelled, stopping Redis subscriber")
			return ctx.Err()
		}
	}
}

// extractUserID extracts the user ID from the channel name (e.g., notification:user:01k4w0c2n3av0evjmvqs6yrf5c)
func extractUserID(channel string) string {
	const prefix = "notification:user:"
	if len(channel) > len(prefix) && strings.HasPrefix(channel, prefix) {
		return channel[len(prefix):]
	}
	return ""
}
