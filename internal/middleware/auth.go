package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// APIKeyAuth returns a gin middleware that validates the x-api-key header
// against the expected apiKey.
//
// Behavior:
//   - apiKey == "" → no-op (all requests pass). This supports local
//     development without configuring HOBOM_INTERNAL_API_KEY.
//   - apiKey != "" → requests without a matching x-api-key header receive
//     401 Unauthorized.
//
// Used to protect internal DLQ management endpoints from unauthorized access.
func APIKeyAuth(apiKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if apiKey == "" {
			c.Next()
			return
		}
		if c.GetHeader("x-api-key") != apiKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}
