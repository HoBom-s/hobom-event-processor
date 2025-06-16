package utils

import (
	"testing"
	"time"
)

func TestIsZeroTime(t *testing.T) {
	now := NowUTC()
	if IsZeroTime(now) {
		t.Errorf("expected NowUTC() to not be zero")
	}

	var zeroTime time.Time
	if !IsZeroTime(zeroTime) {
		t.Errorf("expected zero time to be detected")
	}
}