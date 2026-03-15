package dlq

import (
	poller "github.com/HoBom-s/hobom-event-processor/internal/poller"
)

// allowedPrefixes defines the only DLQ key prefixes accepted by the system.
var allowedPrefixes = []string{
	poller.HoBomTodayMenuDLQPrefix,
	poller.HoBomLogDLQPrefix,
	poller.HoBomSpaceDLQPrefix,
	poller.HoBomSpaceLogDLQPrefix,
	poller.HoBomLawDLQPrefix,
}

// isValidDLQPrefix checks whether the prefix is one of the allowed DLQ prefixes
// or empty (which means "all DLQ keys").
func isValidDLQPrefix(prefix string) bool {
	if prefix == "" {
		return true
	}
	for _, p := range allowedPrefixes {
		if prefix == p {
			return true
		}
	}
	return false
}
