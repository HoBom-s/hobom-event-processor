package dlq

import (
	"fmt"
	"strings"

	poller "github.com/HoBom-s/hobom-event-processor/internal/poller"
)

// DLQCategory represents the category segment of a DLQ key.
// It identifies which poller originally produced the failed event.
//
// Category → Poller origin:
//
//	menu      → MessagePoller (for-hobom-backend)
//	log       → LogPoller (for-hobom-backend)
//	space     → SpacePoller (hobom-space-backend)
//	space-log → SpaceLogPoller (hobom-space-backend)
//	law       → LawPoller (for-hobom-backend + LLM)
type DLQCategory string

const (
	DLQCategoryMenu     DLQCategory = "menu"
	DLQCategoryLog      DLQCategory = "log"
	DLQCategorySpace    DLQCategory = "space"
	DLQCategorySpaceLog DLQCategory = "space-log"
	DLQCategoryLaw      DLQCategory = "law"
	DLQCategoryAngel    DLQCategory = "angel"
	DLQCategoryAngelLog DLQCategory = "angel-log"
)

// DLQKey is a parsed DLQ Redis key.
//
// Redis key format: "dlq:<category>:<event-id>"
// Examples:
//   - "dlq:menu:evt-abc-123"
//   - "dlq:space-log:evt-xyz-456"
//   - "dlq:law:evt-789"
type DLQKey struct {
	Category DLQCategory
	EventID  string
}

// String returns the full Redis key representation.
func (k DLQKey) String() string {
	return fmt.Sprintf("dlq:%s:%s", k.Category, k.EventID)
}

// Valid returns true if the key has a recognized category and non-empty event ID.
func (k DLQKey) Valid() bool {
	if k.EventID == "" {
		return false
	}
	switch k.Category {
	case DLQCategoryMenu, DLQCategoryLog, DLQCategorySpace, DLQCategorySpaceLog, DLQCategoryLaw,
		DLQCategoryAngel, DLQCategoryAngelLog:
		return true
	}
	return false
}

// Topic returns the Kafka topic for this DLQ category.
// Used during retry to republish the event to the correct topic.
// Law events return an empty topic because they bypass Kafka entirely
// (retry re-executes the LLM → save orchestration instead).
func (k DLQKey) Topic() (string, error) {
	switch k.Category {
	case DLQCategoryMenu:
		return poller.HoBomMessage, nil
	case DLQCategoryLog, DLQCategorySpaceLog, DLQCategoryAngelLog:
		return poller.HoBomLog, nil
	case DLQCategorySpace:
		return poller.HoBomSpaceEvents, nil
	case DLQCategoryAngel:
		return poller.HoBomAngelEvents, nil
	case DLQCategoryLaw:
		return "", nil
	default:
		return "", fmt.Errorf("unrecognized DLQ category: %s", k.Category)
	}
}

// IsSpaceKey returns true if this key belongs to space or space-log events.
// Space events require the hobom-space-backend gRPC connection for marking.
func (k DLQKey) IsSpaceKey() bool {
	return k.Category == DLQCategorySpace || k.Category == DLQCategorySpaceLog
}

// IsLawKey returns true if this key belongs to law events.
// Law events follow a completely different retry path (LLM → save → mark)
// instead of Kafka republish.
func (k DLQKey) IsLawKey() bool {
	return k.Category == DLQCategoryLaw
}

// IsAngelKey returns true if this key belongs to angel or angel-log events.
// Angel events require the for-hobom-angel-backend gRPC connection for marking.
func (k DLQKey) IsAngelKey() bool {
	return k.Category == DLQCategoryAngel || k.Category == DLQCategoryAngelLog
}

// ParseDLQKey parses a raw Redis key string into a DLQKey.
//
// Expected format: "dlq:<category>:<event-id>"
// The compound category "space-log" is handled specially because it contains
// a hyphen that would otherwise be ambiguous with simple SplitN parsing.
// Event IDs may contain colons (e.g. UUIDs with custom formats).
func ParseDLQKey(raw string) (DLQKey, error) {
	if !strings.HasPrefix(raw, "dlq:") {
		return DLQKey{}, fmt.Errorf("invalid DLQ key: must start with 'dlq:': %s", raw)
	}

	rest := raw[len("dlq:"):]

	// Check compound categories (hyphenated) first — a plain SplitN would
	// otherwise break "space-log"/"angel-log" at the hyphen's colon boundary.
	if strings.HasPrefix(rest, "space-log:") {
		eventID := rest[len("space-log:"):]
		return DLQKey{Category: DLQCategorySpaceLog, EventID: eventID}, nil
	}
	if strings.HasPrefix(rest, "angel-log:") {
		eventID := rest[len("angel-log:"):]
		return DLQKey{Category: DLQCategoryAngelLog, EventID: eventID}, nil
	}

	// Simple category: split on first colon only to preserve colons in event IDs.
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 {
		return DLQKey{}, fmt.Errorf("invalid DLQ key format: %s", raw)
	}

	return DLQKey{Category: DLQCategory(parts[0]), EventID: parts[1]}, nil
}
