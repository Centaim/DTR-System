package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"dtr/server/internal/engage"
	"dtr/server/internal/settings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func engageError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, engage.ErrInvalid):
		return apiError(c, fiber.StatusUnprocessableEntity, "invalid", engage.Message(err))
	case errors.Is(err, engage.ErrConflict):
		return apiError(c, fiber.StatusConflict, "conflict", engage.Message(err))
	case errors.Is(err, engage.ErrNotFound):
		return apiError(c, fiber.StatusNotFound, "not_found", "That item was not found.")
	}
	return err
}

func paramID(c *fiber.Ctx) (int64, error) {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return 0, apiError(c, fiber.StatusBadRequest, "bad_request", "Invalid item.")
	}
	return id, nil
}

func (a *API) officeOf(ctx context.Context, id int64) string {
	var office string
	_ = a.Pool.QueryRow(ctx, `SELECT office FROM employees WHERE id=$1`, id).Scan(&office)
	return office
}

func (a *API) engageRoutes(s fiber.Router) {
	manage := a.requireRole(roleHR, roleSuper)
	events := a.requireRole(roleHR, roleSuper, roleTimekeeper)
	approver := a.requireRole(roleSupervisor, roleHR, roleSuper)

	s.Get("/notifications", a.listNotifications)
	s.Post("/notifications/read", a.readNotifications)
	s.Get("/announcements", a.listAnnouncements)
	s.Post("/announcements", manage, a.createAnnouncement)
	s.Post("/announcements/:id/ack", a.ackAnnouncement)

	s.Get("/accomplishments", a.myAccomplishments)
	s.Post("/accomplishments", a.createAccomplishment)
	s.Get("/accomplishments/pending", approver, a.pendingAccomplishments)
	s.Post("/accomplishments/:id/review", approver, a.reviewAccomplishment)

	s.Get("/events", a.listEvents)
	s.Post("/events", events, a.createEvent)
	s.Post("/events/:id/check-in", a.eventCheckIn)
	s.Get("/events/:id/attendance", a.requireRole(roleHR, roleSuper, roleTimekeeper, roleAuditor), a.eventAttendance)
	s.Post("/events/:id/mark", events, a.eventMark)

	s.Get("/delegations", approver, a.listDelegations)
	s.Post("/delegations", approver, a.createDelegation)
	s.Delete("/delegations/:id", approver, a.deleteDelegation)

	s.Post("/logbook", a.requireRole(roleTimekeeper, roleHR, roleSuper), a.addLogbook)

	s.Post("/requests/:id/attachment", a.uploadAttachment)
	s.Get("/requests/:id/attachment", a.viewAttachment)
}

func (a *API) listNotifications(c *fiber.Ctx) error {
	list, unread, err := a.Engage.Notifications(c.Context(), actorID(c))
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"items": list, "unread": unread})
}

func (a *API) readNotifications(c *fiber.Ctx) error {
	if err := a.Engage.MarkAllRead(c.Context(), actorID(c)); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (a *API) listAnnouncements(c *fiber.Ctx) error {
	list, err := a.Engage.Announcements(c.Context(), actorID(c), a.officeOf(c.Context(), actorID(c)))
	if err != nil {
		return err
	}
	return c.JSON(list)
}

func (a *API) createAnnouncement(c *fiber.Ctx) error {
	var in struct{ Title, Body, Office string }
	if err := c.BodyParser(&in); err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "The form could not be read.")
	}
	if err := a.Engage.CreateAnnouncement(c.Context(), actorID(c), in.Title, in.Body, in.Office); err != nil {
		return engageError(c, err)
	}
	a.audit(c, "announcement.create", in.Title)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"ok": true})
}

func (a *API) ackAnnouncement(c *fiber.Ctx) error {
	id, err := paramID(c)
	if err != nil {
		return err
	}
	if err := a.Engage.Ack(c.Context(), id, actorID(c)); err != nil {
		return engageError(c, err)
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (a *API) myAccomplishments(c *fiber.Ctx) error {
	list, err := a.Engage.MyAccomplishments(c.Context(), actorID(c))
	if err != nil {
		return err
	}
	return c.JSON(list)
}

func (a *API) createAccomplishment(c *fiber.Ctx) error {
	var in struct {
		From string `json:"period_from"`
		To   string `json:"period_to"`
		Body string `json:"body"`
	}
	if err := c.BodyParser(&in); err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "The form could not be read.")
	}
	if err := a.Engage.CreateAccomplishment(c.Context(), actorID(c), in.From, in.To, in.Body); err != nil {
		return engageError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"ok": true})
}

func (a *API) pendingAccomplishments(c *fiber.Ctx) error {
	list, err := a.Engage.PendingAccomplishments(c.Context(), actorID(c), actorRole(c))
	if err != nil {
		return err
	}
	return c.JSON(list)
}

func (a *API) reviewAccomplishment(c *fiber.Ctx) error {
	id, err := paramID(c)
	if err != nil {
		return err
	}
	var in struct {
		Accept bool   `json:"accept"`
		Note   string `json:"note"`
	}
	if err := c.BodyParser(&in); err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "The form could not be read.")
	}
	if err := a.Engage.ReviewAccomplishment(c.Context(), actorID(c), actorRole(c), id, in.Accept, in.Note); err != nil {
		return engageError(c, err)
	}
	a.audit(c, "accomplishment.review", "report:"+c.Params("id"))
	return c.JSON(fiber.Map{"ok": true})
}

func (a *API) listEvents(c *fiber.Ctx) error {
	list, err := a.Engage.Events(c.Context(), actorID(c), a.officeOf(c.Context(), actorID(c)))
	if err != nil {
		return err
	}
	return c.JSON(list)
}

func (a *API) createEvent(c *fiber.Ctx) error {
	var in struct {
		Title    string `json:"title"`
		Place    string `json:"place"`
		Office   string `json:"office"`
		StartsAt string `json:"starts_at"`
		EndsAt   string `json:"ends_at"`
		Required bool   `json:"required"`
	}
	if err := c.BodyParser(&in); err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "The form could not be read.")
	}
	if err := a.Engage.CreateEvent(c.Context(), actorID(c), in.Title, in.Place, in.Office, in.StartsAt, in.EndsAt, in.Required); err != nil {
		return engageError(c, err)
	}
	a.audit(c, "event.create", in.Title)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"ok": true})
}

func (a *API) eventCheckIn(c *fiber.Ctx) error {
	id, err := paramID(c)
	if err != nil {
		return err
	}
	emp := actorID(c)
	if err := a.Engage.CanCheckIn(c.Context(), id, a.officeOf(c.Context(), emp)); err != nil {
		return engageError(c, err)
	}
	data, code, err := readUpload(c, "photo", int64(a.MaxPhotoMB)<<20)
	if err != nil {
		return apiError(c, code, "photo_required", "A live photo is required to check in.")
	}
	if t := http.DetectContentType(data); t != "image/jpeg" && t != "image/png" && t != "image/webp" {
		return apiError(c, fiber.StatusUnprocessableEntity, "bad_photo", "The photo could not be read. Take it again.")
	}
	key := fmt.Sprintf("events/%d/%d.img", id, emp)
	if _, err := a.Photos.Put(c.Context(), key, data); err != nil {
		return err
	}
	if err := a.Engage.CheckIn(c.Context(), id, emp, key); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"ok": true})
}

func readUpload(c *fiber.Ctx, field string, limit int64) ([]byte, int, error) {
	fh, err := c.FormFile(field)
	if err != nil {
		return nil, fiber.StatusBadRequest, err
	}
	if fh.Size > limit {
		return nil, fiber.StatusRequestEntityTooLarge, errors.New("too large")
	}
	f, err := fh.Open()
	if err != nil {
		return nil, fiber.StatusBadRequest, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	return data, fiber.StatusBadRequest, err
}

func (a *API) eventAttendance(c *fiber.Ctx) error {
	id, err := paramID(c)
	if err != nil {
		return err
	}
	list, err := a.Engage.Attendance(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(list)
}

func (a *API) eventMark(c *fiber.Ctx) error {
	id, err := paramID(c)
	if err != nil {
		return err
	}
	var in struct {
		EmployeeNo string `json:"employee_no"`
	}
	if err := c.BodyParser(&in); err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "The form could not be read.")
	}
	if err := a.Engage.Mark(c.Context(), id, in.EmployeeNo, actorID(c)); err != nil {
		return engageError(c, err)
	}
	a.audit(c, "event.mark", "event:"+c.Params("id"))
	return c.JSON(fiber.Map{"ok": true})
}

func (a *API) listDelegations(c *fiber.Ctx) error {
	list, err := a.Engage.Delegations(c.Context(), actorID(c))
	if err != nil {
		return err
	}
	return c.JSON(list)
}

func (a *API) createDelegation(c *fiber.Ctx) error {
	var in struct {
		EmployeeNo string `json:"employee_no"`
		From       string `json:"from_date"`
		To         string `json:"to_date"`
	}
	if err := c.BodyParser(&in); err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "The form could not be read.")
	}
	if err := a.Engage.Delegate(c.Context(), actorID(c), in.EmployeeNo, in.From, in.To); err != nil {
		return engageError(c, err)
	}
	a.audit(c, "delegation.create", in.EmployeeNo)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"ok": true})
}

func (a *API) deleteDelegation(c *fiber.Ctx) error {
	id, err := paramID(c)
	if err != nil {
		return err
	}
	if err := a.Engage.DeleteDelegation(c.Context(), id, actorID(c)); err != nil {
		return err
	}
	a.audit(c, "delegation.delete", "delegation:"+c.Params("id"))
	return c.JSON(fiber.Map{"ok": true})
}

func (a *API) addLogbook(c *fiber.Ctx) error {
	var in struct {
		EmployeeNo string         `json:"employee_no"`
		Day        string         `json:"day"`
		Reason     string         `json:"reason"`
		Entries    []engage.Entry `json:"entries"`
	}
	if err := c.BodyParser(&in); err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "The form could not be read.")
	}
	n, err := a.Engage.AddLogbook(c.Context(), actorID(c), in.EmployeeNo, in.Day, in.Reason, in.Entries)
	if err != nil {
		return engageError(c, err)
	}
	a.audit(c, "logbook.encode", in.EmployeeNo+" "+in.Day)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"saved": n})
}

const maxAttachment = 3 << 20

func (a *API) uploadAttachment(c *fiber.Ctx) error {
	id, err := paramID(c)
	if err != nil {
		return err
	}
	data, code, err := readUpload(c, "file", maxAttachment)
	if err != nil {
		return apiError(c, code, "bad_file", "Attach a photo or PDF of 3 MB or less.")
	}
	kind := http.DetectContentType(data)
	if kind != "image/jpeg" && kind != "image/png" && kind != "application/pdf" {
		return apiError(c, fiber.StatusUnprocessableEntity, "bad_file", "Attach a JPEG, PNG or PDF file.")
	}
	key := fmt.Sprintf("attachments/%d.bin", id)
	var owner int64
	err = a.Pool.QueryRow(c.Context(), `SELECT employee_id FROM requests WHERE id=$1 AND status='pending'`, id).Scan(&owner)
	if err != nil || owner != actorID(c) {
		return apiError(c, fiber.StatusNotFound, "not_found", "Only your own pending request can take an attachment.")
	}
	if _, err := a.Photos.Put(c.Context(), key, data); err != nil {
		return err
	}
	if _, err := a.Pool.Exec(c.Context(), `UPDATE requests SET attachment_key=$2, attachment_type=$3 WHERE id=$1`, id, key, kind); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"ok": true})
}

// viewAttachment is limited to the owner, their approvers and HR, and is logged.
func (a *API) viewAttachment(c *fiber.Ctx) error {
	id, err := paramID(c)
	if err != nil {
		return err
	}
	var key, kind string
	err = a.Pool.QueryRow(c.Context(), `
		SELECT r.attachment_key, r.attachment_type FROM requests r JOIN employees e ON e.id=r.employee_id
		 WHERE r.id=$3 AND r.attachment_key<>'' AND (r.employee_id=$1 OR `+engage.Approver+`)`, actorID(c), actorRole(c), id).Scan(&key, &kind)
	if errors.Is(err, pgx.ErrNoRows) {
		return apiError(c, fiber.StatusNotFound, "not_found", "That attachment is not available.")
	}
	if err != nil {
		return err
	}
	data, err := a.Photos.Get(c.Context(), key)
	if err != nil {
		return apiError(c, fiber.StatusNotFound, "not_found", "That attachment is not available.")
	}
	a.audit(c, "attachment.view", "request:"+c.Params("id"))
	c.Set("Content-Type", kind)
	c.Set("Cache-Control", "private, no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	return c.Send(data)
}

// RunReminders sends scheduled alerts once an hour until ctx ends.
func (a *API) RunReminders(ctx context.Context) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		if err := a.remind(ctx); err != nil && ctx.Err() == nil {
			log.Printf("reminders: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (a *API) remind(ctx context.Context) error {
	if err := a.Engage.Escalate(ctx, a.Settings.Int(settings.ApprovalEscalateDays)); err != nil {
		return err
	}
	now := time.Now().In(a.Loc)
	if now.Hour() < 7 {
		return nil
	}
	rows, err := a.Pool.Query(ctx, `SELECT id, office FROM employees WHERE active`)
	if err != nil {
		return err
	}
	type emp struct {
		id     int64
		office string
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (emp, error) {
		var e emp
		return e, r.Scan(&e.id, &e.office)
	})
	if err != nil {
		return err
	}
	yesterday := now.AddDate(0, 0, -1)
	prev := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, a.Loc).AddDate(0, -1, 0)
	deadline := now.Day() <= a.Settings.Int(settings.DTRDeadlineDay)
	for _, e := range list {
		res, err := a.Calc.Month(ctx, e.id, e.office, yesterday.Year(), yesterday.Month())
		if err != nil {
			return err
		}
		if d := res.Sheet.Days[yesterday.Day()-1]; d.Incomplete {
			day := yesterday.Format("2006-01-02")
			a.Engage.Notify(ctx, e.id, "missed_log", day, "missed:"+day)
		}
		if deadline {
			sub, _, err := a.latestSubmission(ctx, e.id, prev.Year(), prev.Month())
			if err != nil {
				return err
			}
			if sub == nil || sub.Status == "returned" || sub.Status == "reopened" {
				month := prev.Format("2006-01")
				a.Engage.Notify(ctx, e.id, "dtr_deadline", month, "deadline:"+month)
			}
		}
	}
	return nil
}
