package api

import (
	"errors"
	"net/http"
	"strconv"

	"dtr/server/internal/photoaudit"

	"github.com/gofiber/fiber/v2"
)

func (a *API) photoAuditRoutes(secured fiber.Router) {
	g := secured.Group("/audit", a.requireRole(roleHR, roleSuper, roleAuditor))
	g.Get("/logs", a.auditList)
	g.Get("/logs/:id/photo", a.auditPhoto)
	g.Post("/logs/:id/review", a.requireRole(roleHR, roleSuper), a.auditReview)
}

func auditError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, photoaudit.ErrInvalid):
		return apiError(c, fiber.StatusUnprocessableEntity, "invalid", photoaudit.Message(err))
	case errors.Is(err, photoaudit.ErrNotFound):
		return apiError(c, fiber.StatusNotFound, "not_found", "That log was not found.")
	}
	return err
}

func (a *API) auditList(c *fiber.Ctx) error {
	day := c.Query("date")
	if day == "" {
		day = nowDate(a.Loc)
	}
	items, err := a.PhotoAudit.List(c.Context(), c.Query("view", "flagged"), day)
	if err != nil {
		return auditError(c, err)
	}
	return c.JSON(items)
}

// auditPhoto serves a photo to reviewers only, and records every view.
func (a *API) auditPhoto(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "Invalid log.")
	}
	key, err := a.PhotoAudit.PhotoKey(c.Context(), id)
	if err != nil {
		return auditError(c, err)
	}
	data, err := a.Photos.Get(c.Context(), key)
	if err != nil {
		return apiError(c, fiber.StatusNotFound, "photo_missing", "The photo is not available.")
	}
	a.audit(c, "photo.view", "log:"+c.Params("id"))
	c.Set("Content-Type", http.DetectContentType(data))
	c.Set("Cache-Control", "private, no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	return c.Send(data)
}

func (a *API) auditReview(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "Invalid log.")
	}
	var body struct {
		Outcome string `json:"outcome"`
		Note    string `json:"note"`
	}
	if err := c.BodyParser(&body); err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "The review could not be read.")
	}
	if err := a.PhotoAudit.SetReview(c.Context(), id, actorID(c), body.Outcome, body.Note); err != nil {
		return auditError(c, err)
	}
	a.audit(c, "log.review."+body.Outcome, "log:"+c.Params("id"))
	return c.JSON(fiber.Map{"ok": true})
}
