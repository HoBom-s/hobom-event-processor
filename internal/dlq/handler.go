package dlq

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

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

	keys, err := h.Service.GetDLQS(c.Request.Context(), prefix)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch DLQ keys: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": keys})
}

// `GET` /dlq/:key
// Key값에 해당하는 DLQ를 가져오도록 한다.
func (h *DLQHandler) GetDLQ(c *gin.Context) {
	key := c.Param("key")

	data, err := h.Service.GetDLQValue(context.Background(), key)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("DLQ not found: %v", err)})
		return
	}

	var pretty map[string]interface{}
	if err := json.Unmarshal(data, &pretty); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse DLQ JSON"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"item": pretty})
}

// `POST` /dlq/retry/:key
// DLQ를 재발행 하도록 한다.
func (h *DLQHandler) RetryDLQ(c *gin.Context) {
	key := c.Param("key")

	if err := h.Service.RetryDLQ(c.Request.Context(), key); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "DLQ retried and removed from Redis"})
}