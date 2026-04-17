package crypto

import (
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
)

// VerifyEd25519 checks that signature is a valid Ed25519 signature of message
// under publicKey. All values are hex-encoded strings.
func VerifyEd25519(publicKeyHex, messageHex, signatureHex string) (bool, error) {
	pub, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		return false, fmt.Errorf("decode public key: %w", err)
	}
	if len(pub) != ed25519.PublicKeySize {
		return false, fmt.Errorf("invalid public key length: %d", len(pub))
	}

	msg, err := hex.DecodeString(messageHex)
	if err != nil {
		return false, fmt.Errorf("decode message: %w", err)
	}

	sig, err := hex.DecodeString(signatureHex)
	if err != nil {
		return false, fmt.Errorf("decode signature: %w", err)
	}
	if len(sig) != ed25519.SignatureSize {
		return false, fmt.Errorf("invalid signature length: %d", len(sig))
	}

	return ed25519.Verify(ed25519.PublicKey(pub), msg, sig), nil
}

// VerifyEd25519Bytes checks that signature is a valid Ed25519 signature of
// message under publicKey using raw byte slices.
func VerifyEd25519Bytes(publicKey, message, signature []byte) bool {
	if len(publicKey) != ed25519.PublicKeySize {
		return false
	}
	if len(signature) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(ed25519.PublicKey(publicKey), message, signature)
}
