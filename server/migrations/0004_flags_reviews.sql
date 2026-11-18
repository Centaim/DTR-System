-- Phase 2/4: anomaly flags and photo audit. Flags only inform; a person reviews each one.

ALTER TABLE attendance_logs ADD COLUMN photo_dhash BIGINT;

CREATE TYPE flag_kind AS ENUM
    ('dark_photo', 'reused_photo', 'similar_photo', 'shared_device', 'clock_skew', 'late_sync');

CREATE TABLE attendance_flags (
    id          BIGSERIAL PRIMARY KEY,
    log_id      BIGINT NOT NULL REFERENCES attendance_logs(id),
    kind        flag_kind NOT NULL,
    detail      TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (log_id, kind)
);

CREATE TYPE review_outcome AS ENUM ('cleared', 'follow_up');

CREATE TABLE log_reviews (
    log_id       BIGINT PRIMARY KEY REFERENCES attendance_logs(id),
    outcome      review_outcome NOT NULL,
    note         TEXT NOT NULL DEFAULT '',
    reviewed_by  BIGINT NOT NULL REFERENCES employees(id),
    reviewed_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX attendance_logs_device ON attendance_logs (device_id, recorded_at) WHERE device_id <> '';
CREATE INDEX attendance_logs_recorded ON attendance_logs (recorded_at DESC);
