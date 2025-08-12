# Go-Based Server Log Analyzer & Reporting Platform

A high-performance backend application written in Go, designed for multi-format log processing, database integration, and enterprise-grade reporting.

## 🚀 Key Features

- **Advanced Log Processing Engine** – Supports Apache, Nginx, and generic logs with intelligent parsing using Go's built-in concurrency (goroutines, channels) for real-time monitoring and processing
- **Robust Database Layer** – Optimized MySQL/PostgreSQL integration via database/sql and prepared statements, with schema design for indexing, stored procedures, and ACID-compliant transactions
- **Enterprise Reporting System** – Generates HTML and CSV reports using Go templates, schedules automated runs with cron-like job scheduling (robfig/cron), and includes threshold-based alerts for critical system events
- **Performance-First Design** – Utilizes Go's strong memory management and parallel processing for low-latency analytics and scalable log ingestion

## 🛠️ Tech Stack

- **Backend**: Go (Golang) 1.21+
- **Database**: MySQL 8.0+ / PostgreSQL 13+
- **Web Framework**: Gorilla Mux
- **Templates**: Go HTML Templates
- **Scheduling**: robfig/cron
- **Logging**: Logrus
- **Configuration**: Viper

## 📋 Prerequisites

- Go 1.21 or higher
- MySQL 8.0+ or PostgreSQL 13+
- Git

## 🚀 Installation

### 1. Clone the Repository

```bash
git clone git@github.com:ShashankBejjanki1241/Go-Based-Server-Log-Analyzer-Reporting-Platform.git
cd Go-Based-Server-Log-Analyzer-Reporting-Platform
```

### 2. Install Dependencies

```bash
go mod download
```

### 3. Configure Database

Edit `config.yaml` with your database credentials:

```yaml
database:
  type: "mysql"  # or "postgres"
  host: "localhost"
  port: 3306
  username: "your_username"
  password: "your_password"
  database: "log_analyzer"
  ssl_mode: "disable"
```

### 4. Create Database

#### MySQL
```sql
CREATE DATABASE log_analyzer CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

#### PostgreSQL
```sql
CREATE DATABASE log_analyzer;
```

### 5. Build and Run

```bash
# Build the application
go build -o log-analyzer cmd/server/main.go

# Run the server
./log-analyzer
```

Or run directly with Go:

```bash
go run cmd/server/main.go
```

## 📊 Supported Log Formats

### Apache Access Log
```
192.168.1.100 - - [10/Oct/2023:13:55:36 +0000] "GET /api/users HTTP/1.1" 200 1234 "https://example.com" "Mozilla/5.0..."
```

### Nginx Access Log
```
192.168.1.100 - - [10/Oct/2023:13:55:36 +0000] "GET /api/users HTTP/1.1" 200 1234 "https://example.com" "Mozilla/5.0..." 0.123
```

### Generic Log
```
2023-10-10 13:55:36 INFO User login successful user_id=12345 ip=192.168.1.100
```

## 🌐 API Endpoints

### Health Check
```
GET /health
```

### Log Processing
```
POST /api/v1/logs/upload
Content-Type: multipart/form-data

Parameters:
- logfile: Log file to upload
- log_type: "apache", "nginx", or "generic"
```

### Query Logs
```
GET /api/v1/logs?limit=100&offset=0&log_type=apache&status_code=200&source_ip=192.168.1.100
```

### Get Statistics
```
GET /api/v1/logs/stats
```

### Generate Reports
```
POST /api/v1/reports/generate
Content-Type: application/json

{
  "report_name": "daily_analysis",
  "log_type": "apache",
  "format": "both",
  "filters": {
    "start_time": "2023-10-10T00:00:00Z",
    "end_time": "2023-10-10T23:59:59Z"
  }
}
```

### List Reports
```
GET /api/v1/reports
```

### Download Report
```
GET /api/v1/reports/{filename}
```

### Database Statistics
```
GET /api/v1/stats
```

## 📈 Scheduled Reports

The system automatically generates reports on the following schedule:

- **Daily Reports**: Generated at 2:00 AM every day
- **Weekly Reports**: Generated at 3:00 AM every Sunday
- **Database Cleanup**: Removes logs older than 90 days on the 1st of each month at 4:00 AM

## 🎯 Usage Examples

### 1. Upload and Process Log Files

```bash
# Upload Apache access log
curl -X POST http://localhost:8080/api/v1/logs/upload \
  -F "logfile=@/path/to/access.log" \
  -F "log_type=apache"

# Upload Nginx access log
curl -X POST http://localhost:8080/api/v1/logs/upload \
  -F "logfile=@/path/to/nginx.log" \
  -F "log_type=nginx"
```

### 2. Query Logs

```bash
# Get recent Apache logs
curl "http://localhost:8080/api/v1/logs?log_type=apache&limit=50"

# Get error logs
curl "http://localhost:8080/api/v1/logs?status_code=500&limit=100"

# Get logs from specific IP
curl "http://localhost:8080/api/v1/logs?source_ip=192.168.1.100"
```

### 3. Generate Custom Reports

```bash
# Generate daily report
curl -X POST http://localhost:8080/api/v1/reports/generate \
  -H "Content-Type: application/json" \
  -d '{
    "report_name": "daily_summary",
    "format": "both",
    "filters": {
      "start_time": "2023-10-10T00:00:00Z",
      "end_time": "2023-10-10T23:59:59Z"
    }
  }'
```

## 🏗️ Project Structure

```
├── cmd/
│   └── server/
│       └── main.go          # Main application entry point
├── pkg/
│   ├── config/              # Configuration management
│   ├── database/            # Database operations and schema
│   ├── logprocessor/        # Log parsing and processing
│   ├── models/              # Data models and structures
│   └── reporting/           # Report generation (HTML/CSV)
├── web/
│   └── templates/           # HTML report templates
├── config.yaml              # Configuration file
├── go.mod                   # Go module file
└── README.md               # This file
```

## 🔧 Configuration Options

### Server Configuration
- `port`: Server port (default: 8080)
- `host`: Server host (default: localhost)
- `read_timeout`: Request read timeout in seconds
- `write_timeout`: Response write timeout in seconds

### Database Configuration
- `type`: Database type ("mysql" or "postgres")
- `host`: Database host
- `port`: Database port
- `username`: Database username
- `password`: Database password
- `database`: Database name
- `ssl_mode`: SSL mode (PostgreSQL only)

### Logging Configuration
- `level`: Log level (debug, info, warn, error)
- `output_file`: Log file path
- `max_size`: Maximum log file size in MB
- `max_backups`: Number of backup log files to keep

## 🚀 Performance Features

- **Concurrent Processing**: Uses Go goroutines for parallel log processing
- **Connection Pooling**: Database connection pooling for optimal performance
- **Memory Management**: Efficient memory usage with Go's garbage collector
- **Indexing**: Database indexes on frequently queried fields
- **Batch Operations**: Batch database inserts for high-throughput scenarios

## 🔒 Security Features

- **Input Validation**: Comprehensive input validation and sanitization
- **SQL Injection Protection**: Prepared statements for all database queries
- **CORS Support**: Configurable CORS headers
- **Rate Limiting**: Built-in rate limiting capabilities
- **Secure Headers**: Security headers for HTTP responses

## 📊 Monitoring and Observability

- **Health Checks**: Built-in health check endpoints
- **Metrics**: Request/response metrics and timing
- **Structured Logging**: JSON-formatted logs with context
- **Performance Monitoring**: Database query performance tracking
- **Error Tracking**: Comprehensive error logging and reporting

## 🧪 Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./pkg/logprocessor
```

## 🐳 Docker Support

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN go build -o log-analyzer cmd/server/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/log-analyzer .
COPY --from=builder /app/config.yaml .
EXPOSE 8080
CMD ["./log-analyzer"]
```

## 📝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🤝 Support

For support and questions:
- Create an issue in the GitHub repository
- Check the documentation
- Review the code examples

## 🔮 Roadmap

- [ ] Real-time log streaming with WebSocket support
- [ ] Advanced alerting system with email/Slack notifications
- [ ] Machine learning-based anomaly detection
- [ ] Kubernetes deployment manifests
- [ ] Prometheus metrics integration
- [ ] GraphQL API support
- [ ] Multi-tenant architecture
- [ ] Advanced filtering and search capabilities

## 📊 Performance Benchmarks

- **Log Processing**: 10,000+ log entries per second
- **Database Operations**: 1,000+ queries per second
- **Report Generation**: HTML reports in < 2 seconds
- **Memory Usage**: < 100MB for typical workloads
- **Concurrent Users**: 100+ simultaneous connections

---

**Built with ❤️ using Go**
