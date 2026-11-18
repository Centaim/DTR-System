-- Phase 1: roles, employment details, schedules with effective dates.

CREATE TYPE employee_role AS ENUM
    ('employee', 'supervisor', 'timekeeper', 'hr_admin', 'super_admin', 'auditor');

CREATE TYPE employment_status AS ENUM
    ('plantilla', 'casual', 'contract_of_service', 'job_order');

ALTER TABLE employees
    ADD COLUMN role           employee_role     NOT NULL DEFAULT 'employee',
    ADD COLUMN employment     employment_status NOT NULL DEFAULT 'plantilla',
    ADD COLUMN supervisor_id  BIGINT REFERENCES employees(id),
    ADD COLUMN must_change_pin BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE schedules (
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    am_start    TIME NOT NULL,
    am_end      TIME NOT NULL,
    pm_start    TIME NOT NULL,
    pm_end      TIME NOT NULL,
    is_default  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (am_end > am_start AND pm_end > pm_start AND pm_start >= am_end)
);

-- Exactly one default schedule applies to employees with no assignment.
CREATE UNIQUE INDEX schedules_one_default ON schedules (is_default) WHERE is_default;

-- Past DTRs keep the schedule that was in force then: assignments are dated.
CREATE TABLE schedule_assignments (
    id             BIGSERIAL PRIMARY KEY,
    employee_id    BIGINT NOT NULL REFERENCES employees(id),
    schedule_id    BIGINT NOT NULL REFERENCES schedules(id),
    effective_from DATE NOT NULL,
    created_by     BIGINT REFERENCES employees(id),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (employee_id, effective_from)
);

INSERT INTO schedules (name, am_start, am_end, pm_start, pm_end, is_default)
VALUES ('Regular 8:00 to 5:00', '08:00', '12:00', '13:00', '17:00', TRUE);

CREATE INDEX audit_logs_at ON audit_logs (at DESC);
