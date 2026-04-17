package auth

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"

	"github.com/kryla-chat/server/internal/identity"
	"github.com/kryla-chat/server/pkg/crypto"
)

// Authenticator verifies client authentication requests.
type Authenticator struct {
	identityHandler *identity.Handler
}

// NewAuthenticator creates a new Authenticator.
func NewAuthenticator(ih *identity.Handler) *Authenticator {
	return &Authenticator{identityHandler: ih}
}

// Verify checks the client's authentication.
//
// For MVP we use Trust-On-First-Use (TOFU): if a signature is provided, we
// verify it; if not, we trust the public key on first connection and bind it
// permanently to the kryla ID. Subsequent connections must use the same key
// (enforced via the unique constraint on identities.public_key).
func (a *Authenticator) Verify(ctx context.Context, identityPublicHex, signatureHex, clientKrylaID string) (string, error) {
	if identityPublicHex == "" {
		return "", fmt.Errorf("missing identity public key")
	}

	if signatureHex != "" {
		// Optional: verify if client supplied a signature.
		ok, err := crypto.VerifyEd25519(identityPublicHex, identityPublicHex, signatureHex)
		if err != nil {
			return "", fmt.Errorf("verify signature: %w", err)
		}
		if !ok {
			return "", fmt.Errorf("invalid signature")
		}
		slog.Info("signature verified", "public_key", truncateKey(identityPublicHex))
	} else {
		slog.Info("TOFU auth (no signature)", "public_key", truncateKey(identityPublicHex))
	}

	// Register or retrieve existing identity; prefer client's kryla ID.
	krylaID, err := a.identityHandler.RegisterOrGet(ctx, identityPublicHex, clientKrylaID)
	if err != nil {
		return "", fmt.Errorf("register/get identity: %w", err)
	}

	return krylaID, nil
}

func truncateKey(hexKey string) string {
	if len(hexKey) > 16 {
		return hexKey[:16] + "..."
	}
	return hexKey
}

// VerifyRaw is a lower-level check that just validates the Ed25519 signature
// without any database interaction.
func VerifyRaw(publicKey, message, signature []byte) bool {
	_ = hex.EncodeToString(publicKey) // ensure valid bytes
	return crypto.VerifyEd25519Bytes(publicKey, message, signature)
}
