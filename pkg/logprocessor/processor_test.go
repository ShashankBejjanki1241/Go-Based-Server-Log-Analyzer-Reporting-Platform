package logprocessor

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProcessor(t *testing.T) {
	processor := NewProcessor(5)
	assert.NotNil(t, processor)
	assert.Equal(t, 5, cap(processor.workerPool))
	assert.NotNil(t, processor.processedLogs)
	assert.NotNil(t, processor.errors)
	assert.NotNil(t, processor.stats)
}

func TestParseApacheLog(t *testing.T) {
	processor := NewProcessor(1)
	
	// Valid Apache log line
	line := `192.168.1.100 - - [10/Oct/2023:13:55:36 +0000] "GET /api/users HTTP/1.1" 200 1234 "https://example.com" "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"`
	
	entry, err := processor.parseApacheLog(line)
	require.NoError(t, err)
	assert.NotNil(t, entry)
	
	assert.Equal(t, "apache", entry.LogType)
	assert.Equal(t, "192.168.1.100", entry.SourceIP)
	assert.Equal(t, "GET", entry.Method)
	assert.Equal(t, "/api/users", entry.Path)
	assert.Equal(t, 200, entry.StatusCode)
	assert.Equal(t, int64(1234), entry.ResponseSize)
	assert.Equal(t, "https://example.com", entry.Referer)
	assert.Equal(t, "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36", entry.UserAgent)
	assert.Equal(t, line, entry.RawLog)
}

func TestParseNginxLog(t *testing.T) {
	processor := NewProcessor(1)
	
	// Valid Nginx log line
	line := `192.168.1.101 - - [10/Oct/2023:13:55:37 +0000] "POST /api/login HTTP/1.1" 401 567 "https://example.com" "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)" 0.045`
	
	entry, err := processor.parseNginxLog(line)
	require.NoError(t, err)
	assert.NotNil(t, entry)
	
	assert.Equal(t, "nginx", entry.LogType)
	assert.Equal(t, "192.168.1.101", entry.SourceIP)
	assert.Equal(t, "POST", entry.Method)
	assert.Equal(t, "/api/login", entry.Path)
	assert.Equal(t, 401, entry.StatusCode)
	assert.Equal(t, int64(567), entry.ResponseSize)
	assert.Equal(t, "https://example.com", entry.Referer)
	assert.Equal(t, "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)", entry.UserAgent)
	assert.Equal(t, 0.045, entry.ProcessingTime)
}

func TestParseGenericLog(t *testing.T) {
	processor := NewProcessor(1)
	
	// Valid generic log line
	line := `2023-10-10 13:55:38 INFO User login successful user_id=12345 ip=192.168.1.102`
	
	entry, err := processor.parseGenericLog(line)
	require.NoError(t, err)
	assert.NotNil(t, entry)
	
	assert.Equal(t, "generic", entry.LogType)
	assert.Equal(t, line, entry.RawLog)
	assert.Contains(t, entry.Metadata, "user_id")
	assert.Contains(t, entry.Metadata, "ip")
	assert.Equal(t, 12345, entry.Metadata["user_id"])
	assert.Equal(t, "192.168.1.102", entry.Metadata["ip"])
}

func TestParseLogLineInvalidType(t *testing.T) {
	processor := NewProcessor(1)
	
	_, err := processor.parseLogLine("some log line", "invalid_type")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported log type")
}

func TestParseApacheLogInvalidFormat(t *testing.T) {
	processor := NewProcessor(1)
	
	// Invalid Apache log line (missing parts)
	line := `192.168.1.100 - - [10/Oct/2023:13:55:36 +0000] "GET /api/users HTTP/1.1" 200`
	
	_, err := processor.parseApacheLog(line)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid Apache log format")
}

func TestParseNginxLogInvalidFormat(t *testing.T) {
	processor := NewProcessor(1)
	
	// Invalid Nginx log line (missing parts)
	line := `192.168.1.101 - - [10/Oct/2023:13:55:37 +0000] "POST /api/login HTTP/1.1" 401`
	
	_, err := processor.parseNginxLog(line)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid Nginx log format")
}

func TestParseGenericLogInvalidFormat(t *testing.T) {
	processor := NewProcessor(1)
	
	// Invalid generic log line (missing parts)
	line := `2023-10-10 INFO`
	
	_, err := processor.parseGenericLog(line)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid generic log format")
}

func TestIsValidIP(t *testing.T) {
	processor := NewProcessor(1)
	
	// Valid IPs
	assert.True(t, processor.isValidIP("192.168.1.100"))
	assert.True(t, processor.isValidIP("10.0.0.1"))
	assert.True(t, processor.isValidIP("172.16.0.1"))
	assert.True(t, processor.isValidIP("::1"))
	assert.True(t, processor.isValidIP("2001:db8::1"))
	
	// Invalid IPs
	assert.False(t, processor.isValidIP("256.256.256.256"))
	assert.False(t, processor.isValidIP("192.168.1.256"))
	assert.False(t, processor.isValidIP("invalid"))
	assert.False(t, processor.isValidIP("192.168.1"))
	assert.False(t, processor.isValidIP("192.168.1.1.1"))
}

func TestExtractKeyValuePairs(t *testing.T) {
	processor := NewProcessor(1)
	
	message := "User login successful user_id=12345 ip=192.168.1.100 status=active error=false count=42"
	
	metadata := processor.extractKeyValuePairs(message)
	
	assert.Contains(t, metadata, "user_id")
	assert.Contains(t, metadata, "ip")
	assert.Contains(t, metadata, "status")
	assert.Contains(t, metadata, "error")
	assert.Contains(t, metadata, "count")
	
	assert.Equal(t, 12345, metadata["user_id"])
	assert.Equal(t, "192.168.1.100", metadata["ip"])
	assert.Equal(t, "active", metadata["status"])
	assert.Equal(t, false, metadata["error"])
	assert.Equal(t, 42, metadata["count"])
}

func TestGetStats(t *testing.T) {
	processor := NewProcessor(1)
	
	// Process some logs to generate stats
	processor.stats.incrementProcessed("apache")
	processor.stats.incrementProcessed("apache")
	processor.stats.incrementProcessed("nginx")
	processor.stats.incrementProcessed("generic")
	processor.stats.incrementErrors()
	
	stats := processor.GetStats()
	
	assert.Equal(t, int64(4), stats.TotalProcessed)
	assert.Equal(t, int64(2), stats.ApacheProcessed)
	assert.Equal(t, int64(1), stats.NginxProcessed)
	assert.Equal(t, int64(1), stats.GenericProcessed)
	assert.Equal(t, int64(1), stats.Errors)
}

func TestProcessFile(t *testing.T) {
	processor := NewProcessor(2)
	
	// Create a simple log file content
	logContent := `192.168.1.100 - - [10/Oct/2023:13:55:36 +0000] "GET /api/users HTTP/1.1" 200 1234 "https://example.com" "Mozilla/5.0"
192.168.1.101 - - [10/Oct/2023:13:55:37 +0000] "POST /api/login HTTP/1.1" 401 567 "https://example.com" "Mozilla/5.0"`
	
	reader := strings.NewReader(logContent)
	
	// Process the file
	err := processor.ProcessFile(reader, "apache")
	require.NoError(t, err)
	
	// Wait a bit for processing to complete
	time.Sleep(100 * time.Millisecond)
	
	// Check stats
	stats := processor.GetStats()
	assert.Equal(t, int64(2), stats.ApacheProcessed)
}

func TestSplitApacheLog(t *testing.T) {
	processor := NewProcessor(1)
	
	line := `192.168.1.100 - - [10/Oct/2023:13:55:36 +0000] "GET /api/users HTTP/1.1" 200 1234 "https://example.com" "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"`
	
	parts := processor.splitApacheLog(line)
	
	assert.Len(t, parts, 10)
	assert.Equal(t, "192.168.1.100", parts[0])
	assert.Equal(t, "-", parts[1])
	assert.Equal(t, "-", parts[2])
	assert.Equal(t, "[10/Oct/2023:13:55:36", parts[3])
	assert.Equal(t, "+0000]", parts[4])
	assert.Equal(t, "GET /api/users HTTP/1.1", parts[5])
	assert.Equal(t, "200", parts[6])
	assert.Equal(t, "1234", parts[7])
	assert.Equal(t, "https://example.com", parts[8])
	assert.Equal(t, "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36", parts[9])
}

func TestParseApacheTimestamp(t *testing.T) {
	processor := NewProcessor(1)
	
	// Test various timestamp formats
	timestampStr := "[10/Oct/2023:13:55:36 +0000]"
	
	timestamp, err := processor.parseApacheTimestamp(timestampStr)
	require.NoError(t, err)
	
	expected := time.Date(2023, 10, 10, 13, 55, 36, 0, time.UTC)
	assert.Equal(t, expected.Year(), timestamp.Year())
	assert.Equal(t, expected.Month(), timestamp.Month())
	assert.Equal(t, expected.Day(), timestamp.Day())
	assert.Equal(t, expected.Hour(), timestamp.Hour())
	assert.Equal(t, expected.Minute(), timestamp.Minute())
	assert.Equal(t, expected.Second(), timestamp.Second())
}

func TestParseGenericTimestamp(t *testing.T) {
	processor := NewProcessor(1)
	
	// Test various timestamp formats
	timestampStr := "2023-10-10 13:55:36"
	
	timestamp, err := processor.parseGenericTimestamp(timestampStr)
	require.NoError(t, err)
	
	expected := time.Date(2023, 10, 10, 13, 55, 36, 0, time.UTC)
	assert.Equal(t, expected.Year(), timestamp.Year())
	assert.Equal(t, expected.Month(), timestamp.Month())
	assert.Equal(t, expected.Day(), timestamp.Day())
	assert.Equal(t, expected.Hour(), timestamp.Hour())
	assert.Equal(t, expected.Minute(), timestamp.Minute())
	assert.Equal(t, expected.Second(), timestamp.Second())
}

func TestProcessorClose(t *testing.T) {
	processor := NewProcessor(1)
	
	// Close the processor
	processor.Close()
	
	// Verify channels are closed
	_, ok := <-processor.processedLogs
	assert.False(t, ok)
	
	_, ok = <-processor.errors
	assert.False(t, ok)
	
	_, ok = <-processor.workerPool
	assert.False(t, ok)
}

// Benchmark tests
func BenchmarkParseApacheLog(b *testing.B) {
	processor := NewProcessor(1)
	line := `192.168.1.100 - - [10/Oct/2023:13:55:36 +0000] "GET /api/users HTTP/1.1" 200 1234 "https://example.com" "Mozilla/5.0"`
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := processor.parseApacheLog(line)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseNginxLog(b *testing.B) {
	processor := NewProcessor(1)
	line := `192.168.1.101 - - [10/Oct/2023:13:55:37 +0000] "POST /api/login HTTP/1.1" 401 567 "https://example.com" "Mozilla/5.0" 0.045`
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := processor.parseNginxLog(line)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseGenericLog(b *testing.B) {
	processor := NewProcessor(1)
	line := `2023-10-10 13:55:38 INFO User login successful user_id=12345 ip=192.168.1.102`
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := processor.parseGenericLog(line)
		if err != nil {
			b.Fatal(err)
		}
	}
}
