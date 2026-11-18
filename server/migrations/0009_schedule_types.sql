-- Schedule types. Schedules are never edited after creation, so dated assignments
-- keep past DTRs on the rules that applied then.

ALTER TABLE schedules DROP CONSTRAINT schedules_check;
ALTER TABLE schedules
    ADD COLUMN flexi            BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN grace_minutes    INT NOT NULL DEFAULT 0 CHECK (grace_minutes BETWEEN 0 AND 120),
    ADD COLUMN required_minutes INT NOT NULL DEFAULT 480 CHECK (required_minutes BETWEEN 60 AND 1440),
    ADD COLUMN latest_in        TIME,
    ADD COLUMN work_days        INT NOT NULL DEFAULT 31 CHECK (work_days BETWEEN 1 AND 127), -- bit 0 is Monday
    ADD COLUMN two_logs         BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN overnight        BOOLEAN NOT NULL DEFAULT FALSE;

-- Work suspended for everyone or one office, for the whole day or from a time.
CREATE TABLE work_suspensions (
    id         BIGSERIAL PRIMARY KEY,
    day        DATE NOT NULL,
    from_time  TIME,
    office     TEXT NOT NULL DEFAULT '',
    reason     TEXT NOT NULL,
    created_by BIGINT REFERENCES employees(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (day, office)
);
