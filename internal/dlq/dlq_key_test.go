package dlq

import (
	"testing"

	"github.com/HoBom-s/hobom-event-processor/internal/poller"
)

func TestParseDLQKey_ValidKeys(t *testing.T) {
	tests := []struct {
		raw      string
		category DLQCategory
		eventID  string
	}{
		{"dlq:menu:event-123", DLQCategoryMenu, "event-123"},
		{"dlq:log:event-456", DLQCategoryLog, "event-456"},
		{"dlq:space:event-789", DLQCategorySpace, "event-789"},
		{"dlq:space-log:event-abc", DLQCategorySpaceLog, "event-abc"},
		{"dlq:angel:event-ghi", DLQCategoryAngel, "event-ghi"},
		{"dlq:angel-log:event-jkl", DLQCategoryAngelLog, "event-jkl"},
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			k, err := ParseDLQKey(tt.raw)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if k.Category != tt.category {
				t.Errorf("category = %q, want %q", k.Category, tt.category)
			}
			if k.EventID != tt.eventID {
				t.Errorf("eventID = %q, want %q", k.EventID, tt.eventID)
			}
		})
	}
}

func TestParseDLQKey_InvalidKeys(t *testing.T) {
	tests := []string{
		"",
		"invalid",
		"notdlq:menu:event",
		"dlq:",
		"dlq:menu:", // empty event ID (parsed but invalid via Valid())
	}

	for _, raw := range tests {
		t.Run(raw, func(t *testing.T) {
			k, err := ParseDLQKey(raw)
			if err == nil && k.Valid() {
				t.Errorf("expected parse error or invalid key for %q, got valid key %+v", raw, k)
			}
		})
	}
}

func TestDLQKey_String(t *testing.T) {
	k := DLQKey{Category: DLQCategoryMenu, EventID: "event-123"}
	if got := k.String(); got != "dlq:menu:event-123" {
		t.Errorf("String() = %q, want %q", got, "dlq:menu:event-123")
	}
}

func TestDLQKey_Valid(t *testing.T) {
	tests := []struct {
		key  DLQKey
		want bool
	}{
		{DLQKey{DLQCategoryMenu, "event-1"}, true},
		{DLQKey{DLQCategoryLog, "event-2"}, true},
		{DLQKey{DLQCategorySpace, "event-3"}, true},
		{DLQKey{DLQCategorySpaceLog, "event-4"}, true},
		{DLQKey{DLQCategoryAngel, "event-6"}, true},
		{DLQKey{DLQCategoryAngelLog, "event-7"}, true},
		{DLQKey{DLQCategory("unknown"), "event-8"}, false},
		{DLQKey{DLQCategoryMenu, ""}, false},
	}

	for _, tt := range tests {
		t.Run(tt.key.String(), func(t *testing.T) {
			if got := tt.key.Valid(); got != tt.want {
				t.Errorf("Valid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDLQKey_Topic(t *testing.T) {
	tests := []struct {
		category DLQCategory
		topic    string
	}{
		{DLQCategoryMenu, poller.HoBomMessage},
		{DLQCategoryLog, poller.HoBomLog},
		{DLQCategorySpaceLog, poller.HoBomLog},
		{DLQCategorySpace, poller.HoBomSpaceEvents},
		{DLQCategoryAngel, poller.HoBomAngelEvents},
		{DLQCategoryAngelLog, poller.HoBomLog},
	}

	for _, tt := range tests {
		t.Run(string(tt.category), func(t *testing.T) {
			k := DLQKey{Category: tt.category, EventID: "test"}
			topic, err := k.Topic()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if topic != tt.topic {
				t.Errorf("Topic() = %q, want %q", topic, tt.topic)
			}
		})
	}
}

func TestDLQKey_IsSpaceKey(t *testing.T) {
	if !(&DLQKey{DLQCategorySpace, "e"}).IsSpaceKey() {
		t.Error("space should be space key")
	}
	if !(&DLQKey{DLQCategorySpaceLog, "e"}).IsSpaceKey() {
		t.Error("space-log should be space key")
	}
	if (&DLQKey{DLQCategoryMenu, "e"}).IsSpaceKey() {
		t.Error("menu should not be space key")
	}
}

