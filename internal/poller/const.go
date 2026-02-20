package poller

import "time"

const (
	// EventTypeHoBomMessage is the outbox event type for user-to-user messages.
	EventTypeHoBomMessage = "MESSAGE"
	// EventTypeHoBomLog is the outbox event type for API request/response logs.
	EventTypeHoBomLog = "HOBOM_LOG"

	// OutboxPending is the initial state of an outbox event awaiting dispatch.
	OutboxPending = "PENDING"
	// OutboxSent indicates the event was successfully published to Kafka.
	OutboxSent = "SENT"
	// OutboxFailed indicates the event could not be published after all retries.
	OutboxFailed = "FAILED"

	// HoBomMessage is the Kafka topic for user-to-user message events.
	HoBomMessage = "hobom.messages"
	// HoBomLog is the Kafka topic for API log events.
	HoBomLog = "hobom.logs"

	// Mail identifies an email delivery message type.
	Mail = "MAIL_MESSAGE"
	// Push identifies a push-notification message type.
	Push = "PUSH_MESSAGE"

	// HoBomTodayMenuDLQPrefix is the Redis key prefix for message-event DLQ entries.
	// All DLQ keys must start with "dlq:" for pattern-matching queries.
	HoBomTodayMenuDLQPrefix = "dlq:menu:"
	// HoBomLogDLQPrefix is the Redis key prefix for log-event DLQ entries.
	HoBomLogDLQPrefix = "dlq:log:"

	// TTL72Hours is the retention period for DLQ entries.
	TTL72Hours = 72 * time.Hour

	// HoBomEventProcessorInternalApiPrefix is the base path for internal management APIs.
	HoBomEventProcessorInternalApiPrefix = "/hobom-event-processor/internal/api/v1"
)
