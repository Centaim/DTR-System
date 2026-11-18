package api

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"dtr/server/internal/settings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/xuri/excelize/v2"
)

func (a *API) complianceRoutes(s fiber.Router) {
	read := a.requireRole(roleHR, roleSuper, roleAuditor)
	s.Get("/me/export", rateLimit(), a.exportMine)
	s.Get("/compliance/audit-pack", read, a.auditPack)
	s.Get("/compliance/access-review", read, a.accessReview)
	s.Get("/compliance/access-review.csv", read, a.accessReviewCSV)
	s.Get("/compliance/data-export", a.requireRole(roleHR, roleSuper), a.exportEmployee)
	s.Get("/compliance/system", a.requireRole(roleHR, roleSuper), a.systemStatus)
	s.Get("/reports/summary.xlsx", read, a.summaryXLSX)
}

// ---- Evidence archive

// pack writes a zip to an unlinked temporary file, so a large export neither
// sits in memory nor leaves personal data on disk. Every file is fingerprinted.
type pack struct {
	file  *os.File
	zw    *zip.Writer
	files []packFile
	sent  bool
}

// close releases the file unless it was handed over for streaming, in which
// case the server closes it after the download finishes.
func (p *pack) close() {
	if !p.sent {
		_ = p.file.Close()
	}
}

type packFile struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

func newPack() (*pack, error) {
	f, err := os.CreateTemp("", "dtr-pack-*")
	if err != nil {
		return nil, err
	}
	_ = os.Remove(f.Name())
	return &pack{file: f, zw: zip.NewWriter(f)}, nil
}

type countingHash struct {
	h hash.Hash
	n int64
}

func (c *countingHash) Write(b []byte) (int, error) {
	c.n += int64(len(b))
	return c.h.Write(b)
}

func (p *pack) add(name string, write func(io.Writer) error) error {
	w, err := p.zw.Create(name)
	if err != nil {
		return err
	}
	sum := &countingHash{h: sha256.New()}
	if err := write(io.MultiWriter(w, sum)); err != nil {
		return err
	}
	p.files = append(p.files, packFile{name, hex.EncodeToString(sum.h.Sum(nil)), sum.n})
	return nil
}

func (p *pack) addBytes(name string, b []byte) error {
	return p.add(name, func(w io.Writer) error { _, err := w.Write(b); return err })
}

// addQuery writes one SQL result as a spreadsheet-safe CSV.
func (p *pack) addQuery(ctx context.Context, q querier, name, sql string, args ...any) error {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	defer rows.Close()
	return p.add(name, func(w io.Writer) error {
		if _, err := w.Write([]byte("\xef\xbb\xbf")); err != nil {
			return err
		}
		cw := csv.NewWriter(w)
		head := make([]string, 0, len(rows.FieldDescriptions()))
		for _, f := range rows.FieldDescriptions() {
			head = append(head, f.Name)
		}
		if err := cw.Write(head); err != nil {
			return err
		}
		for rows.Next() {
			vals, err := rows.Values()
			if err != nil {
				return err
			}
			rec := make([]string, len(vals))
			for i, v := range vals {
				if v != nil {
					rec[i] = safeCell(fmt.Sprint(v))
				}
			}
			if err := cw.Write(rec); err != nil {
				return err
			}
		}
		cw.Flush()
		if err := cw.Error(); err != nil {
			return err
		}
		return rows.Err()
	})
}

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// safeCell stops a value being run as a formula when the CSV is opened in Excel.
func safeCell(s string) string {
	if s != "" && strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

// send finishes the archive with its manifest and streams it to the client.
func (p *pack) send(c *fiber.Ctx, filename string, about fiber.Map) error {
	about["generated_at"] = time.Now().UTC().Format(time.RFC3339)
	about["files"] = p.files
	manifest, err := json.MarshalIndent(about, "", "  ")
	if err != nil {
		return err
	}
	w, err := p.zw.Create("manifest.json")
	if err != nil {
		return err
	}
	if _, err := w.Write(manifest); err != nil {
		return err
	}
	if err := p.zw.Close(); err != nil {
		return err
	}
	size, err := p.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}
	if _, err := p.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	c.Set("Content-Type", "application/zip")
	c.Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Set("Cache-Control", "private, no-store")
	p.sent = true
	return c.SendStream(p.file, int(size)) // closes the file when done
}

func at(col string) string {
	return `to_char(` + col + ` AT TIME ZONE 'Asia/Manila','YYYY-MM-DD HH24:MI:SS')`
}

const maxPackPhotos = 5000

// auditPack gathers one month's evidence: DTRs, raw logs, corrections, flags,
// approvals and the activity log, optionally with the photos.
func (a *API) auditPack(c *fiber.Ctx) error {
	y, m, err := a.monthParam(c)
	if err != nil {
		return err
	}
	office := strings.TrimSpace(c.Query("office"))
	withPhotos := c.Query("photos") == "1"
	ctx := c.Context()
	from := monthStart(y, m)
	to := time.Date(y, m+1, 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	scope := `($3 = '' OR e.office = $3)`

	if withPhotos {
		var n int
		if err := a.Pool.QueryRow(ctx, `SELECT count(*) FROM attendance_logs l JOIN employees e ON e.id=l.employee_id
			WHERE l.work_date >= $1 AND l.work_date < $2 AND `+scope, from, to, office).Scan(&n); err != nil {
			return err
		}
		if n > maxPackPhotos {
			return apiError(c, fiber.StatusRequestEntityTooLarge, "too_many_photos",
				fmt.Sprintf("This month has %d photos. Choose one office, or export without photos.", n))
		}
	}

	p, err := newPack()
	if err != nil {
		return err
	}
	defer p.close()

	summary, err := a.Reports.Summary(ctx, y, m, office)
	if err != nil {
		return err
	}
	if err := p.add("dtr-summary.csv", func(w io.Writer) error { return writeSummaryCSV(w, summary.Rows) }); err != nil {
		return err
	}
	queries := []struct{ name, sql string }{
		{"raw-logs.csv", `SELECT l.id AS log_id, e.employee_no, e.full_name, e.office, l.work_date::text, l.slot::text,
			` + at("l.recorded_at") + ` AS recorded_at_manila, l.channel::text, l.offline, l.device_id, l.latitude, l.longitude, l.photo_sha256
			FROM attendance_logs l JOIN employees e ON e.id=l.employee_id
			WHERE l.work_date >= $1 AND l.work_date < $2 AND ` + scope + ` ORDER BY l.work_date, e.employee_no, l.id`},
		{"requests-and-corrections.csv", `SELECT r.id AS request_id, e.employee_no, e.full_name, r.kind::text, r.source, r.work_date::text, r.date_to::text,
			r.slot::text, r.requested_time::text, r.reason, r.status::text, d.full_name AS decided_by, ` + at("r.decided_at") + ` AS decided_at_manila, r.decision_note
			FROM requests r JOIN employees e ON e.id=r.employee_id LEFT JOIN employees d ON d.id=r.decided_by
			WHERE r.work_date < $2 AND COALESCE(r.date_to, r.work_date) >= $1 AND ` + scope + ` ORDER BY r.work_date, r.id`},
		{"flags-and-reviews.csv", `SELECT f.log_id, e.employee_no, e.full_name, f.kind::text AS flag, f.detail, v.outcome::text AS review_outcome,
			v.note AS review_note, x.full_name AS reviewed_by, ` + at("v.reviewed_at") + ` AS reviewed_at_manila
			FROM attendance_flags f JOIN attendance_logs l ON l.id=f.log_id JOIN employees e ON e.id=l.employee_id
			LEFT JOIN log_reviews v ON v.log_id=f.log_id LEFT JOIN employees x ON x.id=v.reviewed_by
			WHERE l.work_date >= $1 AND l.work_date < $2 AND ` + scope + ` ORDER BY f.log_id`},
		{"dtr-certifications.csv", `SELECT d.id, e.employee_no, e.full_name, d.version, d.status::text, ` + at("d.certified_at") + ` AS certified_at_manila,
			x.full_name AS decided_by, ` + at("d.decided_at") + ` AS decided_at_manila, d.note, d.sha256, d.verify_code
			FROM dtr_submissions d JOIN employees e ON e.id=d.employee_id LEFT JOIN employees x ON x.id=d.decided_by
			WHERE d.month >= $1 AND d.month < $2 AND ` + scope + ` ORDER BY e.employee_no, d.version`},
	}
	for _, q := range queries {
		if err := p.addQuery(ctx, a.Pool, q.name, q.sql, from, to, office); err != nil {
			return err
		}
	}
	if err := p.addQuery(ctx, a.Pool, "activity-log.csv", `SELECT l.id, `+at("l.at")+` AS at_manila, COALESCE(e.full_name,'System') AS actor, l.action, l.target
		FROM audit_logs l LEFT JOIN employees e ON e.id=l.actor_id WHERE l.at >= $1::date AND l.at < $2::date ORDER BY l.id`, from, to); err != nil {
		return err
	}
	if err := p.addQuery(ctx, a.Pool, "holidays.csv", `SELECT day::text, name FROM holidays WHERE day >= $1::date AND day < $2::date ORDER BY day`, from, to); err != nil {
		return err
	}

	snaps, err := a.Pool.Query(ctx, `SELECT e.employee_no, d.version, d.snapshot FROM dtr_submissions d JOIN employees e ON e.id=d.employee_id
		WHERE d.month = $1 AND d.status IN ('submitted','verified') AND `+strings.ReplaceAll(scope, "$3", "$2")+` ORDER BY 1`, from, office)
	if err != nil {
		return err
	}
	type snap struct {
		no   string
		ver  int
		body []byte
	}
	list, err := pgx.CollectRows(snaps, func(r pgx.CollectableRow) (snap, error) {
		var s snap
		return s, r.Scan(&s.no, &s.ver, &s.body)
	})
	if err != nil {
		return err
	}
	for _, s := range list {
		if err := p.addBytes(fmt.Sprintf("certified-dtr/%s-v%d.json", s.no, s.ver), s.body); err != nil {
			return err
		}
	}

	photos, missing := 0, 0
	if withPhotos {
		photos, missing, err = a.addPhotos(ctx, p, `SELECT l.id, l.photo_key FROM attendance_logs l JOIN employees e ON e.id=l.employee_id
			WHERE l.work_date >= $1 AND l.work_date < $2 AND `+scope+` ORDER BY l.id`, from, to, office)
		if err != nil {
			return err
		}
	}
	a.audit(c, "audit_pack.export", fmt.Sprintf("%s photos:%d", from[:7], photos))
	return p.send(c, "audit-pack-"+from[:7]+".zip", fiber.Map{
		"month": from[:7], "office": office, "photos_included": photos, "photos_unavailable": missing,
	})
}

// addPhotos decrypts photos into the archive. Photos past retention are counted as unavailable.
func (a *API) addPhotos(ctx context.Context, p *pack, sql string, args ...any) (int, int, error) {
	rows, err := a.Pool.Query(ctx, sql, args...)
	if err != nil {
		return 0, 0, err
	}
	type item struct {
		id  int64
		key string
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (item, error) {
		var it item
		return it, r.Scan(&it.id, &it.key)
	})
	if err != nil {
		return 0, 0, err
	}
	n, missing := 0, 0
	for _, it := range items {
		data, err := a.Photos.Get(ctx, it.key)
		if err != nil {
			missing++
			continue
		}
		ext := ".jpg"
		if http.DetectContentType(data) == "image/png" {
			ext = ".png"
		}
		if err := p.addBytes(fmt.Sprintf("photos/log-%d%s", it.id, ext), data); err != nil {
			return n, missing, err
		}
		n++
	}
	return n, missing, nil
}

// ---- Access review

type accessRow struct {
	EmployeeNo string  `json:"employee_no"`
	Name       string  `json:"name"`
	Office     string  `json:"office"`
	Role       string  `json:"role"`
	Active     bool    `json:"active"`
	TwoStep    bool    `json:"two_step"`
	LastLogin  *string `json:"last_login"`
	DaysIdle   *int    `json:"days_idle"`
	Sessions   int     `json:"sessions"`
	Attention  string  `json:"attention"`
}

const accessStaleDays = 90

// accessRows lists who can do what, most powerful accounts first, and says why
// an account deserves a second look.
func (a *API) accessRows(ctx context.Context) ([]accessRow, error) {
	rows, err := a.Pool.Query(ctx, `
		SELECT e.employee_no, e.full_name, e.office, e.role::text, e.active, e.mfa_enabled,
		       `+at("e.last_login_at")+`, (now()::date - e.last_login_at::date)::int,
		       (SELECT count(*)::int FROM sessions s WHERE s.employee_id=e.id AND s.revoked_at IS NULL AND s.rotated_at IS NULL AND s.expires_at > now())
		  FROM employees e
		 ORDER BY CASE e.role WHEN 'super_admin' THEN 0 WHEN 'hr_admin' THEN 1 WHEN 'auditor' THEN 2 WHEN 'timekeeper' THEN 3 WHEN 'supervisor' THEN 4 ELSE 5 END,
		          e.active DESC, e.full_name`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (accessRow, error) {
		var x accessRow
		err := r.Scan(&x.EmployeeNo, &x.Name, &x.Office, &x.Role, &x.Active, &x.TwoStep, &x.LastLogin, &x.DaysIdle, &x.Sessions)
		return x, err
	})
	if err != nil {
		return nil, err
	}
	for i := range list {
		x := &list[i]
		switch {
		case !x.Active && x.Sessions > 0:
			x.Attention = "inactive_with_session"
		case x.Active && mfaRole(x.Role) && !x.TwoStep:
			x.Attention = "no_two_step"
		case x.Active && x.Role != roleEmployee && (x.DaysIdle == nil || *x.DaysIdle > accessStaleDays):
			x.Attention = "unused_access"
		}
	}
	return list, nil
}

func (a *API) accessReview(c *fiber.Ctx) error {
	list, err := a.accessRows(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(list)
}

func (a *API) accessReviewCSV(c *fiber.Ctx) error {
	list, err := a.accessRows(c.Context())
	if err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("\xef\xbb\xbf")
	w := csv.NewWriter(&b)
	_ = w.Write([]string{"Employee no", "Name", "Office", "Role", "Active", "Two-step sign-in", "Last sign-in (Manila)", "Days since sign-in", "Open sessions", "Needs attention"})
	for _, x := range list {
		last, idle := "", ""
		if x.LastLogin != nil {
			last = *x.LastLogin
		}
		if x.DaysIdle != nil {
			idle = strconv.Itoa(*x.DaysIdle)
		}
		_ = w.Write([]string{safeCell(x.EmployeeNo), safeCell(x.Name), safeCell(x.Office), x.Role, yesNo(x.Active), yesNo(x.TwoStep), last, idle, strconv.Itoa(x.Sessions), x.Attention})
	}
	w.Flush()
	a.audit(c, "access_review.export", "")
	c.Set("Content-Type", "text/csv; charset=utf-8")
	c.Set("Content-Disposition", `attachment; filename="access-review.csv"`)
	c.Set("Cache-Control", "private, no-store")
	return c.SendString(b.String())
}

func yesNo(v bool) string {
	if v {
		return "Yes"
	}
	return "No"
}

// ---- Data subject export (RA 10173)

func (a *API) exportMine(c *fiber.Ctx) error { return a.writeDataExport(c, actorID(c)) }

func (a *API) exportEmployee(c *fiber.Ctx) error {
	var id int64
	err := a.Pool.QueryRow(c.Context(), `SELECT id FROM employees WHERE employee_no=$1`,
		strings.ToUpper(strings.TrimSpace(c.Query("employee_no")))).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return apiError(c, fiber.StatusNotFound, "not_found", "That employee number was not found.")
	}
	if err != nil {
		return err
	}
	return a.writeDataExport(c, id)
}

// writeDataExport packages everything held about one person. PIN hashes, MFA
// secrets and session tokens are security material and are never included.
func (a *API) writeDataExport(c *fiber.Ctx, id int64) error {
	ctx := c.Context()
	var no string
	if err := a.Pool.QueryRow(ctx, `SELECT employee_no FROM employees WHERE id=$1`, id).Scan(&no); err != nil {
		return err
	}
	p, err := newPack()
	if err != nil {
		return err
	}
	defer p.close()
	queries := []struct{ name, sql string }{
		{"profile.csv", `SELECT employee_no, full_name, position, office, role::text, employment::text, email, active, ` + at("created_at") + ` AS created_at_manila, ` + at("last_login_at") + ` AS last_login_manila FROM employees WHERE id=$1`},
		{"attendance-logs.csv", `SELECT id AS log_id, work_date::text, slot::text, ` + at("recorded_at") + ` AS recorded_at_manila, channel::text, offline, device_id, latitude, longitude FROM attendance_logs WHERE employee_id=$1 ORDER BY id`},
		{"flags.csv", `SELECT f.log_id, f.kind::text AS flag, f.detail, v.outcome::text AS review_outcome, v.note AS review_note FROM attendance_flags f JOIN attendance_logs l ON l.id=f.log_id LEFT JOIN log_reviews v ON v.log_id=f.log_id WHERE l.employee_id=$1 ORDER BY f.log_id`},
		{"requests.csv", `SELECT id, kind::text, source, work_date::text, date_to::text, slot::text, requested_time::text, reason, status::text, decision_note FROM requests WHERE employee_id=$1 ORDER BY id`},
		{"dtr-certifications.csv", `SELECT to_char(month,'YYYY-MM') AS month, version, status::text, ` + at("certified_at") + ` AS certified_at_manila, note, verify_code FROM dtr_submissions WHERE employee_id=$1 ORDER BY month, version`},
		{"accomplishments.csv", `SELECT period_from::text, period_to::text, body, status, note FROM accomplishment_reports WHERE employee_id=$1 ORDER BY id`},
		{"event-attendance.csv", `SELECT v.title, ` + at("x.at") + ` AS at_manila, x.method FROM event_attendance x JOIN events v ON v.id=x.event_id WHERE x.employee_id=$1 ORDER BY x.at`},
		{"devices.csv", `SELECT device_id, status::text, ` + at("requested_at") + ` AS requested_at_manila FROM device_bindings WHERE employee_id=$1 ORDER BY id`},
		{"privacy-acceptances.csv", `SELECT version, channel, ` + at("accepted_at") + ` AS accepted_at_manila FROM privacy_acceptances WHERE employee_id=$1 ORDER BY version`},
		{"notifications.csv", `SELECT kind, detail, ` + at("created_at") + ` AS created_at_manila FROM notifications WHERE employee_id=$1 ORDER BY id`},
	}
	for _, q := range queries {
		if err := p.addQuery(ctx, a.Pool, q.name, q.sql, id); err != nil {
			return err
		}
	}
	photos, missing, err := a.addPhotos(ctx, p, `SELECT id, photo_key FROM attendance_logs WHERE employee_id=$1 ORDER BY id`, id)
	if err != nil {
		return err
	}
	a.audit(c, "data.export", "employee:"+no)
	return p.send(c, "my-dtr-data-"+no+".zip", fiber.Map{"employee_no": no, "photos_included": photos, "photos_unavailable": missing})
}

// ---- Excel

func writeSummaryCSV(w io.Writer, rows []reportRow) error {
	if _, err := w.Write([]byte("\xef\xbb\xbf")); err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	_ = cw.Write(summaryHeader)
	for _, r := range rows {
		_ = cw.Write(summaryCells(r))
	}
	cw.Flush()
	return cw.Error()
}

var summaryHeader = []string{"Employee no", "Name", "Office", "Position", "Days present", "Late days", "Absent days", "Excused days",
	"Undertime (min)", "Days to review", "Days with corrections", "Correction requests", "Habitual tardiness", "Frequent corrections"}

func summaryCells(r reportRow) []string {
	return []string{safeCell(r.EmployeeNo), safeCell(r.Name), safeCell(r.Office), safeCell(r.Position),
		strconv.Itoa(r.PresentDays), strconv.Itoa(r.LateDays), strconv.Itoa(r.AbsentDays), strconv.Itoa(r.ExcusedDays),
		strconv.Itoa(r.UndertimeMin), strconv.Itoa(r.Incomplete), strconv.Itoa(r.Adjusted), strconv.Itoa(r.Corrections),
		yesNo(r.Habitual), yesNo(r.FrequentCorrections)}
}

func (a *API) summaryXLSX(c *fiber.Ctx) error {
	y, m, err := a.monthParam(c)
	if err != nil {
		return err
	}
	s, err := a.Reports.Summary(c.Context(), y, m, strings.TrimSpace(c.Query("office")))
	if err != nil {
		return err
	}
	f := excelize.NewFile()
	defer f.Close()
	const sheet = "Attendance summary"
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		return err
	}
	bold, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}, Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"E9EDF1"}}})
	if err != nil {
		return err
	}
	for i, h := range summaryHeader {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellStr(sheet, cell, h)
		_ = f.SetCellStyle(sheet, cell, cell, bold)
	}
	for y2, r := range s.Rows {
		strs := []string{r.EmployeeNo, r.Name, r.Office, r.Position}
		nums := []int{r.PresentDays, r.LateDays, r.AbsentDays, r.ExcusedDays, r.UndertimeMin, r.Incomplete, r.Adjusted, r.Corrections}
		col := 1
		for _, v := range strs {
			cell, _ := excelize.CoordinatesToCellName(col, y2+2)
			_ = f.SetCellStr(sheet, cell, v) // stored as text, so it can never run as a formula
			col++
		}
		for _, v := range nums {
			cell, _ := excelize.CoordinatesToCellName(col, y2+2)
			_ = f.SetCellInt(sheet, cell, int64(v))
			col++
		}
		for _, v := range []bool{r.Habitual, r.FrequentCorrections} {
			cell, _ := excelize.CoordinatesToCellName(col, y2+2)
			_ = f.SetCellStr(sheet, cell, yesNo(v))
			col++
		}
	}
	last, _ := excelize.CoordinatesToCellName(len(summaryHeader), len(s.Rows)+1)
	_ = f.AutoFilter(sheet, "A1:"+last, nil)
	_ = f.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"})
	_ = f.SetColWidth(sheet, "A", "A", 16)
	_ = f.SetColWidth(sheet, "B", "D", 28)
	_ = f.SetColWidth(sheet, "E", "N", 14)
	buf, err := f.WriteToBuffer()
	if err != nil {
		return err
	}
	a.audit(c, "report.export", "summary.xlsx:"+s.Month)
	c.Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Set("Content-Disposition", `attachment; filename="attendance-summary-`+s.Month+`.xlsx"`)
	c.Set("Cache-Control", "private, no-store")
	return c.Send(buf.Bytes())
}

// ---- System status

var photoUsage struct {
	sync.Mutex
	at    time.Time
	bytes int64
	files int64
}

// usage walks the photo folder at most every five minutes.
func usage(dir string) (int64, int64) {
	photoUsage.Lock()
	defer photoUsage.Unlock()
	if time.Since(photoUsage.at) > 5*time.Minute {
		var b, n int64
		_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				if info, e := d.Info(); e == nil {
					b += info.Size()
					n++
				}
			}
			return nil
		})
		photoUsage.bytes, photoUsage.files, photoUsage.at = b, n, time.Now()
	}
	return photoUsage.bytes, photoUsage.files
}

func (a *API) systemStatus(c *fiber.Ctx) error {
	ctx := c.Context()
	out := fiber.Map{"uptime_seconds": int(time.Since(a.Started).Seconds()), "photos_encrypted": a.PhotosEncrypted}

	var dbBytes int64
	if err := a.Pool.QueryRow(ctx, `SELECT pg_database_size(current_database())`).Scan(&dbBytes); err != nil {
		return err
	}
	used, files := usage(a.PhotoDir)
	var st syscall.Statfs_t
	var free, total uint64
	if err := syscall.Statfs(a.PhotoDir, &st); err == nil {
		free, total = st.Bavail*uint64(st.Bsize), st.Blocks*uint64(st.Bsize)
	}
	out["storage"] = fiber.Map{"database_bytes": dbBytes, "photo_bytes": used, "photo_files": files, "free_bytes": free, "total_bytes": total}

	var file string
	var size int64
	var okAt, failAt *time.Time
	err := a.Pool.QueryRow(ctx, `SELECT file, bytes, finished_at FROM backup_runs WHERE ok ORDER BY id DESC LIMIT 1`).Scan(&file, &size, &okAt)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if err := a.Pool.QueryRow(ctx, `SELECT max(finished_at) FROM backup_runs WHERE NOT ok`).Scan(&failAt); err != nil {
		return err
	}
	out["backup"] = fiber.Map{"file": file, "bytes": size, "last_ok_at": okAt, "last_failed_at": failAt}

	var kiosks, online, pendingReq, openFlags, devices, dtrs int
	if err := a.Pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM kiosks WHERE active),
		(SELECT count(*) FROM kiosks WHERE active AND last_seen_at > now() - interval '5 minutes'),
		(SELECT count(*) FROM requests WHERE status='pending'),
		(SELECT count(DISTINCT f.log_id) FROM attendance_flags f LEFT JOIN log_reviews r ON r.log_id=f.log_id WHERE r.log_id IS NULL),
		(SELECT count(*) FROM device_bindings WHERE status='pending'),
		(SELECT count(*) FROM dtr_submissions WHERE status='submitted')`).Scan(&kiosks, &online, &pendingReq, &openFlags, &devices, &dtrs); err != nil {
		return err
	}
	out["kiosks"] = fiber.Map{"active": kiosks, "online": online}
	out["queues"] = fiber.Map{"requests": pendingReq, "flags": openFlags, "devices": devices, "dtrs": dtrs}
	out["jobs"] = fiber.Map{"retention_last_run": unixOrNil(a.Retention.LastRun.Load()), "reminders_last_run": unixOrNil(a.remindersRan.Load())}
	out["retention_days"] = a.Settings.Int(settings.PhotoRetentionDays)
	return c.JSON(out)
}

func unixOrNil(v int64) any {
	if v == 0 {
		return nil
	}
	return time.Unix(v, 0).UTC().Format(time.RFC3339)
}
