package middleware

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Metrics tracks request statistics
type Metrics struct {
	TotalRequests    uint64
	ErrorCount       uint64
	TotalLatency     uint64
	MinLatency       uint64
	MaxLatency       uint64
	MethodCounts     map[string]uint64
	StatusCodeCounts map[int]uint64
	mu               sync.RWMutex
	TotalDuration    uint64
}

// NewMetrics creates a new Metrics instance
func NewMetrics() *Metrics {
	return &Metrics{
		MethodCounts:     make(map[string]uint64),
		StatusCodeCounts: make(map[int]uint64),
		MinLatency:       ^uint64(0), // Initialize to max uint64
	}
}

// RecordRequest records a request with its metrics
func (m *Metrics) RecordRequest(method string, statusCode int, latency time.Duration) {
	atomic.AddUint64(&m.TotalRequests, 1)

	if statusCode >= 400 {
		atomic.AddUint64(&m.ErrorCount, 1)
	}

	latencyMs := uint64(latency.Milliseconds())
	atomic.AddUint64(&m.TotalLatency, latencyMs)

	// Cập nhật MinLatency (bỏ qua nếu MinLatency chưa được set)
	for {
		current := atomic.LoadUint64(&m.MinLatency)
		if current == 0 || latencyMs < current {
			if atomic.CompareAndSwapUint64(&m.MinLatency, current, latencyMs) {
				break
			}
		} else {
			break
		}
	}

	// Cập nhật MaxLatency
	for {
		current := atomic.LoadUint64(&m.MaxLatency)
		if latencyMs > current {
			if atomic.CompareAndSwapUint64(&m.MaxLatency, current, latencyMs) {
				break
			}
		} else {
			break
		}
	}

	m.mu.Lock()
	m.MethodCounts[method]++
	m.StatusCodeCounts[statusCode]++
	m.mu.Unlock()
}

// GetMetrics returns a copy of the current metrics
func (m *Metrics) GetMetrics() map[string]interface{} {
	m.mu.RLock()
	methodCounts := make(map[string]uint64, len(m.MethodCounts))
	for k, v := range m.MethodCounts {
		methodCounts[k] = v
	}
	statusCodeCounts := make(map[int]uint64, len(m.StatusCodeCounts))
	for k, v := range m.StatusCodeCounts {
		statusCodeCounts[k] = v
	}
	m.mu.RUnlock()

	return map[string]interface{}{
		"total_requests":      atomic.LoadUint64(&m.TotalRequests),
		"method_counts":       methodCounts,
		"status_code_counts":  statusCodeCounts,
		"average_duration_ms": atomic.LoadUint64(&m.TotalDuration) / (atomic.LoadUint64(&m.TotalRequests) + 1), // tránh chia 0
	}
}

// PrintMetrics prints the current metrics to stdout
func (m *Metrics) PrintMetrics() {
	metrics := m.GetMetrics()

	fmt.Println("\n=== Server Metrics ===")
	fmt.Printf("Total Requests: %d\n", metrics["total_requests"])
	fmt.Printf("Average Duration (ms): %d\n", metrics["average_duration_ms"])

	fmt.Println("\nRequests by Method:")
	if methodCounts, ok := metrics["method_counts"].(map[string]uint64); ok {
		for method, count := range methodCounts {
			fmt.Printf("  %s: %d\n", method, count)
		}
	} else if methodCounts, ok := metrics["method_counts"].(map[string]interface{}); ok {
		for method, count := range methodCounts {
			fmt.Printf("  %s: %v\n", method, count)
		}
	}

	fmt.Println("\nRequests by Status Code:")
	if statusCounts, ok := metrics["status_code_counts"].(map[int]uint64); ok {
		for status, count := range statusCounts {
			fmt.Printf("  %d: %d\n", status, count)
		}
	} else if statusCounts, ok := metrics["status_code_counts"].(map[string]interface{}); ok {
		for status, count := range statusCounts {
			fmt.Printf("  %s: %v\n", status, count)
		}
	}

	fmt.Println("=====================")
}
