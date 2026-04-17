package prekey

import (
	"context"
	"fmt"
	"log/slog"
)

// Handler processes prekey-related WebSocket messages.
type Handler struct {
	store *Store
}

// NewHandler creates a new prekey Handler.
func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

// HandleFetchBundle retrieves the prekey bundle for the requested user.
// Returns nil bundle if the user has no prekeys uploaded yet.
func (h *Handler) HandleFetchBundle(ctx context.Context, userID string) (*Bundle, error) {
	bundle, err := h.store.FetchBundle(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("fetch bundle: %w", err)
	}
	if bundle == nil {
		slog.Warn("no prekey bundle available", "user_id", userID)
		return nil, nil
	}

	slog.Info("prekey bundle fetched",
		"user_id", userID,
		"has_otk", bundle.OneTimePreKey != "",
	)
	return bundle, nil
}

// HandleUploadPreKeys stores the signed prekey and one-time prekeys for a user.
func (h *Handler) HandleUploadPreKeys(ctx context.Context, krylaID, signedPreKey, signedPreKeySig string, oneTimePreKeys []OneTimePreKey) error {
	if signedPreKey != "" {
		if err := h.store.StoreSignedPreKey(ctx, krylaID, signedPreKey, signedPreKeySig); err != nil {
			return fmt.Errorf("store signed prekey: %w", err)
		}
		slog.Info("signed prekey stored", "kryla_id", krylaID)
	}

	if len(oneTimePreKeys) > 0 {
		if err := h.store.StoreOneTimePreKeys(ctx, krylaID, oneTimePreKeys); err != nil {
			return fmt.Errorf("store one-time prekeys: %w", err)
		}
		slog.Info("one-time prekeys stored", "kryla_id", krylaID, "count", len(oneTimePreKeys))
	}

	return nil
}
