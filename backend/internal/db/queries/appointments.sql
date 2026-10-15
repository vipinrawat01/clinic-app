-- name: CreateAppointment :one
-- Relies on the no_overlapping_active_appointments exclusion constraint
-- (see migration 000001) as the real correctness guarantee against
-- double-booking under concurrent requests — this insert can fail with a
-- constraint violation, which the handler surfaces as "slot no longer
-- available" rather than something the query itself needs to re-check
-- first (a check-then-insert here would still race).
INSERT INTO appointments (patient_id, doctor_id, starts_at, ends_at, reason_for_visit, reported_symptoms, created_by_staff_user_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetAppointment :one
SELECT * FROM appointments WHERE id = $1;

-- name: ListAppointmentsByDoctorAndRange :many
-- The existing bookings a doctor already has in a time range — the
-- booking API subtracts these from the doctor's shift hours (see
-- ListDoctorShifts) to compute real open slots, rather than trusting
-- anyone to track availability by hand. Named range_start/range_end
-- explicitly (rather than letting sqlc infer names from the columns
-- they happen to compare against, which produced misleadingly-named
-- StartsAt/EndsAt params here that don't mean "this appointment's own
-- start/end") so a caller can't mix up which bound is which.
SELECT * FROM appointments
WHERE doctor_id = sqlc.arg(doctor_id)
  AND status IN ('booked', 'arrived', 'in_consult')
  AND starts_at < sqlc.arg(range_end) AND ends_at > sqlc.arg(range_start)
ORDER BY starts_at ASC;

-- name: ListAppointmentsByPatient :many
SELECT * FROM appointments WHERE patient_id = $1 ORDER BY starts_at DESC;

-- name: ListAppointmentsByDateRange :many
-- The clinic's own day view (and what the main platform's ops dashboard
-- would pull for a cross-system picture) — every appointment in a window
-- regardless of doctor, with patient/doctor names joined in since an
-- appointment id alone means nothing to a human looking at a schedule.
SELECT
    a.*, p.full_name AS patient_name, p.phone_number AS patient_phone, d.name AS doctor_name
FROM appointments a
JOIN patients p ON p.id = a.patient_id
JOIN doctors d ON d.id = a.doctor_id
WHERE a.starts_at >= sqlc.arg(range_start) AND a.starts_at < sqlc.arg(range_end)
ORDER BY a.starts_at ASC;

-- name: UpdateAppointmentStatus :one
UPDATE appointments SET status = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: RecordAppointmentArrival :one
UPDATE appointments SET status = 'arrived', arrived_at = now(), updated_at = now()
WHERE id = $1
RETURNING *;

-- name: RecordConsultStart :one
UPDATE appointments SET status = 'in_consult', consult_started_at = now(), updated_at = now()
WHERE id = $1
RETURNING *;

-- name: RecordConsultEnd :one
UPDATE appointments SET status = 'completed', consult_ended_at = now(), updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CancelAppointment :one
UPDATE appointments SET status = 'cancelled', updated_at = now()
WHERE id = $1
RETURNING *;

-- name: MarkAppointmentNoShow :one
UPDATE appointments SET status = 'no_show', updated_at = now()
WHERE id = $1
RETURNING *;

-- name: RescheduleAppointment :one
-- Same exclusion-constraint protection as CreateAppointment — this UPDATE
-- can also fail with a constraint violation if the new time overlaps
-- another active appointment for this doctor.
UPDATE appointments SET starts_at = $2, ends_at = $3, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: SetTransportPickup :one
-- Staff sets/edits the pickup point and time — a plain field save, not
-- itself a send. The handler decides separately whether to call
-- internal/transport after this succeeds (see MarkTransportPosted).
UPDATE appointments SET transport_pickup_point = $2, transport_pickup_time = $3, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: MarkTransportPosted :one
-- Set the moment pickup details are actually sent to the transport
-- coordinator — a real, checkable fact ("has this appointment's
-- transport info been posted") rather than inferred from the pickup
-- fields merely being non-empty, since staff can edit pickup details
-- again later (a change of plan) without that re-triggering a send on
-- its own.
UPDATE appointments SET transport_posted_at = now()
WHERE id = $1
RETURNING *;
