package dlq

import (
	"testing"

	"github.com/HoBom-s/hobom-event-processor/internal/poller"
)

func TestInferTopicFromKey(t *testing.T) {
	tests := []struct {
		key     string
		topic   string
		wantErr bool
	}{
		{poller.HoBomTodayMenuDLQPrefix + "event-1", poller.HoBomMessage, false},
		{poller.HoBomLogDLQPrefix + "event-2", poller.HoBomLog, false},
		{"dlq:unknown:event-3", "", true},
		{"invalid-key", "", true},
		{"", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got, err := inferTopicFromKey(tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("inferTopicFromKey(%q) error = %v, wantErr %v", tt.key, err, tt.wantErr)
				return
			}
			if got != tt.topic {
				t.Errorf("inferTopicFromKey(%q) = %q, want %q", tt.key, got, tt.topic)
			}
		})
	}
}

func TestIsValidDLQKey(t *testing.T) {
	tests := []struct {
		key  string
		want bool
	}{
		{poller.HoBomTodayMenuDLQPrefix + "event-1", true},
		{poller.HoBomLogDLQPrefix + "event-2", true},
		{"dlq:unknown:event-3", false},
		{"invalid-key", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if got := isValidDLQKey(tt.key); got != tt.want {
				t.Errorf("isValidDLQKey(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestIsValidDLQPrefix(t *testing.T) {
	tests := []struct {
		prefix string
		want   bool
	}{
		{"", true},
		{poller.HoBomTodayMenuDLQPrefix, true},
		{poller.HoBomLogDLQPrefix, true},
		{"arbitrary:", false},
		{"dlq:unknown:", false},
	}
	for _, tt := range tests {
		t.Run(tt.prefix, func(t *testing.T) {
			if got := isValidDLQPrefix(tt.prefix); got != tt.want {
				t.Errorf("isValidDLQPrefix(%q) = %v, want %v", tt.prefix, got, tt.want)
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
