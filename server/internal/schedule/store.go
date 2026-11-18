// Package schedule stores official-hours schedules and who they apply to.
package schedule

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"dtr/server/internal/dtr"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalid   = errors.New("invalid schedule")
	ErrDuplicate = errors.New("a schedule with that name already exists")
)

type Schedule struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	AMStart   string `json:"am_start"`
	AMEnd     string `json:"am_end"`
	PMStart   string `json:"pm_start"`
	PMEnd     string `json:"pm_end"`
	IsDefault bool   `json:"is_default"`
}

func (s Schedule) hours() (dtr.Hours, error) {
	return dtr.ParseHours(fmt.Sprintf("%s-%s,%s-%s", s.AMStart, s.AMEnd, s.PMStart, s.PMEnd))
}

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

const cols = `id, name, to_char(am_start,'HH24:MI'), to_char(am_end,'HH24:MI'),
	to_char(pm_start,'HH24:MI'), to_char(pm_end,'HH24:MI'), is_default`

func scan(row pgx.Row) (Schedule, error) {
	var s Schedule
	err := row.Scan(&s.ID, &s.Name, &s.AMStart, &s.AMEnd, &s.PMStart, &s.PMEnd, &s.IsDefault)
	return s, err
}

func (st *Store) List(ctx context.Context) ([]Schedule, error) {
	rows, err := st.pool.Query(ctx, `SELECT `+cols+` FROM schedules ORDER BY is_default DESC, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Schedule{}
	for rows.Next() {
		s, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (st *Store) Create(ctx context.Context, in Schedule) (Schedule, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" || len(in.Name) > 60 {
		return Schedule{}, fmt.Errorf("%w: name is required (60 characters at most)", ErrInvalid)
	}
	if _, err := in.hours(); err != nil {
		return Schedule{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	s, err := scan(st.pool.QueryRow(ctx, `
		INSERT INTO schedules (name, am_start, am_end, pm_start, pm_end)
		VALUES ($1, $2::time, $3::time, $4::time, $5::time) RETURNING `+cols,
		in.Name, in.AMStart, in.AMEnd, in.PMStart, in.PMEnd))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return Schedule{}, ErrDuplicate
	}
	return s, err
}

// Assign makes a schedule apply to an employee from a date. A second
// assignment for the same date replaces nothing: dates are unique per employee.
func (st *Store) Assign(ctx context.Context, employeeID, scheduleID int64, from time.Time, by int64) error {
	_, err := st.pool.Exec(ctx, `
		INSERT INTO schedule_assignments (employee_id, schedule_id, effective_from, created_by)
		VALUES ($1, $2, $3, $4)`, employeeID, scheduleID, from.Format("2006-01-02"), by)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return fmt.Errorf("%w: this employee already has a schedule starting that day", ErrInvalid)
		case "23503":
			return fmt.Errorf("%w: schedule not found", ErrInvalid)
		}
	}
	return err
}

// HoursFunc returns the official hours in force for an employee on any date:
// the latest assignment starting on or before that date, else the default.
func (st *Store) HoursFunc(ctx context.Context, employeeID int64) (func(time.Time) dtr.Hours, error) {
	def, err := scan(st.pool.QueryRow(ctx, `SELECT `+cols+` FROM schedules WHERE is_default`))
	if err != nil {
		return nil, fmt.Errorf("default schedule: %w", err)
	}
	defHours, err := def.hours()
	if err != nil {
		return nil, err
	}

	rows, err := st.pool.Query(ctx, `
		SELECT a.effective_from, to_char(s.am_start,'HH24:MI'), to_char(s.am_end,'HH24:MI'),
		       to_char(s.pm_start,'HH24:MI'), to_char(s.pm_end,'HH24:MI')
		  FROM schedule_assignments a JOIN schedules s ON s.id = a.schedule_id
		 WHERE a.employee_id = $1 ORDER BY a.effective_from`, employeeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type dated struct {
		from  string
		hours dtr.Hours
	}
	var list []dated
	for rows.Next() {
		var from time.Time
		var sc Schedule
		if err := rows.Scan(&from, &sc.AMStart, &sc.AMEnd, &sc.PMStart, &sc.PMEnd); err != nil {
			return nil, err
		}
		h, err := sc.hours()
		if err != nil {
			return nil, err
		}
		list = append(list, dated{from.Format("2006-01-02"), h})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return func(day time.Time) dtr.Hours {
		key := day.Format("2006-01-02")
		h := defHours
		for _, d := range list {
			if d.from <= key {
				h = d.hours
			}
		}
		return h
	}, nil
}

// HoursOn returns the official hours in force on one date for every employee
// that has an assignment, plus the default for everyone else.
func (st *Store) HoursOn(ctx context.Context, day time.Time) (map[int64]dtr.Hours, dtr.Hours, error) {
	def, err := scan(st.pool.QueryRow(ctx, `SELECT `+cols+` FROM schedules WHERE is_default`))
	if err != nil {
		return nil, dtr.Hours{}, fmt.Errorf("default schedule: %w", err)
	}
	defHours, err := def.hours()
	if err != nil {
		return nil, dtr.Hours{}, err
	}
	rows, err := st.pool.Query(ctx, `
		SELECT DISTINCT ON (a.employee_id) a.employee_id,
		       to_char(s.am_start,'HH24:MI'), to_char(s.am_end,'HH24:MI'),
		       to_char(s.pm_start,'HH24:MI'), to_char(s.pm_end,'HH24:MI')
		  FROM schedule_assignments a JOIN schedules s ON s.id = a.schedule_id
		 WHERE a.effective_from <= $1
		 ORDER BY a.employee_id, a.effective_from DESC`, day.Format("2006-01-02"))
	if err != nil {
		return nil, dtr.Hours{}, err
	}
	defer rows.Close()
	out := map[int64]dtr.Hours{}
	for rows.Next() {
		var id int64
		var sc Schedule
		if err := rows.Scan(&id, &sc.AMStart, &sc.AMEnd, &sc.PMStart, &sc.PMEnd); err != nil {
			return nil, dtr.Hours{}, err
		}
		h, err := sc.hours()
		if err != nil {
			return nil, dtr.Hours{}, err
		}
		out[id] = h
	}
	return out, defHours, rows.Err()
}
