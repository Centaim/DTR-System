// Package attendance records clock-ins and reads the current day.
package attendance

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"dtr/server/internal/dtr"
	"dtr/server/internal/storage"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var Slots = []string{"am_in", "am_out", "pm_in", "pm_out"}

var (
	ErrDayComplete = errors.New("all four logs for today are already recorded")
	ErrTooSoon     = errors.New("a log was recorded moments ago")
	ErrBadPhoto    = errors.New("photo must be a JPEG, PNG or WebP image")
)

type Log struct {
	ID         int64      `json:"id"`
	Slot       string     `json:"slot"`
	RecordedAt time.Time  `json:"recorded_at"`
	ClientTime *time.Time `json:"client_time,omitempty"`
	Offline    bool       `json:"offline"`
	Channel    string     `json:"channel"`
}

type Today struct {
	WorkDate string  `json:"work_date"`
	Logs     []Log   `json:"logs"`
	NextSlot *string `json:"next_slot"`
	Now      string  `json:"server_time"`
}

type ClockInput struct {
	IdempotencyKey string
	Photo          []byte
	ClientTime     *time.Time
	Latitude       *float64
	Longitude      *float64
	DeviceID       string
	Offline        bool
	Channel        string
}

type Service struct {
	pool   *pgxpool.Pool
	store  storage.Store
	minGap time.Duration
}

func NewService(pool *pgxpool.Pool, store storage.Store, minGap time.Duration) *Service {
	return &Service{pool: pool, store: store, minGap: minGap}
}

const logCols = `id, slot::text, recorded_at, client_time, offline, channel::text`

func scanLog(row pgx.Row) (Log, error) {
	var l Log
	err := row.Scan(&l.ID, &l.Slot, &l.RecordedAt, &l.ClientTime, &l.Offline, &l.Channel)
	return l, err
}

// Clock records the employee's next slot for today. Retrying with the same
// idempotency key returns the original log instead of creating a second one.
// The server assigns the slot and the official time; the device only suggests.
func (s *Service) Clock(ctx context.Context, employeeID int64, in ClockInput) (Log, bool, error) {
	if !validImage(in.Photo) {
		return Log{}, false, ErrBadPhoto
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Log{}, false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Serialize clock-ins per employee so the slot count cannot race.
	if _, err := tx.Exec(ctx, `SELECT 1 FROM employees WHERE id = $1 FOR UPDATE`, employeeID); err != nil {
		return Log{}, false, err
	}

	existing, err := scanLog(tx.QueryRow(ctx,
		`SELECT `+logCols+` FROM attendance_logs WHERE employee_id = $1 AND idempotency_key = $2`,
		employeeID, in.IdempotencyKey))
	if err == nil {
		return existing, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Log{}, false, err
	}

	var workDate time.Time
	var count int
	var lastAt *time.Time
	if err := tx.QueryRow(ctx, `
		SELECT (now() AT TIME ZONE 'Asia/Manila')::date,
		       (SELECT count(*) FROM attendance_logs
		         WHERE employee_id = $1 AND work_date = (now() AT TIME ZONE 'Asia/Manila')::date),
		       (SELECT max(recorded_at) FROM attendance_logs WHERE employee_id = $1)`,
		employeeID).Scan(&workDate, &count, &lastAt); err != nil {
		return Log{}, false, err
	}
	if count >= len(Slots) {
		return Log{}, false, ErrDayComplete
	}
	if lastAt != nil && time.Since(*lastAt) < s.minGap {
		return Log{}, false, ErrTooSoon
	}

	key, err := photoKey(workDate, employeeID)
	if err != nil {
		return Log{}, false, err
	}
	sum, err := s.store.Put(ctx, key, in.Photo)
	if err != nil {
		return Log{}, false, fmt.Errorf("store photo: %w", err)
	}

	channel := in.Channel
	if channel == "" {
		channel = "mobile"
	}
	l, err := scanLog(tx.QueryRow(ctx, `
		INSERT INTO attendance_logs
		  (employee_id, work_date, slot, client_time, channel, device_id,
		   latitude, longitude, offline, idempotency_key, photo_key, photo_sha256)
		VALUES ($1,$2,$3::log_slot,$4,$5::log_channel,$6,$7,$8,$9,$10,$11,$12)
		RETURNING `+logCols,
		employeeID, workDate, Slots[count], in.ClientTime, channel, in.DeviceID,
		in.Latitude, in.Longitude, in.Offline, in.IdempotencyKey, key, sum))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Log{}, false, ErrTooSoon
		}
		return Log{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Log{}, false, err
	}
	return l, false, nil
}

func (s *Service) Today(ctx context.Context, employeeID int64) (Today, error) {
	var day time.Time
	var now time.Time
	if err := s.pool.QueryRow(ctx,
		`SELECT (now() AT TIME ZONE 'Asia/Manila')::date, now()`).Scan(&day, &now); err != nil {
		return Today{}, err
	}
	rows, err := s.pool.Query(ctx,
		`SELECT `+logCols+` FROM attendance_logs
		  WHERE employee_id = $1 AND work_date = $2 ORDER BY recorded_at`, employeeID, day)
	if err != nil {
		return Today{}, err
	}
	defer rows.Close()

	t := Today{WorkDate: day.Format("2006-01-02"), Logs: []Log{}, Now: now.UTC().Format(time.RFC3339)}
	for rows.Next() {
		l, err := scanLog(rows)
		if err != nil {
			return Today{}, err
		}
		t.Logs = append(t.Logs, l)
	}
	if err := rows.Err(); err != nil {
		return Today{}, err
	}
	if len(t.Logs) < len(Slots) {
		next := Slots[len(t.Logs)]
		t.NextSlot = &next
	}
	return t, nil
}

func validImage(b []byte) bool {
	if len(b) < 12 {
		return false
	}
	switch http.DetectContentType(b) {
	case "image/jpeg", "image/png", "image/webp":
		return true
	}
	return false
}

func photoKey(day time.Time, employeeID int64) (string, error) {
	r := make([]byte, 12)
	if _, err := rand.Read(r); err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%d/%s.img", day.Format("2006/01/02"), employeeID, hex.EncodeToString(r)), nil
}

// MonthLogs returns the employee's logs for a month in Asia/Manila time.
func (s *Service) MonthLogs(ctx context.Context, employeeID int64, year int, month time.Month, loc *time.Location) ([]dtr.Log, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT slot::text, recorded_at FROM attendance_logs
		 WHERE employee_id = $1 AND work_date >= $2 AND work_date < $3
		 ORDER BY recorded_at`,
		employeeID,
		time.Date(year, month, 1, 0, 0, 0, 0, time.UTC),
		time.Date(year, month+1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []dtr.Log
	for rows.Next() {
		var slot string
		var at time.Time
		if err := rows.Scan(&slot, &at); err != nil {
			return nil, err
		}
		local := at.In(loc)
		out = append(out, dtr.Log{Day: local.Day(), Slot: slot, At: local})
	}
	return out, rows.Err()
}
