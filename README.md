# hobom-event-processor

This Go-based service is responsible for polling domain outbox events via gRPC and publishing them to Kafka. It also reports the publishing result (success or failure) back to the source service through a separate gRPC call.

---

### 🔁 Event Flow

1. **Poll Outbox Events**  
   Periodically connects to a gRPC server to retrieve outbox events with a specific `eventType` and `status` (e.g., `PENDING`).

2. **Publish to Kafka**  
   For each retrieved outbox event, publishes the event to a predefined Kafka topic. Custom logic per event type can be handled through internal service composition.

3. **Report Result via gRPC**  
   After publishing, it sends a patch/update request back to the gRPC server with either a success or failure result (e.g., update status to `SENT`, `FAILED`, or increment `retryCount`).

---

### ✅ Key Features

- gRPC-based polling and patching
- Kafka event publishing
- Clean, pluggable architecture (Hexagonal/Port & Adapter style)
- Retry logic and failure handling (coming soon)
- Ready for multiple event types and scalable pipelines

---

This service acts as a **stateless, scalable event forwarder**, bridging your domain event store with external consumers via Kafka.
