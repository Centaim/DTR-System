// Package photoaudit lets HR review attendance logs, their photos and flags.
package photoaudit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalid  = errors.New("invalid")
	ErrNotFound = errors.New("not found")
)

type Flag struct {
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}

type Review struct {
	Outcome    string    `json:"outcome"`
	Note       string    `json:"note"`
	Reviewer   string    `json:"reviewer"`
	ReviewedAt time.Time `json:"reviewed_at"`
}

type Item struct {
	LogID         int64     `json:"log_id"`
	Employee      string    `json:"employee"`
	EmployeeNo    string    `json:"employee_no"`
	Slot          string    `json:"slot"`
	RecordedAt    time.Time `json:"recorded_at"`
	Channel       string    `json:"channel"`
	Offline       bool      `json:"offline"`
	DeviceID      string    `json:"device_id"`
	Flags         []Flag    `json:"flags"`
	PreviousLogID *int64    `json:"previous_log_id"`
	Review        *Review   `json:"review"`
}

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

const base = `
	SELECT l.id, e.full_name, e.employee_no, l.slot::text, l.recorded_at, l.channel::text, l.offline, l.device_id,
	  COALESCE((SELECT json_agg(json_build_object('kind', f.kind::text, 'detail', f.detail) ORDER BY f.id)
	             FROM attendance_flags f WHERE f.log_id = l.id), '[]'::json),
	  (SELECT p.id FROM attendance_logs p WHERE p.employee_id = l.employee_id AND p.id < l.id ORDER BY p.id DESC LIMIT 1),
	  r.outcome::text, r.note, rv.full_name, r.reviewed_at
	FROM attendance_logs l
	JOIN employees e ON e.id = l.employee_id
	LEFT JOIN log_reviews r ON r.log_id = l.id
	LEFT JOIN employees rv ON rv.id = r.reviewed_by`

// List returns logs for one view: "flagged" (open flags), "sample" (random
// unreviewed logs from the last week), "followup", or "date" (a calendar day).
func (s *Store) List(ctx context.Context, view string, day string) ([]Item, error) {
	var where, order string
	var args []any
	switch view {
	case "flagged":
		where = `WHERE r.log_id IS NULL AND EXISTS (SELECT 1 FROM attendance_flags f WHERE f.log_id = l.id)`
		order = `ORDER BY l.recorded_at DESC LIMIT 100`
	case "sample":
		where = `WHERE r.log_id IS NULL AND l.recorded_at > now() - interval '7 days'`
		order = `ORDER BY random() LIMIT 20`
	case "followup":
		where = `WHERE r.outcome = 'follow_up'`
		order = `ORDER BY r.reviewed_at DESC LIMIT 100`
	case "date":
		if _, err := time.Parse("2006-01-02", day); err != nil {
			return nil, fmt.Errorf("%w: choose a date", ErrInvalid)
		}
		where = `WHERE l.work_date = $1`
		order = `ORDER BY l.recorded_at DESC LIMIT 200`
		args = []any{day}
	default:
		return nil, fmt.Errorf("%w: unknown view", ErrInvalid)
	}

	rows, err := s.pool.Query(ctx, base+" "+where+" "+order, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Item{}
	for rows.Next() {
		var it Item
		var flagsJSON []byte
		var outcome, note, reviewer *string
		var at *time.Time
		if err := rows.Scan(&it.LogID, &it.Employee, &it.EmployeeNo, &it.Slot, &it.RecordedAt, &it.Channel,
			&it.Offline, &it.DeviceID, &flagsJSON, &it.PreviousLogID, &outcome, &note, &reviewer, &at); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(flagsJSON, &it.Flags); err != nil {
			return nil, err
		}
		if outcome != nil && at != nil {
			it.Review = &Review{Outcome: *outcome, Note: deref(note), Reviewer: deref(reviewer), ReviewedAt: *at}
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// PhotoKey returns the storage key for a log's photo.
func (s *Store) PhotoKey(ctx context.Context, logID int64) (string, error) {
	var key string
	err := s.pool.QueryRow(ctx, `SELECT photo_key FROM attendance_logs WHERE id = $1`, logID).Scan(&key)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return key, err
}

// SetReview records the reviewer's outcome. A follow-up needs a note.
func (s *Store) SetReview(ctx context.Context, logID, reviewer int64, outcome, note string) error {
	note = strings.TrimSpace(note)
	if outcome != "cleared" && outcome != "follow_up" {
		return fmt.Errorf("%w: choose an outcome", ErrInvalid)
	}
	if outcome == "follow_up" && len(note) < 3 {
		return fmt.Errorf("%w: say what needs follow-up", ErrInvalid)
	}
	if len(note) > 300 {
		return fmt.Errorf("%w: keep the note under 300 characters", ErrInvalid)
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO log_reviews (log_id, outcome, note, reviewed_by)
		VALUES ($1, $2::review_outcome, $3, $4)
		ON CONFLICT (log_id) DO UPDATE
		  SET outcome = EXCLUDED.outcome, note = EXCLUDED.note,
		      reviewed_by = EXCLUDED.reviewed_by, reviewed_at = now()`,
		logID, outcome, note, reviewer)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		return ErrNotFound
	}
	return err
}

// Message strips the sentinel prefix so the text can be shown to a person.
func Message(err error) string {
	return strings.TrimPrefix(err.Error(), "invalid: ")
}
