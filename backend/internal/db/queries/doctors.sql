-- name: CreateDoctor :one
INSERT INTO doctors (name, specialty, linked_staff_user_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListDoctors :many
SELECT * FROM doctors WHERE is_active = true ORDER BY name ASC;

-- name: GetDoctor :one
SELECT * FROM doctors WHERE id = $1;

-- name: UpdateDoctor :one
UPDATE doctors SET name = $2, specialty = $3, is_active = $4, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteDoctor :exec
DELETE FROM doctors WHERE id = $1;

-- name: CreateDoctorShift :one
INSERT INTO doctor_shifts (doctor_id, day_of_week, start_time, end_time)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListDoctorShifts :many
SELECT * FROM doctor_shifts WHERE doctor_id = $1 ORDER BY day_of_week ASC, start_time ASC;

-- name: DeleteDoctorShift :exec
DELETE FROM doctor_shifts WHERE id = $1 AND doctor_id = $2;
