package api

import (
	"errors"
	"io"
	"strings"
	"time"

	"dtr/server/internal/attendance"
	"dtr/server/internal/auth"
	"dtr/server/internal/kiosk"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

func (a *API) kioskRoutes(app *fiber.App) {
	g := app.Group("/api/kiosk", a.requireKiosk)
	g.Get("/info", a.kioskInfo)
	// Per kiosk, not per person: a lock on the account stops guessing at one
	// employee; this stops a flood from one device.
	g.Post("/verify", limiter.New(limiter.Config{
		Max: 30, Expiration: time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string { return "kiosk:" + c.Get("X-Kiosk-Token") },
		LimitReached: func(c *fiber.Ctx) error {
			return apiError(c, fiber.StatusTooManyRequests, "rate_limited", "Too many attempts. Wait a minute and try again.")
		},
	}), a.kioskVerify)
	g.Post("/privacy", a.kioskPrivacy)
	g.Post("/clock", a.kioskClock)
}

// requireKiosk accepts only registered, active kiosks.
func (a *API) requireKiosk(c *fiber.Ctx) error {
	k, err := a.Kiosks.Authenticate(c.Context(), c.Get("X-Kiosk-Token"))
	if err != nil {
		if errors.Is(err, kiosk.ErrToken) {
			return apiError(c, fiber.StatusUnauthorized, "kiosk_unregistered", "This kiosk is not registered.")
		}
		return err
	}
	c.Locals("kiosk", k)
	return c.Next()
}

func kioskOf(c *fiber.Ctx) kiosk.Kiosk { return c.Locals("kiosk").(kiosk.Kiosk) }

func (a *API) kioskInfo(c *fiber.Ctx) error {
	k := kioskOf(c)
	return c.JSON(fiber.Map{"name": k.Name, "office": k.Office, "server_time": time.Now().UTC().Format(time.RFC3339)})
}

// kioskVerify checks who is at the kiosk and says which log is next. It returns
// a two-minute token good for one clock-in at this kiosk, nothing else.
func (a *API) kioskVerify(c *fiber.Ctx) error {
	var body struct {
		EmployeeNo string `json:"employee_no"`
		PIN        string `json:"pin"`
	}
	if err := c.BodyParser(&body); err != nil || body.EmployeeNo == "" || body.PIN == "" {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "Enter your employee number and PIN.")
	}
	e, res, err := a.verifyCredentials(c.Context(), body.EmployeeNo, body.PIN)
	if err != nil {
		return err
	}
	if res != credOK {
		return a.credentialError(c, res)
	}
	if e.MustChangePIN {
		return apiError(c, fiber.StatusForbidden, "pin_change_required",
			"Choose your own PIN first: sign in on the DTR app or website.")
	}
	today, err := a.Attendance.Today(c.Context(), e.ID)
	if err != nil {
		return err
	}
	if today.NextSlot == nil {
		return apiError(c, fiber.StatusConflict, "day_complete", "All logs for today are already recorded.")
	}
	token, err := auth.IssueKioskToken(a.JWTSecret, e.ID, kioskOf(c).ID)
	if err != nil {
		return err
	}
	accepted, err := a.privacyAccepted(c.Context(), e.ID)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"session": token, "name": e.FullName, "next_slot": *today.NextSlot, "privacy_accepted": accepted})
}

func (a *API) kioskClock(c *fiber.Ctx) error {
	empID, kioskID, err := auth.ParseKioskToken(a.JWTSecret, c.FormValue("session"))
	if err != nil || kioskID != kioskOf(c).ID {
		return apiError(c, fiber.StatusUnauthorized, "session_expired", "That took too long. Start again.")
	}
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

	if ok, err := a.privacyAccepted(c.Context(), empID); err != nil {
		return err
	} else if !ok {
		return apiError(c, fiber.StatusForbidden, "privacy_required", "Read and accept the privacy notice to continue.")
	}
	var name string
	var active bool
	if err := a.Pool.QueryRow(c.Context(), `SELECT full_name, active FROM employees WHERE id = $1`, empID).Scan(&name, &active); err != nil || !active {
		return apiError(c, fiber.StatusUnauthorized, "session_expired", "That took too long. Start again.")
	}

	l, replay, err := a.Attendance.Clock(c.Context(), empID, attendance.ClockInput{
		IdempotencyKey: key, Photo: photo, Channel: "kiosk", FaceSeen: faceSeen(c),
		DeviceID: "kiosk:" + strconvID(kioskID),
	})
	switch {
	case errors.Is(err, attendance.ErrChannel):
		return apiError(c, fiber.StatusForbidden, "channel_blocked", "You cannot clock in here. Use the channel HR assigned to you.")
	case errors.Is(err, attendance.ErrBadPhoto):
		return apiError(c, fiber.StatusUnprocessableEntity, "bad_photo", "The photo could not be read. Take it again.")
	case errors.Is(err, attendance.ErrDayComplete):
		return apiError(c, fiber.StatusConflict, "day_complete", "All logs for today are already recorded.")
	case errors.Is(err, attendance.ErrTooSoon):
		return apiError(c, fiber.StatusConflict, "too_soon", "A log was just recorded. Wait a minute before clocking again.")
	case err != nil:
		return err
	}
	status := fiber.StatusCreated
	if replay {
		status = fiber.StatusOK
	}
	return c.Status(status).JSON(fiber.Map{"log": l, "name": name, "replay": replay})
}
