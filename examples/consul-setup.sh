#!/bin/bash
# Setup script for EIR with Consul configuration

set -e

echo "=== EIR Consul Configuration Setup ==="
echo

# Check if consul is installed
if ! command -v consul &> /dev/null; then
    echo "Consul not found. Installing..."
    echo "On macOS: brew install consul"
    echo "On Linux: see https://www.consul.io/downloads"
    exit 1
fi

# Start Consul in dev mode (for testing)
echo "Starting Consul in development mode..."
consul agent -dev -ui &
CONSUL_PID=$!
echo "Consul started with PID: $CONSUL_PID"
sleep 2

# Store EIR configuration in Consul
echo
echo "Storing EIR production configuration in Consul..."

consul kv put config/eir/production - <<'EOF'
{
  "server": {
    "host": "0.0.0.0",
    "port": 8080,
    "read_timeout": "30s",
    "write_timeout": "30s",
    "idle_timeout": "120s"
  },
  "database": {
    "type": "postgres",
    "host": "postgres.example.com",
    "port": 5432,
    "database": "eir_production",
    "username": "eir_user",
    "ssl_mode": "require",
    "max_open_conns": 100,
    "max_idle_conns": 10,
    "conn_max_lifetime": "5m"
  },
  "diameter": {
    "enabled": true,
    "listen_addr": "0.0.0.0:3868",
    "origin_host": "eir.mobile.example.com",
    "origin_realm": "mobile.example.com",
    "watchdog_interval": "30s",
    "idle_timeout": "300s"
  },
  "cache": {
    "provider": "redis",
    "ttl": "10m",
    "redis_addr": "redis.example.com:6379"
  },
  "logging": {
    "level": "info",
    "format": "json"
  },
  "metrics": {
    "enabled": true,
    "port": 9090,
    "path": "/metrics"
  }
}
EOF

echo "✓ Configuration stored in Consul"

# Verify the configuration
echo
echo "Verifying configuration..."
consul kv get config/eir/production | jq .

echo
echo "=== Setup Complete ==="
echo
echo "Configuration is now stored in Consul at: config/eir/production"
echo "Consul UI available at: http://localhost:8500/ui"
echo
echo "To use this configuration, set environment variables:"
echo "  export EIR_DATABASE_PASSWORD=your_db_password"
echo "  export EIR_CACHE_REDIS_PASSWORD=your_redis_password"
echo
echo "Then run your EIR service with:"
echo "  go run examples/config/main.go"
echo
echo "To update configuration (will trigger hot reload):"
echo "  consul kv put config/eir/production @new-config.json"
echo
echo "To stop Consul:"
echo "  kill $CONSUL_PID"
