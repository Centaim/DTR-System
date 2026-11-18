package api

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"dtr/server/internal/auth"
	"dtr/server/internal/settings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgconn"
)

func (a *API) governanceRoutes(admin fiber.Router) {
	admin.Get("/settings", a.listSettings)
	admin.Put("/settings", a.requireRole(roleSuper), a.saveSettings)
	admin.Get("/legal-holds", a.listHolds)
	admin.Post("/legal-holds", a.createHold)
	admin.Post("/legal-holds/:id/release", a.releaseHold)
}

func (a *API) listSettings(c *fiber.Ctx) error { return c.JSON(a.Settings.List()) }

func (a *API) saveSettings(c *fiber.Ctx) error {
	var in map[string]int
	if err := c.BodyParser(&in); err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "The settings could not be read.")
	}
	for k, v := range in {
		old, err := a.Settings.Set(c.Context(), k, v, actorID(c))
		if errors.Is(err, settings.ErrInvalid) {
			return apiError(c, fiber.StatusUnprocessableEntity, "invalid", strings.TrimPrefix(err.Error(), "invalid setting: "))
		}
		if err != nil {
			return err
		}
		if old != v {
			a.audit(c, "setting.change", k+": "+strconv.Itoa(old)+" to "+strconv.Itoa(v))
		}
	}
	return c.JSON(a.Settings.List())
}

func (a *API) listHolds(c *fiber.Ctx) error {
	rows, err := a.Pool.Query(c.Context(), `
		SELECT h.id, e.employee_no, e.full_name, h.reason, h.created_at::text
		  FROM legal_holds h JOIN employees e ON e.id = h.employee_id
		 WHERE h.released_at IS NULL ORDER BY h.created_at DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type hold struct {
		ID         int64  `json:"id"`
		EmployeeNo string `json:"employee_no"`
		Name       string `json:"name"`
		Reason     string `json:"reason"`
		CreatedAt  string `json:"created_at"`
	}
	out := []hold{}
	for rows.Next() {
		var h hold
		if err := rows.Scan(&h.ID, &h.EmployeeNo, &h.Name, &h.Reason, &h.CreatedAt); err != nil {
			return err
		}
		out = append(out, h)
	}
	return c.JSON(out)
}

func (a *API) createHold(c *fiber.Ctx) error {
	var in struct {
		EmployeeNo string `json:"employee_no"`
		Reason     string `json:"reason"`
	}
	if err := c.BodyParser(&in); err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "The form could not be read.")
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if len(in.Reason) < 5 || len(in.Reason) > 300 {
		return apiError(c, fiber.StatusUnprocessableEntity, "invalid", "Give a reason of 5 to 300 characters.")
	}
	tag, err := a.Pool.Exec(c.Context(), `
		INSERT INTO legal_holds (employee_id, reason, created_by)
		SELECT id, $2, $3 FROM employees WHERE employee_no = $1`,
		strings.ToUpper(strings.TrimSpace(in.EmployeeNo)), in.Reason, actorID(c))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return apiError(c, fiber.StatusConflict, "duplicate", "This employee is already on legal hold.")
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apiError(c, fiber.StatusNotFound, "not_found", "That employee number was not found.")
	}
	a.audit(c, "legal_hold.create", "employee:"+in.EmployeeNo)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"ok": true})
}

func (a *API) releaseHold(c *fiber.Ctx) error {
	tag, err := a.Pool.Exec(c.Context(), `UPDATE legal_holds SET released_at = now(), released_by = $2
		WHERE id = $1 AND released_at IS NULL`, c.Params("id"), actorID(c))
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "Invalid legal hold.")
	}
	if tag.RowsAffected() == 0 {
		return apiError(c, fiber.StatusNotFound, "not_found", "That legal hold is not active.")
	}
	a.audit(c, "legal_hold.release", "hold:"+c.Params("id"))
	return c.JSON(fiber.Map{"ok": true})
}

func (a *API) privacyAccepted(ctx context.Context, employeeID int64) (bool, error) {
	var ok bool
	err := a.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM privacy_acceptances WHERE employee_id=$1 AND version=$2)`,
		employeeID, a.Settings.Int(settings.PrivacyVersion)).Scan(&ok)
	return ok, err
}

func (a *API) recordPrivacy(ctx context.Context, employeeID int64, channel string) error {
	_, err := a.Pool.Exec(ctx, `INSERT INTO privacy_acceptances (employee_id, version, channel)
		VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`, employeeID, a.Settings.Int(settings.PrivacyVersion), channel)
	return err
}

func (a *API) acceptPrivacy(c *fiber.Ctx) error {
	if err := a.recordPrivacy(c.Context(), actorID(c), "app"); err != nil {
		return err
	}
	a.audit(c, "privacy.accept", "version:"+strconv.Itoa(a.Settings.Int(settings.PrivacyVersion)))
	return c.JSON(fiber.Map{"ok": true})
}

// kioskPrivacy records acceptance for someone who verified their PIN at this kiosk.
func (a *API) kioskPrivacy(c *fiber.Ctx) error {
	empID, kioskID, err := auth.ParseKioskToken(a.JWTSecret, c.FormValue("session"))
	if err != nil || kioskID != kioskOf(c).ID {
		return apiError(c, fiber.StatusUnauthorized, "session_expired", "That took too long. Start again.")
	}
	if err := a.recordPrivacy(c.Context(), empID, "kiosk"); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"ok": true})
}
