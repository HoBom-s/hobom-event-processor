package poller

import "time"

// EventType represents the type of an outbox event.
type EventType string

func (e EventType) String() string { return string(e) }

func (e EventType) Valid() bool {
	switch e {
	case EventTypeHoBomMessage, EventTypeHoBomLog, EventTypeSpaceEvent, EventTypeSpaceLog, EventTypeLawChanged:
		return true
	}
	return false
}

// OutboxStatus represents the processing state of an outbox event.
type OutboxStatus string

func (s OutboxStatus) String() string { return string(s) }

func (s OutboxStatus) Valid() bool {
	switch s {
	case OutboxPending, OutboxSent, OutboxFailed:
		return true
	}
	return false
}

const (
	// EventTypeHoBomMessage is the outbox event type for user-to-user messages.
	EventTypeHoBomMessage EventType = "MESSAGE"
	// EventTypeHoBomLog is the outbox event type for API request/response logs.
	EventTypeHoBomLog EventType = "HOBOM_LOG"
	// EventTypeSpaceEvent is the outbox event type for space document events.
	EventTypeSpaceEvent EventType = "SPACE_EVENT"
	// EventTypeSpaceLog is the outbox event type for space API request logs.
	EventTypeSpaceLog EventType = "SPACE_LOG"
	// EventTypeLawChanged is the outbox event type for privacy law change events.
	EventTypeLawChanged EventType = "LAW_CHANGED"

	// OutboxPending is the initial state of an outbox event awaiting dispatch.
	OutboxPending OutboxStatus = "PENDING"
	// OutboxSent indicates the event was successfully published to Kafka.
	OutboxSent OutboxStatus = "SENT"
	// OutboxFailed indicates the event could not be published after all retries.
	OutboxFailed OutboxStatus = "FAILED"

	// HoBomMessage is the Kafka topic for user-to-user message events.
	HoBomMessage = "hobom.messages"
	// HoBomLog is the Kafka topic for API log events.
	HoBomLog = "hobom.logs"
	// HoBomSpaceEvents is the Kafka topic for space document events.
	HoBomSpaceEvents = "hobom.space-events"

	// Mail identifies an email delivery message type.
	Mail = "MAIL_MESSAGE"
	// Push identifies a push-notification message type.
	Push = "PUSH_MESSAGE"

	// HoBomTodayMenuDLQPrefix is the Redis key prefix for message-event DLQ entries.
	// All DLQ keys must start with "dlq:" for pattern-matching queries.
	HoBomTodayMenuDLQPrefix = "dlq:menu:"
	// HoBomLogDLQPrefix is the Redis key prefix for log-event DLQ entries.
	HoBomLogDLQPrefix = "dlq:log:"
	// HoBomSpaceDLQPrefix is the Redis key prefix for space-event DLQ entries.
	HoBomSpaceDLQPrefix = "dlq:space:"
	// HoBomSpaceLogDLQPrefix is the Redis key prefix for space-log DLQ entries.
	HoBomSpaceLogDLQPrefix = "dlq:space-log:"
	// HoBomLawDLQPrefix is the Redis key prefix for law-event DLQ entries.
	HoBomLawDLQPrefix = "dlq:law:"

	// TTL72Hours is the retention period for DLQ entries.
	TTL72Hours = 72 * time.Hour

	// HoBomEventProcessorInternalApiPrefix is the base path for internal management APIs.
	HoBomEventProcessorInternalApiPrefix = "/hobom-event-processor/internal/api/v1"
)
