package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
	"github.com/ShashankBejjanki1241/Go-Based-Server-Log-Analyzer-Reporting-Platform/pkg/config"
	"github.com/ShashankBejjanki1241/Go-Based-Server-Log-Analyzer-Reporting-Platform/pkg/database"
	"github.com/ShashankBejjanki1241/Go-Based-Server-Log-Analyzer-Reporting-Platform/pkg/logprocessor"
	"github.com/ShashankBejjanki1241/Go-Based-Server-Log-Analyzer-Reporting-Platform/pkg/models"
	"github.com/ShashankBejjanki1241/Go-Based-Server-Log-Analyzer-Reporting-Platform/pkg/reporting"
)

type Server struct {
	config     *config.Config
	db         *database.Database
	processor  *logprocessor.Processor
	reporter   *reporting.Reporter
	cron       *cron.Cron
	router     *mux.Router
	logger     *logrus.Logger
}

func NewServer(cfg *config.Config) (*Server, error) {
	// Initialize logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetLevel(logrus.InfoLevel)

	// Initialize database
	db, err := database.NewDatabase(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize log processor
	processor := logprocessor.NewProcessor(10) // 10 workers

	// Initialize reporter
	reporter, err := reporting.NewReporter("web/templates", "reports")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize reporter: %w", err)
	}

	// Initialize cron scheduler
	cronScheduler := cron.New(cron.WithSeconds())

	server := &Server{
		config:    cfg,
		db:        db,
		processor: processor,
		reporter:  reporter,
		cron:      cronScheduler,
		router:    mux.NewRouter(),
		logger:    logger,
	}

	// Setup routes
	server.setupRoutes()

	// Setup cron jobs
	server.setupCronJobs()

	return server, nil
}

func (s *Server) setupRoutes() {
	// Health check
	s.router.HandleFunc("/health", s.healthHandler).Methods("GET")
	
	// API routes
	api := s.router.PathPrefix("/api/v1").Subrouter()
	
	// Log processing
	api.HandleFunc("/logs/upload", s.uploadLogHandler).Methods("POST")
	api.HandleFunc("/logs", s.getLogsHandler).Methods("GET")
	api.HandleFunc("/logs/stats", s.getLogStatsHandler).Methods("GET")
	
	// Reports
	api.HandleFunc("/reports/generate", s.generateReportHandler).Methods("POST")
	api.HandleFunc("/reports", s.listReportsHandler).Methods("GET")
	api.HandleFunc("/reports/{id}", s.downloadReportHandler).Methods("GET")
	
	// Database stats
	api.HandleFunc("/stats", s.getDatabaseStatsHandler).Methods("GET")
	
	// Static files (reports)
	s.router.PathPrefix("/reports/").Handler(http.StripPrefix("/reports/", http.FileServer(http.Dir("reports"))))
	
	// Middleware
	s.router.Use(s.loggingMiddleware)
	s.router.Use(s.corsMiddleware)
}

func (s *Server) setupCronJobs() {
	// Daily report generation at 2 AM
	s.cron.AddFunc("0 2 * * *", func() {
		s.logger.Info("Starting scheduled daily report generation")
		if err := s.generateDailyReport(); err != nil {
			s.logger.Errorf("Failed to generate daily report: %v", err)
		}
	})

	// Weekly summary report every Sunday at 3 AM
	s.cron.AddFunc("0 3 * * 0", func() {
		s.logger.Info("Starting scheduled weekly report generation")
		if err := s.generateWeeklyReport(); err != nil {
			s.logger.Errorf("Failed to generate weekly report: %v", err)
		}
	})

	// Database cleanup every month (remove logs older than 90 days)
	s.cron.AddFunc("0 4 1 * *", func() {
		s.logger.Info("Starting scheduled database cleanup")
		if err := s.cleanupOldLogs(); err != nil {
			s.logger.Errorf("Failed to cleanup old logs: %v", err)
		}
	})

	s.cron.Start()
	s.logger.Info("Cron scheduler started")
}

// HTTP Handlers
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"version":   "1.0.0",
	}

	// Check database health
	if err := s.db.HealthCheck(); err != nil {
		health["status"] = "unhealthy"
		health["database_error"] = err.Error()
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	json.NewEncoder(w).Encode(health)
}

func (s *Server) uploadLogHandler(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("logfile")
	if err != nil {
		http.Error(w, "No log file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	logType := r.FormValue("log_type")
	if logType == "" {
		logType = "generic"
	}

	// Validate log type
	if logType != "apache" && logType != "nginx" && logType != "generic" {
		http.Error(w, "Invalid log type. Must be apache, nginx, or generic", http.StatusBadRequest)
		return
	}

	s.logger.Infof("Processing log file: %s, type: %s", header.Filename, logType)

	// Process the log file
	go func() {
		if err := s.processLogFile(file, logType); err != nil {
			s.logger.Errorf("Failed to process log file: %v", err)
		}
	}()

	response := map[string]interface{}{
		"message":   "Log file uploaded successfully",
		"filename":  header.Filename,
		"log_type":  logType,
		"status":    "processing",
	}

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(response)
}

func (s *Server) getLogsHandler(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	logType := r.URL.Query().Get("log_type")
	statusCodeStr := r.URL.Query().Get("status_code")
	sourceIP := r.URL.Query().Get("source_ip")
	path := r.URL.Query().Get("path")
	method := r.URL.Query().Get("method")

	limit := 100 // default limit
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	offset := 0
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Build query
	query := "SELECT * FROM log_entries WHERE 1=1"
	args := []interface{}{}
	argCount := 1

	if logType != "" {
		query += fmt.Sprintf(" AND log_type = $%d", argCount)
		args = append(args, logType)
		argCount++
	}

	if statusCodeStr != "" {
		if statusCode, err := strconv.Atoi(statusCodeStr); err == nil {
			query += fmt.Sprintf(" AND status_code = $%d", argCount)
			args = append(args, statusCode)
			argCount++
		}
	}

	if sourceIP != "" {
		query += fmt.Sprintf(" AND source_ip = $%d", argCount)
		args = append(args, sourceIP)
		argCount++
	}

	if path != "" {
		query += fmt.Sprintf(" AND path LIKE $%d", argCount)
		args = append(args, "%"+path+"%")
		argCount++
	}

	if method != "" {
		query += fmt.Sprintf(" AND method = $%d", argCount)
		args = append(args, method)
		argCount++
	}

	query += " ORDER BY timestamp DESC LIMIT $" + strconv.Itoa(argCount) + " OFFSET $" + strconv.Itoa(argCount+1)
	args = append(args, limit, offset)

	// Execute query
	rows, err := s.db.DB.Query(query, args...)
	if err != nil {
		s.logger.Errorf("Failed to query logs: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var logs []*models.LogEntry
	for rows.Next() {
		var entry models.LogEntry
		if err := rows.Scan(
			&entry.ID, &entry.Timestamp, &entry.LogType, &entry.SourceIP,
			&entry.Method, &entry.Path, &entry.StatusCode, &entry.ResponseSize,
			&entry.UserAgent, &entry.Referer, &entry.ProcessingTime,
			&entry.RawLog, &entry.Metadata, &entry.CreatedAt, &entry.UpdatedAt,
		); err != nil {
			s.logger.Errorf("Failed to scan log entry: %v", err)
			continue
		}
		logs = append(logs, &entry)
	}

	response := map[string]interface{}{
		"logs":   logs,
		"limit":  limit,
		"offset": offset,
		"count":  len(logs),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) getLogStatsHandler(w http.ResponseWriter, r *http.Request) {
	// Get basic stats from database
	stats, err := s.db.GetStats()
	if err != nil {
		s.logger.Errorf("Failed to get database stats: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Get processing stats
	procStats := s.processor.GetStats()

	response := map[string]interface{}{
		"database": stats,
		"processing": map[string]interface{}{
			"total_processed":  procStats.TotalProcessed,
			"apache_processed": procStats.ApacheProcessed,
			"nginx_processed":  procStats.NginxProcessed,
			"generic_processed": procStats.GenericProcessed,
			"errors":           procStats.Errors,
			"start_time":       procStats.StartTime,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) generateReportHandler(w http.ResponseWriter, r *http.Request) {
	var request struct {
		ReportName string           `json:"report_name"`
		LogType    string           `json:"log_type"`
		StartTime  *time.Time       `json:"start_time"`
		EndTime    *time.Time       `json:"end_time"`
		Format     string           `json:"format"` // html, csv, both
		Filters    *models.LogFilter `json:"filters"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if request.ReportName == "" {
		request.ReportName = "log_analysis"
	}

	if request.Format == "" {
		request.Format = "both"
	}

	// Get logs based on filters
	logs, err := s.getLogsForReport(request.Filters)
	if err != nil {
		s.logger.Errorf("Failed to get logs for report: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Prepare report data
	reportData := &reporting.ReportData{
		Title:      request.ReportName,
		GeneratedAt: time.Now(),
		LogEntries:  logs,
		Filters:     request.Filters,
	}

	// Generate reports
	var generatedFiles []string
	if request.Format == "html" || request.Format == "both" {
		htmlFile, err := s.reporter.GenerateHTMLReport(reportData, request.ReportName)
		if err != nil {
			s.logger.Errorf("Failed to generate HTML report: %v", err)
		} else {
			generatedFiles = append(generatedFiles, htmlFile)
		}
	}

	if request.Format == "csv" || request.Format == "both" {
		csvFile, err := s.reporter.GenerateCSVReport(reportData, request.ReportName)
		if err != nil {
			s.logger.Errorf("Failed to generate CSV report: %v", err)
		} else {
			generatedFiles = append(generatedFiles, csvFile)
		}
	}

	response := map[string]interface{}{
		"message":        "Reports generated successfully",
		"generated_files": generatedFiles,
		"format":         request.Format,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

func (s *Server) listReportsHandler(w http.ResponseWriter, r *http.Request) {
	// List available reports from reports directory
	reportsDir := "reports"
	files, err := os.ReadDir(reportsDir)
	if err != nil {
		s.logger.Errorf("Failed to read reports directory: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var reports []map[string]interface{}
	for _, file := range files {
		if !file.IsDir() {
			info, err := file.Info()
			if err != nil {
				continue
			}

			reports = append(reports, map[string]interface{}{
				"filename":    file.Name(),
				"size":        info.Size(),
				"created_at":  info.ModTime(),
				"type":        strings.TrimPrefix(filepath.Ext(file.Name()), "."),
			})
		}
	}

	response := map[string]interface{}{
		"reports": reports,
		"count":   len(reports),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *Server) downloadReportHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	reportID := vars["id"]

	// Construct file path
	filePath := filepath.Join("reports", reportID)
	
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "Report not found", http.StatusNotFound)
		return
	}

	// Serve the file
	http.ServeFile(w, r, filePath)
}

func (s *Server) getDatabaseStatsHandler(w http.ResponseWriter, r *http.Request) {
	stats, err := s.db.GetStats()
	if err != nil {
		s.logger.Errorf("Failed to get database stats: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// Helper methods
func (s *Server) processLogFile(file *os.File, logType string) error {
	// Reset file pointer
	if _, err := file.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek file: %w", err)
	}

	// Process the file
	if err := s.processor.ProcessFile(file, logType); err != nil {
		return fmt.Errorf("failed to process file: %w", err)
	}

	// Store processed logs in database
	go s.storeProcessedLogs()

	return nil
}

func (s *Server) storeProcessedLogs() {
	for entry := range s.processor.GetProcessedLogs() {
		if err := s.storeLogEntry(entry); err != nil {
			s.logger.Errorf("Failed to store log entry: %v", err)
		}
	}
}

func (s *Server) storeLogEntry(entry *models.LogEntry) error {
	query := `
		INSERT INTO log_entries (
			timestamp, log_type, source_ip, method, path, status_code,
			response_size, user_agent, referer, processing_time, raw_log, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.DB.Exec(query,
		entry.Timestamp, entry.LogType, entry.SourceIP, entry.Method,
		entry.Path, entry.StatusCode, entry.ResponseSize, entry.UserAgent,
		entry.Referer, entry.ProcessingTime, entry.RawLog, entry.Metadata,
	)

	return err
}

func (s *Server) getLogsForReport(filters *models.LogFilter) ([]*models.LogEntry, error) {
	// Implementation for getting logs with filters
	// This is a simplified version - you might want to add more sophisticated filtering
	query := "SELECT * FROM log_entries ORDER BY timestamp DESC LIMIT 1000"
	
	rows, err := s.db.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*models.LogEntry
	for rows.Next() {
		var entry models.LogEntry
		if err := rows.Scan(
			&entry.ID, &entry.Timestamp, &entry.LogType, &entry.SourceIP,
			&entry.Method, &entry.Path, &entry.StatusCode, &entry.ResponseSize,
			&entry.UserAgent, &entry.Referer, &entry.ProcessingTime,
			&entry.RawLog, &entry.Metadata, &entry.CreatedAt, &entry.UpdatedAt,
		); err != nil {
			continue
		}
		logs = append(logs, &entry)
	}

	return logs, nil
}

func (s *Server) generateDailyReport() error {
	// Generate daily report for the previous day
	yesterday := time.Now().AddDate(0, 0, -1)
	
	reportData := &reporting.ReportData{
		Title:       "Daily Log Analysis Report",
		GeneratedAt: time.Now(),
		TimeRange:   fmt.Sprintf("%s to %s", yesterday.Format("2006-01-02"), time.Now().Format("2006-01-02")),
	}

	// Get logs for yesterday
	logs, err := s.getLogsForReport(&models.LogFilter{
		StartTime: &yesterday,
		EndTime:   &time.Now(),
	})
	if err != nil {
		return err
	}

	reportData.LogEntries = logs

	// Generate report
	_, err = s.reporter.GenerateCombinedReport(reportData, "daily")
	return err
}

func (s *Server) generateWeeklyReport() error {
	// Generate weekly report for the previous week
	now := time.Now()
	weekStart := now.AddDate(0, 0, -int(now.Weekday())-7)
	weekEnd := weekStart.AddDate(0, 0, 7)

	reportData := &reporting.ReportData{
		Title:       "Weekly Log Analysis Report",
		GeneratedAt: time.Now(),
		TimeRange:   fmt.Sprintf("%s to %s", weekStart.Format("2006-01-02"), weekEnd.Format("2006-01-02")),
	}

	// Get logs for the week
	logs, err := s.getLogsForReport(&models.LogFilter{
		StartTime: &weekStart,
		EndTime:   &weekEnd,
	})
	if err != nil {
		return err
	}

	reportData.LogEntries = logs

	// Generate report
	_, err = s.reporter.GenerateCombinedReport(reportData, "weekly")
	return err
}

func (s *Server) cleanupOldLogs() error {
	// Remove logs older than 90 days
	cutoffDate := time.Now().AddDate(0, 0, -90)
	
	query := "DELETE FROM log_entries WHERE timestamp < ?"
	result, err := s.db.DB.Exec(query, cutoffDate)
	if err != nil {
		return err
	}

	deletedCount, _ := result.RowsAffected()
	s.logger.Infof("Cleaned up %d old log entries", deletedCount)
	
	return nil
}

// Middleware
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Create response writer wrapper to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		
		next.ServeHTTP(wrapped, r)
		
		duration := time.Since(start)
		
		s.logger.WithFields(logrus.Fields{
			"method":     r.Method,
			"path":       r.URL.Path,
			"status":     wrapped.statusCode,
			"duration":   duration,
			"user_agent": r.UserAgent(),
			"remote_ip":  r.RemoteAddr,
		}).Info("HTTP Request")
	})
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	return rw.ResponseWriter.Write(b)
}

func (s *Server) Start() error {
	// Create logs directory
	if err := os.MkdirAll("logs", 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Create reports directory
	if err := os.MkdirAll("reports", 0755); err != nil {
		return fmt.Errorf("failed to create reports directory: %w", err)
	}

	// Start server
	server := &http.Server{
		Addr:         ":" + s.config.Server.Port,
		Handler:      s.router,
		ReadTimeout:  time.Duration(s.config.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(s.config.Server.WriteTimeout) * time.Second,
	}

	// Start server in goroutine
	go func() {
		s.logger.Infof("Starting server on port %s", s.config.Server.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	s.logger.Info("Shutting down server...")

	// Stop cron scheduler
	ctx := s.cron.Stop()
	<-ctx.Done()

	// Shutdown server gracefully
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		s.logger.Errorf("Server forced to shutdown: %v", err)
	}

	// Close database connection
	if err := s.db.Close(); err != nil {
		s.logger.Errorf("Failed to close database: %v", err)
	}

	s.logger.Info("Server stopped")
	return nil
}

func main() {
	// Load configuration
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create and start server
	server, err := NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	if err := server.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
