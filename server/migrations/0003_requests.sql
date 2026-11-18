-- Phase 3: corrections and excused days, with approval. Originals are never edited:
-- an approved missed_log request is merged into the DTR as an adjustment.

CREATE TYPE request_kind AS ENUM ('missed_log', 'official_business', 'travel_order', 'leave');
CREATE TYPE request_status AS ENUM ('pending', 'approved', 'rejected', 'cancelled');

CREATE TABLE requests (
    id             BIGSERIAL PRIMARY KEY,
    employee_id    BIGINT NOT NULL REFERENCES employees(id),
    kind           request_kind NOT NULL,
    work_date      DATE NOT NULL,
    date_to        DATE,
    slot           log_slot,
    requested_time TIME,
    reason         TEXT NOT NULL,
    status         request_status NOT NULL DEFAULT 'pending',
    decided_by     BIGINT REFERENCES employees(id),
    decided_at     TIMESTAMPTZ,
    decision_note  TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (kind <> 'missed_log' OR (slot IS NOT NULL AND requested_time IS NOT NULL)),
    CHECK (date_to IS NULL OR date_to >= work_date)
);

-- One live correction per employee, day and slot.
CREATE UNIQUE INDEX requests_one_live_missed_log
    ON requests (employee_id, work_date, slot)
    WHERE kind = 'missed_log' AND status IN ('pending', 'approved');

CREATE INDEX requests_employee ON requests (employee_id, work_date);
CREATE INDEX requests_pending ON requests (status) WHERE status = 'pending';
