package runner

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"kvstore-benchmarker/pkg/collector"
	"kvstore-benchmarker/pkg/config"
	"kvstore-benchmarker/pkg/kvclient"
)

// BenchmarkRunner orchestrates the benchmark execution
type BenchmarkRunner struct {
	config    *config.BenchmarkConfig
	pool      *kvclient.ConnectionPool
	collector *collector.Collector
	keyGen    *KeyGenerator
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	startTime time.Time
}

// NewBenchmarkRunner creates a new benchmark runner
func NewBenchmarkRunner(cfg *config.BenchmarkConfig) (*BenchmarkRunner, error) {
	// Create connection pool
	pool, err := kvclient.NewConnectionPool(cfg.TargetAddress, cfg.NumConnections)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Create collector
	collector, err := collector.NewCollector(cfg.OutputCSV)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to create collector: %w", err)
	}

	// Create key generator
	keyGen, err := NewKeyGenerator(cfg.KeySpace)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to create key generator: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &BenchmarkRunner{
		config:    cfg,
		pool:      pool,
		collector: collector,
		keyGen:    keyGen,
		ctx:       ctx,
		cancel:    cancel,
		startTime: time.Now(),
	}, nil
}

// Run executes the benchmark
func (r *BenchmarkRunner) Run() error {
	defer r.cleanup()

	log.Printf("Starting benchmark with config: %s", r.config.String())

	// Start collector
	r.collector.Start(r.ctx)

	// Health check
	if err := r.pool.HealthCheck(r.ctx, 5*time.Second); err != nil {
		log.Printf("Warning: health check failed: %v", err)
	}

	// Warm-up phase
	if r.config.WarmupDuration > 0 {
		log.Printf("Starting warm-up phase for %v", r.config.WarmupDuration)
		r.runWorkers(r.config.WarmupDuration, true)
		log.Printf("Warm-up phase completed")
	}

	// Actual benchmark phase
	log.Printf("Starting benchmark phase for %v", r.config.Duration)
	r.runWorkers(r.config.Duration, false)

	// Print final results
	r.printResults()

	return nil
}

// runWorkers starts the worker goroutines for the specified duration
func (r *BenchmarkRunner) runWorkers(duration time.Duration, isWarmup bool) {
	ctx, cancel := context.WithTimeout(r.ctx, duration)
	defer cancel()

	// Start workers
	for i := 0; i < r.config.NumWorkers; i++ {
		r.wg.Add(1)
		go r.worker(ctx, i, isWarmup)
	}

	// Start progress reporter if not in warmup
	if !isWarmup {
		go r.progressReporter(ctx)
	}

	// Wait for completion
	r.wg.Wait()
}

// worker is the main worker goroutine
func (r *BenchmarkRunner) worker(ctx context.Context, workerID int, isWarmup bool) {
	defer r.wg.Done()

	client := r.pool.GetClient()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			r.performOperation(ctx, client, isWarmup, workerID)
		}
	}
}

// performOperation performs a single operation based on configured ratios
func (r *BenchmarkRunner) performOperation(ctx context.Context, client *kvclient.Client, isWarmup bool, workerID int) {
	// Select operation based on ratios
	op := r.selectOperation()

	// Get key and value
	key := r.keyGen.GetRandomKey()
	var value []byte
	var err error

	start := time.Now()

	switch op {
	case "Get":
		_, err = client.Get(ctx, key)
	case "Put":
		value, err = GenerateValue(r.config.ValueSize)
		if err == nil {
			_, err = client.Put(ctx, key, value)
		}
	case "Delete":
		_, err = client.Delete(ctx, key)
	}

	latency := time.Since(start).Milliseconds()

	// Create result
	result := &collector.BenchmarkResult{
		Method:    op,
		LatencyMs: float64(latency),
		Error:     err,
		Timestamp: time.Now(),
	}

	// Add to collector (only if not warmup)
	if !isWarmup {
		r.collector.AddResult(result)
	}

	// Log if configured
	if r.config.LogRequests || (r.config.LogErrors && err != nil) {
		if err != nil {
			log.Printf("Worker %d: %s failed for key %x: %v", workerID, op, key, err)
		} else if r.config.LogRequests {
			log.Printf("Worker %d: %s succeeded for key %x in %dms", workerID, op, key, latency)
		}
	}
}

// selectOperation selects an operation based on configured ratios
func (r *BenchmarkRunner) selectOperation() string {
	// Create weighted distribution
	dist := make([]string, 0, r.config.ReadRatio+r.config.WriteRatio+r.config.DeleteRatio)

	// Add operations based on ratios
	for i := 0; i < r.config.ReadRatio; i++ {
		dist = append(dist, "Get")
	}
	for i := 0; i < r.config.WriteRatio; i++ {
		dist = append(dist, "Put")
	}
	for i := 0; i < r.config.DeleteRatio; i++ {
		dist = append(dist, "Delete")
	}

	// Select random operation
	return dist[rand.Intn(len(dist))]
}

// progressReporter reports progress at regular intervals
func (r *BenchmarkRunner) progressReporter(ctx context.Context) {
	ticker := time.NewTicker(r.config.ReportInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.printProgress()
		}
	}
}

// printProgress prints current progress with aggregated percentiles
func (r *BenchmarkRunner) printProgress() {
	stats := r.collector.GetAggregatedStats()
	if stats.Count == 0 {
		return
	}

	// Calculate RPS based on the report interval
	elapsed := time.Since(r.startTime).Seconds()
	rps := float64(stats.Count) / elapsed

	log.Printf("[%s] Total: %d | RPS: %.0f | Avg: %.1fms | P50: %.1fms | P95: %.1fms | P99: %.1fms | Errors: %d (%.1f%%)",
		time.Now().Format("15:04:05"),
		stats.Count,
		rps,
		stats.AvgLatency,
		stats.P50Latency,
		stats.P95Latency,
		stats.P99Latency,
		stats.ErrorCount,
		stats.ErrorRate,
	)
}

// printResults prints final benchmark results with detailed aggregated statistics
func (r *BenchmarkRunner) printResults() {
	log.Printf("\n=== FINAL RESULTS ===")

	// Print per-method statistics
	stats := r.collector.GetStats()
	for method, stat := range stats {
		if stat.Count == 0 {
			continue
		}

		log.Printf("\n%s:", method)
		log.Printf("  Count: %d", stat.Count)
		log.Printf("  Errors: %d (%.2f%%)", stat.ErrorCount, stat.ErrorRate)
		log.Printf("  Avg Latency: %.2fms", stat.AvgLatency)
		log.Printf("  P50 Latency: %.2fms", stat.P50Latency)
		log.Printf("  P95 Latency: %.2fms", stat.P95Latency)
		log.Printf("  P99 Latency: %.2fms", stat.P99Latency)
		log.Printf("  Min Latency: %.2fms", stat.MinLatency)
		log.Printf("  Max Latency: %.2fms", stat.MaxLatency)
	}

	// Print aggregated statistics
	aggregated := r.collector.GetAggregatedStats()
	if aggregated.Count > 0 {
		log.Printf("\n=== AGGREGATED STATISTICS ===")
		log.Printf("Total Operations: %d", aggregated.Count)
		log.Printf("Total Errors: %d (%.2f%%)", aggregated.ErrorCount, aggregated.ErrorRate)
		log.Printf("Overall Avg Latency: %.2fms", aggregated.AvgLatency)
		log.Printf("Overall P50 Latency: %.2fms", aggregated.P50Latency)
		log.Printf("Overall P95 Latency: %.2fms", aggregated.P95Latency)
		log.Printf("Overall P99 Latency: %.2fms", aggregated.P99Latency)
		log.Printf("Overall Min Latency: %.2fms", aggregated.MinLatency)
		log.Printf("Overall Max Latency: %.2fms", aggregated.MaxLatency)

		// Calculate final throughput
		totalDuration := time.Since(r.startTime).Seconds()
		finalRPS := float64(aggregated.Count) / totalDuration
		log.Printf("Final Throughput: %.0f ops/sec", finalRPS)
	}
}

// cleanup performs cleanup operations
func (r *BenchmarkRunner) cleanup() {
	r.cancel()
	r.collector.Stop()
	r.pool.Close()
}
