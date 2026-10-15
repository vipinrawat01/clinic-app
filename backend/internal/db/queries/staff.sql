-- name: CreateStaffUser :one
INSERT INTO staff_users (email, name, password_hash, role)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetStaffUserByEmail :one
SELECT * FROM staff_users WHERE email = $1 AND is_active = true;

-- name: GetStaffUser :one
SELECT * FROM staff_users WHERE id = $1;

-- name: ListStaffUsers :many
SELECT * FROM staff_users ORDER BY created_at ASC;

-- name: UpdateStaffUser :one
UPDATE staff_users SET name = $2, role = $3, is_active = $4, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteStaffUser :exec
DELETE FROM staff_users WHERE id = $1;

-- name: SetStaffUserPassword :one
UPDATE staff_users SET password_hash = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CountActiveAdmins :one
-- Used before demoting/deactivating an admin to make sure at least one
-- other active admin still exists afterward — see
-- handlers.StaffHandler.Update's doc on preventing a self-lockout with
-- no account left able to grant admin access back.
SELECT count(*) FROM staff_users WHERE role = 'admin' AND is_active = true;
