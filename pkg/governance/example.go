package governance

/*
Example usage of the governance client in EIR service:

In your main.go or initialization code:

```go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hsdfat/telco/eir/pkg/governance"
	govmodels "github.com/chronnie/governance/models"
)

func main() {
	// Initialize governance client
	err := governance.Initialize(&governance.Config{
		ManagerURL:       "http://governance-manager:8080", // or from env: os.Getenv("GOVERNANCE_MANAGER_URL")
		ServiceName:      "eir-service",
		PodName:          os.Getenv("POD_NAME"), // from k8s downward API
		NotificationPort: 9001,                  // port for receiving notifications
		PodIP:            "127.0.0.1",           // or pod IP in k8s
		ServicePort:      8080,                  // your main service port
		Subscriptions:    []string{"diam-gw", "hss"}, // services to subscribe to
		Timeout:          10 * time.Second,
	})
	if err != nil {
		log.Fatalf("Failed to initialize governance: %v", err)
	}

	// Get the governance client
	govClient := governance.MustGetClient()

	// Example 1: Get own pods
	ownPods := govClient.GetOwnPods()
	log.Printf("EIR service has %d pods", len(ownPods))
	for _, pod := range ownPods {
		log.Printf("  - Pod: %s, Status: %s", pod.PodName, pod.Status)
	}

	// Example 2: Get pods of a subscribed service (e.g., diam-gw)
	if diamPods, exists := govClient.GetSubscribedServicePods("diam-gw"); exists {
		log.Printf("DIAM-GW service has %d pods", len(diamPods))
		for _, pod := range diamPods {
			log.Printf("  - Pod: %s, Status: %s", pod.PodName, pod.Status)
			for _, provider := range pod.Providers {
				log.Printf("    Provider: %s://%s:%d", provider.Protocol, provider.IP, provider.Port)
			}
		}
	}

	// Example 3: Get all subscribed services
	allSubscribed := govClient.GetAllSubscribedServices()
	log.Printf("Subscribed to %d services", len(allSubscribed))
	for serviceName, pods := range allSubscribed {
		log.Printf("  - %s: %d pods", serviceName, len(pods))
	}

	// Example 4: Custom notification handler
	govClient.SetNotificationHandler(func(payload *govmodels.NotificationPayload) {
		log.Printf("Custom handler: service=%s, event=%s, pods=%d",
			payload.ServiceName, payload.EventType, len(payload.Pods))

		// Your custom logic here
		if payload.ServiceName == "diam-gw" {
			// Update your connection pool, load balancer, etc.
			updateDiamGwConnections(payload.Pods)
		}
	})

	// ... rest of your application code ...

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	// Graceful shutdown
	log.Println("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := governance.Shutdown(ctx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}
}

func updateDiamGwConnections(pods []govmodels.PodInfo) {
	// Example: Update your connection pool with new DIAM-GW endpoints
	for _, pod := range pods {
		if pod.Status == govmodels.StatusHealthy {
			for _, provider := range pod.Providers {
				// Add to connection pool
				log.Printf("Adding DIAM-GW endpoint: %s:%d", provider.IP, provider.Port)
			}
		}
	}
}
```

In Kubernetes deployment:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: eir-service-pod-1
spec:
  containers:
  - name: eir
    image: eir-service:latest
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
    ports:
    - containerPort: 8080
      name: http
    - containerPort: 9001
      name: governance
```
*/
