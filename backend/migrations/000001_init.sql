CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    username VARCHAR(80) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    email VARCHAR(180),
    role VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL,
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uni_users_username UNIQUE (username)
);
CREATE INDEX IF NOT EXISTS idx_users_tenant_id ON users(tenant_id);
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);

CREATE TABLE IF NOT EXISTS groups (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    resource_type VARCHAR(40) NOT NULL,
    name VARCHAR(100) NOT NULL,
    description VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_groups_tenant_id ON groups(tenant_id);
CREATE INDEX IF NOT EXISTS idx_groups_resource_type ON groups(resource_type);

CREATE TABLE IF NOT EXISTS terminals (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    phone VARCHAR(40),
    nickname VARCHAR(120),
    avatar_url VARCHAR(500),
    bio VARCHAR(500),
    homepage VARCHAR(255),
    status VARCHAR(30),
    last_online_at TIMESTAMPTZ,
    access_type VARCHAR(20),
    origin_country VARCHAR(80),
    origin_flag VARCHAR(16),
    exit_ip VARCHAR(80),
    exit_country VARCHAR(80),
    exit_flag VARCHAR(16),
    group_id UUID,
    today_success BIGINT DEFAULT 0,
    total_success BIGINT DEFAULT 0,
    today_failed BIGINT DEFAULT 0,
    total_failed BIGINT DEFAULT 0,
    risk_status VARCHAR(30),
    ban_status VARCHAR(30),
    file_path VARCHAR(500),
    session_hash VARCHAR(128),
    sleep_until TIMESTAMPTZ,
    last_message_at TIMESTAMPTZ,
    dm_hourly_limit INT DEFAULT 0,
    dm_daily_limit INT DEFAULT 0,
    join_hourly_limit INT DEFAULT 0,
    join_daily_limit INT DEFAULT 0,
    dm_hourly_count INT DEFAULT 0,
    dm_daily_count INT DEFAULT 0,
    join_hourly_count INT DEFAULT 0,
    join_daily_count INT DEFAULT 0,
    dm_hourly_reset_at TIMESTAMPTZ,
    dm_daily_reset_at TIMESTAMPTZ,
    join_hourly_reset_at TIMESTAMPTZ,
    join_daily_reset_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_terminals_tenant_id ON terminals(tenant_id);
CREATE INDEX IF NOT EXISTS idx_terminals_phone ON terminals(phone);
CREATE INDEX IF NOT EXISTS idx_terminals_group_id ON terminals(group_id);
CREATE INDEX IF NOT EXISTS idx_terminals_session_hash ON terminals(session_hash);

CREATE TABLE IF NOT EXISTS terminal_target_restrictions (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    terminal_id UUID NOT NULL,
    action VARCHAR(20) NOT NULL,
    target_type VARCHAR(30),
    target_value VARCHAR(255),
    target_key VARCHAR(320) NOT NULL,
    reason VARCHAR(500),
    fail_count INT DEFAULT 0,
    cooldown_until TIMESTAMPTZ,
    last_failed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_terminal_target_restrictions_lookup ON terminal_target_restrictions(tenant_id, terminal_id, action, target_key);
CREATE INDEX IF NOT EXISTS idx_terminal_target_restrictions_cooldown ON terminal_target_restrictions(cooldown_until);

CREATE TABLE IF NOT EXISTS network_nodes (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    code VARCHAR(60),
    ip VARCHAR(80),
    port INT,
    protocol VARCHAR(20),
    username VARCHAR(120),
    password VARCHAR(255),
    latency_ms INT DEFAULT 0,
    country VARCHAR(80),
    flag VARCHAR(16),
    bound_terminals BIGINT DEFAULT 0,
    status VARCHAR(30),
    group_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_network_nodes_tenant_id ON network_nodes(tenant_id);
CREATE INDEX IF NOT EXISTS idx_network_nodes_group_id ON network_nodes(group_id);

CREATE TABLE IF NOT EXISTS targets (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    avatar_url VARCHAR(500),
    identifier VARCHAR(160),
    name VARCHAR(160),
    type VARCHAR(30),
    size BIGINT DEFAULT 0,
    group_id UUID,
    notification_count BIGINT DEFAULT 0,
    linked_terminals BIGINT DEFAULT 0,
    has_verification BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_targets_tenant_id ON targets(tenant_id);
CREATE INDEX IF NOT EXISTS idx_targets_identifier ON targets(identifier);
CREATE INDEX IF NOT EXISTS idx_targets_group_id ON targets(group_id);

CREATE TABLE IF NOT EXISTS assets (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    group_id UUID,
    name VARCHAR(180),
    mime_type VARCHAR(80),
    md5 VARCHAR(64),
    url VARCHAR(500),
    file_path VARCHAR(500),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_assets_tenant_id ON assets(tenant_id);
CREATE INDEX IF NOT EXISTS idx_assets_md5 ON assets(md5);

CREATE TABLE IF NOT EXISTS workflows (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    name VARCHAR(160),
    description VARCHAR(500),
    definition JSONB,
    status VARCHAR(30),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_workflows_tenant_id ON workflows(tenant_id);

CREATE TABLE IF NOT EXISTS tasks (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    name VARCHAR(180),
    type VARCHAR(60) NOT NULL,
    terminal_group_id UUID,
    target_group_id UUID,
    status VARCHAR(30) NOT NULL,
    progress INT DEFAULT 0,
    payload JSONB,
    summary JSONB,
    run_id VARCHAR(80),
    run_locked_at TIMESTAMPTZ,
    created_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_tasks_tenant_id ON tasks(tenant_id);
CREATE INDEX IF NOT EXISTS idx_tasks_type ON tasks(type);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_run_id ON tasks(run_id);
CREATE INDEX IF NOT EXISTS idx_tasks_run_locked_at ON tasks(run_locked_at);
CREATE INDEX IF NOT EXISTS idx_tasks_tenant_created ON tasks(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_tasks_tenant_status_created ON tasks(tenant_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_tasks_tenant_type_created ON tasks(tenant_id, type, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_tasks_tenant_updated ON tasks(tenant_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS task_logs (
    id UUID PRIMARY KEY,
    tenant_id UUID NOT NULL,
    task_id UUID NOT NULL,
    level VARCHAR(20) NOT NULL,
    category VARCHAR(60),
    terminal_ref VARCHAR(160),
    target_ref VARCHAR(160),
    action VARCHAR(120),
    details VARCHAR(5000),
    duration_ms BIGINT DEFAULT 0,
    trace_id VARCHAR(80),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_task_logs_tenant_id ON task_logs(tenant_id);
CREATE INDEX IF NOT EXISTS idx_task_logs_task_id ON task_logs(task_id);
CREATE INDEX IF NOT EXISTS idx_task_logs_created_at ON task_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_task_logs_tenant_created ON task_logs(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_task_logs_tenant_task_created ON task_logs(tenant_id, task_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_task_logs_tenant_level_created ON task_logs(tenant_id, level, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_task_logs_tenant_category_created ON task_logs(tenant_id, category, created_at DESC);
