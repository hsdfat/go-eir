# EIR Service Configuration

This directory contains configuration files for the EIR (Equipment Identity Register) service.

## Configuration Files

### config.default.yaml
The default configuration file with sensible defaults for development. This file is used as a fallback when no other configuration file is found.

### config.yaml
Your custom configuration file (not tracked in git). Create this file to override default settings.

### config.example.yaml
Example configuration showing all available options with documentation.

## Configuration Priority

The configuration is loaded in the following priority order (highest to lowest):

1. **Environment Variables** - Prefixed with `EIR_`
   - Example: `EIR_DATABASE_HOST=localhost`
   - Example: `EIR_SERVER_PORT=8080`

2. **Specified Config File** - If passed to `config.Load(configPath)`

3. **config.yaml** - Searched in:
   - Current directory (`.`)
   - `./config` directory
   - `/etc/eir` directory

4. **config.default.yaml** - Fallback configuration with defaults

5. **Hardcoded Defaults** - Built into the application

## Configuration Sections

### Server
HTTP/2 server configuration:
- `host`: Server bind address (default: "0.0.0.0")
- `port`: Server port (default: 8080, must be 1-65535)
- `readTimeout`: Read timeout duration (must be positive)
- `writeTimeout`: Write timeout duration (must be positive)
- `idleTimeout`: Idle timeout duration (must be positive)

### Database
PostgreSQL database configuration:
- `host`: Database host (required)
- `port`: Database port (required, must be 1-65535)
- `user`: Database username (required)
- `password`: Database password (use env var: `EIR_DATABASE_PASSWORD`)
- `database`: Database name (required)
- `sslMode`: SSL mode (must be: disable, require, verify-ca, verify-full)
- `maxOpenConns`: Maximum open connections (must be at least 1)
- `maxIdleConns`: Maximum idle connections (must be non-negative, cannot exceed maxOpenConns)
- `connMaxLifetime`: Connection max lifetime
- `connMaxIdleTime`: Connection max idle time

### Diameter
Diameter S13 interface configuration:
- `host`: Server bind address (required, default: "0.0.0.0")
- `port`: Server port (required, must be 1-65535, default: 3868)
- `originHost`: Diameter origin host (required)
- `originRealm`: Diameter origin realm (required)
- `productName`: Product name (required)
- `vendorID`: Vendor ID
- `maxConnections`: Maximum concurrent connections (must be at least 1)
- `readTimeout`: Read timeout duration (must be non-negative)
- `writeTimeout`: Write timeout duration (must be non-negative)
- `watchdogInterval`: Watchdog interval duration (must be non-negative)
- `watchdogTimeout`: Watchdog timeout duration (must be non-negative)
- `maxMessageSize`: Maximum message size in bytes (must be at least 1)
- `sendChannelSize`: Send channel buffer size (must be at least 1)
- `recvChannelSize`: Receive channel buffer size (must be at least 1)

**Diameter Metrics**: The Diameter server exposes Prometheus metrics:
- `diameter_requests_total`: Total number of Diameter requests (labeled by command and result)
- `diameter_request_duration_seconds`: Request processing duration histogram
- `diameter_active_connections`: Current number of active connections
- `diameter_errors_total`: Total errors (labeled by error type)

### Cache
Cache configuration:
- `enabled`: Enable/disable caching
- `provider`: Cache provider (must be: redis, memcached, inmemory)
- `redis`: Redis-specific settings
  - `host`: Redis host (required when provider is redis)
  - `port`: Redis port (must be 1-65535)
  - `password`: Redis password
  - `db`: Redis database number

### Logging
Logging configuration:
- `level`: Log level (must be: debug, info, warn, error)
- `format`: Log format (must be: json, text)
- `outputPath`: Output path (required, e.g., stdout, stderr, or file path)

### Metrics
Metrics/Prometheus configuration:
- `enabled`: Enable/disable metrics
- `port`: Metrics server port (must be 1-65535 when enabled)
- `path`: Metrics endpoint path (required when enabled, must start with /)

## Configuration Validation

The configuration is automatically validated when loaded. If any validation errors are found, the application will fail to start with a descriptive error message.

Validation checks include:
- **Required fields**: Certain fields like database host, user, and database name must be provided
- **Port ranges**: All port numbers must be between 1 and 65535
- **Valid enums**: Fields like `sslMode`, `logging.level`, `logging.format` must use valid values
- **Logical constraints**: For example, `maxIdleConns` cannot exceed `maxOpenConns`
- **Positive durations**: Timeout values must be positive
- **Path formats**: Metrics path must start with `/`

Example validation errors:
```
Failed to load configuration: invalid configuration: server config: port must be between 1 and 65535, got 70000
Failed to load configuration: invalid configuration: database config: sslMode must be one of: disable, require, verify-ca, verify-full
Failed to load configuration: invalid configuration: logging config: level must be one of: debug, info, warn, error
```

## Usage Examples

### Using Default Configuration
```bash
# Uses config.default.yaml
./eir
```

### Using Custom Configuration File
```bash
# Create your custom config
cp config.example.yaml config.yaml
# Edit config.yaml with your settings
vim config.yaml
# Run the service (will use config.yaml)
./eir
```

### Using Environment Variables
```bash
# Override specific settings via environment
export EIR_DATABASE_HOST=production-db.example.com
export EIR_DATABASE_PASSWORD=secure_password
export EIR_SERVER_PORT=9090
export EIR_LOGGING_LEVEL=debug
./eir
```

### Programmatic Usage
```go
import "github.com/hsdfat8/eir/internal/config"

// Load default config
cfg, err := config.Load("")

// Load specific config file
cfg, err := config.Load("/path/to/config.yaml")

// Access config in service
service := eirService.GetConfig()
fmt.Printf("Server running on %s:%d\n", cfg.Server.Host, cfg.Server.Port)
```

## Security Best Practices

1. **Never commit sensitive data** - Use environment variables for passwords and secrets
2. **Use config.yaml for local overrides** - Add it to `.gitignore`
3. **Use SSL in production** - Set `database.sslMode` to `verify-full`
4. **Rotate credentials regularly** - Update passwords in environment variables
5. **Limit file permissions** - `chmod 600 config.yaml` for production configs

## Environment Variables Reference

All configuration values can be overridden with environment variables using the format:
`EIR_<SECTION>_<KEY>=value`

Examples:
```bash
EIR_SERVER_HOST=0.0.0.0
EIR_SERVER_PORT=8080
EIR_DATABASE_HOST=localhost
EIR_DATABASE_PORT=5432
EIR_DATABASE_USER=eir
EIR_DATABASE_PASSWORD=secret
EIR_DATABASE_DATABASE=eir
EIR_DIAMETER_LISTENADDR=0.0.0.0:3868
EIR_CACHE_ENABLED=true
EIR_LOGGING_LEVEL=debug
EIR_METRICS_ENABLED=true
```
