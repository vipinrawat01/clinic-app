-- name: CreatePatient :one
INSERT INTO patients (
    external_contact_ref, full_name, date_of_birth, phone_number, residential_address,
    company_name, supervisor_name, supervisor_phone, known_allergies, preferred_language, consent_given_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING *;

-- name: GetPatient :one
SELECT * FROM patients WHERE id = $1;

-- name: GetPatientByExternalContactRef :one
-- Looked up by the main chatbot platform's own contact identity (a
-- WhatsApp phone number, etc) so a booking made through a conversation
-- can be matched to this same patient record without duplicating their
-- details — the integration seam between the two systems.
SELECT * FROM patients WHERE external_contact_ref = $1;

-- name: ListPatients :many
SELECT * FROM patients ORDER BY full_name ASC LIMIT $1 OFFSET $2;

-- name: SearchPatients :many
SELECT * FROM patients
WHERE full_name ILIKE '%' || sqlc.arg(query)::text || '%' OR phone_number ILIKE '%' || sqlc.arg(query)::text || '%'
ORDER BY full_name ASC
LIMIT 20;

-- name: UpdatePatient :one
UPDATE patients SET
    full_name = $2, date_of_birth = $3, phone_number = $4, residential_address = $5,
    company_name = $6, supervisor_name = $7, supervisor_phone = $8, known_allergies = $9,
    preferred_language = $10, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: SetPatientConsent :one
UPDATE patients SET consent_given_at = now(), updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeletePatient :exec
DELETE FROM patients WHERE id = $1;
