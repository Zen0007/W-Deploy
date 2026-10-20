ALTER TABLE apps
    ADD COLUMN IF NOT EXISTS github_full_name VARCHAR(300),
    ADD COLUMN IF NOT EXISTS webhook_id       BIGINT  NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS auto_deploy      BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN IF NOT EXISTS owner_login      VARCHAR(100);
CREATE INDEX IF NOT EXISTS idx_apps_github_full_name ON apps(github_full_name);
CREATE INDEX IF NOT EXISTS idx_apps_owner_login      ON apps(owner_login);

CREATE TABLE IF NOT EXISTS user_tokens (
    id           BIGSERIAL PRIMARY KEY,
    github_id    BIGINT       NOT NULL UNIQUE,
    login        VARCHAR(100) NOT NULL UNIQUE,
    avatar_url   VARCHAR(500),
    name         VARCHAR(200),
    access_token TEXT         NOT NULL, -- terenkripsi AES-GCM (base64)
    token_type   VARCHAR(30),
    scope        VARCHAR(300),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS app_databases (
    id             BIGSERIAL PRIMARY KEY,
    app_id         BIGINT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    engine         VARCHAR(20) NOT NULL,
    container_id   VARCHAR(100),
    container_name VARCHAR(200),
    volume_name    VARCHAR(200),
    host           VARCHAR(200),
    port           INTEGER,
    database       VARCHAR(200),
    username       VARCHAR(200),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_app_databases_app ON app_databases(app_id);
