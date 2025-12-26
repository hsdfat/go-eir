# Governance Client for EIR Service

This package provides a global governance client for the EIR service to register with the governance manager and receive notifications about other services.

## Features

- **Global singleton client** - Single instance accessible throughout the application
- **Automatic registration** - Registers with governance manager on initialization
- **Pod discovery** - Get real-time information about own pods and subscribed services
- **Notification handling** - Receive updates when services register/unregister
- **Thread-safe** - Safe for concurrent access
- **Graceful shutdown** - Clean unregistration and resource cleanup

## Installation

The governance client is already included in the EIR service codebase at `pkg/governance`.

## Quick Start

### 1. Initialize during application startup

```go
import "github.com/hsdfat/telco/eir/pkg/governance"

func main() {
    // Initialize governance client
    err := governance.Initialize(&governance.Config{
        ManagerURL:       "http://governance-manager:8080",
        ServiceName:      "eir-service",
        PodName:          os.Getenv("POD_NAME"),
        NotificationPort: 9001,
        PodIP:            os.Getenv("POD_IP"),
        ServicePort:      8080,
        Subscriptions:    []string{"diam-gw", "hss"},
    })
    if err != nil {
        log.Fatalf("Failed to initialize governance: %v", err)
    }

    // ... rest of your app ...
}
```

### 2. Access the client anywhere in your code

```go
// Get the global client
govClient := governance.MustGetClient()

// Get own pods
ownPods := govClient.GetOwnPods()
log.Printf("EIR has %d pods", len(ownPods))

// Get subscribed service pods
if diamPods, exists := govClient.GetSubscribedServicePods("diam-gw"); exists {
    for _, pod := range diamPods {
        log.Printf("DIAM-GW Pod: %s [%s]", pod.PodName, pod.Status)
    }
}
```

### 3. Shutdown gracefully

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

if err := governance.Shutdown(ctx); err != nil {
    log.Printf("Error during shutdown: %v", err)
}
```

## API Reference

### Initialization

#### `Initialize(cfg *Config) error`

Initializes the global governance client. Should be called once during application startup.

**Config fields:**
- `ManagerURL` - URL of the governance manager (e.g., "http://governance-manager:8080")
- `ServiceName` - Name of this service (e.g., "eir-service")
- `PodName` - Unique pod identifier (usually from k8s downward API)
- `NotificationPort` - Port for receiving notifications and health checks
- `PodIP` - IP address of this pod
- `ServicePort` - Main service port
- `Subscriptions` - List of services to subscribe to
- `Timeout` - Timeout for API calls (default: 10s)

### Client Methods

#### `GetClient() *Client`

Returns the global client instance. Returns nil if not initialized.

#### `MustGetClient() *Client`

Returns the global client instance. Panics if not initialized.

#### `GetOwnPods() []models.PodInfo`

Returns the list of pods for this service.

#### `GetSubscribedServicePods(serviceName string) ([]models.PodInfo, bool)`

Returns the pods for a specific subscribed service.

**Returns:**
- `[]models.PodInfo` - List of pods
- `bool` - true if the service exists in subscriptions

#### `GetAllSubscribedServices() map[string][]models.PodInfo`

Returns all subscribed services and their pods.

#### `SetNotificationHandler(handler NotificationHandler)`

Sets a custom notification handler. Replaces the default handler.

```go
govClient.SetNotificationHandler(func(payload *models.NotificationPayload) {
    log.Printf("Service %s changed: %s", payload.ServiceName, payload.EventType)
    // Your custom logic
})
```

#### `Shutdown(ctx context.Context) error`

Gracefully shuts down the client, unregisters from governance, and stops the notification server.

### Helper Methods

#### `IsRegistered() bool`

Returns whether the service is currently registered.

#### `GetServiceName() string`

Returns the service name.

#### `GetPodName() string`

Returns the pod name.

#### `GetSubscriptions() []string`

Returns the list of subscribed services.

## Data Structures

### PodInfo

```go
type PodInfo struct {
    PodName   string         // Pod identifier
    Status    ServiceStatus  // healthy, unhealthy, unknown
    Providers []ProviderInfo // List of service endpoints
}
```

### ProviderInfo

```go
type ProviderInfo struct {
    Protocol Protocol // http, grpc, diameter
    IP       string   // IP address
    Port     int      // Port number
}
```

## Environment Variables

Recommended environment variables for Kubernetes:

```yaml
env:
- name: POD_NAME
  valueFrom:
    fieldRef:
      fieldPath: metadata.name
- name: POD_IP
  valueFrom:
    fieldRef:
      fieldPath: status.podIP
- name: GOVERNANCE_MANAGER_URL
  value: "http://governance-manager:8080"
```

## Example Integration

See [example.go](./example.go) for a complete example of integrating the governance client into the EIR service.

## Thread Safety

All methods are thread-safe and can be called concurrently from multiple goroutines.

## Error Handling

The client handles transient errors gracefully:
- Network failures during registration are returned as errors
- Notification delivery failures are logged but don't crash the application
- The client maintains pod info even if notifications are temporarily unavailable
