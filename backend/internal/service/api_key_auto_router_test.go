package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

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
	router := NewAPIKeyAutoRouter(nil, nil, nil)
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
	router := NewAPIKeyAutoRouter(groupRepo, accountRepo, capacityService)

	selected, decision, err := router.Resolve(context.Background(), &APIKey{
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
	router := NewAPIKeyAutoRouter(groupRepo, accountRepo, capacityService)

	selected, decision, err := router.Resolve(context.Background(), &APIKey{
		GroupID:           &groupID,
		RoutingMode:       APIKeyRoutingModeAuto,
		AutoRouteGroupIDs: []int64{2},
	}, "gpt-5.5")

	require.NoError(t, err)
	require.Nil(t, selected)
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
	router := NewAPIKeyAutoRouter(groupRepo, accountRepo, capacityService)

	selected, decision, err := router.Resolve(context.Background(), &APIKey{
		GroupID:           &groupID,
		RoutingMode:       APIKeyRoutingModeAuto,
		AutoRouteGroupIDs: []int64{2},
	}, "gpt-5.5")

	require.NoError(t, err)
	require.Nil(t, selected)
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
	router := NewAPIKeyAutoRouter(groupRepo, accountRepo, capacityService)

	selected, decision, err := router.Resolve(context.Background(), &APIKey{
		GroupID:           &groupID,
		RoutingMode:       APIKeyRoutingModeAuto,
		AutoRouteGroupIDs: []int64{2},
	}, "gpt-5.5", WithAPIKeyAutoRouteImageGenerationRequired())

	require.NoError(t, err)
	require.Nil(t, selected)
	require.True(t, decision.Enabled)
	require.False(t, decision.Routed)
	require.Equal(t, "primary_best", decision.Reason)
	require.Equal(t, int64(1), decision.SelectedGroupID)
}
