package utils

import (
	"strings"
)

// `IsEmptyString` checks if a string is empty after trimming spaces.
func IsEmptyString(s string) bool {
	return len(strings.TrimSpace(s)) == 0
}
