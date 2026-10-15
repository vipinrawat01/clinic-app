-- name: DetectNoShows :many
-- An appointment is a no-show candidate once it's still 'booked' (never
-- arrived) and grace_minutes past its own start time — long enough that
-- it's clearly not just a patient running a few minutes late, short
-- enough that staff still have same-day time to react (offer the slot to
-- someone else, follow up with the patient).
SELECT id AS appointment_id FROM appointments
WHERE status = 'booked' AND starts_at < now() - (sqlc.arg(grace_minutes)::int * interval '1 minute');

-- name: DetectDelayCascades :many
-- A consult still running past its own scheduled end time, for a doctor
-- who has another appointment starting within warning_minutes, is a real
-- "the next patient is about to be kept waiting" signal — the exact
-- bottleneck the brief's Feature 3 asks to surface, not just "this one
-- consult is long" in isolation. Returns the AT-RISK appointment's own
-- id/duration alongside the overrunning one, so the caller can compute a
-- suggested new time for the at-risk appointment (push it back to start
-- when the overrunning consult is still found running, preserving its
-- original duration) — see CreateOperationalException's affected_* args.
SELECT
    a.id AS appointment_id,
    nxt.id AS affected_appointment_id,
    EXTRACT(EPOCH FROM (nxt.ends_at - nxt.starts_at))::int AS affected_duration_seconds
FROM appointments a
JOIN appointments nxt ON nxt.doctor_id = a.doctor_id
    AND nxt.status = 'booked'
    AND nxt.starts_at BETWEEN now() AND now() + (sqlc.arg(warning_minutes)::int * interval '1 minute')
WHERE a.status = 'in_consult' AND a.ends_at < now();

-- name: CreateOperationalException :exec
-- ON CONFLICT DO NOTHING against idx_operational_exceptions_open_unique —
-- the sweep runs every tick and will keep re-detecting the same
-- still-unresolved problem every time; this is what makes re-detecting
-- it a no-op instead of a duplicate row, without the caller needing to
-- check existence first. affected_appointment_id/suggested_starts_at/
-- suggested_ends_at are only meaningful for delay_cascade (NULL for
-- no_show, which has nothing to reschedule) — see ApplyRescheduleSuggestion.
INSERT INTO operational_exceptions (kind, appointment_id, detail, affected_appointment_id, suggested_starts_at, suggested_ends_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (kind, appointment_id, affected_appointment_id) WHERE (resolved_at IS NULL) DO NOTHING;

-- name: ListOpenExceptions :many
-- affected_* are joined from a SECOND appointment (left-joined — only
-- present for delay_cascade, NULL for no_show) so a reschedule
-- suggestion's own patient/doctor display name is available without a
-- second round-trip from the dashboard.
SELECT
    oe.id, oe.kind, oe.detail, oe.detected_at,
    oe.affected_appointment_id, oe.suggested_starts_at, oe.suggested_ends_at,
    a.id AS appointment_id, a.starts_at, a.status AS appointment_status,
    p.full_name AS patient_name,
    d.name AS doctor_name,
    affected_p.full_name AS affected_patient_name
FROM operational_exceptions oe
JOIN appointments a ON a.id = oe.appointment_id
JOIN patients p ON p.id = a.patient_id
JOIN doctors d ON d.id = a.doctor_id
LEFT JOIN appointments affected_a ON affected_a.id = oe.affected_appointment_id
LEFT JOIN patients affected_p ON affected_p.id = affected_a.patient_id
WHERE oe.resolved_at IS NULL
ORDER BY oe.detected_at DESC;

-- name: GetExceptionForReschedule :one
-- Fetched first so the handler has the suggestion's own stored
-- affected_appointment_id/suggested_starts_at/suggested_ends_at to apply
-- — see ApplyRescheduleSuggestion's doc for why this is two statements
-- rather than one combined query.
SELECT * FROM operational_exceptions WHERE id = $1 AND kind = 'delay_cascade' AND resolved_at IS NULL;

-- name: ApplyRescheduleSuggestion :one
-- Moves the SUGGESTED appointment (not the exception's own
-- appointment_id — see this table's column doc) to exactly the new time
-- already shown to staff when they reviewed it. Deliberately a plain
-- UPDATE by appointment id + exact starts_at/ends_at params (not a
-- sub-query reading the exception row inline) — simpler to reason about
-- and test than one combined statement, at the cost of the handler
-- making two round-trips (GetExceptionForReschedule then this) instead
-- of one; this table is never written at a volume where that matters.
UPDATE appointments SET starts_at = $2, ends_at = $3, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ResolveRescheduledException :exec
-- Marks the exception resolved as a direct consequence of
-- ApplyRescheduleSuggestion's real action — not waiting for the
-- separate sweep-driven AutoResolveDelayCascades, which would also catch
-- this on its own next tick, but resolving here immediately avoids a
-- window where the UI still shows an already-actioned suggestion as open.
UPDATE operational_exceptions SET resolved_at = now() WHERE id = $1;

-- name: AutoResolveNoShows :exec
-- An open no_show exception clears itself the moment its appointment
-- actually leaves 'booked' (staff marked it arrived/no_show/cancelled
-- from the schedule) — deliberately the ONLY way a no_show exception
-- can go away. There is no separate "dismiss" action: dismissing the
-- alert without acting on the appointment would let the exceptions
-- panel and the real schedule disagree about reality, which is strictly
-- worse than an alert that keeps showing until someone actually deals
-- with it. resolved_by_staff_user_id stays NULL here (no single staff
-- action caused this specifically; the appointment-status handler that
-- changed the appointment is the actual auditable action).
UPDATE operational_exceptions SET resolved_at = now()
WHERE kind = 'no_show' AND resolved_at IS NULL
  AND appointment_id IN (SELECT id FROM appointments WHERE status != 'booked');

-- name: AutoResolveDelayCascades :exec
-- A delay_cascade exception clears once the consult that was running
-- long is no longer 'in_consult' (ended) OR the doctor's next
-- appointment that was at risk is no longer imminent/booked (it got
-- rescheduled, cancelled, or its own time window simply passed) — same
-- "only the real condition clearing resolves it" principle as no-shows.
UPDATE operational_exceptions SET resolved_at = now()
WHERE kind = 'delay_cascade' AND resolved_at IS NULL
  AND appointment_id IN (
    SELECT a.id FROM appointments a
    WHERE a.status != 'in_consult'
       OR NOT EXISTS (
         SELECT 1 FROM appointments nxt
         WHERE nxt.doctor_id = a.doctor_id AND nxt.status = 'booked'
           AND nxt.starts_at BETWEEN now() AND now() + (sqlc.arg(warning_minutes)::int * interval '1 minute')
       )
  );
