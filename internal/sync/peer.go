package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	presenceTTL    = 2 * time.Minute
	presencePrefix = "presence:"
	relayPrefix    = "relay:"
)

// PeerMessage is the envelope for cross-server relay via Redis Pub/Sub.
type PeerMessage struct {
	MessageID string `json:"message_id"`
	From      string `json:"from"`
	To        string `json:"to"`
	Encrypted string `json:"encrypted"`
	Header    string `json:"header"`
}

// MessageHandler is called when a message arrives from another server for a
// user connected to this server.
type MessageHandler func(msg PeerMessage)

// PeerSync manages cross-server communication via Redis Pub/Sub and presence
// keys.
type PeerSync struct {
	rdb      *redis.Client
	serverID string
	handler  MessageHandler
}

// NewPeerSync creates a new PeerSync for the given server ID.
func NewPeerSync(rdb *redis.Client, serverID string, handler MessageHandler) *PeerSync {
	return &PeerSync{
		rdb:      rdb,
		serverID: serverID,
		handler:  handler,
	}
}

// SetPresence marks a user as connected to this server by setting a Redis key
// with a TTL.
func (p *PeerSync) SetPresence(ctx context.Context, krylaID string) error {
	if p.rdb == nil {
		return nil
	}
	key := presencePrefix + krylaID
	return p.rdb.Set(ctx, key, p.serverID, presenceTTL).Err()
}

// ClearPresence removes the presence key for a user on this server.
func (p *PeerSync) ClearPresence(ctx context.Context, krylaID string) error {
	if p.rdb == nil {
		return nil
	}
	key := presencePrefix + krylaID
	// Only delete if the value is this server's ID (avoid clearing another
	// server's presence for the same user).
	val, err := p.rdb.Get(ctx, key).Result()
	if err != nil {
		return nil // key doesn't exist, nothing to clear
	}
	if val == p.serverID {
		return p.rdb.Del(ctx, key).Err()
	}
	return nil
}

// RefreshPresence extends the TTL on a user's presence key. Call this
// periodically (e.g., on each ping).
func (p *PeerSync) RefreshPresence(ctx context.Context, krylaID string) error {
	if p.rdb == nil {
		return nil
	}
	key := presencePrefix + krylaID
	return p.rdb.Expire(ctx, key, presenceTTL).Err()
}

// LookupServer returns the server ID that owns a user's presence, or "" if the
// user is not online anywhere.
func (p *PeerSync) LookupServer(ctx context.Context, krylaID string) (string, error) {
	if p.rdb == nil {
		return "", nil
	}
	val, err := p.rdb.Get(ctx, presencePrefix+krylaID).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

// Publish sends a relay message to the server that owns the target user.
func (p *PeerSync) Publish(ctx context.Context, targetServer string, msg PeerMessage) error {
	if p.rdb == nil {
		return nil
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal peer message: %w", err)
	}
	channel := relayPrefix + targetServer
	return p.rdb.Publish(ctx, channel, data).Err()
}

// Subscribe listens for relay messages addressed to this server and dispatches
// them to the handler. This blocks until ctx is cancelled.
func (p *PeerSync) Subscribe(ctx context.Context) error {
	if p.rdb == nil {
		<-ctx.Done()
		return ctx.Err()
	}
	channel := relayPrefix + p.serverID
	sub := p.rdb.Subscribe(ctx, channel)
	defer sub.Close()

	slog.Info("peer sync subscribed", "channel", channel)

	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case redisMsg, ok := <-ch:
			if !ok {
				return nil
			}
			var msg PeerMessage
			if err := json.Unmarshal([]byte(redisMsg.Payload), &msg); err != nil {
				slog.Error("unmarshal peer message", "err", err)
				continue
			}
			slog.Info("peer message received", "from", msg.From, "to", msg.To)
			if p.handler != nil {
				p.handler(msg)
			}
		}
	}
}
