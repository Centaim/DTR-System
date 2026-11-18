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
	"dtr/server/internal/photocheck"
	"dtr/server/internal/settings"
	"dtr/server/internal/storage"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrDayComplete = errors.New("all logs for today are already recorded")
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
	WorkDate string   `json:"work_date"`
	Logs     []Log    `json:"logs"`
	NextSlot *string  `json:"next_slot"`
	Slots    []string `json:"slots"`
	Now      string   `json:"server_time"`
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

// FlagRules are the thresholds for anomaly flags. Flags only inform HR; they
// never change a record or penalize anyone.
type FlagRules struct {
	DarkLuma        float64       // mean brightness below this is a dark photo
	SimilarDistance int           // dHash bits at or below this count as the same photo
	SkewTolerance   time.Duration // online logs: device clock vs server
	LateSync        time.Duration // offline logs: delay before reaching the server
	SharedDevice    time.Duration // another employee used this device within this window
	ReuseLookback   time.Duration
}

func DefaultFlagRules() FlagRules {
	return FlagRules{
		DarkLuma: 45, SimilarDistance: 2, SkewTolerance: 5 * time.Minute,
		LateSync: 2 * time.Hour, SharedDevice: 10 * time.Minute, ReuseLookback: 60 * 24 * time.Hour,
	}
}

type Service struct {
	pool     *pgxpool.Pool
	store    storage.Store
	settings *settings.Store
	hours    HoursSource
	loc      *time.Location
}

// HoursSource gives the rules in force for an employee on any date.
type HoursSource interface {
	HoursFunc(ctx context.Context, employeeID int64) (func(time.Time) dtr.Hours, error)
}

func NewService(pool *pgxpool.Pool, store storage.Store, s *settings.Store, hours HoursSource, loc *time.Location) *Service {
	return &Service{pool: pool, store: store, settings: s, hours: hours, loc: loc}
}

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// workDay decides which calendar day a log belongs to. A night shift that began
// yesterday and is still open keeps collecting logs on yesterday's record.
func (s *Service) workDay(ctx context.Context, q querier, employeeID int64) (time.Time, dtr.Hours, error) {
	hf, err := s.hours.HoursFunc(ctx, employeeID)
	if err != nil {
		return time.Time{}, dtr.Hours{}, err
	}
	now := time.Now().In(s.loc)
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.loc)
	yesterday := today.AddDate(0, 0, -1)
	if prev := hf(yesterday); prev.Overnight {
		var n int
		var last *time.Time
		if err := q.QueryRow(ctx, `SELECT count(*), max(recorded_at) FROM attendance_logs WHERE employee_id=$1 AND work_date=$2`,
			employeeID, yesterday.Format("2006-01-02")).Scan(&n, &last); err != nil {
			return time.Time{}, dtr.Hours{}, err
		}
		if n > 0 && n < len(prev.Slots()) && last != nil && time.Since(*last) < 18*time.Hour {
			return yesterday, prev, nil
		}
	}
	return today, hf(today), nil
}

func (s *Service) minGap() time.Duration {
	return time.Duration(s.settings.Int(settings.MinGapSeconds)) * time.Second
}

// flagRules reads the agency's current thresholds.
func (s *Service) flagRules() FlagRules {
	r := DefaultFlagRules()
	r.DarkLuma = float64(s.settings.Int(settings.FlagDarkLuma))
	r.SimilarDistance = s.settings.Int(settings.FlagSimilarBits)
	r.SkewTolerance = time.Duration(s.settings.Int(settings.FlagSkewMinutes)) * time.Minute
	r.LateSync = time.Duration(s.settings.Int(settings.FlagLateSyncMins)) * time.Minute
	r.SharedDevice = time.Duration(s.settings.Int(settings.FlagSharedDevMins)) * time.Minute
	return r
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
	pc := photocheck.Analyze(in.Photo)

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

	day, hours, err := s.workDay(ctx, tx, employeeID)
	if err != nil {
		return Log{}, false, err
	}
	workDate := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	slots := hours.Slots()
	var count int
	var lastAt *time.Time
	if err := tx.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM attendance_logs WHERE employee_id = $1 AND work_date = $2),
		       (SELECT max(recorded_at) FROM attendance_logs WHERE employee_id = $1)`,
		employeeID, workDate.Format("2006-01-02")).Scan(&count, &lastAt); err != nil {
		return Log{}, false, err
	}
	if count >= len(slots) {
		return Log{}, false, ErrDayComplete
	}
	if lastAt != nil && time.Since(*lastAt) < s.minGap() {
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
		   latitude, longitude, offline, idempotency_key, photo_key, photo_sha256, photo_dhash)
		VALUES ($1,$2,$3::log_slot,$4,$5::log_channel,$6,$7,$8,$9,$10,$11,$12,$13)
		RETURNING `+logCols,
		employeeID, workDate, slots[count], in.ClientTime, channel, in.DeviceID,
		in.Latitude, in.Longitude, in.Offline, in.IdempotencyKey, key, sum, dhashArg(pc)))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Log{}, false, ErrTooSoon
		}
		return Log{}, false, err
	}
	if err := s.flag(ctx, tx, employeeID, l, in, channel, pc, sum); err != nil {
		return Log{}, false, fmt.Errorf("flag log: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Log{}, false, err
	}
	return l, false, nil
}

func dhashArg(pc photocheck.Result) any {
	if !pc.Decoded {
		return nil
	}
	return int64(pc.DHash)
}

// flag records anomaly flags for a new log, in the same transaction as the log.
func (s *Service) flag(ctx context.Context, tx pgx.Tx, employeeID int64, l Log, in ClockInput, channel string, pc photocheck.Result, sha string) error {
	rules := s.flagRules()
	type finding struct{ kind, detail string }
	var found []finding

	if pc.Decoded && pc.Luma < rules.DarkLuma {
		found = append(found, finding{"dark_photo", fmt.Sprintf("Average brightness %.0f of 255", pc.Luma)})
	}

	rows, err := tx.Query(ctx, `
		SELECT id, photo_sha256, photo_dhash FROM attendance_logs
		 WHERE employee_id = $1 AND id <> $2 AND recorded_at > now() - ($3 * interval '1 second')
		 ORDER BY recorded_at DESC LIMIT 300`,
		employeeID, l.ID, int64(rules.ReuseLookback/time.Second))
	if err != nil {
		return err
	}
	var identical, similar string
	for rows.Next() {
		var id int64
		var prevSum string
		var prevHash *int64
		if err := rows.Scan(&id, &prevSum, &prevHash); err != nil {
			rows.Close()
			return err
		}
		if prevSum == sha && identical == "" {
			identical = fmt.Sprintf("Identical to log #%d", id)
		} else if pc.Decoded && photocheck.Informative(pc.DHash) && prevHash != nil && photocheck.Informative(uint64(*prevHash)) && similar == "" {
			if d := photocheck.Distance(pc.DHash, uint64(*prevHash)); d <= rules.SimilarDistance {
				similar = fmt.Sprintf("Nearly identical to log #%d (%d of 64 bits differ)", id, d)
			}
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if identical != "" {
		found = append(found, finding{"reused_photo", identical})
	} else if similar != "" {
		found = append(found, finding{"similar_photo", similar})
	}

	if in.DeviceID != "" && channel == "mobile" {
		var no string
		var secs int
		err := tx.QueryRow(ctx, `
			SELECT e.employee_no, extract(epoch FROM now() - l.recorded_at)::int
			  FROM attendance_logs l JOIN employees e ON e.id = l.employee_id
			 WHERE l.device_id = $1 AND l.employee_id <> $2 AND l.channel = 'mobile'
			   AND l.recorded_at > now() - ($3 * interval '1 second')
			 ORDER BY l.recorded_at DESC LIMIT 1`,
			in.DeviceID, employeeID, int64(rules.SharedDevice/time.Second)).Scan(&no, &secs)
		if err == nil {
			found = append(found, finding{"shared_device", fmt.Sprintf("Same device used by %s %d min earlier", no, secs/60)})
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
	}

	if in.ClientTime != nil {
		diff := l.RecordedAt.Sub(*in.ClientTime)
		if !in.Offline && (diff > rules.SkewTolerance || -diff > rules.SkewTolerance) {
			found = append(found, finding{"clock_skew", fmt.Sprintf("Device clock differs from the server by %d min", int(diff.Abs().Minutes()))})
		}
		if in.Offline && diff > rules.LateSync {
			found = append(found, finding{"late_sync", fmt.Sprintf("Saved offline and reached the server %d min later", int(diff.Minutes()))})
		}
	}

	for _, f := range found {
		if _, err := tx.Exec(ctx, `INSERT INTO attendance_flags (log_id, kind, detail)
			VALUES ($1, $2::flag_kind, $3) ON CONFLICT DO NOTHING`, l.ID, f.kind, f.detail); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) Today(ctx context.Context, employeeID int64) (Today, error) {
	workDay, hours, err := s.workDay(ctx, s.pool, employeeID)
	if err != nil {
		return Today{}, err
	}
	day := workDay.Format("2006-01-02")
	now := time.Now()
	rows, err := s.pool.Query(ctx,
		`SELECT `+logCols+` FROM attendance_logs
		  WHERE employee_id = $1 AND work_date = $2 ORDER BY recorded_at`, employeeID, day)
	if err != nil {
		return Today{}, err
	}
	defer rows.Close()

	slots := hours.Slots()
	t := Today{WorkDate: day, Logs: []Log{}, Slots: slots, Now: now.UTC().Format(time.RFC3339)}
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
	if len(t.Logs) < len(slots) {
		next := slots[len(t.Logs)]
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
		SELECT work_date, slot::text, recorded_at FROM attendance_logs
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
		var work, at time.Time
		if err := rows.Scan(&work, &slot, &at); err != nil {
			return nil, err
		}
		out = append(out, dtr.Log{Day: work.Day(), Slot: slot, At: at.In(loc)})
	}
	return out, rows.Err()
}
