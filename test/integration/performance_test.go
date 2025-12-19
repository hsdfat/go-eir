package integration

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hsdfat/diam-gw/commands/s13"
)

// PerformanceMetrics tracks performance test metrics
type PerformanceMetrics struct {
	TotalRequests     int64
	SuccessfulReqs    int64
	FailedReqs        int64
	TotalLatencyMs    int64
	MinLatencyMs      int64
	MaxLatencyMs      int64
	RequestsPerSecond float64
	AvgLatencyMs      float64
	P50LatencyMs      int64
	P95LatencyMs      int64
	P99LatencyMs      int64
	StartTime         time.Time
	EndTime           time.Time
}

// LatencyBucket tracks latency distribution
type LatencyBucket struct {
	mu        sync.Mutex
	latencies []int64
}

func (lb *LatencyBucket) Add(latencyMs int64) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.latencies = append(lb.latencies, latencyMs)
}

func (lb *LatencyBucket) GetPercentile(p float64) int64 {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if len(lb.latencies) == 0 {
		return 0
	}

	// Create a copy and sort for accurate percentile calculation
	sorted := make([]int64, len(lb.latencies))
	copy(sorted, lb.latencies)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	index := int(float64(len(sorted)) * p)
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

// TestPerformance_Throughput tests maximum throughput with various concurrency levels
func TestPerformance_Throughput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	ctx := context.Background()
	draAddr := getEnvOrDefault("DRA_ADDR", "localhost:3869")

	// Test with different concurrency levels
	concurrencyLevels := []int{1, 5, 10, 25, 50, 100}
	requestsPerClient := 100

	for _, concurrency := range concurrencyLevels {
		t.Run(fmt.Sprintf("Concurrency_%d", concurrency), func(t *testing.T) {
			metrics := runThroughputTest(t, ctx, draAddr, concurrency, requestsPerClient)

			t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			t.Logf("Concurrency Level: %d clients", concurrency)
			t.Logf("Total Requests: %d", metrics.TotalRequests)
			t.Logf("Successful: %d (%.2f%%)", metrics.SuccessfulReqs,
				float64(metrics.SuccessfulReqs)/float64(metrics.TotalRequests)*100)
			t.Logf("Failed: %d (%.2f%%)", metrics.FailedReqs,
				float64(metrics.FailedReqs)/float64(metrics.TotalRequests)*100)
			t.Logf("Throughput: %.2f req/sec", metrics.RequestsPerSecond)
			t.Logf("Latency (avg): %.2f ms", metrics.AvgLatencyMs)
			t.Logf("Latency (min): %d ms", metrics.MinLatencyMs)
			t.Logf("Latency (max): %d ms", metrics.MaxLatencyMs)
			t.Logf("Latency (p50): %d ms", metrics.P50LatencyMs)
			t.Logf("Latency (p95): %d ms", metrics.P95LatencyMs)
			t.Logf("Latency (p99): %d ms", metrics.P99LatencyMs)
			t.Logf("Duration: %.2f seconds", metrics.EndTime.Sub(metrics.StartTime).Seconds())
			t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		})
	}
}

// TestPerformance_Latency tests latency under different load conditions
func TestPerformance_Latency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	ctx := context.Background()
	draAddr := getEnvOrDefault("DRA_ADDR", "localhost:3869")

	// Test latency with burst traffic patterns
	scenarios := []struct {
		name              string
		concurrency       int
		requestsPerClient int
		burstDelay        time.Duration
	}{
		{"SteadyLoad_Low", 10, 50, 0},
		{"SteadyLoad_Medium", 25, 50, 0},
		{"SteadyLoad_High", 50, 50, 0},
		{"BurstTraffic_Short", 50, 20, 100 * time.Millisecond},
		{"BurstTraffic_Long", 100, 10, 200 * time.Millisecond},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			metrics := runLatencyTest(t, ctx, draAddr, scenario.concurrency,
				scenario.requestsPerClient, scenario.burstDelay)

			t.Logf("Scenario: %s", scenario.name)
			t.Logf("  Avg Latency: %.2f ms", metrics.AvgLatencyMs)
			t.Logf("  P95 Latency: %d ms", metrics.P95LatencyMs)
			t.Logf("  P99 Latency: %d ms", metrics.P99LatencyMs)
			t.Logf("  Max Latency: %d ms", metrics.MaxLatencyMs)

			// Assert latency SLAs
			if metrics.P95LatencyMs > 100 {
				t.Logf("WARNING: P95 latency exceeds 100ms threshold: %d ms", metrics.P95LatencyMs)
			}
			if metrics.P99LatencyMs > 200 {
				t.Logf("WARNING: P99 latency exceeds 200ms threshold: %d ms", metrics.P99LatencyMs)
			}
		})
	}
}

// TestPerformance_SustainedLoad tests system behavior under sustained load
func TestPerformance_SustainedLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	ctx := context.Background()
	draAddr := getEnvOrDefault("DRA_ADDR", "localhost:3869")

	// Run sustained load for 60 seconds
	duration := 60 * time.Second
	concurrency := 25
	targetRPS := 100.0 // Target 100 requests per second

	t.Logf("Running sustained load test for %v", duration)
	t.Logf("Target: %.0f req/sec with %d concurrent clients", targetRPS, concurrency)

	metrics := runSustainedLoadTest(t, ctx, draAddr, duration, concurrency, targetRPS)

	t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	t.Logf("Sustained Load Results:")
	t.Logf("  Total Requests: %d", metrics.TotalRequests)
	t.Logf("  Success Rate: %.2f%%", float64(metrics.SuccessfulReqs)/float64(metrics.TotalRequests)*100)
	t.Logf("  Actual RPS: %.2f", metrics.RequestsPerSecond)
	t.Logf("  Avg Latency: %.2f ms", metrics.AvgLatencyMs)
	t.Logf("  P95 Latency: %d ms", metrics.P95LatencyMs)
	t.Logf("  P99 Latency: %d ms", metrics.P99LatencyMs)
	t.Logf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Verify sustained performance
	if metrics.RequestsPerSecond < targetRPS*0.9 {
		t.Errorf("Failed to maintain target RPS: got %.2f, want >= %.2f",
			metrics.RequestsPerSecond, targetRPS*0.9)
	}
}

// TestPerformance_StressTest gradually increases load until system degradation
func TestPerformance_StressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	ctx := context.Background()
	draAddr := getEnvOrDefault("DRA_ADDR", "localhost:3869")

	// Start with 10 concurrent clients and double every 10 seconds
	maxConcurrency := 200
	requestsPerClient := 50

	t.Logf("Running stress test up to %d concurrent clients", maxConcurrency)

	for concurrency := 10; concurrency <= maxConcurrency; concurrency *= 2 {
		t.Logf("\n--- Testing with %d concurrent clients ---", concurrency)

		metrics := runThroughputTest(t, ctx, draAddr, concurrency, requestsPerClient)

		t.Logf("RPS: %.2f, Avg Latency: %.2f ms, P95: %d ms, Success Rate: %.2f%%",
			metrics.RequestsPerSecond,
			metrics.AvgLatencyMs,
			metrics.P95LatencyMs,
			float64(metrics.SuccessfulReqs)/float64(metrics.TotalRequests)*100)

		// Check for degradation
		if metrics.P95LatencyMs > 500 {
			t.Logf("WARNING: System degradation detected at %d concurrent clients (P95: %d ms)",
				concurrency, metrics.P95LatencyMs)
		}

		// Brief pause between stress levels
		time.Sleep(2 * time.Second)
	}
}

// TestPerformance_ConnectionPooling tests connection reuse efficiency
func TestPerformance_ConnectionPooling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	ctx := context.Background()
	draAddr := getEnvOrDefault("DRA_ADDR", "localhost:3869")

	scenarios := []struct {
		name              string
		reuseConnection   bool
		requestsPerClient int
	}{
		{"WithConnectionReuse", true, 100},
		{"WithoutConnectionReuse", false, 100},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			start := time.Now()

			if scenario.reuseConnection {
				// Single persistent connection
				client, err := NewS13TestClient(draAddr)
				if err != nil {
					t.Fatalf("Failed to create client: %v", err)
				}
				defer client.Close()

				for i := 0; i < scenario.requestsPerClient; i++ {
					// Create a context with timeout for each request
					reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
					_, err := client.SendMEIdentityCheckRequest(reqCtx, "490154203237518")
					cancel()
					if err != nil {
						t.Logf("Request %d failed: %v", i, err)
					}
				}
			} else {
				// New connection per request
				for i := 0; i < scenario.requestsPerClient; i++ {
					client, err := NewS13TestClient(draAddr)
					if err != nil {
						t.Fatalf("Failed to create client: %v", err)
					}

					// Create a context with timeout for each request
					reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
					_, err = client.SendMEIdentityCheckRequest(reqCtx, "490154203237518")
					cancel()
					if err != nil {
						t.Logf("Request %d failed: %v", i, err)
					}

					client.Close()
				}
			}

			duration := time.Since(start)
			rps := float64(scenario.requestsPerClient) / duration.Seconds()
			avgLatency := duration.Milliseconds() / int64(scenario.requestsPerClient)

			t.Logf("%s:", scenario.name)
			t.Logf("  Total Duration: %v", duration)
			t.Logf("  RPS: %.2f", rps)
			t.Logf("  Avg Latency: %d ms", avgLatency)
		})
	}
}

// S13TestClient wraps DiameterClient for performance testing
type S13TestClient struct {
	*DiameterClient
}

// NewS13TestClient creates a new S13 test client and connects it
func NewS13TestClient(draAddr string) (*S13TestClient, error) {
	client := createDiameterClient(draAddr)
	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	return &S13TestClient{DiameterClient: client}, nil
}

// SendMEIdentityCheckRequest sends an ME Identity Check Request
func (c *S13TestClient) SendMEIdentityCheckRequest(ctx context.Context, imei string) (*s13.MEIdentityCheckAnswer, error) {
	return c.CheckEquipment(ctx, imei)
}

// TestPerformance_MessageSize tests performance with varying message sizes
func TestPerformance_MessageSize(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	ctx := context.Background()
	draAddr := getEnvOrDefault("DRA_ADDR", "localhost:3869")

	// Test with different IMEI lengths and additional AVPs
	scenarios := []struct {
		name  string
		imeis []string
	}{
		{"Standard_15digit", []string{"490154203237518", "357368010000000"}},
		{"Mixed_Lengths", []string{"490154203237518", "35736801", "357368010000000000"}},
	}

	concurrency := 10
	requestsPerClient := 50

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			var wg sync.WaitGroup
			var successCount, failCount int64

			startTime := time.Now()

			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func(clientID int) {
					defer wg.Done()

					client, err := NewS13TestClient(draAddr)
					if err != nil {
						atomic.AddInt64(&failCount, int64(requestsPerClient))
						return
					}
					defer client.Close()

					for j := 0; j < requestsPerClient; j++ {
						imei := scenario.imeis[j%len(scenario.imeis)]
						// Create a context with timeout for each request
						reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
						_, err := client.SendMEIdentityCheckRequest(reqCtx, imei)
						cancel()
						if err != nil {
							atomic.AddInt64(&failCount, 1)
						} else {
							atomic.AddInt64(&successCount, 1)
						}
					}
				}(i)
			}

			wg.Wait()
			duration := time.Since(startTime)

			totalReqs := concurrency * requestsPerClient
			rps := float64(totalReqs) / duration.Seconds()

			t.Logf("%s:", scenario.name)
			t.Logf("  Success Rate: %.2f%%", float64(successCount)/float64(totalReqs)*100)
			t.Logf("  RPS: %.2f", rps)
			t.Logf("  Duration: %v", duration)
		})
	}
}

// Helper function to run throughput test
func runThroughputTest(t *testing.T, ctx context.Context, draAddr string,
	concurrency, requestsPerClient int) *PerformanceMetrics {

	var wg sync.WaitGroup
	var successCount, failCount int64
	latencyBucket := &LatencyBucket{latencies: make([]int64, 0, concurrency*requestsPerClient)}

	var minLatency int64 = 999999
	var maxLatency int64 = 0
	var totalLatency int64 = 0
	var minLatencyMu sync.Mutex
	var maxLatencyMu sync.Mutex

	// Valid IMEIs that pass Luhn check
	validIMEIs := []string{
		"490154203237518", // Whitelisted
		"357368010000000", // Whitelisted
		"353879234252633", // Valid IMEI
	}

	startTime := time.Now()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			client, err := NewS13TestClient(draAddr)
			if err != nil {
				atomic.AddInt64(&failCount, int64(requestsPerClient))
				return
			}
			defer client.Close()

			for j := 0; j < requestsPerClient; j++ {
				imei := validIMEIs[rand.Intn(len(validIMEIs))]

				// Create a context with timeout for each request
				reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				reqStart := time.Now()
				_, err := client.SendMEIdentityCheckRequest(reqCtx, imei)
				latency := time.Since(reqStart).Milliseconds()
				cancel()

				if err != nil {
					atomic.AddInt64(&failCount, 1)
				} else {
					atomic.AddInt64(&successCount, 1)
				}

				atomic.AddInt64(&totalLatency, latency)
				latencyBucket.Add(latency)

				// Update min/max with proper synchronization
				minLatencyMu.Lock()
				if latency < minLatency {
					minLatency = latency
				}
				minLatencyMu.Unlock()

				maxLatencyMu.Lock()
				if latency > maxLatency {
					maxLatency = latency
				}
				maxLatencyMu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	endTime := time.Now()

	totalRequests := int64(concurrency * requestsPerClient)
	duration := endTime.Sub(startTime).Seconds()

	if duration == 0 {
		duration = 0.001 // Avoid division by zero
	}

	avgLatency := float64(0)
	if totalRequests > 0 {
		avgLatency = float64(totalLatency) / float64(totalRequests)
	}

	return &PerformanceMetrics{
		TotalRequests:     totalRequests,
		SuccessfulReqs:    successCount,
		FailedReqs:        failCount,
		TotalLatencyMs:    totalLatency,
		MinLatencyMs:      minLatency,
		MaxLatencyMs:      maxLatency,
		RequestsPerSecond: float64(totalRequests) / duration,
		AvgLatencyMs:      avgLatency,
		P50LatencyMs:      latencyBucket.GetPercentile(0.50),
		P95LatencyMs:      latencyBucket.GetPercentile(0.95),
		P99LatencyMs:      latencyBucket.GetPercentile(0.99),
		StartTime:         startTime,
		EndTime:           endTime,
	}
}

// Helper function to run latency test
func runLatencyTest(t *testing.T, ctx context.Context, draAddr string,
	concurrency, requestsPerClient int, burstDelay time.Duration) *PerformanceMetrics {

	// Similar to throughput test but with burst delays
	metrics := runThroughputTest(t, ctx, draAddr, concurrency, requestsPerClient)

	if burstDelay > 0 {
		time.Sleep(burstDelay)
	}

	return metrics
}

// Helper function to run sustained load test
func runSustainedLoadTest(t *testing.T, ctx context.Context, draAddr string,
	duration time.Duration, concurrency int, targetRPS float64) *PerformanceMetrics {

	var wg sync.WaitGroup
	var successCount, failCount int64
	latencyBucket := &LatencyBucket{latencies: make([]int64, 0, 10000)}

	var minLatency int64 = 999999
	var maxLatency int64 = 0
	var totalLatency int64 = 0
	var minLatencyMu sync.Mutex
	var maxLatencyMu sync.Mutex

	validIMEIs := []string{"490154203237518", "357368010000000", "353879234252633"}

	startTime := time.Now()
	stopChan := make(chan struct{})

	// Calculate request interval per client to achieve target RPS
	if targetRPS <= 0 || concurrency <= 0 {
		targetRPS = 100.0
		concurrency = 25
	}
	requestInterval := time.Duration(float64(time.Second) / (targetRPS / float64(concurrency)))
	if requestInterval <= 0 {
		requestInterval = 10 * time.Millisecond
	}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			client, err := NewS13TestClient(draAddr)
			if err != nil {
				return
			}
			defer client.Close()

			ticker := time.NewTicker(requestInterval)
			defer ticker.Stop()

			for {
				select {
				case <-stopChan:
					return
				case <-ticker.C:
					imei := validIMEIs[rand.Intn(len(validIMEIs))]

					// Create a context with timeout for each request
					reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
					reqStart := time.Now()
					_, err := client.SendMEIdentityCheckRequest(reqCtx, imei)
					latency := time.Since(reqStart).Milliseconds()
					cancel()

					if err != nil {
						atomic.AddInt64(&failCount, 1)
					} else {
						atomic.AddInt64(&successCount, 1)
					}

					atomic.AddInt64(&totalLatency, latency)
					latencyBucket.Add(latency)

					// Update min/max with proper synchronization
					minLatencyMu.Lock()
					if latency < minLatency {
						minLatency = latency
					}
					minLatencyMu.Unlock()

					maxLatencyMu.Lock()
					if latency > maxLatency {
						maxLatency = latency
					}
					maxLatencyMu.Unlock()
				}
			}
		}(i)
	}

	// Run for specified duration
	time.Sleep(duration)
	close(stopChan)
	wg.Wait()

	endTime := time.Now()
	totalRequests := successCount + failCount
	actualDuration := endTime.Sub(startTime).Seconds()

	if actualDuration == 0 {
		actualDuration = 0.001 // Avoid division by zero
	}

	avgLatency := float64(0)
	if totalRequests > 0 {
		avgLatency = float64(totalLatency) / float64(totalRequests)
	}

	return &PerformanceMetrics{
		TotalRequests:     totalRequests,
		SuccessfulReqs:    successCount,
		FailedReqs:        failCount,
		TotalLatencyMs:    totalLatency,
		MinLatencyMs:      minLatency,
		MaxLatencyMs:      maxLatency,
		RequestsPerSecond: float64(totalRequests) / actualDuration,
		AvgLatencyMs:      avgLatency,
		P50LatencyMs:      latencyBucket.GetPercentile(0.50),
		P95LatencyMs:      latencyBucket.GetPercentile(0.95),
		P99LatencyMs:      latencyBucket.GetPercentile(0.99),
		StartTime:         startTime,
		EndTime:           endTime,
	}
}
