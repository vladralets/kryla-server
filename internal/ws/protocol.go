package ws

import (
	"encoding/json"
	"fmt"
	"time"
)

// ──────────────────────────────────────────────
// Message type constants
// ──────────────────────────────────────────────

const (
	// Client -> Server
	TypeAuthenticate    = "authenticate"
	TypeSendMessage     = "send_message"
	TypeFetchPreKey     = "fetch_prekey_bundle"
	TypeUploadPreKeys   = "upload_prekeys"
	TypeAck             = "ack"
	TypePing            = "ping"

	// Server -> Client
	TypeAuthenticated    = "authenticated"
	TypeIncomingMessage  = "incoming_message"
	TypePreKeyBundle     = "prekey_bundle"
	TypeMessageDelivered = "message_delivered"
	TypeMessageStored    = "message_stored"
	TypePong             = "pong"
	TypeError            = "error"
)

// ──────────────────────────────────────────────
// Base envelope (every message has these fields)
// ──────────────────────────────────────────────

// BaseEnvelope is embedded in every protocol message.
type BaseEnvelope struct {
	Type      string `json:"type"`
	ID        string `json:"id"`
	Timestamp int64  `json:"timestamp"`
}

// NewBase creates a BaseEnvelope with the current unix-millis timestamp.
func NewBase(typ, id string) BaseEnvelope {
	return BaseEnvelope{
		Type:      typ,
		ID:        id,
		Timestamp: time.Now().UnixMilli(),
	}
}

// ──────────────────────────────────────────────
// Client messages
// ──────────────────────────────────────────────

type AuthenticateMsg struct {
	BaseEnvelope
	KrylaID        string `json:"kryla_id,omitempty"`
	IdentityPublic string `json:"identity_public"`
	Signature      string `json:"signature"`
}

type SendMessageMsg struct {
	BaseEnvelope
	To        string `json:"to"`
	Encrypted string `json:"encrypted"`
	Header    string `json:"header"`
}

type FetchPreKeyBundleMsg struct {
	BaseEnvelope
	UserID string `json:"user_id"`
}

type PreKeyData struct {
	ID        string `json:"id"`
	PublicKey string `json:"public_key"`
}

type UploadPreKeysMsg struct {
	BaseEnvelope
	SignedPreKey    string       `json:"signed_pre_key"`
	SignedPreKeySig string       `json:"signed_pre_key_sig"`
	OneTimePreKeys  []PreKeyData `json:"one_time_pre_keys"`
}

type AckMsg struct {
	BaseEnvelope
	MessageID string `json:"message_id"`
}

type PingMsg struct {
	BaseEnvelope
}

// ──────────────────────────────────────────────
// Server messages
// ──────────────────────────────────────────────

type AuthenticatedMsg struct {
	BaseEnvelope
	KrylaID string `json:"kryla_id"`
}

type IncomingMessageMsg struct {
	BaseEnvelope
	From      string `json:"from"`
	Encrypted string `json:"encrypted"`
	Header    string `json:"header"`
	MessageID string `json:"message_id"`
}

type PreKeyBundleMsg struct {
	BaseEnvelope
	IdentityPublic string `json:"identity_public"`
	SignedPreKey    string `json:"signed_pre_key"`
	SignedPreKeySig string `json:"signed_pre_key_sig"`
	OneTimePreKey  string `json:"one_time_pre_key,omitempty"`
}

type MessageDeliveredMsg struct {
	BaseEnvelope
	MessageID string `json:"message_id"`
}

type MessageStoredMsg struct {
	BaseEnvelope
	MessageID string `json:"message_id"`
}

type PongMsg struct {
	BaseEnvelope
}

type ErrorMsg struct {
	BaseEnvelope
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ──────────────────────────────────────────────
// Parsing
// ──────────────────────────────────────────────

// ParseClientMessage reads the "type" field from raw JSON and unmarshals into
// the matching concrete struct.
func ParseClientMessage(data []byte) (interface{}, error) {
	var env BaseEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("unmarshal envelope: %w", err)
	}

	var msg interface{}
	switch env.Type {
	case TypeAuthenticate:
		msg = &AuthenticateMsg{}
	case TypeSendMessage:
		msg = &SendMessageMsg{}
	case TypeFetchPreKey:
		msg = &FetchPreKeyBundleMsg{}
	case TypeUploadPreKeys:
		msg = &UploadPreKeysMsg{}
	case TypeAck:
		msg = &AckMsg{}
	case TypePing:
		msg = &PingMsg{}
	default:
		return nil, fmt.Errorf("unknown message type: %s", env.Type)
	}

	if err := json.Unmarshal(data, msg); err != nil {
		return nil, fmt.Errorf("unmarshal %s: %w", env.Type, err)
	}
	return msg, nil
}
