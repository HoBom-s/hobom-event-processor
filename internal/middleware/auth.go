package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// APIKeyAuth returns a gin middleware that validates the x-api-key header.
// If apiKey is empty, the middleware is a no-op (local dev compatibility).
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
