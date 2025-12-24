.PHONY: build run test clean docker-build docker-up docker-down db-migrate help migrate migrate-verify migrate-status build-migrate migrate-create-partition

# Application name
APP_NAME = eir

# Build directory
BUILD_DIR = bin

# Database configuration
DATABASE_URL ?= "host=14.225.198.206 user=adong password=adong123 dbname=adongfoodv4 port=5432 sslmode=disable"

# Go parameters
GOCMD = go
GOBUILD = $(GOCMD) build
GOTEST = $(GOCMD) test
GOMOD = $(GOCMD) mod
GOFMT = $(GOCMD) fmt

# Build flags
LDFLAGS = -ldflags="-w -s"

## help: Display this help message
help:
	@echo "Available targets:"
	@echo "  build                  - Build the application binary"
	@echo "  run                    - Run the application locally"
	@echo "  test                   - Run unit tests"
	@echo "  test-coverage          - Run tests with coverage report"
	@echo "  clean                  - Remove build artifacts"
	@echo "  docker-build           - Build Docker image"
	@echo "  docker-up              - Start all services with Docker Compose"
	@echo "  docker-down            - Stop all services"
	@echo "  build-migrate          - Build the migration tool"
	@echo "  migrate                - Run database auto-migration"
	@echo "  migrate-verify         - Run migration and verify schema"
	@echo "  migrate-status         - Show migration status"
	@echo "  migrate-create-partition - Create partitions for a year (e.g., YEAR=2027)"
	@echo "  fmt                    - Format Go code"
	@echo "  lint                   - Run golangci-lint"
	@echo "  deps                   - Download and tidy dependencies"

## build: Build the application binary
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME) ./cmd/eir
	@echo "Build complete: $(BUILD_DIR)/$(APP_NAME)"

## run: Run the application locally
run:
	@echo "Running $(APP_NAME)..."
	@$(GOCMD) run ./cmd/eir/main.go

## test: Run unit tests
test:
	@echo "Running tests..."
	@$(GOTEST) -v -race ./...

## test-coverage: Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	@$(GOTEST) -v -race -coverprofile=coverage.out ./...
	@$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## clean: Remove build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html
	@echo "Clean complete"

## docker-build: Build Docker image
docker-build:
	@echo "Building Docker image..."
	@docker build -t $(APP_NAME):latest .
	@echo "Docker build complete"

## docker-up: Start all services with Docker Compose
docker-up:
	@echo "Starting services..."
	@docker-compose up -d
	@echo "Services started"

## docker-down: Stop all services
docker-down:
	@echo "Stopping services..."
	@docker-compose down
	@echo "Services stopped"

## build-migrate: Build the migration tool
build-migrate:
	@echo "Building migration tool..."
	@mkdir -p $(BUILD_DIR)
	@$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/migrate ./cmd/migrate
	@echo "Migration tool built: $(BUILD_DIR)/migrate"

## migrate: Run database auto-migration
migrate: build-migrate
	@echo "Running database migration with auto-migrate..."
	@./$(BUILD_DIR)/migrate -database-url=$(DATABASE_URL)

## migrate-verify: Run migration and verify schema
migrate-verify: build-migrate
	@echo "Running database migration with verification..."
	@./$(BUILD_DIR)/migrate -database-url=$(DATABASE_URL) -verify

## migrate-status: Show migration status
migrate-status: build-migrate
	@echo "Checking migration status..."
	@./$(BUILD_DIR)/migrate -database-url=$(DATABASE_URL) -status

## migrate-create-partition: Create partitions for a specific year
migrate-create-partition: build-migrate
	@echo "Creating partitions for year $(YEAR)..."
	@./$(BUILD_DIR)/migrate -database-url=$(DATABASE_URL) -create-partition=$(YEAR)

## fmt: Format Go code
fmt:
	@echo "Formatting code..."
	@$(GOFMT) ./...
	@echo "Format complete"

## lint: Run golangci-lint
lint:
	@echo "Running linter..."
	@golangci-lint run ./...
	@echo "Lint complete"

## deps: Download and tidy dependencies
deps:
	@echo "Downloading dependencies..."
	@$(GOMOD) download
	@$(GOMOD) tidy
	@echo "Dependencies updated"
