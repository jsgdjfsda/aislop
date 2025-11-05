-- Enable TimescaleDB extension
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- Create telemetry table
CREATE TABLE IF NOT EXISTS device_telemetry (
    time TIMESTAMPTZ NOT NULL,
    device_id UUID NOT NULL,
    metric_name VARCHAR(100) NOT NULL,
    metric_value DOUBLE PRECISION NOT NULL,
    unit VARCHAR(20),
    metadata JSONB,
    CONSTRAINT device_telemetry_pkey PRIMARY KEY (time, device_id, metric_name)
);

-- Convert to hypertable (time-series optimized)
SELECT create_hypertable('device_telemetry', 'time', if_not_exists => TRUE);

-- Create indexes for common queries
CREATE INDEX IF NOT EXISTS idx_device_telemetry_device_id ON device_telemetry(device_id, time DESC);
CREATE INDEX IF NOT EXISTS idx_device_telemetry_metric_name ON device_telemetry(metric_name, time DESC);

-- Create continuous aggregates for common analytics queries
-- Hourly aggregates
CREATE MATERIALIZED VIEW IF NOT EXISTS device_telemetry_hourly
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 hour', time) AS hour,
    device_id,
    metric_name,
    AVG(metric_value) AS avg_value,
    MIN(metric_value) AS min_value,
    MAX(metric_value) AS max_value,
    COUNT(*) AS count
FROM device_telemetry
GROUP BY hour, device_id, metric_name
WITH NO DATA;

-- Daily aggregates
CREATE MATERIALIZED VIEW IF NOT EXISTS device_telemetry_daily
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 day', time) AS day,
    device_id,
    metric_name,
    AVG(metric_value) AS avg_value,
    MIN(metric_value) AS min_value,
    MAX(metric_value) AS max_value,
    COUNT(*) AS count
FROM device_telemetry
GROUP BY day, device_id, metric_name
WITH NO DATA;

-- Set up refresh policies (automatically update aggregates)
SELECT add_continuous_aggregate_policy('device_telemetry_hourly',
    start_offset => INTERVAL '3 hours',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour',
    if_not_exists => TRUE
);

SELECT add_continuous_aggregate_policy('device_telemetry_daily',
    start_offset => INTERVAL '3 days',
    end_offset => INTERVAL '1 day',
    schedule_interval => INTERVAL '1 day',
    if_not_exists => TRUE
);

-- Set up retention policy (delete data older than 90 days)
SELECT add_retention_policy('device_telemetry', INTERVAL '90 days', if_not_exists => TRUE);

-- Create device events table
CREATE TABLE IF NOT EXISTS device_events (
    time TIMESTAMPTZ NOT NULL,
    device_id UUID NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    event_data JSONB,
    severity VARCHAR(20),
    CONSTRAINT device_events_pkey PRIMARY KEY (time, device_id, event_type)
);

-- Convert to hypertable
SELECT create_hypertable('device_events', 'time', if_not_exists => TRUE);

-- Create index
CREATE INDEX IF NOT EXISTS idx_device_events_device_id ON device_events(device_id, time DESC);
CREATE INDEX IF NOT EXISTS idx_device_events_type ON device_events(event_type, time DESC);

-- Retention policy for events (keep for 30 days)
SELECT add_retention_policy('device_events', INTERVAL '30 days', if_not_exists => TRUE);
