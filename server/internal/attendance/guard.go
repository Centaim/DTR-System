package attendance

import (
	"context"
	"errors"
	"fmt"
	"time"

	"dtr/server/internal/geo"
	"dtr/server/internal/settings"

	"github.com/jackc/pgx/v5"
)

var (
	ErrChannel       = errors.New("this clock-in channel is not allowed for this employee")
	ErrDevice        = errors.New("this device is waiting for HR approval")
	ErrOutOfArea     = errors.New("outside the allowed area")
	ErrNeedsLocation = errors.New("location is required")
)

type finding struct{ kind, detail string }

const (
	modeOff = iota
	modeFlag
	modeRequire
)

// guard applies the agency's channel, device and location rules. Depending on
// the setting a problem either stops the clock-in or is flagged for HR. A log
// made offline already happened, so it is only ever flagged.
func (s *Service) guard(ctx context.Context, tx pgx.Tx, employeeID int64, in ClockInput, channel string) ([]finding, error) {
	var office string
	var channels int
	if err := tx.QueryRow(ctx, `SELECT office, clock_channels FROM employees WHERE id=$1`, employeeID).Scan(&office, &channels); err != nil {
		return nil, err
	}
	bit := 1
	if channel == "kiosk" {
		bit = 2
	}
	if channels&bit == 0 {
		return nil, ErrChannel
	}
	if channel != "mobile" {
		return nil, nil // a kiosk is a registered device in a known place
	}
	var out []finding
	f, err := s.checkDevice(ctx, tx, employeeID, in)
	if err != nil {
		return nil, err
	}
	out = append(out, f...)
	f, err = s.checkLocation(ctx, tx, employeeID, office, in)
	if err != nil {
		return nil, err
	}
	return append(out, f...), nil
}

func (s *Service) checkDevice(ctx context.Context, tx pgx.Tx, employeeID int64, in ClockInput) ([]finding, error) {
	mode := s.settings.Int(settings.DeviceBindingMode)
	if mode == modeOff || in.DeviceID == "" {
		return nil, nil
	}
	var bound string
	err := tx.QueryRow(ctx, `SELECT device_id FROM device_bindings WHERE employee_id=$1 AND status='approved'`, employeeID).Scan(&bound)
	if errors.Is(err, pgx.ErrNoRows) {
		_, err = tx.Exec(ctx, `INSERT INTO device_bindings (employee_id, device_id, status) VALUES ($1,$2,'approved')`, employeeID, in.DeviceID)
		return nil, err
	}
	if err != nil || bound == in.DeviceID {
		return nil, err
	}
	// Written outside the transaction, so the request survives a refused clock-in.
	if _, err := s.pool.Exec(ctx, `INSERT INTO device_bindings (employee_id, device_id, status) VALUES ($1,$2,'pending')
		ON CONFLICT DO NOTHING`, employeeID, in.DeviceID); err != nil {
		return nil, err
	}
	if mode == modeRequire && !in.Offline {
		return nil, ErrDevice
	}
	return []finding{{"unapproved_device", "Clocked in from a device HR has not approved"}}, nil
}

func (s *Service) checkLocation(ctx context.Context, tx pgx.Tx, employeeID int64, office string, in ClockInput) ([]finding, error) {
	mode := s.settings.Int(settings.GeofenceMode)
	if mode == modeOff {
		return nil, nil
	}
	rows, err := tx.Query(ctx, `SELECT latitude, longitude, radius_m FROM geofences WHERE office='' OR office=$1`, office)
	if err != nil {
		return nil, err
	}
	type fence struct {
		lat, lng float64
		r        int
	}
	fences, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (fence, error) {
		var f fence
		return f, r.Scan(&f.lat, &f.lng, &f.r)
	})
	if err != nil || len(fences) == 0 {
		return nil, err
	}
	// Approved official business or travel means the employee is meant to be elsewhere.
	today := time.Now().In(s.loc).Format("2006-01-02")
	var away bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM requests WHERE employee_id=$1 AND status='approved'
		AND kind IN ('official_business','travel_order') AND work_date <= $2 AND COALESCE(date_to, work_date) >= $2)`,
		employeeID, today).Scan(&away); err != nil || away {
		return nil, err
	}
	hard := mode == modeRequire && !in.Offline
	if in.Latitude == nil || in.Longitude == nil {
		if hard {
			return nil, ErrNeedsLocation
		}
		return []finding{{"no_location", "No location was received with this log"}}, nil
	}
	nearest := -1.0
	for _, f := range fences {
		d := geo.DistanceM(*in.Latitude, *in.Longitude, f.lat, f.lng)
		if d <= float64(f.r) {
			return nil, nil
		}
		if gap := d - float64(f.r); nearest < 0 || gap < nearest {
			nearest = gap
		}
	}
	if hard {
		return nil, ErrOutOfArea
	}
	return []finding{{"out_of_area", fmt.Sprintf("%.0f m outside the nearest allowed area", nearest)}}, nil
}
