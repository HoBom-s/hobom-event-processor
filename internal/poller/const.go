package poller

import "time"

// EventType represents the type of an outbox event.
// Each backend service writes events with a specific EventType, and the
// corresponding poller filters by that type when fetching PENDING events.
//
// Mapping:
//
//	EventType         → Poller           → Target
//	MESSAGE           → MessagePoller    → Kafka (hobom.messages)
//	HOBOM_LOG         → LogPoller        → Kafka (hobom.logs)
//	SPACE_EVENT       → SpacePoller      → Kafka (hobom.space-events)
//	SPACE_LOG         → SpaceLogPoller   → Kafka (hobom.logs)
//	LAW_CHANGED       → LawPoller        → LLM gRPC → DB (no Kafka)
type EventType string

func (e EventType) String() string { return string(e) }

func (e EventType) Valid() bool {
	switch e {
	case EventTypeHoBomMessage, EventTypeHoBomLog, EventTypeSpaceEvent, EventTypeSpaceLog, EventTypeLawChanged,
		EventTypeAngelAdoptionApproved, EventTypeAngelFosterApproved, EventTypeAngelStaffPromotionApproved,
		EventTypeAngelShelterVerificationApproved, EventTypeAngelFosterTerminated:
		return true
	}
	return false
}

// OutboxStatus represents the processing state of an outbox event.
// State machine: PENDING → SENT (success) or PENDING → FAILED (error).
// Once SENT or FAILED, the event is not polled again.
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
	EventTypeHoBomMessage EventType = "MESSAGE"
	EventTypeHoBomLog     EventType = "HOBOM_LOG"
	EventTypeSpaceEvent   EventType = "SPACE_EVENT"
	EventTypeSpaceLog     EventType = "SPACE_LOG"
	EventTypeLawChanged   EventType = "LAW_CHANGED"

	// Angel notification events (for-hobom-angel-backend). Each is polled
	// separately and published to hobom.angel-events. Angel access logs reuse
	// EventTypeHoBomLog and are drained to hobom.logs by AngelLogPoller.
	EventTypeAngelAdoptionApproved            EventType = "ADOPTION_APPROVED"
	EventTypeAngelFosterApproved              EventType = "FOSTER_APPROVED"
	EventTypeAngelStaffPromotionApproved      EventType = "STAFF_PROMOTION_APPROVED"
	EventTypeAngelShelterVerificationApproved EventType = "SHELTER_VERIFICATION_APPROVED"
	EventTypeAngelFosterTerminated            EventType = "FOSTER_TERMINATED"

	OutboxPending OutboxStatus = "PENDING"
	OutboxSent    OutboxStatus = "SENT"
	OutboxFailed  OutboxStatus = "FAILED"

	// Kafka topics — each poller publishes to a specific topic.
	// Consumers downstream (hobom-internal-backend, etc.) subscribe to these.
	HoBomMessage     = "hobom.messages"
	HoBomLog         = "hobom.logs"
	HoBomSpaceEvents = "hobom.space-events"
	HoBomAngelEvents = "hobom.angel-events"

	// Message delivery types used in DeliverHoBomMessageCommand.
	Mail = "MAIL_MESSAGE"
	Push = "PUSH_MESSAGE"

	// DLQ key prefixes — each poller uses a distinct prefix so DLQ entries
	// can be filtered and retried per category.
	// Format: "dlq:<category>:<event-id>", e.g. "dlq:menu:evt-abc-123"
	HoBomTodayMenuDLQPrefix = "dlq:menu:"
	HoBomLogDLQPrefix       = "dlq:log:"
	HoBomSpaceDLQPrefix     = "dlq:space:"
	HoBomSpaceLogDLQPrefix  = "dlq:space-log:"
	HoBomLawDLQPrefix       = "dlq:law:"
	HoBomAngelDLQPrefix     = "dlq:angel:"
	HoBomAngelLogDLQPrefix  = "dlq:angel-log:"

	// TTL72Hours is the Redis TTL for DLQ entries. After 72h, unretried
	// entries expire automatically to prevent unbounded storage growth.
	TTL72Hours = 72 * time.Hour

	// HoBomEventProcessorInternalApiPrefix is the base path for DLQ
	// management endpoints, scoped under an internal namespace.
	HoBomEventProcessorInternalApiPrefix = "/hobom-event-processor/internal/api/v1"
)
