-- Phase 0 schema: employees, attendance logs (append-only), photos.

CREATE TABLE employees (
    id            BIGSERIAL PRIMARY KEY,
    employee_no   TEXT NOT NULL UNIQUE,
    full_name     TEXT NOT NULL,
    position      TEXT NOT NULL DEFAULT '',
    office        TEXT NOT NULL DEFAULT '',
    pin_hash      TEXT NOT NULL,
    active        BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Slots follow the four-log day: AM in, AM out, PM in, PM out.
CREATE TYPE log_slot AS ENUM ('am_in', 'am_out', 'pm_in', 'pm_out');
CREATE TYPE log_channel AS ENUM ('mobile', 'kiosk', 'web');

CREATE TABLE attendance_logs (
    id               BIGSERIAL PRIMARY KEY,
    employee_id      BIGINT NOT NULL REFERENCES employees(id),
    work_date        DATE NOT NULL,                 -- Asia/Manila calendar day
    slot             log_slot NOT NULL,
    recorded_at      TIMESTAMPTZ NOT NULL DEFAULT now(), -- official server time
    client_time      TIMESTAMPTZ,                   -- device time, for audit only
    channel          log_channel NOT NULL DEFAULT 'mobile',
    device_id        TEXT NOT NULL DEFAULT '',
    latitude         DOUBLE PRECISION,
    longitude        DOUBLE PRECISION,
    offline          BOOLEAN NOT NULL DEFAULT FALSE,
    idempotency_key  TEXT NOT NULL,
    photo_key        TEXT NOT NULL,
    photo_sha256     TEXT NOT NULL,
    UNIQUE (employee_id, idempotency_key),
    UNIQUE (employee_id, work_date, slot)
);

CREATE INDEX attendance_logs_employee_date ON attendance_logs (employee_id, work_date);

-- Raw logs are append-only. Corrections are new records, never edits.
CREATE FUNCTION attendance_logs_immutable() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'attendance_logs is append-only';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER attendance_logs_no_update
    BEFORE UPDATE OR DELETE ON attendance_logs
    FOR EACH ROW EXECUTE FUNCTION attendance_logs_immutable();

CREATE TABLE audit_logs (
    id          BIGSERIAL PRIMARY KEY,
    actor_id    BIGINT REFERENCES employees(id),
    action      TEXT NOT NULL,
    target      TEXT NOT NULL DEFAULT '',
    at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
