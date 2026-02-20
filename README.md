# hobom-event-processor

A Go service that polls domain outbox events via gRPC and forwards them to Kafka.
Failed events are stored in a Redis-backed Dead Letter Queue (DLQ) and can be replayed via a management API.

---

## Architecture

```
┌──────────────────────────────────────────────────┐
│              hobom-backend (gRPC Server)          │
│  Outbox Table: PENDING → SENT / FAILED            │
└───────────────────┬──────────────────────────────┘
                    │ gRPC poll (every 5s)
        ┌───────────▼────────────┐
        │   MessagePoller        │  EVENT_TYPE=MESSAGE
        │   LogPoller            │  EVENT_TYPE=HOBOM_LOG
        └───────────┬────────────┘
                    │
          ┌─────────▼─────────┐
          │  publishWithRetry  │  up to 3 attempts, exponential backoff
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

| Event Type   | Kafka Topic      | DLQ Prefix   | Description                      |
|-------------|-----------------|-------------|----------------------------------|
| `MESSAGE`   | `hobom.messages` | `dlq:menu:` | User-to-user message delivery    |
| `HOBOM_LOG` | `hobom.logs`     | `dlq:log:`  | API request/response log batches |

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

All endpoints are currently hardcoded. Set them in `cmd/main.go`:

| Service       | Default                          |
|--------------|----------------------------------|
| gRPC backend | `dev-for-hobom-backend:50051`    |
| Kafka broker | `kafka:9092`                     |
| Redis        | `redis:6379`                     |
| HTTP server  | `:8082`                          |

Kafka publisher defaults (via `DefaultKafkaConfig`): `RequireOne` acks, `LeastBytes` balancer, 10s write timeout.

---

## Running locally

```sh
# 1. Start infrastructure
docker compose -f infra/kafka/docker-compose.yml up -d
docker compose -f infra/redis/docker-compose.yml up -d

# 2. Generate protobuf code and run
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
