package relay

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// ClientLookup is the interface the relay uses to find connected clients.
type ClientLookup interface {
	GetClient(krylaID string) ClientSender
}

// ClientSender is the interface for sending a message to a connected client.
type ClientSender interface {
	Send(data []byte)
}

// IncomingEnvelope is the JSON shape sent to recipients.
type IncomingEnvelope struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Timestamp int64  `json:"timestamp"`
	From      string `json:"from"`
	Encrypted string `json:"encrypted"`
	Header    string `json:"header"`
	MessageID string `json:"message_id"`
}

// DeliveryResult describes what happened to a relayed message.
type DeliveryResult struct {
	MessageID string
	Delivered bool // true = delivered immediately, false = stored offline
}

// Relay holds the components needed to forward messages between users.
type Relay struct {
	hub     ClientLookup
	offline *OfflineQueue
}

// NewRelay creates a new Relay.
func NewRelay(hub ClientLookup, offline *OfflineQueue) *Relay {
	return &Relay{hub: hub, offline: offline}
}

// RelayMessage attempts to deliver a message to the recipient. If the
// recipient is online, the message is sent directly. Otherwise it is queued
// for later delivery.
func (r *Relay) RelayMessage(ctx context.Context, from, to, encrypted, header string) (*DeliveryResult, error) {
	messageID := uuid.New().String()

	env := IncomingEnvelope{
		Type:      "incoming_message",
		ID:        uuid.New().String(),
		Timestamp: time.Now().UnixMilli(),
		From:      from,
		Encrypted: encrypted,
		Header:    header,
		MessageID: messageID,
	}

	// Check if recipient is online
	client := r.hub.GetClient(to)
	if client != nil {
		data, err := json.Marshal(env)
		if err != nil {
			return nil, err
		}
		client.Send(data)
		slog.Info("message delivered", "from", from, "to", to, "message_id", messageID)
		return &DeliveryResult{MessageID: messageID, Delivered: true}, nil
	}

	// Recipient offline: queue the message
	offMsg := OfflineMessage{
		MessageID: messageID,
		From:      from,
		Encrypted: encrypted,
		Header:    header,
		StoredAt:  time.Now().UnixMilli(),
	}
	if err := r.offline.Enqueue(ctx, to, offMsg); err != nil {
		return nil, err
	}

	slog.Info("message stored offline", "from", from, "to", to, "message_id", messageID)
	return &DeliveryResult{MessageID: messageID, Delivered: false}, nil
}

// DrainOffline retrieves and delivers all queued messages for a user who just
// came online.
func (r *Relay) DrainOffline(ctx context.Context, krylaID string, sender ClientSender) error {
	msgs, err := r.offline.Drain(ctx, krylaID)
	if err != nil {
		return err
	}

	for _, m := range msgs {
		env := IncomingEnvelope{
			Type:      "incoming_message",
			ID:        uuid.New().String(),
			Timestamp: time.Now().UnixMilli(),
			From:      m.From,
			Encrypted: m.Encrypted,
			Header:    m.Header,
			MessageID: m.MessageID,
		}
		data, err := json.Marshal(env)
		if err != nil {
			slog.Error("marshal offline message", "err", err)
			continue
		}
		sender.Send(data)
	}

	if len(msgs) > 0 {
		slog.Info("drained offline messages", "kryla_id", krylaID, "count", len(msgs))
	}
	return nil
}
