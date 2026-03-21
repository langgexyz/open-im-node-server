package gateway_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/langgexyz/open-im-node-server/internal/gateway"
	"github.com/langgexyz/open-im-node-server/internal/token"
)

func TestAuthMiddlewareMissingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	verifier := token.NewVerifier("nodepubkeyplaceholder")
	r.GET("/biz/test", gateway.AuthMiddleware(verifier), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/biz/test", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddlewareInvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	verifier := token.NewVerifier("nodepubkeyplaceholder")
	r.GET("/biz/test", gateway.AuthMiddleware(verifier), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/biz/test", nil)
	req.Header.Set("Authorization", "Bearer notavalidtoken")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
