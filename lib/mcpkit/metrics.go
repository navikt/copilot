package mcpkit

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"
)

// UnknownTool is the label for a call to a tool the server does not have, so
// a caller cannot grow the label set with made-up names.
const UnknownTool = "unknown"

var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

type toolStats struct {
	requests, errors uint64
	buckets          []uint64 // cumulative counts per durationBuckets entry
	sum              float64
}

// ToolMetrics counts calls, errors and latency per tool, and serves them in
// the Prometheus text format. The only label is the tool name, taken from a
// fixed list, so cardinality is bounded by the number of tools plus one.
type ToolMetrics struct {
	mu    sync.Mutex
	tools map[string]*toolStats
	names []string
}

// NewToolMetrics returns metrics for the named tools.
func NewToolMetrics(tools []string) *ToolMetrics {
	m := &ToolMetrics{tools: map[string]*toolStats{}}
	for _, name := range append(slices.Clone(tools), UnknownTool) {
		if _, dup := m.tools[name]; !dup {
			m.tools[name] = &toolStats{buckets: make([]uint64, len(durationBuckets))}
			m.names = append(m.names, name)
		}
	}
	slices.Sort(m.names)
	return m
}

// Observe records one call. A tool not passed to NewToolMetrics counts as UnknownTool.
func (m *ToolMetrics) Observe(tool string, failed bool, d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st, ok := m.tools[tool]
	if !ok {
		st = m.tools[UnknownTool]
	}
	st.requests++
	if failed {
		st.errors++
	}
	secs := d.Seconds()
	st.sum += secs
	for i, b := range durationBuckets {
		if secs <= b {
			st.buckets[i]++
		}
	}
}

// ServeHTTP writes the metrics in the Prometheus text exposition format.
func (m *ToolMetrics) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	fmt.Fprintf(w, "# HELP mcp_tool_requests_total MCP tool calls.\n")
	fmt.Fprintf(w, "# TYPE mcp_tool_requests_total counter\n")
	for _, n := range m.names {
		fmt.Fprintf(w, "mcp_tool_requests_total{tool=%q} %d\n", n, m.tools[n].requests)
	}
	fmt.Fprintf(w, "# HELP mcp_tool_errors_total MCP tool calls that returned an error.\n")
	fmt.Fprintf(w, "# TYPE mcp_tool_errors_total counter\n")
	for _, n := range m.names {
		fmt.Fprintf(w, "mcp_tool_errors_total{tool=%q} %d\n", n, m.tools[n].errors)
	}
	fmt.Fprintf(w, "# HELP mcp_tool_duration_seconds MCP tool call latency.\n")
	fmt.Fprintf(w, "# TYPE mcp_tool_duration_seconds histogram\n")
	for _, n := range m.names {
		st := m.tools[n]
		for i, b := range durationBuckets {
			fmt.Fprintf(w, "mcp_tool_duration_seconds_bucket{tool=%q,le=%q} %d\n", n, strconv.FormatFloat(b, 'g', -1, 64), st.buckets[i])
		}
		fmt.Fprintf(w, "mcp_tool_duration_seconds_bucket{tool=%q,le=\"+Inf\"} %d\n", n, st.requests)
		fmt.Fprintf(w, "mcp_tool_duration_seconds_sum{tool=%q} %s\n", n, strconv.FormatFloat(st.sum, 'g', -1, 64))
		fmt.Fprintf(w, "mcp_tool_duration_seconds_count{tool=%q} %d\n", n, st.requests)
	}
}
