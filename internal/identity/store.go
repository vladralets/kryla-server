package identity

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Identity represents a registered user identity.
type Identity struct {
	KrylaID   string
	PublicKey string
	CreatedAt time.Time
}

// Store persists identity records in PostgreSQL.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore creates a new identity Store.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// KrylaIDFromPublicKey deterministically derives a kryla ID from a public key
// hex string. The ID is the first 16 hex chars of SHA-256(pubkey_bytes).
func KrylaIDFromPublicKey(publicKeyHex string) (string, error) {
	pub, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		return "", fmt.Errorf("decode public key: %w", err)
	}
	h := sha256.Sum256(pub)
	return hex.EncodeToString(h[:8]), nil
}

// Register inserts a new identity. Returns the krylaID.
func (s *Store) Register(ctx context.Context, krylaID, publicKey string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO identities (kryla_id, public_key, created_at)
		 VALUES ($1, $2, NOW())
		 ON CONFLICT (kryla_id) DO NOTHING`,
		krylaID, publicKey,
	)
	return err
}

// GetByID looks up an identity by its kryla ID.
func (s *Store) GetByID(ctx context.Context, krylaID string) (*Identity, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT kryla_id, public_key, created_at FROM identities WHERE kryla_id = $1`,
		krylaID,
	)
	var id Identity
	err := row.Scan(&id.KrylaID, &id.PublicKey, &id.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// GetByPublicKey looks up an identity by its public key hex.
func (s *Store) GetByPublicKey(ctx context.Context, publicKey string) (*Identity, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT kryla_id, public_key, created_at FROM identities WHERE public_key = $1`,
		publicKey,
	)
	var id Identity
	err := row.Scan(&id.KrylaID, &id.PublicKey, &id.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &id, nil
}
