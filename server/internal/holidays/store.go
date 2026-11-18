// Package holidays keeps the agency's holiday calendar.
package holidays

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalid   = errors.New("invalid")
	ErrDuplicate = errors.New("duplicate")
	ErrNotFound  = errors.New("not found")
)

type Holiday struct {
	Day  string `json:"day"`
	Name string `json:"name"`
}

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) List(ctx context.Context, year int) ([]Holiday, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT day::text, name FROM holidays WHERE EXTRACT(year FROM day) = $1 ORDER BY day`, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Holiday{}
	for rows.Next() {
		var h Holiday
		if err := rows.Scan(&h.Day, &h.Name); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// ForMonth returns day-of-month to an upper-case label, as printed on the DTR.
func (s *Store) ForMonth(ctx context.Context, year int, month time.Month) (map[int]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT EXTRACT(day FROM day)::int, upper(name) FROM holidays
		WHERE EXTRACT(year FROM day) = $1 AND EXTRACT(month FROM day) = $2`, year, int(month))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]string{}
	for rows.Next() {
		var d int
		var name string
		if err := rows.Scan(&d, &name); err != nil {
			return nil, err
		}
		out[d] = name
	}
	return out, rows.Err()
}

func (s *Store) Create(ctx context.Context, day, name string, by int64) error {
	name = strings.TrimSpace(name)
	if _, err := time.Parse("2006-01-02", day); err != nil {
		return fmt.Errorf("%w: choose a date", ErrInvalid)
	}
	if name == "" || len(name) > 80 {
		return fmt.Errorf("%w: give the holiday a name (80 characters at most)", ErrInvalid)
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO holidays (day, name, created_by) VALUES ($1, $2, $3)`, day, name, by)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrDuplicate
	}
	return err
}

func (s *Store) Delete(ctx context.Context, day string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM holidays WHERE day = $1`, day)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func Message(err error) string { return strings.TrimPrefix(err.Error(), "invalid: ") }
