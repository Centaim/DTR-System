// Package requests handles corrections and excused days, and their approval.
package requests

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"dtr/server/internal/dtr"
	"dtr/server/internal/settings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalid   = errors.New("invalid")
	ErrConflict  = errors.New("conflict")
	ErrNotFound  = errors.New("not found")
	ErrForbidden = errors.New("forbidden")
)

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, a...))
}

// Message strips the sentinel prefix so the text can be shown to a person.
func Message(err error) string {
	for _, p := range []string{"invalid: ", "conflict: ", "forbidden: ", "not found: "} {
		if rest, ok := strings.CutPrefix(err.Error(), p); ok {
			return rest
		}
	}
	return err.Error()
}

var kinds = map[string]string{
	"missed_log": "Missed log", "official_business": "OFFICIAL BUSINESS",
	"travel_order": "TRAVEL ORDER", "leave": "LEAVE",
}

var slots = map[string]bool{"am_in": true, "am_out": true, "pm_in": true, "pm_out": true}

type Request struct {
	ID           int64      `json:"id"`
	Kind         string     `json:"kind"`
	WorkDate     string     `json:"work_date"`
	DateTo       *string    `json:"date_to"`
	Slot         *string    `json:"slot"`
	Time         *string    `json:"time"`
	Reason       string     `json:"reason"`
	Status       string     `json:"status"`
	DecisionNote string     `json:"decision_note"`
	DecidedAt    *time.Time `json:"decided_at"`
	CreatedAt    time.Time  `json:"created_at"`
	HasFile      bool       `json:"has_attachment"`
	Source       string     `json:"source"`
	Employee     string     `json:"employee,omitempty"`
	EmployeeNo   string     `json:"employee_no,omitempty"`
}

type Input struct {
	Kind     string `json:"kind"`
	WorkDate string `json:"work_date"`
	DateTo   string `json:"date_to"`
	Slot     string `json:"slot"`
	Time     string `json:"time"`
	Reason   string `json:"reason"`
}

type Store struct {
	pool     *pgxpool.Pool
	settings *settings.Store
	loc      *time.Location
}

func NewStore(pool *pgxpool.Pool, s *settings.Store, loc *time.Location) *Store {
	return &Store{pool: pool, settings: s, loc: loc}
}

const cols = `r.id, r.kind::text, r.work_date::text, r.date_to::text, r.slot::text,
	to_char(r.requested_time,'HH24:MI'), r.reason, r.status::text, r.decision_note,
	r.decided_at, r.created_at, r.attachment_key <> '', r.source`

func scan(row pgx.Row, withEmployee bool) (Request, error) {
	var r Request
	dest := []any{&r.ID, &r.Kind, &r.WorkDate, &r.DateTo, &r.Slot, &r.Time, &r.Reason,
		&r.Status, &r.DecisionNote, &r.DecidedAt, &r.CreatedAt, &r.HasFile, &r.Source}
	if withEmployee {
		dest = append(dest, &r.Employee, &r.EmployeeNo)
	}
	return r, row.Scan(dest...)
}

func (s *Store) Create(ctx context.Context, employeeID int64, in Input) (Request, error) {
	if _, ok := kinds[in.Kind]; !ok {
		return Request{}, invalid("choose a request type")
	}
	reason := strings.TrimSpace(in.Reason)
	if len(reason) < 5 || len(reason) > 500 {
		return Request{}, invalid("give a reason of 5 to 500 characters")
	}
	day, err := time.ParseInLocation("2006-01-02", in.WorkDate, s.loc)
	if err != nil {
		return Request{}, invalid("choose a date")
	}
	today := time.Now().In(s.loc)
	todayDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, s.loc)
	window := s.settings.Int(settings.RequestWindowDays)
	if day.Before(todayDate.AddDate(0, 0, -window)) {
		return Request{}, invalid("requests can go back %d days at most. Ask HR for older dates", window)
	}

	var dateTo, slot, at any
	if in.Kind == "missed_log" {
		if day.After(todayDate) {
			return Request{}, invalid("a missed log cannot be dated in the future")
		}
		if !slots[in.Slot] {
			return Request{}, invalid("choose which log was missed")
		}
		if _, err := time.Parse("15:04", in.Time); err != nil {
			return Request{}, invalid("enter the time you arrived or left")
		}
		var exists bool
		if err := s.pool.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM attendance_logs WHERE employee_id=$1 AND work_date=$2 AND slot=$3::log_slot)`,
			employeeID, day.Format("2006-01-02"), in.Slot).Scan(&exists); err != nil {
			return Request{}, err
		}
		if exists {
			return Request{}, fmt.Errorf("%w: that log is already recorded for the day", ErrConflict)
		}
		slot, at = in.Slot, in.Time
	} else {
		if in.DateTo != "" {
			to, err := time.ParseInLocation("2006-01-02", in.DateTo, s.loc)
			if err != nil || to.Before(day) {
				return Request{}, invalid("the end date must be on or after the start date")
			}
			if to.Sub(day) > 31*24*time.Hour {
				return Request{}, invalid("a request can cover 31 days at most")
			}
			dateTo = in.DateTo
		}
		if day.After(todayDate.AddDate(1, 0, 0)) {
			return Request{}, invalid("that date is too far ahead")
		}
	}

	r, err := scan(s.pool.QueryRow(ctx, `
		WITH r AS (
		  INSERT INTO requests (employee_id, kind, work_date, date_to, slot, requested_time, reason)
		  VALUES ($1, $2::request_kind, $3, $4, $5::log_slot, $6::time, $7) RETURNING *)
		SELECT `+cols+` FROM r`,
		employeeID, in.Kind, day.Format("2006-01-02"), dateTo, slot, at, reason), false)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return Request{}, fmt.Errorf("%w: you already have a request for that log", ErrConflict)
	}
	return r, err
}

// Owner returns who filed a request.
func (s *Store) Owner(ctx context.Context, id int64) (int64, error) {
	var emp int64
	err := s.pool.QueryRow(ctx, `SELECT employee_id FROM requests WHERE id=$1`, id).Scan(&emp)
	return emp, err
}

func (s *Store) Mine(ctx context.Context, employeeID int64) ([]Request, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+cols+` FROM requests r
		WHERE r.employee_id = $1 ORDER BY r.created_at DESC LIMIT 100`, employeeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Request{}
	for rows.Next() {
		r, err := scan(rows, false)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) Cancel(ctx context.Context, employeeID, id int64) error {
	tag, err := s.pool.Exec(ctx, `UPDATE requests SET status='cancelled'
		WHERE id=$1 AND employee_id=$2 AND status='pending'`, id, employeeID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%w: only your own pending requests can be cancelled", ErrConflict)
	}
	return nil
}

// visible is the approval rule: a supervisor sees their own reports' requests,
// HR and super admins see everyone's, and nobody ever decides their own.
const visible = `r.employee_id <> $1 AND (e.supervisor_id = $1 OR $2 IN ('hr_admin','super_admin') OR e.supervisor_id IN (
	SELECT d.from_id FROM approval_delegations d WHERE d.to_id = $1
	   AND (now() AT TIME ZONE 'Asia/Manila')::date BETWEEN d.from_date AND d.to_date))`

func (s *Store) Pending(ctx context.Context, actorID int64, role string) ([]Request, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+cols+`, e.full_name, e.employee_no
		FROM requests r JOIN employees e ON e.id = r.employee_id
		WHERE r.status='pending' AND `+visible+` ORDER BY r.created_at LIMIT 200`, actorID, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Request{}
	for rows.Next() {
		r, err := scan(rows, true)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) Decide(ctx context.Context, actorID int64, role string, id int64, approve bool, note string) (Request, error) {
	note = strings.TrimSpace(note)
	if !approve && len(note) < 3 {
		return Request{}, invalid("say why the request is rejected")
	}
	if len(note) > 300 {
		return Request{}, invalid("keep the note under 300 characters")
	}
	status := "rejected"
	if approve {
		status = "approved"
	}
	r, err := scan(s.pool.QueryRow(ctx, `
		WITH upd AS (
		  UPDATE requests r SET status=$4::request_status, decided_by=$1, decided_at=now(), decision_note=$5
		  FROM employees e
		  WHERE r.id=$3 AND r.status='pending' AND e.id = r.employee_id AND `+visible+`
		  RETURNING r.*)
		SELECT `+cols+` FROM upd r`, actorID, role, id, status, note), false)
	if errors.Is(err, pgx.ErrNoRows) {
		return Request{}, fmt.Errorf("%w: this request is no longer waiting for you", ErrConflict)
	}
	return r, err
}

// ForMonth returns the approved corrections as adjustments, and a label for each
// excused day, for building the DTR.
func (s *Store) ForMonth(ctx context.Context, employeeID int64, year int, month time.Month) ([]dtr.Log, map[int]string, error) {
	first := time.Date(year, month, 1, 0, 0, 0, 0, s.loc)
	last := first.AddDate(0, 1, -1)
	rows, err := s.pool.Query(ctx, `
		SELECT kind::text, work_date, COALESCE(date_to, work_date), slot::text, to_char(requested_time,'HH24:MI')
		  FROM requests
		 WHERE employee_id=$1 AND status='approved'
		   AND work_date <= $3 AND COALESCE(date_to, work_date) >= $2`,
		employeeID, first.Format("2006-01-02"), last.Format("2006-01-02"))
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var adj []dtr.Log
	marks := map[int]string{}
	for rows.Next() {
		var kind string
		var from, to time.Time
		var slot, hhmm *string
		if err := rows.Scan(&kind, &from, &to, &slot, &hhmm); err != nil {
			return nil, nil, err
		}
		if kind == "missed_log" && slot != nil && hhmm != nil {
			t, err := time.ParseInLocation("15:04", *hhmm, s.loc)
			if err != nil {
				continue
			}
			at := time.Date(from.Year(), from.Month(), from.Day(), t.Hour(), t.Minute(), 0, 0, s.loc)
			adj = append(adj, dtr.Log{Day: from.Day(), Slot: *slot, At: at, Adjusted: true})
			continue
		}
		for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
			if d.Year() == year && d.Month() == month {
				marks[d.Day()] = kinds[kind]
			}
		}
	}
	return adj, marks, rows.Err()
}
