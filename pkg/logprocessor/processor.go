package logprocessor

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ShashankBejjanki1241/Go-Based-Server-Log-Analyzer-Reporting-Platform/pkg/models"
)

// Processor handles log parsing and processing
type Processor struct {
	mu sync.RWMutex
	// Channel for processed log entries
	processedLogs chan *models.LogEntry
	// Channel for errors
	errors chan error
	// Worker pool for concurrent processing
	workerPool chan struct{}
	// Statistics
	stats *ProcessingStats
}

// ProcessingStats tracks processing statistics
type ProcessingStats struct {
	mu              sync.RWMutex
	TotalProcessed  int64
	ApacheProcessed int64
	NginxProcessed  int64
	GenericProcessed int64
	Errors          int64
	StartTime       time.Time
}

func NewProcessor(workerCount int) *Processor {
	return &Processor{
		processedLogs: make(chan *models.LogEntry, 1000),
		errors:        make(chan error, 100),
		workerPool:    make(chan struct{}, workerCount),
		stats: &ProcessingStats{
			StartTime: time.Now(),
		},
	}
}

// ProcessFile processes a log file with the specified format
func (p *Processor) ProcessFile(reader io.Reader, logType string) error {
	scanner := bufio.NewScanner(reader)
	
	// Use a larger buffer for long log lines
	const maxCapacity = 1024 * 1024 // 1MB
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	var wg sync.WaitGroup
	lineCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		lineCount++
		wg.Add(1)

		// Acquire worker slot
		p.workerPool <- struct{}{}

		go func(line string, lineNum int) {
			defer wg.Done()
			defer func() { <-p.workerPool }()

			entry, err := p.parseLogLine(line, logType)
			if err != nil {
				p.errors <- fmt.Errorf("line %d: %w", lineNum, err)
				p.stats.incrementErrors()
				return
			}

			if entry != nil {
				p.processedLogs <- entry
				p.stats.incrementProcessed(logType)
			}
		}(line, lineCount)
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	// Wait for all workers to complete
	wg.Wait()

	return nil
}

// parseLogLine parses a single log line based on the log type
func (p *Processor) parseLogLine(line, logType string) (*models.LogEntry, error) {
	switch logType {
	case "apache":
		return p.parseApacheLog(line)
	case "nginx":
		return p.parseNginxLog(line)
	case "generic":
		return p.parseGenericLog(line)
	default:
		return nil, fmt.Errorf("unsupported log type: %s", logType)
	}
}

// parseApacheLog parses Apache access log format
func (p *Processor) parseApacheLog(line string) (*models.LogEntry, error) {
	// Apache Combined Log Format:
	// %h %l %u %t \"%r\" %>s %b \"%{Referer}i\" \"%{User-Agent}i\"
	
	// Split by spaces, but handle quoted strings properly
	parts := p.splitApacheLog(line)
	if len(parts) < 9 {
		return nil, fmt.Errorf("invalid Apache log format: expected at least 9 parts, got %d", len(parts))
	}

	// Parse IP address
	ip := parts[0]
	if !p.isValidIP(ip) {
		return nil, fmt.Errorf("invalid IP address: %s", ip)
	}

	// Parse timestamp
	timestamp, err := p.parseApacheTimestamp(parts[3] + " " + parts[4])
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp: %w", err)
	}

	// Parse request line (method, path, protocol)
	requestParts := strings.Fields(parts[5])
	if len(requestParts) < 2 {
		return nil, fmt.Errorf("invalid request format: %s", parts[5])
	}
	method := requestParts[0]
	path := requestParts[1]

	// Parse status code
	statusCode, err := strconv.Atoi(parts[6])
	if err != nil {
		return nil, fmt.Errorf("invalid status code: %s", parts[6])
	}

	// Parse response size
	responseSize, err := strconv.ParseInt(parts[7], 10, 64)
	if err != nil {
		responseSize = 0 // Set to 0 if parsing fails
	}

	// Parse referer and user agent (remove quotes)
	referer := strings.Trim(parts[8], `"`)
	userAgent := strings.Trim(parts[9], `"`)

	entry := &models.LogEntry{
		Timestamp:    timestamp,
		LogType:      "apache",
		SourceIP:     ip,
		Method:       method,
		Path:         path,
		StatusCode:   statusCode,
		ResponseSize: responseSize,
		UserAgent:    userAgent,
		Referer:      referer,
		RawLog:       line,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	return entry, nil
}

// parseNginxLog parses Nginx access log format
func (p *Processor) parseNginxLog(line string) (*models.LogEntry, error) {
	// Nginx Combined Log Format:
	// $remote_addr - $remote_user [$time_local] "$request" $status $body_bytes_sent "$http_referer" "$http_user_agent" "$request_time"
	
	parts := p.splitNginxLog(line)
	if len(parts) < 9 {
		return nil, fmt.Errorf("invalid Nginx log format: expected at least 9 parts, got %d", len(parts))
	}

	// Parse IP address
	ip := parts[0]
	if !p.isValidIP(ip) {
		return nil, fmt.Errorf("invalid IP address: %s", ip)
	}

	// Parse timestamp
	timestamp, err := p.parseNginxTimestamp(parts[3] + " " + parts[4])
	if err != nil {
		return nil, fmt.Errorf("invalid timestamp: %w", err)
	}

	// Parse request line
	requestParts := strings.Fields(parts[5])
	if len(requestParts) < 2 {
		return nil, fmt.Errorf("invalid request format: %s", parts[5])
	}
	method := requestParts[0]
	path := requestParts[1]

	// Parse status code
	statusCode, err := strconv.Atoi(parts[6])
	if err != nil {
		return nil, fmt.Errorf("invalid status code: %s", parts[6])
	}

	// Parse response size
	responseSize, err := strconv.ParseInt(parts[7], 10, 64)
	if err != nil {
		responseSize = 0
	}

	// Parse referer and user agent
	referer := strings.Trim(parts[8], `"`)
	userAgent := strings.Trim(parts[9], `"`)

	// Parse request time (if available)
	var processingTime float64
	if len(parts) > 10 {
		processingTime, _ = strconv.ParseFloat(parts[10], 64)
	}

	entry := &models.LogEntry{
		Timestamp:      timestamp,
		LogType:        "nginx",
		SourceIP:       ip,
		Method:         method,
		Path:           path,
		StatusCode:     statusCode,
		ResponseSize:   responseSize,
		UserAgent:      userAgent,
		Referer:        referer,
		ProcessingTime: processingTime,
		RawLog:         line,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	return entry, nil
}

// parseGenericLog parses generic log format
func (p *Processor) parseGenericLog(line string) (*models.LogEntry, error) {
	// Generic format: timestamp level message [key=value]...
	parts := strings.Fields(line)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid generic log format: expected at least 3 parts, got %d", len(parts))
	}

	// Parse timestamp (try common formats)
	timestamp, err := p.parseGenericTimestamp(parts[0] + " " + parts[1])
	if err != nil {
		// If timestamp parsing fails, use current time
		timestamp = time.Now()
	}

	level := parts[2]
	message := strings.Join(parts[3:], " ")

	// Extract key-value pairs from message
	metadata := p.extractKeyValuePairs(message)

	entry := &models.LogEntry{
		Timestamp: timestamp,
		LogType:   "generic",
		Path:      message, // Store message in path field for consistency
		RawLog:    line,
		Metadata:  metadata,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	return entry, nil
}

// Helper methods for parsing
func (p *Processor) splitApacheLog(line string) []string {
	// Handle quoted strings properly
	var parts []string
	var current strings.Builder
	inQuotes := false
	escapeNext := false

	for i, char := range line {
		if escapeNext {
			current.WriteRune(char)
			escapeNext = false
			continue
		}

		if char == '\\' {
			escapeNext = true
			continue
		}

		if char == '"' {
			inQuotes = !inQuotes
			continue
		}

		if char == ' ' && !inQuotes {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		} else {
			current.WriteRune(char)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

func (p *Processor) splitNginxLog(line string) []string {
	// Similar to Apache but simpler
	return p.splitApacheLog(line)
}

func (p *Processor) parseApacheTimestamp(timestampStr string) (time.Time, error) {
	// Apache format: [dd/MMM/yyyy:HH:mm:ss +zzzz]
	timestampStr = strings.Trim(timestampStr, "[]")
	
	// Try multiple formats
	formats := []string{
		"02/Jan/2006:15:04:05 -0700",
		"02/Jan/2006:15:04:05 +0700",
		"02/Jan/2006:15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timestampStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse timestamp: %s", timestampStr)
}

func (p *Processor) parseNginxTimestamp(timestampStr string) (time.Time, error) {
	// Nginx format: dd/MMM/yyyy:HH:mm:ss +zzzz
	return p.parseApacheTimestamp(timestampStr)
}

func (p *Processor) parseGenericTimestamp(timestampStr string) (time.Time, error) {
	// Try common timestamp formats
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000Z",
		"Jan 2, 2006 at 3:04pm (MST)",
		"2006-01-02 15:04:05.000",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, timestampStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse timestamp: %s", timestampStr)
}

func (p *Processor) isValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

func (p *Processor) extractKeyValuePairs(message string) models.LogMetadata {
	metadata := make(models.LogMetadata)
	
	// Look for key=value patterns
	re := regexp.MustCompile(`(\w+)=([^\s]+)`)
	matches := re.FindAllStringSubmatch(message, -1)
	
	for _, match := range matches {
		if len(match) == 3 {
			key := match[1]
			value := match[2]
			
			// Try to convert to appropriate type
			if intVal, err := strconv.Atoi(value); err == nil {
				metadata[key] = intVal
			} else if floatVal, err := strconv.ParseFloat(value, 64); err == nil {
				metadata[key] = floatVal
			} else if boolVal, err := strconv.ParseBool(value); err == nil {
				metadata[key] = boolVal
			} else {
				metadata[key] = value
			}
		}
	}
	
	return metadata
}

// GetProcessedLogs returns the channel for processed log entries
func (p *Processor) GetProcessedLogs() <-chan *models.LogEntry {
	return p.processedLogs
}

// GetErrors returns the channel for processing errors
func (p *Processor) GetErrors() <-chan error {
	return p.errors
}

// GetStats returns current processing statistics
func (p *Processor) GetStats() *ProcessingStats {
	p.stats.mu.RLock()
	defer p.stats.mu.RUnlock()
	
	return &ProcessingStats{
		TotalProcessed:  p.stats.TotalProcessed,
		ApacheProcessed: p.stats.ApacheProcessed,
		NginxProcessed:  p.stats.NginxProcessed,
		GenericProcessed: p.stats.GenericProcessed,
		Errors:          p.stats.Errors,
		StartTime:       p.stats.StartTime,
	}
}

// Close closes the processor and its channels
func (p *Processor) Close() {
	close(p.processedLogs)
	close(p.errors)
	close(p.workerPool)
}

// Stats methods
func (s *ProcessingStats) incrementProcessed(logType string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.TotalProcessed++
	switch logType {
	case "apache":
		s.ApacheProcessed++
	case "nginx":
		s.NginxProcessed++
	case "generic":
		s.GenericProcessed++
	}
}

func (s *ProcessingStats) incrementErrors() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Errors++
}
