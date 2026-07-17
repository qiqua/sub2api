package service

import (
	"context"
	"math"
	"sort"
)

type APIKeyAutoRouteDecision struct {
	Enabled           bool
	Routed            bool
	Reason            string
	PrimaryGroupID    int64
	SelectedGroupID   int64
	CandidateGroupIDs []int64
	Capacity          GroupCapacitySummary
}

type APIKeyAutoRouteOptions struct {
	RequireImageGeneration bool
}

type APIKeyAutoRouteOption func(*APIKeyAutoRouteOptions)

func WithAPIKeyAutoRouteImageGenerationRequired() APIKeyAutoRouteOption {
	return func(opts *APIKeyAutoRouteOptions) {
		if opts != nil {
			opts.RequireImageGeneration = true
		}
	}
}

type APIKeyAutoRouter struct {
	groupRepo       GroupRepository
	accountRepo     AccountRepository
	capacityService *GroupCapacityService
}

func NewAPIKeyAutoRouter(groupRepo GroupRepository, accountRepo AccountRepository, capacityService *GroupCapacityService) *APIKeyAutoRouter {
	return &APIKeyAutoRouter{
		groupRepo:       groupRepo,
		accountRepo:     accountRepo,
		capacityService: capacityService,
	}
}

func (r *APIKeyAutoRouter) Resolve(ctx context.Context, apiKey *APIKey, requestedModel string, options ...APIKeyAutoRouteOption) (*Group, APIKeyAutoRouteDecision, error) {
	decision := APIKeyAutoRouteDecision{}
	routeOptions := APIKeyAutoRouteOptions{}
	for _, option := range options {
		if option != nil {
			option(&routeOptions)
		}
	}
	if r == nil || apiKey == nil || apiKey.GroupID == nil || *apiKey.GroupID <= 0 {
		decision.Reason = "missing_primary_group"
		return nil, decision, nil
	}
	primaryID := *apiKey.GroupID
	decision.PrimaryGroupID = primaryID
	if NormalizeAPIKeyRoutingMode(apiKey.RoutingMode) != APIKeyRoutingModeAuto {
		decision.Reason = "fixed"
		return nil, decision, nil
	}
	decision.Enabled = true
	if r.groupRepo == nil || r.accountRepo == nil || r.capacityService == nil {
		decision.Reason = "router_unavailable"
		return nil, decision, nil
	}

	primaryGroup, err := r.groupRepo.GetByID(ctx, primaryID)
	if err != nil || primaryGroup == nil || !primaryGroup.IsActive() {
		decision.Reason = "primary_group_unavailable"
		return nil, decision, err
	}

	candidateIDs := NormalizeAPIKeyAutoRouteGroupIDs(append([]int64{primaryID}, apiKey.AutoRouteGroupIDs...))
	decision.CandidateGroupIDs = candidateIDs
	if len(candidateIDs) == 0 {
		decision.Reason = "no_candidates"
		return nil, decision, nil
	}

	groups := make([]*Group, 0, len(candidateIDs))
	eligibleAccountsByGroup := make(map[int64][]Account, len(candidateIDs))
	for _, groupID := range candidateIDs {
		group, groupErr := r.groupRepo.GetByID(ctx, groupID)
		if groupErr != nil || group == nil || !group.IsActive() {
			continue
		}
		if group.Platform != primaryGroup.Platform {
			continue
		}
		accounts := r.schedulableModelCandidates(ctx, group, requestedModel, routeOptions)
		if len(accounts) == 0 {
			continue
		}
		groups = append(groups, group)
		eligibleAccountsByGroup[group.ID] = accounts
	}
	if len(groups) == 0 {
		decision.Reason = "no_usable_candidates"
		return nil, decision, nil
	}

	capacityMap := r.capacityService.summarizeCapacitiesFromAccounts(ctx, eligibleAccountsByGroup)

	sort.SliceStable(groups, func(i, j int) bool {
		left := apiKeyAutoRouteGroupScore(groups[i], capacityMap[groups[i].ID])
		right := apiKeyAutoRouteGroupScore(groups[j], capacityMap[groups[j].ID])
		if math.Abs(left-right) > 0.000001 {
			return left > right
		}
		if groups[i].ID == primaryID {
			return true
		}
		if groups[j].ID == primaryID {
			return false
		}
		return groups[i].ID < groups[j].ID
	})

	selected := groups[0]
	decision.SelectedGroupID = selected.ID
	decision.Capacity = capacityMap[selected.ID]
	if selected.ID == primaryID {
		decision.Reason = "primary_best"
		return nil, decision, nil
	}
	decision.Routed = true
	decision.Reason = "auto"
	return selected, decision, nil
}

func apiKeyAutoRouteGroupScore(group *Group, capacity GroupCapacitySummary) float64 {
	if group == nil {
		return -1
	}
	maxConcurrency := capacity.ConcurrencyMax
	if maxConcurrency <= 0 {
		return 0
	}
	usedConcurrency := capacity.ConcurrencyUsed
	freeConcurrency := maxConcurrency - usedConcurrency
	if freeConcurrency < 0 {
		freeConcurrency = 0
	}
	loadFactor := 1 - clamp01(float64(usedConcurrency)/float64(maxConcurrency))
	score := float64(freeConcurrency)*100 + loadFactor*10

	if capacity.SessionsMax > 0 {
		freeSessions := capacity.SessionsMax - capacity.SessionsUsed
		if freeSessions < 0 {
			freeSessions = 0
		}
		score += float64(freeSessions) + (1-clamp01(float64(capacity.SessionsUsed)/float64(capacity.SessionsMax)))*5
	}
	if capacity.RPMMax > 0 {
		freeRPM := capacity.RPMMax - capacity.RPMUsed
		if freeRPM < 0 {
			freeRPM = 0
		}
		score += float64(freeRPM) / 10
		score += (1 - clamp01(float64(capacity.RPMUsed)/float64(capacity.RPMMax))) * 5
	}
	return score
}

func (r *APIKeyAutoRouter) schedulableModelCandidates(ctx context.Context, group *Group, requestedModel string, options APIKeyAutoRouteOptions) []Account {
	if r == nil || r.accountRepo == nil || group == nil || group.ID <= 0 {
		return nil
	}
	if (options.RequireImageGeneration || IsGPTImageGenerationModel(requestedModel)) && !GroupAllowsImageGeneration(group) {
		return nil
	}
	accounts, err := r.accountRepo.ListSchedulableByGroupID(ctx, group.ID)
	if err != nil || len(accounts) == 0 {
		return nil
	}
	out := make([]Account, 0, len(accounts))
	for i := range accounts {
		acc := &accounts[i]
		if !acc.IsSchedulable() {
			continue
		}
		if acc.Platform != group.Platform && !(acc.IsMixedSchedulingEnabled() && (group.Platform == PlatformAnthropic || group.Platform == PlatformGemini)) {
			continue
		}
		if requestedModel == "" || acc.IsModelSupported(requestedModel) {
			out = append(out, *acc)
		}
	}
	return out
}

func (r *APIKeyAutoRouter) groupHasSchedulableModelCandidate(ctx context.Context, group *Group, requestedModel string) bool {
	return len(r.schedulableModelCandidates(ctx, group, requestedModel, APIKeyAutoRouteOptions{})) > 0
}
