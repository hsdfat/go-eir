# Dockerfile for Performance Test Runner
# Runs performance tests against the EIR system

FROM golang:1.25.3-alpine

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy entire workspace
COPY . /app

# Download diam-gw dependencies
WORKDIR /app/diam-gw
RUN go mod download

# Set working directory to EIR
WORKDIR /app/eir

# Download EIR dependencies
RUN go mod download

# Set working directory to test integration
WORKDIR /app/eir/test/integration

# Set environment variables
ENV DRA_ADDR=simulated-dra:3869
ENV EIR_HTTP_ADDR=eir-core:8080
ENV CGO_ENABLED=0

# Run performance tests
# Wait for services to be ready, then run tests
ENTRYPOINT ["sh", "-c", "echo 'Waiting for services to be ready...' && sleep 5 && go test -v -timeout 15m -run '^TestPerformance' ."]

