CREATE TABLE IF NOT EXISTS restore_points (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    company_name TEXT NOT NULL,
    owner TEXT NOT NULL,
    lifecycle_stage TEXT NOT NULL CHECK (lifecycle_stage IN ('planning', 'active', 'maintenance', 'retired')),
    status TEXT NOT NULL CHECK (status IN ('healthy', 'warning', 'critical', 'disabled')),
    latest_backup_time TIMESTAMPTZ NOT NULL,
    rpo_minutes INTEGER NOT NULL CHECK (rpo_minutes > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_restore_points_latest_backup_time ON restore_points (latest_backup_time DESC);
CREATE INDEX IF NOT EXISTS idx_restore_points_company_name ON restore_points (company_name);
CREATE INDEX IF NOT EXISTS idx_restore_points_owner ON restore_points (owner);
CREATE INDEX IF NOT EXISTS idx_restore_points_lifecycle_stage ON restore_points (lifecycle_stage);
CREATE INDEX IF NOT EXISTS idx_restore_points_status ON restore_points (status);

INSERT INTO restore_points (name, company_name, owner, lifecycle_stage, status, latest_backup_time, rpo_minutes)
SELECT 'ERP Primary Restore Point', 'Acme Corp', 'Alice', 'active', 'healthy', NOW() - INTERVAL '20 minutes', 30
WHERE NOT EXISTS (SELECT 1 FROM restore_points WHERE name = 'ERP Primary Restore Point');

INSERT INTO restore_points (name, company_name, owner, lifecycle_stage, status, latest_backup_time, rpo_minutes)
SELECT 'CRM Daily Snapshot', 'Beta Ltd', 'Bob', 'maintenance', 'warning', NOW() - INTERVAL '130 minutes', 60
WHERE NOT EXISTS (SELECT 1 FROM restore_points WHERE name = 'CRM Daily Snapshot');

INSERT INTO restore_points (name, company_name, owner, lifecycle_stage, status, latest_backup_time, rpo_minutes)
SELECT 'Finance Archive', 'Gamma Inc', 'Carol', 'retired', 'disabled', NOW() - INTERVAL '2 days 4 hours', 1440
WHERE NOT EXISTS (SELECT 1 FROM restore_points WHERE name = 'Finance Archive');

INSERT INTO restore_points (name, company_name, owner, lifecycle_stage, status, latest_backup_time, rpo_minutes)
SELECT 'Analytics Hourly Backup', 'Acme Corp', 'David', 'active', 'critical', NOW() - INTERVAL '95 minutes', 90
WHERE NOT EXISTS (SELECT 1 FROM restore_points WHERE name = 'Analytics Hourly Backup');
