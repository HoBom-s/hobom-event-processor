# hobom-event-processor

A Go service that polls domain outbox events via gRPC and forwards them to Kafka.
Failed events are stored in a Redis-backed Dead Letter Queue (DLQ) and can be replayed via a management API.

---

## Architecture

```
┌──────────────────────────────┐  ┌──────────────────────────────┐
│  for-hobom-backend (gRPC)    │  │  hobom-space-backend (gRPC)  │
│  Outbox: PENDING → SENT      │  │  Outbox: PENDING → SENT      │
└──────────────┬───────────────┘  └──────────────┬───────────────┘
               │ gRPC poll (5s)                   │ gRPC poll (5s)
    ┌──────────▼──────────┐            ┌──────────▼──────────┐
    │  MessagePoller      │            │  SpacePoller         │
    │  LogPoller          │            │  EVENT_TYPE=SPACE    │
    └──────────┬──────────┘            └──────────┬──────────┘
               │                                  │
               └──────────────┬───────────────────┘
                    ┌─────────▼─────────┐
                    │  publishWithRetry  │  3 attempts, exponential backoff
                    └─────────┬──────┬──┘
                              │      │ on failure
                     ┌────────▼─┐  ┌─▼──────────────────────────┐
                     │  Kafka   │  │  Redis DLQ (TTL: 72h)       │
                     │ Topics   │  │  Key: dlq:[category]:[id]   │
                     └──────────┘  └─────────────────────────────┘
                                              │
                                   ┌──────────▼──────────────────┐
                                   │  DLQ Management API (Gin)    │
                                   │  GET  /dlq                   │
                                   │  GET  /dlq/:key              │
                                   │  POST /dlq/retry/:key        │
                                   └─────────────────────────────┘
```

---

## Event Types

| Event Type    | Kafka Topic          | DLQ Prefix   | gRPC Source         | Description                      |
| ------------- | -------------------- | ------------ | ------------------- | -------------------------------- |
| `MESSAGE`     | `hobom.messages`     | `dlq:menu:`  | for-hobom-backend   | User-to-user message delivery    |
| `HOBOM_LOG`   | `hobom.logs`         | `dlq:log:`   | for-hobom-backend   | API request/response log batches |
| `SPACE_EVENT` | `hobom.space-events` | `dlq:space:` | hobom-space-backend | Space document events            |

---

## Retry & Error Handling

1. **Polling**: every 5 seconds via gRPC, fetches all `PENDING` outbox events.
2. **Publish with retry**: up to 3 attempts with exponential backoff (200ms → 400ms).
3. **On success**: marks the outbox record as `SENT` via gRPC.
4. **On failure**: marks as `FAILED` via gRPC, stores payload in Redis DLQ (72h TTL).
5. **DLQ replay**: call `POST /dlq/retry/:key` to re-publish and remove from DLQ.

Log events are published as a single JSON array per poll cycle for efficiency. DLQ entries for log events store individual payloads as single-element arrays to ensure consistent format on retry.

---

## DLQ Management API

Base path: `/hobom-event-processor/internal/api/v1`

### List DLQ entries

```sh
# All entries
curl http://localhost:8082/hobom-event-processor/internal/api/v1/dlq

# Filter by prefix
curl "http://localhost:8082/hobom-event-processor/internal/api/v1/dlq?prefix=dlq:log:"
```

### Inspect a DLQ entry

```sh
curl http://localhost:8082/hobom-event-processor/internal/api/v1/dlq/dlq:menu:event-abc
```

### Replay a DLQ entry

```sh
curl -X POST http://localhost:8082/hobom-event-processor/internal/api/v1/dlq/retry/dlq:menu:event-abc
```

### Health check

```sh
curl http://localhost:8082/health
# {"status":"ok","statusCode":200,"message":"Service is healthy"}
```

---

## Configuration

All configuration is managed via environment variables. Locally, create a `.env` file in the project root (already in `.gitignore`).

| Variable                   | Required | Default                 | Description                                                                  |
| -------------------------- | -------- | ----------------------- | ---------------------------------------------------------------------------- |
| `HOBOM_GRPC_ADDR`          | Yes      | -                       | for-hobom-backend gRPC address (e.g. `dev-for-hobom-backend:50051`)          |
| `HOBOM_GRPC_API_KEY`       | Yes      | -                       | API key for gRPC authentication                                              |
| `HOBOM_KAFKA_BROKER`       | Yes      | -                       | Kafka broker address (e.g. `kafka:9092`)                                     |
| `HOBOM_REDIS_ADDR`         | Yes      | -                       | Redis address (e.g. `redis:6379`)                                            |
| `HOBOM_SPACE_GRPC_ADDR`    | No       | -                       | hobom-space-backend gRPC address. SpacePoller only runs when this is set     |
| `HOBOM_SPACE_GRPC_API_KEY` | No       | `HOBOM_GRPC_API_KEY`    | Dedicated API key for space gRPC. Falls back to the default API key if unset |
| `HOBOM_HTTP_ADDR`          | No       | `:8082`                 | HTTP server listen address                                                   |

Kafka publisher defaults (via `DefaultKafkaConfig`): `RequireOne` acks, `LeastBytes` balancer, 10s write timeout.

**Production (Docker)**: `.env` is loaded from the deploy server via `--env-file` (e.g. `/etc/hobom-dev/dev-hobom-event-processor/.env`).

---

## Running locally

```sh
# 1. Start infrastructure
docker compose -f infra/kafka/docker-compose.yml up -d
docker compose -f infra/redis/docker-compose.yml up -d

# 2. Create .env
cp .env.example .env   # edit values as needed

# 3. Generate protobuf code and run
make run
```

---

## Development

```sh
# Generate proto files
make proto

# Run tests
go test ./...

# Sync protobuf submodule
make sync-submodule
```

---

## Graceful Shutdown

On `SIGTERM` / `SIGINT`:

1. Context is cancelled — pollers finish their current poll cycle before stopping.
2. In-flight poll results are waited on via `sync.WaitGroup`.
3. HTTP server shuts down with a 5s timeout.
