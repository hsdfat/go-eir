package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds the application configuration
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Diameter DiameterConfig
	Cache    CacheConfig
	Logging  LoggingConfig
	Metrics  MetricsConfig
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Host         string
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// DatabaseConfig holds PostgreSQL configuration
type DatabaseConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// DiameterConfig holds Diameter server configuration
type DiameterConfig struct {
	ListenAddr  string
	OriginHost  string
	OriginRealm string
	ProductName string
	VendorID    uint32
}

// CacheConfig holds cache configuration
type CacheConfig struct {
	Enabled  bool
	Provider string // "redis", "memcached", "inmemory"
	Redis    RedisConfig
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level      string // "debug", "info", "warn", "error"
	Format     string // "json", "text"
	OutputPath string // "stdout", "stderr", or file path
}

// MetricsConfig holds metrics configuration
type MetricsConfig struct {
	Enabled bool
	Port    int
	Path    string
}

// Load loads configuration from file and environment variables
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set default values
	setDefaults(v)

	// Set config file
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
		v.AddConfigPath("/etc/eir")
	}

	// Read environment variables
	v.AutomaticEnv()
	v.SetEnvPrefix("EIR")

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
		// Config file not found; using defaults and environment variables
	}

	// Unmarshal config
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

// setDefaults sets default configuration values
func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.readTimeout", "30s")
	v.SetDefault("server.writeTimeout", "30s")
	v.SetDefault("server.idleTimeout", "120s")

	// Database defaults
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.user", "eir")
	v.SetDefault("database.password", "eir")
	v.SetDefault("database.database", "eir")
	v.SetDefault("database.sslMode", "disable")
	v.SetDefault("database.maxOpenConns", 25)
	v.SetDefault("database.maxIdleConns", 5)
	v.SetDefault("database.connMaxLifetime", "5m")
	v.SetDefault("database.connMaxIdleTime", "10m")

	// Diameter defaults
	v.SetDefault("diameter.listenAddr", "0.0.0.0:3868")
	v.SetDefault("diameter.originHost", "eir.example.com")
	v.SetDefault("diameter.originRealm", "example.com")
	v.SetDefault("diameter.productName", "Go-EIR")
	v.SetDefault("diameter.vendorID", 10415)

	// Cache defaults
	v.SetDefault("cache.enabled", false)
	v.SetDefault("cache.provider", "redis")
	v.SetDefault("cache.redis.host", "localhost")
	v.SetDefault("cache.redis.port", 6379)
	v.SetDefault("cache.redis.password", "")
	v.SetDefault("cache.redis.db", 0)

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
	v.SetDefault("logging.outputPath", "stdout")

	// Metrics defaults
	v.SetDefault("metrics.enabled", true)
	v.SetDefault("metrics.port", 9090)
	v.SetDefault("metrics.path", "/metrics")
}
