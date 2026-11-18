-- Rotating refresh sessions and two-factor sign-in for administrators.

ALTER TABLE employees
    ADD COLUMN mfa_secret    TEXT NOT NULL DEFAULT '',
    ADD COLUMN mfa_enabled   BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN mfa_last_step BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN last_login_at TIMESTAMPTZ;

CREATE TABLE mfa_recovery_codes (
    id          BIGSERIAL PRIMARY KEY,
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    code_hash   TEXT NOT NULL,
    used_at     TIMESTAMPTZ
);

CREATE TABLE sessions (
    id          BIGSERIAL PRIMARY KEY,
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    family      TEXT NOT NULL,
    token_hash  TEXT NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL,
    rotated_at  TIMESTAMPTZ,
    revoked_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX sessions_family ON sessions (family);
CREATE INDEX sessions_employee ON sessions (employee_id);
