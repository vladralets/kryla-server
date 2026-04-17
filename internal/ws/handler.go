package ws

import (
	"log/slog"
	"net/http"

	"nhooyr.io/websocket"
)

// Handler returns an http.HandlerFunc that upgrades connections to WebSocket
// and creates a Client for each connection.
func Handler(hub *Hub, router *Router) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			// Allow all origins for development. Tighten in production.
			InsecureSkipVerify: true,
		})
		if err != nil {
			slog.Error("websocket accept", "err", err)
			return
		}

		client := NewClient(conn, hub, router)
		slog.Info("new websocket connection", "remote", r.RemoteAddr)

		// Run blocks until the connection is closed.
		client.Run()
	}
}
