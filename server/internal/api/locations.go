package api

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (a *API) locationRoutes(admin fiber.Router) {
	admin.Get("/geofences", a.listGeofences)
	admin.Post("/geofences", a.createGeofence)
	admin.Delete("/geofences/:id", a.deleteGeofence)
	admin.Get("/device-requests", a.listDeviceRequests)
	admin.Post("/device-requests/:id/approve", a.decideDevice(true))
	admin.Post("/device-requests/:id/reject", a.decideDevice(false))
}

type geofence struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	RadiusM   int     `json:"radius_m"`
	Office    string  `json:"office"`
}

func (a *API) listGeofences(c *fiber.Ctx) error {
	rows, err := a.Pool.Query(c.Context(), `SELECT id, name, latitude, longitude, radius_m, office FROM geofences ORDER BY name`)
	if err != nil {
		return err
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[geofence])
	if err != nil {
		return err
	}
	if list == nil {
		list = []geofence{}
	}
	return c.JSON(list)
}

func (a *API) createGeofence(c *fiber.Ctx) error {
	var in geofence
	if err := c.BodyParser(&in); err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "The form could not be read.")
	}
	in.Name, in.Office = strings.TrimSpace(in.Name), strings.TrimSpace(in.Office)
	if in.Name == "" || len(in.Name) > 80 || in.Latitude < -90 || in.Latitude > 90 || in.Longitude < -180 || in.Longitude > 180 ||
		in.RadiusM < 20 || in.RadiusM > 20000 {
		return apiError(c, fiber.StatusUnprocessableEntity, "invalid", "Enter a name, a valid location and a radius of 20 to 20,000 metres.")
	}
	_, err := a.Pool.Exec(c.Context(), `INSERT INTO geofences (name, latitude, longitude, radius_m, office) VALUES ($1,$2,$3,$4,$5)`,
		in.Name, in.Latitude, in.Longitude, in.RadiusM, in.Office)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return apiError(c, fiber.StatusConflict, "duplicate", "A location with that name already exists.")
	}
	if err != nil {
		return err
	}
	a.audit(c, "geofence.create", in.Name)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"ok": true})
}

func (a *API) deleteGeofence(c *fiber.Ctx) error {
	tag, err := a.Pool.Exec(c.Context(), `DELETE FROM geofences WHERE id=$1`, c.Params("id"))
	if err != nil || tag.RowsAffected() == 0 {
		return apiError(c, fiber.StatusNotFound, "not_found", "That location was not found.")
	}
	a.audit(c, "geofence.delete", "geofence:"+c.Params("id"))
	return c.JSON(fiber.Map{"ok": true})
}

func (a *API) listDeviceRequests(c *fiber.Ctx) error {
	rows, err := a.Pool.Query(c.Context(), `
		SELECT d.id, e.employee_no, e.full_name, d.device_id, d.requested_at
		  FROM device_bindings d JOIN employees e ON e.id=d.employee_id
		 WHERE d.status='pending' ORDER BY d.requested_at`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type row struct {
		ID          int64     `json:"id"`
		EmployeeNo  string    `json:"employee_no"`
		Name        string    `json:"name"`
		DeviceID    string    `json:"device_id"`
		RequestedAt time.Time `json:"requested_at"`
	}
	out := []row{}
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.ID, &r.EmployeeNo, &r.Name, &r.DeviceID, &r.RequestedAt); err != nil {
			return err
		}
		out = append(out, r)
	}
	return c.JSON(out)
}

func (a *API) approveDevice(ctx context.Context, id string, actor int64) (bool, error) {
	tx, err := a.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	var emp int64
	if err := tx.QueryRow(ctx, `SELECT employee_id FROM device_bindings WHERE id=$1 AND status='pending' FOR UPDATE`, id).Scan(&emp); err != nil {
		return false, nil
	}
	if _, err := tx.Exec(ctx, `UPDATE device_bindings SET status='revoked', decided_by=$2, decided_at=now() WHERE employee_id=$1 AND status='approved'`, emp, actor); err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `UPDATE device_bindings SET status='approved', decided_by=$2, decided_at=now() WHERE id=$1`, id, actor); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	a.Engage.Notify(ctx, emp, "device_approved", "", "")
	return true, nil
}

func (a *API) decideDevice(approve bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var ok bool
		var err error
		if approve {
			ok, err = a.approveDevice(c.Context(), c.Params("id"), actorID(c))
		} else {
			tag, e := a.Pool.Exec(c.Context(), `UPDATE device_bindings SET status='rejected', decided_by=$2, decided_at=now() WHERE id=$1 AND status='pending'`, c.Params("id"), actorID(c))
			ok, err = tag.RowsAffected() > 0, e
		}
		if err != nil {
			return err
		}
		if !ok {
			return apiError(c, fiber.StatusConflict, "conflict", "This request is no longer pending.")
		}
		action := "device.reject"
		if approve {
			action = "device.approve"
		}
		a.audit(c, action, "device_request:"+c.Params("id"))
		return c.JSON(fiber.Map{"ok": true})
	}
}
