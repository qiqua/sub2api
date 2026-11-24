package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type routeSubscriptions struct {
	service.UserSubscriptionRepository
	subs map[int64]*service.UserSubscription
}

func (s routeSubscriptions) GetActiveByUserIDAndGroupID(_ context.Context, userID, groupID int64) (*service.UserSubscription, error) {
	if sub := s.subs[groupID]; sub != nil && sub.UserID == userID {
		return sub, nil
	}
	return nil, service.ErrSubscriptionNotFound
}

func TestAutoRouteInstallsSelectedBillingContext(t *testing.T) {
	for _, tc := range []struct {
		name                                        string
		primarySub, selectedSub, publicModelBlocked bool
		wantGroup, wantSub                          int64
	}{
		{"subscription to subscription", true, true, false, 2, 102},
		{"balance to subscription", false, true, false, 2, 102},
		{"subscription to balance", true, false, false, 2, 0},
		{"selected group allows only the rewritten name", true, true, true, 1, 101},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const userID = 3
			a := &service.Group{ID: 1, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true}
			b := &service.Group{ID: 2, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true}
			groups := &systemOneAutoRouteGroupRepo{groups: map[int64]*service.Group{1: a, 2: b}}
			now := time.Now()
			subs := routeSubscriptions{subs: make(map[int64]*service.UserSubscription)}
			for id, enabled := range map[int64]bool{1: tc.primarySub, 2: tc.selectedSub} {
				if enabled {
					groups.groups[id].SubscriptionType = service.SubscriptionTypeSubscription
					subs.subs[id] = &service.UserSubscription{ID: 100 + id, UserID: userID, GroupID: id, Status: service.SubscriptionStatusActive, StartsAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour), DailyWindowStart: &now, WeeklyWindowStart: &now, MonthlyWindowStart: &now}
				}
			}
			if tc.publicModelBlocked {
				b.ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"upstream-model"}}
			}
			accounts := &systemOneAutoRouteAccountRepo{accounts: map[int64][]service.Account{
				2: {{ID: 20, Platform: service.PlatformOpenAI, Status: service.StatusActive, Schedulable: true, Concurrency: 5}},
			}}
			router := service.NewAPIKeyAutoRouter(groups, accounts, service.NewGroupCapacityService(accounts, groups, nil, nil, nil), systemOneAutoRouteUserRepo{}, service.NewSubscriptionService(groups, subs, nil, nil, nil), nil, nil)
			key := &service.APIKey{ID: 9, UserID: userID, User: &service.User{ID: userID}, Group: a, GroupID: &a.ID, RoutingMode: service.APIKeyRoutingModeAuto, AutoRouteGroupIDs: []int64{2}}
			r := gin.New()
			r.Use(func(c *gin.Context) {
				c.Set(string(middleware.ContextKeyAPIKey), key)
				if sub := subs.subs[a.ID]; sub != nil {
					c.Set(string(middleware.ContextKeySubscription), sub)
				}
				c.Next()
			}, middleware.GroupModelAllowlist())
			r.POST("/v1/responses", func(c *gin.Context) {
				routed := applyAPIKeyAutoRoute(c, router, nil, key, "upstream-model")
				require.NotNil(t, routed)
				require.Equal(t, tc.wantGroup, *routed.GroupID)
				require.Same(t, routed.Group, c.Request.Context().Value(ctxkey.Group))
				authKey, ok := middleware.GetAPIKeyFromContext(c)
				require.True(t, ok)
				require.Same(t, routed, authKey)
				sub, exists := middleware.GetSubscriptionFromContext(c)
				require.Equal(t, tc.wantSub != 0, exists)
				if tc.wantSub != 0 {
					require.Equal(t, tc.wantSub, sub.ID)
					require.Equal(t, *routed.GroupID, sub.GroupID)
				}
				c.Status(http.StatusNoContent)
			})
			req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"public-model"}`))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			require.Equal(t, http.StatusNoContent, w.Code)
			require.Same(t, a, key.Group)
		})
	}
}

func TestAutoRouteGeminiForcedGetModelUsesPublicName(t *testing.T) {
	for _, tc := range []struct {
		allow     string
		wantGroup int64
	}{
		{"public-name", 2},
		{"gemini-upstream", 1},
	} {
		t.Run(tc.allow, func(t *testing.T) {
			a := &service.Group{ID: 1, Status: service.StatusActive, Platform: service.PlatformGemini}
			b := &service.Group{ID: 2, Status: service.StatusActive, Platform: service.PlatformGemini, ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{tc.allow}}}
			groups := &systemOneAutoRouteGroupRepo{groups: map[int64]*service.Group{1: a, 2: b}}
			accounts := &systemOneAutoRouteAccountRepo{accounts: map[int64][]service.Account{
				2: {{ID: 21, Status: service.StatusActive, Platform: service.PlatformAntigravity, Schedulable: true, Concurrency: 2,
					Credentials: map[string]any{"model_mapping": map[string]any{"gemini-upstream": "gemini-upstream"}}}},
			}}
			h := GatewayHandler{apiKeyAutoRouter: service.NewAPIKeyAutoRouter(groups, accounts, service.NewGroupCapacityService(accounts, groups, nil, nil, nil), systemOneAutoRouteUserRepo{}, nil, nil, nil)}
			key := &service.APIKey{ID: 17, UserID: 3, GroupID: &a.ID, Group: a, RoutingMode: service.APIKeyRoutingModeAuto, AutoRouteGroupIDs: []int64{2}}
			r := gin.New()
			r.Use(middleware.ForcePlatform(service.PlatformAntigravity), func(c *gin.Context) {
				c.Set(string(middleware.ContextKeyAPIKey), key)
				c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.ResolvedUpstreamModel, "gemini-upstream"))
				c.Next()
			}, middleware.GroupModelAllowlist())
			r.GET("/antigravity/v1beta/models/:model", func(c *gin.Context) {
				h.GeminiV1BetaGetModel(c)
				got, _ := middleware.GetAPIKeyFromContext(c)
				require.Equal(t, tc.wantGroup, got.Group.ID)
			})
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/antigravity/v1beta/models/public-name", nil))
			require.Equal(t, http.StatusOK, w.Code)
		})
	}
}
