package api

import (
	"errors"
	"fmt"
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

// csvCell neutralises spreadsheet formulas: a cell that starts with = + - @
// would run as a formula when opened in Excel.
func csvCell(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		s = "'" + s
	}
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
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
	b.WriteString("Employee no,Name,Office,Position,Days present,Late days,Absent days,Excused days,Undertime (min),Days to review,Days with corrections,Habitual tardiness\r\n")
	for _, r := range s.Rows {
		habitual := "No"
		if r.Habitual {
			habitual = "Yes"
		}
		fmt.Fprintf(&b, "%s,%s,%s,%s,%d,%d,%d,%d,%d,%d,%d,%s\r\n",
			csvCell(r.EmployeeNo), csvCell(r.Name), csvCell(r.Office), csvCell(r.Position),
			r.PresentDays, r.LateDays, r.AbsentDays, r.ExcusedDays, r.UndertimeMin,
			r.Incomplete, r.Adjusted, habitual)
	}
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
