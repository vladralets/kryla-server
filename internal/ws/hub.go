package ws

import (
	"log/slog"
	"sync"

	"github.com/kryla-chat/server/internal/relay"
)

// Hub manages all active WebSocket clients, keyed by their kryla ID once
// authenticated.
type Hub struct {
	mu          sync.RWMutex
	clients     map[string]*Client // krylaID -> client
	register    chan *Client
	unregister  chan *Client
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		register:   make(chan *Client, 64),
		unregister: make(chan *Client, 64),
	}
}

// Run starts the hub's main loop. It should be called in a separate goroutine.
func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			if c.krylaID == "" {
				continue
			}
			h.mu.Lock()
			// If there's an existing connection for this ID, close it.
			if old, ok := h.clients[c.krylaID]; ok {
				slog.Warn("replacing existing connection", "kryla_id", c.krylaID)
				old.Close()
			}
			h.clients[c.krylaID] = c
			h.mu.Unlock()
			slog.Info("client registered", "kryla_id", c.krylaID)

		case c := <-h.unregister:
			if c.krylaID == "" {
				continue
			}
			h.mu.Lock()
			if existing, ok := h.clients[c.krylaID]; ok && existing == c {
				delete(h.clients, c.krylaID)
				slog.Info("client unregistered", "kryla_id", c.krylaID)
			}
			h.mu.Unlock()
		}
	}
}

// Register queues a client for registration (call after successful auth).
func (h *Hub) Register(c *Client) {
	h.register <- c
}

// Unregister queues a client for removal.
func (h *Hub) Unregister(c *Client) {
	h.unregister <- c
}

// GetClient returns the connected client for a kryla ID, or nil if offline.
// The return type satisfies relay.ClientSender so Hub implements
// relay.ClientLookup.
func (h *Hub) GetClient(krylaID string) relay.ClientSender {
	h.mu.RLock()
	defer h.mu.RUnlock()
	c, ok := h.clients[krylaID]
	if !ok {
		return nil
	}
	return c
}

// OnlineCount returns the number of connected authenticated clients.
func (h *Hub) OnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
