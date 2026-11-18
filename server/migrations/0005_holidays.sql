-- Holidays are entered by HR. They excuse undertime and absence for everyone.
CREATE TABLE holidays (
    day         DATE PRIMARY KEY,
    name        TEXT NOT NULL,
    created_by  BIGINT REFERENCES employees(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
