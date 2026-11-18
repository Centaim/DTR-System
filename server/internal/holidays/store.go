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

// Suspension stops work for everyone or one office, for a day or from a time.
type Suspension struct {
	ID     int64  `json:"id"`
	Day    string `json:"day"`
	From   string `json:"from_time"` // empty means the whole day
	Office string `json:"office"`    // empty means every office
	Reason string `json:"reason"`
}

func (s *Store) Suspensions(ctx context.Context, year int, month time.Month) ([]Suspension, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, day::text, COALESCE(to_char(from_time,'HH24:MI'),''), office, reason
		FROM work_suspensions WHERE EXTRACT(year FROM day)=$1 AND ($2 = 0 OR EXTRACT(month FROM day)=$2) ORDER BY day`, year, int(month))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Suspension{}
	for rows.Next() {
		var x Suspension
		if err := rows.Scan(&x.ID, &x.Day, &x.From, &x.Office, &x.Reason); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Store) CreateSuspension(ctx context.Context, in Suspension, by int64) error {
	in.Reason, in.Office = strings.TrimSpace(in.Reason), strings.TrimSpace(in.Office)
	if _, err := time.Parse("2006-01-02", in.Day); err != nil {
		return fmt.Errorf("%w: choose a date", ErrInvalid)
	}
	if len(in.Reason) < 3 || len(in.Reason) > 200 {
		return fmt.Errorf("%w: give a reason of 3 to 200 characters", ErrInvalid)
	}
	var from any
	if in.From != "" {
		if _, err := time.Parse("15:04", in.From); err != nil {
			return fmt.Errorf("%w: enter a valid time", ErrInvalid)
		}
		from = in.From
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO work_suspensions (day, from_time, office, reason, created_by)
		VALUES ($1, $2::time, $3, $4, $5)`, in.Day, from, in.Office, in.Reason, by)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrDuplicate
	}
	return err
}

func (s *Store) DeleteSuspension(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM work_suspensions WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
