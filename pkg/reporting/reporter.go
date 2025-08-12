package reporting

import (
	"encoding/csv"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ShashankBejjanki1241/Go-Based-Server-Log-Analyzer-Reporting-Platform/pkg/models"
)

// Reporter handles report generation
type Reporter struct {
	templates *template.Template
	outputDir string
}

// ReportData contains all data needed for report generation
type ReportData struct {
	Title       string
	GeneratedAt time.Time
	TimeRange   string
	Stats       *models.LogStats
	LogEntries  []*models.LogEntry
	Filters     *models.LogFilter
	Summary     ReportSummary
}

type ReportSummary struct {
	TotalRequests    int64
	UniqueIPs        int64
	AvgResponseTime  float64
	ErrorRate        float64
	TopPaths         []PathSummary
	TopIPs           []IPSummary
	StatusCodeBreakdown map[string]int64
	HourlyTraffic    []HourlyTraffic
}

type PathSummary struct {
	Path  string
	Count int64
	Percentage float64
}

type IPSummary struct {
	IP    string
	Count int64
	Percentage float64
}

type HourlyTraffic struct {
	Hour  int
	Count int64
}

func NewReporter(templateDir, outputDir string) (*Reporter, error) {
	// Parse HTML templates
	templates, err := template.ParseGlob(filepath.Join(templateDir, "*.html"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	return &Reporter{
		templates: templates,
		outputDir: outputDir,
	}, nil
}

// GenerateHTMLReport generates an HTML report
func (r *Reporter) GenerateHTMLReport(data *ReportData, reportName string) (string, error) {
	// Prepare summary data
	r.prepareSummary(data)

	// Generate filename with timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("%s_%s.html", reportName, timestamp)
	filepath := filepath.Join(r.outputDir, filename)

	// Create output file
	file, err := os.Create(filepath)
	if err != nil {
		return "", fmt.Errorf("failed to create report file: %w", err)
	}
	defer file.Close()

	// Execute template
	if err := r.templates.ExecuteTemplate(file, "report.html", data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return filepath, nil
}

// GenerateCSVReport generates a CSV report
func (r *Reporter) GenerateCSVReport(data *ReportData, reportName string) (string, error) {
	// Generate filename with timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("%s_%s.csv", reportName, timestamp)
	filepath := filepath.Join(r.outputDir, filename)

	// Create output file
	file, err := os.Create(filepath)
	if err != nil {
		return "", fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	// Create CSV writer
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{
		"Timestamp", "Log Type", "Source IP", "Method", "Path",
		"Status Code", "Response Size", "User Agent", "Referer",
		"Processing Time", "Raw Log",
	}
	if err := writer.Write(header); err != nil {
		return "", fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write data rows
	for _, entry := range data.LogEntries {
		row := []string{
			entry.Timestamp.Format("2006-01-02 15:04:05"),
			entry.LogType,
			entry.SourceIP,
			entry.Method,
			entry.Path,
			fmt.Sprintf("%d", entry.StatusCode),
			fmt.Sprintf("%d", entry.ResponseSize),
			entry.UserAgent,
			entry.Referer,
			fmt.Sprintf("%.3f", entry.ProcessingTime),
			entry.RawLog,
		}
		if err := writer.Write(row); err != nil {
			return "", fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	return filepath, nil
}

// GenerateSummaryReport generates a summary report with statistics
func (r *Reporter) GenerateSummaryReport(data *ReportData, reportName string) (string, error) {
	// Prepare summary data
	r.prepareSummary(data)

	// Generate filename with timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("%s_summary_%s.html", reportName, timestamp)
	filepath := filepath.Join(r.outputDir, filename)

	// Create output file
	file, err := os.Create(filepath)
	if err != nil {
		return "", fmt.Errorf("failed to create summary file: %w", err)
	}
	defer file.Close()

	// Execute summary template
	if err := r.templates.ExecuteTemplate(file, "summary.html", data); err != nil {
		return "", fmt.Errorf("failed to execute summary template: %w", err)
	}

	return filepath, nil
}

// prepareSummary prepares summary data for reports
func (r *Reporter) prepareSummary(data *ReportData) {
	if data.Stats == nil {
		data.Stats = &models.LogStats{}
	}

	// Calculate basic stats
	data.Summary.TotalRequests = int64(len(data.LogEntries))
	
	// Count unique IPs
	ipCounts := make(map[string]int64)
	for _, entry := range data.LogEntries {
		ipCounts[entry.SourceIP]++
	}
	data.Summary.UniqueIPs = int64(len(ipCounts))

	// Calculate average response time
	var totalTime float64
	var timeCount int
	for _, entry := range data.LogEntries {
		if entry.ProcessingTime > 0 {
			totalTime += entry.ProcessingTime
			timeCount++
		}
	}
	if timeCount > 0 {
		data.Summary.AvgResponseTime = totalTime / float64(timeCount)
	}

	// Calculate error rate
	var errorCount int64
	for _, entry := range data.LogEntries {
		if entry.StatusCode >= 400 {
			errorCount++
		}
	}
	if data.Summary.TotalRequests > 0 {
		data.Summary.ErrorRate = float64(errorCount) / float64(data.Summary.TotalRequests) * 100
	}

	// Top paths
	pathCounts := make(map[string]int64)
	for _, entry := range data.LogEntries {
		pathCounts[entry.Path]++
	}
	data.Summary.TopPaths = r.getTopItems(pathCounts, 10)

	// Top IPs
	data.Summary.TopIPs = r.getTopIPs(ipCounts, 10)

	// Status code breakdown
	statusCounts := make(map[string]int64)
	for _, entry := range data.LogEntries {
		statusStr := fmt.Sprintf("%d", entry.StatusCode)
		statusCounts[statusStr]++
	}
	data.Summary.StatusCodeBreakdown = statusCounts

	// Hourly traffic
	data.Summary.HourlyTraffic = r.getHourlyTraffic(data.LogEntries)
}

// getTopItems returns top N items by count
func (r *Reporter) getTopItems(counts map[string]int64, n int) []PathSummary {
	var items []PathSummary
	for item, count := range counts {
		items = append(items, PathSummary{
			Path:  item,
			Count: count,
		})
	}

	// Sort by count (descending)
	sort.Slice(items, func(i, j int) bool {
		return items[i].Count > items[j].Count
	})

	// Limit to top N
	if len(items) > n {
		items = items[:n]
	}

	// Calculate percentages
	var total int64
	for _, item := range items {
		total += item.Count
	}

	for i := range items {
		if total > 0 {
			items[i].Percentage = float64(items[i].Count) / float64(total) * 100
		}
	}

	return items
}

// getTopIPs returns top N IPs by count
func (r *Reporter) getTopIPs(counts map[string]int64, n int) []IPSummary {
	var items []IPSummary
	for item, count := range counts {
		items = append(items, IPSummary{
			IP:    item,
			Count: count,
		})
	}

	// Sort by count (descending)
	sort.Slice(items, func(i, j int) bool {
		return items[i].Count > items[j].Count
	})

	// Limit to top N
	if len(items) > n {
		items = items[:n]
	}

	// Calculate percentages
	var total int64
	for _, item := range items {
		total += item.Count
	}

	for i := range items {
		if total > 0 {
			items[i].Percentage = float64(items[i].Count) / float64(total) * 100
		}
	}

	return items
}

// getHourlyTraffic returns hourly traffic distribution
func (r *Reporter) getHourlyTraffic(entries []*models.LogEntry) []HourlyTraffic {
	hourlyCounts := make(map[int]int64)
	
	for _, entry := range entries {
		hour := entry.Timestamp.Hour()
		hourlyCounts[hour]++
	}

	var traffic []HourlyTraffic
	for hour := 0; hour < 24; hour++ {
		traffic = append(traffic, HourlyTraffic{
			Hour:  hour,
			Count: hourlyCounts[hour],
		})
	}

	return traffic
}

// GenerateCombinedReport generates both HTML and CSV reports
func (r *Reporter) GenerateCombinedReport(data *ReportData, reportName string) ([]string, error) {
	var generatedFiles []string

	// Generate HTML report
	htmlFile, err := r.GenerateHTMLReport(data, reportName)
	if err != nil {
		return nil, fmt.Errorf("failed to generate HTML report: %w", err)
	}
	generatedFiles = append(generatedFiles, htmlFile)

	// Generate CSV report
	csvFile, err := r.GenerateCSVReport(data, reportName)
	if err != nil {
		return nil, fmt.Errorf("failed to generate CSV report: %w", err)
	}
	generatedFiles = append(generatedFiles, csvFile)

	// Generate summary report
	summaryFile, err := r.GenerateSummaryReport(data, reportName)
	if err != nil {
		return nil, fmt.Errorf("failed to generate summary report: %w", err)
	}
	generatedFiles = append(generatedFiles, summaryFile)

	return generatedFiles, nil
}

// ExportToFile exports data to a specific format
func (r *Reporter) ExportToFile(data interface{}, format, filename string) (string, error) {
	filepath := filepath.Join(r.outputDir, filename)

	switch strings.ToLower(format) {
	case "csv":
		return r.exportToCSV(data, filepath)
	case "json":
		return r.exportToJSON(data, filepath)
	default:
		return "", fmt.Errorf("unsupported export format: %s", format)
	}
}

func (r *Reporter) exportToCSV(data interface{}, filepath string) (string, error) {
	// Implementation depends on data structure
	// This is a placeholder for CSV export logic
	return "", fmt.Errorf("CSV export not implemented for this data type")
}

func (r *Reporter) exportToJSON(data interface{}, filepath string) (string, error) {
	// Implementation depends on data structure
	// This is a placeholder for JSON export logic
	return "", fmt.Errorf("JSON export not implemented for this data type")
}
