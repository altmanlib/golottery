CREATE TABLE IF NOT EXISTS guest_sessions (
    id uuid PRIMARY KEY,
    event_id uuid NOT NULL REFERENCES events (id),
    openid varchar(64) NOT NULL,
    token_hash bytea NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL,
    UNIQUE (event_id, openid)
);

ALTER TABLE attendees ADD COLUMN IF NOT EXISTS openid varchar(64);
ALTER TABLE attendees ADD COLUMN IF NOT EXISTS checkin_at timestamptz;
ALTER TABLE attendees ADD COLUMN IF NOT EXISTS checkin_method varchar(16)
    CHECK (checkin_method IN ('geo', 'direct', 'manual', 'proxy'));
ALTER TABLE attendees ADD COLUMN IF NOT EXISTS checkin_by varchar(64);
ALTER TABLE attendees DROP CONSTRAINT IF EXISTS attendees_status_check;
ALTER TABLE attendees ADD CONSTRAINT attendees_status_check CHECK (status IN ('pending', 'checked_in'));
CREATE UNIQUE INDEX IF NOT EXISTS uniq_attendees_event_openid ON attendees (event_id, openid) WHERE openid IS NOT NULL;

CREATE TABLE IF NOT EXISTS checkin_attempts (
    id bigserial PRIMARY KEY,
    event_id uuid NOT NULL REFERENCES events (id),
    attendee_id uuid REFERENCES attendees (id) ON DELETE SET NULL,
    openid varchar(64) NOT NULL,
    lat numeric(9, 6),
    lng numeric(9, 6),
    accuracy_m numeric(8, 1),
    distance_m numeric(10, 1),
    result varchar(16) NOT NULL CHECK (result IN ('passed', 'rejected')),
    reason varchar(32) NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_checkin_attempts_event ON checkin_attempts (event_id, created_at);

CREATE TABLE IF NOT EXISTS manual_requests (
    id uuid PRIMARY KEY,
    event_id uuid NOT NULL REFERENCES events (id),
    openid varchar(64) NOT NULL,
    attendee_id uuid REFERENCES attendees (id) ON DELETE SET NULL,
    claimed_name varchar(100) NOT NULL DEFAULT '',
    claimed_phone_last4 varchar(4) NOT NULL DEFAULT '',
    reason varchar(200) NOT NULL DEFAULT '',
    status varchar(16) NOT NULL CHECK (status IN ('pending', 'approved', 'rejected')),
    handled_by varchar(64),
    handled_at timestamptz,
    created_at timestamptz NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_manual_requests_pending
    ON manual_requests (event_id, openid) WHERE status = 'pending';

CREATE TABLE IF NOT EXISTS event_staff (
    id uuid PRIMARY KEY,
    event_id uuid NOT NULL REFERENCES events (id),
    openid varchar(64) NOT NULL,
    role varchar(16) NOT NULL CHECK (role IN ('staff', 'admin')),
    created_at timestamptz NOT NULL,
    UNIQUE (event_id, openid)
);

CREATE TABLE IF NOT EXISTS staff_invites (
    id uuid PRIMARY KEY,
    event_id uuid NOT NULL REFERENCES events (id),
    role varchar(16) NOT NULL CHECK (role IN ('staff', 'admin')),
    code_hash bytea NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    used_by varchar(64),
    used_at timestamptz,
    created_at timestamptz NOT NULL
);
