package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type batchRouteAccounts struct{ *systemOneAutoRouteAccountRepo }

func (a batchRouteAccounts) ListSchedulableByGroupIDAndPlatform(_ context.Context, id int64, _ string) ([]service.Account, error) {
	return a.accounts[id], nil
}

type batchRoutePricing struct{}

func (batchRoutePricing) BatchImageUnitPrice(context.Context, *service.BatchImageJob) (float64, error) {
	return 1, nil
}

func TestBatchImageModelsHonorsBothGroupAllowlists(t *testing.T) {
	const shared, selectedOnly, primaryOnly = "gemini-2.5-flash-image", "gemini-3-pro-image", "gemini-3.1-flash-image"
	primary := &service.Group{ID: 1, Platform: service.PlatformGemini, Status: service.StatusActive, AllowBatchImageGeneration: true,
		ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{shared, primaryOnly}}}
	selected := *primary
	selected.ID = 2
	selected.ModelAllowlist.Models = []string{shared, selectedOnly}
	groups := &systemOneAutoRouteGroupRepo{groups: map[int64]*service.Group{1: primary, 2: &selected}}
	accounts := batchRouteAccounts{&systemOneAutoRouteAccountRepo{accounts: map[int64][]service.Account{
		2: {{ID: 3, Platform: service.PlatformGemini, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Concurrency: 5,
			Credentials: map[string]any{"api_key": "fixture", "model_mapping": map[string]any{shared: shared, selectedOnly: selectedOnly, primaryOnly: primaryOnly}}}},
	}}}
	cfg := &config.Config{RunMode: config.RunModeSimple}
	cfg.BatchImage.Enabled = true
	auto := service.NewAPIKeyAutoRouter(groups, accounts, service.NewGroupCapacityService(accounts, groups, nil, nil, nil), systemOneAutoRouteUserRepo{}, nil, cfg, nil)
	h := NewBatchImageHandler(&service.BatchImagePublicService{Config: cfg, Repo: struct{ service.BatchImageRepository }{}, AccountRepo: accounts, GroupRepo: groups,
		Pricing: batchRoutePricing{}, ProviderRegistry: service.NewDefaultBatchImageProviderRegistry()}, nil, nil, auto)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/images/batches/models", nil)
	c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 20, UserID: 10, GroupID: &primary.ID, Group: primary, RoutingMode: service.APIKeyRoutingModeAuto, AutoRouteGroupIDs: []int64{2}})
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 10})
	h.Models(c)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), shared)
	require.NotContains(t, w.Body.String(), selectedOnly)
	require.NotContains(t, w.Body.String(), primaryOnly)
}

func TestBatchImageSubmissionStopsOnRoutingDenial(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/batches", strings.NewReader(`{"model":"gemini-2.5-flash-image","items":[{"custom_id":"one","prompt":"cat"}]}`))
	groupID := int64(1)
	c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 20, UserID: 10, GroupID: &groupID, RoutingMode: service.APIKeyRoutingModeAuto})
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 10})
	(&BatchImageHandler{}).Submit(c)
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.Contains(t, w.Body.String(), "Unable to verify API key routing permissions")
}
