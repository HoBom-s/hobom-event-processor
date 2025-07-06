package utils

import (
	"testing"
)

func TestIsEmptyString(t *testing.T) {
	tests := []struct {
		input 	string
		want 	bool
	}{
		{"", true},
		{"  ", true},
		{"test", false},
	}

	for _, tt := range tests {
		got := IsEmptyString(tt.input)
		if got != tt.want {
			t.Errorf("IsEmptyString(%q) = %v; want %v", tt.input, got, tt.want)
		}
	}
}

func TestCoalesceString(t *testing.T) {
	got := CoalesceString("", "  ", "first", "second")
	if got != "first" {
		t.Errorf("expected 'first', got %q", got)
	}
}
