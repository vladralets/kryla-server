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

// Verify checks the client's authentication. The client signs its own public
// key with its private key (self-signature). On success the identity is
// registered (or looked up) and the kryla ID is returned.
func (a *Authenticator) Verify(ctx context.Context, identityPublicHex, signatureHex string) (string, error) {
	// The message being signed is the public key bytes themselves.
	ok, err := crypto.VerifyEd25519(identityPublicHex, identityPublicHex, signatureHex)
	if err != nil {
		return "", fmt.Errorf("verify signature: %w", err)
	}
	if !ok {
		return "", fmt.Errorf("invalid signature")
	}

	slog.Info("signature verified", "public_key", truncateKey(identityPublicHex))

	// Register or retrieve existing identity
	krylaID, err := a.identityHandler.RegisterOrGet(ctx, identityPublicHex)
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
