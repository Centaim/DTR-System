// Package kiosk registers office kiosks and authenticates them by device token.
package kiosk

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalid   = errors.New("invalid")
	ErrDuplicate = errors.New("duplicate")
	ErrNotFound  = errors.New("not found")
	ErrToken     = errors.New("unknown or inactive kiosk")
)

type Kiosk struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	Office     string     `json:"office"`
	Active     bool       `json:"active"`
	LastSeenAt *time.Time `json:"last_seen_at"`
}

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func hash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

const cols = `id, name, office, active, last_seen_at`

func scan(row pgx.Row) (Kiosk, error) {
	var k Kiosk
	return k, row.Scan(&k.ID, &k.Name, &k.Office, &k.Active, &k.LastSeenAt)
}

// Create registers a kiosk and returns its token. The token is shown once and
// cannot be recovered: only its hash is kept.
func (s *Store) Create(ctx context.Context, name, office string, by int64) (Kiosk, string, error) {
	name, office = strings.TrimSpace(name), strings.TrimSpace(office)
	if name == "" || len(name) > 60 || len(office) > 80 {
		return Kiosk{}, "", fmt.Errorf("%w: give the kiosk a name (60 characters at most)", ErrInvalid)
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return Kiosk{}, "", err
	}
	token := "ks_" + base64.RawURLEncoding.EncodeToString(raw)
	k, err := scan(s.pool.QueryRow(ctx,
		`INSERT INTO kiosks (name, office, token_hash, created_by) VALUES ($1,$2,$3,$4) RETURNING `+cols,
		name, office, hash(token), by))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return Kiosk{}, "", ErrDuplicate
	}
	return k, token, err
}

func (s *Store) List(ctx context.Context) ([]Kiosk, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+cols+` FROM kiosks ORDER BY active DESC, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Kiosk{}
	for rows.Next() {
		k, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (s *Store) SetActive(ctx context.Context, id int64, active bool) error {
	tag, err := s.pool.Exec(ctx, `UPDATE kiosks SET active = $2 WHERE id = $1`, id, active)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Authenticate finds an active kiosk by its token and records that it is alive.
func (s *Store) Authenticate(ctx context.Context, token string) (Kiosk, error) {
	if !strings.HasPrefix(token, "ks_") || len(token) > 100 {
		return Kiosk{}, ErrToken
	}
	k, err := scan(s.pool.QueryRow(ctx,
		`UPDATE kiosks SET last_seen_at = CASE WHEN last_seen_at IS NULL OR last_seen_at < now() - interval '30 seconds'
		                                       THEN now() ELSE last_seen_at END
		  WHERE token_hash = $1 AND active RETURNING `+cols, hash(token)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Kiosk{}, ErrToken
	}
	return k, err
}

func Message(err error) string { return strings.TrimPrefix(err.Error(), "invalid: ") }
