package prekey

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Bundle is the prekey bundle returned to a requesting client.
type Bundle struct {
	IdentityPublic string
	SignedPreKey    string
	SignedPreKeySig string
	OneTimePreKey  string // may be empty if none available
}

// Store manages prekey storage in PostgreSQL.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore creates a new prekey Store.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// StoreSignedPreKey upserts the user's current signed prekey.
func (s *Store) StoreSignedPreKey(ctx context.Context, krylaID, prekey, sig string) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO signed_prekeys (kryla_id, signed_pre_key, signature, created_at)
		 VALUES ($1, $2, $3, NOW())
		 ON CONFLICT (kryla_id)
		 DO UPDATE SET signed_pre_key = EXCLUDED.signed_pre_key,
		               signature      = EXCLUDED.signature,
		               created_at     = NOW()`,
		krylaID, prekey, sig,
	)
	return err
}

// StoreOneTimePreKeys inserts a batch of one-time prekeys for a user.
func (s *Store) StoreOneTimePreKeys(ctx context.Context, krylaID string, keys []OneTimePreKey) error {
	if len(keys) == 0 {
		return nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, k := range keys {
		_, err := tx.Exec(ctx,
			`INSERT INTO one_time_prekeys (kryla_id, key_id, public_key, used, created_at)
			 VALUES ($1, $2, $3, false, NOW())
			 ON CONFLICT (kryla_id, key_id) DO NOTHING`,
			krylaID, k.ID, k.PublicKey,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// FetchBundle returns the prekey bundle for a user. It consumes one unused
// one-time prekey (marks it as used). If no one-time prekeys remain, the
// bundle is returned without one.
func (s *Store) FetchBundle(ctx context.Context, krylaID string) (*Bundle, error) {
	// Get identity public key
	var identityPublic string
	err := s.pool.QueryRow(ctx,
		`SELECT public_key FROM identities WHERE kryla_id = $1`, krylaID,
	).Scan(&identityPublic)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Get signed prekey
	var signedPreKey, signedPreKeySig string
	err = s.pool.QueryRow(ctx,
		`SELECT signed_pre_key, signature FROM signed_prekeys WHERE kryla_id = $1`, krylaID,
	).Scan(&signedPreKey, &signedPreKeySig)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // no signed prekey uploaded yet
	}
	if err != nil {
		return nil, err
	}

	bundle := &Bundle{
		IdentityPublic: identityPublic,
		SignedPreKey:    signedPreKey,
		SignedPreKeySig: signedPreKeySig,
	}

	// Try to claim one unused one-time prekey
	var otkPublic string
	err = s.pool.QueryRow(ctx,
		`UPDATE one_time_prekeys
		 SET used = true
		 WHERE ctid = (
		   SELECT ctid FROM one_time_prekeys
		   WHERE kryla_id = $1 AND used = false
		   ORDER BY created_at ASC
		   LIMIT 1
		   FOR UPDATE SKIP LOCKED
		 )
		 RETURNING public_key`, krylaID,
	).Scan(&otkPublic)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	bundle.OneTimePreKey = otkPublic

	return bundle, nil
}

// CountAvailable returns the number of unused one-time prekeys for a user.
func (s *Store) CountAvailable(ctx context.Context, krylaID string) (int, error) {
	var count int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM one_time_prekeys WHERE kryla_id = $1 AND used = false`, krylaID,
	).Scan(&count)
	return count, err
}

// OneTimePreKey is a single one-time prekey to store.
type OneTimePreKey struct {
	ID        string
	PublicKey string
}

// SignedPreKeyRecord is the stored signed prekey for a user.
type SignedPreKeyRecord struct {
	KrylaID      string
	SignedPreKey  string
	Signature    string
	CreatedAt    time.Time
}
