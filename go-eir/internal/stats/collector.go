package stats

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	unifiedStats "github.com/hsdfat/telco/stats"
)

// StatsCollector collects real-time EIR statistics
type StatsCollector struct {
	startTime time.Time

	// Atomic counters for requests
	totalRequests      atomic.Uint64
	successfulRequests atomic.Uint64
	failedRequests     atomic.Uint64

	diameterRequests atomic.Uint64
	httpRequests     atomic.Uint64

	diameterSuccess atomic.Uint64
	diameterFailed  atomic.Uint64
	httpSuccess     atomic.Uint64
	httpFailed      atomic.Uint64

	// Cache statistics
	cacheHits   atomic.Uint64
	cacheMisses atomic.Uint64

	// Database operations
	dbQueries atomic.Uint64
	dbInserts atomic.Uint64
	dbUpdates atomic.Uint64
	dbDeletes atomic.Uint64

	// Equipment status counters
	whitelistedCount atomic.Uint64
	blacklistedCount atomic.Uint64
	greylistedCount  atomic.Uint64

	// Result code counters
	mu              sync.RWMutex
	resultCodeCount map[string]uint64

	// Active connections
	activeConnections atomic.Int64
}

// NewStatsCollector creates a new stats collector
func NewStatsCollector() *StatsCollector {
	return &StatsCollector{
		startTime:       time.Now(),
		resultCodeCount: make(map[string]uint64),
	}
}

// RecordRequest records a new request with source and success status
func (c *StatsCollector) RecordRequest(source string, success bool) {
	c.totalRequests.Add(1)

	if success {
		c.successfulRequests.Add(1)
	} else {
		c.failedRequests.Add(1)
	}

	switch source {
	case "diameter":
		c.diameterRequests.Add(1)
		if success {
			c.diameterSuccess.Add(1)
		} else {
			c.diameterFailed.Add(1)
		}
	case "http":
		c.httpRequests.Add(1)
		if success {
			c.httpSuccess.Add(1)
		} else {
			c.httpFailed.Add(1)
		}
	}
}

// GetStats returns current statistics in unified format
func (c *StatsCollector) GetStats() *unifiedStats.ServiceStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.totalRequests.Load()
	success := c.successfulRequests.Load()
	failed := c.failedRequests.Load()

	diamReq := c.diameterRequests.Load()
	diamSuccess := c.diameterSuccess.Load()
	diamFailed := c.diameterFailed.Load()

	httpReq := c.httpRequests.Load()
	httpSuccess := c.httpSuccess.Load()
	httpFailed := c.httpFailed.Load()

	hits := c.cacheHits.Load()
	misses := c.cacheMisses.Load()

	var hitRate float64
	if totalCache := hits + misses; totalCache > 0 {
		hitRate = (float64(hits) / float64(totalCache)) * 100
	}

	uptime := time.Since(c.startTime)
	var tps float64
	if uptime.Seconds() > 0 {
		tps = float64(total) / uptime.Seconds()
	}
	// Copy result code map
	resultCodes := make(map[int]uint64)
	for code, count := range c.resultCodeCount {
		var codeInt int
		if _, err := fmt.Sscanf(code, "%d", &codeInt); err == nil {
			resultCodes[codeInt] = count
		}
	}

	stats := &unifiedStats.ServiceStats{
		ServiceName:    "EIR",
		ServiceVersion: "1.0.0",
		Uptime:         uptime.String(),
		Timestamp:      time.Now(),
		Connections: unifiedStats.ConnectionStats{
			Active: uint64(c.activeConnections.Load()),
		},
		Requests: unifiedStats.RequestStats{
			Total:   total,
			Success: success,
			Failed:  failed,
			BySource: map[string]unifiedStats.SourceStats{
				"diameter": {
					Total:   diamReq,
					Success: diamSuccess,
					Failed:  diamFailed,
				},
				"http": {
					Total:   httpReq,
					Success: httpSuccess,
					Failed:  httpFailed,
				},
			},
			ByOperation: make(map[string]unifiedStats.OperationStats),
		},
		Performance: unifiedStats.PerformanceStats{
			RequestsPerSecond: tps,
		},
		Errors: unifiedStats.ErrorStats{
			ByType:      make(map[string]uint64),
			ByInterface: make(map[string]uint64),
		},
		CustomMetrics: map[string]interface{}{
			"eir": &unifiedStats.EIRStats{
				EquipmentChecks: unifiedStats.EquipmentCheckStats{
					Total: total,
					ByInterface: map[string]uint64{
						"diameter": diamReq,
						"http":     httpReq,
					},
					ByResultCode: resultCodes,
				},
				CacheStats: unifiedStats.CacheStats{
					Hits:    hits,
					Misses:  misses,
					HitRate: hitRate,
				},
				DatabaseOps: unifiedStats.DatabaseOperationStats{
					Queries: c.dbQueries.Load(),
					Inserts: c.dbInserts.Load(),
					Updates: c.dbUpdates.Load(),
					Deletes: c.dbDeletes.Load(),
				},
				ByEquipmentStatus: map[string]uint64{
					"whitelisted": c.whitelistedCount.Load(),
					"blacklisted": c.blacklistedCount.Load(),
					"greylisted":  c.greylistedCount.Load(),
				},
			},
		},
	}

	return stats
}
