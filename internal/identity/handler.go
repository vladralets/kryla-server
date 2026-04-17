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
//
// If clientKrylaID is provided (non-empty) and the public key is unseen, use
// it as the kryla ID. This preserves the user's locally-generated identifier.
// If the public key already exists, return the stored kryla ID regardless of
// what the client supplied.
func (h *Handler) RegisterOrGet(ctx context.Context, publicKeyHex, clientKrylaID string) (string, error) {
	// Check if this public key already exists
	existing, err := h.store.GetByPublicKey(ctx, publicKeyHex)
	if err != nil {
		return "", fmt.Errorf("lookup by public key: %w", err)
	}
	if existing != nil {
		slog.Info("identity found", "kryla_id", existing.KrylaID)
		return existing.KrylaID, nil
	}

	// Pick kryla ID: prefer client's, fall back to deterministic derivation.
	krylaID := clientKrylaID
	if krylaID == "" {
		krylaID, err = KrylaIDFromPublicKey(publicKeyHex)
		if err != nil {
			return "", fmt.Errorf("derive kryla id: %w", err)
		}
	}

	if err := h.store.Register(ctx, krylaID, publicKeyHex); err != nil {
		return "", fmt.Errorf("register identity: %w", err)
	}

	slog.Info("identity registered", "kryla_id", krylaID)
	return krylaID, nil
}
