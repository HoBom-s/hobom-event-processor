package utils

import (
	"time"
)

// `NowUTC` returns current UTC time.
func NowUTC() time.Time {
	return time.Now().UTC()
}

// `IsZeroTime` checks if the time is zero.
func IsZeroTime(t time.Time) bool {
	return t.IsZero()
}