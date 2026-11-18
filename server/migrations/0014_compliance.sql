-- New photo flags and a record of each backup, so HR can see backups are working.
ALTER TYPE flag_kind ADD VALUE IF NOT EXISTS 'blurry_photo';
ALTER TYPE flag_kind ADD VALUE IF NOT EXISTS 'no_face';

CREATE TABLE backup_runs (
    id          BIGSERIAL PRIMARY KEY,
    file        TEXT NOT NULL DEFAULT '',
    bytes       BIGINT NOT NULL DEFAULT 0,
    ok          BOOLEAN NOT NULL,
    finished_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
