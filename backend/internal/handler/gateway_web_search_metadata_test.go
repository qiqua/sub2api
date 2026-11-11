//go:build unit

package handler

import (
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestWebSearch_RecordsCommittedResponseMetadata(t *testing.T) {
	for _, xSearch := range []bool{false, true} {
		name := "web_search"
		if xSearch {
			name = "x_search"
		}
		t.Run(name, func(t *testing.T) {
			env := newCachedBillingEnv(t, 500, 900)
			env.group.Platform = service.PlatformGrok
			env.accounts[900].Platform = service.PlatformGrok
			env.upstream.createBody = `{"output":[{"type":"web_search_call","action":{"sources":[{"url":"https://example.com/result","title":"Result"}]}}]}`
			rec := env.serve(http.MethodPost, "/v1/"+name, `{"query":"test query"}`, nil, func(c *gin.Context) {
				c.Set("grok_x_search_endpoint", xSearch)
				env.h.WebSearch(c)
			})
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			require.Contains(t, rec.Body.String(), "https://example.com/result")
			env.waitForUsage(t, 1)
			env.usage.requireMetadata(t, http.MethodPost, rec.Code)
		})
	}
}
