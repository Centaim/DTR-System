package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"dtr/server/internal/dtr"
	"dtr/server/internal/settings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type dtrView struct {
	Employee      employee    `json:"employee"`
	Sheet         dtr.Sheet   `json:"sheet"`
	OfficialHours []fiber.Map `json:"official_hours"`
}

type submission struct {
	ID          int64      `json:"id"`
	Version     int        `json:"version"`
	Status      string     `json:"status"`
	VerifyCode  string     `json:"verify_code"`
	CertifiedAt time.Time  `json:"certified_at"`
	DecidedAt   *time.Time `json:"decided_at"`
	Note        string     `json:"note"`
}

func (a *API) dtrFlowRoutes(secured fiber.Router) {
	secured.Get("/dtr", a.dtr)
	secured.Post("/dtr/certify", a.certifyDTR)

	review := secured.Group("/dtr-review", a.requireRole(roleSupervisor, roleHR, roleSuper))
	review.Get("/", a.dtrPending)
	review.Get("/:id", a.dtrSubmission)
	review.Post("/:id/verify", a.dtrDecide("verified"))
	review.Post("/:id/return", a.dtrDecide("returned"))

	hr := secured.Group("/dtr-admin", a.requireRole(roleHR, roleSuper, roleAuditor))
	hr.Get("/tracker", a.dtrTracker)
	hr.Get("/bulk", a.dtrBulk)
	hr.Post("/:id/reopen", a.requireRole(roleHR, roleSuper), a.dtrReopen)
}

// buildDTR computes a month from raw logs, approved requests, holidays and schedules.
func (a *API) buildDTR(ctx context.Context, id int64, year int, month time.Month) (dtrView, error) {
	e, err := scanEmployee(a.Pool.QueryRow(ctx, `SELECT `+empCols+` FROM employees WHERE id = $1`, id))
	if err != nil {
		return dtrView{}, err
	}
	res, err := a.Calc.Month(ctx, id, e.Office, year, month)
	if err != nil {
		return dtrView{}, err
	}
	sheet, hoursFor := res.Sheet, res.HoursFor

	official := []fiber.Map{}
	seen := map[[2]dtr.Window]bool{}
	for _, d := range sheet.Days {
		h := hoursFor(time.Date(year, month, d.Day, 0, 0, 0, 0, a.Loc))
		if key := [2]dtr.Window{h.AM, h.PM}; !seen[key] {
			seen[key] = true
			official = append(official, fiber.Map{
				"am_start": fmtHM(h.AM.Start), "am_end": fmtHM(h.AM.End),
				"pm_start": fmtHM(h.PM.Start), "pm_end": fmtHM(h.PM.End),
			})
		}
	}
	return dtrView{Employee: e, Sheet: sheet, OfficialHours: official}, nil
}

func monthStart(y int, m time.Month) string {
	return time.Date(y, m, 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
}

const subCols = `id, version, status::text, verify_code, certified_at, decided_at, note`

func scanSub(row pgx.Row) (submission, error) {
	var s submission
	err := row.Scan(&s.ID, &s.Version, &s.Status, &s.VerifyCode, &s.CertifiedAt, &s.DecidedAt, &s.Note)
	return s, err
}

func (a *API) latestSubmission(ctx context.Context, id int64, y int, m time.Month) (*submission, json.RawMessage, error) {
	var snap json.RawMessage
	var s submission
	err := a.Pool.QueryRow(ctx, `SELECT `+subCols+`, snapshot FROM dtr_submissions
		WHERE employee_id=$1 AND month=$2 ORDER BY version DESC LIMIT 1`, id, monthStart(y, m)).
		Scan(&s.ID, &s.Version, &s.Status, &s.VerifyCode, &s.CertifiedAt, &s.DecidedAt, &s.Note, &snap)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	return &s, snap, nil
}

func (a *API) monthEnded(y int, m time.Month) bool {
	return !time.Now().In(a.Loc).Before(time.Date(y, m+1, 1, 0, 0, 0, 0, a.Loc))
}

// dtr returns a month. Once certified, the frozen version is what everyone sees.
func (a *API) dtr(c *fiber.Ctx) error {
	y, m, err := a.monthParam(c)
	if err != nil {
		return err
	}
	id := actorID(c)
	sub, snap, err := a.latestSubmission(c.Context(), id, y, m)
	if err != nil {
		return err
	}
	frozen := sub != nil && (sub.Status == "submitted" || sub.Status == "verified")
	canCertify := !frozen && (a.monthEnded(y, m) || a.Settings.Int(settings.DTREarlySubmit) == 1)
	if frozen {
		var v dtrView
		if err := json.Unmarshal(snap, &v); err != nil {
			return err
		}
		return c.JSON(fiber.Map{"employee": v.Employee, "sheet": v.Sheet, "official_hours": v.OfficialHours, "submission": sub, "can_certify": false})
	}
	v, err := a.buildDTR(c.Context(), id, y, m)
	if err != nil {
		return err
	}
	return c.JSON(fiber.Map{"employee": v.Employee, "sheet": v.Sheet, "official_hours": v.OfficialHours, "submission": sub, "can_certify": canCertify})
}

func newVerifyCode() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// certifyDTR freezes the employee's month as a new numbered version.
func (a *API) certifyDTR(c *fiber.Ctx) error {
	y, m, err := a.monthParam(c)
	if err != nil {
		return err
	}
	if !a.monthEnded(y, m) && a.Settings.Int(settings.DTREarlySubmit) != 1 {
		return apiError(c, fiber.StatusConflict, "month_open", "You can certify this DTR after the month ends.")
	}
	id := actorID(c)
	v, err := a.buildDTR(c.Context(), id, y, m)
	if err != nil {
		return err
	}
	snap, err := json.Marshal(v)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(snap)
	code, err := newVerifyCode()
	if err != nil {
		return err
	}
	s, err := scanSub(a.Pool.QueryRow(c.Context(), `
		INSERT INTO dtr_submissions (employee_id, month, version, snapshot, sha256, verify_code)
		VALUES ($1, $2, COALESCE((SELECT max(version) FROM dtr_submissions WHERE employee_id=$1 AND month=$2), 0) + 1, $3, $4, $5)
		RETURNING `+subCols, id, monthStart(y, m), snap, hex.EncodeToString(sum[:]), code))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return apiError(c, fiber.StatusConflict, "already_certified", "This month is already certified.")
	}
	if err != nil {
		return err
	}
	a.audit(c, "dtr.certify", "dtr:"+strconv.FormatInt(s.ID, 10))
	return c.Status(fiber.StatusCreated).JSON(s)
}

// reviewable is the approval rule: own reports, or everyone for HR. Never yourself.
const reviewable = `d.employee_id <> $1 AND (e.supervisor_id = $1 OR $2 IN ('hr_admin','super_admin'))`

func (a *API) dtrPending(c *fiber.Ctx) error {
	rows, err := a.Pool.Query(c.Context(), `
		SELECT d.id, e.full_name, e.employee_no, e.office, to_char(d.month,'YYYY-MM'), d.version, d.certified_at
		  FROM dtr_submissions d JOIN employees e ON e.id = d.employee_id
		 WHERE d.status = 'submitted' AND `+reviewable+` ORDER BY d.certified_at LIMIT 300`, actorID(c), actorRole(c))
	if err != nil {
		return err
	}
	defer rows.Close()
	type row struct {
		ID          int64     `json:"id"`
		Name        string    `json:"name"`
		EmployeeNo  string    `json:"employee_no"`
		Office      string    `json:"office"`
		Month       string    `json:"month"`
		Version     int       `json:"version"`
		CertifiedAt time.Time `json:"certified_at"`
	}
	out := []row{}
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.ID, &r.Name, &r.EmployeeNo, &r.Office, &r.Month, &r.Version, &r.CertifiedAt); err != nil {
			return err
		}
		out = append(out, r)
	}
	return c.JSON(out)
}

func (a *API) dtrSubmission(c *fiber.Ctx) error {
	var snap json.RawMessage
	var s submission
	err := a.Pool.QueryRow(c.Context(), `
		SELECT d.id, d.version, d.status::text, d.verify_code, d.certified_at, d.decided_at, d.note, d.snapshot
		  FROM dtr_submissions d JOIN employees e ON e.id = d.employee_id
		 WHERE d.id = $3 AND `+reviewable, actorID(c), actorRole(c), c.Params("id")).
		Scan(&s.ID, &s.Version, &s.Status, &s.VerifyCode, &s.CertifiedAt, &s.DecidedAt, &s.Note, &snap)
	if err != nil {
		return apiError(c, fiber.StatusNotFound, "not_found", "That DTR is not available to you.")
	}
	return c.JSON(fiber.Map{"submission": s, "dtr": snap})
}

func (a *API) dtrDecide(status string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		var body struct {
			Note string `json:"note"`
		}
		_ = c.BodyParser(&body)
		body.Note = strings.TrimSpace(body.Note)
		if status == "returned" && len(body.Note) < 3 {
			return apiError(c, fiber.StatusUnprocessableEntity, "invalid", "Say what needs correcting.")
		}
		if len(body.Note) > 300 {
			return apiError(c, fiber.StatusUnprocessableEntity, "invalid", "Keep the note under 300 characters.")
		}
		tag, err := a.Pool.Exec(c.Context(), `
			UPDATE dtr_submissions d SET status=$4::dtr_status, decided_by=$1, decided_at=now(), note=$5
			  FROM employees e
			 WHERE d.id=$3 AND d.status='submitted' AND e.id=d.employee_id AND `+reviewable,
			actorID(c), actorRole(c), c.Params("id"), status, body.Note)
		if err != nil {
			return apiError(c, fiber.StatusBadRequest, "bad_request", "Invalid DTR.")
		}
		if tag.RowsAffected() == 0 {
			return apiError(c, fiber.StatusConflict, "conflict", "This DTR is no longer waiting for you.")
		}
		a.audit(c, "dtr."+status, "dtr:"+c.Params("id"))
		return c.JSON(fiber.Map{"ok": true})
	}
}

// dtrReopen unlocks a verified month so the employee can certify a corrected version.
func (a *API) dtrReopen(c *fiber.Ctx) error {
	var body struct {
		Note string `json:"note"`
	}
	_ = c.BodyParser(&body)
	if len(strings.TrimSpace(body.Note)) < 3 {
		return apiError(c, fiber.StatusUnprocessableEntity, "invalid", "Say why this DTR is being reopened.")
	}
	tag, err := a.Pool.Exec(c.Context(), `UPDATE dtr_submissions SET status='reopened', decided_by=$1, decided_at=now(), note=$2
		WHERE id=$3 AND status='verified'`, actorID(c), strings.TrimSpace(body.Note), c.Params("id"))
	if err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "Invalid DTR.")
	}
	if tag.RowsAffected() == 0 {
		return apiError(c, fiber.StatusConflict, "conflict", "Only a verified DTR can be reopened.")
	}
	a.audit(c, "dtr.reopen", "dtr:"+c.Params("id"))
	return c.JSON(fiber.Map{"ok": true})
}

func (a *API) dtrTracker(c *fiber.Ctx) error {
	y, m, err := a.monthParam(c)
	if err != nil {
		return err
	}
	rows, err := a.Pool.Query(c.Context(), `
		SELECT e.employee_no, e.full_name, e.office, COALESCE(s.id, 0), COALESCE(s.status::text, 'not_submitted'),
		       COALESCE(s.version, 0), s.certified_at
		  FROM employees e
		  LEFT JOIN LATERAL (SELECT * FROM dtr_submissions d WHERE d.employee_id=e.id AND d.month=$1
		                      ORDER BY d.version DESC LIMIT 1) s ON TRUE
		 WHERE e.active ORDER BY e.office, e.full_name`, monthStart(y, m))
	if err != nil {
		return err
	}
	defer rows.Close()
	type row struct {
		EmployeeNo  string     `json:"employee_no"`
		Name        string     `json:"name"`
		Office      string     `json:"office"`
		ID          int64      `json:"id"`
		Status      string     `json:"status"`
		Version     int        `json:"version"`
		CertifiedAt *time.Time `json:"certified_at"`
	}
	out := []row{}
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.EmployeeNo, &r.Name, &r.Office, &r.ID, &r.Status, &r.Version, &r.CertifiedAt); err != nil {
			return err
		}
		out = append(out, r)
	}
	return c.JSON(out)
}

// dtrBulk returns every active employee's month for printing together.
func (a *API) dtrBulk(c *fiber.Ctx) error {
	y, m, err := a.monthParam(c)
	if err != nil {
		return err
	}
	office := strings.TrimSpace(c.Query("office"))
	rows, err := a.Pool.Query(c.Context(), `SELECT id FROM employees WHERE active AND ($1 = '' OR office = $1)
		ORDER BY office, full_name LIMIT 500`, office)
	if err != nil {
		return err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
	if err != nil {
		return err
	}
	out := make([]fiber.Map, 0, len(ids))
	for _, id := range ids {
		sub, snap, err := a.latestSubmission(c.Context(), id, y, m)
		if err != nil {
			return err
		}
		var v dtrView
		if sub != nil && (sub.Status == "submitted" || sub.Status == "verified") {
			err = json.Unmarshal(snap, &v)
		} else {
			v, err = a.buildDTR(c.Context(), id, y, m)
		}
		if err != nil {
			return err
		}
		out = append(out, fiber.Map{"employee": v.Employee, "sheet": v.Sheet, "official_hours": v.OfficialHours, "submission": sub})
	}
	a.audit(c, "dtr.bulk_print", monthStart(y, m)[:7])
	return c.JSON(out)
}

// publicVerify lets anyone holding a printed DTR confirm it is genuine. It
// reveals only what is already printed on the form.
func (a *API) publicVerify(c *fiber.Ctx) error {
	code := strings.ToLower(strings.TrimSpace(c.Params("code")))
	if len(code) != 32 {
		return apiError(c, fiber.StatusNotFound, "not_found", "This code does not match any DTR.")
	}
	var name, month, status, sum string
	var version int
	var latest int
	err := a.Pool.QueryRow(c.Context(), `
		SELECT e.full_name, to_char(d.month,'YYYY-MM'), d.status::text, d.version, d.sha256,
		       (SELECT max(version) FROM dtr_submissions x WHERE x.employee_id=d.employee_id AND x.month=d.month)
		  FROM dtr_submissions d JOIN employees e ON e.id=d.employee_id WHERE d.verify_code=$1`, code).
		Scan(&name, &month, &status, &version, &sum, &latest)
	if err != nil {
		return apiError(c, fiber.StatusNotFound, "not_found", "This code does not match any DTR.")
	}
	return c.JSON(fiber.Map{"name": name, "month": month, "status": status, "version": version,
		"is_latest": version == latest, "fingerprint": sum[:12]})
}
