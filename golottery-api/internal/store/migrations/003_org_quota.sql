CREATE TABLE IF NOT EXISTS orgs (
    id uuid PRIMARY KEY,
    name varchar(100) NOT NULL,
    contact varchar(100) NOT NULL DEFAULT '',
    status varchar(16) NOT NULL CHECK (status IN ('active', 'disabled')),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_orgs_created ON orgs (created_at DESC, id);

CREATE TABLE IF NOT EXISTS org_quotas (
    org_id uuid PRIMARY KEY REFERENCES orgs (id) ON DELETE CASCADE,
    event_credits integer NOT NULL CHECK (event_credits >= 0),
    max_attendees integer NOT NULL CHECK (max_attendees > 0),
    updated_at timestamptz NOT NULL
);

CREATE TABLE IF NOT EXISTS credit_ledger (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs (id),
    delta integer NOT NULL CHECK (delta <> 0),
    balance_after integer NOT NULL CHECK (balance_after >= 0),
    reason varchar(200) NOT NULL,
    event_id uuid,
    operator_type varchar(32) NOT NULL,
    operator_id varchar(64) NOT NULL,
    created_at timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_credit_ledger_org_created ON credit_ledger (org_id, created_at DESC);
