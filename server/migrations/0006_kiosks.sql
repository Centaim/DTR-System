-- Phase 4: office kiosks, and PIN lockout for every sign-in path.

ALTER TABLE employees
    ADD COLUMN failed_attempts INT NOT NULL DEFAULT 0,
    ADD COLUMN locked_until    TIMESTAMPTZ;

-- A kiosk is a registered device. Only a hash of its token is stored.
CREATE TABLE kiosks (
    id           BIGSERIAL PRIMARY KEY,
    name         TEXT NOT NULL UNIQUE,
    office       TEXT NOT NULL DEFAULT '',
    token_hash   TEXT NOT NULL UNIQUE,
    active       BOOLEAN NOT NULL DEFAULT TRUE,
    last_seen_at TIMESTAMPTZ,
    created_by   BIGINT REFERENCES employees(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
