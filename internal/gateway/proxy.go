package gateway

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/langgexyz/open-im-node-server/internal/registry"
)

// safeWriter wraps an http.ResponseWriter and intentionally does NOT implement
// http.CloseNotifier, preventing a panic in httputil.ReverseProxy when the
// underlying writer (e.g. httptest.ResponseRecorder via gin) doesn't support it.
type safeWriter struct {
	http.ResponseWriter
}

// ProxyHandler reverse-proxies /biz/<service>/* requests to the registered backend.
// The gin route must be registered as /biz/*path so that c.Param("path") yields the full suffix.
func ProxyHandler(reg *registry.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		// path looks like "/articles/publish"
		path := c.Param("path")
		parts := strings.SplitN(strings.TrimPrefix(path, "/"), "/", 2)
		serviceName := parts[0]

		if serviceName == "" {
			c.JSON(http.StatusNotFound, gin.H{"error": "missing service name"})
			return
		}

		backend, ok := reg.Get(serviceName)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("service %q not registered", serviceName)})
			return
		}

		target, err := url.Parse(backend)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid backend address"})
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(target)
		// Wrap the writer to strip the CloseNotifier interface.
		// gin's responseWriter panics on CloseNotify() when the underlying writer
		// (e.g. httptest.ResponseRecorder) does not implement the deprecated
		// http.CloseNotifier; wrapping here prevents that type assertion.
		proxy.ServeHTTP(&safeWriter{c.Writer}, c.Request)
	}
}
