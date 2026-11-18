package api

import (
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"dtr/server/internal/auth"
	"dtr/server/internal/importer"
	"dtr/server/internal/kiosk"
	"dtr/server/internal/schedule"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var employmentTypes = map[string]bool{
	"plantilla": true, "casual": true, "contract_of_service": true, "job_order": true,
}

func privileged(role string) bool { return role == roleHR || role == roleSuper }

// canManage: only a super admin may change HR or super admin accounts, so an
// HR admin cannot take over a more powerful account by resetting its PIN.
func canManage(actor, target string) bool {
	return actor == roleSuper || !privileged(target)
}

type adminEmployee struct {
	ID            int64   `json:"id"`
	EmployeeNo    string  `json:"employee_no"`
	FullName      string  `json:"full_name"`
	Position      string  `json:"position"`
	Office        string  `json:"office"`
	Role          string  `json:"role"`
	Employment    string  `json:"employment"`
	Active        bool    `json:"active"`
	SupervisorID  *int64  `json:"supervisor_id"`
	Schedule      *string `json:"schedule"`
	ClockChannels int     `json:"clock_channels"`
	Email         string  `json:"email"`
}

const adminCols = `e.id, e.employee_no, e.full_name, e.position, e.office, e.role::text,
	e.employment::text, e.active, e.supervisor_id,
	COALESCE(
	  (SELECT s.name FROM schedule_assignments a JOIN schedules s ON s.id = a.schedule_id
	    WHERE a.employee_id = e.id AND a.effective_from <= (now() AT TIME ZONE 'Asia/Manila')::date
	    ORDER BY a.effective_from DESC LIMIT 1),
	  (SELECT name FROM schedules WHERE is_default)), e.clock_channels, e.email`

func scanAdmin(row pgx.Row) (adminEmployee, error) {
	var e adminEmployee
	err := row.Scan(&e.ID, &e.EmployeeNo, &e.FullName, &e.Position, &e.Office, &e.Role,
		&e.Employment, &e.Active, &e.SupervisorID, &e.Schedule, &e.ClockChannels, &e.Email)
	return e, err
}

func (a *API) adminRoutes(g fiber.Router) {
	g.Get("/employees", a.listEmployees)
	g.Post("/employees", a.createEmployee)
	g.Patch("/employees/:id", a.updateEmployee)
	g.Post("/employees/:id/reset-pin", a.resetPIN)
	g.Put("/employees/:id/schedule", a.assignSchedule)
	g.Get("/schedules", a.listSchedules)
	g.Post("/schedules", a.createSchedule)
	g.Post("/employees/import", a.importEmployees)
	g.Get("/kiosks", a.listKiosks)
	g.Post("/kiosks", a.createKiosk)
	g.Patch("/kiosks/:id", a.setKioskActive)
}

func (a *API) listEmployees(c *fiber.Ctx) error {
	q := "%" + strings.TrimSpace(c.Query("q")) + "%"
	rows, err := a.Pool.Query(c.Context(),
		`SELECT `+adminCols+` FROM employees e
		  WHERE e.full_name ILIKE $1 OR e.employee_no ILIKE $1 OR e.office ILIKE $1
		  ORDER BY e.active DESC, e.full_name LIMIT 500`, q)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []adminEmployee{}
	for rows.Next() {
		e, err := scanAdmin(rows)
		if err != nil {
			return err
		}
		out = append(out, e)
	}
	return c.JSON(out)
}

type employeeInput struct {
	EmployeeNo    *string `json:"employee_no"`
	FullName      *string `json:"full_name"`
	Position      *string `json:"position"`
	Office        *string `json:"office"`
	Role          *string `json:"role"`
	Employment    *string `json:"employment"`
	Active        *bool   `json:"active"`
	SupervisorID  *int64  `json:"supervisor_id"` // 0 clears the supervisor
	ClockChannels *int    `json:"clock_channels"`
	Email         *string `json:"email"`
}

func clean(p *string, max int) (*string, bool) {
	if p == nil {
		return nil, true
	}
	v := strings.TrimSpace(*p)
	return &v, len(v) <= max
}

func (a *API) createEmployee(c *fiber.Ctx) error {
	var in employeeInput
	if err := c.BodyParser(&in); err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "The form could not be read.")
	}
	no, ok1 := clean(in.EmployeeNo, 30)
	name, ok2 := clean(in.FullName, 120)
	pos, ok3 := clean(in.Position, 80)
	office, ok4 := clean(in.Office, 80)
	if !ok1 || !ok2 || !ok3 || !ok4 || no == nil || *no == "" || name == nil || *name == "" {
		return apiError(c, fiber.StatusUnprocessableEntity, "invalid", "Employee number and full name are required.")
	}
	role, employment := roleEmployee, "plantilla"
	if in.Role != nil {
		role = *in.Role
	}
	if in.Employment != nil {
		employment = *in.Employment
	}
	if !allRoles[role] || !employmentTypes[employment] {
		return apiError(c, fiber.StatusUnprocessableEntity, "invalid", "Choose a valid role and employment type.")
	}
	if privileged(role) && actorRole(c) != roleSuper {
		return apiError(c, fiber.StatusForbidden, "forbidden", "Only a super admin can create HR or super admin accounts.")
	}

	temp, err := auth.TempPIN()
	if err != nil {
		return err
	}
	hash, err := auth.HashPIN(temp)
	if err != nil {
		return err
	}
	var supervisor any
	if in.SupervisorID != nil && *in.SupervisorID > 0 {
		supervisor = *in.SupervisorID
	}
	e, err := scanAdmin(a.Pool.QueryRow(c.Context(), `
		WITH ins AS (
		  INSERT INTO employees (employee_no, full_name, position, office, role, employment,
		                         supervisor_id, pin_hash, must_change_pin)
		  VALUES ($1,$2,$3,$4,$5::employee_role,$6::employment_status,$7,$8,TRUE)
		  RETURNING *)
		SELECT `+adminCols+` FROM ins e`,
		strings.ToUpper(*no), *name, deref(pos), deref(office), role, employment, supervisor, hash))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				return apiError(c, fiber.StatusConflict, "duplicate", "That employee number already exists.")
			case "23503":
				return apiError(c, fiber.StatusUnprocessableEntity, "invalid", "The chosen supervisor does not exist.")
			}
		}
		return err
	}
	a.audit(c, "employee.create", "employee:"+strconv.FormatInt(e.ID, 10))
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"employee": e, "temp_pin": temp})
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (a *API) targetID(c *fiber.Ctx) (int64, string, error) {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return 0, "", apiError(c, fiber.StatusBadRequest, "bad_request", "Invalid employee.")
	}
	var role string
	err = a.Pool.QueryRow(c.Context(), `SELECT role::text FROM employees WHERE id = $1`, id).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", apiError(c, fiber.StatusNotFound, "not_found", "Employee not found.")
	}
	return id, role, err
}

func (a *API) updateEmployee(c *fiber.Ctx) error {
	id, targetRole, err := a.targetID(c)
	if err != nil || id == 0 {
		return err
	}
	if !canManage(actorRole(c), targetRole) {
		return apiError(c, fiber.StatusForbidden, "forbidden", "Only a super admin can change HR or super admin accounts.")
	}
	var in employeeInput
	if err := c.BodyParser(&in); err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "The form could not be read.")
	}
	name, ok1 := clean(in.FullName, 120)
	pos, ok2 := clean(in.Position, 80)
	office, ok3 := clean(in.Office, 80)
	if !ok1 || !ok2 || !ok3 || (name != nil && *name == "") {
		return apiError(c, fiber.StatusUnprocessableEntity, "invalid", "A name is required, and entries must be short enough.")
	}
	if in.Role != nil {
		if !allRoles[*in.Role] {
			return apiError(c, fiber.StatusUnprocessableEntity, "invalid", "Choose a valid role.")
		}
		if privileged(*in.Role) && actorRole(c) != roleSuper {
			return apiError(c, fiber.StatusForbidden, "forbidden", "Only a super admin can grant HR or super admin roles.")
		}
		if id == actorID(c) && *in.Role != targetRole {
			return apiError(c, fiber.StatusForbidden, "forbidden", "You cannot change your own role.")
		}
	}
	if in.Email != nil {
		v := strings.TrimSpace(*in.Email)
		if v != "" && (len(v) > 120 || !strings.Contains(v, "@") || strings.ContainsAny(v, " \r\n")) {
			return apiError(c, fiber.StatusUnprocessableEntity, "invalid", "Enter a valid email address.")
		}
		in.Email = &v
	}
	if in.ClockChannels != nil && (*in.ClockChannels < 1 || *in.ClockChannels > 3) {
		return apiError(c, fiber.StatusUnprocessableEntity, "invalid", "Choose at least one clock-in channel.")
	}
	if in.Employment != nil && !employmentTypes[*in.Employment] {
		return apiError(c, fiber.StatusUnprocessableEntity, "invalid", "Choose a valid employment type.")
	}
	if in.Active != nil && !*in.Active && id == actorID(c) {
		return apiError(c, fiber.StatusForbidden, "forbidden", "You cannot deactivate your own account.")
	}
	if in.SupervisorID != nil && *in.SupervisorID == id {
		return apiError(c, fiber.StatusUnprocessableEntity, "invalid", "An employee cannot supervise themselves.")
	}

	e, err := scanAdmin(a.Pool.QueryRow(c.Context(), `
		WITH upd AS (
		  UPDATE employees SET
		    full_name     = COALESCE($2, full_name),
		    position      = COALESCE($3, position),
		    office        = COALESCE($4, office),
		    role          = COALESCE($5::employee_role, role),
		    employment    = COALESCE($6::employment_status, employment),
		    active        = COALESCE($7, active),
		    supervisor_id = CASE WHEN $8::bigint IS NULL THEN supervisor_id
		                         WHEN $8::bigint = 0 THEN NULL ELSE $8::bigint END,
		    clock_channels = COALESCE($9, clock_channels),
		    email = COALESCE($10, email)
		  WHERE id = $1 RETURNING *)
		SELECT `+adminCols+` FROM upd e`,
		id, name, pos, office, in.Role, in.Employment, in.Active, in.SupervisorID, in.ClockChannels, in.Email))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return apiError(c, fiber.StatusUnprocessableEntity, "invalid", "The chosen supervisor does not exist.")
		}
		return err
	}
	a.audit(c, "employee.update", "employee:"+strconv.FormatInt(id, 10))
	return c.JSON(e)
}

func (a *API) resetPIN(c *fiber.Ctx) error {
	id, targetRole, err := a.targetID(c)
	if err != nil || id == 0 {
		return err
	}
	if !canManage(actorRole(c), targetRole) {
		return apiError(c, fiber.StatusForbidden, "forbidden", "Only a super admin can reset HR or super admin PINs.")
	}
	temp, err := auth.TempPIN()
	if err != nil {
		return err
	}
	hash, err := auth.HashPIN(temp)
	if err != nil {
		return err
	}
	if _, err := a.Pool.Exec(c.Context(),
		`UPDATE employees SET pin_hash = $2, must_change_pin = TRUE, failed_attempts = 0, locked_until = NULL WHERE id = $1`, id, hash); err != nil {
		return err
	}
	a.audit(c, "employee.reset_pin", "employee:"+strconv.FormatInt(id, 10))
	return c.JSON(fiber.Map{"temp_pin": temp})
}

func (a *API) listSchedules(c *fiber.Ctx) error {
	list, err := a.Schedules.List(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(list)
}

func (a *API) createSchedule(c *fiber.Ctx) error {
	var in schedule.Schedule
	if err := c.BodyParser(&in); err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "The form could not be read.")
	}
	s, err := a.Schedules.Create(c.Context(), in)
	switch {
	case errors.Is(err, schedule.ErrDuplicate):
		return apiError(c, fiber.StatusConflict, "duplicate", "A schedule with that name already exists.")
	case errors.Is(err, schedule.ErrInvalid):
		return apiError(c, fiber.StatusUnprocessableEntity, "invalid",
			"Check the times: each period must end after it starts, and the afternoon must begin after the morning ends.")
	case err != nil:
		return err
	}
	a.audit(c, "schedule.create", "schedule:"+strconv.FormatInt(s.ID, 10))
	return c.Status(fiber.StatusCreated).JSON(s)
}

func (a *API) assignSchedule(c *fiber.Ctx) error {
	id, targetRole, err := a.targetID(c)
	if err != nil || id == 0 {
		return err
	}
	if !canManage(actorRole(c), targetRole) {
		return apiError(c, fiber.StatusForbidden, "forbidden", "Only a super admin can change HR or super admin accounts.")
	}
	var in struct {
		ScheduleID    int64  `json:"schedule_id"`
		EffectiveFrom string `json:"effective_from"`
	}
	if err := c.BodyParser(&in); err != nil || in.ScheduleID == 0 {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "Choose a schedule and a start date.")
	}
	from, err := time.Parse("2006-01-02", in.EffectiveFrom)
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "Start date must look like 2026-11-30.")
	}
	if err := a.Schedules.Assign(c.Context(), id, in.ScheduleID, from, actorID(c)); err != nil {
		if errors.Is(err, schedule.ErrInvalid) {
			return apiError(c, fiber.StatusUnprocessableEntity, "invalid", strings.TrimPrefix(err.Error(), "invalid schedule: "))
		}
		return err
	}
	a.audit(c, "schedule.assign", "employee:"+strconv.FormatInt(id, 10))
	return c.JSON(fiber.Map{"ok": true})
}

func (a *API) listAudit(c *fiber.Ctx) error {
	rows, err := a.Pool.Query(c.Context(), `
		SELECT l.id, l.at, COALESCE(e.full_name, ''), l.action, l.target
		  FROM audit_logs l LEFT JOIN employees e ON e.id = l.actor_id
		 ORDER BY l.at DESC, l.id DESC LIMIT 100`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type row struct {
		ID     int64     `json:"id"`
		At     time.Time `json:"at"`
		Actor  string    `json:"actor"`
		Action string    `json:"action"`
		Target string    `json:"target"`
	}
	out := []row{}
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.ID, &r.At, &r.Actor, &r.Action, &r.Target); err != nil {
			return err
		}
		out = append(out, r)
	}
	return c.JSON(out)
}

const maxImportBytes = 2 << 20

// importEmployees reads an uploaded spreadsheet. Without commit=true it only
// previews, so HR sees every problem before any account is created.
func (a *API) importEmployees(c *fiber.Ctx) error {
	fh, err := c.FormFile("file")
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "file_required", "Choose a spreadsheet to import.")
	}
	if fh.Size > maxImportBytes {
		return apiError(c, fiber.StatusRequestEntityTooLarge, "file_too_large", "The file is larger than 2 MB.")
	}
	f, err := fh.Open()
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxImportBytes+1))
	if err != nil {
		return err
	}

	rows, err := importer.Parse(data, fh.Filename)
	if err != nil {
		if errors.Is(err, importer.ErrFile) {
			return apiError(c, fiber.StatusUnprocessableEntity, "bad_file", importer.Message(err))
		}
		return err
	}
	if err := a.Importer.MarkExisting(c.Context(), rows); err != nil {
		return err
	}

	summary := fiber.Map{"ok": 0, "duplicate": 0, "invalid": 0}
	for _, r := range rows {
		summary[r.Status] = summary[r.Status].(int) + 1
	}
	out := fiber.Map{"rows": rows, "summary": summary, "created": []importer.Created{}}

	if c.Query("commit") == "true" {
		created, err := a.Importer.Commit(c.Context(), rows)
		if err != nil {
			return err
		}
		a.audit(c, "employees.import", "count:"+strconv.Itoa(len(created)))
		if created == nil {
			created = []importer.Created{}
		}
		out["created"] = created
	}
	return c.JSON(out)
}

func (a *API) listKiosks(c *fiber.Ctx) error {
	list, err := a.Kiosks.List(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(list)
}

// createKiosk registers a kiosk. Its token is shown once, in this response.
func (a *API) createKiosk(c *fiber.Ctx) error {
	var in struct {
		Name   string `json:"name"`
		Office string `json:"office"`
	}
	if err := c.BodyParser(&in); err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "The form could not be read.")
	}
	k, token, err := a.Kiosks.Create(c.Context(), in.Name, in.Office, actorID(c))
	switch {
	case errors.Is(err, kiosk.ErrInvalid):
		return apiError(c, fiber.StatusUnprocessableEntity, "invalid", kiosk.Message(err))
	case errors.Is(err, kiosk.ErrDuplicate):
		return apiError(c, fiber.StatusConflict, "duplicate", "A kiosk with that name already exists.")
	case err != nil:
		return err
	}
	a.audit(c, "kiosk.create", "kiosk:"+strconv.FormatInt(k.ID, 10))
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"kiosk": k, "token": token})
}

func (a *API) setKioskActive(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "Invalid kiosk.")
	}
	var in struct {
		Active bool `json:"active"`
	}
	if err := c.BodyParser(&in); err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "The form could not be read.")
	}
	if err := a.Kiosks.SetActive(c.Context(), id, in.Active); err != nil {
		if errors.Is(err, kiosk.ErrNotFound) {
			return apiError(c, fiber.StatusNotFound, "not_found", "That kiosk was not found.")
		}
		return err
	}
	a.audit(c, "kiosk.active."+strconv.FormatBool(in.Active), "kiosk:"+c.Params("id"))
	return c.JSON(fiber.Map{"ok": true})
}
