package dlq

import (
	poller "github.com/HoBom-s/hobom-event-processor/internal/poller"
)

func inferTopicFromKey(key string) string {
	switch {
	case hasPrefix(key, poller.HoBomTodayMenuDLQPrefix):
		return poller.HoBomMessage

	case hasPrefix(key, poller.HoBomLogDLQPrefix):
		return poller.HoBomLog
		
	default:
		return "unknown-topic"
	}
}

func hasPrefix(s string, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}