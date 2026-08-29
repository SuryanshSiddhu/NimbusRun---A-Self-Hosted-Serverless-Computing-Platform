# NimbusRun — CLAUDE.md

## Project Overview

NimbusRun is a self-hosted serverless execution engine. You upload a Python function as a ZIP, the platform builds a Docker image, and you invoke it over HTTP — with the platform handling containerized execution, multi-worker scheduling, failure recovery, resource isolation, and observability.

**Technology**: Go backend, Redis Streams (job queue), PostgreSQL (metadata), Docker (isolated execution), Prometheus + Grafana (metrics), React (minimal status dashboard), k6 (load testing).

## Directory Layout

```
CODE/
├── cmd/
│   ├── api-gateway/      # HTTP API server (auth, function CRUD, invocation routing)
│   ├── worker/           # Worker process (polls queue, executes containers)
│   ├── scheduler/        # Scheduler (heartbeat processing, job dispatch)
│   └── cli/              # nimbus CLI tool
├── internal/
│   ├── auth/             # JWT, bcrypt hashing, API key generation
│   ├── build/            # Docker image build pipeline
│   ├── config/           # Viper-based configuration
│   ├── db/               # PostgreSQL connection pool + migrations
│   ├── http/             # Gin HTTP server + middleware + handlers
│   ├── idempotency/      # Redis-backed idempotency key deduplication
│   ├── invocation/       # Invocation service (retry logic, DLQ, result processing)
│   ├── isolation/        # Resource limits (memory, CPU, PID, network), failure tests
│   ├── models/            # Domain models
│   ├── observability/     # Prometheus metrics, Zap structured logging
│   ├── queue/             # Redis Streams queue (enqueue, dequeue, consumer groups)
│   ├── repository/        # PostgreSQL repositories (function, version, invocation)
│   ├── retry/             # Exponential backoff calculator, DLQ service
│   ├── scheduler/         # Load-based worker selection, heartbeat store
│   └── worker/            # Docker container runner with isolation
├── configs/               # Docker configs for Prometheus/Grafana
├── frontend/              # Minimal React status dashboard
├── loadtest/             # k6 load testing scripts
├── migrations/            # PostgreSQL schema (001_init.sql)
├── docker-compose.yml     # Local dev infrastructure
├── go.mod / go.sum       # Go dependencies
└── CLAUDE.md
```

## Build Prerequisites

The following tools must be installed on the machine:

| Tool | Version | Purpose |
|---|---|---|
| Go | 1.21+ | Backend |
| Docker | 24+ | Container isolation, local registry |
| PostgreSQL | 15+ | Metadata store |
| Redis | 7+ | Job queue + idempotency |
| k6 | latest | Load testing |
| Node.js | 18+ | Frontend (optional) |

## Build Commands

```bash
# 1. Start infrastructure
docker compose up -d postgres redis

# 2. Download Go dependencies
go mod download

# 3. Build all binaries
go build ./cmd/api-gateway
go build ./cmd/worker
go build ./cmd/scheduler
go build ./cmd/cli

# 4. Run the API gateway
./api-gateway

# 5. Run workers (in separate terminals)
./worker --name=worker-1
./worker --name=worker-2

# 6. Run the scheduler
./scheduler

# 7. Run CLI
./nimbus login --email you@example.com --password yourpass
./nimbus deploy --name hello --zip ./examples/hello.zip
./nimbus invoke hello --payload '{"name":"World"}'

# 8. Run load tests
k6 run loadtest/benchmark.js
```

## Key Architectural Decisions

1. **Control plane never executes user code** — only workers run containers. This is the core design point for interview questions.

2. **Redis Streams for job queue** — XADD/XREADGROUP for durable, ordered job distribution. Consumer groups allow multiple workers to share the load.

3. **Heartbeat-based worker health** — Workers send heartbeats every 5s. Scheduler marks them UNHEALTHY after 15s of silence and requeues their in-flight jobs.

4. **Load-based scheduling** — `0.5 * cpuUsage + 0.5 * (runningTasks/availableSlots)` — the lowest-scoring healthy worker gets the next job.

5. **Exponential backoff with jitter** — `initialDelay=1s, multiplier=2, maxRetries=3, maxDelay=30s`. Jitter (±30%) prevents thundering herd.

6. **Idempotency via Redis SET NX** — duplicate requests with the same `Idempotency-Key` return the cached result instantly.

7. **Docker isolation per invocation** — memory limit, CPU quota, PIDs limit, no network, read-only rootfs, non-root user, writable /tmp via tmpfs.

## Environment Variables

Prefix all with `NIMBUSRUN_`:

```
NIMBUSRUN_DATABASE_HOST=localhost
NIMBUSRUN_DATABASE_PORT=5432
NIMBUSRUN_DATABASE_USER=nimbusrun
NIMBUSRUN_DATABASE_PASSWORD=nimbusrun
NIMBUSRUN_DATABASE_DBNAME=nimbusrun
NIMBUSRUN_REDIS_ADDR=localhost:6379
NIMBUSRUN_AUTH_JWT_SECRET=change-me-in-production
NIMBUSRUN_DOCKER_REGISTRY=localhost:5000
```

## API Endpoints

| Method | Path | Auth | Description |
|---|---|---|---|
| POST | /auth/register | None | Register a new user |
| POST | /auth/login | None | Login, get JWT + API key |
| GET | /functions | JWT | List user's functions |
| POST | /functions | JWT | Create a function |
| GET | /functions/:id | JWT | Get function details |
| DELETE | /functions/:id | JWT | Delete a function |
| POST | /functions/:id/deploy | JWT | Trigger a new build |
| GET | /functions/:id/versions | JWT | List versions |
| POST | /functions/:id/rollback/:v | JWT | Rollback to version |
| POST | /f/:id | API key | Invoke function |
| GET | /f/:id/invocations/:inv_id/logs | API key | Get invocation logs |
| GET | /health | None | Health check |

## Interview Talking Points

The project is deliberately scoped so you can answer these confidently:

- **Why separate control and data planes?** — The control plane handles metadata and scheduling; workers handle execution. This means the API never runs user code, and workers can be scaled independently.
- **What happens when a worker dies?** — Scheduler misses heartbeats after 15s, marks worker UNHEALTHY, and requeues in-flight jobs. Workers ACKs jobs after completion, so unACKed jobs are re-delivered.
- **How is duplicate execution avoided?** — Idempotency-Key checked via Redis SET NX; if already processed, cached result is returned instantly.
- **How does container isolation work?** — Memory limit, CPU quota, PIDs limit, no network, read-only rootfs, non-root user, tmpfs for /tmp.
- **What breaks first at 10x load?** — Scheduler single-threaded dispatch loop becomes the bottleneck. Redis Streams can handle ~50k ops/s; PostgreSQL metadata queries are the second bottleneck.
- **What if Redis goes down?** — API returns 503 on new invocations. In-flight jobs complete. Worker heartbeats stop. Scheduler marks all workers unhealthy. System degrades gracefully.

## Common Issues

- **"connection refused" on localhost:5000** — Run `docker compose up -d` to start the local Docker registry.
- **"no healthy workers"** — Start at least one worker before invoking. Workers register via heartbeat.
- **Build fails with "python: command not found"** — The function's Dockerfile uses `python:3.12-slim`. Ensure the function ZIP contains a `requirements.txt`.
- **k6 test shows high error rate** — Check worker logs for container OOMs or timeout issues.
