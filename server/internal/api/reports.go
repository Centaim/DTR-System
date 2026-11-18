package api

import (
	"encoding/csv"
	"errors"
	"strconv"
	"strings"
	"time"

	"dtr/server/internal/holidays"

	"github.com/gofiber/fiber/v2"
)

func (a *API) reportRoutes(secured fiber.Router) {
	g := secured.Group("/reports", a.requireRole(roleHR, roleSuper, roleAuditor))
	g.Get("/dashboard", a.dashboard)
	g.Get("/summary", a.summary)
	g.Get("/summary.csv", a.summaryCSV)
	g.Get("/holidays", a.listHolidays)
}

func (a *API) holidayAdminRoutes(g fiber.Router) {
	g.Post("/holidays", a.createHoliday)
	g.Delete("/holidays/:day", a.deleteHoliday)
	g.Get("/suspensions", a.listSuspensions)
	g.Post("/suspensions", a.createSuspension)
	g.Delete("/suspensions/:id", a.deleteSuspension)
}

func (a *API) dashboard(c *fiber.Ctx) error {
	d, err := a.Reports.Dashboard(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(d)
}

func (a *API) monthParam(c *fiber.Ctx) (int, time.Month, error) {
	now := time.Now().In(a.Loc)
	year, month := now.Year(), now.Month()
	if q := c.Query("month"); q != "" {
		t, err := time.Parse("2006-01", q)
		if err != nil {
			return 0, 0, apiError(c, fiber.StatusBadRequest, "bad_request", "Month must look like 2026-11.")
		}
		year, month = t.Year(), t.Month()
	}
	return year, month, nil
}

func (a *API) summary(c *fiber.Ctx) error {
	year, month, err := a.monthParam(c)
	if err != nil {
		return err
	}
	s, err := a.Reports.Summary(c.Context(), year, month, strings.TrimSpace(c.Query("office")))
	if err != nil {
		return err
	}
	return c.JSON(s)
}

func (a *API) summaryCSV(c *fiber.Ctx) error {
	year, month, err := a.monthParam(c)
	if err != nil {
		return err
	}
	s, err := a.Reports.Summary(c.Context(), year, month, strings.TrimSpace(c.Query("office")))
	if err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("\xef\xbb\xbf") // lets Excel read UTF-8
	cw := csv.NewWriter(&b)
	_ = cw.Write(summaryHeader)
	for _, r := range s.Rows {
		_ = cw.Write(summaryCells(r))
	}
	cw.Flush()
	a.audit(c, "report.export", "summary:"+s.Month)
	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", `attachment; filename="attendance-summary-`+s.Month+`.csv"`)
	c.Set("Cache-Control", "private, no-store")
	return c.SendString(b.String())
}

func (a *API) listHolidays(c *fiber.Ctx) error {
	year := time.Now().In(a.Loc).Year()
	if q := c.Query("year"); q != "" {
		y, err := strconv.Atoi(q)
		if err != nil || y < 2000 || y > 2100 {
			return apiError(c, fiber.StatusBadRequest, "bad_request", "Invalid year.")
		}
		year = y
	}
	list, err := a.Holidays.List(c.Context(), year)
	if err != nil {
		return err
	}
	return c.JSON(list)
}

func (a *API) createHoliday(c *fiber.Ctx) error {
	var in holidays.Holiday
	if err := c.BodyParser(&in); err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "The form could not be read.")
	}
	err := a.Holidays.Create(c.Context(), in.Day, in.Name, actorID(c))
	switch {
	case errors.Is(err, holidays.ErrInvalid):
		return apiError(c, fiber.StatusUnprocessableEntity, "invalid", holidays.Message(err))
	case errors.Is(err, holidays.ErrDuplicate):
		return apiError(c, fiber.StatusConflict, "duplicate", "That date is already a holiday.")
	case err != nil:
		return err
	}
	a.audit(c, "holiday.create", "holiday:"+in.Day)
	return c.Status(fiber.StatusCreated).JSON(in)
}

func (a *API) deleteHoliday(c *fiber.Ctx) error {
	if err := a.Holidays.Delete(c.Context(), c.Params("day")); err != nil {
		if errors.Is(err, holidays.ErrNotFound) {
			return apiError(c, fiber.StatusNotFound, "not_found", "That holiday was not found.")
		}
		return err
	}
	a.audit(c, "holiday.delete", "holiday:"+c.Params("day"))
	return c.JSON(fiber.Map{"ok": true})
}

func (a *API) listSuspensions(c *fiber.Ctx) error {
	list, err := a.Holidays.Suspensions(c.Context(), time.Now().In(a.Loc).Year(), 0)
	if err != nil {
		return err
	}
	return c.JSON(list)
}

func (a *API) createSuspension(c *fiber.Ctx) error {
	var in holidays.Suspension
	if err := c.BodyParser(&in); err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "The form could not be read.")
	}
	err := a.Holidays.CreateSuspension(c.Context(), in, actorID(c))
	switch {
	case errors.Is(err, holidays.ErrInvalid):
		return apiError(c, fiber.StatusUnprocessableEntity, "invalid", holidays.Message(err))
	case errors.Is(err, holidays.ErrDuplicate):
		return apiError(c, fiber.StatusConflict, "duplicate", "A suspension already exists for that date and office.")
	case err != nil:
		return err
	}
	a.audit(c, "suspension.create", in.Day)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"ok": true})
}

func (a *API) deleteSuspension(c *fiber.Ctx) error {
	if err := a.Holidays.DeleteSuspension(c.Context(), c.Params("id")); err != nil {
		return apiError(c, fiber.StatusNotFound, "not_found", "That suspension was not found.")
	}
	a.audit(c, "suspension.delete", "suspension:"+c.Params("id"))
	return c.JSON(fiber.Map{"ok": true})
}
