package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/kryla-chat/server/internal/config"
	"github.com/kryla-chat/server/internal/ws"
)

// Server wraps the HTTP server with health and WebSocket endpoints.
type Server struct {
	httpServer *http.Server
	cfg        *config.Config
}

// New creates a new Server.
func New(cfg *config.Config, hub *ws.Hub, router *ws.Router) *Server {
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","server_id":"%s","online":%d}`, cfg.ServerID, hub.OnlineCount())
	})

	// WebSocket endpoint
	mux.HandleFunc("GET /ws", ws.Handler(hub, router))

	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Configure TLS if cert and key are provided.
	if cfg.TLSCert != "" && cfg.TLSKey != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLSCert, cfg.TLSKey)
		if err != nil {
			slog.Error("load TLS keypair", "err", err)
		} else {
			srv.TLSConfig = &tls.Config{
				Certificates: []tls.Certificate{cert},
				MinVersion:   tls.VersionTLS12,
			}
		}
	}

	return &Server{httpServer: srv, cfg: cfg}
}

// Start begins listening. It blocks until the server is shut down.
func (s *Server) Start() error {
	if s.httpServer.TLSConfig != nil {
		slog.Info("starting TLS server", "addr", s.cfg.ListenAddr)
		return s.httpServer.ListenAndServeTLS("", "")
	}
	slog.Info("starting server", "addr", s.cfg.ListenAddr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the server with a timeout.
func (s *Server) Shutdown(ctx context.Context) error {
	slog.Info("shutting down server")
	return s.httpServer.Shutdown(ctx)
}
