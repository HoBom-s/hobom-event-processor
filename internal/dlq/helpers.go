package dlq

import (
	"fmt"
	"strings"

	poller "github.com/HoBom-s/hobom-event-processor/internal/poller"
)

// allowedPrefixes defines the only DLQ key prefixes accepted by the system.
var allowedPrefixes = []string{
	poller.HoBomTodayMenuDLQPrefix,
	poller.HoBomLogDLQPrefix,
}

// inferTopicFromKey maps a DLQ key to its Kafka topic.
// Returns an error if the key has an unrecognized prefix.
func inferTopicFromKey(key string) (string, error) {
	switch {
	case strings.HasPrefix(key, poller.HoBomTodayMenuDLQPrefix):
		return poller.HoBomMessage, nil
	case strings.HasPrefix(key, poller.HoBomLogDLQPrefix):
		return poller.HoBomLog, nil
	default:
		return "", fmt.Errorf("unrecognized DLQ key prefix: %s", key)
	}
}

// isValidDLQKey checks whether the key starts with an allowed prefix.
func isValidDLQKey(key string) bool {
	for _, p := range allowedPrefixes {
		if strings.HasPrefix(key, p) {
			return true
		}
	}
	return false
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

// 전달받은 Parameter에서 `:` 기준으로 문자열을 자른 후, `EventID`를 추출하도록 한다.
// Redis에 저장되는 DLQ Key의 경우 `dlq:[category]:event-id`와 같은 규칙을 따르고 있으므로,
// `:` 로 분류된 맨 마지막 문자열이 `EventID` 이다.
func extractEventIdFromKey(key string) string {
	parts := strings.Split(key, ":")
	return parts[len(parts)-1]
}