package ws

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 30 * time.Second
	maxMsgSize = 64 * 1024 // 64 KB
	sendBuffer = 256
)

// Client represents a single WebSocket connection.
type Client struct {
	conn    *websocket.Conn
	hub     *Hub
	router  *Router
	krylaID string
	send    chan []byte
	ctx     context.Context
	cancel  context.CancelFunc
	once    sync.Once
	Authenticated bool
}

// NewClient creates a new Client for the given WebSocket connection.
func NewClient(conn *websocket.Conn, hub *Hub, router *Router) *Client {
	ctx, cancel := context.WithCancel(context.Background())
	return &Client{
		conn:   conn,
		hub:    hub,
		router: router,
		send:   make(chan []byte, sendBuffer),
		ctx:    ctx,
		cancel: cancel,
	}
}

// KrylaID returns the authenticated kryla ID for this client.
func (c *Client) KrylaID() string {
	return c.krylaID
}

// SetKrylaID sets the kryla ID after successful authentication.
func (c *Client) SetKrylaID(id string) {
	c.krylaID = id
}

// Send queues a message for delivery to the WebSocket client.
func (c *Client) Send(data []byte) {
	select {
	case c.send <- data:
	default:
		slog.Warn("send buffer full, dropping message", "kryla_id", c.krylaID)
	}
}

// Close shuts down the client connection.
func (c *Client) Close() {
	c.once.Do(func() {
		c.cancel()
		close(c.send)
	})
}

// Run starts the read and write pumps. It blocks until the connection is
// closed.
func (c *Client) Run() {
	go c.writePump()
	c.readPump()
}

// readPump reads messages from the WebSocket and routes them.
func (c *Client) readPump() {
	defer func() {
		c.hub.Unregister(c)
		c.Close()
		c.conn.Close(websocket.StatusNormalClosure, "")
	}()

	c.conn.SetReadLimit(maxMsgSize)

	for {
		_, data, err := c.conn.Read(c.ctx)
		if err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
				websocket.CloseStatus(err) == websocket.StatusGoingAway {
				slog.Info("client disconnected normally", "kryla_id", c.krylaID)
			} else {
				slog.Debug("read error", "kryla_id", c.krylaID, "err", err)
			}
			return
		}

		c.router.Route(c, data)
	}
}

// writePump sends queued messages to the WebSocket.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				// Channel closed.
				return
			}
			ctx, cancel := context.WithTimeout(c.ctx, writeWait)
			err := c.conn.Write(ctx, websocket.MessageText, msg)
			cancel()
			if err != nil {
				slog.Debug("write error", "kryla_id", c.krylaID, "err", err)
				return
			}

		case <-ticker.C:
			ctx, cancel := context.WithTimeout(c.ctx, writeWait)
			err := c.conn.Ping(ctx)
			cancel()
			if err != nil {
				slog.Debug("ping error", "kryla_id", c.krylaID, "err", err)
				return
			}

		case <-c.ctx.Done():
			return
		}
	}
}
