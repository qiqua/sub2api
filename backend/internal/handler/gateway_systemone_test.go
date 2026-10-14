package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const validSystemOneHandlerBody = `{"model":"jev-latest","state":"sample","questions":{"q":{"type":"noul","instructions":"Evaluate"}}}`

func newSystemOneHandlerContext(body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/systemone", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, recorder
}

func TestSystemOneRequiresAuthentication(t *testing.T) {
	c, recorder := newSystemOneHandlerContext(validSystemOneHandlerBody)
	(&GatewayHandler{}).SystemOne(c)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	require.Contains(t, recorder.Body.String(), "authentication_error")
}

func TestSystemOneRejectsNonTypeSafeGroupBeforeScheduling(t *testing.T) {
	c, recorder := newSystemOneHandlerContext(validSystemOneHandlerBody)
	groupID := int64(3)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{ID: 4, UserID: 5, GroupID: &groupID, Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI}})
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 5, Concurrency: 1})

	(&GatewayHandler{cfg: &config.Config{Gateway: config.GatewayConfig{MaxBodySize: 1 << 20}}}).SystemOne(c)
	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Contains(t, recorder.Body.String(), "only available for TypeSafe")
}

func newTypeSafeGroupContext(t *testing.T, path, body, groupPlatform string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	c, recorder := newSystemOneHandlerContext(body)
	c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	groupID := int64(9)
	c.Set(string(middleware2.ContextKeyAPIKey), &service.APIKey{ID: 4, UserID: 5, GroupID: &groupID, Group: &service.Group{ID: groupID, Platform: groupPlatform}})
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 5, Concurrency: 1})
	return c, recorder
}

func TestRejectSystemOneOnlyPlatform(t *testing.T) {
	write := func(c *gin.Context, status int, errType, message string) {
		c.JSON(status, gin.H{"type": errType, "message": message})
	}

	c, recorder := newTypeSafeGroupContext(t, "/v1/messages", `{}`, service.PlatformTypeSafe)
	require.True(t, rejectSystemOneOnlyPlatform(c, mustAPIKey(t, c), write))
	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Contains(t, recorder.Body.String(), systemOneOnlyPlatformMessage)

	c, _ = newTypeSafeGroupContext(t, "/v1/messages", `{}`, service.PlatformComposite)
	require.False(t, rejectSystemOneOnlyPlatform(c, mustAPIKey(t, c), write))
	c.Request = c.Request.WithContext(service.WithResolvedTargetPlatform(c.Request.Context(), service.PlatformTypeSafe))
	require.True(t, rejectSystemOneOnlyPlatform(c, mustAPIKey(t, c), write))

	c, _ = newTypeSafeGroupContext(t, "/v1/messages", `{}`, service.PlatformAnthropic)
	require.False(t, rejectSystemOneOnlyPlatform(c, mustAPIKey(t, c), write))

	c, _ = newTypeSafeGroupContext(t, "/antigravity/v1/messages", `{}`, service.PlatformTypeSafe)
	c.Set(string(middleware2.ContextKeyForcePlatform), service.PlatformAntigravity)
	require.False(t, rejectSystemOneOnlyPlatform(c, mustAPIKey(t, c), write))
}

func mustAPIKey(t *testing.T, c *gin.Context) *service.APIKey {
	t.Helper()
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	require.True(t, ok)
	return apiKey
}

type systemOneAutoRouteGroupRepo struct {
	service.GroupRepository
	groups map[int64]*service.Group
}

func (r *systemOneAutoRouteGroupRepo) GetByID(_ context.Context, id int64) (*service.Group, error) {
	return r.groups[id], nil
}

type systemOneAutoRouteAccountRepo struct {
	service.AccountRepository
	accounts map[int64][]service.Account
}

func (r *systemOneAutoRouteAccountRepo) ListSchedulableByGroupID(_ context.Context, id int64) ([]service.Account, error) {
	return r.accounts[id], nil
}

func TestSystemOneAppliesAPIKeyRoutingBeforeScheduling(t *testing.T) {
	for _, tc := range []struct {
		name        string
		routingMode string
		wantGroupID int64
	}{
		{name: "auto", routingMode: service.APIKeyRoutingModeAuto, wantGroupID: 10},
		{name: "fixed", routingMode: service.APIKeyRoutingModeFixed, wantGroupID: 9},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, recorder := newTypeSafeGroupContext(t, "/v1/systemone", validSystemOneHandlerBody, service.PlatformTypeSafe)
			apiKey := mustAPIKey(t, c)
			apiKey.RoutingMode = tc.routingMode
			apiKey.AutoRouteGroupIDs = []int64{10}
			apiKey.Group.Status = service.StatusActive
			groupRepo := &systemOneAutoRouteGroupRepo{groups: map[int64]*service.Group{
				9:  apiKey.Group,
				10: {ID: 10, Platform: service.PlatformTypeSafe, Status: service.StatusActive},
			}}
			accountRepo := &systemOneAutoRouteAccountRepo{accounts: map[int64][]service.Account{
				10: {{ID: 100, Platform: service.PlatformTypeSafe, Status: service.StatusActive, Schedulable: true, Concurrency: 1}},
			}}
			capacityService := service.NewGroupCapacityService(accountRepo, groupRepo, nil, nil, nil)
			slotChecked := false
			cache := &concurrencyCacheMock{
				acquireUserSlotFn: func(_ context.Context, _ int64, _ int, _ string) (bool, error) {
					slotChecked = true
					routed := mustAPIKey(t, c)
					require.Equal(t, tc.wantGroupID, *routed.GroupID)
					require.Equal(t, tc.wantGroupID, routed.Group.ID)
					return false, errors.New("stop before billing")
				},
			}
			h := &GatewayHandler{
				cfg:               &config.Config{Gateway: config.GatewayConfig{MaxBodySize: 1 << 20}},
				gatewayService:    &service.GatewayService{},
				apiKeyAutoRouter:  service.NewAPIKeyAutoRouter(groupRepo, accountRepo, capacityService),
				concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, 0),
			}

			h.SystemOne(c)

			require.True(t, slotChecked)
			require.Equal(t, http.StatusServiceUnavailable, recorder.Code, recorder.Body.String())
			require.Equal(t, int64(9), *apiKey.GroupID, "routing must not mutate the authenticated key")
		})
	}
}

func TestTypeSafeGroupsRejectNonSystemOneProtocolsBeforeScheduling(t *testing.T) {
	h := &GatewayHandler{
		cfg:            &config.Config{Gateway: config.GatewayConfig{MaxBodySize: 1 << 20}},
		gatewayService: &service.GatewayService{},
	}
	for _, tc := range []struct {
		name    string
		path    string
		body    string
		handler func(*gin.Context)
	}{
		{"messages", "/v1/messages", `{"model":"jev-latest","max_tokens":1,"messages":[{"role":"user","content":"hi"}]}`, h.Messages},
		{"count_tokens", "/v1/messages/count_tokens", `{"model":"jev-latest","messages":[{"role":"user","content":"hi"}]}`, h.CountTokens},
		{"chat_completions", "/v1/chat/completions", `{"model":"jev-latest","messages":[{"role":"user","content":"hi"}]}`, h.ChatCompletions},
		{"responses", "/v1/responses", `{"model":"jev-latest","input":"hi"}`, h.Responses},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, recorder := newTypeSafeGroupContext(t, tc.path, tc.body, service.PlatformTypeSafe)
			tc.handler(c)
			require.Equal(t, http.StatusNotFound, recorder.Code, recorder.Body.String())
			require.Contains(t, recorder.Body.String(), systemOneOnlyPlatformMessage)
		})
	}
}
