// Package dlq provides HTTP endpoints for inspecting and retrying failed
// events stored in the Redis Dead Letter Queue.
//
// These endpoints are internal management APIs, gated by x-api-key auth,
// and are used by operators to:
//   - List DLQ entries (optionally filtered by category prefix)
//   - Inspect the raw payload of a specific DLQ entry
//   - Retry a failed event (republish to Kafka or re-execute law orchestration)
package dlq

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// --- Response types for consistent JSON structure ---

type DLQListResponse struct {
	Items []string `json:"items"`
}

type DLQValueResponse struct {
	Item any `json:"item"`
}

type DLQRetryResponse struct {
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type DLQHandler struct {
	Service *DLQService
}

func NewHandler(service *DLQService) *DLQHandler {
	return &DLQHandler{
		Service: service,
	}
}

// GetDLQS lists DLQ keys from Redis.
//
//	GET /dlq?prefix=dlq:menu:   → keys matching "dlq:menu:*"
//	GET /dlq                    → all keys matching "dlq:*"
//
// The prefix must be one of the allowed DLQ prefixes (dlq:menu:, dlq:log:,
// dlq:space:, dlq:space-log:, dlq:law:) or empty.
func (h *DLQHandler) GetDLQS(c *gin.Context) {
	prefix := c.Query("prefix")
	if !isValidDLQPrefix(prefix) {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid prefix"})
		return
	}

	keys, err := h.Service.GetDLQS(c.Request.Context(), prefix)
	if err != nil {
		slog.Error("failed to fetch DLQ keys", "err", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to fetch DLQ keys"})
		return
	}

	c.JSON(http.StatusOK, DLQListResponse{Items: keys})
}

// GetDLQ returns the raw payload for a single DLQ entry.
//
//	GET /dlq/:key   (key = full Redis key, e.g. "dlq:menu:evt-abc-123")
//
// The raw bytes are unmarshaled to produce pretty JSON in the response.
func (h *DLQHandler) GetDLQ(c *gin.Context) {
	key := c.Param("key")
	if _, err := ParseDLQKey(key); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid key"})
		return
	}

	data, err := h.Service.GetDLQValue(c.Request.Context(), key)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{Error: "DLQ not found"})
		return
	}

	var pretty any
	if err := json.Unmarshal(data, &pretty); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "failed to parse DLQ"})
		return
	}

	c.JSON(http.StatusOK, DLQValueResponse{Item: pretty})
}

// RetryDLQ retries a failed DLQ event.
//
//	POST /dlq/retry/:key
//
// Retry strategy depends on the DLQ category:
//   - menu/log/space/space-log → republish to Kafka + mark SENT + delete DLQ
//   - law                      → re-execute LLM → save → mark SENT + delete DLQ
//
// On success, the DLQ entry is removed from Redis.
func (h *DLQHandler) RetryDLQ(c *gin.Context) {
	key := c.Param("key")
	if _, err := ParseDLQKey(key); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid key"})
		return
	}

	if err := h.Service.RetryDLQ(c.Request.Context(), key); err != nil {
		slog.Error("DLQ retry failed", "key", key, "err", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "retry failed"})
		return
	}

	c.JSON(http.StatusOK, DLQRetryResponse{Message: "DLQ retried and removed from Redis"})
}
