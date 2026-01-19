package logger

import (
	"fmt"
	"os"
	"time"

	"github.com/hsdfat/go-eir/internal/config"
	"github.com/hsdfat/go-zlog/logger"
	"github.com/hsdfat/go-zlog/sink"
	"go.uber.org/zap"
)

// Log is the global logger instance for the go-eir project
var Log logger.LoggerI = logger.NewLogger()

func init() {
	Log.(*logger.Logger).SugaredLogger = Log.(*logger.Logger).SugaredLogger.WithOptions(zap.AddCallerSkip(1))
}

// InitializeWithConfig initializes the logger with centralized logging support
func InitializeWithConfig(cfg *config.Config) error {
	var sinks []sink.Sink

	// Setup centralized logging if enabled
	if cfg.Logging.Centralized.Enabled {
		var remoteSink sink.Sink
		var err error

		// Create the appropriate sink based on backend type
		switch cfg.Logging.Centralized.Backend {
		case "loki":
			remoteSink, err = createLokiSink(cfg)
		case "http":
			remoteSink, err = createHTTPSink(cfg)
		default:
			return fmt.Errorf("unsupported centralized logging backend: %s", cfg.Logging.Centralized.Backend)
		}

		if err != nil {
			return fmt.Errorf("failed to create centralized logging sink: %w", err)
		}

		// Wrap with buffering
		bufferedSink := sink.NewBufferedSink(remoteSink, createSinkConfig(cfg))
		sinks = append(sinks, bufferedSink)
	}

	// Create new logger with remote sinks
	loggerConfig := &logger.LoggerConfig{
		EnableConsole: true,
		RemoteSinks:   sinks,
	}

	newLogger := logger.NewLoggerWithConfig(loggerConfig)
	newLogger.SugaredLogger = newLogger.SugaredLogger.WithOptions(zap.AddCallerSkip(1))
	Log = newLogger

	// Set log level
	SetLevel(cfg.Logging.Level)

	return nil
}

// createLokiSink creates a Loki sink from configuration
func createLokiSink(cfg *config.Config) (sink.Sink, error) {
	hostname, _ := os.Hostname()

	labels := map[string]string{
		"service":     "eir",
		"environment": "production",
		"hostname":    hostname,
	}

	// Add custom labels
	for k, v := range cfg.Logging.Centralized.Labels {
		labels[k] = v
	}

	lokiConfig := &sink.LokiSinkConfig{
		Config:      createSinkConfig(cfg),
		URL:         cfg.Logging.Centralized.LokiURL,
		TenantID:    cfg.Logging.Centralized.TenantID,
		Labels:      labels,
		BearerToken: cfg.Logging.Centralized.BearerToken,
	}

	return sink.NewLokiSink(lokiConfig)
}

// createHTTPSink creates a generic HTTP sink from configuration
func createHTTPSink(cfg *config.Config) (sink.Sink, error) {
	httpConfig := &sink.HTTPSinkConfig{
		Config:      createSinkConfig(cfg),
		URL:         cfg.Logging.Centralized.HTTPURL,
		Method:      "POST",
		ContentType: "application/json",
		BearerToken: cfg.Logging.Centralized.BearerToken,
	}

	return sink.NewHTTPSink(httpConfig)
}

// createSinkConfig creates a sink.Config from the EIR configuration
func createSinkConfig(cfg *config.Config) *sink.Config {
	hostname, _ := os.Hostname()

	sinkCfg := sink.DefaultConfig()
	sinkCfg.ServiceName = "eir"
	sinkCfg.Environment = "production"
	sinkCfg.InstanceID = hostname

	// Apply custom buffer settings if provided
	if cfg.Logging.Centralized.BufferSize > 0 {
		sinkCfg.BufferSize = cfg.Logging.Centralized.BufferSize
	}
	if cfg.Logging.Centralized.FlushInterval > 0 {
		sinkCfg.FlushInterval = time.Duration(cfg.Logging.Centralized.FlushInterval) * time.Second
	}
	if cfg.Logging.Centralized.MaxBatchSize > 0 {
		sinkCfg.MaxBatchSize = cfg.Logging.Centralized.MaxBatchSize
	}

	return sinkCfg
}

// SetLevel sets the global log level
// Valid levels: "debug", "info", "warn", "error", "fatal"
func SetLevel(level string) {
	logger.SetLevel(level)
}

// WithFields creates a new logger with contextual fields
// Example: logger.WithFields("conn_id", "abc123", "state", "OPEN")
func WithFields(args ...any) logger.LoggerI {
	return Log.With(args...).(logger.LoggerI)
}

// Logger is an alias for the underlying logger interface
type Logger = logger.LoggerI

// New creates a new logger with a name and level
func New(name, level string) Logger {
	if level != "" {
		// Set level if provided
		logger.SetLevel(level)
	}
	return Log.With("component", name).(logger.LoggerI)
}
