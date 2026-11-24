package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

type autoRouteUserRepo struct {
	UserRepository
	user *User
}

func (r autoRouteUserRepo) GetByID(_ context.Context, id int64) (*User, error) {
	if r.user == nil || r.user.ID != id {
		return nil, ErrUserNotFound
	}
	return r.user, nil
}

func autoRouteTestUser() autoRouteUserRepo {
	return autoRouteUserRepo{user: &User{ID: 7, Status: StatusActive, Balance: 10}}
}

func (r *apiKeyAutoRouteGroupRepoStub) GetByIDLite(ctx context.Context, id int64) (*Group, error) {
	return r.GetByID(ctx, id)
}

type apiKeyAutoRouteGroupRepoStub struct {
	GroupRepository
	groups map[int64]*Group
}

func (r *apiKeyAutoRouteGroupRepoStub) GetByID(_ context.Context, id int64) (*Group, error) {
	if r == nil {
		return nil, ErrGroupNotFound
	}
	group := r.groups[id]
	if group == nil {
		return nil, ErrGroupNotFound
	}
	return group, nil
}

type apiKeyAutoRouteAccountRepoStub struct {
	AccountRepository
	accounts map[int64][]Account
}

func (r *apiKeyAutoRouteAccountRepoStub) ListSchedulableByGroupID(_ context.Context, groupID int64) ([]Account, error) {
	if r == nil {
		return nil, nil
	}
	return append([]Account(nil), r.accounts[groupID]...), nil
}

func (r *apiKeyAutoRouteAccountRepoStub) ListSchedulableCapacityByGroupIDs(_ context.Context, groupIDs []int64) ([]GroupAccountCapacityRow, error) {
	rows := make([]GroupAccountCapacityRow, 0)
	if r == nil {
		return rows, nil
	}
	for _, groupID := range groupIDs {
		for _, account := range r.accounts[groupID] {
			rows = append(rows, GroupAccountCapacityRow{
				GroupID:     groupID,
				AccountID:   account.ID,
				Concurrency: account.Concurrency,
				LoadFactor:  account.LoadFactor,
				Extra:       account.Extra,
			})
		}
	}
	return rows, nil
}

func TestAPIKeyAutoRouterResolve_FixedDoesNotRoute(t *testing.T) {
	groupID := int64(1)
	router := NewAPIKeyAutoRouter(nil, nil, nil, nil, nil, nil, nil)
	group, decision, err := router.Resolve(context.Background(), &APIKey{
		GroupID:     &groupID,
		RoutingMode: APIKeyRoutingModeFixed,
	}, "gpt-5.5")

	require.NoError(t, err)
	require.Nil(t, group)
	require.False(t, decision.Enabled)
	require.False(t, decision.Routed)
	require.Equal(t, "fixed", decision.Reason)
}

func TestAPIKeyAutoRouterResolve_AutoSelectsSamePlatformGroupByLoadFactorCapacity(t *testing.T) {
	groupID := int64(1)
	group1Load := 1
	group2Load := 10
	groupRepo := &apiKeyAutoRouteGroupRepoStub{groups: map[int64]*Group{
		1: &Group{ID: 1, Platform: PlatformOpenAI, Status: StatusActive},
		2: &Group{ID: 2, Platform: PlatformOpenAI, Status: StatusActive},
	}}
	accountRepo := &apiKeyAutoRouteAccountRepoStub{accounts: map[int64][]Account{
		1: []Account{{ID: 11, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 50, LoadFactor: &group1Load}},
		2: []Account{{ID: 22, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, LoadFactor: &group2Load}},
	}}
	capacityService := NewGroupCapacityService(accountRepo, groupRepo, nil, nil, nil)
	router := NewAPIKeyAutoRouter(groupRepo, accountRepo, capacityService, autoRouteTestUser(), nil, nil, nil)

	selected, decision, err := router.Resolve(context.Background(), &APIKey{
		UserID:            7,
		GroupID:           &groupID,
		RoutingMode:       APIKeyRoutingModeAuto,
		AutoRouteGroupIDs: []int64{2},
	}, "gpt-5.5")

	require.NoError(t, err)
	require.NotNil(t, selected)
	require.True(t, decision.Enabled)
	require.True(t, decision.Routed)
	require.Equal(t, int64(2), selected.ID)
	require.Equal(t, 10, decision.Capacity.ConcurrencyMax, "group capacity must use load_factor before concurrency")
}

func TestAPIKeyAutoRouterResolve_AutoRejectsDifferentPlatformCandidate(t *testing.T) {
	groupID := int64(1)
	primaryLoad := 1
	otherPlatformLoad := 100
	groupRepo := &apiKeyAutoRouteGroupRepoStub{groups: map[int64]*Group{
		1: &Group{ID: 1, Platform: PlatformOpenAI, Status: StatusActive},
		2: &Group{ID: 2, Platform: PlatformGemini, Status: StatusActive},
	}}
	accountRepo := &apiKeyAutoRouteAccountRepoStub{accounts: map[int64][]Account{
		1: []Account{{ID: 11, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, LoadFactor: &primaryLoad}},
		2: []Account{{ID: 22, Platform: PlatformGemini, Status: StatusActive, Schedulable: true, Concurrency: 1, LoadFactor: &otherPlatformLoad}},
	}}
	capacityService := NewGroupCapacityService(accountRepo, groupRepo, nil, nil, nil)
	router := NewAPIKeyAutoRouter(groupRepo, accountRepo, capacityService, autoRouteTestUser(), nil, nil, nil)

	selected, decision, err := router.Resolve(context.Background(), &APIKey{
		UserID:            7,
		GroupID:           &groupID,
		RoutingMode:       APIKeyRoutingModeAuto,
		AutoRouteGroupIDs: []int64{2},
	}, "gpt-5.5")

	require.NoError(t, err)
	require.Same(t, groupRepo.groups[1], selected)
	require.True(t, decision.Enabled)
	require.False(t, decision.Routed)
	require.Equal(t, "primary_best", decision.Reason)
	require.Equal(t, int64(1), decision.SelectedGroupID)
}

func TestAPIKeyAutoRouterResolve_CapacityOnlyCountsRequestedModelAccounts(t *testing.T) {
	groupID := int64(1)
	primaryLoad := 5
	wrongModelLoad := 100
	matchingLoad := 1
	groupRepo := &apiKeyAutoRouteGroupRepoStub{groups: map[int64]*Group{
		1: &Group{ID: 1, Platform: PlatformOpenAI, Status: StatusActive},
		2: &Group{ID: 2, Platform: PlatformOpenAI, Status: StatusActive},
	}}
	accountRepo := &apiKeyAutoRouteAccountRepoStub{accounts: map[int64][]Account{
		1: []Account{{
			ID:          11,
			Platform:    PlatformOpenAI,
			Status:      StatusActive,
			Schedulable: true,
			Concurrency: 1,
			LoadFactor:  &primaryLoad,
			Credentials: map[string]any{"model_mapping": map[string]any{"gpt-5.5": "gpt-5.5"}},
		}},
		2: []Account{
			{
				ID:          22,
				Platform:    PlatformOpenAI,
				Status:      StatusActive,
				Schedulable: true,
				Concurrency: 1,
				LoadFactor:  &wrongModelLoad,
				Credentials: map[string]any{"model_mapping": map[string]any{"gpt-4.1": "gpt-4.1"}},
			},
			{
				ID:          23,
				Platform:    PlatformOpenAI,
				Status:      StatusActive,
				Schedulable: true,
				Concurrency: 1,
				LoadFactor:  &matchingLoad,
				Credentials: map[string]any{"model_mapping": map[string]any{"gpt-5.5": "gpt-5.5"}},
			},
		},
	}}
	capacityService := NewGroupCapacityService(accountRepo, groupRepo, nil, nil, nil)
	router := NewAPIKeyAutoRouter(groupRepo, accountRepo, capacityService, autoRouteTestUser(), nil, nil, nil)

	selected, decision, err := router.Resolve(context.Background(), &APIKey{
		UserID:            7,
		GroupID:           &groupID,
		RoutingMode:       APIKeyRoutingModeAuto,
		AutoRouteGroupIDs: []int64{2},
	}, "gpt-5.5")

	require.NoError(t, err)
	require.Same(t, groupRepo.groups[1], selected)
	require.True(t, decision.Enabled)
	require.False(t, decision.Routed)
	require.Equal(t, "primary_best", decision.Reason)
	require.Equal(t, int64(1), decision.SelectedGroupID)
	require.Equal(t, 5, decision.Capacity.ConcurrencyMax)
}

func TestAPIKeyAutoRouterResolve_ImageIntentRequiresImageEnabledGroup(t *testing.T) {
	groupID := int64(1)
	primaryLoad := 2
	nonImageLoad := 100
	groupRepo := &apiKeyAutoRouteGroupRepoStub{groups: map[int64]*Group{
		1: &Group{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, AllowImageGeneration: true},
		2: &Group{ID: 2, Platform: PlatformOpenAI, Status: StatusActive, AllowImageGeneration: false},
	}}
	accountRepo := &apiKeyAutoRouteAccountRepoStub{accounts: map[int64][]Account{
		1: []Account{{ID: 11, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, LoadFactor: &primaryLoad}},
		2: []Account{{ID: 22, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, LoadFactor: &nonImageLoad}},
	}}
	capacityService := NewGroupCapacityService(accountRepo, groupRepo, nil, nil, nil)
	router := NewAPIKeyAutoRouter(groupRepo, accountRepo, capacityService, autoRouteTestUser(), nil, nil, nil)

	selected, decision, err := router.Resolve(context.Background(), &APIKey{
		UserID:            7,
		GroupID:           &groupID,
		RoutingMode:       APIKeyRoutingModeAuto,
		AutoRouteGroupIDs: []int64{2},
	}, "gpt-5.5", WithAPIKeyAutoRouteImageGenerationRequired())

	require.NoError(t, err)
	require.Same(t, groupRepo.groups[1], selected)
	require.True(t, decision.Enabled)
	require.False(t, decision.Routed)
	require.Equal(t, "primary_best", decision.Reason)
	require.Equal(t, int64(1), decision.SelectedGroupID)
}

type autoRouteSubscriptionRepo struct {
	UserSubscriptionRepository
	byGroup map[int64]*UserSubscription
}

func (r autoRouteSubscriptionRepo) GetActiveByUserIDAndGroupID(_ context.Context, userID, groupID int64) (*UserSubscription, error) {
	sub := r.byGroup[groupID]
	if sub == nil || sub.UserID != userID {
		return nil, ErrSubscriptionNotFound
	}
	copy := *sub
	return &copy, nil
}

type autoRouteFixture struct {
	router   *APIKeyAutoRouter
	key      APIKey
	groups   map[int64]*Group
	accounts map[int64][]Account
	user     *User
	subs     map[int64]*UserSubscription
}

func newAutoRouteFixture() autoRouteFixture {
	f := autoRouteFixture{
		groups:   make(map[int64]*Group),
		accounts: make(map[int64][]Account),
		subs:     make(map[int64]*UserSubscription),
		user:     autoRouteTestUser().user,
	}
	now := time.Now()
	for _, id := range []int64{1, 2} {
		f.groups[id] = &Group{ID: id, Platform: PlatformOpenAI, Status: StatusActive}
		f.accounts[id] = []Account{{ID: id, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: int(id)}}
		f.subs[id] = &UserSubscription{ID: 100 + id, UserID: f.user.ID, GroupID: id, Status: SubscriptionStatusActive,
			StartsAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour), DailyWindowStart: &now, WeeklyWindowStart: &now, MonthlyWindowStart: &now}
	}
	g := &apiKeyAutoRouteGroupRepoStub{groups: f.groups}
	a := &apiKeyAutoRouteAccountRepoStub{accounts: f.accounts}
	subscriptions := &SubscriptionService{userSubRepo: autoRouteSubscriptionRepo{byGroup: f.subs}, now: time.Now}
	f.router = NewAPIKeyAutoRouter(g, a, NewGroupCapacityService(a, g, nil, nil, nil), autoRouteUserRepo{user: f.user}, subscriptions, nil, nil)
	f.key = APIKey{GroupID: &f.groups[1].ID, UserID: f.user.ID, RoutingMode: APIKeyRoutingModeAuto, AutoRouteGroupIDs: []int64{2}}
	return f
}

func TestAPIKeyAutoRouterResolve_BillingGrantMatchesSelectedGroup(t *testing.T) {
	for _, tc := range []struct {
		name         string
		primarySub   bool
		alternateSub bool
		balance      float64
		wantGroup    int64
		wantSub      int64
	}{
		{"both subscriptions", true, true, 0, 2, 102},
		{"balance to subscription", false, true, 1, 2, 102},
		{"subscription to balance", true, false, 1, 2, 0},
		{"unfunded balance candidate", true, false, 0, 1, 101},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newAutoRouteFixture()
			f.user.Balance = tc.balance
			if tc.primarySub {
				f.groups[1].SubscriptionType = SubscriptionTypeSubscription
			}
			if tc.alternateSub {
				f.groups[2].SubscriptionType = SubscriptionTypeSubscription
			}
			group, d, err := f.router.Resolve(context.Background(), &f.key, "gpt-5.5")
			require.NoError(t, err)
			require.Same(t, f.groups[tc.wantGroup], group)
			if tc.wantSub == 0 {
				require.Nil(t, d.Subscription)
			} else {
				require.NotNil(t, d.Subscription)
				require.Equal(t, tc.wantSub, d.Subscription.ID)
				require.Equal(t, group.ID, d.Subscription.GroupID)
			}
		})
	}
}

func TestAPIKeyAutoRouterResolve_RechecksPrimaryAndPinnedGrants(t *testing.T) {
	f := newAutoRouteFixture()
	f.key.User = &User{ID: f.user.ID, AllowedGroups: []int64{1, 2}}
	f.user.RestrictPublicGroups = true
	f.user.AllowedGroups = []int64{1}
	ctx := context.Background()
	group, d, err := f.router.Resolve(ctx, &f.key, "gpt-5.5")
	require.NoError(t, err)
	require.Equal(t, int64(1), group.ID)
	require.False(t, d.Routed)
	_, _, err = f.router.Resolve(ctx, &f.key, "gpt-5.5", WithAPIKeyAutoRoutePinnedGroup(2))
	require.ErrorIs(t, err, ErrGroupNotAllowed)
	f.user.AllowedGroups = []int64{1, 2, 3}
	_, _, err = f.router.Resolve(ctx, &f.key, "gpt-5.5", WithAPIKeyAutoRoutePinnedGroup(3))
	require.ErrorIs(t, err, ErrGroupNotAllowed)
	delete(f.accounts, 2)
	group, _, err = f.router.Resolve(ctx, &f.key, "gpt-5.5", WithAPIKeyAutoRoutePinnedGroup(2))
	require.NoError(t, err)
	require.Equal(t, int64(2), group.ID)
	f.user.AllowedGroups = []int64{2}
	_, _, err = f.router.Resolve(ctx, &f.key, "gpt-5.5", WithAPIKeyAutoRoutePinnedGroup(2))
	require.ErrorIs(t, err, ErrGroupNotAllowed)
}

func TestAPIKeyAutoRouterResolve_PublicModelsOnPrimaryAndSelected(t *testing.T) {
	f := newAutoRouteFixture()
	f.groups[1].ModelAllowlist = GroupModelAllowlist{Enabled: true, Models: []string{"public-a", "public-b"}}
	f.groups[2].ModelAllowlist = GroupModelAllowlist{Enabled: true, Models: []string{"public-a", "internal"}}
	public := WithAPIKeyAutoRoutePublicModels("public-a", "public-b")
	ctx := context.Background()
	group, _, err := f.router.Resolve(ctx, &f.key, "internal", public)
	require.NoError(t, err)
	require.Equal(t, int64(1), group.ID)
	f.groups[2].ModelAllowlist.Models = []string{"public-*"}
	group, _, err = f.router.Resolve(ctx, &f.key, "internal", public)
	require.NoError(t, err)
	require.Equal(t, int64(2), group.ID)
	f.groups[1].ModelAllowlist.Models = []string{"public-a"}
	_, _, err = f.router.Resolve(ctx, &f.key, "internal", public)
	require.ErrorIs(t, err, errAutoRouteModelDenied)
}

func TestAPIKeyAutoRouterResolve_SubscriptionRevocationAndLimits(t *testing.T) {
	f := newAutoRouteFixture()
	f.groups[2].SubscriptionType = SubscriptionTypeSubscription
	limit := 1.0
	f.groups[2].DailyLimitUSD = &limit
	f.subs[2].DailyUsageUSD = 5
	ctx := context.Background()
	group, _, err := f.router.Resolve(ctx, &f.key, "gpt-5.5")
	require.NoError(t, err)
	require.Equal(t, int64(1), group.ID)
	_, _, err = f.router.Resolve(ctx, &f.key, "gpt-5.5", WithAPIKeyAutoRoutePinnedGroup(2))
	require.ErrorIs(t, err, ErrDailyLimitExceeded)
	f.subs[2].Status = SubscriptionStatusSuspended
	_, _, err = f.router.Resolve(ctx, &f.key, "gpt-5.5", WithAPIKeyAutoRoutePinnedGroup(2))
	require.Error(t, err)
	f.groups[1].SubscriptionType = SubscriptionTypeSubscription
	delete(f.subs, 1)
	_, _, err = f.router.Resolve(ctx, &f.key, "gpt-5.5")
	require.Error(t, err)
}

func TestAPIKeyAutoRouterResolve_SimpleMode(t *testing.T) {
	f := newAutoRouteFixture()
	f.router.cfg = &config.Config{RunMode: config.RunModeSimple}
	f.groups[1].SubscriptionType = SubscriptionTypeSubscription
	f.groups[2].SubscriptionType = SubscriptionTypeSubscription
	delete(f.subs, 1)
	f.user.Balance = 0
	ctx := context.Background()
	group, d, err := f.router.Resolve(ctx, &f.key, "gpt-5.5")
	require.NoError(t, err)
	require.Equal(t, int64(2), group.ID)
	require.Nil(t, d.Subscription)
	delete(f.subs, 2)
	group, d, err = f.router.Resolve(ctx, &f.key, "gpt-5.5")
	require.NoError(t, err)
	require.Equal(t, int64(1), group.ID)
	require.Nil(t, d.Subscription)
	f.groups[2].SubscriptionType = ""
	f.groups[2].IsExclusive = true
	_, _, err = f.router.Resolve(ctx, &f.key, "gpt-5.5", WithAPIKeyAutoRoutePinnedGroup(2))
	require.ErrorIs(t, err, ErrGroupNotAllowed)
}

func TestAPIKeyAutoRouterResolve_ForcedPlatformAndVerifiedFallback(t *testing.T) {
	f := newAutoRouteFixture()
	f.accounts[1][0].Platform = PlatformAntigravity
	ctx := context.WithValue(context.Background(), ctxkey.ForcePlatform, PlatformAntigravity)
	group, d, err := f.router.Resolve(ctx, &f.key, "")
	require.NoError(t, err)
	require.Equal(t, int64(1), group.ID)
	require.Equal(t, 1, d.Capacity.ConcurrencyMax)
	delete(f.accounts, 1)
	group, d, err = f.router.Resolve(ctx, &f.key, "")
	require.NoError(t, err)
	require.Equal(t, int64(1), group.ID)
	require.Equal(t, "primary_fallback", d.Reason)
	f.user.RestrictPublicGroups = true
	_, _, err = f.router.Resolve(ctx, &f.key, "")
	require.ErrorIs(t, err, ErrGroupNotAllowed)
}

func TestAPIKeyAutoRouterResolve_CompositePreservesDispatchedRoute(t *testing.T) {
	f := newAutoRouteFixture()
	f.groups[1].Platform, f.groups[2].Platform = PlatformComposite, PlatformComposite
	routes := compositeRouteRepoStub{}
	for _, id := range []int64{1, 2} {
		routes.routes = append(routes.routes, CompositeModelRoute{GroupID: id, Enabled: true, MatchType: CompositeRouteMatchExact,
			PublicModel: "public-alias", UpstreamModel: "gpt-5.5", TargetPlatform: PlatformOpenAI, Endpoint: CompositeRouteEndpointResponses})
	}
	f.router.compositeRoutes = NewCompositeRouteResolver(&routes)
	ctx := WithCompositeRouteDecision(context.Background(), CompositeRouteDecision{
		Matched: true, GroupID: 1, TargetPlatform: PlatformOpenAI, PublicModel: "public-alias", UpstreamModel: "gpt-5.5",
	})
	endpoint := WithAPIKeyAutoRouteCompositeEndpoint(CompositeRouteEndpointResponses)
	group, d, err := f.router.Resolve(ctx, &f.key, "gpt-5.5", endpoint)
	require.NoError(t, err)
	require.Equal(t, int64(2), group.ID)
	require.Same(t, f.groups[1], d.PrimaryGroup)
	routes.routes[1].UpstreamModel = "another-upstream"
	group, _, err = f.router.Resolve(ctx, &f.key, "gpt-5.5", endpoint)
	require.NoError(t, err)
	require.Equal(t, int64(1), group.ID)
	_, _, err = f.router.Resolve(ctx, &f.key, "gpt-5.5", endpoint, WithAPIKeyAutoRoutePinnedGroup(2))
	require.ErrorIs(t, err, ErrGroupNotAllowed)
}

func TestAPIKeyAutoRouterResolve_DispatchAndConversionPolicy(t *testing.T) {
	f := newAutoRouteFixture()
	f.groups[1].AllowMessagesDispatch = true
	opts := WithAPIKeyAutoRouteMessagesDispatchRequired()
	ctx := context.Background()
	group, _, err := f.router.Resolve(ctx, &f.key, "gpt-5.5", opts)
	require.NoError(t, err)
	require.Equal(t, int64(1), group.ID)
	f.groups[2].AllowMessagesDispatch = true
	f.groups[1].MaxReasoningEffort = "low"
	group, _, err = f.router.Resolve(ctx, &f.key, "gpt-5.5", opts)
	require.NoError(t, err)
	require.Equal(t, int64(1), group.ID)
	f.groups[2].MaxReasoningEffort = "low"
	f.groups[1].ClaudeCodeOnly = true
	group, _, err = f.router.Resolve(ctx, &f.key, "gpt-5.5", opts)
	require.NoError(t, err)
	require.Equal(t, int64(1), group.ID)
	f.groups[2].ClaudeCodeOnly = true
	group, _, err = f.router.Resolve(ctx, &f.key, "gpt-5.5", opts)
	require.NoError(t, err)
	require.Equal(t, int64(2), group.ID)
}

func TestAPIKeyAutoRouterResolve_BatchCapabilityIndependentOfInteractiveImage(t *testing.T) {
	f := newAutoRouteFixture()
	for id, group := range f.groups {
		group.Platform = PlatformGemini
		group.AllowBatchImageGeneration = true
		f.accounts[id][0].Platform = PlatformGemini
		f.accounts[id][0].Type = AccountTypeAPIKey
	}
	f.accounts[1][0].Credentials = map[string]any{"api_key": "test"}
	batch := WithAPIKeyAutoRouteBatchImageRequired(BatchImageProviderGeminiAPI)
	ctx := context.Background()
	group, _, err := f.router.Resolve(ctx, &f.key, "", batch)
	require.NoError(t, err)
	require.Equal(t, int64(1), group.ID)
	f.accounts[2][0].Credentials = map[string]any{"api_key": "test"}
	group, _, err = f.router.Resolve(ctx, &f.key, "", batch)
	require.NoError(t, err)
	require.Equal(t, int64(2), group.ID)
	f.groups[2].SubscriptionType = SubscriptionTypeSubscription
	limit := 1.0
	f.groups[2].DailyLimitUSD = &limit
	f.subs[2].DailyUsageUSD = limit
	group, _, err = f.router.Resolve(ctx, &f.key, "", batch)
	require.NoError(t, err)
	require.Equal(t, int64(2), group.ID, "batch jobs use the prepaid wallet after group authorization")
	f.groups[2].AllowBatchImageGeneration = false
	group, _, err = f.router.Resolve(ctx, &f.key, "", batch)
	require.NoError(t, err)
	require.Equal(t, int64(1), group.ID)
	interactive := WithAPIKeyAutoRouteGeminiImageRequest([]byte(`{"generationConfig":{"responseModalities":["IMAGE"]}}`))
	_, _, err = f.router.Resolve(ctx, &f.key, "", interactive)
	require.Error(t, err)
	f.groups[1].AllowImageGeneration = true
	group, _, err = f.router.Resolve(ctx, &f.key, "", interactive)
	require.NoError(t, err)
	require.Equal(t, int64(1), group.ID)
}

// Simulate another request consuming the new window between activation and the mandatory readback.
type autoRouteWindowRepo struct{ autoRouteSubscriptionRepo }

func (r autoRouteWindowRepo) ActivateWindows(_ context.Context, id int64, dailyStart, periodicStart time.Time) error {
	for _, sub := range r.byGroup {
		if sub.ID == id {
			sub.DailyWindowStart, sub.WeeklyWindowStart, sub.MonthlyWindowStart = &dailyStart, &periodicStart, &periodicStart
			sub.DailyUsageUSD = 5
			return nil
		}
	}
	return ErrSubscriptionNotFound
}

func (r autoRouteWindowRepo) GetByID(_ context.Context, id int64) (*UserSubscription, error) {
	for _, sub := range r.byGroup {
		if sub.ID == id {
			return sub, nil
		}
	}
	return nil, ErrSubscriptionNotFound
}

func TestAPIKeyAutoRouterResolve_WindowReadbackCanRejectHighestCapacityGrant(t *testing.T) {
	f := newAutoRouteFixture()
	f.router.subscriptions.userSubRepo = autoRouteWindowRepo{autoRouteSubscriptionRepo{byGroup: f.subs}}
	f.groups[2].SubscriptionType = SubscriptionTypeSubscription
	limit := 1.0
	f.groups[2].DailyLimitUSD = &limit
	f.subs[2].DailyWindowStart, f.subs[2].WeeklyWindowStart, f.subs[2].MonthlyWindowStart = nil, nil, nil
	group, decision, err := f.router.Resolve(context.Background(), &f.key, "gpt-5.5")
	require.NoError(t, err)
	require.NotNil(t, f.subs[2].DailyWindowStart)
	require.Equal(t, int64(1), group.ID)
	require.Nil(t, decision.Subscription)
}

func TestAPIKeyAutoRouterResolve_IndeterminateSubscriptionGrant(t *testing.T) {
	f := newAutoRouteFixture()
	f.router.subscriptions = nil
	f.groups[2].SubscriptionType = SubscriptionTypeSubscription
	ctx := context.Background()
	group, _, err := f.router.Resolve(ctx, &f.key, "gpt-5.5")
	require.NoError(t, err)
	require.Equal(t, int64(1), group.ID)
	_, _, err = f.router.Resolve(ctx, &f.key, "gpt-5.5", WithAPIKeyAutoRoutePinnedGroup(2))
	require.ErrorIs(t, err, errAutoRouteUnavailable)
	f.groups[1].SubscriptionType = SubscriptionTypeSubscription
	_, _, err = f.router.Resolve(ctx, &f.key, "gpt-5.5")
	require.ErrorIs(t, err, errAutoRouteUnavailable)
}
