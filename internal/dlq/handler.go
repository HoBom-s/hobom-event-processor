package dlq

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

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

// `GET` /dlq
// Redis에 저장된 DLQ 키 목록을 가져온다.
// prefix가 빈 문자열("") 이라면 모든 DLQ를 조회하도록 한다.
// ex) ?prefix=dlq:menu: 또는 ?prefix=dlq:log:
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

// `GET` /dlq/:key
// Key값에 해당하는 DLQ를 가져오도록 한다.
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

// `POST` /dlq/retry/:key
// DLQ를 재발행 하도록 한다.
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
