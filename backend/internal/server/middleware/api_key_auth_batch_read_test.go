//go:build unit

package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyAuthPaidBatchReadsAfterBalanceHold(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const root = "/v1/images/batches"
	for _, tc := range []struct {
		name, method, route, path string
		keyStatus                 string
		expired                   bool
		want                      int
	}{
		{"poll", "GET", root + "/:id", root + "/imgbatch_test", service.StatusActive, false, 200},
		{"list own items", "GET", root + "/:id/items", root + "/imgbatch_test/items", service.StatusActive, false, 200},
		{"download own result", "GET", root + "/:id/items/:custom_id/content", root + "/imgbatch_test/items/canvas_image/content", service.StatusActive, false, 200},
		{"download own archive", "GET", root + "/:id/download", root + "/imgbatch_test/download", service.StatusActive, false, 200},
		{"find own batches", "GET", root, root, service.StatusActive, false, 200},
		{"recover submission", "GET", root + "/by-idempotency-key", root + "/by-idempotency-key", service.StatusActive, false, 200},
		{"model listing", "GET", root + "/models", root + "/models", service.StatusActive, false, 403},
		{"submit", "POST", root, root, service.StatusActive, false, 403},
		{"disabled key", "GET", root + "/:id", root + "/imgbatch_test", service.StatusDisabled, false, 401},
		{"expired key", "GET", root + "/:id", root + "/imgbatch_test", service.StatusActive, true, 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key := &service.APIKey{ID: 22, UserID: 11, Key: "paid-batch-test", Status: tc.keyStatus,
				User: &service.User{ID: 11, Role: service.RoleUser, Status: service.StatusActive, Balance: 0},
			}
			if tc.expired {
				past := time.Now().Add(-time.Hour)
				key.ExpiresAt = &past
			}
			repo := &stubApiKeyRepo{getByKey: func(context.Context, string) (*service.APIKey, error) { return key, nil }}
			cfg := &config.Config{RunMode: config.RunModeStandard}
			router := gin.New()
			router.Use(gin.HandlerFunc(NewAPIKeyAuthMiddleware(service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, cfg), nil, cfg)))
			router.Handle(tc.method, tc.route, func(c *gin.Context) {
				owner, ok := GetAPIKeyFromContext(c)
				require.True(t, ok)
				require.Equal(t, key.ID, owner.ID)
				require.Equal(t, key.UserID, owner.UserID)
				c.Status(http.StatusOK)
			})
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("X-API-Key", key.Key)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			require.Equal(t, tc.want, w.Code)
		})
	}
}
