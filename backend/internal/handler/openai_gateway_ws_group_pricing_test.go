package handler

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
	"github.com/stretchr/testify/require"
)

// 分组调价对已打开的 WebSocket 连接生效：同一条连接的两个 turn 之间由管理端
// 改分组倍率（并按分组失效认证缓存），第二个 turn 必须按新倍率计费、按新售价
// 过利润门；HTTP 请求每次经认证缓存取快照，本来就是这个行为。

const (
	wsGroupPricingGroupID = int64(4201) // 与 runOpenAIResponsesWebSocketUsageLogCase 的调度分组一致
	wsGroupPricingKey     = "sk-ws-group-pricing"
	wsGroupPricingFirst   = `{"type":"response.create","model":"gpt-5.6-sol","stream":false}`
	wsGroupPricingSecond  = `{"type":"response.create","model":"gpt-5.6-sol","stream":false}`
)

// wsGroupPricingAPIKeyRepoStub 只实现认证查询与按分组列 Key（认证缓存失效用）。
type wsGroupPricingAPIKeyRepoStub struct {
	service.APIKeyRepository
	mu      sync.Mutex
	apiKey  service.APIKey
	group   service.Group
	err     error
	lookups int
}

func newWSGroupPricingAPIKeyRepoStub(rate float64) *wsGroupPricingAPIKeyRepoStub {
	groupID := wsGroupPricingGroupID
	return &wsGroupPricingAPIKeyRepoStub{
		apiKey: service.APIKey{
			ID:      1801,
			UserID:  1701,
			Key:     wsGroupPricingKey,
			Status:  service.StatusActive,
			GroupID: &groupID,
			User:    &service.User{ID: 1701, Status: service.StatusActive, Concurrency: 1},
		},
		group: service.Group{
			ID:             groupID,
			Name:           "ws-group-pricing",
			Platform:       service.PlatformOpenAI,
			Status:         service.StatusActive,
			RateMultiplier: rate,
			Hydrated:       true,
		},
	}
}

func (s *wsGroupPricingAPIKeyRepoStub) GetByKeyForAuth(ctx context.Context, key string) (*service.APIKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lookups++
	if s.err != nil {
		return nil, s.err
	}
	if key != s.apiKey.Key {
		return nil, service.ErrAPIKeyNotFound
	}
	apiKey := s.apiKey
	user := *s.apiKey.User
	apiKey.User = &user
	group := s.group
	apiKey.Group = &group
	return &apiKey, nil
}

func (s *wsGroupPricingAPIKeyRepoStub) ListKeysByGroupID(ctx context.Context, groupID int64) ([]string, error) {
	return []string{s.apiKey.Key}, nil
}

// adminUpdate 模拟管理端改 Key/分组后按分组失效认证缓存（与 admin 分组更新同一入口）。
func (s *wsGroupPricingAPIKeyRepoStub) adminUpdate(apiKeyService *service.APIKeyService, fn func(apiKey *service.APIKey, group *service.Group)) error {
	s.mu.Lock()
	fn(&s.apiKey, &s.group)
	s.mu.Unlock()
	apiKeyService.InvalidateAuthCacheByGroupID(context.Background(), wsGroupPricingGroupID)
	return nil
}

func (s *wsGroupPricingAPIKeyRepoStub) lookupCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lookups
}

func newWSGroupPricingAPIKeyService(repo *wsGroupPricingAPIKeyRepoStub) *service.APIKeyService {
	return service.NewAPIKeyService(repo, nil, nil, nil, nil, nil, &config.Config{})
}

func TestOpenAIResponsesWebSocket_GroupRateChangeAppliesToNextTurn(t *testing.T) {
	for _, mode := range []string{service.OpenAIWSIngressModePassthrough, service.OpenAIWSIngressModeCtxPool} {
		t.Run(mode, func(t *testing.T) {
			repo := newWSGroupPricingAPIKeyRepoStub(3.0)
			apiKeyService := newWSGroupPricingAPIKeyService(repo)
			got := runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
				firstPayload:     wsGroupPricingFirst,
				secondPayload:    wsGroupPricingSecond,
				ingressMode:      mode,
				apiKeyService:    apiKeyService,
				apiKeyCredential: wsGroupPricingKey,
				afterFirstUpstreamRequest: func(*service.ChannelService) error {
					return repo.adminUpdate(apiKeyService, func(_ *service.APIKey, group *service.Group) { group.RateMultiplier = 0.3 })
				},
			})

			require.Len(t, got.logs, 2)
			require.InDelta(t, 3.0, got.logs[0].RateMultiplier, 1e-12, "turn 1 bills at the rate in force at connect")
			require.InDelta(t, 0.3, got.logs[1].RateMultiplier, 1e-12, "turn 2 must bill at the group rate in force when it starts")
			require.Greater(t, got.logs[0].ActualCost, 0.0)
			require.InDelta(t, got.logs[0].ActualCost/10, got.logs[1].ActualCost, 1e-15)
		})
	}
}

func TestOpenAIResponsesWebSocket_GroupRateChangeReachesProfitGateOnNextTurn(t *testing.T) {
	repo := newWSGroupPricingAPIKeyRepoStub(3.0)
	repo.group.ProfitControlEnabled = true
	repo.group.ProfitMinMargin = 0.1
	apiKeyService := newWSGroupPricingAPIKeyService(repo)
	accountRate := 1.0
	runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
		firstPayload:          wsGroupPricingFirst,
		secondPayload:         wsGroupPricingSecond,
		apiKeyService:         apiKeyService,
		apiKeyCredential:      wsGroupPricingKey,
		accountRateMultiplier: &accountRate,
		// 3.0 → 0.3：账号倍率 1.0 在新售价下已越线，第二个 turn 必须在转发上游前被拒。
		afterFirstUpstreamRequest: func(*service.ChannelService) error {
			return repo.adminUpdate(apiKeyService, func(_ *service.APIKey, group *service.Group) { group.RateMultiplier = 0.3 })
		},
		secondTurnCloseExpected: true,
		closeStatus:             coderws.StatusTryAgainLater,
		closeReason:             "no longer eligible for this connection",
	})
}

func TestOpenAIResponsesWebSocket_KeyMovedToAnotherGroupKeepsConnectionGroup(t *testing.T) {
	repo := newWSGroupPricingAPIKeyRepoStub(3.0)
	apiKeyService := newWSGroupPricingAPIKeyService(repo)
	got := runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
		firstPayload:     wsGroupPricingFirst,
		secondPayload:    wsGroupPricingSecond,
		apiKeyService:    apiKeyService,
		apiKeyCredential: wsGroupPricingKey,
		afterFirstUpstreamRequest: func(*service.ChannelService) error {
			return repo.adminUpdate(apiKeyService, func(apiKey *service.APIKey, group *service.Group) {
				other := wsGroupPricingGroupID + 1
				apiKey.GroupID = &other
				group.ID = other
				group.RateMultiplier = 0.3
			})
		},
	})

	require.Len(t, got.logs, 2)
	require.InDelta(t, 3.0, got.logs[1].RateMultiplier, 1e-12,
		"a key moved to another group keeps the connection's group: the connection was scheduled from it")
}

func TestOpenAIResponsesWebSocket_FailedKeyRefreshKeepsConnectionGroup(t *testing.T) {
	repo := newWSGroupPricingAPIKeyRepoStub(3.0)
	apiKeyService := newWSGroupPricingAPIKeyService(repo)
	got := runOpenAIResponsesWebSocketUsageLogCase(t, openAIResponsesWSUsageLogCase{
		firstPayload:     wsGroupPricingFirst,
		secondPayload:    wsGroupPricingSecond,
		apiKeyService:    apiKeyService,
		apiKeyCredential: wsGroupPricingKey,
		afterFirstUpstreamRequest: func(*service.ChannelService) error {
			return repo.adminUpdate(apiKeyService, func(_ *service.APIKey, group *service.Group) {
				group.RateMultiplier = 0.3
				repo.err = errors.New("auth store unavailable")
			})
		},
	})

	require.Len(t, got.logs, 2)
	require.InDelta(t, 3.0, got.logs[1].RateMultiplier, 1e-12, "a failed refresh keeps the connection snapshot")
	require.GreaterOrEqual(t, repo.lookupCount(), 2, "the second turn must have attempted a refresh")
}

type wsTurnAPIKeyLookupFunc func(ctx context.Context, key string) (*service.APIKey, error)

func (f wsTurnAPIKeyLookupFunc) GetByKey(ctx context.Context, key string) (*service.APIKey, error) {
	return f(ctx, key)
}

func TestRefreshOpenAIWSTurnBillingAPIKey(t *testing.T) {
	groupID := int64(77)
	conn := &service.APIKey{
		ID:      5,
		Key:     "sk-conn",
		GroupID: &groupID,
		User:    &service.User{ID: 9},
		Group:   &service.Group{ID: groupID, Platform: service.PlatformOpenAI, SubscriptionType: service.SubscriptionTypeStandard, RateMultiplier: 3},
	}
	latestWith := func(mutate func(k *service.APIKey)) *service.APIKey {
		gid := groupID
		k := &service.APIKey{
			ID:      5,
			Key:     "sk-conn",
			GroupID: &gid,
			User:    &service.User{ID: 9, Balance: 123},
			Group:   &service.Group{ID: groupID, Platform: service.PlatformOpenAI, SubscriptionType: service.SubscriptionTypeStandard, RateMultiplier: 0.3},
		}
		if mutate != nil {
			mutate(k)
		}
		return k
	}

	t.Run("same group adopts the latest group and nothing else", func(t *testing.T) {
		got := refreshOpenAIWSTurnBillingAPIKey(context.Background(), wsTurnAPIKeyLookupFunc(func(ctx context.Context, key string) (*service.APIKey, error) {
			require.Equal(t, "sk-conn", key)
			return latestWith(nil), nil
		}), conn)
		require.NotSame(t, conn, got)
		require.InDelta(t, 0.3, got.Group.RateMultiplier, 1e-12)
		require.Same(t, conn.User, got.User, "only the group snapshot is refreshed")
		require.Equal(t, conn.ID, got.ID)
		require.InDelta(t, 3.0, conn.Group.RateMultiplier, 1e-12, "the connection snapshot is never mutated")
	})

	keep := map[string]wsTurnAPIKeyLookupFunc{
		"lookup error": func(ctx context.Context, key string) (*service.APIKey, error) {
			return nil, errors.New("boom")
		},
		"not found": func(ctx context.Context, key string) (*service.APIKey, error) {
			return nil, service.ErrAPIKeyNotFound
		},
		"nil key": func(ctx context.Context, key string) (*service.APIKey, error) { return nil, nil },
		"different key id": func(ctx context.Context, key string) (*service.APIKey, error) {
			return latestWith(func(k *service.APIKey) { k.ID = 6 }), nil
		},
		"moved group": func(ctx context.Context, key string) (*service.APIKey, error) {
			return latestWith(func(k *service.APIKey) { other := int64(78); k.GroupID = &other; k.Group.ID = other }), nil
		},
		"no group": func(ctx context.Context, key string) (*service.APIKey, error) {
			return latestWith(func(k *service.APIKey) { k.Group = nil }), nil
		},
		"platform changed": func(ctx context.Context, key string) (*service.APIKey, error) {
			return latestWith(func(k *service.APIKey) { k.Group.Platform = service.PlatformAnthropic }), nil
		},
		"billing type changed": func(ctx context.Context, key string) (*service.APIKey, error) {
			return latestWith(func(k *service.APIKey) { k.Group.SubscriptionType = service.SubscriptionTypeSubscription }), nil
		},
	}
	for name, lookup := range keep {
		t.Run(name+" keeps the connection snapshot", func(t *testing.T) {
			require.Same(t, conn, refreshOpenAIWSTurnBillingAPIKey(context.Background(), lookup, conn))
		})
	}

	t.Run("no credential or no group skips the lookup", func(t *testing.T) {
		called := false
		lookup := wsTurnAPIKeyLookupFunc(func(ctx context.Context, key string) (*service.APIKey, error) {
			called = true
			return latestWith(nil), nil
		})
		noKey := *conn
		noKey.Key = ""
		require.Same(t, &noKey, refreshOpenAIWSTurnBillingAPIKey(context.Background(), lookup, &noKey))
		noGroup := *conn
		noGroup.Group = nil
		require.Same(t, &noGroup, refreshOpenAIWSTurnBillingAPIKey(context.Background(), lookup, &noGroup))
		require.Nil(t, refreshOpenAIWSTurnBillingAPIKey(context.Background(), lookup, nil))
		require.Same(t, conn, refreshOpenAIWSTurnBillingAPIKey(context.Background(), nil, conn))
		require.False(t, called)
	})
}

func TestOpenAIWSTurnBillingAPIKeysKeepPreviousTurnUntilItIsRecorded(t *testing.T) {
	conn := &service.APIKey{ID: 1}
	turn2 := &service.APIKey{ID: 2}
	turn3 := &service.APIKey{ID: 3}
	var keys openAIWSTurnBillingAPIKeys
	require.Same(t, conn, keys.forTurn(1, conn), "a turn without BeforeTurn bills with the connection snapshot")
	keys.set(2, turn2)
	keys.set(3, turn3) // turn 3 starts before turn 2's usage is submitted
	require.Same(t, turn2, keys.forTurn(2, conn))
	require.Same(t, turn3, keys.forTurn(3, conn))
	keys.set(4, conn)
	require.Same(t, conn, keys.forTurn(2, conn), "only the current and previous turn are kept")
	require.Len(t, keys.keys, 2)
}

func TestRefreshOpenAIWSTurnBillingAPIKey_AutoRoutedGroup(t *testing.T) {
	primaryID, selectedID := int64(77), int64(78)
	authenticated := &service.APIKey{
		ID: 5, Key: "sk-auto", GroupID: &primaryID, RoutingMode: service.APIKeyRoutingModeAuto,
		AutoRouteGroupIDs: []int64{selectedID}, User: &service.User{ID: 9},
		Group: &service.Group{ID: primaryID, Platform: service.PlatformOpenAI, SubscriptionType: service.SubscriptionTypeStandard, RateMultiplier: 1},
	}
	selected := &service.Group{ID: selectedID, Platform: service.PlatformOpenAI, SubscriptionType: service.SubscriptionTypeStandard, RateMultiplier: 3}
	conn := cloneAPIKeyWithGroup(authenticated, selected)
	for _, test := range []struct {
		name        string
		mutateKey   func(*service.APIKey)
		mutateGroup func(*service.Group)
		lookupErr   bool
		groupErr    bool
		wantRefresh bool
	}{
		{name: "refreshes selected group not primary", wantRefresh: true},
		{name: "key lookup error", lookupErr: true},
		{name: "selected group lookup error", groupErr: true},
		{name: "key moved", mutateKey: func(key *service.APIKey) { id := int64(79); key.GroupID = &id; key.Group.ID = id }},
		{name: "different key", mutateKey: func(key *service.APIKey) { key.ID++ }},
		{name: "routing disabled", mutateKey: func(key *service.APIKey) { key.RoutingMode = service.APIKeyRoutingModeFixed }},
		{name: "selected candidate removed", mutateKey: func(key *service.APIKey) { key.AutoRouteGroupIDs = nil }},
		{name: "primary platform changed", mutateKey: func(key *service.APIKey) { key.Group.Platform = service.PlatformAnthropic }},
		{name: "primary billing type changed", mutateKey: func(key *service.APIKey) { key.Group.SubscriptionType = service.SubscriptionTypeSubscription }},
		{name: "selected platform changed", mutateGroup: func(group *service.Group) { group.Platform = service.PlatformAnthropic }},
		{name: "selected billing type changed", mutateGroup: func(group *service.Group) { group.SubscriptionType = service.SubscriptionTypeSubscription }},
		{name: "selected identity changed", mutateGroup: func(group *service.Group) { group.ID++ }},
	} {
		t.Run(test.name, func(t *testing.T) {
			latest := *authenticated
			primary := *authenticated.Group
			latest.Group = &primary
			if test.mutateKey != nil {
				test.mutateKey(&latest)
			}
			group := *selected
			group.RateMultiplier = 0.3
			if test.mutateGroup != nil {
				test.mutateGroup(&group)
			}
			lookup := wsTurnAPIKeyLookupFunc(func(context.Context, string) (*service.APIKey, error) {
				if test.lookupErr {
					return nil, errors.New("auth unavailable")
				}
				return &latest, nil
			})
			route := &openAIWSTurnBillingRoute{
				authenticatedKey: authenticated,
				lookupGroup: func(_ context.Context, id int64) (*service.Group, error) {
					require.Equal(t, selectedID, id, "a later turn must not resolve a new route")
					if test.groupErr {
						return nil, errors.New("group unavailable")
					}
					return &group, nil
				},
			}
			got := refreshOpenAIWSTurnBillingAPIKey(context.Background(), lookup, conn, route)
			if !test.wantRefresh {
				require.Same(t, conn, got)
				return
			}
			require.NotSame(t, conn, got)
			require.Equal(t, selectedID, *got.GroupID)
			require.Same(t, &group, got.Group)
			require.Same(t, conn.User, got.User)
			require.InDelta(t, 3, conn.Group.RateMultiplier, 1e-12)
		})
	}
}

func TestOpenAIWSTurnBillingAPIKeys_AutoRouteRefreshesBillingAndProfitContext(t *testing.T) {
	repo := newWSGroupPricingAPIKeyRepoStub(1)
	selectedID := wsGroupPricingGroupID + 1
	repo.apiKey.RoutingMode = service.APIKeyRoutingModeAuto
	repo.apiKey.AutoRouteGroupIDs = []int64{selectedID}
	keyService := newWSGroupPricingAPIKeyService(repo)
	authenticated, err := keyService.GetByKey(context.Background(), wsGroupPricingKey)
	require.NoError(t, err)
	selected := &service.Group{ID: selectedID, Platform: service.PlatformOpenAI, RateMultiplier: 3}
	conn := cloneAPIKeyWithGroup(authenticated, selected)
	latestGroup := *selected
	latestGroup.RateMultiplier = 0.3
	lookups := 0
	keys := openAIWSTurnBillingAPIKeys{route: &openAIWSTurnBillingRoute{
		authenticatedKey: authenticated,
		lookupGroup: func(_ context.Context, id int64) (*service.Group, error) {
			lookups++
			require.Equal(t, selectedID, id)
			return &latestGroup, nil
		},
	}}
	ctx := context.WithValue(context.Background(), ctxkey.Group, selected)
	firstCtx := keys.begin(ctx, keyService, 1, conn)
	require.Same(t, selected, firstCtx.Value(ctxkey.Group))
	require.Zero(t, lookups)
	secondCtx := keys.begin(ctx, keyService, 2, conn)
	require.Same(t, &latestGroup, secondCtx.Value(ctxkey.Group))
	require.Same(t, &latestGroup, keys.forTurn(2, conn).Group)
	require.Same(t, selected, keys.forTurn(1, conn).Group)
	require.Equal(t, 1, lookups)
}
