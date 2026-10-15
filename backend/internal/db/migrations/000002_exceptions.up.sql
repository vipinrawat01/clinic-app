-- Operational exceptions: a durable record every time the periodic sweep
-- (see internal/exceptions) detects something staff should notice —
-- a no-show, a consult running long enough to threaten the next
-- appointment, etc. Kept as rows (not just logged/computed on each
-- dashboard read) so: (a) the dashboard query is a cheap SELECT instead
-- of re-deriving every exception on every page load, (b) resolving one is
-- a real, auditable action staff take, and (c) this is the shape a future
-- proactive-notification feature (push to staff WhatsApp/a group) would
-- read from without any rework — see this table's own history for why
-- that wasn't built yet.
CREATE TABLE operational_exceptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind TEXT NOT NULL, -- no_show | delay_cascade
    appointment_id UUID NOT NULL REFERENCES appointments(id) ON DELETE CASCADE,
    detail TEXT NOT NULL DEFAULT '',
    detected_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ,
    resolved_by_staff_user_id UUID REFERENCES staff_users(id) ON DELETE SET NULL
);

-- At most one OPEN exception of a given kind per appointment — the sweep
-- runs every tick and must not create a fresh duplicate row for the same
-- still-unresolved problem every time it finds it again. A resolved one
-- doesn't count against this (the WHERE clause excludes it), so the same
-- appointment can get a new row of the same kind later if it's ever
-- resolved and then regresses — a partial index, not a plain UNIQUE
-- constraint, is what makes that possible.
CREATE UNIQUE INDEX idx_operational_exceptions_open_unique ON operational_exceptions(kind, appointment_id) WHERE resolved_at IS NULL;
CREATE INDEX idx_operational_exceptions_open ON operational_exceptions(detected_at) WHERE resolved_at IS NULL;
