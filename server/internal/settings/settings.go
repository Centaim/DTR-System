// Package settings holds the numbers each agency may set for itself.
package settings

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	DTREarlySubmit       = "dtr_allow_early_submit"
	ApprovalEscalateDays = "approval_escalate_days"
	DTRDeadlineDay       = "dtr_deadline_day"
	MFARequired          = "mfa_required_admins"
	SessionDays          = "session_days"
	GeofenceMode         = "geofence_mode"
	DeviceBindingMode    = "device_binding_mode"
	PhotoRetentionDays   = "photo_retention_days"
	PrivacyVersion       = "privacy_notice_version"
	MinGapSeconds        = "min_gap_seconds"
	RequestWindowDays    = "request_window_days"
	HabitualLateDays     = "habitual_late_days"
	PINMaxAttempts       = "pin_max_attempts"
	PINLockMinutes       = "pin_lock_minutes"
	FlagDarkLuma         = "flag_dark_luma"
	FlagBlurThreshold    = "flag_blur_threshold"
	FlagCorrectionLimit  = "flag_correction_limit"
	FlagSimilarBits      = "flag_similar_bits"
	FlagSkewMinutes      = "flag_skew_minutes"
	FlagLateSyncMins     = "flag_late_sync_minutes"
	FlagSharedDevMins    = "flag_shared_device_minutes"
)

type Definition struct {
	Key     string `json:"key"`
	Group   string `json:"group"`
	Default int    `json:"default"`
	Min     int    `json:"min"`
	Max     int    `json:"max"`
}

// Definitions lists every setting with its allowed range.
func Definitions(defaults map[string]int) []Definition {
	defs := []Definition{
		{MinGapSeconds, "attendance", 60, 0, 3600},
		{RequestWindowDays, "attendance", 30, 1, 365},
		{HabitualLateDays, "attendance", 10, 1, 31},
		{DTREarlySubmit, "attendance", 0, 0, 1},
		{DTRDeadlineDay, "attendance", 5, 1, 28},
		{ApprovalEscalateDays, "attendance", 3, 1, 30},
		{PINMaxAttempts, "security", 5, 3, 20},
		{PINLockMinutes, "security", 10, 1, 1440},
		{MFARequired, "security", 1, 0, 1},
		{SessionDays, "security", 14, 1, 90},
		{PhotoRetentionDays, "privacy", 0, 0, 3650},
		{PrivacyVersion, "privacy", 1, 1, 1000},
		{GeofenceMode, "mobile", 0, 0, 2},
		{DeviceBindingMode, "mobile", 1, 0, 2},
		{FlagDarkLuma, "flags", 45, 0, 255},
		{FlagBlurThreshold, "flags", 20, 0, 500},
		{FlagCorrectionLimit, "flags", 5, 1, 31},
		{FlagSimilarBits, "flags", 2, 0, 16},
		{FlagSkewMinutes, "flags", 5, 1, 1440},
		{FlagLateSyncMins, "flags", 120, 1, 10080},
		{FlagSharedDevMins, "flags", 10, 1, 1440},
	}
	for i := range defs {
		if v, ok := defaults[defs[i].Key]; ok {
			defs[i].Default = v
		}
	}
	return defs
}

var ErrInvalid = errors.New("invalid setting")

type Value struct {
	Definition
	Value int `json:"value"`
}

type Store struct {
	pool *pgxpool.Pool
	defs map[string]Definition
	list []Definition

	mu     sync.RWMutex
	values map[string]int
	loaded time.Time
}

// New builds the store. defaults come from the environment on a fresh install.
func New(ctx context.Context, pool *pgxpool.Pool, defaults map[string]int) (*Store, error) {
	s := &Store{pool: pool, defs: map[string]Definition{}, list: Definitions(defaults)}
	for _, d := range s.list {
		s.defs[d.Key] = d
	}
	return s, s.reload(ctx)
}

func (s *Store) reload(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return err
	}
	defer rows.Close()
	vals := map[string]int{}
	for rows.Next() {
		var k string
		var v int
		if err := rows.Scan(&k, &v); err != nil {
			return err
		}
		vals[k] = v
	}
	if err := rows.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	s.values, s.loaded = vals, time.Now()
	s.mu.Unlock()
	return nil
}

// Int returns the current value, re-reading the database at most every 30 seconds.
func (s *Store) Int(key string) int {
	s.mu.RLock()
	stale := time.Since(s.loaded) > 30*time.Second
	s.mu.RUnlock()
	if stale {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = s.reload(ctx) // on failure keep the last known values
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if v, ok := s.values[key]; ok {
		return v
	}
	return s.defs[key].Default
}

func (s *Store) List() []Value {
	out := make([]Value, 0, len(s.list))
	for _, d := range s.list {
		out = append(out, Value{Definition: d, Value: s.Int(d.Key)})
	}
	return out
}

// Set validates and stores one value, returning the previous value.
func (s *Store) Set(ctx context.Context, key string, value int, by int64) (int, error) {
	d, ok := s.defs[key]
	if !ok {
		return 0, fmt.Errorf("%w: unknown setting", ErrInvalid)
	}
	if value < d.Min || value > d.Max {
		return 0, fmt.Errorf("%w: %s must be from %d to %d", ErrInvalid, key, d.Min, d.Max)
	}
	old := s.Int(key)
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO settings (key, value, updated_by) VALUES ($1,$2,$3)
		ON CONFLICT (key) DO UPDATE SET value=$2, updated_by=$3, updated_at=now()`, key, value, by); err != nil {
		return 0, err
	}
	return old, s.reload(ctx)
}
