# Dockerfile for Diameter Gateway (Pure Forwarding Service)
# Multi-stage build for production-grade container

# Stage 1: Build
FROM golang:1.25.3-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /build

# Copy entire workspace (context is /Users/loannt70/Documents/phatlc/telco)
COPY . /build

# Download parent dependencies first
WORKDIR /build/diam-gw
RUN go mod download

# Build the gateway from the workspace root
WORKDIR /build/eir
RUN go mod download

# Build the gateway with optimizations from the eir module root
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=1.0.0" \
    -o /build/diameter-gateway \
    ./test/integration/containers/diameter-gateway/main.go

# Stage 2: Runtime
FROM alpine:3.18

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata curl

# Create non-root user
RUN addgroup -g 1001 gateway && \
    adduser -D -u 1001 -G gateway gateway

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/diameter-gateway /app/diameter-gateway

# Change ownership
RUN chown -R gateway:gateway /app

# Switch to non-root user
USER gateway

# Expose Diameter port
EXPOSE 3868

# Health check
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD netstat -an | grep 3868 || exit 1

# Run the gateway
ENTRYPOINT ["/app/diameter-gateway"]
