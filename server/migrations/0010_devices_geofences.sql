-- Mobile controls: approved devices, allowed locations and clock-in channels.

ALTER TYPE flag_kind ADD VALUE IF NOT EXISTS 'out_of_area';
ALTER TYPE flag_kind ADD VALUE IF NOT EXISTS 'no_location';
ALTER TYPE flag_kind ADD VALUE IF NOT EXISTS 'unapproved_device';

-- Bit 1 is the mobile app, bit 2 is the office kiosk.
ALTER TABLE employees ADD COLUMN clock_channels INT NOT NULL DEFAULT 3 CHECK (clock_channels BETWEEN 1 AND 3);

CREATE TABLE geofences (
    id        BIGSERIAL PRIMARY KEY,
    name      TEXT NOT NULL UNIQUE,
    latitude  DOUBLE PRECISION NOT NULL CHECK (latitude BETWEEN -90 AND 90),
    longitude DOUBLE PRECISION NOT NULL CHECK (longitude BETWEEN -180 AND 180),
    radius_m  INT NOT NULL CHECK (radius_m BETWEEN 20 AND 20000),
    office    TEXT NOT NULL DEFAULT ''
);

CREATE TYPE binding_status AS ENUM ('pending', 'approved', 'rejected', 'revoked');

CREATE TABLE device_bindings (
    id           BIGSERIAL PRIMARY KEY,
    employee_id  BIGINT NOT NULL REFERENCES employees(id),
    device_id    TEXT NOT NULL,
    status       binding_status NOT NULL,
    requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    decided_by   BIGINT REFERENCES employees(id),
    decided_at   TIMESTAMPTZ
);
CREATE UNIQUE INDEX device_one_approved ON device_bindings (employee_id) WHERE status = 'approved';
CREATE UNIQUE INDEX device_one_pending ON device_bindings (employee_id, device_id) WHERE status = 'pending';
