CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Staff accounts — the people who log into this app (receptionists,
-- doctors acting as users, clinic admins). Single-tenant by design: this
-- whole app is one clinic's own operations tool, not a multi-tenant SaaS
-- like the main chatbot platform, so there's no company_id scoping here.
CREATE TABLE staff_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'receptionist', -- admin | doctor | receptionist
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Doctors as a real clinical entity, not just a label on an appointment —
-- a doctor has their own working hours (doctor_shifts) and appointments
-- reference them directly, so capacity/availability can be computed for
-- real instead of staff tracking it by memory. linked_staff_user_id is
-- optional: a doctor may or may not also be a staff_users login (e.g. a
-- doctor who never logs into this system themselves, just has their
-- roster managed by reception).
CREATE TABLE doctors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    linked_staff_user_id UUID REFERENCES staff_users(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    specialty TEXT NOT NULL DEFAULT '',
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- A doctor's recurring weekly availability — the source of truth the
-- booking API checks against (see appointments below) rather than staff
-- maintaining a separate "who's free when" spreadsheet. day_of_week
-- follows Postgres's own EXTRACT(DOW) convention: 0 = Sunday .. 6 =
-- Saturday, so it composes directly with date arithmetic without an
-- extra lookup table.
CREATE TABLE doctor_shifts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    doctor_id UUID NOT NULL REFERENCES doctors(id) ON DELETE CASCADE,
    day_of_week SMALLINT NOT NULL CHECK (day_of_week BETWEEN 0 AND 6),
    start_time TIME NOT NULL,
    end_time TIME NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (end_time > start_time)
);

CREATE INDEX idx_doctor_shifts_doctor ON doctor_shifts(doctor_id);

-- Patient records — the Annex-1 field set from the SAMWMC brief
-- (name/DOB/phone/address/employer/supervisor/allergies/symptoms), kept
-- as real typed columns rather than a JSONB blob since every one of these
-- is a known, stable, clinically-meaningful field for this one app (this
-- is the deliberate difference from the main platform's agent_variables —
-- that system stays generic because it serves every business; this app
-- is allowed to be exactly this specific, since it only ever serves one
-- clinic). external_contact_ref optionally links back to the main
-- platform's own contact identity (a WhatsApp phone number, etc) so a
-- booking made through the conversational platform can be matched to the
-- same patient record here without duplicating their details.
CREATE TABLE patients (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_contact_ref TEXT, -- e.g. the WhatsApp number used on the main platform
    full_name TEXT NOT NULL,
    date_of_birth DATE,
    phone_number TEXT NOT NULL DEFAULT '',
    residential_address TEXT NOT NULL DEFAULT '',
    company_name TEXT NOT NULL DEFAULT '',
    supervisor_name TEXT NOT NULL DEFAULT '',
    supervisor_phone TEXT NOT NULL DEFAULT '',
    known_allergies TEXT NOT NULL DEFAULT '',
    preferred_language TEXT NOT NULL DEFAULT '',
    consent_given_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_patients_external_contact_ref ON patients(external_contact_ref);
CREATE INDEX idx_patients_phone ON patients(phone_number);

-- The actual bookable unit — one patient, one doctor, one time range.
-- status tracks the real-world lifecycle (not just "booked or not") so
-- the clinic's own ops view (and anything the main platform's dashboard
-- pulls via the API) reflects what's really happening: a booked slot that
-- never shows up is a different operational fact than one that completed
-- normally. reason_for_visit/symptoms mirror the Annex-1 "main reason for
-- seeking medical care" / cough-fever-sorethroat fields — captured per
-- visit, not per patient, since those change every time.
CREATE TABLE appointments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id UUID NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    doctor_id UUID NOT NULL REFERENCES doctors(id) ON DELETE CASCADE,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    status TEXT NOT NULL DEFAULT 'booked', -- booked | arrived | in_consult | completed | no_show | cancelled
    reason_for_visit TEXT NOT NULL DEFAULT '',
    reported_symptoms TEXT NOT NULL DEFAULT '',
    -- Patient-flow timestamps (see the SAMWMC brief's Annex-2 reporting
    -- fields) — captured as the visit actually happens, not
    -- reconstructed afterward from memory.
    arrived_at TIMESTAMPTZ,
    consult_started_at TIMESTAMPTZ,
    consult_ended_at TIMESTAMPTZ,
    created_by_staff_user_id UUID REFERENCES staff_users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (ends_at > starts_at)
);

CREATE INDEX idx_appointments_doctor_time ON appointments(doctor_id, starts_at);
CREATE INDEX idx_appointments_patient ON appointments(patient_id);
CREATE INDEX idx_appointments_status ON appointments(status);
-- A doctor can't be double-booked for an overlapping time range while the
-- appointment is still active (booked/arrived/in_consult) — cancelled/
-- completed/no_show appointments don't hold the slot, so the time can be
-- rebooked. Enforced with a real exclusion constraint (not just
-- application-level checking) so this can never be violated even under
-- concurrent booking requests.
CREATE EXTENSION IF NOT EXISTS btree_gist;
ALTER TABLE appointments ADD CONSTRAINT no_overlapping_active_appointments
    EXCLUDE USING gist (
        doctor_id WITH =,
        tstzrange(starts_at, ends_at) WITH &&
    ) WHERE (status IN ('booked', 'arrived', 'in_consult'));
