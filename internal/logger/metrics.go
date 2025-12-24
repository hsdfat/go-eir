package logger

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// EquipmentCheckTotal counts total equipment check requests
	EquipmentCheckTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "eir_equipment_check_total",
			Help: "Total number of equipment check requests",
		},
		[]string{"source", "status"},
	)

	// EquipmentCheckDuration measures equipment check latency
	EquipmentCheckDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "eir_equipment_check_duration_seconds",
			Help:    "Equipment check request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"source", "status"},
	)

	// DatabaseQueryDuration measures database query latency
	DatabaseQueryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "eir_database_query_duration_seconds",
			Help:    "Database query duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation"},
	)

	// CacheHitTotal counts cache hits and misses
	CacheHitTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "eir_cache_hit_total",
			Help: "Total number of cache hits and misses",
		},
		[]string{"result"}, // "hit" or "miss"
	)

	// ActiveConnections tracks active Diameter connections
	ActiveConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "eir_active_diameter_connections",
			Help: "Number of active Diameter connections",
		},
	)

	// EquipmentByStatus tracks equipment count by status
	EquipmentByStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "eir_equipment_by_status",
			Help: "Number of equipment by status",
		},
		[]string{"status"},
	)
)

// InitMetrics registers Prometheus metrics
func InitMetrics() {
	prometheus.MustRegister(EquipmentCheckTotal)
	prometheus.MustRegister(EquipmentCheckDuration)
	prometheus.MustRegister(DatabaseQueryDuration)
	prometheus.MustRegister(CacheHitTotal)
	prometheus.MustRegister(ActiveConnections)
	prometheus.MustRegister(EquipmentByStatus)
}

// MetricsHandler returns HTTP handler for Prometheus metrics
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}
