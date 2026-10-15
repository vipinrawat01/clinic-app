-- Transport coordination (see the SAMWMC brief's Use Case 1: pickup
-- point/time coordinated inside a transport WhatsApp group, tied to the
-- appointment). Columns on appointments directly rather than a separate
-- table — this is one-to-one, simple pickup logistics for one visit, not
-- an entity with its own lifecycle worth modeling separately.
ALTER TABLE appointments ADD COLUMN transport_pickup_point TEXT NOT NULL DEFAULT '';
ALTER TABLE appointments ADD COLUMN transport_pickup_time TIMESTAMPTZ;
-- Set the moment the pickup details are actually posted to the
-- transport WhatsApp group — lets staff edit pickup_point/pickup_time
-- again later (a change of plan) and re-post deliberately, while making
-- "has this already been sent" a real, checkable fact instead of
-- inferred from whether the fields are non-empty.
ALTER TABLE appointments ADD COLUMN transport_posted_at TIMESTAMPTZ;
