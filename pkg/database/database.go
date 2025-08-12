package database

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/ShashankBejjanki1241/Go-Based-Server-Log-Analyzer-Reporting-Platform/pkg/config"
	"github.com/ShashankBejjanki1241/Go-Based-Server-Log-Analyzer-Reporting-Platform/pkg/models"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

type Database struct {
	DB     *sql.DB
	Config *config.Config
}

func NewDatabase(cfg *config.Config) (*Database, error) {
	db, err := sql.Open(cfg.GetDriverName(), cfg.GetDSN())
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	database := &Database{
		DB:     db,
		Config: cfg,
	}

	// Initialize schema
	if err := database.InitSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return database, nil
}

func (d *Database) InitSchema() error {
	switch d.Config.Database.Type {
	case "mysql":
		return d.initMySQLSchema()
	case "postgres":
		return d.initPostgreSQLSchema()
	default:
		return fmt.Errorf("unsupported database type: %s", d.Config.Database.Type)
	}
}

func (d *Database) initMySQLSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS log_entries (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			timestamp DATETIME NOT NULL,
			log_type VARCHAR(20) NOT NULL,
			source_ip VARCHAR(45) NOT NULL,
			method VARCHAR(10),
			path TEXT,
			status_code INT,
			response_size BIGINT,
			user_agent TEXT,
			referer TEXT,
			processing_time DOUBLE,
			raw_log LONGTEXT,
			metadata JSON,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_timestamp (timestamp),
			INDEX idx_log_type (log_type),
			INDEX idx_source_ip (source_ip),
			INDEX idx_status_code (status_code),
			INDEX idx_method (method)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		
		`CREATE TABLE IF NOT EXISTS log_stats_cache (
			id INT AUTO_INCREMENT PRIMARY KEY,
			stat_type VARCHAR(50) NOT NULL,
			stat_data JSON NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY unique_stat_type (stat_type)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		
		`CREATE TABLE IF NOT EXISTS alert_rules (
			id INT AUTO_INCREMENT PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			description TEXT,
			condition_type VARCHAR(20) NOT NULL,
			threshold_value DOUBLE NOT NULL,
			time_window INT NOT NULL,
			is_active BOOLEAN DEFAULT TRUE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		
		`CREATE TABLE IF NOT EXISTS alert_history (
			id INT AUTO_INCREMENT PRIMARY KEY,
			rule_id INT NOT NULL,
			message TEXT NOT NULL,
			severity VARCHAR(20) NOT NULL,
			triggered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (rule_id) REFERENCES alert_rules(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	}

	for _, query := range queries {
		if _, err := d.DB.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query: %s, error: %w", query, err)
		}
	}

	return nil
}

func (d *Database) initPostgreSQLSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS log_entries (
			id BIGSERIAL PRIMARY KEY,
			timestamp TIMESTAMP NOT NULL,
			log_type VARCHAR(20) NOT NULL,
			source_ip INET NOT NULL,
			method VARCHAR(10),
			path TEXT,
			status_code INTEGER,
			response_size BIGINT,
			user_agent TEXT,
			referer TEXT,
			processing_time DOUBLE PRECISION,
			raw_log TEXT,
			metadata JSONB,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		
		`CREATE INDEX IF NOT EXISTS idx_log_entries_timestamp ON log_entries(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_log_entries_log_type ON log_entries(log_type)`,
		`CREATE INDEX IF NOT EXISTS idx_log_entries_source_ip ON log_entries(source_ip)`,
		`CREATE INDEX IF NOT EXISTS idx_log_entries_status_code ON log_entries(status_code)`,
		`CREATE INDEX IF NOT EXISTS idx_log_entries_method ON log_entries(method)`,
		
		`CREATE TABLE IF NOT EXISTS log_stats_cache (
			id SERIAL PRIMARY KEY,
			stat_type VARCHAR(50) NOT NULL UNIQUE,
			stat_data JSONB NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		
		`CREATE TABLE IF NOT EXISTS alert_rules (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			description TEXT,
			condition_type VARCHAR(20) NOT NULL,
			threshold_value DOUBLE PRECISION NOT NULL,
			time_window INTEGER NOT NULL,
			is_active BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		
		`CREATE TABLE IF NOT EXISTS alert_history (
			id SERIAL PRIMARY KEY,
			rule_id INTEGER NOT NULL,
			message TEXT NOT NULL,
			severity VARCHAR(20) NOT NULL,
			triggered_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (rule_id) REFERENCES alert_rules(id) ON DELETE CASCADE
		)`,
	}

	for _, query := range queries {
		if _, err := d.DB.Exec(query); err != nil {
			return fmt.Errorf("failed to execute query: %s, error: %w", query, err)
		}
	}

	return nil
}

func (d *Database) Close() error {
	return d.DB.Close()
}

// HealthCheck performs a simple health check on the database
func (d *Database) HealthCheck() error {
	return d.DB.Ping()
}

// GetStats returns database statistics
func (d *Database) GetStats() (map[string]interface{}, error) {
	var totalLogs int64
	var totalSize int64

	// Get total log entries
	err := d.DB.QueryRow("SELECT COUNT(*) FROM log_entries").Scan(&totalLogs)
	if err != nil {
		return nil, fmt.Errorf("failed to count log entries: %w", err)
	}

	// Get total size (approximate)
	err = d.DB.QueryRow("SELECT COALESCE(SUM(response_size), 0) FROM log_entries").Scan(&totalSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get total size: %w", err)
	}

	return map[string]interface{}{
		"total_logs":   totalLogs,
		"total_size":   totalSize,
		"database_type": d.Config.Database.Type,
		"connected":    true,
	}, nil
}
