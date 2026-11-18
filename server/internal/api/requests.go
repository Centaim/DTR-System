package api

import (
	"errors"
	"strconv"

	"dtr/server/internal/requests"

	"github.com/gofiber/fiber/v2"
)

func (a *API) requestRoutes(secured fiber.Router) {
	secured.Post("/requests", a.createRequest)
	secured.Get("/requests", a.myRequests)
	secured.Post("/requests/:id/cancel", a.cancelRequest)

	approvers := secured.Group("/approvals", a.requireRole(roleSupervisor, roleHR, roleSuper))
	approvers.Get("/", a.pendingApprovals)
	approvers.Post("/:id/decide", a.decideRequest)
}

func requestError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, requests.ErrInvalid):
		return apiError(c, fiber.StatusUnprocessableEntity, "invalid", requests.Message(err))
	case errors.Is(err, requests.ErrConflict):
		return apiError(c, fiber.StatusConflict, "conflict", requests.Message(err))
	case errors.Is(err, requests.ErrNotFound):
		return apiError(c, fiber.StatusNotFound, "not_found", "Request not found.")
	case errors.Is(err, requests.ErrForbidden):
		return apiError(c, fiber.StatusForbidden, "forbidden", requests.Message(err))
	}
	return err
}

func (a *API) createRequest(c *fiber.Ctx) error {
	var in requests.Input
	if err := c.BodyParser(&in); err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "The form could not be read.")
	}
	r, err := a.Requests.Create(c.Context(), actorID(c), in)
	if err != nil {
		return requestError(c, err)
	}
	a.audit(c, "request.create", "request:"+strconv.FormatInt(r.ID, 10))
	return c.Status(fiber.StatusCreated).JSON(r)
}

func (a *API) myRequests(c *fiber.Ctx) error {
	list, err := a.Requests.Mine(c.Context(), actorID(c))
	if err != nil {
		return err
	}
	return c.JSON(list)
}

func (a *API) cancelRequest(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "Invalid request.")
	}
	if err := a.Requests.Cancel(c.Context(), actorID(c), id); err != nil {
		return requestError(c, err)
	}
	a.audit(c, "request.cancel", "request:"+c.Params("id"))
	return c.JSON(fiber.Map{"ok": true})
}

func (a *API) pendingApprovals(c *fiber.Ctx) error {
	list, err := a.Requests.Pending(c.Context(), actorID(c), actorRole(c))
	if err != nil {
		return err
	}
	return c.JSON(list)
}

func (a *API) decideRequest(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "Invalid request.")
	}
	var body struct {
		Approve bool   `json:"approve"`
		Note    string `json:"note"`
	}
	if err := c.BodyParser(&body); err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "The decision could not be read.")
	}
	r, err := a.Requests.Decide(c.Context(), actorID(c), actorRole(c), id, body.Approve, body.Note)
	if err != nil {
		return requestError(c, err)
	}
	a.audit(c, "request."+r.Status, "request:"+c.Params("id"))
	return c.JSON(r)
}
