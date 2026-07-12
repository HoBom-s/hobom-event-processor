package dlq

import (
	"testing"

	"github.com/HoBom-s/hobom-event-processor/internal/poller"
)

func TestIsValidDLQPrefix(t *testing.T) {
	tests := []struct {
		prefix string
		want   bool
	}{
		{"", true},
		{poller.HoBomTodayMenuDLQPrefix, true},
		{poller.HoBomLogDLQPrefix, true},
		{poller.HoBomSpaceDLQPrefix, true},
		{poller.HoBomSpaceLogDLQPrefix, true},
		{poller.HoBomLawDLQPrefix, true},
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
