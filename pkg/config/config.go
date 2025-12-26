package config

import (
	"context"
	"fmt"
	"time"

	sharedconfig "github.com/hsdfat/telco/config"
)

// Config represents the complete EIR service configuration
type Config struct {
	Server     ServerConfig     `json:"server" yaml:"server"`
	Database   DatabaseConfig   `json:"database" yaml:"database"`
	Diameter   DiameterConfig   `json:"diameter" yaml:"diameter"`
	Cache      CacheConfig      `json:"cache" yaml:"cache"`
	Logging    LoggingConfig    `json:"logging" yaml:"logging"`
	Metrics    MetricsConfig    `json:"metrics" yaml:"metrics"`
	Governance GovernanceConfig `json:"governance" yaml:"governance"`
}

// ServerConfig configures the HTTP/2 server
type ServerConfig struct {
	Host         string        `json:"host" yaml:"host" env:"HOST" envDefault:"0.0.0.0"`
	Port         int           `json:"port" yaml:"port" env:"PORT" envDefault:"8080" validate:"min=1,max=65535"`
	ReadTimeout  time.Duration `json:"read_timeout" yaml:"read_timeout" env:"READ_TIMEOUT" envDefault:"30s"`
	WriteTimeout time.Duration `json:"write_timeout" yaml:"write_timeout" env:"WRITE_TIMEOUT" envDefault:"30s"`
	IdleTimeout  time.Duration `json:"idle_timeout" yaml:"idle_timeout" env:"IDLE_TIMEOUT" envDefault:"120s"`
}

// DatabaseConfig configures database connection
type DatabaseConfig struct {
	Type     string `json:"type" yaml:"type" env:"TYPE" envDefault:"postgres" validate:"required,oneof=postgres mongodb"`
	Host     string `json:"host" yaml:"host" env:"HOST" envDefault:"localhost" validate:"required"`
	Port     int    `json:"port" yaml:"port" env:"PORT" envDefault:"5432" validate:"min=1,max=65535"`
	Database string `json:"database" yaml:"database" env:"DATABASE" validate:"required"`
	Username string `json:"username" yaml:"username" env:"USERNAME" validate:"required"`
	Password string `json:"password" yaml:"password" env:"PASSWORD" validate:"required"`
	SSLMode  string `json:"ssl_mode" yaml:"ssl_mode" env:"SSL_MODE" envDefault:"disable"`

	// Connection pool settings
	MaxOpenConns    int           `json:"max_open_conns" yaml:"max_open_conns" env:"MAX_OPEN_CONNS" envDefault:"25"`
	MaxIdleConns    int           `json:"max_idle_conns" yaml:"max_idle_conns" env:"MAX_IDLE_CONNS" envDefault:"5"`
	ConnMaxLifetime time.Duration `json:"conn_max_lifetime" yaml:"conn_max_lifetime" env:"CONN_MAX_LIFETIME" envDefault:"5m"`
}

// DiameterConfig configures the Diameter S13 interface
type DiameterConfig struct {
	Enabled     bool   `json:"enabled" yaml:"enabled" env:"ENABLED" envDefault:"true"`
	ListenAddr  string `json:"listen_addr" yaml:"listen_addr" env:"LISTEN_ADDR" envDefault:"0.0.0.0:3868" validate:"required"`
	OriginHost  string `json:"origin_host" yaml:"origin_host" env:"ORIGIN_HOST" validate:"required"`
	OriginRealm string `json:"origin_realm" yaml:"origin_realm" env:"ORIGIN_REALM" validate:"required"`

	// Diameter connection settings
	WatchdogInterval time.Duration `json:"watchdog_interval" yaml:"watchdog_interval" env:"WATCHDOG_INTERVAL" envDefault:"30s"`
	IdleTimeout      time.Duration `json:"idle_timeout" yaml:"idle_timeout" env:"IDLE_TIMEOUT" envDefault:"300s"`
}

// CacheConfig configures caching layer
type CacheConfig struct {
	Provider string        `json:"provider" yaml:"provider" env:"PROVIDER" envDefault:"memory" validate:"oneof=memory redis memcached"`
	TTL      time.Duration `json:"ttl" yaml:"ttl" env:"TTL" envDefault:"5m"`

	// Redis-specific settings
	RedisAddr     string `json:"redis_addr" yaml:"redis_addr" env:"REDIS_ADDR"`
	RedisPassword string `json:"redis_password" yaml:"redis_password" env:"REDIS_PASSWORD"`
	RedisDB       int    `json:"redis_db" yaml:"redis_db" env:"REDIS_DB" envDefault:"0"`

	// Memcached-specific settings
	MemcachedServers []string `json:"memcached_servers" yaml:"memcached_servers" env:"MEMCACHED_SERVERS"`
}

// LoggingConfig configures logging
type LoggingConfig struct {
	Level  string `json:"level" yaml:"level" env:"LEVEL" envDefault:"info" validate:"oneof=debug info warn error"`
	Format string `json:"format" yaml:"format" env:"FORMAT" envDefault:"json" validate:"oneof=json text"`
}

// MetricsConfig configures metrics exposition
type MetricsConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled" env:"ENABLED" envDefault:"true"`
	Port    int    `json:"port" yaml:"port" env:"PORT" envDefault:"9090" validate:"min=1,max=65535"`
	Path    string `json:"path" yaml:"path" env:"PATH" envDefault:"/metrics"`
}

// GovernanceConfig configures governance client
type GovernanceConfig struct {
	Enabled          bool          `json:"enabled" yaml:"enabled" env:"ENABLED" envDefault:"true"`
	ManagerURL       string        `json:"manager_url" yaml:"manager_url" env:"MANAGER_URL" envDefault:"http://governance-manager:8080" validate:"required_if=Enabled true"`
	ServiceName      string        `json:"service_name" yaml:"service_name" env:"SERVICE_NAME" envDefault:"eir-service" validate:"required_if=Enabled true"`
	PodName          string        `json:"pod_name" yaml:"pod_name" env:"POD_NAME"`
	NotificationPort int           `json:"notification_port" yaml:"notification_port" env:"NOTIFICATION_PORT" envDefault:"9001" validate:"min=1,max=65535"`
	PodIP            string        `json:"pod_ip" yaml:"pod_ip" env:"POD_IP" envDefault:"127.0.0.1" validate:"required_if=Enabled true"`
	Subscriptions    []string      `json:"subscriptions" yaml:"subscriptions" env:"SUBSCRIPTIONS" envDefault:"diam-gw,hss"`
	Timeout          time.Duration `json:"timeout" yaml:"timeout" env:"TIMEOUT" envDefault:"10s"`
}

// Validate validates the governance configuration
func (g *GovernanceConfig) Validate() error {
	if !g.Enabled {
		return nil
	}

	if g.ManagerURL == "" {
		return fmt.Errorf("governance.manager_url is required when governance is enabled")
	}

	if g.ServiceName == "" {
		return fmt.Errorf("governance.service_name is required when governance is enabled")
	}

	if g.NotificationPort < 1 || g.NotificationPort > 65535 {
		return fmt.Errorf("governance.notification_port must be between 1 and 65535, got %d", g.NotificationPort)
	}

	if g.PodIP == "" {
		return fmt.Errorf("governance.pod_ip is required when governance is enabled")
	}

	if g.Timeout <= 0 {
		return fmt.Errorf("governance.timeout must be positive, got %v", g.Timeout)
	}

	return nil
}

// LoaderConfig configures how configuration is loaded
type LoaderConfig struct {
	// ConfigFile is the path to the YAML configuration file
	ConfigFile string

	// ConfigFileSearchPaths are directories to search for config file
	ConfigFileSearchPaths []string

	// EnvPrefix is the prefix for environment variables
	EnvPrefix string

	// RemoteConfig configures remote config server
	RemoteConfig *RemoteConfig

	// EnableHotReload enables automatic config reloading
	EnableHotReload bool

	// ReloadCallback is called when config is reloaded
	ReloadCallback func(*Config) error
}

// RemoteConfig configures remote configuration source
type RemoteConfig struct {
	Provider  string     `json:"provider" yaml:"provider" validate:"oneof=consul etcd"`
	Endpoints []string   `json:"endpoints" yaml:"endpoints" validate:"required"`
	Key       string     `json:"key" yaml:"key" validate:"required"`
	TLS       *TLSConfig `json:"tls" yaml:"tls"`
}

// TLSConfig holds TLS configuration
type TLSConfig struct {
	CertFile string `json:"cert_file" yaml:"cert_file"`
	KeyFile  string `json:"key_file" yaml:"key_file"`
	CAFile   string `json:"ca_file" yaml:"ca_file"`
}

// Loader loads and manages EIR configuration
type Loader struct {
	manager *sharedconfig.Manager
	config  *Config
}

// NewLoader creates a configuration loader
func NewLoader(cfg LoaderConfig) (*Loader, error) {
	var providers []sharedconfig.Provider
	var watcher sharedconfig.Watcher

	// Add file provider if configured
	if cfg.ConfigFile != "" {
		fileProvider, err := sharedconfig.NewFileProvider(sharedconfig.FileProviderConfig{
			Path:        cfg.ConfigFile,
			SearchPaths: cfg.ConfigFileSearchPaths,
			Required:    false, // Optional - can run with env vars only
		})
		if err == nil {
			providers = append(providers, fileProvider)

			// Setup file watcher for hot reload
			if cfg.EnableHotReload {
				fw, err := sharedconfig.NewFileWatcher(
					[]string{cfg.ConfigFile},
					100*time.Millisecond,
				)
				if err == nil {
					watcher = fw
				}
			}
		}
	}

	// Add remote provider if configured
	if cfg.RemoteConfig != nil {
		var remoteProvider sharedconfig.Provider
		var err error

		remoteCfg := sharedconfig.RemoteProviderConfig{
			Type:        sharedconfig.RemoteProviderType(cfg.RemoteConfig.Provider),
			Endpoints:   cfg.RemoteConfig.Endpoints,
			Key:         cfg.RemoteConfig.Key,
			Timeout:     10 * time.Second,
			RetryConfig: sharedconfig.DefaultRetryConfig(),
		}

		if cfg.RemoteConfig.TLS != nil {
			remoteCfg.TLSConfig = &sharedconfig.TLSConfig{
				CertFile: cfg.RemoteConfig.TLS.CertFile,
				KeyFile:  cfg.RemoteConfig.TLS.KeyFile,
				CAFile:   cfg.RemoteConfig.TLS.CAFile,
			}
		}

		switch cfg.RemoteConfig.Provider {
		case "consul":
			remoteProvider, err = sharedconfig.NewConsulProvider(remoteCfg)
		case "etcd":
			remoteProvider, err = sharedconfig.NewEtcdProvider(remoteCfg)
		default:
			return nil, fmt.Errorf("unsupported remote provider: %s", cfg.RemoteConfig.Provider)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to create remote provider: %w", err)
		}

		providers = append(providers, remoteProvider)

		// Setup remote watcher for hot reload
		if cfg.EnableHotReload && cfg.RemoteConfig.Provider == "consul" {
			_ = remoteProvider.(*sharedconfig.ConsulProvider)
			// Extract client from provider (need to modify ConsulProvider to expose it)
			// For now, we'll skip remote watching in this example
		}
	}

	// Add environment variable provider (highest priority)
	if cfg.EnvPrefix != "" {
		envProvider := sharedconfig.NewEnvProvider(sharedconfig.EnvProviderConfig{
			Prefix:    cfg.EnvPrefix,
			Separator: "_",
		})
		providers = append(providers, envProvider)
	}

	// Create validator
	configInstance := &Config{}
	validator := sharedconfig.NewStructValidator(configInstance)

	// Create manager
	manager := sharedconfig.NewManager(sharedconfig.ManagerConfig{
		Providers:       providers,
		Validator:       validator,
		Watcher:         watcher,
		EnableHotReload: cfg.EnableHotReload,
		ReloadCallback: func(data map[string]interface{}) error {
			if cfg.ReloadCallback != nil {
				return cfg.ReloadCallback(configInstance)
			}
			return nil
		},
	})

	return &Loader{
		manager: manager,
		config:  configInstance,
	}, nil
}

// Load loads the configuration
func (l *Loader) Load(ctx context.Context) (*Config, error) {
	data, err := l.manager.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Unmarshal into Config struct
	if err := sharedconfig.UnmarshalEnv(data, l.config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return l.config, nil
}

// Watch starts watching for configuration changes
func (l *Loader) Watch(ctx context.Context, callback func(*Config) error) error {
	return l.manager.Watch(ctx, func(data map[string]interface{}) error {
		// Unmarshal into new config instance
		newConfig := &Config{}
		if err := sharedconfig.UnmarshalEnv(data, newConfig); err != nil {
			return err
		}

		l.config = newConfig
		if callback != nil {
			return callback(newConfig)
		}
		return nil
	})
}

// Close closes the configuration loader
func (l *Loader) Close() error {
	return l.manager.Close()
}

// DefaultConfig returns configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:         "0.0.0.0",
			Port:         8080,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
		},
		Database: DatabaseConfig{
			Type:            "postgres",
			Host:            "localhost",
			Port:            5432,
			SSLMode:         "disable",
			MaxOpenConns:    25,
			MaxIdleConns:    5,
			ConnMaxLifetime: 5 * time.Minute,
		},
		Diameter: DiameterConfig{
			Enabled:          true,
			ListenAddr:       "0.0.0.0:3868",
			WatchdogInterval: 30 * time.Second,
			IdleTimeout:      300 * time.Second,
		},
		Cache: CacheConfig{
			Provider: "memory",
			TTL:      5 * time.Minute,
			RedisDB:  0,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		Metrics: MetricsConfig{
			Enabled: true,
			Port:    9090,
			Path:    "/metrics",
		},
	}
}
