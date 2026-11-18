package api

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"dtr/server/internal/auth"
	"dtr/server/internal/settings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func mfaRole(role string) bool { return role == roleHR || role == roleSuper || role == roleAuditor }

func (a *API) mfaNeeded(role string, enabled bool) bool {
	return mfaRole(role) && !enabled && a.Settings.Int(settings.MFARequired) == 1
}

// openSession issues a short access token and a rotating refresh token.
func (a *API) openSession(ctx context.Context, employeeID int64, family string) (fiber.Map, error) {
	access, err := auth.IssueToken(a.JWTSecret, employeeID)
	if err != nil {
		return nil, err
	}
	refresh, err := auth.RandomToken(32)
	if err != nil {
		return nil, err
	}
	if family == "" {
		if family, err = auth.RandomToken(12); err != nil {
			return nil, err
		}
	}
	days := a.Settings.Int(settings.SessionDays)
	if _, err := a.Pool.Exec(ctx, `INSERT INTO sessions (employee_id, family, token_hash, expires_at)
		VALUES ($1,$2,$3, now() + ($4 * interval '1 day'))`, employeeID, family, auth.HashToken(refresh), days); err != nil {
		return nil, err
	}
	return fiber.Map{"token": access, "refresh_token": refresh}, nil
}

// finishLogin returns the session and the employee once every factor is checked.
func (a *API) finishLogin(c *fiber.Ctx, e employee) error {
	var err error
	if e.PrivacyAccepted, err = a.privacyAccepted(c.Context(), e.ID); err != nil {
		return err
	}
	var enabled bool
	if err := a.Pool.QueryRow(c.Context(), `UPDATE employees SET last_login_at=now() WHERE id=$1 RETURNING mfa_enabled`, e.ID).Scan(&enabled); err != nil {
		return err
	}
	e.MFASetupRequired = a.mfaNeeded(e.Role, enabled)
	out, err := a.openSession(c.Context(), e.ID, "")
	if err != nil {
		return err
	}
	out["employee"] = e
	return c.JSON(out)
}

// refresh swaps a refresh token for a new pair. Using an old token again means
// it was stolen or copied, so the whole session family is ended.
func (a *API) refresh(c *fiber.Ctx) error {
	var in struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.BodyParser(&in); err != nil || in.RefreshToken == "" {
		return apiError(c, fiber.StatusUnauthorized, "unauthorized", "Sign in to continue.")
	}
	var id, emp int64
	var family string
	var rotated, revoked *time.Time
	var expires time.Time
	var active bool
	err := a.Pool.QueryRow(c.Context(), `SELECT s.id, s.employee_id, s.family, s.rotated_at, s.revoked_at, s.expires_at, e.active
		FROM sessions s JOIN employees e ON e.id=s.employee_id WHERE s.token_hash=$1`, auth.HashToken(in.RefreshToken)).
		Scan(&id, &emp, &family, &rotated, &revoked, &expires, &active)
	if errors.Is(err, pgx.ErrNoRows) {
		return apiError(c, fiber.StatusUnauthorized, "unauthorized", "Your session has ended. Sign in again.")
	}
	if err != nil {
		return err
	}
	if rotated != nil {
		_, _ = a.Pool.Exec(c.Context(), `UPDATE sessions SET revoked_at=now() WHERE family=$1 AND revoked_at IS NULL`, family)
		_, _ = a.Pool.Exec(c.Context(), `INSERT INTO audit_logs (actor_id, action, target) VALUES ($1,'session.reuse_blocked','family')`, emp)
		return apiError(c, fiber.StatusUnauthorized, "unauthorized", "Your session has ended. Sign in again.")
	}
	if revoked != nil || !active || time.Now().After(expires) {
		return apiError(c, fiber.StatusUnauthorized, "unauthorized", "Your session has ended. Sign in again.")
	}
	tag, err := a.Pool.Exec(c.Context(), `UPDATE sessions SET rotated_at=now() WHERE id=$1 AND rotated_at IS NULL`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apiError(c, fiber.StatusUnauthorized, "unauthorized", "Your session has ended. Sign in again.")
	}
	out, err := a.openSession(c.Context(), emp, family)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

func (a *API) logout(c *fiber.Ctx) error {
	var in struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.BodyParser(&in); err == nil && in.RefreshToken != "" {
		_, _ = a.Pool.Exec(c.Context(), `UPDATE sessions SET revoked_at=now() WHERE revoked_at IS NULL AND family IN
			(SELECT family FROM sessions WHERE token_hash=$1)`, auth.HashToken(in.RefreshToken))
	}
	return c.JSON(fiber.Map{"ok": true})
}

// revokeSessions ends every session of an employee, as after a PIN reset.
func (a *API) revokeSessions(ctx context.Context, employeeID int64) error {
	_, err := a.Pool.Exec(ctx, `UPDATE sessions SET revoked_at=now() WHERE employee_id=$1 AND revoked_at IS NULL`, employeeID)
	return err
}

// mfaLogin completes sign-in with an authenticator or recovery code.
func (a *API) mfaLogin(c *fiber.Ctx) error {
	var in struct {
		Token string `json:"mfa_token"`
		Code  string `json:"code"`
	}
	if err := c.BodyParser(&in); err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "Enter your code.")
	}
	id, err := auth.ParseMFAToken(a.JWTSecret, in.Token)
	if err != nil {
		return apiError(c, fiber.StatusUnauthorized, "session_expired", "That took too long. Sign in again.")
	}
	var sealed string
	var last int64
	var locked *time.Time
	e, err := func() (employee, error) {
		var e employee
		err := a.Pool.QueryRow(c.Context(), `SELECT `+empCols+`, mfa_secret, mfa_last_step, locked_until FROM employees WHERE id=$1 AND active`, id).
			Scan(&e.ID, &e.EmployeeNo, &e.FullName, &e.Position, &e.Office, &e.Role, &e.MustChangePIN, &sealed, &last, &locked)
		return e, err
	}()
	if err != nil {
		return apiError(c, fiber.StatusUnauthorized, "session_expired", "That took too long. Sign in again.")
	}
	if locked != nil && locked.After(time.Now()) {
		return a.credentialError(c, credLocked)
	}
	secret, err := auth.OpenSecret(a.JWTSecret, sealed)
	if err != nil {
		return err
	}
	if step, ok := auth.VerifyTOTP(secret, in.Code, time.Now(), last); ok {
		if _, err := a.Pool.Exec(c.Context(), `UPDATE employees SET mfa_last_step=$2, failed_attempts=0 WHERE id=$1`, id, step); err != nil {
			return err
		}
		return a.finishLogin(c, e)
	}
	code := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(in.Code), "-", ""))
	tag, err := a.Pool.Exec(c.Context(), `UPDATE mfa_recovery_codes SET used_at=now() WHERE employee_id=$1 AND code_hash=$2 AND used_at IS NULL`, id, auth.HashToken(code))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 1 {
		_, _ = a.Pool.Exec(c.Context(), `INSERT INTO audit_logs (actor_id, action, target) VALUES ($1,'mfa.recovery_used','')`, id)
		return a.finishLogin(c, e)
	}
	if _, err := a.Pool.Exec(c.Context(), `
		UPDATE employees SET
		  failed_attempts = CASE WHEN failed_attempts + 1 >= $2 THEN 0 ELSE failed_attempts + 1 END,
		  locked_until    = CASE WHEN failed_attempts + 1 >= $2 THEN now() + ($3 * interval '1 minute') ELSE locked_until END
		WHERE id=$1`, id, a.Settings.Int(settings.PINMaxAttempts), a.Settings.Int(settings.PINLockMinutes)); err != nil {
		return err
	}
	return apiError(c, fiber.StatusUnauthorized, "invalid_code", "That code is not correct.")
}

func (a *API) mfaStart(c *fiber.Ctx) error {
	var no string
	var enabled bool
	if err := a.Pool.QueryRow(c.Context(), `SELECT employee_no, mfa_enabled FROM employees WHERE id=$1`, actorID(c)).Scan(&no, &enabled); err != nil {
		return err
	}
	if enabled {
		return apiError(c, fiber.StatusConflict, "already_enabled", "Two-step sign-in is already on.")
	}
	secret, err := auth.NewTOTPSecret()
	if err != nil {
		return err
	}
	sealed, err := auth.SealSecret(a.JWTSecret, secret)
	if err != nil {
		return err
	}
	if _, err := a.Pool.Exec(c.Context(), `UPDATE employees SET mfa_secret=$2 WHERE id=$1`, actorID(c), sealed); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"secret": secret, "url": auth.TOTPURL("DTR", no, secret)})
}

func (a *API) mfaConfirm(c *fiber.Ctx) error {
	var in struct {
		Code string `json:"code"`
	}
	if err := c.BodyParser(&in); err != nil {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "Enter your code.")
	}
	var sealed string
	if err := a.Pool.QueryRow(c.Context(), `SELECT mfa_secret FROM employees WHERE id=$1 AND NOT mfa_enabled`, actorID(c)).Scan(&sealed); err != nil || sealed == "" {
		return apiError(c, fiber.StatusConflict, "not_started", "Start the setup again.")
	}
	secret, err := auth.OpenSecret(a.JWTSecret, sealed)
	if err != nil {
		return err
	}
	step, ok := auth.VerifyTOTP(secret, in.Code, time.Now(), 0)
	if !ok {
		return apiError(c, fiber.StatusUnauthorized, "invalid_code", "That code is not correct. Check the time on your phone and try again.")
	}
	codes := make([]string, 8)
	tx, err := a.Pool.Begin(c.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(c.Context()) //nolint:errcheck
	if _, err := tx.Exec(c.Context(), `DELETE FROM mfa_recovery_codes WHERE employee_id=$1`, actorID(c)); err != nil {
		return err
	}
	for i := range codes {
		raw, err := auth.RandomToken(8)
		if err != nil {
			return err
		}
		code := strings.ToUpper(strings.NewReplacer("-", "A", "_", "B").Replace(raw))[:10]
		codes[i] = code[:5] + "-" + code[5:]
		if _, err := tx.Exec(c.Context(), `INSERT INTO mfa_recovery_codes (employee_id, code_hash) VALUES ($1,$2)`, actorID(c), auth.HashToken(code)); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(c.Context(), `UPDATE employees SET mfa_enabled=TRUE, mfa_last_step=$2 WHERE id=$1`, actorID(c), step); err != nil {
		return err
	}
	if err := tx.Commit(c.Context()); err != nil {
		return err
	}
	a.audit(c, "mfa.enable", "employee:"+strconv.FormatInt(actorID(c), 10))
	return c.JSON(fiber.Map{"recovery_codes": codes})
}

// resetMFA lets a super admin help someone who lost their phone.
func (a *API) resetMFA(c *fiber.Ctx) error {
	id, _, err := a.targetID(c)
	if err != nil || id == 0 {
		return err
	}
	if id == actorID(c) {
		return apiError(c, fiber.StatusForbidden, "forbidden", "Another super admin must reset your two-step sign-in.")
	}
	if _, err := a.Pool.Exec(c.Context(), `UPDATE employees SET mfa_enabled=FALSE, mfa_secret='', mfa_last_step=0 WHERE id=$1`, id); err != nil {
		return err
	}
	if err := a.revokeSessions(c.Context(), id); err != nil {
		return err
	}
	a.audit(c, "mfa.reset", "employee:"+c.Params("id"))
	return c.JSON(fiber.Map{"ok": true})
}
