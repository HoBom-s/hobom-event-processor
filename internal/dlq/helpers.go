package dlq

import (
	poller "github.com/HoBom-s/hobom-event-processor/internal/poller"
)

// allowedPrefixes defines the only DLQ key prefixes accepted by the
// handler's list endpoint. Used to prevent arbitrary Redis key scanning.
var allowedPrefixes = []string{
	poller.HoBomTodayMenuDLQPrefix,
	poller.HoBomLogDLQPrefix,
	poller.HoBomSpaceDLQPrefix,
	poller.HoBomSpaceLogDLQPrefix,
	poller.HoBomLawDLQPrefix,
}

// isValidDLQPrefix returns true if prefix is one of the allowed DLQ
// prefixes or empty (meaning "list all DLQ keys").
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
