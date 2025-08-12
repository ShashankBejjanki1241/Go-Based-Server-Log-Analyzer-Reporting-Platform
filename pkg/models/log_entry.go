package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// LogEntry represents a parsed log entry
type LogEntry struct {
	ID          int64                  `json:"id" db:"id"`
	Timestamp   time.Time              `json:"timestamp" db:"timestamp"`
	LogType     string                 `json:"log_type" db:"log_type"` // apache, nginx, generic
	SourceIP    string                 `json:"source_ip" db:"source_ip"`
	Method      string                 `json:"method" db:"method"`
	Path        string                 `json:"path" db:"path"`
	StatusCode  int                    `json:"status_code" db:"status_code"`
	ResponseSize int64                 `json:"response_size" db:"response_size"`
	UserAgent   string                 `json:"user_agent" db:"user_agent"`
	Referer     string                 `json:"referer" db:"referer"`
	ProcessingTime float64             `json:"processing_time" db:"processing_time"`
	RawLog      string                 `json:"raw_log" db:"raw_log"`
	Metadata    LogMetadata            `json:"metadata" db:"metadata"`
	CreatedAt   time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at" db:"updated_at"`
}

// LogMetadata stores additional parsed information
type LogMetadata map[string]interface{}

// Value implements driver.Valuer for database storage
func (lm LogMetadata) Value() (driver.Value, error) {
	if lm == nil {
		return nil, nil
	}
	return json.Marshal(lm)
}

// Scan implements sql.Scanner for database retrieval
func (lm *LogMetadata) Scan(value interface{}) error {
	if value == nil {
		*lm = nil
		return nil
	}

	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return nil
	}

	return json.Unmarshal(bytes, lm)
}

// ApacheLogEntry represents Apache access log format
type ApacheLogEntry struct {
	IP          string
	Timestamp   time.Time
	Method      string
	Path        string
	Protocol    string
	StatusCode  int
	ResponseSize int64
	Referer     string
	UserAgent   string
}

// NginxLogEntry represents Nginx access log format
type NginxLogEntry struct {
	IP          string
	Timestamp   time.Time
	Method      string
	Path        string
	Protocol    string
	StatusCode  int
	ResponseSize int64
	Referer     string
	UserAgent   string
	ProcessingTime float64
}

// GenericLogEntry represents a generic log format
type GenericLogEntry struct {
	Timestamp   time.Time
	Level       string
	Message     string
	Fields      map[string]interface{}
}

// LogStats represents aggregated log statistics
type LogStats struct {
	TotalRequests    int64   `json:"total_requests"`
	UniqueIPs        int64   `json:"unique_ips"`
	AvgResponseTime  float64 `json:"avg_response_time"`
	ErrorRate        float64 `json:"error_rate"`
	TopPaths         []PathStats `json:"top_paths"`
	TopIPs           []IPStats   `json:"top_ips"`
	StatusCodeCounts map[int]int64 `json:"status_code_counts"`
}

type PathStats struct {
	Path  string `json:"path"`
	Count int64  `json:"count"`
}

type IPStats struct {
	IP    string `json:"ip"`
	Count int64  `json:"count"`
}

// LogFilter represents filtering options for log queries
type LogFilter struct {
	StartTime    *time.Time `json:"start_time"`
	EndTime      *time.Time `json:"end_time"`
	LogType      string     `json:"log_type"`
	StatusCode   *int       `json:"status_code"`
	SourceIP     string     `json:"source_ip"`
	Path         string     `json:"path"`
	Method       string     `json:"method"`
	Limit        int        `json:"limit"`
	Offset       int        `json:"offset"`
}
