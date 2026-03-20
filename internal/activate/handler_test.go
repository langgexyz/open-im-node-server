package activate_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/langgexyz/open-im-node-server/internal/activate"
	"github.com/langgexyz/open-im-node-server/internal/config"
)

func TestActivateWrongCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	h := activate.NewHandler(cfg, t.TempDir()+"/config.json", nil)

	r := gin.New()
	r.POST("/node/activate", h.Activate)

	h.SetCode("correctcode12345678901234567890")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/node/activate?code=wrongcode", bytes.NewReader([]byte("garbage")))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestActivateAlreadyActivated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{NodePrivateKey: "already_set"} // already activated
	h := activate.NewHandler(cfg, t.TempDir()+"/config.json", nil)

	r := gin.New()
	r.POST("/node/activate", h.Activate)
	h.SetCode("code1234567890123456789012345678")

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/node/activate?code=code1234567890123456789012345678", bytes.NewReader([]byte{}))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}
