CREATE TABLE IF NOT EXISTS events (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs (id),
    public_id varchar(21) NOT NULL UNIQUE,
    name varchar(100) NOT NULL,
    status varchar(16) NOT NULL CHECK (status IN ('draft', 'ready', 'closed')),
    checkin_mode varchar(16) NOT NULL DEFAULT 'geo' CHECK (checkin_mode IN ('geo', 'direct')),
    center_lat numeric(9, 6) CHECK (center_lat BETWEEN -90 AND 90),
    center_lng numeric(9, 6) CHECK (center_lng BETWEEN -180 AND 180),
    radius_m integer NOT NULL DEFAULT 400 CHECK (radius_m BETWEEN 100 AND 1000),
    checkin_start timestamptz,
    checkin_end timestamptz,
    allow_multi_win boolean NOT NULL DEFAULT false,
    max_attendees integer NOT NULL CHECK (max_attendees > 0),
    credit_consumed_at timestamptz,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CHECK (checkin_end IS NULL OR checkin_start IS NULL OR checkin_end > checkin_start)
);

CREATE INDEX IF NOT EXISTS idx_events_org_created ON events (org_id, created_at DESC);

CREATE TABLE IF NOT EXISTS attendees (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs (id),
    event_id uuid NOT NULL REFERENCES events (id),
    name varchar(100) NOT NULL,
    dept varchar(100) NOT NULL DEFAULT '',
    phone_last4 char(4) NOT NULL CHECK (phone_last4 ~ '^[0-9]{4}$'),
    status varchar(16) NOT NULL DEFAULT 'pending',
    created_at timestamptz NOT NULL,
    UNIQUE (event_id, name, phone_last4)
);

CREATE INDEX IF NOT EXISTS idx_attendees_event_created ON attendees (event_id, created_at, id);

CREATE TABLE IF NOT EXISTS prizes (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs (id),
    event_id uuid NOT NULL REFERENCES events (id),
    name varchar(100) NOT NULL,
    gift varchar(200) NOT NULL DEFAULT '',
    quota integer NOT NULL CHECK (quota > 0),
    sort_no integer NOT NULL,
    UNIQUE (event_id, sort_no)
);

ALTER TABLE credit_ledger
    DROP CONSTRAINT IF EXISTS credit_ledger_event_id_fkey;
ALTER TABLE credit_ledger
    ADD CONSTRAINT credit_ledger_event_id_fkey FOREIGN KEY (event_id) REFERENCES events (id);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_credit_ledger_event_consumed
    ON credit_ledger (event_id) WHERE event_id IS NOT NULL AND delta < 0;
