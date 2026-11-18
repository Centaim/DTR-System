// Package api wires the HTTP routes.
package api

import (
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"dtr/server/internal/attendance"
	"dtr/server/internal/auth"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type API struct {
	Pool       *pgxpool.Pool
	Attendance *attendance.Service
	JWTSecret  []byte
	MaxPhotoMB int
}

type employee struct {
	ID         int64  `json:"id"`
	EmployeeNo string `json:"employee_no"`
	FullName   string `json:"full_name"`
	Position   string `json:"position"`
	Office     string `json:"office"`
}

func apiError(c *fiber.Ctx, status int, code, msg string) error {
	return c.Status(status).JSON(fiber.Map{"error": fiber.Map{"code": code, "message": msg}})
}

func (a *API) Routes(app *fiber.App) {
	app.Get("/api/health", func(c *fiber.Ctx) error {
		if err := a.Pool.Ping(c.Context()); err != nil {
			return apiError(c, fiber.StatusServiceUnavailable, "db_down", "database unavailable")
		}
		return c.JSON(fiber.Map{"status": "ok"})
	})

	app.Post("/api/auth/login", limiter.New(limiter.Config{
		Max:        10,
		Expiration: time.Minute,
		LimitReached: func(c *fiber.Ctx) error {
			return apiError(c, fiber.StatusTooManyRequests, "rate_limited", "Too many attempts. Wait a minute and try again.")
		},
	}), a.login)

	secured := app.Group("/api", a.requireAuth)
	secured.Get("/me", a.me)
	secured.Get("/attendance/today", a.today)
	secured.Post("/attendance/clock", a.clock)
}

// dummyHash makes unknown employee numbers cost the same as wrong PINs.
var dummyHash, _ = auth.HashPIN("not-a-real-pin")

func (a *API) login(c *fiber.Ctx) error {
	var body struct {
		EmployeeNo string `json:"employee_no"`
		PIN        string `json:"pin"`
	}
	if err := c.BodyParser(&body); err != nil || body.EmployeeNo == "" || body.PIN == "" {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "Enter your employee number and PIN.")
	}

	var e employee
	var hash string
	var active bool
	err := a.Pool.QueryRow(c.Context(),
		`SELECT id, employee_no, full_name, position, office, pin_hash, active
		   FROM employees WHERE employee_no = $1`, strings.TrimSpace(body.EmployeeNo)).
		Scan(&e.ID, &e.EmployeeNo, &e.FullName, &e.Position, &e.Office, &hash, &active)
	if errors.Is(err, pgx.ErrNoRows) {
		auth.VerifyPIN(body.PIN, dummyHash)
		return apiError(c, fiber.StatusUnauthorized, "invalid_credentials", "Employee number or PIN is incorrect.")
	}
	if err != nil {
		return err
	}
	if !auth.VerifyPIN(body.PIN, hash) || !active {
		return apiError(c, fiber.StatusUnauthorized, "invalid_credentials", "Employee number or PIN is incorrect.")
	}

	token, err := auth.IssueToken(a.JWTSecret, e.ID)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"token": token, "employee": e})
}

func (a *API) requireAuth(c *fiber.Ctx) error {
	h := c.Get("Authorization")
	token, ok := strings.CutPrefix(h, "Bearer ")
	if !ok {
		return apiError(c, fiber.StatusUnauthorized, "unauthorized", "Sign in to continue.")
	}
	id, err := auth.ParseToken(a.JWTSecret, token)
	if err != nil {
		return apiError(c, fiber.StatusUnauthorized, "unauthorized", "Your session has ended. Sign in again.")
	}
	c.Locals("employee_id", id)
	return c.Next()
}

func (a *API) me(c *fiber.Ctx) error {
	var e employee
	err := a.Pool.QueryRow(c.Context(),
		`SELECT id, employee_no, full_name, position, office FROM employees WHERE id = $1 AND active`,
		c.Locals("employee_id")).Scan(&e.ID, &e.EmployeeNo, &e.FullName, &e.Position, &e.Office)
	if errors.Is(err, pgx.ErrNoRows) {
		return apiError(c, fiber.StatusUnauthorized, "unauthorized", "This account is no longer active.")
	}
	if err != nil {
		return err
	}
	return c.JSON(e)
}

func (a *API) today(c *fiber.Ctx) error {
	t, err := a.Attendance.Today(c.Context(), c.Locals("employee_id").(int64))
	if err != nil {
		return err
	}
	return c.JSON(t)
}

func (a *API) clock(c *fiber.Ctx) error {
	key := strings.TrimSpace(c.FormValue("idempotency_key"))
	if len(key) < 16 || len(key) > 100 {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "Missing request key.")
	}
	fh, err := c.FormFile("photo")
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "photo_required", "A live photo is required to clock in.")
	}
	if fh.Size > int64(a.MaxPhotoMB)<<20 {
		return apiError(c, fiber.StatusRequestEntityTooLarge, "photo_too_large", "The photo is too large. Try again.")
	}
	f, err := fh.Open()
	if err != nil {
		return err
	}
	defer f.Close()
	photo, err := io.ReadAll(io.LimitReader(f, int64(a.MaxPhotoMB)<<20+1))
	if err != nil {
		return err
	}

	in := attendance.ClockInput{
		IdempotencyKey: key,
		Photo:          photo,
		DeviceID:       truncate(c.FormValue("device_id"), 100),
		Offline:        c.FormValue("offline") == "true",
		Channel:        "mobile",
	}
	if v := c.FormValue("client_time"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			in.ClientTime = &t
		}
	}
	in.Latitude = parseCoord(c.FormValue("latitude"), 90)
	in.Longitude = parseCoord(c.FormValue("longitude"), 180)

	l, replay, err := a.Attendance.Clock(c.Context(), c.Locals("employee_id").(int64), in)
	switch {
	case errors.Is(err, attendance.ErrBadPhoto):
		return apiError(c, fiber.StatusUnprocessableEntity, "bad_photo", "The photo could not be read. Take it again.")
	case errors.Is(err, attendance.ErrDayComplete):
		return apiError(c, fiber.StatusConflict, "day_complete", "All four logs for today are already recorded.")
	case errors.Is(err, attendance.ErrTooSoon):
		return apiError(c, fiber.StatusConflict, "too_soon", "A log was just recorded. Wait a minute before clocking again.")
	case err != nil:
		return err
	}
	status := fiber.StatusCreated
	if replay {
		status = fiber.StatusOK
	}
	return c.Status(status).JSON(fiber.Map{"log": l, "replay": replay})
}

func parseCoord(s string, limit float64) *float64 {
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < -limit || v > limit {
		return nil
	}
	return &v
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
