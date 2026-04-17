package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const offlineTTL = 7 * 24 * time.Hour // 7 days

// OfflineMessage is the envelope stored when the recipient is not connected.
type OfflineMessage struct {
	MessageID string `json:"message_id"`
	From      string `json:"from"`
	Encrypted string `json:"encrypted"`
	Header    string `json:"header"`
	StoredAt  int64  `json:"stored_at"`
}

// OfflineQueue stores and retrieves messages for offline users via Redis.
type OfflineQueue struct {
	rdb *redis.Client
}

// NewOfflineQueue creates a new OfflineQueue backed by the given Redis client.
func NewOfflineQueue(rdb *redis.Client) *OfflineQueue {
	return &OfflineQueue{rdb: rdb}
}

func offlineKey(recipientID string) string {
	return fmt.Sprintf("offline:%s", recipientID)
}

// Enqueue appends a message to the recipient's offline queue and sets a 7-day
// expiration on the list.
func (q *OfflineQueue) Enqueue(ctx context.Context, recipientID string, msg OfflineMessage) error {
	if q.rdb == nil {
		return nil // Redis disabled — drop the message silently for MVP
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal offline msg: %w", err)
	}

	key := offlineKey(recipientID)
	pipe := q.rdb.Pipeline()
	pipe.RPush(ctx, key, data)
	pipe.Expire(ctx, key, offlineTTL)
	_, err = pipe.Exec(ctx)
	return err
}

// Drain retrieves all queued messages for a recipient and deletes the list
// atomically.
func (q *OfflineQueue) Drain(ctx context.Context, recipientID string) ([]OfflineMessage, error) {
	if q.rdb == nil {
		return nil, nil
	}
	key := offlineKey(recipientID)

	// Use a transaction to LRANGE + DEL atomically.
	var msgs []OfflineMessage
	err := q.rdb.Watch(ctx, func(tx *redis.Tx) error {
		raw, err := tx.LRange(ctx, key, 0, -1).Result()
		if err != nil {
			return err
		}

		if len(raw) == 0 {
			return nil
		}

		for _, r := range raw {
			var m OfflineMessage
			if err := json.Unmarshal([]byte(r), &m); err != nil {
				return fmt.Errorf("unmarshal offline msg: %w", err)
			}
			msgs = append(msgs, m)
		}

		_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			pipe.Del(ctx, key)
			return nil
		})
		return err
	}, key)

	return msgs, err
}

// Count returns the number of messages queued for a recipient.
func (q *OfflineQueue) Count(ctx context.Context, recipientID string) (int64, error) {
	if q.rdb == nil {
		return 0, nil
	}
	return q.rdb.LLen(ctx, offlineKey(recipientID)).Result()
}
