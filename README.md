# KVStore Benchmarker

A high-performance, configurable benchmark tool for evaluating gRPC-based Key-Value stores backed by SPDK or kernel block devices.

## 🚀 Features

- **High Performance**: Concurrent worker architecture with connection pooling
- **Configurable Workloads**: Tunable operation mix (Get/Put/Delete ratios)
- **Comprehensive Metrics**: Latency percentiles, throughput, error rates
- **Multiple Output Formats**: Console reporting, CSV export, detailed logging
- **Warm-up Support**: Pre-benchmark warm-up phase for accurate measurements
- **Health Checks**: Connection validation before benchmark starts

## 📋 Requirements

- Go 1.21 or later
- Protocol Buffers compiler (`protoc`)
- gRPC Go plugins

## 🛠️ Installation

1. **Install Protocol Buffers compiler:**
   ```bash
   # Ubuntu/Debian
   sudo apt install protobuf-compiler
   
   # macOS
   brew install protobuf
   ```

2. **Install gRPC Go plugins:**
   ```bash
   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
   ```

3. **Build the benchmarker:**
   ```bash
   go mod tidy
   go build -o benchmarker cmd/benchmarker/main.go
   ```

## 🎯 Usage

### Basic Usage

```bash
./benchmarker --target=localhost:50051 --workers=100 --duration=30s
```

### Advanced Usage

```bash
./benchmarker \
  --target=127.0.0.1:50051 \
  --connections=8 \
  --workers=100 \
  --duration=30s \
  --warmup=5s \
  --keyspace=50000 \
  --valuesize=1024 \
  --read=70 --write=25 --delete=5 \
  --log-requests \
  --log-errors \
  --csv=results/benchmark_$(date +%Y%m%d_%H%M%S).csv
```

### Configuration Options

| Option | Default | Description |
|--------|---------|-------------|
| `--target` | `localhost:50051` | gRPC server address |
| `--connections` | `8` | Number of gRPC connections |
| `--workers` | `100` | Number of concurrent workers |
| `--duration` | `30s` | Benchmark duration |
| `--warmup` | `5s` | Warm-up duration |
| `--keyspace` | `50000` | Number of unique keys |
| `--valuesize` | `1024` | Size of values in bytes |
| `--read` | `70` | Percentage of read operations |
| `--write` | `25` | Percentage of write operations |
| `--delete` | `5` | Percentage of delete operations |
| `--report-interval` | `5s` | Progress report interval |
| `--csv` | `` | Output CSV file path |
| `--log-requests` | `false` | Log all requests |
| `--log-errors` | `false` | Log error requests |

## 📊 Output

### Console Output

```
2024/01/15 10:30:00 Benchmark Configuration:
  Target: localhost:50051
  Connections: 8
  Workers: 100
  Duration: 30s
  Warm-up: 5s
  Key Space: 50000
  Value Size: 1024 bytes
  Operation Mix: Read=70%, Write=25%, Delete=5%

2024/01/15 10:30:01 Starting warm-up phase for 5s
2024/01/15 10:30:06 Warm-up phase completed
2024/01/15 10:30:06 Starting benchmark phase for 30s
[10:30:11] Total: 15000 | RPS: 3000 | Avg Latency: 2.5ms | P95: 4.3ms | Errors: 0
[10:30:16] Total: 30200 | RPS: 3020 | Avg Latency: 2.6ms | P95: 4.2ms | Errors: 3

=== FINAL RESULTS ===

Get:
  Count: 21000
  Errors: 0 (0.00%)
  Avg Latency: 2.1ms
  P50 Latency: 1.8ms
  P95 Latency: 3.2ms
  P99 Latency: 4.8ms
  Min Latency: 0.5ms
  Max Latency: 12.3ms

Put:
  Count: 7500
  Errors: 2 (0.03%)
  Avg Latency: 3.2ms
  P50 Latency: 2.9ms
  P95 Latency: 5.1ms
  P99 Latency: 7.2ms
  Min Latency: 1.2ms
  Max Latency: 15.6ms

Delete:
  Count: 1500
  Errors: 1 (0.07%)
  Avg Latency: 2.8ms
  P50 Latency: 2.5ms
  P95 Latency: 4.5ms
  P99 Latency: 6.1ms
  Min Latency: 0.8ms
  Max Latency: 11.2ms

TOTAL:
  Total Operations: 30000
  Total Errors: 3 (0.01%)
  Overall Avg Latency: 2.4ms
```

### CSV Output

If `--csv` is specified, detailed per-request data is written to a CSV file:

```csv
timestamp,method,latency_ms,error
2024-01-15T10:30:06.123456789Z,Get,2.1,
2024-01-15T10:30:06.124567890Z,Put,3.2,
2024-01-15T10:30:06.125678901Z,Delete,2.8,
2024-01-15T10:30:06.126789012Z,Get,1.9,connection refused
```

## 🏗️ Architecture

```
                 +------------------+
                 |  CLI / Config    |
                 +--------+---------+
                          |
                          v
                 +--------+--------+
                 |  BenchmarkRunner |
                 +--------+--------+
                          |
        +-----------------+------------------+
        |                |                  |
        v                v                  v
   +----+----+      +----+----+        +----+----+
   | Worker 1 | ...  | Worker N |  ...  | Collector|
   +----+----+      +----+----+        +----+----+
        |                |                  ^
        v                v                  |
   [gRPC Client]    [gRPC Client]    <--  Results Channel
```

### Components

- **CLI/Config**: Command-line interface and configuration management
- **BenchmarkRunner**: Orchestrates the entire benchmark execution
- **Worker Pool**: Concurrent goroutines performing operations
- **Connection Pool**: Manages multiple gRPC connections
- **Collector**: Aggregates results and generates reports
- **Key Generator**: Generates random keys and values

## 🔧 Development

### Project Structure

```
kvstore-benchmarker/
├── cmd/
│   └── benchmarker/
│       └── main.go           # CLI entrypoint
├── pkg/
│   ├── runner/
│   │   ├── runner.go         # Main benchmark runner
│   │   └── keygen.go         # Key/value generation
│   ├── kvclient/
│   │   └── client.go         # gRPC client wrapper
│   ├── collector/
│   │   └── collector.go      # Result aggregation
│   └── config/
│       └── config.go         # Configuration management
├── internal/
│   └── proto/
│       ├── kvstore.proto     # Protocol buffer definition
│       ├── kvstore.pb.go     # Generated Go code
│       └── kvstore_grpc.pb.go # Generated gRPC code
├── go.mod
├── go.sum
├── README.md
└── benchmark_tool_design.md
```

### Building

```bash
# Generate protobuf code
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       internal/proto/kvstore.proto

# Build the tool
go build -o benchmarker cmd/benchmarker/main.go
```

### Testing

```bash
# Run tests
go test ./...

# Run with race detection
go test -race ./...
```

## 📈 Best Practices

### For SPDK Testing

1. **CPU Affinity**: Pin workers to specific CPU cores
2. **Preload Data**: Always preload keys before benchmark
3. **Warm-up**: Use 5-10 second warm-up phase
4. **Compare Backends**: Test SPDK vs kernel with same parameters
5. **Disable Logging**: Reduce server console logging during tests

### For Accurate Results

1. **Multiple Runs**: Run benchmarks multiple times
2. **Steady State**: Ensure system is in steady state
3. **Resource Monitoring**: Monitor CPU, memory, and I/O
4. **Network Isolation**: Use dedicated network for testing
5. **Baseline Comparison**: Always compare against baseline

## 🚀 Performance Tips

- **Connection Pooling**: Use multiple gRPC connections
- **Key Distribution**: Use random key selection for realistic workloads
- **Value Sizes**: Test with various value sizes (1KB, 4KB, 16KB)
- **Concurrency**: Scale workers based on target system capacity
- **Duration**: Use longer durations for stable measurements

## 🔍 Troubleshooting

### Common Issues

1. **Connection Refused**: Check if gRPC server is running
2. **High Latency**: Verify network connectivity and server performance
3. **Low Throughput**: Increase worker count or check server capacity
4. **Memory Issues**: Reduce key space or worker count

### Debug Mode

Enable detailed logging:

```bash
./benchmarker --target=localhost:50051 --log-requests --log-errors
```

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## 📚 References

- [gRPC Documentation](https://grpc.io/docs/)
- [Protocol Buffers](https://developers.google.com/protocol-buffers)
- [SPDK Documentation](https://spdk.io/doc/)
- [Go Concurrency Patterns](https://golang.org/doc/effective_go.html#concurrency) 