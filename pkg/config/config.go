package config

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"
)

// BenchmarkConfig holds all benchmark parameters
type BenchmarkConfig struct {
	TargetAddress  string        `json:"target_address"`
	NumConnections int           `json:"num_connections"`
	NumWorkers     int           `json:"num_workers"`
	Duration       time.Duration `json:"duration"`
	WarmupDuration time.Duration `json:"warmup_duration"`
	KeySpace       int           `json:"key_space"`
	ValueSize      int           `json:"value_size"`
	ReadRatio      int           `json:"read_ratio"`
	WriteRatio     int           `json:"write_ratio"`
	DeleteRatio    int           `json:"delete_ratio"`
	ReportInterval time.Duration `json:"report_interval"`
	OutputCSV      string        `json:"output_csv"`
	LogRequests    bool          `json:"log_requests"`
	LogErrors      bool          `json:"log_errors"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() *BenchmarkConfig {
	return &BenchmarkConfig{
		TargetAddress:  "localhost:50051",
		NumConnections: 8,
		NumWorkers:     100,
		Duration:       30 * time.Second,
		WarmupDuration: 5 * time.Second,
		KeySpace:       50000,
		ValueSize:      1024,
		ReadRatio:      70,
		WriteRatio:     25,
		DeleteRatio:    5,
		ReportInterval: 5 * time.Second,
		OutputCSV:      "",
		LogRequests:    false,
		LogErrors:      false,
	}
}

// ParseFlags parses command line flags and returns a config
func ParseFlags() *BenchmarkConfig {
	config := DefaultConfig()

	flag.StringVar(&config.TargetAddress, "target", config.TargetAddress, "gRPC server address")
	flag.IntVar(&config.NumConnections, "connections", config.NumConnections, "Number of gRPC connections")
	flag.IntVar(&config.NumWorkers, "workers", config.NumWorkers, "Number of concurrent workers")
	flag.DurationVar(&config.Duration, "duration", config.Duration, "Benchmark duration")
	flag.DurationVar(&config.WarmupDuration, "warmup", config.WarmupDuration, "Warm-up duration")
	flag.IntVar(&config.KeySpace, "keyspace", config.KeySpace, "Number of unique keys")
	flag.IntVar(&config.ValueSize, "valuesize", config.ValueSize, "Size of values in bytes")
	flag.IntVar(&config.ReadRatio, "read", config.ReadRatio, "Percentage of read operations")
	flag.IntVar(&config.WriteRatio, "write", config.WriteRatio, "Percentage of write operations")
	flag.IntVar(&config.DeleteRatio, "delete", config.DeleteRatio, "Percentage of delete operations")
	flag.DurationVar(&config.ReportInterval, "report-interval", config.ReportInterval, "Report interval")
	flag.StringVar(&config.OutputCSV, "csv", config.OutputCSV, "Output CSV file path")
	flag.BoolVar(&config.LogRequests, "log-requests", config.LogRequests, "Log all requests")
	flag.BoolVar(&config.LogErrors, "log-errors", config.LogErrors, "Log error requests")

	flag.Parse()

	return config
}

// LoadFromFile loads configuration from a JSON file
func LoadFromFile(filename string) (*BenchmarkConfig, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config := DefaultConfig()
	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}

// Validate checks if the configuration is valid
func (c *BenchmarkConfig) Validate() error {
	if c.TargetAddress == "" {
		return fmt.Errorf("target address cannot be empty")
	}
	if c.NumConnections <= 0 {
		return fmt.Errorf("number of connections must be positive")
	}
	if c.NumWorkers <= 0 {
		return fmt.Errorf("number of workers must be positive")
	}
	if c.Duration <= 0 {
		return fmt.Errorf("duration must be positive")
	}
	if c.KeySpace <= 0 {
		return fmt.Errorf("key space must be positive")
	}
	if c.ValueSize <= 0 {
		return fmt.Errorf("value size must be positive")
	}
	if c.ReadRatio < 0 || c.WriteRatio < 0 || c.DeleteRatio < 0 {
		return fmt.Errorf("operation ratios cannot be negative")
	}
	if c.ReadRatio+c.WriteRatio+c.DeleteRatio != 100 {
		return fmt.Errorf("operation ratios must sum to 100")
	}

	return nil
}

// String returns a string representation of the configuration
func (c *BenchmarkConfig) String() string {
	return fmt.Sprintf(
		"Target: %s, Connections: %d, Workers: %d, Duration: %v, "+
			"KeySpace: %d, ValueSize: %d, Read: %d%%, Write: %d%%, Delete: %d%%",
		c.TargetAddress, c.NumConnections, c.NumWorkers, c.Duration,
		c.KeySpace, c.ValueSize, c.ReadRatio, c.WriteRatio, c.DeleteRatio,
	)
}
