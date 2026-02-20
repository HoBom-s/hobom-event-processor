package dlq

import (
	"testing"

	"github.com/HoBom-s/hobom-event-processor/internal/poller"
)

func TestInferTopicFromKey(t *testing.T) {
	tests := []struct {
		key   string
		topic string
	}{
		{poller.HoBomTodayMenuDLQPrefix + "event-1", poller.HoBomMessage},
		{poller.HoBomLogDLQPrefix + "event-2", poller.HoBomLog},
		{"dlq:unknown:event-3", "unknown-topic"},
		{"invalid-key", "unknown-topic"},
		{"", "unknown-topic"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := inferTopicFromKey(tt.key)
			if got != tt.topic {
				t.Errorf("inferTopicFromKey(%q) = %q, want %q", tt.key, got, tt.topic)
			}
		})
	}
}

func TestExtractEventIdFromKey(t *testing.T) {
	tests := []struct {
		key     string
		eventId string
	}{
		{"dlq:menu:event-123", "event-123"},
		{"dlq:log:some-uuid-here", "some-uuid-here"},
		{"dlq:menu:", ""},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := extractEventIdFromKey(tt.key)
			if got != tt.eventId {
				t.Errorf("extractEventIdFromKey(%q) = %q, want %q", tt.key, got, tt.eventId)
			}
		})
	}
}
