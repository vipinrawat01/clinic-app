ALTER TABLE operational_exceptions DROP COLUMN IF EXISTS suggested_ends_at;
ALTER TABLE operational_exceptions DROP COLUMN IF EXISTS suggested_starts_at;
ALTER TABLE operational_exceptions DROP COLUMN IF EXISTS affected_appointment_id;
