# NimbusRun Load Testing

k6-based load tests for the NimbusRun serverless platform.

## Prerequisites

Install k6:
```bash
# macOS
brew install k6

# Windows (Chocolatey)
choco install k6

# Linux (apt)
sudo apt-key adv --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69
echo "deb https://dl.k6.io/deb stable main" | sudo tee /etc/apt/sources.list.d/k6.list
sudo apt-get update && sudo apt-get install k6
```

## Test Scenarios

### 1. Smoke Test (100 requests, 1 VU)
```bash
k6 run --vus 1 --iterations 100 loadtest/smoke.js
```

### 2. Standard Load Test (100/1k/10k stages)
```bash
k6 run loadtest/load.js
```

### 3. Benchmark (configurable RPS)
```bash
# 100 RPS for 60s
RPS=100 DURATION=60s k6 run loadtest/benchmark.js

# 1000 RPS for 60s
RPS=1000 DURATION=60s k6 run loadtest/benchmark.js

# 10000 RPS for 30s
RPS=10000 DURATION=30s k6 run loadtest/benchmark.js
```

## Recorded Results

Results are written to `summary.json` and `benchmark.json` after each run.

## Metrics Collected

- **Latency**: P50, P95, P99, max
- **Throughput**: requests/sec
- **Error rate**: 4xx + 5xx
- **Custom**: queue depth, cold starts (via Prometheus)

## Setting Up the Environment

Before running load tests:

1. Start the infrastructure:
   ```bash
   docker compose up -d postgres redis
   ```

2. Build and deploy a sample function:
   ```bash
   ./nimbus login
   ./nimbus deploy --name hello --zip ./examples/hello.zip
   ```

3. Note the function ID and use it as `FUNCTION_ID`:
   ```bash
   export FUNCTION_ID=<uuid>
   ```

4. Run the load test:
   ```bash
   k6 run loadtest/benchmark.js
   ```
