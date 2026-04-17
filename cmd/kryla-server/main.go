package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/kryla-chat/server/internal/auth"
	"github.com/kryla-chat/server/internal/config"
	"github.com/kryla-chat/server/internal/identity"
	"github.com/kryla-chat/server/internal/migrate"
	"github.com/kryla-chat/server/internal/prekey"
	"github.com/kryla-chat/server/internal/relay"
	"github.com/kryla-chat/server/internal/server"
	ksync "github.com/kryla-chat/server/internal/sync"
	"github.com/kryla-chat/server/internal/ws"
)

func main() {
	// Structured logging
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg := config.Load()
	slog.Info("config loaded",
		"server_id", cfg.ServerID,
		"listen", cfg.ListenAddr,
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// ── PostgreSQL ──────────────────────────────
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("connect to postgres", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		slog.Error("ping postgres", "err", err)
		os.Exit(1)
	}
	slog.Info("postgres connected")

	// ── Run migrations ─────────────────────────
	if err := migrate.Run(ctx, pool); err != nil {
		slog.Error("run migrations", "err", err)
		os.Exit(1)
	}

	// ── Redis (optional) ───────────────────────
	var rdb *redis.Client
	if cfg.RedisURL != "" {
		redisOpts, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			slog.Error("parse redis url", "err", err)
			os.Exit(1)
		}
		rdb = redis.NewClient(redisOpts)
		defer rdb.Close()

		if err := rdb.Ping(ctx).Err(); err != nil {
			slog.Warn("redis unreachable, continuing without it", "err", err)
			rdb = nil
		} else {
			slog.Info("redis connected")
		}
	} else {
		slog.Info("redis disabled (no REDIS_URL)")
	}

	// ── Domain layers ──────────────────────────
	identityStore := identity.NewStore(pool)
	identityHandler := identity.NewHandler(identityStore)

	prekeyStore := prekey.NewStore(pool)
	prekeyHandler := prekey.NewHandler(prekeyStore)

	authenticator := auth.NewAuthenticator(identityHandler)

	offlineQueue := relay.NewOfflineQueue(rdb)

	hub := ws.NewHub()
	go hub.Run()

	msgRelay := relay.NewRelay(hub, offlineQueue)

	// ── Peer sync ──────────────────────────────
	peerHandler := func(msg ksync.PeerMessage) {
		client := hub.GetClient(msg.To)
		if client == nil {
			slog.Warn("peer message for offline user", "to", msg.To)
			return
		}
		env := struct {
			Type      string `json:"type"`
			ID        string `json:"id"`
			Timestamp int64  `json:"timestamp"`
			From      string `json:"from"`
			Encrypted string `json:"encrypted"`
			Header    string `json:"header"`
			MessageID string `json:"message_id"`
		}{
			Type:      "incoming_message",
			ID:        msg.MessageID,
			Timestamp: time.Now().UnixMilli(),
			From:      msg.From,
			Encrypted: msg.Encrypted,
			Header:    msg.Header,
			MessageID: msg.MessageID,
		}
		data, err := json.Marshal(env)
		if err != nil {
			slog.Error("marshal peer env", "err", err)
			return
		}
		client.Send(data)
	}

	peerSync := ksync.NewPeerSync(rdb, cfg.ServerID, peerHandler)
	go func() {
		if err := peerSync.Subscribe(ctx); err != nil && ctx.Err() == nil {
			slog.Error("peer sync subscribe", "err", err)
		}
	}()

	router := ws.NewRouter(authenticator, msgRelay, prekeyHandler, peerSync)

	// ── HTTP server ────────────────────────────
	srv := server.New(cfg, hub, router)

	go func() {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	slog.Info("kryla server running", "addr", cfg.ListenAddr)

	// Wait for shutdown signal
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "err", err)
	}

	slog.Info("kryla server stopped")
}
