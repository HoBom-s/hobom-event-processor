package dlq

import (
	"fmt"
	"strings"

	poller "github.com/HoBom-s/hobom-event-processor/internal/poller"
)

// DLQCategory represents the category segment of a DLQ key (e.g. "menu", "log").
type DLQCategory string

const (
	DLQCategoryMenu     DLQCategory = "menu"
	DLQCategoryLog      DLQCategory = "log"
	DLQCategorySpace    DLQCategory = "space"
	DLQCategorySpaceLog DLQCategory = "space-log"
	DLQCategoryLaw      DLQCategory = "law"
)

// DLQKey is a parsed DLQ Redis key with format "dlq:<category>:<event-id>".
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
	case DLQCategoryMenu, DLQCategoryLog, DLQCategorySpace, DLQCategorySpaceLog, DLQCategoryLaw:
		return true
	}
	return false
}

// Topic returns the Kafka topic for this DLQ category.
// Law events have no topic (they bypass Kafka).
func (k DLQKey) Topic() (string, error) {
	switch k.Category {
	case DLQCategoryMenu:
		return poller.HoBomMessage, nil
	case DLQCategoryLog, DLQCategorySpaceLog:
		return poller.HoBomLog, nil
	case DLQCategorySpace:
		return poller.HoBomSpaceEvents, nil
	case DLQCategoryLaw:
		return "", nil
	default:
		return "", fmt.Errorf("unrecognized DLQ category: %s", k.Category)
	}
}

// IsSpaceKey returns true if this key belongs to space or space-log events.
func (k DLQKey) IsSpaceKey() bool {
	return k.Category == DLQCategorySpace || k.Category == DLQCategorySpaceLog
}

// IsLawKey returns true if this key belongs to law events.
func (k DLQKey) IsLawKey() bool {
	return k.Category == DLQCategoryLaw
}

// ParseDLQKey parses a raw Redis key string into a DLQKey.
// Expected format: "dlq:<category>:<event-id>"
// For compound categories like "space-log", handles the 4-segment case.
func ParseDLQKey(raw string) (DLQKey, error) {
	if !strings.HasPrefix(raw, "dlq:") {
		return DLQKey{}, fmt.Errorf("invalid DLQ key: must start with 'dlq:': %s", raw)
	}

	// Try compound category "space-log" first (4 segments: dlq:space-log:event-id)
	// Use SplitN to handle event IDs that might contain colons
	rest := raw[len("dlq:"):]
	if strings.HasPrefix(rest, "space-log:") {
		eventID := rest[len("space-log:"):]
		return DLQKey{Category: DLQCategorySpaceLog, EventID: eventID}, nil
	}

	// Simple category: dlq:<category>:<event-id>
	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 {
		return DLQKey{}, fmt.Errorf("invalid DLQ key format: %s", raw)
	}

	return DLQKey{Category: DLQCategory(parts[0]), EventID: parts[1]}, nil
}
