// Package api wires the HTTP routes.
package api

import (
	"errors"
	"io"
	"log"
	"strconv"
	"strings"
	"time"

	"dtr/server/internal/attendance"
	"dtr/server/internal/auth"
	"dtr/server/internal/dtr"
	"dtr/server/internal/holidays"
	"dtr/server/internal/photoaudit"
	"dtr/server/internal/reports"
	"dtr/server/internal/requests"
	"dtr/server/internal/schedule"
	"dtr/server/internal/storage"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	roleEmployee   = "employee"
	roleSupervisor = "supervisor"
	roleTimekeeper = "timekeeper"
	roleHR         = "hr_admin"
	roleSuper      = "super_admin"
	roleAuditor    = "auditor"
)

var allRoles = map[string]bool{
	roleEmployee: true, roleSupervisor: true, roleTimekeeper: true,
	roleHR: true, roleSuper: true, roleAuditor: true,
}

type API struct {
	Pool       *pgxpool.Pool
	Attendance *attendance.Service
	Schedules  *schedule.Store
	Requests   *requests.Store
	PhotoAudit *photoaudit.Store
	Holidays   *holidays.Store
	Reports    *reports.Service
	Photos     storage.Store
	JWTSecret  []byte
	MaxPhotoMB int
	Loc        *time.Location
}

type employee struct {
	ID            int64  `json:"id"`
	EmployeeNo    string `json:"employee_no"`
	FullName      string `json:"full_name"`
	Position      string `json:"position"`
	Office        string `json:"office"`
	Role          string `json:"role"`
	MustChangePIN bool   `json:"must_change_pin"`
}

const empCols = `id, employee_no, full_name, position, office, role::text, must_change_pin`

func scanEmployee(row pgx.Row) (employee, error) {
	var e employee
	err := row.Scan(&e.ID, &e.EmployeeNo, &e.FullName, &e.Position, &e.Office, &e.Role, &e.MustChangePIN)
	return e, err
}

func apiError(c *fiber.Ctx, status int, code, msg string) error {
	return c.Status(status).JSON(fiber.Map{"error": fiber.Map{"code": code, "message": msg}})
}

func rateLimit() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        10,
		Expiration: time.Minute,
		LimitReached: func(c *fiber.Ctx) error {
			return apiError(c, fiber.StatusTooManyRequests, "rate_limited", "Too many attempts. Wait a minute and try again.")
		},
	})
}

func (a *API) Routes(app *fiber.App) {
	app.Get("/api/health", func(c *fiber.Ctx) error {
		if err := a.Pool.Ping(c.Context()); err != nil {
			return apiError(c, fiber.StatusServiceUnavailable, "db_down", "database unavailable")
		}
		return c.JSON(fiber.Map{"status": "ok"})
	})

	app.Post("/api/auth/login", rateLimit(), a.login)

	secured := app.Group("/api", a.requireAuth)
	secured.Get("/me", a.me)
	secured.Post("/me/pin", rateLimit(), a.changePIN)
	secured.Get("/attendance/today", a.today)
	secured.Post("/attendance/clock", a.clock)
	secured.Get("/dtr", a.dtr)
	a.requestRoutes(secured)
	a.photoAuditRoutes(secured)
	a.reportRoutes(secured)

	adminGroup := secured.Group("/admin", a.requireRole(roleHR, roleSuper))
	a.adminRoutes(adminGroup)
	a.holidayAdminRoutes(adminGroup)
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

	var hash string
	var active bool
	row := a.Pool.QueryRow(c.Context(),
		`SELECT `+empCols+`, pin_hash, active FROM employees WHERE employee_no = $1`,
		strings.ToUpper(strings.TrimSpace(body.EmployeeNo)))
	var e employee
	err := row.Scan(&e.ID, &e.EmployeeNo, &e.FullName, &e.Position, &e.Office, &e.Role, &e.MustChangePIN, &hash, &active)
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

// requireAuth checks the token, then re-reads the account every request so a
// deactivated employee or a changed role takes effect immediately.
func (a *API) requireAuth(c *fiber.Ctx) error {
	token, ok := strings.CutPrefix(c.Get("Authorization"), "Bearer ")
	if !ok {
		return apiError(c, fiber.StatusUnauthorized, "unauthorized", "Sign in to continue.")
	}
	id, err := auth.ParseToken(a.JWTSecret, token)
	if err != nil {
		return apiError(c, fiber.StatusUnauthorized, "unauthorized", "Your session has ended. Sign in again.")
	}

	var role string
	var active, mustChange bool
	err = a.Pool.QueryRow(c.Context(),
		`SELECT role::text, active, must_change_pin FROM employees WHERE id = $1`, id).
		Scan(&role, &active, &mustChange)
	if err != nil || !active {
		return apiError(c, fiber.StatusUnauthorized, "unauthorized", "This account is no longer active.")
	}
	if mustChange && c.Path() != "/api/me" && c.Path() != "/api/me/pin" {
		return apiError(c, fiber.StatusForbidden, "pin_change_required", "Choose a new PIN before continuing.")
	}
	c.Locals("employee_id", id)
	c.Locals("role", role)
	return c.Next()
}

func (a *API) requireRole(allowed ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		role, _ := c.Locals("role").(string)
		for _, r := range allowed {
			if r == role {
				return c.Next()
			}
		}
		return apiError(c, fiber.StatusForbidden, "forbidden", "You do not have access to this.")
	}
}

func actorID(c *fiber.Ctx) int64 { return c.Locals("employee_id").(int64) }
func actorRole(c *fiber.Ctx) string {
	r, _ := c.Locals("role").(string)
	return r
}

// audit records who did what. Failures are logged, never shown to the user.
func (a *API) audit(c *fiber.Ctx, action, target string) {
	if _, err := a.Pool.Exec(c.Context(),
		`INSERT INTO audit_logs (actor_id, action, target) VALUES ($1, $2, $3)`,
		actorID(c), action, target); err != nil {
		log.Printf("audit write failed: %s %s: %v", action, target, err)
	}
}

func (a *API) me(c *fiber.Ctx) error {
	e, err := scanEmployee(a.Pool.QueryRow(c.Context(),
		`SELECT `+empCols+` FROM employees WHERE id = $1`, actorID(c)))
	if err != nil {
		return err
	}
	return c.JSON(e)
}

func (a *API) changePIN(c *fiber.Ctx) error {
	var body struct {
		CurrentPIN string `json:"current_pin"`
		NewPIN     string `json:"new_pin"`
	}
	if err := c.BodyParser(&body); err != nil || body.CurrentPIN == "" || body.NewPIN == "" {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "Enter your current PIN and a new PIN.")
	}
	var hash string
	if err := a.Pool.QueryRow(c.Context(), `SELECT pin_hash FROM employees WHERE id = $1`, actorID(c)).Scan(&hash); err != nil {
		return err
	}
	if !auth.VerifyPIN(body.CurrentPIN, hash) {
		return apiError(c, fiber.StatusUnauthorized, "invalid_credentials", "Your current PIN is incorrect.")
	}
	if body.NewPIN == body.CurrentPIN {
		return apiError(c, fiber.StatusUnprocessableEntity, "weak_pin", "Choose a PIN different from the current one.")
	}
	if err := auth.ValidatePIN(body.NewPIN); err != nil {
		return apiError(c, fiber.StatusUnprocessableEntity, "weak_pin", err.Error())
	}
	newHash, err := auth.HashPIN(body.NewPIN)
	if err != nil {
		return err
	}
	if _, err := a.Pool.Exec(c.Context(),
		`UPDATE employees SET pin_hash = $2, must_change_pin = FALSE WHERE id = $1`, actorID(c), newHash); err != nil {
		return err
	}
	a.audit(c, "pin.change", "employee:"+strconv.FormatInt(actorID(c), 10))
	return c.JSON(fiber.Map{"ok": true})
}

func (a *API) today(c *fiber.Ctx) error {
	t, err := a.Attendance.Today(c.Context(), actorID(c))
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

	l, replay, err := a.Attendance.Clock(c.Context(), actorID(c), in)
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

// dtr returns the employee's own Daily Time Record for ?month=YYYY-MM.
func (a *API) dtr(c *fiber.Ctx) error {
	now := time.Now().In(a.Loc)
	year, month := now.Year(), now.Month()
	if q := c.Query("month"); q != "" {
		t, err := time.Parse("2006-01", q)
		if err != nil {
			return apiError(c, fiber.StatusBadRequest, "bad_request", "Month must look like 2026-11.")
		}
		year, month = t.Year(), t.Month()
	}
	id := actorID(c)

	e, err := scanEmployee(a.Pool.QueryRow(c.Context(), `SELECT `+empCols+` FROM employees WHERE id = $1`, id))
	if err != nil {
		return err
	}
	logs, err := a.Attendance.MonthLogs(c.Context(), id, year, month, a.Loc)
	if err != nil {
		return err
	}
	hoursFor, err := a.Schedules.HoursFunc(c.Context(), id)
	if err != nil {
		return err
	}
	adjustments, marks, err := a.Requests.ForMonth(c.Context(), id, year, month)
	if err != nil {
		return err
	}
	hol, err := a.Holidays.ForMonth(c.Context(), year, month)
	if err != nil {
		return err
	}
	for d, label := range hol {
		if _, ok := marks[d]; !ok {
			marks[d] = label
		}
	}
	sheet := dtr.Compute(year, month, append(logs, adjustments...), marks, hoursFor, a.Loc)

	// Distinct official hours in the month, in order, for the form header.
	var official []fiber.Map
	seen := map[dtr.Hours]bool{}
	for _, d := range sheet.Days {
		h := hoursFor(time.Date(year, month, d.Day, 0, 0, 0, 0, a.Loc))
		if !seen[h] {
			seen[h] = true
			official = append(official, fiber.Map{
				"am_start": fmtHM(h.AM.Start), "am_end": fmtHM(h.AM.End),
				"pm_start": fmtHM(h.PM.Start), "pm_end": fmtHM(h.PM.End),
			})
		}
	}
	return c.JSON(fiber.Map{"employee": e, "sheet": sheet, "official_hours": official})
}

func fmtHM(m int) string { return time.Date(2000, 1, 1, m/60, m%60, 0, 0, time.UTC).Format("15:04") }

func nowDate(loc *time.Location) string { return time.Now().In(loc).Format("2006-01-02") }
