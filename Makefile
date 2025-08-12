.PHONY: help build run test clean deps lint docker-build docker-run

# Default target
help:
	@echo "Available commands:"
	@echo "  build       - Build the application"
	@echo "  run         - Run the application"
	@echo "  test        - Run tests"
	@echo "  clean       - Clean build artifacts"
	@echo "  deps        - Download dependencies"
	@echo "  lint        - Run linter"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Run Docker container"

# Build the application
build:
	@echo "Building log analyzer..."
	@go build -o bin/log-analyzer cmd/server/main.go
	@echo "Build complete: bin/log-analyzer"

# Run the application
run:
	@echo "Running log analyzer..."
	@go run cmd/server/main.go

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@rm -rf coverage.out coverage.html
	@go clean

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

# Run linter
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	@docker build -t log-analyzer:latest .

# Run Docker container
docker-run:
	@echo "Running Docker container..."
	@docker run -d \
		--name log-analyzer \
		-p 8080:8080 \
		-v $(PWD)/config.yaml:/root/config.yaml \
		-v $(PWD)/logs:/root/logs \
		-v $(PWD)/reports:/root/reports \
		log-analyzer:latest

# Stop Docker container
docker-stop:
	@echo "Stopping Docker container..."
	@docker stop log-analyzer || true
	@docker rm log-analyzer || true

# Development mode with hot reload (requires air)
dev:
	@if command -v air >/dev/null 2>&1; then \
		air; \
	else \
		echo "air not found. Install with: go install github.com/cosmtrek/air@latest"; \
		echo "Or run: make run"; \
	fi

# Install development tools
install-tools:
	@echo "Installing development tools..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/cosmtrek/air@latest
	@echo "Development tools installed"

# Generate mock data for testing
generate-mocks:
	@echo "Generating mock data..."
	@mkdir -p testdata
	@echo "192.168.1.100 - - [10/Oct/2023:13:55:36 +0000] \"GET /api/users HTTP/1.1\" 200 1234 \"https://example.com\" \"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36\"" > testdata/apache.log
	@echo "192.168.1.101 - - [10/Oct/2023:13:55:37 +0000] \"POST /api/login HTTP/1.1\" 401 567 \"https://example.com\" \"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)\" 0.045" > testdata/nginx.log
	@echo "2023-10-10 13:55:38 INFO User login successful user_id=12345 ip=192.168.1.102" > testdata/generic.log
	@echo "Mock data generated in testdata/ directory"

# Database setup helpers
db-setup-mysql:
	@echo "Setting up MySQL database..."
	@echo "Run the following SQL commands:"
	@echo "CREATE DATABASE log_analyzer CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
	@echo "CREATE USER 'loguser'@'localhost' IDENTIFIED BY 'logpass';"
	@echo "GRANT ALL PRIVILEGES ON log_analyzer.* TO 'loguser'@'localhost';"
	@echo "FLUSH PRIVILEGES;"

db-setup-postgres:
	@echo "Setting up PostgreSQL database..."
	@echo "Run the following SQL commands:"
	@echo "CREATE DATABASE log_analyzer;"
	@echo "CREATE USER loguser WITH PASSWORD 'logpass';"
	@echo "GRANT ALL PRIVILEGES ON DATABASE log_analyzer TO loguser;"

# Performance testing
bench:
	@echo "Running benchmarks..."
	@go test -bench=. ./pkg/logprocessor/
	@go test -bench=. ./pkg/database/

# Security scan
security-scan:
	@echo "Running security scan..."
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "gosec not found. Install with: go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest"; \
	fi

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@go vet ./...

# Check for vulnerabilities
vuln-check:
	@echo "Checking for vulnerabilities..."
	@go list -json -deps ./... | nancy sleuth

# Create release
release:
	@echo "Creating release..."
	@version=$$(git describe --tags --always --dirty); \
	echo "Building version: $$version"; \
	GOOS=linux GOARCH=amd64 go build -ldflags="-X main.version=$$version" -o bin/log-analyzer-linux-amd64 cmd/server/main.go; \
	GOOS=darwin GOARCH=amd64 go build -ldflags="-X main.version=$$version" -o bin/log-analyzer-darwin-amd64 cmd/server/main.go; \
	GOOS=windows GOARCH=amd64 go build -ldflags="-X main.version=$$version" -o bin/log-analyzer-windows-amd64.exe cmd/server/main.go; \
	echo "Release binaries created in bin/ directory"

# Install the application
install:
	@echo "Installing log analyzer..."
	@go install ./cmd/server
	@echo "Installation complete. Run with: log-analyzer"

# Uninstall the application
uninstall:
	@echo "Uninstalling log analyzer..."
	@go clean -i ./cmd/server
	@echo "Uninstallation complete"
