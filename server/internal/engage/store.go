// Package engage holds notifications, announcements, accomplishment reports,
// events and approval delegation.
package engage

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalid  = errors.New("invalid")
	ErrConflict = errors.New("conflict")
	ErrNotFound = errors.New("not found")
)

func invalid(msg string) error { return fmt.Errorf("%w: %s", ErrInvalid, msg) }

// Message strips the sentinel prefix so the text can be shown to a person.
func Message(err error) string {
	s := err.Error()
	for _, p := range []string{"invalid: ", "conflict: "} {
		s = strings.TrimPrefix(s, p)
	}
	return s
}

type Store struct {
	pool *pgxpool.Pool
	mail Mailer
	loc  *time.Location
}

func NewStore(pool *pgxpool.Pool, mail Mailer, loc *time.Location) *Store {
	return &Store{pool: pool, mail: mail, loc: loc}
}

// ---- Notifications

var subjects = map[string]string{
	"request_filed":    "A request is waiting for your approval",
	"request_approved": "Your request was approved",
	"request_rejected": "Your request was rejected",
	"dtr_submitted":    "A DTR is waiting for your verification",
	"dtr_verified":     "Your DTR was verified",
	"dtr_returned":     "Your DTR was returned for correction",
	"accomp_returned":  "Your accomplishment report was returned",
	"approval_overdue": "A request has been waiting too long",
	"missed_log":       "You have a missing log",
	"dtr_deadline":     "Please certify your DTR",
	"announcement":     "New announcement",
	"device_approved":  "Your new device was approved",
}

// Notify records an in-app alert and, when configured, sends an email in the
// background. A delivery failure never affects the action that caused it.
func (s *Store) Notify(ctx context.Context, employeeID int64, kind, detail, dedupe string) {
	var key any
	if dedupe != "" {
		key = dedupe
	}
	var email string
	err := s.pool.QueryRow(ctx, `
		WITH ins AS (
		  INSERT INTO notifications (employee_id, kind, detail, dedupe_key) VALUES ($1,$2,$3,$4)
		  ON CONFLICT DO NOTHING RETURNING employee_id)
		SELECT e.email FROM ins JOIN employees e ON e.id = ins.employee_id`, employeeID, kind, detail, key).Scan(&email)
	if errors.Is(err, pgx.ErrNoRows) {
		return
	}
	if err != nil {
		log.Printf("notify %s: %v", kind, err)
		return
	}
	if s.mail.Enabled() && email != "" {
		subject := subjects[kind]
		go func() {
			if err := s.mail.Send(email, subject, subject+"\n\n"+detail+"\n\nOpen the DTR system for details."); err != nil {
				log.Printf("email %s failed", kind)
			}
		}()
	}
}

// NotifyApprovers alerts an employee's supervisor, or HR when there is none.
func (s *Store) NotifyApprovers(ctx context.Context, employeeID int64, kind, detail string) {
	rows, err := s.pool.Query(ctx, `
		SELECT COALESCE(e.supervisor_id, 0) FROM employees e WHERE e.id=$1`, employeeID)
	if err != nil {
		return
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil || len(ids) == 0 {
		return
	}
	if ids[0] != 0 {
		s.Notify(ctx, ids[0], kind, detail, "")
		return
	}
	s.NotifyRole(ctx, kind, detail, "")
}

func (s *Store) NotifyRole(ctx context.Context, kind, detail, dedupe string) {
	rows, err := s.pool.Query(ctx, `SELECT id FROM employees WHERE active AND role IN ('hr_admin','super_admin')`)
	if err != nil {
		return
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return
	}
	for _, id := range ids {
		s.Notify(ctx, id, kind, detail, dedupe)
	}
}

type Notification struct {
	ID        int64      `json:"id"`
	Kind      string     `json:"kind"`
	Detail    string     `json:"detail"`
	ReadAt    *time.Time `json:"read_at"`
	CreatedAt time.Time  `json:"created_at"`
}

func (s *Store) Notifications(ctx context.Context, emp int64) ([]Notification, int, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, kind, detail, read_at, created_at FROM notifications
		WHERE employee_id=$1 ORDER BY created_at DESC, id DESC LIMIT 60`, emp)
	if err != nil {
		return nil, 0, err
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Notification])
	if err != nil {
		return nil, 0, err
	}
	if list == nil {
		list = []Notification{}
	}
	var unread int
	err = s.pool.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE employee_id=$1 AND read_at IS NULL`, emp).Scan(&unread)
	return list, unread, err
}

func (s *Store) MarkAllRead(ctx context.Context, emp int64) error {
	_, err := s.pool.Exec(ctx, `UPDATE notifications SET read_at=now() WHERE employee_id=$1 AND read_at IS NULL`, emp)
	return err
}

// ---- Announcements

type Announcement struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	Office    string    `json:"office"`
	CreatedAt time.Time `json:"created_at"`
	Acked     bool      `json:"acked"`
	AckCount  int       `json:"ack_count"`
	Audience  int       `json:"audience"`
}

func (s *Store) Announcements(ctx context.Context, emp int64, office string) ([]Announcement, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT a.id, a.title, a.body, a.office, a.created_at,
		       EXISTS (SELECT 1 FROM announcement_acks k WHERE k.announcement_id=a.id AND k.employee_id=$1),
		       (SELECT count(*)::int FROM announcement_acks k WHERE k.announcement_id=a.id),
		       (SELECT count(*)::int FROM employees e WHERE e.active AND (a.office='' OR e.office=a.office))
		  FROM announcements a WHERE a.office='' OR a.office=$2
		 ORDER BY a.created_at DESC LIMIT 50`, emp, office)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Announcement])
	if list == nil {
		list = []Announcement{}
	}
	return list, err
}

func (s *Store) CreateAnnouncement(ctx context.Context, by int64, title, body, office string) error {
	title, body, office = strings.TrimSpace(title), strings.TrimSpace(body), strings.TrimSpace(office)
	if len(title) < 3 || len(title) > 120 || len(body) < 3 || len(body) > 4000 {
		return invalid("give a title and a message")
	}
	if _, err := s.pool.Exec(ctx, `INSERT INTO announcements (title, body, office, created_by) VALUES ($1,$2,$3,$4)`, title, body, office, by); err != nil {
		return err
	}
	rows, err := s.pool.Query(ctx, `SELECT id FROM employees WHERE active AND id<>$1 AND ($2='' OR office=$2)`, by, office)
	if err != nil {
		return err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return err
	}
	for _, id := range ids {
		s.Notify(ctx, id, "announcement", title, "")
	}
	return nil
}

func (s *Store) Ack(ctx context.Context, id, emp int64) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO announcement_acks (announcement_id, employee_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, id, emp)
	if err != nil {
		return ErrNotFound
	}
	return nil
}

// ---- Accomplishment reports

type Accomplishment struct {
	ID         int64     `json:"id"`
	PeriodFrom string    `json:"period_from"`
	PeriodTo   string    `json:"period_to"`
	Body       string    `json:"body"`
	Status     string    `json:"status"`
	Note       string    `json:"note"`
	CreatedAt  time.Time `json:"created_at"`
	Employee   string    `json:"employee"`
}

const accompCols = `a.id, a.period_from::text, a.period_to::text, a.body, a.status, a.note, a.created_at, e.full_name`

func (s *Store) CreateAccomplishment(ctx context.Context, emp int64, from, to, body string) error {
	body = strings.TrimSpace(body)
	f, err1 := time.Parse("2006-01-02", from)
	t, err2 := time.Parse("2006-01-02", to)
	if err1 != nil || err2 != nil || t.Before(f) {
		return invalid("choose a valid period")
	}
	if len(body) < 10 || len(body) > 8000 {
		return invalid("describe your work in 10 to 8,000 characters")
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO accomplishment_reports (employee_id, period_from, period_to, body) VALUES ($1,$2,$3,$4)`, emp, from, to, body)
	return err
}

func (s *Store) MyAccomplishments(ctx context.Context, emp int64) ([]Accomplishment, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+accompCols+` FROM accomplishment_reports a JOIN employees e ON e.id=a.employee_id
		WHERE a.employee_id=$1 ORDER BY a.created_at DESC LIMIT 60`, emp)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Accomplishment])
	if list == nil {
		list = []Accomplishment{}
	}
	return list, err
}

// Approver is the shared rule: your reports, reports delegated to you today, or
// everyone for HR. Nobody reviews their own work. It expects $1 actor and $2 role.
const Approver = `e.id <> $1 AND (e.supervisor_id = $1 OR $2 IN ('hr_admin','super_admin') OR e.supervisor_id IN (
	SELECT d.from_id FROM approval_delegations d WHERE d.to_id = $1
	   AND (now() AT TIME ZONE 'Asia/Manila')::date BETWEEN d.from_date AND d.to_date))`

func (s *Store) PendingAccomplishments(ctx context.Context, actor int64, role string) ([]Accomplishment, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+accompCols+` FROM accomplishment_reports a JOIN employees e ON e.id=a.employee_id
		WHERE a.status='submitted' AND `+Approver+` ORDER BY a.created_at LIMIT 200`, actor, role)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Accomplishment])
	if list == nil {
		list = []Accomplishment{}
	}
	return list, err
}

func (s *Store) ReviewAccomplishment(ctx context.Context, actor int64, role string, id int64, accept bool, note string) error {
	note = strings.TrimSpace(note)
	status := "accepted"
	if !accept {
		status = "returned"
		if len(note) < 3 {
			return invalid("say what needs correcting")
		}
	}
	var emp int64
	err := s.pool.QueryRow(ctx, `UPDATE accomplishment_reports a SET status=$3, reviewed_by=$1, reviewed_at=now(), note=$4
		FROM employees e WHERE a.id=$5 AND a.status='submitted' AND e.id=a.employee_id AND `+Approver+` RETURNING a.employee_id`,
		actor, role, status, note, id).Scan(&emp)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("%w: this report is no longer waiting for you", ErrConflict)
	}
	if err == nil && !accept {
		s.Notify(ctx, emp, "accomp_returned", note, "")
	}
	return err
}

// ---- Events

type Event struct {
	ID       int64     `json:"id"`
	Title    string    `json:"title"`
	Place    string    `json:"place"`
	Office   string    `json:"office"`
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
	Required bool      `json:"required"`
	Attended bool      `json:"attended"`
	Count    int       `json:"count"`
	Audience int       `json:"audience"`
}

func (s *Store) Events(ctx context.Context, emp int64, office string) ([]Event, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT v.id, v.title, v.place, v.office, v.starts_at, v.ends_at, v.required,
		       EXISTS (SELECT 1 FROM event_attendance x WHERE x.event_id=v.id AND x.employee_id=$1),
		       (SELECT count(*)::int FROM event_attendance x WHERE x.event_id=v.id),
		       (SELECT count(*)::int FROM employees e WHERE e.active AND (v.office='' OR e.office=v.office))
		  FROM events v WHERE (v.office='' OR v.office=$2) AND v.ends_at > now() - interval '45 days'
		 ORDER BY v.starts_at DESC LIMIT 60`, emp, office)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Event])
	if list == nil {
		list = []Event{}
	}
	return list, err
}

func (s *Store) CreateEvent(ctx context.Context, by int64, title, place, office, starts, ends string, required bool) error {
	title = strings.TrimSpace(title)
	a, err1 := time.ParseInLocation("2006-01-02T15:04", starts, s.loc)
	b, err2 := time.ParseInLocation("2006-01-02T15:04", ends, s.loc)
	if len(title) < 3 || len(title) > 120 {
		return invalid("give the event a title")
	}
	if err1 != nil || err2 != nil || !b.After(a) {
		return invalid("the event must end after it starts")
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO events (title, place, office, starts_at, ends_at, required, created_by) VALUES ($1,$2,$3,$4,$5,$6,$7)`,
		title, strings.TrimSpace(place), strings.TrimSpace(office), a, b, required, by)
	return err
}

// CanCheckIn says whether an employee may check in now: 30 minutes before start until the end.
func (s *Store) CanCheckIn(ctx context.Context, id int64, office string) error {
	var open bool
	err := s.pool.QueryRow(ctx, `SELECT now() BETWEEN starts_at - interval '30 minutes' AND ends_at
		FROM events WHERE id=$1 AND (office='' OR office=$2)`, id, office).Scan(&open)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if !open {
		return fmt.Errorf("%w: check-in is open from 30 minutes before the event until it ends", ErrConflict)
	}
	return nil
}

func (s *Store) CheckIn(ctx context.Context, id, emp int64, photoKey string) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO event_attendance (event_id, employee_id, method, photo_key) VALUES ($1,$2,'photo',$3)
		ON CONFLICT DO NOTHING`, id, emp, photoKey)
	return err
}

// Mark records attendance on someone's behalf, by employee number.
func (s *Store) Mark(ctx context.Context, id int64, employeeNo string, by int64) error {
	tag, err := s.pool.Exec(ctx, `INSERT INTO event_attendance (event_id, employee_id, method, marked_by)
		SELECT $1, e.id, 'marked', $3 FROM employees e WHERE e.employee_no=$2 AND e.active ON CONFLICT DO NOTHING`,
		id, strings.ToUpper(strings.TrimSpace(employeeNo)), by)
	if err != nil {
		return ErrNotFound
	}
	if tag.RowsAffected() == 0 {
		return invalid("that employee was not found or is already marked present")
	}
	return nil
}

type Attendee struct {
	EmployeeNo string     `json:"employee_no"`
	Name       string     `json:"name"`
	Office     string     `json:"office"`
	Method     *string    `json:"method"`
	At         *time.Time `json:"at"`
}

func (s *Store) Attendance(ctx context.Context, id int64) ([]Attendee, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT e.employee_no, e.full_name, e.office, x.method, x.at
		  FROM events v JOIN employees e ON e.active AND (v.office='' OR e.office=v.office)
		  LEFT JOIN event_attendance x ON x.event_id=v.id AND x.employee_id=e.id
		 WHERE v.id=$1 ORDER BY x.at NULLS LAST, e.full_name LIMIT 1000`, id)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Attendee])
	if list == nil {
		list = []Attendee{}
	}
	return list, err
}

// ---- Delegation

type Delegation struct {
	ID       int64  `json:"id"`
	ToName   string `json:"to_name"`
	FromDate string `json:"from_date"`
	ToDate   string `json:"to_date"`
}

func (s *Store) Delegations(ctx context.Context, emp int64) ([]Delegation, error) {
	rows, err := s.pool.Query(ctx, `SELECT d.id, e.full_name, d.from_date::text, d.to_date::text
		FROM approval_delegations d JOIN employees e ON e.id=d.to_id
		WHERE d.from_id=$1 AND d.to_date >= (now() AT TIME ZONE 'Asia/Manila')::date ORDER BY d.from_date`, emp)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Delegation])
	if list == nil {
		list = []Delegation{}
	}
	return list, err
}

func (s *Store) Delegate(ctx context.Context, emp int64, toNo, from, to string) error {
	f, err1 := time.Parse("2006-01-02", from)
	t, err2 := time.Parse("2006-01-02", to)
	if err1 != nil || err2 != nil || t.Before(f) {
		return invalid("choose a valid period")
	}
	tag, err := s.pool.Exec(ctx, `INSERT INTO approval_delegations (from_id, to_id, from_date, to_date)
		SELECT $1, e.id, $3, $4 FROM employees e
		 WHERE e.employee_no=$2 AND e.active AND e.id<>$1 AND e.role IN ('supervisor','hr_admin','super_admin')`,
		emp, strings.ToUpper(strings.TrimSpace(toNo)), from, to)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return invalid("enter the employee number of another supervisor")
	}
	return nil
}

func (s *Store) DeleteDelegation(ctx context.Context, id, emp int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM approval_delegations WHERE id=$1 AND from_id=$2`, id, emp)
	return err
}

// ---- Logbook

// Entry is one log typed in from the paper logbook during an outage.
type Entry struct {
	Slot string `json:"slot"`
	Time string `json:"time"`
}

// AddLogbook records paper entries as approved corrections, marked as logbook
// entries, so raw logs stay untouched and the source stays visible.
func (s *Store) AddLogbook(ctx context.Context, actor int64, employeeNo, day, reason string, entries []Entry) (int, error) {
	reason = strings.TrimSpace(reason)
	if _, err := time.Parse("2006-01-02", day); err != nil {
		return 0, invalid("choose a date")
	}
	if len(reason) < 5 || len(reason) > 500 {
		return 0, invalid("say why the paper logbook was used")
	}
	if len(entries) == 0 {
		return 0, invalid("enter at least one time")
	}
	var emp int64
	err := s.pool.QueryRow(ctx, `SELECT id FROM employees WHERE employee_no=$1 AND active`, strings.ToUpper(strings.TrimSpace(employeeNo))).Scan(&emp)
	if err != nil {
		return 0, invalid("that employee number was not found")
	}
	if emp == actor {
		return 0, invalid("someone else must encode your own logbook entries")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	n := 0
	for _, e := range entries {
		if _, err := time.Parse("15:04", e.Time); err != nil {
			return 0, invalid("enter valid times")
		}
		tag, err := tx.Exec(ctx, `
			INSERT INTO requests (employee_id, kind, work_date, slot, requested_time, reason, status, decided_by, decided_at, decision_note, source)
			SELECT $1, 'missed_log', $2, $3::log_slot, $4::time, $5, 'approved', $6, now(), 'Encoded from the paper logbook', 'logbook'
			 WHERE NOT EXISTS (SELECT 1 FROM attendance_logs l WHERE l.employee_id=$1 AND l.work_date=$2 AND l.slot=$3::log_slot)
			ON CONFLICT DO NOTHING`, emp, day, e.Slot, e.Time, reason, actor)
		if err != nil {
			return 0, invalid("check the log type and times")
		}
		n += int(tag.RowsAffected())
	}
	return n, tx.Commit(ctx)
}

// ---- Reminders

// Escalate tells HR about requests left waiting too long, once each.
func (s *Store) Escalate(ctx context.Context, days int) error {
	rows, err := s.pool.Query(ctx, `UPDATE requests SET escalated_at=now()
		WHERE status='pending' AND escalated_at IS NULL AND created_at < now() - ($1 * interval '1 day') RETURNING id`, days)
	if err != nil {
		return err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return err
	}
	for _, id := range ids {
		s.NotifyRole(ctx, "approval_overdue", fmt.Sprintf("Request #%d", id), fmt.Sprintf("overdue:%d", id))
	}
	return nil
}
