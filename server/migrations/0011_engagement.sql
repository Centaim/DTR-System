-- Notifications, announcements, accomplishments, events, delegation, attachments, logbook.

ALTER TABLE employees ADD COLUMN email TEXT NOT NULL DEFAULT '';

CREATE TABLE notifications (
    id          BIGSERIAL PRIMARY KEY,
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    kind        TEXT NOT NULL,
    detail      TEXT NOT NULL DEFAULT '',
    dedupe_key  TEXT,
    read_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX notifications_employee ON notifications (employee_id, created_at DESC);
CREATE UNIQUE INDEX notifications_once ON notifications (employee_id, dedupe_key) WHERE dedupe_key IS NOT NULL;

CREATE TABLE announcements (
    id         BIGSERIAL PRIMARY KEY,
    title      TEXT NOT NULL,
    body       TEXT NOT NULL,
    office     TEXT NOT NULL DEFAULT '',
    created_by BIGINT NOT NULL REFERENCES employees(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE announcement_acks (
    announcement_id BIGINT NOT NULL REFERENCES announcements(id) ON DELETE CASCADE,
    employee_id     BIGINT NOT NULL REFERENCES employees(id),
    acked_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (announcement_id, employee_id)
);

CREATE TABLE accomplishment_reports (
    id          BIGSERIAL PRIMARY KEY,
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    period_from DATE NOT NULL,
    period_to   DATE NOT NULL CHECK (period_to >= period_from),
    body        TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'submitted' CHECK (status IN ('submitted','accepted','returned')),
    reviewed_by BIGINT REFERENCES employees(id),
    reviewed_at TIMESTAMPTZ,
    note        TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE events (
    id         BIGSERIAL PRIMARY KEY,
    title      TEXT NOT NULL,
    place      TEXT NOT NULL DEFAULT '',
    office     TEXT NOT NULL DEFAULT '',
    starts_at  TIMESTAMPTZ NOT NULL,
    ends_at    TIMESTAMPTZ NOT NULL CHECK (ends_at > starts_at),
    required   BOOLEAN NOT NULL DEFAULT TRUE,
    created_by BIGINT NOT NULL REFERENCES employees(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE event_attendance (
    event_id    BIGINT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    method      TEXT NOT NULL CHECK (method IN ('photo','marked')),
    photo_key   TEXT NOT NULL DEFAULT '',
    marked_by   BIGINT REFERENCES employees(id),
    at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (event_id, employee_id)
);

CREATE TABLE approval_delegations (
    id         BIGSERIAL PRIMARY KEY,
    from_id    BIGINT NOT NULL REFERENCES employees(id),
    to_id      BIGINT NOT NULL REFERENCES employees(id),
    from_date  DATE NOT NULL,
    to_date    DATE NOT NULL CHECK (to_date >= from_date),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (from_id <> to_id)
);

ALTER TABLE requests
    ADD COLUMN source          TEXT NOT NULL DEFAULT 'employee' CHECK (source IN ('employee','logbook')),
    ADD COLUMN attachment_key  TEXT NOT NULL DEFAULT '',
    ADD COLUMN attachment_type TEXT NOT NULL DEFAULT '',
    ADD COLUMN escalated_at    TIMESTAMPTZ;
