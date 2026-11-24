//go:build unit

package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAsyncImageWorkerRevalidatesPinnedGrant(t *testing.T) {
	for _, tc := range []struct {
		name       string
		change     func(*service.APIKey, *service.Group)
		wantStatus int
		wantCalls  int
	}{
		{"capacity moves to primary", func(*service.APIKey, *service.Group) {}, http.StatusBadRequest, 1},
		{"alternate removed", func(key *service.APIKey, _ *service.Group) { key.AutoRouteGroupIDs = nil }, http.StatusForbidden, 0},
		{"key disabled", func(key *service.APIKey, _ *service.Group) { key.Status = service.StatusDisabled }, http.StatusUnauthorized, 0},
		{"owner changed", func(key *service.APIKey, _ *service.Group) { key.UserID++; key.User.ID++ }, http.StatusForbidden, 0},
		{"selected image right revoked", func(_ *service.APIKey, group *service.Group) { group.AllowImageGeneration = false }, http.StatusForbidden, 0},
		{"duplicate public model denied", func(_ *service.APIKey, group *service.Group) {
			group.ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"grok-imagine-image"}}
		}, http.StatusForbidden, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, slots, _, upstream := newGrokMediaSlotHandler(t, false, false)
			var executionGroup int64
			upstream.call = func(req *http.Request, _ int64) (*http.Response, error) {
				if group, _ := req.Context().Value(ctxkey.Group).(*service.Group); group != nil {
					executionGroup = group.ID
				}
				return &http.Response{StatusCode: http.StatusBadRequest, Header: http.Header{"Content-Type": {"application/json"}},
					Body: io.NopCloser(strings.NewReader(`{"code":"invalid_image","error":"provider received pinned request"}`))}, nil
			}
			const primaryID, alternateID int64 = 23, 24
			primary := grokMediaTaskKey()
			primary.Group.ID = primaryID
			primary.GroupID = &primary.Group.ID
			primary.RoutingMode = service.APIKeyRoutingModeAuto
			primary.AutoRouteGroupIDs = []int64{alternateID}
			alternate := *primary.Group
			alternate.ID = alternateID
			groups := &systemOneAutoRouteGroupRepo{groups: map[int64]*service.Group{primaryID: primary.Group, alternateID: &alternate}}
			accounts := &systemOneAutoRouteAccountRepo{accounts: map[int64][]service.Account{
				alternateID: {{ID: 12, Platform: service.PlatformGrok, Status: service.StatusActive, Schedulable: true, Concurrency: 5}},
			}}
			h.apiKeyAutoRouter = service.NewAPIKeyAutoRouter(groups, accounts, service.NewGroupCapacityService(accounts, groups, nil, nil, nil), systemOneAutoRouteUserRepo{}, nil, h.cfg, nil)
			h.apiKeyService = service.NewAPIKeyService(grokMediaTaskKeyRepo{load: func() *service.APIKey {
				// Reentry would select the primary if it reranked after authentication.
				accounts.accounts[primaryID] = accounts.accounts[alternateID]
				delete(accounts.accounts, alternateID)
				fresh := *primary
				user := *primary.User
				fresh.User = &user
				tc.change(&fresh, &alternate)
				return &fresh
			}}, nil, nil, nil, nil, nil, h.cfg)
			tasks := service.NewImageTaskServiceWithUploader(&asyncImageMemoryStore{tasks: map[string]*service.ImageTaskRecord{}}, nil, time.Hour, time.Minute)
			worker := NewAsyncImageHandler(tasks, h, nil)
			var selectedAtSubmit int64
			router := gin.New()
			router.Use(func(c *gin.Context) {
				c.Set(string(middleware.ContextKeyAPIKey), primary)
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: primary.UserID})
			}, middleware.GroupModelAllowlist())
			router.POST("/v1/images/generations/async", func(c *gin.Context) {
				worker.Submit(c)
				key, _ := middleware.GetAPIKeyFromContext(c)
				selectedAtSubmit = *key.GroupID
			})
			// Capture all client spellings, including a variant that the media parser ignores.
			req := httptest.NewRequest(http.MethodPost, "/v1/images/generations/async", strings.NewReader(`{"model":"grok-imagine-image","Model":"public-other","prompt":"cat"}`))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-API-Key", "test-key")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			require.Equal(t, http.StatusAccepted, w.Code, w.Body.String())
			require.Equal(t, alternateID, selectedAtSubmit)
			id := gjson.GetBytes(w.Body.Bytes(), "task_id").String()
			owner := service.ImageTaskOwner{UserID: primary.UserID, APIKeyID: primary.ID}
			require.Eventually(t, func() bool {
				task, err := tasks.Get(context.Background(), owner, id)
				return err == nil && task.Status == service.ImageTaskStatusFailed && task.HTTPStatus == tc.wantStatus
			}, time.Second, 10*time.Millisecond)
			require.Equal(t, tc.wantCalls, upstream.calls)
			if tc.wantCalls != 0 {
				require.Equal(t, alternateID, executionGroup)
			}
			slots.assertReleased(t)
		})
	}
}

func TestAsyncImageSubmissionUsesResolvedPlatformAndSubject(t *testing.T) {
	for _, tc := range []struct {
		name   string
		userID int64
		forced string
		want   int
	}{
		{"resolved composite", 10, "", http.StatusAccepted},
		{"different subject", 11, "", http.StatusForbidden},
		{"unsupported forced platform", 10, service.PlatformAntigravity, http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &asyncImageMemoryStore{tasks: map[string]*service.ImageTaskRecord{}}
			tasks := service.NewImageTaskServiceWithUploader(store, nil, time.Hour, time.Minute)
			dispatched := make(chan string, 1)
			h := &AsyncImageHandler{tasks: tasks, execute: func(platform string, c *gin.Context) {
				dispatched <- platform
				c.JSON(http.StatusOK, gin.H{"data": []string{}})
			}}
			c, w := grokMediaSlotContext(context.Background(), true)
			key, _ := middleware.GetAPIKeyFromContext(c)
			key.Group.Platform = service.PlatformComposite
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: tc.userID})
			ctx := service.WithResolvedTargetPlatform(context.Background(), service.PlatformGrok)
			if tc.forced != "" {
				ctx = context.WithValue(ctx, ctxkey.ForcePlatform, tc.forced)
			}
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations/async", strings.NewReader(`{"model":"grok-imagine-image","prompt":"cat"}`)).WithContext(ctx)
			c.Request.Header.Set("Content-Type", "application/json")
			h.Submit(c)
			require.Equal(t, tc.want, w.Code, w.Body.String())
			if tc.want == http.StatusAccepted {
				select {
				case platform := <-dispatched:
					require.Equal(t, service.PlatformGrok, platform)
				case <-time.After(time.Second):
					t.Fatal("accepted task was not dispatched")
				}
			} else {
				require.Empty(t, store.tasks)
			}
		})
	}
}

func TestOpenAIImageWorkerRechecksPublicAliasOnSelectedGroup(t *testing.T) {
	h, slots, _, upstream := newGrokMediaSlotHandler(t, false, false)
	primary := grokMediaTaskKey()
	primary.Group.Platform = service.PlatformOpenAI
	primary.Group.ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"public-art"}}
	primary.RoutingMode = service.APIKeyRoutingModeAuto
	const selectedID int64 = 25
	primary.AutoRouteGroupIDs = []int64{selectedID}
	selected := *primary.Group
	selected.ID = selectedID
	groups := &systemOneAutoRouteGroupRepo{groups: map[int64]*service.Group{primary.Group.ID: primary.Group, selectedID: &selected}}
	accounts := &systemOneAutoRouteAccountRepo{accounts: map[int64][]service.Account{
		selectedID: {{ID: 3, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Concurrency: 5}},
	}}
	h.apiKeyAutoRouter = service.NewAPIKeyAutoRouter(groups, accounts, service.NewGroupCapacityService(accounts, groups, nil, nil, nil), systemOneAutoRouteUserRepo{}, nil, h.cfg, nil)
	h.apiKeyService = service.NewAPIKeyService(grokMediaTaskKeyRepo{load: func() *service.APIKey {
		selected.ModelAllowlist = service.GroupModelAllowlist{Enabled: true, Models: []string{"gpt-image-2"}}
		fresh := *primary
		return &fresh
	}}, nil, nil, nil, nil, nil, h.cfg)
	tasks := service.NewImageTaskServiceWithUploader(&asyncImageMemoryStore{tasks: map[string]*service.ImageTaskRecord{}}, nil, time.Hour, time.Minute)
	worker := NewAsyncImageHandler(tasks, h, nil)
	c, w := grokMediaSlotContext(context.Background(), true)
	c.Set(string(middleware.ContextKeyAPIKey), primary)
	ctx := service.WithCompositeRouteDecision(context.Background(), service.CompositeRouteDecision{
		Matched: true, PublicModel: "public-art", TargetPlatform: service.PlatformOpenAI, UpstreamModel: "gpt-image-2",
	})
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations/async", strings.NewReader(`{"model":"gpt-image-2","prompt":"cat"}`)).WithContext(ctx)
	c.Request.Header.Set("X-API-Key", "test-key")
	c.Request.Header.Set("Content-Type", "application/json")
	worker.Submit(c)
	require.Equal(t, http.StatusAccepted, w.Code, w.Body.String())
	key, _ := middleware.GetAPIKeyFromContext(c)
	require.Equal(t, selectedID, *key.GroupID)
	id := gjson.GetBytes(w.Body.Bytes(), "task_id").String()
	require.Eventually(t, func() bool {
		task, err := tasks.Get(context.Background(), service.ImageTaskOwner{UserID: primary.UserID, APIKeyID: primary.ID}, id)
		return err == nil && task.Status == service.ImageTaskStatusFailed && task.HTTPStatus == http.StatusForbidden
	}, time.Second, 10*time.Millisecond)
	require.Zero(t, upstream.calls)
	slots.assertReleased(t)
}

type imageTaskReentrySubscriptions struct {
	service.UserSubscriptionRepository
	sub *service.UserSubscription
}

func (r imageTaskReentrySubscriptions) GetActiveByUserIDAndGroupID(context.Context, int64, int64) (*service.UserSubscription, error) {
	return r.sub, nil
}

func TestAsyncImageWorkerRejectsReplacedBillingSubscription(t *testing.T) {
	h, slots, _, upstream := newGrokMediaSlotHandler(t, false, false)
	h.cfg.RunMode = config.RunModeStandard
	key := grokMediaTaskKey()
	key.Group.SubscriptionType = service.SubscriptionTypeSubscription
	h.apiKeyService = service.NewAPIKeyService(grokMediaTaskKeyRepo{load: func() *service.APIKey { return key }}, nil, nil, nil, nil, nil, h.cfg)
	now := time.Now()
	admitted := &service.UserSubscription{ID: 100, GroupID: *key.GroupID, UserID: key.UserID, Status: service.SubscriptionStatusActive,
		ExpiresAt: now.Add(time.Hour), DailyWindowStart: &now, WeeklyWindowStart: &now, MonthlyWindowStart: &now}
	replacement := *admitted
	replacement.ID++
	subs := service.NewSubscriptionService(nil, imageTaskReentrySubscriptions{sub: &replacement}, nil, nil, h.cfg)
	t.Cleanup(subs.Stop)
	tasks := service.NewImageTaskServiceWithUploader(&asyncImageMemoryStore{tasks: map[string]*service.ImageTaskRecord{}}, nil, time.Hour, time.Minute)
	worker := NewAsyncImageHandler(tasks, h, subs)
	c, w := grokMediaSlotContext(context.Background(), true)
	c.Set(string(middleware.ContextKeyAPIKey), key)
	c.Set(string(middleware.ContextKeySubscription), admitted)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations/async", strings.NewReader(`{"model":"grok-imagine-image","prompt":"cat"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("X-API-Key", "test-key")
	worker.Submit(c)
	require.Equal(t, http.StatusAccepted, w.Code, w.Body.String())
	id := gjson.GetBytes(w.Body.Bytes(), "task_id").String()
	require.Eventually(t, func() bool {
		task, err := tasks.Get(context.Background(), service.ImageTaskOwner{UserID: key.UserID, APIKeyID: key.ID}, id)
		return err == nil && task.Status == service.ImageTaskStatusFailed && task.HTTPStatus == http.StatusForbidden && strings.Contains(string(task.Error), "IMAGE_TASK_FORBIDDEN")
	}, time.Second, 10*time.Millisecond)
	require.Zero(t, upstream.calls)
	slots.assertReleased(t)
}
