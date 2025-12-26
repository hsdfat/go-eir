package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds the application configuration
type Config struct {
	Server     ServerConfig
	Database   DatabaseConfig
	Diameter   DiameterConfig
	Cache      CacheConfig
	Logging    LoggingConfig
	Metrics    MetricsConfig
	Governance GovernanceConfig
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
	Host             string
	Port             int
	OriginHost       string
	OriginRealm      string
	ProductName      string
	VendorID         uint32
	MaxConnections   int
	ReadTimeout      time.Duration
	WriteTimeout     time.Duration
	WatchdogInterval time.Duration
	WatchdogTimeout  time.Duration
	MaxMessageSize   int
	SendChannelSize  int
	RecvChannelSize  int
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

// GovernanceConfig holds governance/service discovery configuration
type GovernanceConfig struct {
	Enabled     bool   // Enable/disable governance registration
	URL         string // Governance manager URL
	FailOnError bool   // Panic if registration fails when enabled
}

// Load loads configuration from file and environment variables
// Priority order (highest to lowest):
// 1. Environment variables (prefixed with EIR_)
// 2. Config file specified by configPath
// 3. config.yaml in standard paths
// 4. config.default.yaml as fallback
// 5. Hardcoded defaults
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set default values (lowest priority)
	setDefaults(v)

	// Set config file paths
	if configPath != "" {
		// Use specified config file
		v.SetConfigFile(configPath)
	} else {
		// Search for config.yaml first, then fall back to config.default.yaml
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
		v.AddConfigPath("/etc/eir")
	}

	// Read environment variables (highest priority)
	v.AutomaticEnv()
	v.SetEnvPrefix("EIR")

	// Try to read config file
	configFileRead := false
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found, try default config
			v.SetConfigName("config.default")
			if err := v.ReadInConfig(); err != nil {
				if _, ok := err.(viper.ConfigFileNotFoundError); ok {
					// No config files found; using defaults and environment variables
					fmt.Println("Warning: No config file found, using defaults and environment variables")
				} else {
					return nil, fmt.Errorf("failed to read default config file: %w", err)
				}
			} else {
				configFileRead = true
			}
		} else {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	} else {
		configFileRead = true
	}

	if configFileRead {
		fmt.Printf("Using config file: %s\n", v.ConfigFileUsed())
	}

	// Unmarshal config
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate config
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
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
	v.SetDefault("diameter.host", "0.0.0.0")
	v.SetDefault("diameter.port", 3868)
	v.SetDefault("diameter.originHost", "eir.example.com")
	v.SetDefault("diameter.originRealm", "example.com")
	v.SetDefault("diameter.productName", "Go-EIR")
	v.SetDefault("diameter.vendorID", 10415)
	v.SetDefault("diameter.maxConnections", 1000)
	v.SetDefault("diameter.readTimeout", "30s")
	v.SetDefault("diameter.writeTimeout", "10s")
	v.SetDefault("diameter.watchdogInterval", "30s")
	v.SetDefault("diameter.watchdogTimeout", "10s")
	v.SetDefault("diameter.maxMessageSize", 65535)
	v.SetDefault("diameter.sendChannelSize", 100)
	v.SetDefault("diameter.recvChannelSize", 100)

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

	// Governance defaults
	v.SetDefault("governance.enabled", true)
	v.SetDefault("governance.url", "http://telco-governance:8080")
	v.SetDefault("governance.failOnError", true)
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate Server configuration
	if err := c.Server.Validate(); err != nil {
		return fmt.Errorf("server config: %w", err)
	}

	// Validate Database configuration
	if err := c.Database.Validate(); err != nil {
		return fmt.Errorf("database config: %w", err)
	}

	// Validate Diameter configuration
	if err := c.Diameter.Validate(); err != nil {
		return fmt.Errorf("diameter config: %w", err)
	}

	// Validate Cache configuration
	if err := c.Cache.Validate(); err != nil {
		return fmt.Errorf("cache config: %w", err)
	}

	// Validate Logging configuration
	if err := c.Logging.Validate(); err != nil {
		return fmt.Errorf("logging config: %w", err)
	}

	// Validate Metrics configuration
	if err := c.Metrics.Validate(); err != nil {
		return fmt.Errorf("metrics config: %w", err)
	}

	// Validate Governance configuration
	if err := c.Governance.Validate(); err != nil {
		return fmt.Errorf("governance config: %w", err)
	}

	return nil
}

// Validate validates the ServerConfig
func (c *ServerConfig) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", c.Port)
	}
	if c.ReadTimeout < 0 {
		return fmt.Errorf("readTimeout must be positive")
	}
	if c.WriteTimeout < 0 {
		return fmt.Errorf("writeTimeout must be positive")
	}
	if c.IdleTimeout < 0 {
		return fmt.Errorf("idleTimeout must be positive")
	}
	return nil
}

// Validate validates the DatabaseConfig
func (c *DatabaseConfig) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("host is required")
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", c.Port)
	}
	if c.User == "" {
		return fmt.Errorf("user is required")
	}
	if c.Database == "" {
		return fmt.Errorf("database is required")
	}
	validSSLModes := map[string]bool{
		"disable":     true,
		"require":     true,
		"verify-ca":   true,
		"verify-full": true,
	}
	if !validSSLModes[c.SSLMode] {
		return fmt.Errorf("sslMode must be one of: disable, require, verify-ca, verify-full")
	}
	if c.MaxOpenConns < 1 {
		return fmt.Errorf("maxOpenConns must be at least 1")
	}
	if c.MaxIdleConns < 0 {
		return fmt.Errorf("maxIdleConns must be non-negative")
	}
	if c.MaxIdleConns > c.MaxOpenConns {
		return fmt.Errorf("maxIdleConns (%d) cannot exceed maxOpenConns (%d)", c.MaxIdleConns, c.MaxOpenConns)
	}
	return nil
}

// Validate validates the DiameterConfig
func (c *DiameterConfig) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("host is required")
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", c.Port)
	}
	if c.OriginHost == "" {
		return fmt.Errorf("originHost is required")
	}
	if c.OriginRealm == "" {
		return fmt.Errorf("originRealm is required")
	}
	if c.ProductName == "" {
		return fmt.Errorf("productName is required")
	}
	if c.MaxConnections < 1 {
		return fmt.Errorf("maxConnections must be at least 1")
	}
	if c.ReadTimeout < 0 {
		return fmt.Errorf("readTimeout must be non-negative")
	}
	if c.WriteTimeout < 0 {
		return fmt.Errorf("writeTimeout must be non-negative")
	}
	if c.WatchdogInterval < 0 {
		return fmt.Errorf("watchdogInterval must be non-negative")
	}
	if c.WatchdogTimeout < 0 {
		return fmt.Errorf("watchdogTimeout must be non-negative")
	}
	if c.MaxMessageSize < 1 {
		return fmt.Errorf("maxMessageSize must be at least 1")
	}
	if c.SendChannelSize < 1 {
		return fmt.Errorf("sendChannelSize must be at least 1")
	}
	if c.RecvChannelSize < 1 {
		return fmt.Errorf("recvChannelSize must be at least 1")
	}
	return nil
}

// Validate validates the CacheConfig
func (c *CacheConfig) Validate() error {
	if !c.Enabled {
		return nil // No validation needed if cache is disabled
	}
	validProviders := map[string]bool{
		"redis":      true,
		"memcached":  true,
		"inmemory":   true,
	}
	if !validProviders[c.Provider] {
		return fmt.Errorf("provider must be one of: redis, memcached, inmemory")
	}
	if c.Provider == "redis" {
		if c.Redis.Host == "" {
			return fmt.Errorf("redis.host is required when provider is redis")
		}
		if c.Redis.Port < 1 || c.Redis.Port > 65535 {
			return fmt.Errorf("redis.port must be between 1 and 65535, got %d", c.Redis.Port)
		}
	}
	return nil
}

// Validate validates the LoggingConfig
func (c *LoggingConfig) Validate() error {
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLevels[c.Level] {
		return fmt.Errorf("level must be one of: debug, info, warn, error")
	}
	validFormats := map[string]bool{
		"json": true,
		"text": true,
	}
	if !validFormats[c.Format] {
		return fmt.Errorf("format must be one of: json, text")
	}
	if c.OutputPath == "" {
		return fmt.Errorf("outputPath is required")
	}
	return nil
}

// Validate validates the MetricsConfig
func (c *MetricsConfig) Validate() error {
	if !c.Enabled {
		return nil // No validation needed if metrics is disabled
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", c.Port)
	}
	if c.Path == "" {
		return fmt.Errorf("path is required when metrics is enabled")
	}
	if c.Path[0] != '/' {
		return fmt.Errorf("path must start with /")
	}
	return nil
}

// Validate validates the GovernanceConfig
func (c *GovernanceConfig) Validate() error {
	if !c.Enabled {
		return nil // No validation needed if governance is disabled
	}
	if c.URL == "" {
		return fmt.Errorf("url is required when governance is enabled")
	}
	return nil
}
