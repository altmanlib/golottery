CREATE TABLE IF NOT EXISTS schema_migrations (
    version integer PRIMARY KEY,
    applied_at timestamptz NOT NULL
);

CREATE TABLE IF NOT EXISTS settings (
    key varchar(64) PRIMARY KEY,
    value text NOT NULL,
    category varchar(32) NOT NULL DEFAULT '',
    updated_by varchar(64) NOT NULL DEFAULT '',
    updated_at timestamptz NOT NULL
);

CREATE TABLE IF NOT EXISTS api_tokens (
    id uuid PRIMARY KEY,
    principal_type varchar(32) NOT NULL,
    principal_id varchar(64) NOT NULL,
    name varchar(64) NOT NULL,
    token_hash bytea NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    last_used_at timestamptz,
    created_at timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_api_tokens_principal
    ON api_tokens (principal_type, principal_id);

CREATE TABLE IF NOT EXISTS login_attempts (
    id bigserial PRIMARY KEY,
    key varchar(128) NOT NULL,
    created_at timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_login_attempts_key_created
    ON login_attempts (key, created_at);
