package gateway

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/langgexyz/open-im-node-server/internal/token"
)

// AuthMiddleware validates the user_token in the Authorization header.
// On success, it injects X-App-UID and X-Node-UID into the downstream request headers.
func AuthMiddleware(verifier *token.Verifier) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

		claims, err := verifier.Verify(tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		c.Request.Header.Set("X-App-UID", claims.AppUID)
		c.Request.Header.Set("X-Node-UID", fmt.Sprintf("%d", claims.NodeUID))
		c.Next()
	}
}
