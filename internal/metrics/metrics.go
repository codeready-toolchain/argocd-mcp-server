package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (

	// ToolCallsTotal counts total tool invocation by tool name
	MCPCallsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mcp_calls_total",
			Help: "Total number of MCP calls",
		},
		[]string{"method", "name", "success"},
	)

	MCPCallDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mcp_call_duration_seconds",
			Help:    "Duration of MCP calls in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "name", "success"},
	)
)
