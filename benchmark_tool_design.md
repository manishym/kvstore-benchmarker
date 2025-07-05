# Go Benchmark Tool for gRPC Key-Value Store

## 📌 Objective

Design a high-performance, configurable, and extensible benchmark tool to evaluate the performance of a gRPC-based Key-Value store backed by SPDK or kernel block device. The tool will simulate concurrent client workloads with tunable operation mix and measure latency, throughput, and error rates.

---

## 🧱 Architecture Overview

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

---

## ⚙️ Configuration

All benchmark parameters are controlled via the `BenchmarkConfig` struct or via CLI flags / JSON.

### `BenchmarkConfig`

```go
type BenchmarkConfig struct {
    TargetAddress   string        // gRPC server (e.g., "localhost:50051")
    NumConnections  int           // Number of gRPC connections
    NumWorkers      int           // Total concurrent goroutines
    Duration        time.Duration // Benchmark duration
    WarmupDuration  time.Duration // Optional warm-up before measurement
    KeySpace        int           // Unique keys to use
    ValueSize       int           // Size of values in bytes
    ReadRatio       int           // % of Get requests
    WriteRatio      int           // % of Put requests
    DeleteRatio     int           // % of Delete requests
    ReportInterval  time.Duration // Intermediate stats print interval
    OutputCSV       string        // Path to save final results
    LogRequests     bool          // Whether to log all requests
    LogErrors       bool          // Whether to log only failed requests
}
```

---

## 🔌 Connections

- Create a pool of persistent `grpc.ClientConn` (length = `NumConnections`)
- Each worker selects a connection (round-robin or random) to spread load

---

## 🚀 Workers

- Spawn `NumWorkers` goroutines
- Each worker:
  - Picks an operation (`Get`, `Put`, `Delete`) based on configured ratio
  - Picks a key from a pre-generated pool of `KeySpace` entries
  - Generates a value (for `Put`)
  - Issues gRPC request
  - Measures latency
  - Reports result to collector

---

## 🧠 Key/Value Handling

- Pre-generate `KeySpace` keys
- Use base64/random 8–16 byte strings for keys
- For `Put`, generate `ValueSize` byte values per request

---

## 📊 Result Reporting

### `BenchmarkResult`

```go
type BenchmarkResult struct {
    Method    string
    LatencyMs float64
    Error     error
    Timestamp time.Time
}
```

- All results sent to a buffered `results` channel
- Aggregator/Collector computes:
  - Ops/sec (RPS)
  - Average, P50, P95, P99 latency
  - Error counts
  - Writes to CSV if configured
  - Logs requests/errors if enabled in config

---

## 📄 Code Structure

```
cmd/
└── benchmarker/
    └── main.go           → CLI entrypoint
pkg/
├── runner/
│   ├── runner.go         → Runs workers, manages connections
│   └── keygen.go         → Key/value generator utilities
├── kvclient/
│   └── client.go         → Wrapper over gRPC kvstore client
├── collector/
│   └── collector.go      → Result aggregator and reporter
└── config/
    └── config.go         → Config parsing (JSON or flags)

internal/
└── proto/
    └── kvstore.pb.go     → gRPC client interface
```

---

## 📈 Metrics Collected

| Metric             | Description                              |
| ------------------ | ---------------------------------------- |
| Ops/sec (RPS)      | Throughput per method & overall          |
| Latency            | P50, P95, P99, max, avg (ms)             |
| Error Rate         | % of failed requests                     |
| Success Count      | Total completed requests                 |
| CSV Output         | Per-request latency + timestamp          |
| Request/Error Logs | Raw gRPC request or failure (if enabled) |

---

## 🧪 Sample Output (Console)

```
[5s] Total: 15,000 | RPS: 3,000 | Avg Latency: 2.5ms | P95: 4.3ms | Errors: 0
[10s] Total: 30,200 | RPS: 3,020 | Avg Latency: 2.6ms | P95: 4.2ms | Errors: 3
```

---

## 🧠 Extensibility Ideas

- Support **TLS** and **auth token**
- Add **histogram visualizer**
- Export to **Prometheus**
- Support **multi-host** testing
- **Ramp-up phase** before measuring
- **Multiple key spaces / prefixes** for simulating multi-tenant behavior

---

## 🚡 Best Practices

- Pin CPU affinity for SPDK tests
- Always preload keys before benchmark
- Disable console logging from server to reduce noise
- Warm-up the system for 5–10 seconds before actual measurement
- Always compare SPDK vs kernel with same thread count & hardware

---

## 🏁 Example Command

```bash
go run main.go \
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
  --csv=results/spdk_2025-07-02.csv
```

---

## ✅ Milestones

1. ✅ Define config and CLI
2. ✅ Implement connection pool and clients
3. ✅ Worker logic for random ops
4. ✅ Latency tracking and collector
5. ✅ CSV & live console reporting
6. ✅ Run with both SPDK and kernel backends

