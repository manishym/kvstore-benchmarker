package collector

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"sort"
	"sync"
	"time"
)

// BenchmarkResult represents a single benchmark operation result
type BenchmarkResult struct {
	Method    string
	LatencyMs float64
	Error     error
	Timestamp time.Time
}

// Metrics holds aggregated metrics for a method
type Metrics struct {
	Method       string
	Count        int64
	ErrorCount   int64
	TotalLatency float64
	MinLatency   float64
	MaxLatency   float64
	Latencies    []float64 // For percentile calculations
	mu           sync.RWMutex
	maxLatencies int // Maximum number of latencies to store
}

// NewMetrics creates a new metrics instance
func NewMetrics(method string) *Metrics {
	return &Metrics{
		Method:       method,
		MinLatency:   float64(^uint(0) >> 1), // Max float64
		MaxLatency:   0,
		Latencies:    make([]float64, 0, 1000), // Pre-allocate for efficiency
		maxLatencies: 10000,                    // Default limit
	}
}

// SetMaxLatencies sets the maximum number of latencies to store for percentile calculation
func (m *Metrics) SetMaxLatencies(max int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maxLatencies = max
}

// AddResult adds a result to the metrics
func (m *Metrics) AddResult(result *BenchmarkResult) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.Count++
	if result.Error != nil {
		m.ErrorCount++
		return
	}

	m.TotalLatency += result.LatencyMs
	m.Latencies = append(m.Latencies, result.LatencyMs)

	// Limit the number of stored latencies to prevent memory issues
	if len(m.Latencies) > m.maxLatencies {
		// Keep only the most recent latencies
		m.Latencies = m.Latencies[len(m.Latencies)-m.maxLatencies:]
	}

	if result.LatencyMs < m.MinLatency {
		m.MinLatency = result.LatencyMs
	}
	if result.LatencyMs > m.MaxLatency {
		m.MaxLatency = result.LatencyMs
	}
}

// GetStats returns computed statistics
func (m *Metrics) GetStats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.Count == 0 {
		return Stats{}
	}

	successCount := m.Count - m.ErrorCount
	if successCount == 0 {
		return Stats{
			Method:     m.Method,
			Count:      m.Count,
			ErrorCount: m.ErrorCount,
			ErrorRate:  100.0,
		}
	}

	avgLatency := m.TotalLatency / float64(successCount)
	errorRate := float64(m.ErrorCount) / float64(m.Count) * 100.0

	// Calculate percentiles
	sortedLatencies := make([]float64, len(m.Latencies))
	copy(sortedLatencies, m.Latencies)
	sort.Float64s(sortedLatencies)

	p50 := percentile(sortedLatencies, 50)
	p95 := percentile(sortedLatencies, 95)
	p99 := percentile(sortedLatencies, 99)

	return Stats{
		Method:     m.Method,
		Count:      m.Count,
		ErrorCount: m.ErrorCount,
		ErrorRate:  errorRate,
		AvgLatency: avgLatency,
		MinLatency: m.MinLatency,
		MaxLatency: m.MaxLatency,
		P50Latency: p50,
		P95Latency: p95,
		P99Latency: p99,
	}
}

// Stats represents computed statistics
type Stats struct {
	Method       string
	Count        int64
	ErrorCount   int64
	ErrorRate    float64
	AvgLatency   float64
	MinLatency   float64
	MaxLatency   float64
	P50Latency   float64
	P95Latency   float64
	P99Latency   float64
	TotalLatency float64
}

// Collector manages result collection and reporting
type Collector struct {
	metrics   map[string]*Metrics
	results   chan *BenchmarkResult
	done      chan struct{}
	csvWriter *csv.Writer
	csvFile   *os.File
	mu        sync.RWMutex
}

// NewCollector creates a new collector
func NewCollector(csvPath string) (*Collector, error) {
	var csvFile *os.File
	var csvWriter *csv.Writer

	if csvPath != "" {
		var err error
		csvFile, err = os.Create(csvPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create CSV file: %w", err)
		}

		csvWriter = csv.NewWriter(csvFile)
		// Write CSV header for aggregated metrics
		csvWriter.Write([]string{
			"timestamp",
			"method",
			"total_ops",
			"success_ops",
			"error_ops",
			"error_rate_pct",
			"avg_latency_ms",
			"p50_latency_ms",
			"p95_latency_ms",
			"p99_latency_ms",
			"min_latency_ms",
			"max_latency_ms",
			"throughput_ops_per_sec",
		})
	}

	return &Collector{
		metrics:   make(map[string]*Metrics),
		results:   make(chan *BenchmarkResult, 10000), // Buffered channel
		done:      make(chan struct{}),
		csvWriter: csvWriter,
		csvFile:   csvFile,
	}, nil
}

// Start starts the collector goroutine
func (c *Collector) Start(ctx context.Context) {
	go c.run(ctx)
}

// Stop stops the collector and writes final aggregated metrics to CSV
func (c *Collector) Stop() {
	close(c.done)

	// Write final aggregated metrics to CSV
	c.WriteAggregatedMetricsToCSV()

	if c.csvFile != nil {
		c.csvWriter.Flush()
		c.csvFile.Close()
	}
}

// AddResult adds a result to the collector
func (c *Collector) AddResult(result *BenchmarkResult) {
	select {
	case c.results <- result:
	default:
		// Channel is full, log warning
		log.Printf("Warning: results channel is full, dropping result")
	}
}

// run is the main collector loop
func (c *Collector) run(ctx context.Context) {
	for {
		select {
		case result := <-c.results:
			c.processResult(result)
		case <-ctx.Done():
			return
		case <-c.done:
			return
		}
	}
}

// processResult processes a single result
func (c *Collector) processResult(result *BenchmarkResult) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Get or create metrics for this method
	metrics, exists := c.metrics[result.Method]
	if !exists {
		metrics = NewMetrics(result.Method)
		c.metrics[result.Method] = metrics
	}

	// Add to metrics
	metrics.AddResult(result)

	// Note: We don't write individual operations to CSV anymore
	// CSV will be written with aggregated metrics at the end
}

// GetAggregatedStats returns aggregated statistics across all methods with proper percentile calculation
func (c *Collector) GetAggregatedStats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var allLatencies []float64
	var totalCount int64
	var totalErrorCount int64
	var totalLatency float64

	// Collect all latencies and basic stats
	for _, metrics := range c.metrics {
		metrics.mu.RLock()
		allLatencies = append(allLatencies, metrics.Latencies...)
		totalCount += metrics.Count
		totalErrorCount += metrics.ErrorCount
		totalLatency += metrics.TotalLatency
		metrics.mu.RUnlock()
	}

	if totalCount == 0 {
		return Stats{Method: "AGGREGATED"}
	}

	// Calculate aggregated statistics
	successCount := totalCount - totalErrorCount
	errorRate := float64(totalErrorCount) / float64(totalCount) * 100.0
	avgLatency := totalLatency / float64(successCount)

	var minLatency, maxLatency, p50, p95, p99 float64

	if len(allLatencies) > 0 {
		sort.Float64s(allLatencies)
		minLatency = allLatencies[0]
		maxLatency = allLatencies[len(allLatencies)-1]
		p50 = percentile(allLatencies, 50)
		p95 = percentile(allLatencies, 95)
		p99 = percentile(allLatencies, 99)
	}

	return Stats{
		Method:       "AGGREGATED",
		Count:        totalCount,
		ErrorCount:   totalErrorCount,
		ErrorRate:    errorRate,
		AvgLatency:   avgLatency,
		MinLatency:   minLatency,
		MaxLatency:   maxLatency,
		P50Latency:   p50,
		P95Latency:   p95,
		P99Latency:   p99,
		TotalLatency: totalLatency,
	}
}

// GetStats returns statistics for all methods
func (c *Collector) GetStats() map[string]Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := make(map[string]Stats)
	for method, metrics := range c.metrics {
		stats[method] = metrics.GetStats()
	}
	return stats
}

// GetTotalStats returns combined statistics across all methods
func (c *Collector) GetTotalStats() Stats {
	stats := c.GetStats()

	var total Stats
	total.Method = "TOTAL"

	// Collect all latencies from all methods for proper percentile calculation
	var allLatencies []float64
	var totalSuccessCount int64

	for _, stat := range stats {
		total.Count += stat.Count
		total.ErrorCount += stat.ErrorCount
		total.TotalLatency += stat.AvgLatency * float64(stat.Count-stat.ErrorCount)
		totalSuccessCount += stat.Count - stat.ErrorCount

		// Get the actual latencies from the metrics for proper percentile calculation
		c.mu.RLock()
		if metrics, exists := c.metrics[stat.Method]; exists {
			metrics.mu.RLock()
			allLatencies = append(allLatencies, metrics.Latencies...)
			metrics.mu.RUnlock()
		}
		c.mu.RUnlock()
	}

	if total.Count > 0 {
		total.ErrorRate = float64(total.ErrorCount) / float64(total.Count) * 100.0
		total.AvgLatency = total.TotalLatency / float64(totalSuccessCount)

		// Calculate percentiles from all latencies combined
		if len(allLatencies) > 0 {
			sort.Float64s(allLatencies)
			total.MinLatency = allLatencies[0]
			total.MaxLatency = allLatencies[len(allLatencies)-1]
			total.P50Latency = percentile(allLatencies, 50)
			total.P95Latency = percentile(allLatencies, 95)
			total.P99Latency = percentile(allLatencies, 99)
		}
	}

	return total
}

// percentile calculates the nth percentile from sorted values
func percentile(values []float64, n int) float64 {
	if len(values) == 0 {
		return 0
	}

	// Calculate the index for the nth percentile
	index := float64(n) / 100.0 * float64(len(values)-1)

	// Handle integer index
	if index == float64(int(index)) {
		return values[int(index)]
	}

	// Handle fractional index (interpolate between two values)
	lowerIndex := int(index)
	upperIndex := lowerIndex + 1

	if upperIndex >= len(values) {
		return values[lowerIndex]
	}

	// Linear interpolation
	fraction := index - float64(lowerIndex)
	return values[lowerIndex] + fraction*(values[upperIndex]-values[lowerIndex])
}

// WriteAggregatedMetricsToCSV writes aggregated metrics for all methods to CSV
func (c *Collector) WriteAggregatedMetricsToCSV() {
	var throughput float64

	if c.csvWriter == nil {
		return
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	timestamp := time.Now().Format(time.RFC3339Nano)

	// Write per-method aggregated metrics
	for _, metrics := range c.metrics {
		stats := metrics.GetStats()
		if stats.Count == 0 {
			continue
		}
		elapsedTime := time.Since(metrics.StartTime).Seconds()
		if elapsedTime > 0 {
			throughput = float64(stats.Count-stats.ErrorCount) / elapsedTime
		} else {
			throughput = 0.0
		}
		c.csvWriter.Write([]string{
			timestamp,
			stats.Method,
			fmt.Sprintf("%d", stats.Count),
			fmt.Sprintf("%d", stats.Count-stats.ErrorCount),
			fmt.Sprintf("%d", stats.ErrorCount),
			fmt.Sprintf("%.2f", stats.ErrorRate),
			fmt.Sprintf("%.3f", stats.AvgLatency),
			fmt.Sprintf("%.3f", stats.P50Latency),
			fmt.Sprintf("%.3f", stats.P95Latency),
			fmt.Sprintf("%.3f", stats.P99Latency),
			fmt.Sprintf("%.3f", stats.MinLatency),
			fmt.Sprintf("%.3f", stats.MaxLatency),
			fmt.Sprintf("%.0f", throughput),
		})
	}

	// Write overall aggregated metrics
	aggregated := c.GetAggregatedStats()
	if aggregated.Count > 0 {
		throughput := float64(aggregated.Count - aggregated.ErrorCount) // ops per second

		c.csvWriter.Write([]string{
			timestamp,
			"AGGREGATED",
			fmt.Sprintf("%d", aggregated.Count),
			fmt.Sprintf("%d", aggregated.Count-aggregated.ErrorCount),
			fmt.Sprintf("%d", aggregated.ErrorCount),
			fmt.Sprintf("%.2f", aggregated.ErrorRate),
			fmt.Sprintf("%.3f", aggregated.AvgLatency),
			fmt.Sprintf("%.3f", aggregated.P50Latency),
			fmt.Sprintf("%.3f", aggregated.P95Latency),
			fmt.Sprintf("%.3f", aggregated.P99Latency),
			fmt.Sprintf("%.3f", aggregated.MinLatency),
			fmt.Sprintf("%.3f", aggregated.MaxLatency),
			fmt.Sprintf("%.0f", throughput),
		})
	}
}
