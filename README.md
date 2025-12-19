# Equipment Identity Register (EIR)

A production-grade Equipment Identity Register (EIR) system implemented in Go following hexagonal (clean) architecture principles.

## Overview

This EIR system provides:

- **Diameter S13 Interface** (4G LTE) - ME-Identity-Check operations
- **HTTP N5g-eir Interface** (5G) - Equipment status query API (3GPP TS 29.511)
- **PostgreSQL Storage** - Persistent equipment database with audit logging
- **Hexagonal Architecture** - Clean separation of domain, ports, and adapters
- **Production-Ready** - Observability, metrics, health checks, and graceful shutdown

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     Adapters (Infrastructure)                │
├──────────────────┬──────────────────┬───────────────────────┤
│  Diameter S13    │   HTTP (5G)      │   PostgreSQL          │
│  Gateway         │   Gateway        │   Repository          │
└────────┬─────────┴─────────┬────────┴───────────┬───────────┘
         │                   │                    │
         └───────────────────┼────────────────────┘
                             │
         ┌───────────────────▼────────────────────┐
         │          Ports (Interfaces)            │
         │   - EIRService                         │
         │   - IMEIRepository                     │
         │   - AuditRepository                    │
         └───────────────────┬────────────────────┘
                             │
         ┌───────────────────▼────────────────────┐
         │        Domain Layer (Business Logic)   │
         │   - IMEI Validation (Luhn algorithm)   │
         │   - Policy Evaluation                  │
         │   - Equipment Status Mapping           │
         │   - Audit Event Generation             │
         └────────────────────────────────────────┘
```

### Key Principles

1. **Business logic is isolated** - Domain layer has NO dependencies on infrastructure
2. **Dependency Inversion** - Interfaces (ports) are owned by the domain
3. **Testability** - Each layer can be tested independently
4. **Flexibility** - Easy to swap implementations (e.g., different databases)

## Project Structure

```
eir/
├── cmd/
│   └── eir/
│       └── main.go              # Application entry point
├── internal/
│   ├── domain/                  # Domain layer (business logic)
│   │   ├── models/              # Domain models
│   │   │   └── equipment.go     # Equipment entity, IMEI validation
│   │   ├── ports/               # Interfaces (owned by domain)
│   │   │   ├── repository.go    # Repository interfaces
│   │   │   └── service.go       # Service interfaces
│   │   └── service/             # Business logic implementation
│   │       └── eir_service.go   # Core EIR service
│   ├── adapters/                # Infrastructure adapters
│   │   ├── diameter/            # Diameter S13 adapter
│   │   │   ├── s13_handler.go   # S13 message handler
│   │   │   └── server.go        # Diameter server
│   │   ├── http/                # HTTP adapter (5G N5g-eir)
│   │   │   ├── handler.go       # HTTP handlers
│   │   │   ├── models.go        # HTTP DTOs
│   │   │   └── router.go        # HTTP routes
│   │   └── postgres/            # PostgreSQL adapter
│   │       ├── db.go            # Database connection
│   │       ├── imei_repository.go
│   │       ├── audit_repository.go
│   │       └── schema.sql       # Database schema
│   ├── config/                  # Configuration management
│   │   └── config.go
│   └── observability/           # Logging & metrics
│       ├── logger.go
│       └── metrics.go
├── config/
│   └── config.yaml              # Configuration file
├── deploy/
│   └── prometheus.yml           # Prometheus configuration
├── Dockerfile                   # Multi-stage Docker build
├── docker-compose.yaml          # Docker Compose setup
├── Makefile                     # Build automation
└── README.md
```

## Quick Start

### Prerequisites

- Go 1.21+
- PostgreSQL 15+
- Docker & Docker Compose (optional)

### Running with Docker Compose

```bash
# Start all services (PostgreSQL, Redis, EIR, Prometheus, Grafana)
docker-compose up -d

# Check logs
docker-compose logs -f eir

# Stop services
docker-compose down
```

The services will be available at:
- **EIR HTTP API**: http://localhost:8080
- **Diameter S13**: localhost:3868
- **Prometheus Metrics**: http://localhost:9090
- **Grafana Dashboard**: http://localhost:3000 (admin/admin)

### Running Locally

1. **Start PostgreSQL**:
```bash
docker run -d \
  --name eir-postgres \
  -e POSTGRES_USER=eir \
  -e POSTGRES_PASSWORD=eir_password \
  -e POSTGRES_DB=eir \
  -p 5432:5432 \
  postgres:15-alpine
```

2. **Apply database schema**:
```bash
make db-migrate
# Or manually:
psql -h localhost -U eir -d eir -f internal/adapters/postgres/schema.sql
```

3. **Run the application**:
```bash
make run
# Or:
go run cmd/eir/main.go
```

### Building

```bash
# Build binary
make build

# Run tests
make test

# Run tests with coverage
make test-coverage

# Format code
make fmt

# Download dependencies
make deps
```

## API Usage

### 5G HTTP Interface (N5g-eir)

**Check Equipment Status**:
```bash
curl "http://localhost:8080/n5g-eir-eic/v1/equipment-status?pei=123456789012345"
```

Response:
```json
{
  "status": "WHITELISTED"
}
```

### Management API (Provisioning)

**Provision Equipment**:
```bash
curl -X POST http://localhost:8080/api/v1/equipment \
  -H "Content-Type: application/json" \
  -d '{
    "imei": "123456789012345",
    "status": "BLACKLISTED",
    "reason": "Reported stolen"
  }'
```

**Get Equipment**:
```bash
curl http://localhost:8080/api/v1/equipment/123456789012345
```

**List Equipment**:
```bash
curl "http://localhost:8080/api/v1/equipment?offset=0&limit=100"
```

**Delete Equipment**:
```bash
curl -X DELETE http://localhost:8080/api/v1/equipment/123456789012345
```

### Diameter S13 Interface

The Diameter S13 interface listens on port 3868 and supports:

- **ME-Identity-Check-Request (MICR)** - Command Code 324
- **ME-Identity-Check-Answer (MICA)** - Returns equipment status

Example flow:
1. MME sends MICR with IMEI in Terminal-Information AVP
2. EIR validates IMEI and checks database
3. EIR returns MICA with Equipment-Status AVP (0=WHITELISTED, 1=BLACKLISTED, 2=GREYLISTED)

## Database Schema

### Equipment Table
- **Primary storage** for IMEI records
- Indexed by IMEI, status, manufacturer TAC
- Atomic check count increment via stored procedure

### Audit Log Table
- **Partitioned by time** (quarterly partitions)
- Records all equipment check operations
- Indexed by IMEI, check_time, status

## Configuration

Configuration can be provided via:
1. YAML file (`config/config.yaml`)
2. Environment variables (prefixed with `EIR_`)

Example environment variables:
```bash
export EIR_DATABASE_HOST=localhost
export EIR_DATABASE_PORT=5432
export EIR_DATABASE_USER=eir
export EIR_DATABASE_PASSWORD=eir_password
export EIR_DIAMETER_ORIGINHOST=eir.example.com
export EIR_LOGGING_LEVEL=debug
```

## Observability

### Metrics

Prometheus metrics exposed at `/metrics`:

- `eir_equipment_check_total` - Total check requests by source and status
- `eir_equipment_check_duration_seconds` - Check latency histogram
- `eir_database_query_duration_seconds` - Database query latency
- `eir_cache_hit_total` - Cache hit/miss counts
- `eir_active_diameter_connections` - Active Diameter connections

### Logging

Structured JSON logging with configurable levels:
- `debug`, `info`, `warn`, `error`

### Health Check

```bash
curl http://localhost:8080/health
```

## IMEI Validation

The system performs strict IMEI validation:

1. **Length check**: 14-16 digits
2. **Format check**: Numeric only
3. **Luhn algorithm**: Validates check digit for 15-digit IMEIs
4. **TAC extraction**: Extracts Type Allocation Code (first 8 digits)

## Default Policy

**Unknown equipment** (not in database) is **WHITELISTED** by default.

This is a permissive policy. For restrictive policy, modify `applyDefaultPolicy()` in `eir_service.go` to return `BLACKLISTED` or `GREYLISTED`.

## Security Considerations

- Database credentials should be stored in secrets management (e.g., Vault, AWS Secrets Manager)
- Enable TLS for Diameter connections in production
- Enable PostgreSQL SSL mode in production
- Use authentication middleware for HTTP provisioning API
- Implement rate limiting for public APIs

## Performance

Production optimizations:
- Connection pooling (PostgreSQL, Redis)
- Database indexing strategy
- Optional caching layer (Redis)
- Asynchronous audit logging
- Partitioned audit tables for scalability

## Testing

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Run specific package
go test -v ./internal/domain/service/
```

## Troubleshooting

**Database connection fails**:
- Check PostgreSQL is running
- Verify credentials in config
- Check network connectivity

**Diameter connection fails**:
- Verify port 3868 is not in use
- Check firewall rules
- Verify origin-host/realm configuration

**High latency**:
- Enable Redis caching
- Review database query performance
- Check connection pool settings

## License

Copyright © 2024. All rights reserved.

## Contributing

This is a production system. All changes require:
1. Unit tests with >80% coverage
2. Integration tests for adapters
3. Documentation updates
4. Security review for sensitive changes

## Support

For issues and questions, contact the Telecom Core Network team.
