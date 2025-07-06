package utils

import (
	"strings"
)

// `IsEmptyString` checks if a string is empty after trimming spaces.
func IsEmptyString(s string) bool {
	return len(strings.TrimSpace(s)) == 0
}

// `CoalesceString` returns the first non-empty string from the parameters.
func CoalesceString(values ...string) string {
	for _, v := range values {
		if !IsEmptyString(v) {
			return v
		}
	}

	return ""
}