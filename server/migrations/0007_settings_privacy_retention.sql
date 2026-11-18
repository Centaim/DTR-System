-- Agency settings, photo retention, legal holds and privacy notice acceptance.

CREATE TABLE settings (
    key        TEXT PRIMARY KEY,
    value      INT NOT NULL,
    updated_by BIGINT REFERENCES employees(id),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- attendance_logs is append-only, so a purged photo is recorded beside the log.
CREATE TABLE photo_purges (
    log_id    BIGINT PRIMARY KEY REFERENCES attendance_logs(id),
    purged_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE legal_holds (
    id          BIGSERIAL PRIMARY KEY,
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    reason      TEXT NOT NULL,
    created_by  BIGINT NOT NULL REFERENCES employees(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    released_by BIGINT REFERENCES employees(id),
    released_at TIMESTAMPTZ
);
CREATE UNIQUE INDEX legal_holds_one_active ON legal_holds (employee_id) WHERE released_at IS NULL;

CREATE TABLE privacy_acceptances (
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    version     INT NOT NULL,
    channel     TEXT NOT NULL,
    accepted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (employee_id, version)
);
