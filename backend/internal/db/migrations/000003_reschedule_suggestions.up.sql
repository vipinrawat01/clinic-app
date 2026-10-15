-- A delay_cascade exception's appointment_id is the overrunning consult
-- (the CAUSE) — but the thing a reschedule suggestion actually proposes
-- moving is the doctor's NEXT appointment (the one at risk of being kept
-- waiting), a different row entirely. These columns carry that proposal:
-- which appointment would move, and to what new time — computed once at
-- detection time (see internal/exceptions' sweep) so accepting it later
-- is just "apply exactly what was already shown", not a fresh
-- recalculation that could silently differ from what staff reviewed.
ALTER TABLE operational_exceptions ADD COLUMN affected_appointment_id UUID REFERENCES appointments(id) ON DELETE CASCADE;
ALTER TABLE operational_exceptions ADD COLUMN suggested_starts_at TIMESTAMPTZ;
ALTER TABLE operational_exceptions ADD COLUMN suggested_ends_at TIMESTAMPTZ;

-- The original idx_operational_exceptions_open_unique (kind, appointment_id)
-- would silently drop a second delay_cascade row if one overrunning
-- consult threatens TWO different later appointments (ON CONFLICT DO
-- NOTHING on the second insert) — a real second problem going unreported.
-- Widening the uniqueness to include affected_appointment_id fixes this:
-- each at-risk appointment gets its own exception row. no_show has no
-- affected_appointment_id (NULL), and NULL <> NULL in a unique index, so
-- this widening changes nothing for no_show's own one-row-per-appointment
-- behavior.
DROP INDEX idx_operational_exceptions_open_unique;
CREATE UNIQUE INDEX idx_operational_exceptions_open_unique ON operational_exceptions(kind, appointment_id, affected_appointment_id) WHERE resolved_at IS NULL;
