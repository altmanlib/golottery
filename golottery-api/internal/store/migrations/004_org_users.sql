CREATE TABLE IF NOT EXISTS org_users (
    id uuid PRIMARY KEY,
    org_id uuid NOT NULL REFERENCES orgs (id),
    email varchar(254) NOT NULL UNIQUE,
    name varchar(100) NOT NULL,
    password_hash text NOT NULL,
    status varchar(16) NOT NULL CHECK (status IN ('active', 'disabled')),
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_org_users_org ON org_users (org_id, created_at);
