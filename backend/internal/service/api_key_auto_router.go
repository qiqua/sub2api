package service

import (
	"context"
	"math"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type APIKeyAutoRouteDecision struct {
	Enabled           bool
	Routed            bool
	Reason            string
	PrimaryGroupID    int64
	PrimaryGroup      *Group
	SelectedGroupID   int64
	CandidateGroupIDs []int64
	Capacity          GroupCapacitySummary
	Subscription      *UserSubscription
}

type APIKeyAutoRouteOptions struct {
	RequireImageGeneration  bool
	GeminiImageIntent       bool
	RequireBatchImage       bool
	BatchImageProvider      string
	RequireMessagesDispatch bool
	PublicModels            []string
	PinnedGroup             *int64
	CompositeEndpoint       string
}

type APIKeyAutoRouteOption func(*APIKeyAutoRouteOptions)

func WithAPIKeyAutoRouteImageGenerationRequired() APIKeyAutoRouteOption {
	return func(opts *APIKeyAutoRouteOptions) {
		if opts != nil {
			opts.RequireImageGeneration = true
		}
	}
}

func WithAPIKeyAutoRouteGeminiImageRequest(body []byte, models ...string) APIKeyAutoRouteOption {
	return func(opts *APIKeyAutoRouteOptions) {
		opts.GeminiImageIntent = opts.GeminiImageIntent || isGeminiImageGenerationIntent(body, models...)
	}
}

// WithAPIKeyAutoRoutePublicModels supplies client-visible names when scheduling uses a rewritten model.
func WithAPIKeyAutoRoutePublicModels(models ...string) APIKeyAutoRouteOption {
	return func(opts *APIKeyAutoRouteOptions) {
		opts.PublicModels = opts.PublicModels[:0]
		for _, model := range models {
			if model = strings.TrimSpace(model); model != "" {
				opts.PublicModels = append(opts.PublicModels, model)
			}
		}
	}
}

func WithAPIKeyAutoRoutePinnedGroup(id int64) APIKeyAutoRouteOption {
	return func(opts *APIKeyAutoRouteOptions) { opts.PinnedGroup = &id }
}

func WithAPIKeyAutoRouteMessagesDispatchRequired() APIKeyAutoRouteOption {
	return func(opts *APIKeyAutoRouteOptions) { opts.RequireMessagesDispatch = true }
}

func WithAPIKeyAutoRouteCompositeEndpoint(endpoint string) APIKeyAutoRouteOption {
	return func(opts *APIKeyAutoRouteOptions) { opts.CompositeEndpoint = endpoint }
}

func WithAPIKeyAutoRouteBatchImageRequired(provider ...string) APIKeyAutoRouteOption {
	return func(opts *APIKeyAutoRouteOptions) {
		opts.RequireBatchImage = true
		if len(provider) > 0 {
			opts.BatchImageProvider = strings.TrimSpace(provider[0])
		}
	}
}

var (
	errAutoRouteUnavailable = infraerrors.ServiceUnavailable("AUTO_ROUTE_UNAVAILABLE", "unable to verify API key route")
	errAutoRouteModelDenied = infraerrors.Forbidden("MODEL_NOT_ALLOWED", "model is not allowed for this group")
	errAutoRouteBalance     = infraerrors.Forbidden("INSUFFICIENT_BALANCE", "insufficient account balance")
)

type APIKeyAutoRouter struct {
	groupRepo       GroupRepository
	accountRepo     AccountRepository
	capacityService *GroupCapacityService
	userRepo        UserRepository
	subscriptions   *SubscriptionService
	cfg             *config.Config
	compositeRoutes *CompositeRouteResolver
}

func NewAPIKeyAutoRouter(groupRepo GroupRepository, accountRepo AccountRepository, capacityService *GroupCapacityService, userRepo UserRepository, subscriptions *SubscriptionService, cfg *config.Config, compositeRoutes *CompositeRouteResolver) *APIKeyAutoRouter {
	return &APIKeyAutoRouter{
		groupRepo:       groupRepo,
		accountRepo:     accountRepo,
		capacityService: capacityService,
		userRepo:        userRepo,
		subscriptions:   subscriptions,
		cfg:             cfg,
		compositeRoutes: compositeRoutes,
	}
}

// Resolve returns the verified group and its billing subscription for every successful auto route, including the primary.
func (r *APIKeyAutoRouter) Resolve(ctx context.Context, apiKey *APIKey, requestedModel string, options ...APIKeyAutoRouteOption) (*Group, APIKeyAutoRouteDecision, error) {
	decision := APIKeyAutoRouteDecision{}
	routeOptions := APIKeyAutoRouteOptions{}
	for _, option := range options {
		if option != nil {
			option(&routeOptions)
		}
	}
	if apiKey == nil || NormalizeAPIKeyRoutingMode(apiKey.RoutingMode) != APIKeyRoutingModeAuto {
		if pin := routeOptions.PinnedGroup; pin != nil && (apiKey == nil || apiKey.GroupID == nil || *pin != *apiKey.GroupID) {
			return nil, decision, ErrGroupNotAllowed
		}
		decision.Reason = "fixed"
		return nil, decision, nil
	}
	decision.Enabled = true
	if apiKey.GroupID == nil || *apiKey.GroupID <= 0 || apiKey.UserID <= 0 {
		decision.Reason = "missing_primary_grant"
		return nil, decision, ErrGroupNotAllowed
	}
	primaryID := *apiKey.GroupID
	decision.PrimaryGroupID = primaryID
	if r == nil || r.groupRepo == nil || r.userRepo == nil {
		return nil, decision, errAutoRouteUnavailable
	}
	user, err := r.userRepo.GetByID(ctx, apiKey.UserID)
	if err != nil {
		return nil, decision, autoRouteLookupError(err)
	}
	if user == nil || user.ID != apiKey.UserID || !user.IsActive() || user.DeletedAt != nil {
		return nil, decision, ErrGroupNotAllowed
	}
	primary, err := r.groupRepo.GetByIDLite(ctx, primaryID)
	if err != nil {
		return nil, decision, autoRouteLookupError(err)
	}
	if primary == nil || primary.ID != primaryID || !primary.IsActive() {
		return nil, decision, ErrGroupNotAllowed
	}
	// Protocol dispatch and request conversion may already have run using the authenticated group.
	if authenticated := apiKey.Group; authenticated != nil && (authenticated.ID != primaryID || !autoRoutePoliciesCompatible(ctx, authenticated, primary)) {
		return nil, decision, ErrGroupNotAllowed
	}
	decision.PrimaryGroup = primary
	if len(routeOptions.PublicModels) == 0 {
		public := requestedModel
		if original, ok := RequestedPublicModelFromContext(ctx); ok {
			public = original
		}
		routeOptions.PublicModels = []string{public}
	}
	primaryGrant, err := r.grant(ctx, user, primary, true, requestedModel, routeOptions)
	if err == nil {
		err = r.maintainGrant(ctx, &primaryGrant)
	}
	if err != nil {
		decision.Reason = "primary_grant_denied"
		return nil, decision, err
	}
	candidateIDs := NormalizeAPIKeyAutoRouteGroupIDs(append([]int64{primaryID}, apiKey.AutoRouteGroupIDs...))
	decision.CandidateGroupIDs = candidateIDs
	if pin := routeOptions.PinnedGroup; pin != nil && !slices.Contains(candidateIDs, *pin) {
		return nil, decision, ErrGroupNotAllowed
	}
	grants := make([]autoRouteGrant, 0, len(candidateIDs))
	accountsByGroup := make(map[int64][]Account, len(candidateIDs))
	for _, id := range candidateIDs {
		if pin := routeOptions.PinnedGroup; pin != nil && id != *pin {
			continue
		}
		grant := primaryGrant
		if id != primaryID {
			group, lookupErr := r.groupRepo.GetByIDLite(ctx, id)
			switch {
			case lookupErr != nil:
				err = autoRouteLookupError(lookupErr)
			case group == nil || group.ID != id || !group.IsActive() || !autoRoutePoliciesCompatible(ctx, primary, group):
				err = ErrGroupNotAllowed
			default:
				grant, err = r.grant(ctx, user, group, false, requestedModel, routeOptions)
			}
			if err != nil {
				if routeOptions.PinnedGroup != nil {
					return nil, decision, err
				}
				continue
			}
		}
		if routeOptions.PinnedGroup != nil {
			if err := r.maintainGrant(ctx, &grant); err != nil {
				return nil, decision, err
			}
			decision.selectGrant(grant, "pinned")
			return grant.group, decision, nil
		}
		accounts := r.schedulableModelCandidates(ctx, grant, routeOptions)
		if len(accounts) > 0 {
			accountsByGroup[id] = accounts
			grants = append(grants, grant)
		}
	}
	capacity := r.capacityService.summarizeCapacitiesFromAccounts(ctx, accountsByGroup)
	sort.SliceStable(grants, func(i, j int) bool {
		left, right := grants[i].group, grants[j].group
		diff := apiKeyAutoRouteGroupScore(left, capacity[left.ID]) - apiKeyAutoRouteGroupScore(right, capacity[right.ID])
		if math.Abs(diff) > 0.000001 {
			return diff > 0
		}
		if left.ID == primaryID || right.ID == primaryID {
			return left.ID == primaryID
		}
		return left.ID < right.ID
	})
	for _, grant := range grants {
		if err := r.maintainGrant(ctx, &grant); err != nil {
			continue
		}
		reason := "auto"
		if grant.group.ID == primaryID {
			reason = "primary_best"
		}
		decision.selectGrant(grant, reason)
		decision.Capacity = capacity[grant.group.ID]
		return grant.group, decision, nil
	}
	// The downstream scheduler may have a broader view of account availability.
	decision.selectGrant(primaryGrant, "primary_fallback")
	return primary, decision, nil
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

// Preserve restrictions that downstream scheduling and request conversion apply to just one group.
func autoRoutePoliciesCompatible(ctx context.Context, primary, candidate *Group) bool {
	if primary.Platform != candidate.Platform {
		return false
	}
	if (primary.RequirePrivacySet && !candidate.RequirePrivacySet) || (primary.RequireOAuthOnly && !candidate.RequireOAuthOnly) {
		return false
	}
	if forced, _ := ctx.Value(ctxkey.ForcePlatform).(string); forced == "" && (primary.ClaudeCodeOnly || candidate.ClaudeCodeOnly) {
		if primary.ClaudeCodeOnly != candidate.ClaudeCodeOnly || derefGroupID(primary.FallbackGroupID) != derefGroupID(candidate.FallbackGroupID) {
			return false
		}
	}
	limit := NormalizeMaxReasoningEffort(primary.MaxReasoningEffort)
	if limit != NormalizeMaxReasoningEffort(candidate.MaxReasoningEffort) || !slices.Equal(primary.ReasoningEffortMappings, candidate.ReasoningEffortMappings) {
		return false
	}
	return limit == "" || NormalizeMaxReasoningEffortOverLimit(primary.MaxReasoningEffortOverLimit) == NormalizeMaxReasoningEffortOverLimit(candidate.MaxReasoningEffortOverLimit)
}

type autoRouteGrant struct {
	group            *Group
	subscription     *UserSubscription
	needsMaintenance bool
	platform         string
	model            string
}

func (d *APIKeyAutoRouteDecision) selectGrant(grant autoRouteGrant, reason string) {
	d.SelectedGroupID = grant.group.ID
	d.Routed = d.SelectedGroupID != d.PrimaryGroupID
	d.Subscription = grant.subscription
	d.Reason = reason
}

func (r *APIKeyAutoRouter) grant(ctx context.Context, user *User, group *Group, primary bool, model string, opts APIKeyAutoRouteOptions) (autoRouteGrant, error) {
	grant := autoRouteGrant{group: group, platform: group.Platform, model: model}
	for _, public := range opts.PublicModels {
		if !group.ModelAllowlist.Allows(public) {
			return grant, errAutoRouteModelDenied
		}
	}
	if forced, _ := ctx.Value(ctxkey.ForcePlatform).(string); forced != "" {
		grant.platform = forced
	} else if group.Platform == PlatformComposite && model != "" {
		public := model
		if name, ok := RequestedPublicModelFromContext(ctx); ok {
			public = name
		}
		if r.compositeRoutes == nil {
			return grant, errAutoRouteUnavailable
		}
		route, err := r.compositeRoutes.Resolve(ctx, group.ID, public, opts.CompositeEndpoint)
		if err != nil {
			return grant, autoRouteLookupError(err)
		}
		if !route.Matched {
			return grant, ErrGroupNotAllowed
		}
		if platform, ok := ResolvedTargetPlatformFromContext(ctx); ok && route.TargetPlatform != platform {
			return grant, ErrGroupNotAllowed
		}
		if upstream, ok := ResolvedUpstreamModelFromContext(ctx); ok && route.UpstreamModel != upstream {
			return grant, ErrGroupNotAllowed
		}
		grant.platform, grant.model = route.TargetPlatform, route.UpstreamModel
	}
	if opts.RequireMessagesDispatch && !group.AllowMessagesDispatch && grant.platform != PlatformGrok && !IsMultiProtocolAPIKeyProvider(grant.platform) {
		return grant, ErrGroupNotAllowed
	}
	if opts.RequireBatchImage && (group.Platform != PlatformGemini || !group.AllowBatchImageGeneration) {
		return grant, ErrBatchImageGroupDisabled
	}
	geminiGroup := group.Platform == PlatformGemini || group.Platform == PlatformAntigravity || group.Platform == PlatformComposite
	if (opts.RequireImageGeneration || IsGPTImageGenerationModel(model) || (geminiGroup && opts.GeminiImageIntent)) && !GroupAllowsImageGeneration(group) {
		return grant, infraerrors.Forbidden("IMAGE_GENERATION_DISABLED", ImageGenerationPermissionMessage())
	}
	simple := r.cfg != nil && r.cfg.RunMode == config.RunModeSimple
	if simple && primary && group.IsSubscriptionType() {
		return grant, nil
	}
	var subs UserSubscriptionRepository
	if r.subscriptions != nil {
		subs = r.subscriptions.userSubRepo
	}
	sub, err := apiKeyGroupBindGrant(ctx, user, group, subs)
	if err != nil {
		return grant, err
	}
	if !simple && sub == nil && user.Balance <= 0 {
		return grant, errAutoRouteBalance
	}
	if !simple && sub != nil {
		grant.subscription = sub
		// The batch service reserves wallet funds. On an alternate group the active
		// subscription grants access, but its token usage limits do not fund the job.
		if opts.RequireBatchImage && !primary {
			return grant, nil
		}
		grant.needsMaintenance, err = r.subscriptions.ValidateAndCheckLimits(sub, group)
	}
	return grant, err
}

// apiKeyGroupBindGrant is shared by key configuration and request admission. A standard
// grant has no subscription; a subscription grant identifies the row to charge.
func apiKeyGroupBindGrant(ctx context.Context, user *User, group *Group, subs UserSubscriptionRepository) (*UserSubscription, error) {
	if !group.IsSubscriptionType() {
		if user.CanBindGroup(group.ID, group.IsExclusive) {
			return nil, nil
		}
		return nil, ErrGroupNotAllowed
	}
	if subs == nil {
		return nil, errAutoRouteUnavailable
	}
	sub, err := subs.GetActiveByUserIDAndGroupID(ctx, user.ID, group.ID)
	if err != nil {
		return nil, autoRouteLookupError(err)
	}
	if !validAutoRouteSubscription(sub, user.ID, group.ID) {
		return nil, ErrGroupNotAllowed
	}
	return sub, nil
}

func autoRouteLookupError(err error) error {
	if infraerrors.IsNotFound(err) {
		return ErrGroupNotAllowed.WithCause(err)
	}
	return errAutoRouteUnavailable.WithCause(err)
}

func validAutoRouteSubscription(sub *UserSubscription, userID, groupID int64) bool {
	return sub != nil && sub.ID > 0 && sub.UserID == userID && sub.GroupID == groupID && sub.DeletedAt == nil && !sub.StartsAt.After(time.Now()) && sub.IsActive()
}

func (r *APIKeyAutoRouter) maintainGrant(ctx context.Context, grant *autoRouteGrant) error {
	if !grant.needsMaintenance {
		return nil
	}
	sub, err := r.subscriptions.EnsureWindowMaintenance(ctx, grant.subscription)
	if err != nil {
		return autoRouteLookupError(err)
	}
	if !validAutoRouteSubscription(sub, grant.subscription.UserID, grant.group.ID) || sub.ID != grant.subscription.ID {
		return ErrGroupNotAllowed
	}
	grant.subscription = sub
	grant.needsMaintenance, err = r.subscriptions.ValidateAndCheckLimits(sub, grant.group)
	if grant.needsMaintenance && err == nil {
		return errAutoRouteUnavailable
	}
	return err
}

func (r *APIKeyAutoRouter) schedulableModelCandidates(ctx context.Context, grant autoRouteGrant, opts APIKeyAutoRouteOptions) []Account {
	if r.accountRepo == nil {
		return nil
	}
	accounts, err := r.accountRepo.ListSchedulableByGroupID(ctx, grant.group.ID)
	if err != nil {
		return nil
	}
	forced, _ := ctx.Value(ctxkey.ForcePlatform).(string)
	mixed := forced == "" && (grant.platform == PlatformAnthropic || grant.platform == PlatformGemini)
	compositeListing := grant.platform == PlatformComposite && grant.model == ""
	out := make([]Account, 0, len(accounts))
	for i := range accounts {
		acc := &accounts[i]
		if !acc.IsSchedulable() || (grant.group.RequirePrivacySet && !acc.IsPrivacySet()) {
			continue
		}
		if grant.model != "" && (!acc.IsModelSupported(grant.model) || acc.isModelRateLimitedWithContext(ctx, grant.model)) {
			continue
		}
		if !compositeListing && acc.Platform != grant.platform && !(mixed && acc.Platform == PlatformAntigravity && acc.IsMixedSchedulingEnabled()) {
			continue
		}
		if opts.RequireBatchImage {
			provider := opts.BatchImageProvider
			gemini := (provider == "" || provider == BatchImageProviderGeminiAPI) && (&GeminiAPIBatchImageProvider{}).SupportsAccount(acc)
			vertex := (provider == "" || provider == BatchImageProviderVertex) && (&VertexBatchImageProvider{}).SupportsAccount(acc)
			if !gemini && !vertex {
				continue
			}
		}
		out = append(out, *acc)
	}
	return out
}
