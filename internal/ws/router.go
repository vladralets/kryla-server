package ws

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/kryla-chat/server/internal/auth"
	"github.com/kryla-chat/server/internal/prekey"
	"github.com/kryla-chat/server/internal/relay"
	ksync "github.com/kryla-chat/server/internal/sync"
)

// Router dispatches parsed WebSocket messages to the appropriate handler.
type Router struct {
	authenticator *auth.Authenticator
	relay         *relay.Relay
	prekeyHandler *prekey.Handler
	peerSync      *ksync.PeerSync
}

// NewRouter creates a new Router with all handler dependencies.
func NewRouter(
	authenticator *auth.Authenticator,
	relay *relay.Relay,
	prekeyHandler *prekey.Handler,
	peerSync *ksync.PeerSync,
) *Router {
	return &Router{
		authenticator: authenticator,
		relay:         relay,
		prekeyHandler: prekeyHandler,
		peerSync:      peerSync,
	}
}

// Route parses a raw message and dispatches it to the correct handler.
func (r *Router) Route(c *Client, data []byte) {
	msg, err := ParseClientMessage(data)
	if err != nil {
		slog.Warn("parse error", "err", err)
		sendError(c, "", 400, "invalid message: "+err.Error())
		return
	}

	switch m := msg.(type) {
	case *AuthenticateMsg:
		r.handleAuthenticate(c, m)
	case *PingMsg:
		r.handlePing(c, m)
	case *SendMessageMsg:
		r.handleSendMessage(c, m)
	case *FetchPreKeyBundleMsg:
		r.handleFetchPreKey(c, m)
	case *UploadPreKeysMsg:
		r.handleUploadPreKeys(c, m)
	case *AckMsg:
		r.handleAck(c, m)
	default:
		sendError(c, "", 400, "unhandled message type")
	}
}

// ──────────────────────────────────────────────
// Handlers
// ──────────────────────────────────────────────

func (r *Router) handleAuthenticate(c *Client, m *AuthenticateMsg) {
	ctx := context.Background()

	krylaID, err := r.authenticator.Verify(ctx, m.IdentityPublic, m.Signature, m.KrylaID)
	if err != nil {
		slog.Warn("auth failed", "err", err)
		sendError(c, m.ID, 401, "authentication failed: "+err.Error())
		return
	}

	c.SetKrylaID(krylaID)
	c.Authenticated = true
	c.hub.Register(c)

	// Set presence in Redis for cross-server routing.
	if r.peerSync != nil {
		if err := r.peerSync.SetPresence(ctx, krylaID); err != nil {
			slog.Error("set presence", "err", err)
		}
	}

	resp := AuthenticatedMsg{
		BaseEnvelope: NewBase(TypeAuthenticated, m.ID),
		KrylaID:      krylaID,
	}
	sendJSON(c, resp)

	// Drain offline messages.
	if err := r.relay.DrainOffline(ctx, krylaID, c); err != nil {
		slog.Error("drain offline", "err", err)
	}
}

func (r *Router) handlePing(c *Client, m *PingMsg) {
	if c.Authenticated && r.peerSync != nil {
		_ = r.peerSync.RefreshPresence(context.Background(), c.KrylaID())
	}
	resp := PongMsg{
		BaseEnvelope: NewBase(TypePong, m.ID),
	}
	sendJSON(c, resp)
}

func (r *Router) handleSendMessage(c *Client, m *SendMessageMsg) {
	if !requireAuth(c, m.ID) {
		return
	}
	ctx := context.Background()

	result, err := r.relay.RelayMessage(ctx, c.KrylaID(), m.To, m.Encrypted, m.Header)
	if err != nil {
		slog.Error("relay message", "err", err)
		sendError(c, m.ID, 500, "relay failed")
		return
	}

	if result.Delivered {
		resp := MessageDeliveredMsg{
			BaseEnvelope: NewBase(TypeMessageDelivered, m.ID),
			MessageID:    result.MessageID,
		}
		sendJSON(c, resp)
	} else {
		resp := MessageStoredMsg{
			BaseEnvelope: NewBase(TypeMessageStored, m.ID),
			MessageID:    result.MessageID,
		}
		sendJSON(c, resp)
	}
}

func (r *Router) handleFetchPreKey(c *Client, m *FetchPreKeyBundleMsg) {
	if !requireAuth(c, m.ID) {
		return
	}
	ctx := context.Background()

	bundle, err := r.prekeyHandler.HandleFetchBundle(ctx, m.UserID)
	if err != nil {
		slog.Error("fetch prekey bundle", "err", err)
		sendError(c, m.ID, 500, "fetch prekey bundle failed")
		return
	}
	if bundle == nil {
		sendError(c, m.ID, 404, "no prekey bundle for user")
		return
	}

	resp := PreKeyBundleMsg{
		BaseEnvelope:   NewBase(TypePreKeyBundle, m.ID),
		IdentityPublic: bundle.IdentityPublic,
		SignedPreKey:    bundle.SignedPreKey,
		SignedPreKeySig: bundle.SignedPreKeySig,
		OneTimePreKey:  bundle.OneTimePreKey,
	}
	sendJSON(c, resp)
}

func (r *Router) handleUploadPreKeys(c *Client, m *UploadPreKeysMsg) {
	if !requireAuth(c, m.ID) {
		return
	}
	ctx := context.Background()

	// Convert protocol PreKeyData to prekey.OneTimePreKey
	otks := make([]prekey.OneTimePreKey, len(m.OneTimePreKeys))
	for i, k := range m.OneTimePreKeys {
		otks[i] = prekey.OneTimePreKey{ID: k.ID, PublicKey: k.PublicKey}
	}

	err := r.prekeyHandler.HandleUploadPreKeys(ctx, c.KrylaID(), m.SignedPreKey, m.SignedPreKeySig, otks)
	if err != nil {
		slog.Error("upload prekeys", "err", err)
		sendError(c, m.ID, 500, "upload prekeys failed")
		return
	}

	// Acknowledge with a simple pong-style response (no dedicated type needed).
	resp := BaseEnvelope{
		Type:      "prekeys_stored",
		ID:        m.ID,
	}
	resp.Timestamp = NewBase("", "").Timestamp
	sendJSON(c, resp)
}

func (r *Router) handleAck(c *Client, m *AckMsg) {
	if !requireAuth(c, m.ID) {
		return
	}
	// For now, ACKs are logged. Future: update delivery status in DB.
	slog.Info("ack received", "kryla_id", c.KrylaID(), "message_id", m.MessageID)
}

// ──────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────

func requireAuth(c *Client, reqID string) bool {
	if !c.Authenticated {
		sendError(c, reqID, 401, "not authenticated")
		return false
	}
	return true
}

func sendJSON(c *Client, v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		slog.Error("marshal response", "err", err)
		return
	}
	c.Send(data)
}

func sendError(c *Client, reqID string, code int, message string) {
	resp := ErrorMsg{
		BaseEnvelope: NewBase(TypeError, reqID),
		Code:         code,
		Message:      message,
	}
	sendJSON(c, resp)
}
