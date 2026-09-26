ALTER TABLE events ADD COLUMN IF NOT EXISTS draw_version bigint NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS host_users (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs (id),
    event_id uuid NOT NULL UNIQUE REFERENCES events (id),
    password_hash text NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE TABLE IF NOT EXISTS draw_logs (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs (id),
    event_id uuid NOT NULL REFERENCES events (id),
    prize_id uuid NOT NULL REFERENCES prizes (id),
    pool_size integer NOT NULL,
    draw_count integer NOT NULL,
    operator_id uuid NOT NULL,
    request_id uuid NOT NULL UNIQUE,
    created_at timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_draw_logs_event_created
    ON draw_logs (event_id, created_at, id);

CREATE TABLE IF NOT EXISTS draw_results (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs (id),
    event_id uuid NOT NULL REFERENCES events (id),
    prize_id uuid NOT NULL REFERENCES prizes (id),
    attendee_id uuid NOT NULL REFERENCES attendees (id),
    request_id uuid NOT NULL REFERENCES draw_logs (request_id),
    status varchar(16) NOT NULL CHECK (status IN ('valid', 'void')),
    exclusive boolean NOT NULL,
    void_reason varchar(200),
    voided_at timestamptz,
    created_at timestamptz NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_draw_results_prize_attendee_valid
    ON draw_results (prize_id, attendee_id) WHERE status = 'valid';

CREATE UNIQUE INDEX IF NOT EXISTS uniq_draw_results_event_attendee_exclusive
    ON draw_results (event_id, attendee_id) WHERE status = 'valid' AND exclusive;

CREATE INDEX IF NOT EXISTS idx_draw_results_event_created
    ON draw_results (event_id, created_at, id);

CREATE INDEX IF NOT EXISTS idx_draw_results_request
    ON draw_results (request_id);
