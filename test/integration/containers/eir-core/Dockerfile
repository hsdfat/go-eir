# Dockerfile for EIR Core Application (Test Version with Mock Data)
# Multi-stage build for production-grade container

# Stage 1: Build
FROM golang:1.25.3-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy entire workspace (context is /Users/loannt70/Documents/phatlc/telco)
COPY . /build

# Download diam-gw dependencies first
WORKDIR /build/diam-gw
RUN go mod download

# Set working directory to EIR
WORKDIR /build/eir

# Download parent module dependencies
RUN go mod download

# Build the EIR Core test application from the workspace root
# This avoids go.mod/go.sum issues in the nested module
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /build/eir-core \
    ./test/integration/containers/eir-core/main.go

# Stage 2: Runtime
FROM alpine:3.18

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata curl

# Create non-root user
RUN addgroup -g 1000 eir && \
    adduser -D -u 1000 -G eir eir

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/eir-core /app/eir-core

# Change ownership
RUN chown -R eir:eir /app

# Switch to non-root user
USER eir

# Expose ports
# 8080: HTTP API
# 3868: Diameter S13
# 9090: Prometheus metrics
EXPOSE 8080 3868 9090

# Health check
HEALTHCHECK --interval=10s --timeout=3s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Run the application
ENTRYPOINT ["/app/eir-core"]
