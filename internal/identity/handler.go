package identity

import (
	"context"
	"fmt"
	"log/slog"
)

// Handler processes identity-related operations during the auth flow.
type Handler struct {
	store *Store
}

// NewHandler creates a new identity Handler.
func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

// RegisterOrGet either registers a new identity for the given public key or
// returns the existing identity. It returns the kryla ID.
func (h *Handler) RegisterOrGet(ctx context.Context, publicKeyHex string) (string, error) {
	// Check if this public key already exists
	existing, err := h.store.GetByPublicKey(ctx, publicKeyHex)
	if err != nil {
		return "", fmt.Errorf("lookup by public key: %w", err)
	}
	if existing != nil {
		slog.Info("identity found", "kryla_id", existing.KrylaID)
		return existing.KrylaID, nil
	}

	// Derive deterministic kryla ID
	krylaID, err := KrylaIDFromPublicKey(publicKeyHex)
	if err != nil {
		return "", fmt.Errorf("derive kryla id: %w", err)
	}

	if err := h.store.Register(ctx, krylaID, publicKeyHex); err != nil {
		return "", fmt.Errorf("register identity: %w", err)
	}

	slog.Info("identity registered", "kryla_id", krylaID)
	return krylaID, nil
}
