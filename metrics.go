package main

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Metrics is a simple, thread-safe store of sync statistics that can be
// serialised to Prometheus text format without any external dependencies.
type Metrics struct {
	mu sync.RWMutex

	syncsTotal map[string]int64 // label: status="success"|"error"
	dnsOps     map[string]int64 // label: op="create"|"update"|"delete"|"error"
	nodeCount  int

	lastDurSec float64
	lastSyncAt time.Time
	lastUp     int // 1 success, 0 error, -1 never synced
	lastStats  *SyncStats
	lastErr    string
}

// globalMetrics is the single metrics instance updated by all sync operations.
var globalMetrics = &Metrics{
	syncsTotal: map[string]int64{"success": 0, "error": 0},
	dnsOps:     map[string]int64{"create": 0, "update": 0, "delete": 0, "error": 0},
	lastUp:     -1,
}

func (m *Metrics) recordSync(dur time.Duration, stats *SyncStats, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastDurSec = dur.Seconds()
	m.lastSyncAt = time.Now()

	if err != nil {
		m.syncsTotal["error"]++
		m.lastUp = 0
		m.lastErr = err.Error()
		m.lastStats = nil
		return
	}

	m.syncsTotal["success"]++
	m.lastUp = 1
	m.lastErr = ""
	m.lastStats = stats

	m.dnsOps["create"] += int64(stats.Created)
	m.dnsOps["update"] += int64(stats.Updated)
	m.dnsOps["delete"] += int64(stats.Deleted)
	m.dnsOps["error"] += int64(stats.Errors)
}

func (m *Metrics) setNodeCount(n int) {
	m.mu.Lock()
	m.nodeCount = n
	m.mu.Unlock()
}

// text returns Prometheus text-format metrics.
func (m *Metrics) text() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var b strings.Builder
	line := func(format string, args ...interface{}) {
		fmt.Fprintf(&b, format+"\n", args...)
	}

	line("# HELP hsync_syncs_total Total sync operations by status")
	line("# TYPE hsync_syncs_total counter")
	for status, v := range m.syncsTotal {
		line(`hsync_syncs_total{status=%q} %d`, status, v)
	}

	line("# HELP hsync_dns_operations_total Cumulative DNS record operations")
	line("# TYPE hsync_dns_operations_total counter")
	for op, v := range m.dnsOps {
		line(`hsync_dns_operations_total{op=%q} %d`, op, v)
	}

	line("# HELP hsync_nodes_total Headscale nodes seen in the last sync")
	line("# TYPE hsync_nodes_total gauge")
	line("hsync_nodes_total %d", m.nodeCount)

	line("# HELP hsync_last_sync_duration_seconds Wall-clock duration of the last sync")
	line("# TYPE hsync_last_sync_duration_seconds gauge")
	line("hsync_last_sync_duration_seconds %g", m.lastDurSec)

	line("# HELP hsync_last_sync_timestamp_seconds Unix timestamp of the last sync attempt")
	line("# TYPE hsync_last_sync_timestamp_seconds gauge")
	ts := float64(0)
	if !m.lastSyncAt.IsZero() {
		ts = float64(m.lastSyncAt.Unix())
	}
	line("hsync_last_sync_timestamp_seconds %g", ts)

	line("# HELP hsync_up 1 if the last sync succeeded, 0 if it errored, -1 if never run")
	line("# TYPE hsync_up gauge")
	line("hsync_up %d", m.lastUp)

	return b.String()
}

// statusJSON returns a JSON-serialisable snapshot for the /status endpoint.
func (m *Metrics) statusJSON() interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var lastSync, lastDur string
	if !m.lastSyncAt.IsZero() {
		lastSync = m.lastSyncAt.UTC().Format(time.RFC3339)
		lastDur = time.Duration(m.lastDurSec * float64(time.Second)).Round(time.Millisecond).String()
	}

	up := "never"
	switch m.lastUp {
	case 1:
		up = "ok"
	case 0:
		up = "error"
	}

	return map[string]interface{}{
		"status":        up,
		"last_sync":     lastSync,
		"last_duration": lastDur,
		"last_error":    m.lastErr,
		"last_stats":    m.lastStats,
		"nodes_seen":    m.nodeCount,
	}
}
