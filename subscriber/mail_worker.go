package subscriber

import (
	"context"
	"encoding/json"
	"log"
	"net/smtp"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	Stream     = "mail_queue"
	Group      = "go-mailer"
	DLQ        = "mail_dlq"
	MaxRetries = 5
	TrimCount  = 1000
	RetryDelay = 5 * time.Second
)

type Payload struct {
	To      string `json:"to"`
	Subject string `json:"subject"`
	HTML    string `json:"html"`
	Type    string `json:"type"`
	Data    string `json:"data,omitempty"`
}

func StartMailWorker(rdb *redis.Client) error {
	ctx := context.Background()

	if err := rdb.XGroupCreateMkStream(ctx, Stream, Group, "$").Err(); err != nil && !isGroupExists(err) {
		return err
	}

	consumerName := "mail-worker-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	log.Printf("Mail worker started: %s", consumerName)

	for {
		// Read new messages
		streams, err := rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    Group,
			Consumer: consumerName,
			Streams:  []string{Stream, ">"},
			Count:    10,
			Block:    5 * time.Second,
		}).Result()

		if err == redis.Nil {
			continue
		}
		if err != nil {
			log.Printf("XReadGroup error: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for _, s := range streams {
			for _, msg := range s.Messages {
				if err := processMessage(ctx, rdb, msg); err != nil {
					log.Printf("Failed %s: %v", msg.ID, err)
				}
			}
		}

		// Reprocess pending messages occasionally
		retryPendingMessages(ctx, rdb, consumerName)
	}
}

func processMessage(ctx context.Context, rdb *redis.Client, msg redis.XMessage) error {
	payload, err := parsePayload(msg.Values)
	if err != nil {
		rdb.XAck(ctx, Stream, Group, msg.ID)
		return err
	}

	deliveryCount, _ := getPendingInfo(ctx, rdb, msg.ID)
	if deliveryCount > MaxRetries {
		moveToDLQ(ctx, rdb, msg.ID, payload, "max retries")
		return nil
	}

	log.Printf("Sending to %s (attempt %d)", payload.To, deliveryCount)
	if err := sendEmail(payload); err != nil {
		log.Printf("Send failed: %v", err)
		return err
	}

	rdb.XAck(ctx, Stream, Group, msg.ID)
	rdb.XDel(ctx, Stream, msg.ID)
	rdb.XTrimMaxLen(ctx, Stream, TrimCount)
	log.Printf("Sent to %s | ID: %s", payload.To, msg.ID)
	return nil
}

func retryPendingMessages(ctx context.Context, rdb *redis.Client, consumer string) {
	pending, err := rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: Stream,
		Group:  Group,
		Start:  "-",
		End:    "+",
		Count:  10,
	}).Result()
	if err != nil || len(pending) == 0 {
		return
	}

	for _, item := range pending {
		if item.RetryCount < MaxRetries {
			rdb.XClaim(ctx, &redis.XClaimArgs{
				Stream:   Stream,
				Group:    Group,
				Consumer: consumer,
				MinIdle:  5 * time.Second,
				Messages: []string{item.ID},
			})
			log.Printf("Reclaimed pending message: %s", item.ID)
		}
	}
}

func sendEmail(p Payload) error {
	println("sending mail...")
	auth := smtp.PlainAuth("", "olaoluwavincent@gmail.com", "you mail password", "smtp.gmail.com") // Mail password i used was gotten from Google OAuth Credentials sha, yours can be any.
	msg := []byte("To: " + p.To + "\r\n" +
		"Subject: " + p.Subject + "\r\n" +
		"MIME-version: 1.0;\r\n" +
		"Content-Type: text/html; charset=\"UTF-8\";\r\n\r\n" + p.HTML)
	return smtp.SendMail("smtp.gmail.com:587", auth, "olaoluwavincent@gmail.com", []string{p.To}, msg)
}

func parsePayload(values map[string]interface{}) (Payload, error) {
	var p Payload
	b, _ := json.Marshal(values)
	return p, json.Unmarshal(b, &p)
}

func isGroupExists(err error) bool {
	return err.Error() == "BUSYGROUP Consumer Group name already exists"
}

func getPendingInfo(ctx context.Context, rdb *redis.Client, msgID string) (int, error) {
	info, err := rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: Stream,
		Group:  Group,
		Start:  msgID,
		End:    msgID,
		Count:  1,
	}).Result()
	if err != nil || len(info) == 0 {
		return 1, nil
	}
	return int(info[0].RetryCount + 1), nil
}

func moveToDLQ(ctx context.Context, rdb *redis.Client, id string, p Payload, reason string) {
	rdb.XAck(ctx, Stream, Group, id)
	rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: DLQ,
		Values: map[string]interface{}{
			"original_id": id,
			"to":          p.To,
			"subject":     p.Subject,
			"reason":      reason,
			"failed_at":   time.Now().Unix(),
		},
	})
	log.Printf("Moved to DLQ: %s | %s", id, reason)
}
