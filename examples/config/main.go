package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	eirconfig "github.com/hsdfat8/eir/pkg/config"
)

func main() {
	// Example 1: File-based configuration
	fileBasedExample()

	// Example 2: Remote configuration with Consul
	// consulExample()

	// Example 3: Confd-managed configuration
	// confdExample()

	// Example 4: Hot reload example
	// hotReloadExample()
}

// fileBasedExample demonstrates basic file-based configuration
func fileBasedExample() {
	fmt.Println("=== File-Based Configuration Example ===")

	loader, err := eirconfig.NewLoader(eirconfig.LoaderConfig{
		ConfigFile:            "config.yaml",
		ConfigFileSearchPaths: []string{".", "./config", "/etc/eir"},
		EnvPrefix:             "EIR_",
		EnableHotReload:       false,
	})
	if err != nil {
		log.Fatalf("Failed to create loader: %v", err)
	}
	defer loader.Close()

	cfg, err := loader.Load(context.Background())
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("Server: %s:%d\n", cfg.Server.Host, cfg.Server.Port)
	fmt.Printf("Database: %s@%s:%d/%s\n",
		cfg.Database.Username,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Database)
	fmt.Printf("Diameter: %s (enabled: %v)\n",
		cfg.Diameter.ListenAddr,
		cfg.Diameter.Enabled)
	fmt.Printf("Cache: %s (TTL: %v)\n",
		cfg.Cache.Provider,
		cfg.Cache.TTL)
	fmt.Printf("Logging: %s/%s\n",
		cfg.Logging.Level,
		cfg.Logging.Format)
	fmt.Println()
}

// consulExample demonstrates Consul-based remote configuration
func consulExample() {
	fmt.Println("=== Consul Remote Configuration Example ===")

	loader, err := eirconfig.NewLoader(eirconfig.LoaderConfig{
		EnvPrefix: "EIR_",
		RemoteConfig: &eirconfig.RemoteConfig{
			Provider:  "consul",
			Endpoints: []string{"localhost:8500"},
			Key:       "config/eir/production",
		},
		EnableHotReload: true,
		ReloadCallback: func(cfg *eirconfig.Config) error {
			log.Printf("Configuration reloaded from Consul!")
			log.Printf("New server port: %d", cfg.Server.Port)
			return nil
		},
	})
	if err != nil {
		log.Fatalf("Failed to create loader: %v", err)
	}
	defer loader.Close()

	cfg, err := loader.Load(context.Background())
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("Loaded from Consul: %s:%d\n", cfg.Server.Host, cfg.Server.Port)
	fmt.Println()
}

// confdExample demonstrates confd-managed configuration
func confdExample() {
	fmt.Println("=== Confd-Managed Configuration Example ===")

	loader, err := eirconfig.NewLoader(eirconfig.LoaderConfig{
		EnvPrefix: "EIR_",
		RemoteConfig: &eirconfig.RemoteConfig{
			Provider:  "confd",
			Endpoints: []string{"localhost:8500"}, // Confd backend (consul/etcd)
			Key:       "config/eir/production",
		},
		EnableHotReload: true,
	})
	if err != nil {
		log.Fatalf("Failed to create loader: %v", err)
	}
	defer loader.Close()

	cfg, err := loader.Load(context.Background())
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("Loaded via confd: %s:%d\n", cfg.Server.Host, cfg.Server.Port)
	fmt.Println()
}

// hotReloadExample demonstrates hot reload with file watching
func hotReloadExample() {
	fmt.Println("=== Hot Reload Example ===")

	loader, err := eirconfig.NewLoader(eirconfig.LoaderConfig{
		ConfigFile:      "config.yaml",
		EnvPrefix:       "EIR_",
		EnableHotReload: true,
		ReloadCallback: func(cfg *eirconfig.Config) error {
			log.Println("🔄 Configuration reloaded!")
			log.Printf("  Server: %s:%d", cfg.Server.Host, cfg.Server.Port)
			log.Printf("  Log level: %s", cfg.Logging.Level)
			return nil
		},
	})
	if err != nil {
		log.Fatalf("Failed to create loader: %v", err)
	}
	defer loader.Close()

	ctx := context.Background()

	// Load initial config
	cfg, err := loader.Load(ctx)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("Initial config loaded: %s:%d\n", cfg.Server.Host, cfg.Server.Port)
	fmt.Println("Watching for configuration changes...")
	fmt.Println("Try editing config.yaml to see hot reload in action")
	fmt.Println("Press Ctrl+C to exit")

	// Start watching in background
	go func() {
		err := loader.Watch(ctx, func(newCfg *eirconfig.Config) error {
			// This callback is invoked when config changes
			log.Printf("Applying new configuration...")
			// Here you would update your running service
			return nil
		})
		if err != nil {
			log.Printf("Watch error: %v", err)
		}
	}()

	// Wait for interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	fmt.Println("\nShutting down...")
}

// layeredConfigExample demonstrates layered configuration
func layeredConfigExample() {
	fmt.Println("=== Layered Configuration Example ===")
	fmt.Println("Priority: Env Vars > Consul > Config File")

	// Set some environment overrides
	os.Setenv("EIR_SERVER_PORT", "9090")
	os.Setenv("EIR_LOGGING_LEVEL", "debug")
	defer func() {
		os.Unsetenv("EIR_SERVER_PORT")
		os.Unsetenv("EIR_LOGGING_LEVEL")
	}()

	loader, err := eirconfig.NewLoader(eirconfig.LoaderConfig{
		// Layer 1: Base config file
		ConfigFile: "config.yaml",

		// Layer 2: Remote config (commented out - would override file)
		// RemoteConfig: &eirconfig.RemoteConfig{
		//     Provider:  "consul",
		//     Endpoints: []string{"localhost:8500"},
		//     Key:       "config/eir/production",
		// },

		// Layer 3: Environment variables (highest priority)
		EnvPrefix: "EIR_",
	})
	if err != nil {
		log.Fatalf("Failed to create loader: %v", err)
	}
	defer loader.Close()

	cfg, err := loader.Load(context.Background())
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("Server Port: %d (from env var EIR_SERVER_PORT)\n", cfg.Server.Port)
	fmt.Printf("Log Level: %s (from env var EIR_LOGGING_LEVEL)\n", cfg.Logging.Level)
	fmt.Printf("Database Host: %s (from config file)\n", cfg.Database.Host)
	fmt.Println()
}
