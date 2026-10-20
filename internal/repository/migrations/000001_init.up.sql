CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS apps (
    id            BIGSERIAL PRIMARY KEY,
    slug          VARCHAR(100) UNIQUE NOT NULL,
    name          VARCHAR(200) NOT NULL,
    description   TEXT,
    source_type   VARCHAR(20) NOT NULL DEFAULT 'github',
    repo_url      VARCHAR(500),
    branch        VARCHAR(100) DEFAULT 'main',
    buildpack     VARCHAR(200),
    port          INTEGER DEFAULT 3000,
    container_id  VARCHAR(100),
    image_tag     VARCHAR(200),
    status        VARCHAR(30) NOT NULL DEFAULT 'pending',
    last_err      TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at    TIMESTAMPTZ
);
CREATE INDEX idx_apps_deleted_at ON apps(deleted_at);
CREATE INDEX idx_apps_status     ON apps(status);

CREATE TABLE IF NOT EXISTS deployments (
    id            BIGSERIAL PRIMARY KEY,
    app_id        BIGINT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    commit_sha    VARCHAR(64),
    commit_msg    TEXT,
    image_tag     VARCHAR(200),
    status        VARCHAR(30) NOT NULL DEFAULT 'pending',
    build_log     TEXT,
    error_msg     TEXT,
    started_at    TIMESTAMPTZ,
    finished_at   TIMESTAMPTZ,
    duration_sec  INTEGER DEFAULT 0,
    triggered_by  VARCHAR(100),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_deployments_app    ON deployments(app_id);
CREATE INDEX idx_deployments_status ON deployments(status);

CREATE TABLE IF NOT EXISTS domains (
    id              BIGSERIAL PRIMARY KEY,
    app_id          BIGINT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    hostname        VARCHAR(255) UNIQUE NOT NULL,
    is_primary      BOOLEAN DEFAULT FALSE,
    ssl_enabled     BOOLEAN DEFAULT TRUE,
    cf_zone_id      VARCHAR(64),
    cf_record_id    VARCHAR(64),
    cf_tunnel_id    VARCHAR(64),
    cf_tunnel_name  VARCHAR(128),
    cf_dns_record   VARCHAR(64),
    status          VARCHAR(30) NOT NULL DEFAULT 'pending',
    last_error      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_domains_app      ON domains(app_id);
CREATE INDEX idx_domains_hostname ON domains(hostname);
CREATE INDEX idx_domains_status   ON domains(status);

CREATE TABLE IF NOT EXISTS env_vars (
    id         BIGSERIAL PRIMARY KEY,
    app_id     BIGINT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    key        VARCHAR(200) NOT NULL,
    value      TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (app_id, key)
);
CREATE INDEX idx_envvars_app ON env_vars(app_id);