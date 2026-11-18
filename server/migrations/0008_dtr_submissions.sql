-- DTR certification: each certification is a frozen, numbered version of the month.

CREATE TYPE dtr_status AS ENUM ('submitted', 'verified', 'returned', 'reopened');

CREATE TABLE dtr_submissions (
    id           BIGSERIAL PRIMARY KEY,
    employee_id  BIGINT NOT NULL REFERENCES employees(id),
    month        DATE NOT NULL,
    version      INT NOT NULL,
    snapshot     JSONB NOT NULL,
    sha256       TEXT NOT NULL,
    verify_code  TEXT NOT NULL UNIQUE,
    status       dtr_status NOT NULL DEFAULT 'submitted',
    certified_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    decided_by   BIGINT REFERENCES employees(id),
    decided_at   TIMESTAMPTZ,
    note         TEXT NOT NULL DEFAULT '',
    UNIQUE (employee_id, month, version)
);

-- Only one version of a month can be awaiting review or verified at a time.
CREATE UNIQUE INDEX dtr_one_live ON dtr_submissions (employee_id, month)
    WHERE status IN ('submitted', 'verified');
CREATE INDEX dtr_submissions_month ON dtr_submissions (month, status);
