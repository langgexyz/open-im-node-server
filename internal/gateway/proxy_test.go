package gateway_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/langgexyz/open-im-node-server/internal/gateway"
	"github.com/langgexyz/open-im-node-server/internal/registry"
)

func TestProxyNoRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	reg := registry.New() // empty routing table
	r := gin.New()
	r.Any("/biz/*path", gateway.ProxyHandler(reg))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/biz/articles/publish", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestProxyForwards(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/publish" {
			http.Error(w, "wrong path: "+r.URL.Path, http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	gin.SetMode(gin.TestMode)
	reg := registry.New()
	reg.Set("articles", backend.URL)

	r := gin.New()
	r.Any("/biz/*path", gateway.ProxyHandler(reg))

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/biz/articles/publish", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body: %s", w.Code, w.Body.String())
	}
}
