-- EIR Database Schema for PostgreSQL
-- Production-grade schema with indexing, constraints, and partitioning support

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm"; -- For text search optimization

-- Equipment table: Main storage for IMEI/equipment records
CREATE TABLE IF NOT EXISTS equipment (
    id BIGSERIAL PRIMARY KEY,
    imei VARCHAR(16) NOT NULL UNIQUE,
    imeisv VARCHAR(16),
    status VARCHAR(20) NOT NULL CHECK (status IN ('WHITELISTED', 'BLACKLISTED', 'GREYLISTED')),
    reason TEXT,
    last_updated TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    last_check_time TIMESTAMP WITH TIME ZONE,
    check_count BIGINT NOT NULL DEFAULT 0,
    added_by VARCHAR(255) NOT NULL,
    metadata JSONB,
    manufacturer_tac VARCHAR(8),
    manufacturer_name VARCHAR(255),

    CONSTRAINT imei_format_check CHECK (imei ~ '^\d{14,16}$')
);

-- Indexes for equipment table
CREATE INDEX idx_equipment_imei ON equipment USING btree (imei);
CREATE INDEX idx_equipment_status ON equipment USING btree (status);
CREATE INDEX idx_equipment_manufacturer_tac ON equipment USING btree (manufacturer_tac);
CREATE INDEX idx_equipment_last_check_time ON equipment USING btree (last_check_time DESC NULLS LAST);
CREATE INDEX idx_equipment_check_count ON equipment USING btree (check_count DESC);
CREATE INDEX idx_equipment_metadata_gin ON equipment USING gin (metadata jsonb_path_ops);

-- Audit log table: Track all equipment check operations
-- This table is partitioned by check_time for efficient querying and maintenance
CREATE TABLE IF NOT EXISTS audit_log (
    id BIGSERIAL,
    imei VARCHAR(16) NOT NULL,
    imeisv VARCHAR(16),
    status VARCHAR(20) NOT NULL,
    check_time TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    origin_host VARCHAR(255),
    origin_realm VARCHAR(255),
    user_name VARCHAR(255),
    supi VARCHAR(255),
    gpsi VARCHAR(255),
    request_source VARCHAR(50) NOT NULL,
    session_id VARCHAR(255),
    result_code INTEGER,

    PRIMARY KEY (id, check_time)
) PARTITION BY RANGE (check_time);

-- Create partitions for audit_log (quarterly partitions)
-- Production systems should automate partition creation

CREATE TABLE IF NOT EXISTS audit_log_2024_q1 PARTITION OF audit_log
    FOR VALUES FROM ('2024-01-01') TO ('2024-04-01');

CREATE TABLE IF NOT EXISTS audit_log_2024_q2 PARTITION OF audit_log
    FOR VALUES FROM ('2024-04-01') TO ('2024-07-01');

CREATE TABLE IF NOT EXISTS audit_log_2024_q3 PARTITION OF audit_log
    FOR VALUES FROM ('2024-07-01') TO ('2024-10-01');

CREATE TABLE IF NOT EXISTS audit_log_2024_q4 PARTITION OF audit_log
    FOR VALUES FROM ('2024-10-01') TO ('2025-01-01');

CREATE TABLE IF NOT EXISTS audit_log_2025_q1 PARTITION OF audit_log
    FOR VALUES FROM ('2025-01-01') TO ('2025-04-01');

CREATE TABLE IF NOT EXISTS audit_log_2025_q2 PARTITION OF audit_log
    FOR VALUES FROM ('2025-04-01') TO ('2025-07-01');

CREATE TABLE IF NOT EXISTS audit_log_2025_q3 PARTITION OF audit_log
    FOR VALUES FROM ('2025-07-01') TO ('2025-10-01');

CREATE TABLE IF NOT EXISTS audit_log_2025_q4 PARTITION OF audit_log
    FOR VALUES FROM ('2025-10-01') TO ('2026-01-01');

CREATE TABLE IF NOT EXISTS audit_log_2026_q1 PARTITION OF audit_log
    FOR VALUES FROM ('2026-01-01') TO ('2026-04-01');

-- Indexes for audit_log partitions (applied to parent table)
CREATE INDEX idx_audit_log_imei ON audit_log USING btree (imei);
CREATE INDEX idx_audit_log_check_time ON audit_log USING btree (check_time DESC);
CREATE INDEX idx_audit_log_status ON audit_log USING btree (status);
CREATE INDEX idx_audit_log_request_source ON audit_log USING btree (request_source);
CREATE INDEX idx_audit_log_supi ON audit_log USING btree (supi) WHERE supi IS NOT NULL;

-- Function to automatically update last_updated timestamp
CREATE OR REPLACE FUNCTION update_last_updated_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.last_updated = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to update last_updated on equipment table
CREATE TRIGGER update_equipment_last_updated
    BEFORE UPDATE ON equipment
    FOR EACH ROW
    EXECUTE FUNCTION update_last_updated_column();

-- Function to increment check count atomically
CREATE OR REPLACE FUNCTION increment_equipment_check_count(p_imei VARCHAR)
RETURNS VOID AS $$
BEGIN
    UPDATE equipment
    SET check_count = check_count + 1,
        last_check_time = NOW()
    WHERE imei = p_imei;
END;
$$ LANGUAGE plpgsql;

-- View for frequently accessed equipment (hot data)
CREATE OR REPLACE VIEW hot_equipment AS
SELECT *
FROM equipment
WHERE last_check_time >= NOW() - INTERVAL '7 days'
ORDER BY check_count DESC;

-- View for equipment statistics
CREATE OR REPLACE VIEW equipment_statistics AS
SELECT
    status,
    COUNT(*) as count,
    AVG(check_count) as avg_check_count,
    MAX(check_count) as max_check_count,
    COUNT(CASE WHEN last_check_time >= NOW() - INTERVAL '24 hours' THEN 1 END) as active_last_24h,
    COUNT(CASE WHEN last_check_time >= NOW() - INTERVAL '7 days' THEN 1 END) as active_last_7d
FROM equipment
GROUP BY status;

-- Grant permissions (adjust for your user)
-- GRANT SELECT, INSERT, UPDATE, DELETE ON equipment TO eir_app;
-- GRANT SELECT, INSERT ON audit_log TO eir_app;
-- GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO eir_app;
