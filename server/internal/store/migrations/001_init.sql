CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    username      TEXT        NOT NULL UNIQUE,
    password_hash TEXT        NOT NULL,
    is_admin      BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE accounts (
    id            BIGSERIAL PRIMARY KEY,
    name          TEXT        NOT NULL UNIQUE,
    uid           TEXT        NOT NULL DEFAULT '',
    cookie        TEXT        NOT NULL,
    rate_limit_ms INTEGER     NOT NULL DEFAULT 1500,
    max_length    INTEGER     NOT NULL DEFAULT 40,
    owner_id      BIGINT      NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE bindings (
    id         BIGSERIAL PRIMARY KEY,
    account_id BIGINT      NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    room_id    TEXT        NOT NULL,
    enabled    BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (account_id, room_id)
);

CREATE TABLE memberships (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT      NOT NULL REFERENCES users(id)    ON DELETE CASCADE,
    binding_id  BIGINT      NOT NULL REFERENCES bindings(id) ON DELETE CASCADE,
    permissions TEXT[]      NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, binding_id)
);
-- 注意：查询这个数组必须写成 permissions @> ARRAY['rule:write']。
-- 写成 'rule:write' = ANY(permissions) 语义相同，但那是逐行的数组展开，
-- PostgreSQL 不会改写成可索引形式，本索引对它完全不起作用（实测 20 万行
-- 时前者走 Bitmap Index Scan，后者 Parallel Seq Scan 扫完全表）。
CREATE INDEX memberships_permissions_idx ON memberships USING GIN (permissions);

CREATE TABLE rules (
    id         BIGSERIAL PRIMARY KEY,
    binding_id BIGINT      NOT NULL REFERENCES bindings(id) ON DELETE CASCADE,
    name       TEXT        NOT NULL,
    enabled    BOOLEAN     NOT NULL DEFAULT TRUE,
    position   INTEGER     NOT NULL DEFAULT 0,
    spec       JSONB       NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (binding_id, name)
);

CREATE TABLE cooldown_groups (
    id          BIGSERIAL PRIMARY KEY,
    binding_id  BIGINT  NOT NULL REFERENCES bindings(id) ON DELETE CASCADE,
    name        TEXT    NOT NULL,
    interval_ms INTEGER NOT NULL,
    UNIQUE (binding_id, name)
);

CREATE TABLE kv_store (
    binding_id BIGINT      NOT NULL REFERENCES bindings(id) ON DELETE CASCADE,
    key        TEXT        NOT NULL,
    value      TEXT        NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (binding_id, key)
);

CREATE TABLE block_list (
    id         BIGSERIAL PRIMARY KEY,
    binding_id BIGINT      NOT NULL REFERENCES bindings(id) ON DELETE CASCADE,
    uid        TEXT        NOT NULL,
    username   TEXT        NOT NULL DEFAULT '',
    reason     TEXT        NOT NULL DEFAULT '',
    created_by BIGINT      REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (binding_id, uid)
);

CREATE TABLE activity_logs (
    id          BIGSERIAL PRIMARY KEY,
    account_id  BIGINT      NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    binding_id  BIGINT      REFERENCES bindings(id) ON DELETE SET NULL,
    room_id     TEXT        NOT NULL DEFAULT '',
    kind        TEXT        NOT NULL,
    event_type  TEXT        NOT NULL DEFAULT '',
    action_type TEXT        NOT NULL DEFAULT '',
    rule_name   TEXT        NOT NULL DEFAULT '',
    user_uid    TEXT        NOT NULL DEFAULT '',
    user_name   TEXT        NOT NULL DEFAULT '',
    detail      JSONB,
    occurred_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX activity_logs_account_time_idx ON activity_logs (account_id, occurred_at DESC);
CREATE INDEX activity_logs_binding_time_idx ON activity_logs (binding_id, occurred_at DESC);
CREATE INDEX activity_logs_type_time_idx    ON activity_logs (event_type, occurred_at DESC);
