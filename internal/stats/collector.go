package stats

import (
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

	// Result code counters per interface
	mu                  sync.RWMutex
	diameterResultCodes map[int]uint64
	httpResultCodes     map[int]uint64

	// Active connections
	activeConnections atomic.Int64
}

// NewStatsCollector creates a new stats collector
func NewStatsCollector() *StatsCollector {
	return &StatsCollector{
		startTime:           time.Now(),
		diameterResultCodes: make(map[int]uint64),
		httpResultCodes:     make(map[int]uint64),
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

// RecordResultCode records a result code for equipment checks from a specific interface
func (c *StatsCollector) RecordResultCode(source string, code int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch source {
	case "diameter":
		c.diameterResultCodes[code]++
	case "http":
		c.httpResultCodes[code]++
	}
}

// RecordCacheHit records a cache operation
func (c *StatsCollector) RecordCacheHit(hit bool) {
	if hit {
		c.cacheHits.Add(1)
	} else {
		c.cacheMisses.Add(1)
	}
}

// RecordDatabaseOperation records a database operation
func (c *StatsCollector) RecordDatabaseOperation(operation string) {
	switch operation {
	case "query", "select":
		c.dbQueries.Add(1)
	case "insert":
		c.dbInserts.Add(1)
	case "update":
		c.dbUpdates.Add(1)
	case "delete":
		c.dbDeletes.Add(1)
	}
}

// RecordEquipmentStatus records equipment by status
func (c *StatsCollector) RecordEquipmentStatus(status string) {
	switch status {
	case "whitelisted":
		c.whitelistedCount.Add(1)
	case "blacklisted":
		c.blacklistedCount.Add(1)
	case "greylisted":
		c.greylistedCount.Add(1)
	}
}

// SetActiveConnections sets the current number of active Diameter connections
func (c *StatsCollector) SetActiveConnections(count int64) {
	c.activeConnections.Store(count)
}

// IncrementActiveConnections increments active connection count
func (c *StatsCollector) IncrementActiveConnections() {
	c.activeConnections.Add(1)
}

// DecrementActiveConnections decrements active connection count
func (c *StatsCollector) DecrementActiveConnections() {
	c.activeConnections.Add(-1)
}

// GetStats returns current statistics as interface{} (implements StatsCollector interface)
func (c *StatsCollector) GetStats() interface{} {
	return c.GetStatsData()
}

// GetStatsData returns current statistics in unified format
func (c *StatsCollector) GetStatsData() *unifiedStats.ServiceStats {
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

	// Copy result code maps per interface
	diameterResultCodes := make(map[int]uint64)
	for code, count := range c.diameterResultCodes {
		diameterResultCodes[code] = count
	}

	httpResultCodes := make(map[int]uint64)
	for code, count := range c.httpResultCodes {
		httpResultCodes[code] = count
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
					Total:   total,
					Success: success,
					Failed:  failed,
					ByInterface: map[string]unifiedStats.InterfaceCheckStats{
						"diameter": {
							Total:        diamReq,
							Success:      diamSuccess,
							Failed:       diamFailed,
							ByResultCode: diameterResultCodes,
						},
						"http": {
							Total:        httpReq,
							Success:      httpSuccess,
							Failed:       httpFailed,
							ByResultCode: httpResultCodes,
						},
					},
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

// Reset resets all counters (useful for testing)
func (c *StatsCollector) Reset() {
	c.startTime = time.Now()
	c.totalRequests.Store(0)
	c.successfulRequests.Store(0)
	c.failedRequests.Store(0)
	c.diameterRequests.Store(0)
	c.httpRequests.Store(0)
	c.diameterSuccess.Store(0)
	c.diameterFailed.Store(0)
	c.httpSuccess.Store(0)
	c.httpFailed.Store(0)
	c.cacheHits.Store(0)
	c.cacheMisses.Store(0)
	c.dbQueries.Store(0)
	c.dbInserts.Store(0)
	c.dbUpdates.Store(0)
	c.dbDeletes.Store(0)
	c.whitelistedCount.Store(0)
	c.blacklistedCount.Store(0)
	c.greylistedCount.Store(0)
	c.activeConnections.Store(0)

	c.mu.Lock()
	c.diameterResultCodes = make(map[int]uint64)
	c.httpResultCodes = make(map[int]uint64)
	c.mu.Unlock()
}
