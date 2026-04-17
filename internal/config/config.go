package config

import "os"

// Config holds all server configuration loaded from environment variables.
type Config struct {
	ServerID    string
	ListenAddr  string
	PeerURL     string
	DatabaseURL string
	RedisURL    string
	TLSCert     string
	TLSKey      string
}

// Load reads configuration from environment variables, applying defaults
// where appropriate.
func Load() *Config {
	// Render.com injects $PORT; honor it if present.
	listenAddr := envOrDefault("LISTEN_ADDR", ":8443")
	if port := os.Getenv("PORT"); port != "" {
		listenAddr = ":" + port
	}
	return &Config{
		ServerID:    envOrDefault("SERVER_ID", "ge"),
		ListenAddr:  listenAddr,
		PeerURL:     os.Getenv("PEER_URL"),
		DatabaseURL: envOrDefault("DATABASE_URL", "postgres://kryla:kryla@localhost:5432/kryla?sslmode=disable"),
		RedisURL:    envOrDefault("REDIS_URL", "redis://localhost:6379"),
		TLSCert:     os.Getenv("TLS_CERT"),
		TLSKey:      os.Getenv("TLS_KEY"),
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
