package dlq

import (
	"strings"

	poller "github.com/HoBom-s/hobom-event-processor/internal/poller"
)

// DLQ Key를 통해, Kafka Topic을 추출하도록 한다.
// 올바른 Key가 맵핑되지 않을 경우, `unknown-topic`을 반환하도록 한다.
func inferTopicFromKey(key string) string {
	switch {
	case strings.HasPrefix(key, poller.HoBomTodayMenuDLQPrefix):
		return poller.HoBomMessage
	case strings.HasPrefix(key, poller.HoBomLogDLQPrefix):
		return poller.HoBomLog
	default:
		return "unknown-topic"
	}
}

// 전달받은 Parameter에서 `:` 기준으로 문자열을 자른 후, `EventID`를 추출하도록 한다.
// Redis에 저장되는 DLQ Key의 경우 `dlq:[category]:event-id`와 같은 규칙을 따르고 있으므로,
// `:` 로 분류된 맨 마지막 문자열이 `EventID` 이다.
func extractEventIdFromKey(key string) string {
	parts := strings.Split(key, ":")
	return parts[len(parts)-1]
}