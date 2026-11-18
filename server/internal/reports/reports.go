// Package reports builds the dashboard and the monthly attendance summary.
// The summary reuses dtr.Compute so a report can never disagree with a DTR.
package reports

import (
	"context"
	"strings"
	"time"

	"dtr/server/internal/attendance"
	"dtr/server/internal/dtr"
	"dtr/server/internal/holidays"
	"dtr/server/internal/requests"
	"dtr/server/internal/schedule"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool     *pgxpool.Pool
	att      *attendance.Service
	reqs     *requests.Store
	sched    *schedule.Store
	hol      *holidays.Store
	loc      *time.Location
	habitual int
}

func NewService(pool *pgxpool.Pool, att *attendance.Service, reqs *requests.Store,
	sched *schedule.Store, hol *holidays.Store, loc *time.Location, habitualLateDays int) *Service {
	return &Service{pool: pool, att: att, reqs: reqs, sched: sched, hol: hol, loc: loc, habitual: habitualLateDays}
}

type Row struct {
	EmployeeNo   string `json:"employee_no"`
	Name         string `json:"name"`
	Office       string `json:"office"`
	Position     string `json:"position"`
	PresentDays  int    `json:"present_days"`
	LateDays     int    `json:"late_days"`
	AbsentDays   int    `json:"absent_days"`
	ExcusedDays  int    `json:"excused_days"` // approved leave, official business, travel order
	UndertimeMin int    `json:"undertime_min"`
	Incomplete   int    `json:"incomplete_days"`
	Adjusted     int    `json:"adjusted_days"`
	Habitual     bool   `json:"habitual_late"`
}

type Summary struct {
	Month             string `json:"month"`
	WorkingDays       int    `json:"working_days"`
	HabitualThreshold int    `json:"habitual_threshold"`
	Rows              []Row  `json:"rows"`
}

func hhmmMinutes(s string) int {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return -1
	}
	return t.Hour()*60 + t.Minute()
}

func isWeekday(d time.Time) bool { return d.Weekday() != time.Saturday && d.Weekday() != time.Sunday }

// Summary counts per employee for one month. Absent means a weekday before
// today with no log, no approved request and no holiday.
func (s *Service) Summary(ctx context.Context, year int, month time.Month, office string) (Summary, error) {
	hol, err := s.hol.ForMonth(ctx, year, month)
	if err != nil {
		return Summary{}, err
	}
	now := time.Now().In(s.loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.loc)
	first := time.Date(year, month, 1, 0, 0, 0, 0, s.loc)

	out := Summary{Month: first.Format("2006-01"), HabitualThreshold: s.habitual, Rows: []Row{}}
	for d := first; d.Month() == month; d = d.AddDate(0, 0, 1) {
		if isWeekday(d) && hol[d.Day()] == "" {
			out.WorkingDays++
		}
	}

	q := `SELECT id, employee_no, full_name, office, position FROM employees WHERE active`
	args := []any{}
	if office != "" {
		q += ` AND office = $1`
		args = append(args, office)
	}
	rows, err := s.pool.Query(ctx, q+` ORDER BY office, full_name`, args...)
	if err != nil {
		return Summary{}, err
	}
	type emp struct {
		id                         int64
		no, name, office, position string
	}
	var emps []emp
	for rows.Next() {
		var e emp
		if err := rows.Scan(&e.id, &e.no, &e.name, &e.office, &e.position); err != nil {
			rows.Close()
			return Summary{}, err
		}
		emps = append(emps, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Summary{}, err
	}

	for _, e := range emps {
		logs, err := s.att.MonthLogs(ctx, e.id, year, month, s.loc)
		if err != nil {
			return Summary{}, err
		}
		adj, reqMarks, err := s.reqs.ForMonth(ctx, e.id, year, month)
		if err != nil {
			return Summary{}, err
		}
		marks := map[int]string{}
		for d, l := range hol {
			marks[d] = l
		}
		for d, l := range reqMarks {
			marks[d] = l
		}
		hf, err := s.sched.HoursFunc(ctx, e.id)
		if err != nil {
			return Summary{}, err
		}
		sheet := dtr.Compute(year, month, append(logs, adj...), marks, hf, s.loc)

		r := Row{EmployeeNo: e.no, Name: e.name, Office: e.office, Position: e.position,
			UndertimeMin: sheet.TotalUndertimeMin}
		for _, d := range sheet.Days {
			date := time.Date(year, month, d.Day, 0, 0, 0, 0, s.loc)
			has := d.AMIn != "" || d.AMOut != "" || d.PMIn != "" || d.PMOut != ""
			if has {
				r.PresentDays++
			}
			if d.AMIn != "" && hhmmMinutes(d.AMIn) > hf(date).AM.Start {
				r.LateDays++
			}
			if _, requested := reqMarks[d.Day]; requested && isWeekday(date) {
				r.ExcusedDays++
			}
			if !has && d.Mark == "" && isWeekday(date) && date.Before(today) {
				r.AbsentDays++
			}
			if d.Incomplete {
				r.Incomplete++
			}
			if d.Adjusted {
				r.Adjusted++
			}
		}
		r.Habitual = r.LateDays >= s.habitual
		out.Rows = append(out.Rows, r)
	}
	return out, nil
}

type OfficeCount struct {
	Office  string `json:"office"`
	Total   int    `json:"total"`
	Present int    `json:"present"`
	Late    int    `json:"late"`
	Excused int    `json:"excused"`
	NotIn   int    `json:"not_in"`
}

type Recent struct {
	Employee string    `json:"employee"`
	Slot     string    `json:"slot"`
	At       time.Time `json:"at"`
	Offline  bool      `json:"offline"`
}

type Dashboard struct {
	Date            string        `json:"date"`
	WorkingDay      bool          `json:"working_day"`
	Holiday         string        `json:"holiday"`
	Total           int           `json:"total"`
	Present         int           `json:"present"`
	Late            int           `json:"late"`
	Excused         int           `json:"excused"`
	NotIn           int           `json:"not_in"`
	PendingRequests int           `json:"pending_requests"`
	OpenFlags       int           `json:"open_flags"`
	OfflineToday    int           `json:"offline_today"`
	Offices         []OfficeCount `json:"offices"`
	Recent          []Recent      `json:"recent"`
}

func (s *Service) Dashboard(ctx context.Context) (Dashboard, error) {
	now := time.Now().In(s.loc)
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.loc)
	dayStr := day.Format("2006-01-02")

	hol, err := s.hol.ForMonth(ctx, day.Year(), day.Month())
	if err != nil {
		return Dashboard{}, err
	}
	d := Dashboard{Date: dayStr, Holiday: hol[day.Day()], Offices: []OfficeCount{}, Recent: []Recent{}}
	d.WorkingDay = isWeekday(day) && d.Holiday == ""

	perEmp, def, err := s.sched.HoursOn(ctx, day)
	if err != nil {
		return Dashboard{}, err
	}

	firstIn := map[int64]time.Time{}
	present := map[int64]bool{}
	rows, err := s.pool.Query(ctx, `SELECT employee_id, slot::text, recorded_at FROM attendance_logs WHERE work_date = $1`, dayStr)
	if err != nil {
		return Dashboard{}, err
	}
	for rows.Next() {
		var id int64
		var slot string
		var at time.Time
		if err := rows.Scan(&id, &slot, &at); err != nil {
			rows.Close()
			return Dashboard{}, err
		}
		present[id] = true
		if slot == "am_in" {
			firstIn[id] = at.In(s.loc)
		}
	}
	rows.Close()

	excused := map[int64]bool{}
	rows, err = s.pool.Query(ctx, `SELECT DISTINCT employee_id FROM requests
		WHERE status = 'approved' AND kind <> 'missed_log' AND work_date <= $1 AND COALESCE(date_to, work_date) >= $1`, dayStr)
	if err != nil {
		return Dashboard{}, err
	}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return Dashboard{}, err
		}
		excused[id] = true
	}
	rows.Close()

	rows, err = s.pool.Query(ctx, `SELECT id, office FROM employees WHERE active ORDER BY office`)
	if err != nil {
		return Dashboard{}, err
	}
	byOffice := map[string]*OfficeCount{}
	var order []string
	for rows.Next() {
		var id int64
		var office string
		if err := rows.Scan(&id, &office); err != nil {
			rows.Close()
			return Dashboard{}, err
		}
		if strings.TrimSpace(office) == "" {
			office = "No office"
		}
		oc := byOffice[office]
		if oc == nil {
			oc = &OfficeCount{Office: office}
			byOffice[office] = oc
			order = append(order, office)
		}
		d.Total++
		oc.Total++
		h, ok := perEmp[id]
		if !ok {
			h = def
		}
		switch {
		case present[id]:
			d.Present++
			oc.Present++
			if t, ok := firstIn[id]; ok && t.Hour()*60+t.Minute() > h.AM.Start {
				d.Late++
				oc.Late++
			}
		case excused[id]:
			d.Excused++
			oc.Excused++
		case d.WorkingDay:
			d.NotIn++
			oc.NotIn++
		}
	}
	rows.Close()
	for _, o := range order {
		d.Offices = append(d.Offices, *byOffice[o])
	}

	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM requests WHERE status = 'pending'`).Scan(&d.PendingRequests); err != nil {
		return Dashboard{}, err
	}
	if err := s.pool.QueryRow(ctx, `
		SELECT count(DISTINCT f.log_id) FROM attendance_flags f
		  LEFT JOIN log_reviews r ON r.log_id = f.log_id WHERE r.log_id IS NULL`).Scan(&d.OpenFlags); err != nil {
		return Dashboard{}, err
	}
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM attendance_logs WHERE work_date = $1 AND offline`, dayStr).Scan(&d.OfflineToday); err != nil {
		return Dashboard{}, err
	}

	rows, err = s.pool.Query(ctx, `SELECT e.full_name, l.slot::text, l.recorded_at, l.offline
		FROM attendance_logs l JOIN employees e ON e.id = l.employee_id
		WHERE l.work_date = $1 ORDER BY l.recorded_at DESC LIMIT 8`, dayStr)
	if err != nil {
		return Dashboard{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var r Recent
		if err := rows.Scan(&r.Employee, &r.Slot, &r.At, &r.Offline); err != nil {
			return Dashboard{}, err
		}
		d.Recent = append(d.Recent, r)
	}
	return d, rows.Err()
}
