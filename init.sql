-- Database initialization for Go-Based Server Log Analyzer & Reporting Platform
-- This file is automatically executed when the MySQL container starts

USE log_analyzer;

-- Create log_entries table
CREATE TABLE IF NOT EXISTS log_entries (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    timestamp TIMESTAMP NOT NULL,
    log_type VARCHAR(50) NOT NULL,
    source_ip VARCHAR(45) NOT NULL,
    method VARCHAR(10),
    path TEXT,
    status_code INT,
    response_size BIGINT,
    user_agent TEXT,
    referer TEXT,
    processing_time DECIMAL(10,6),
    raw_log TEXT,
    metadata JSON,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_timestamp (timestamp),
    INDEX idx_log_type (log_type),
    INDEX idx_source_ip (source_ip),
    INDEX idx_status_code (status_code),
    INDEX idx_method (method)
);

-- Create log_stats_cache table for performance optimization
CREATE TABLE IF NOT EXISTS log_stats_cache (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    cache_key VARCHAR(255) NOT NULL UNIQUE,
    cache_value JSON NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_cache_key (cache_key),
    INDEX idx_expires_at (expires_at)
);

-- Create alert_rules table for monitoring
CREATE TABLE IF NOT EXISTS alert_rules (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    rule_type VARCHAR(50) NOT NULL,
    conditions JSON NOT NULL,
    threshold DECIMAL(10,2),
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_rule_type (rule_type),
    INDEX idx_is_active (is_active)
);

-- Create alert_history table for tracking alerts
CREATE TABLE IF NOT EXISTS alert_history (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    rule_id BIGINT,
    alert_message TEXT NOT NULL,
    severity VARCHAR(20) NOT NULL,
    triggered_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    resolved_at TIMESTAMP NULL,
    metadata JSON,
    INDEX idx_rule_id (rule_id),
    INDEX idx_severity (severity),
    INDEX idx_triggered_at (triggered_at),
    FOREIGN KEY (rule_id) REFERENCES alert_rules(id) ON DELETE SET NULL
);

-- Insert some sample alert rules
INSERT INTO alert_rules (name, description, rule_type, conditions, threshold) VALUES
('High Error Rate', 'Alert when error rate exceeds threshold', 'error_rate', '{"status_codes": [400, 401, 403, 404, 500, 502, 503, 504]}', 5.0),
('High Response Time', 'Alert when average response time is too high', 'response_time', '{"method": "GET"}', 2.0),
('Suspicious IP Activity', 'Alert when single IP makes too many requests', 'ip_activity', '{"time_window": 300}', 100);

-- Create indexes for better performance
CREATE INDEX idx_log_entries_composite ON log_entries(log_type, timestamp, status_code);
CREATE INDEX idx_log_entries_path ON log_entries(path(100));
CREATE INDEX idx_log_entries_user_agent ON log_entries(user_agent(100));

-- Show table creation status
SHOW TABLES;
