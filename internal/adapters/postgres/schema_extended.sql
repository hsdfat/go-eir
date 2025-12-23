-- Extended schema for audit and history tracking
-- This extends the base schema with additional tables for comprehensive tracking

-- Equipment History Table
-- Tracks all changes to equipment records
CREATE TABLE IF NOT EXISTS equipment_history (
    id BIGSERIAL PRIMARY KEY,
    imei VARCHAR(16) NOT NULL,
    change_type VARCHAR(20) NOT NULL CHECK (change_type IN ('CREATE', 'UPDATE', 'DELETE', 'CHECK')),
    changed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    changed_by VARCHAR(255) NOT NULL,
    previous_status VARCHAR(20),
    new_status VARCHAR(20) NOT NULL,
    previous_reason TEXT,
    new_reason TEXT,
    change_details JSONB,
    session_id VARCHAR(255),
    CONSTRAINT fk_equipment_history_imei FOREIGN KEY (imei) REFERENCES equipment(imei) ON DELETE CASCADE
);

-- Create indexes for equipment_history
CREATE INDEX IF NOT EXISTS idx_equipment_history_imei ON equipment_history(imei);
CREATE INDEX IF NOT EXISTS idx_equipment_history_changed_at ON equipment_history(changed_at DESC);
CREATE INDEX IF NOT EXISTS idx_equipment_history_change_type ON equipment_history(change_type);
CREATE INDEX IF NOT EXISTS idx_equipment_history_changed_by ON equipment_history(changed_by);
CREATE INDEX IF NOT EXISTS idx_equipment_history_session_id ON equipment_history(session_id) WHERE session_id IS NOT NULL;

-- Equipment Snapshots Table
-- Stores point-in-time snapshots of equipment state
CREATE TABLE IF NOT EXISTS equipment_snapshots (
    id BIGSERIAL PRIMARY KEY,
    equipment_id BIGINT NOT NULL,
    imei VARCHAR(16) NOT NULL,
    snapshot_time TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(20) NOT NULL,
    reason TEXT,
    check_count BIGINT NOT NULL DEFAULT 0,
    metadata JSONB,
    created_by VARCHAR(255) NOT NULL,
    snapshot_type VARCHAR(20) NOT NULL CHECK (snapshot_type IN ('MANUAL', 'SCHEDULED', 'PRE_UPDATE')),
    CONSTRAINT fk_equipment_snapshots_equipment FOREIGN KEY (equipment_id) REFERENCES equipment(id) ON DELETE CASCADE
);

-- Create indexes for equipment_snapshots
CREATE INDEX IF NOT EXISTS idx_equipment_snapshots_imei ON equipment_snapshots(imei);
CREATE INDEX IF NOT EXISTS idx_equipment_snapshots_snapshot_time ON equipment_snapshots(snapshot_time DESC);
CREATE INDEX IF NOT EXISTS idx_equipment_snapshots_equipment_id ON equipment_snapshots(equipment_id);
CREATE INDEX IF NOT EXISTS idx_equipment_snapshots_type ON equipment_snapshots(snapshot_type);

-- Extended Audit Log Table
-- Extends audit_log with additional metadata
CREATE TABLE IF NOT EXISTS audit_log_extended (
    id BIGSERIAL PRIMARY KEY,
    audit_log_id BIGINT NOT NULL,
    ip_address VARCHAR(45), -- IPv4 or IPv6
    user_agent TEXT,
    additional_data JSONB,
    processing_time_ms BIGINT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_audit_log_extended_audit FOREIGN KEY (audit_log_id) REFERENCES audit_log(id) ON DELETE CASCADE
);

-- Create indexes for audit_log_extended
CREATE INDEX IF NOT EXISTS idx_audit_log_extended_audit_log_id ON audit_log_extended(audit_log_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_extended_ip_address ON audit_log_extended(ip_address) WHERE ip_address IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_audit_log_extended_processing_time ON audit_log_extended(processing_time_ms) WHERE processing_time_ms IS NOT NULL;

-- Function to automatically create snapshots before updates
CREATE OR REPLACE FUNCTION create_equipment_snapshot_before_update()
RETURNS TRIGGER AS $$
BEGIN
    -- Only create snapshot if status is changing
    IF OLD.status IS DISTINCT FROM NEW.status THEN
        INSERT INTO equipment_snapshots (
            equipment_id, imei, snapshot_time, status, reason,
            check_count, metadata, created_by, snapshot_type
        ) VALUES (
            OLD.id, OLD.imei, CURRENT_TIMESTAMP, OLD.status, OLD.reason,
            OLD.check_count, OLD.metadata, 'SYSTEM', 'PRE_UPDATE'
        );
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to create snapshot before equipment update
DROP TRIGGER IF EXISTS trigger_equipment_snapshot_before_update ON equipment;
CREATE TRIGGER trigger_equipment_snapshot_before_update
    BEFORE UPDATE ON equipment
    FOR EACH ROW
    EXECUTE FUNCTION create_equipment_snapshot_before_update();

-- Function to automatically record equipment changes
CREATE OR REPLACE FUNCTION record_equipment_change()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        INSERT INTO equipment_history (
            imei, change_type, changed_at, changed_by,
            previous_status, new_status, previous_reason, new_reason
        ) VALUES (
            NEW.imei, 'CREATE', CURRENT_TIMESTAMP, NEW.added_by,
            NULL, NEW.status, NULL, NEW.reason
        );
    ELSIF TG_OP = 'UPDATE' THEN
        -- Only record if status or reason changed
        IF OLD.status IS DISTINCT FROM NEW.status OR OLD.reason IS DISTINCT FROM NEW.reason THEN
            INSERT INTO equipment_history (
                imei, change_type, changed_at, changed_by,
                previous_status, new_status, previous_reason, new_reason
            ) VALUES (
                NEW.imei, 'UPDATE', CURRENT_TIMESTAMP, COALESCE(NEW.added_by, 'SYSTEM'),
                OLD.status, NEW.status, OLD.reason, NEW.reason
            );
        END IF;
    ELSIF TG_OP = 'DELETE' THEN
        INSERT INTO equipment_history (
            imei, change_type, changed_at, changed_by,
            previous_status, new_status, previous_reason, new_reason
        ) VALUES (
            OLD.imei, 'DELETE', CURRENT_TIMESTAMP, 'SYSTEM',
            OLD.status, OLD.status, OLD.reason, NULL
        );
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to automatically record equipment changes
DROP TRIGGER IF EXISTS trigger_equipment_change_history ON equipment;
CREATE TRIGGER trigger_equipment_change_history
    AFTER INSERT OR UPDATE OR DELETE ON equipment
    FOR EACH ROW
    EXECUTE FUNCTION record_equipment_change();

-- Partitioning for equipment_history (by month)
-- Create partitions for equipment_history table
CREATE TABLE IF NOT EXISTS equipment_history_2024_q1 PARTITION OF equipment_history
    FOR VALUES FROM ('2024-01-01') TO ('2024-04-01');

CREATE TABLE IF NOT EXISTS equipment_history_2024_q2 PARTITION OF equipment_history
    FOR VALUES FROM ('2024-04-01') TO ('2024-07-01');

CREATE TABLE IF NOT EXISTS equipment_history_2024_q3 PARTITION OF equipment_history
    FOR VALUES FROM ('2024-07-01') TO ('2024-10-01');

CREATE TABLE IF NOT EXISTS equipment_history_2024_q4 PARTITION OF equipment_history
    FOR VALUES FROM ('2024-10-01') TO ('2025-01-01');

CREATE TABLE IF NOT EXISTS equipment_history_2025_q1 PARTITION OF equipment_history
    FOR VALUES FROM ('2025-01-01') TO ('2025-04-01');

CREATE TABLE IF NOT EXISTS equipment_history_2025_q2 PARTITION OF equipment_history
    FOR VALUES FROM ('2025-04-01') TO ('2025-07-01');

CREATE TABLE IF NOT EXISTS equipment_history_2025_q3 PARTITION OF equipment_history
    FOR VALUES FROM ('2025-07-01') TO ('2025-10-01');

CREATE TABLE IF NOT EXISTS equipment_history_2025_q4 PARTITION OF equipment_history
    FOR VALUES FROM ('2025-10-01') TO ('2026-01-01');

-- Views for reporting and analytics

-- View: Recent Equipment Changes
CREATE OR REPLACE VIEW v_recent_equipment_changes AS
SELECT
    eh.id,
    eh.imei,
    eh.change_type,
    eh.changed_at,
    eh.changed_by,
    eh.previous_status,
    eh.new_status,
    eh.previous_reason,
    eh.new_reason,
    e.manufacturer_tac,
    e.manufacturer_name
FROM equipment_history eh
LEFT JOIN equipment e ON eh.imei = e.imei
WHERE eh.changed_at >= NOW() - INTERVAL '7 days'
ORDER BY eh.changed_at DESC;

-- View: Audit Statistics by Day
CREATE OR REPLACE VIEW v_audit_stats_by_day AS
SELECT
    DATE(check_time) as check_date,
    request_source,
    status,
    COUNT(*) as check_count,
    COUNT(DISTINCT imei) as unique_imeis
FROM audit_log
WHERE check_time >= NOW() - INTERVAL '30 days'
GROUP BY DATE(check_time), request_source, status
ORDER BY check_date DESC, request_source, status;

-- View: Equipment with Change History
CREATE OR REPLACE VIEW v_equipment_with_history AS
SELECT
    e.id,
    e.imei,
    e.status,
    e.last_updated,
    e.check_count,
    COUNT(eh.id) as change_count,
    MAX(eh.changed_at) as last_change
FROM equipment e
LEFT JOIN equipment_history eh ON e.imei = eh.imei
GROUP BY e.id, e.imei, e.status, e.last_updated, e.check_count;

-- Function to get equipment change timeline
CREATE OR REPLACE FUNCTION get_equipment_timeline(p_imei VARCHAR(16))
RETURNS TABLE (
    event_time TIMESTAMP,
    event_type VARCHAR(20),
    event_source VARCHAR(50),
    status VARCHAR(20),
    details JSONB
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        eh.changed_at as event_time,
        'CHANGE'::VARCHAR(20) as event_type,
        'HISTORY'::VARCHAR(50) as event_source,
        eh.new_status as status,
        jsonb_build_object(
            'change_type', eh.change_type,
            'changed_by', eh.changed_by,
            'previous_status', eh.previous_status,
            'new_status', eh.new_status
        ) as details
    FROM equipment_history eh
    WHERE eh.imei = p_imei
    UNION ALL
    SELECT
        al.check_time as event_time,
        'CHECK'::VARCHAR(20) as event_type,
        al.request_source as event_source,
        al.status as status,
        jsonb_build_object(
            'origin_host', al.origin_host,
            'supi', al.supi,
            'result_code', al.result_code
        ) as details
    FROM audit_log al
    WHERE al.imei = p_imei
    ORDER BY event_time DESC;
END;
$$ LANGUAGE plpgsql;

-- Cleanup function for old data
CREATE OR REPLACE FUNCTION cleanup_old_data(days_to_keep INTEGER DEFAULT 90)
RETURNS TABLE (
    table_name TEXT,
    rows_deleted BIGINT
) AS $$
DECLARE
    cutoff_date TIMESTAMP;
    deleted_count BIGINT;
BEGIN
    cutoff_date := NOW() - INTERVAL '1 day' * days_to_keep;

    -- Clean audit_log_extended
    DELETE FROM audit_log_extended
    WHERE created_at < cutoff_date;
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    table_name := 'audit_log_extended';
    rows_deleted := deleted_count;
    RETURN NEXT;

    -- Clean equipment_snapshots
    DELETE FROM equipment_snapshots
    WHERE snapshot_time < cutoff_date;
    GET DIAGNOSTICS deleted_count = ROW_COUNT;
    table_name := 'equipment_snapshots';
    rows_deleted := deleted_count;
    RETURN NEXT;

    RETURN;
END;
$$ LANGUAGE plpgsql;

-- Comments for documentation
COMMENT ON TABLE equipment_history IS 'Tracks all changes to equipment records including create, update, and delete operations';
COMMENT ON TABLE equipment_snapshots IS 'Point-in-time snapshots of equipment state, useful for audit and rollback';
COMMENT ON TABLE audit_log_extended IS 'Extended audit information including IP addresses, user agents, and processing metrics';
COMMENT ON FUNCTION get_equipment_timeline IS 'Returns a complete timeline of all events (changes and checks) for a specific IMEI';
COMMENT ON FUNCTION cleanup_old_data IS 'Removes historical data older than specified number of days';
