package service

import (
	"context"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrAPIKeyInvalidRoutingMode = infraerrors.BadRequest("API_KEY_INVALID_ROUTING_MODE", "routing_mode must be fixed or auto")
	ErrAPIKeyAutoRouteNoGroup   = infraerrors.BadRequest("API_KEY_AUTO_ROUTE_NO_GROUP", "routing_mode=auto requires a primary group_id")
	ErrAPIKeyAutoRouteGroup     = infraerrors.BadRequest("API_KEY_AUTO_ROUTE_GROUP_INVALID", "auto_route_group_ids must be active groups on the same platform as group_id")
)

// NormalizeAPIKeyRoutingMode canonicalizes API key routing mode. Empty values keep
// the historical behavior: a key is pinned to its configured group.
func NormalizeAPIKeyRoutingMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", APIKeyRoutingModeFixed:
		return APIKeyRoutingModeFixed
	case APIKeyRoutingModeAuto:
		return APIKeyRoutingModeAuto
	default:
		return ""
	}
}

func NormalizeAPIKeyAutoRouteGroupIDs(ids []int64) []int64 {
	if len(ids) == 0 {
		return []int64{}
	}
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (s *APIKeyService) normalizeAndValidateAPIKeyRouting(ctx context.Context, user *User, groupID *int64, mode string, autoGroupIDs []int64) (string, []int64, error) {
	normalizedMode := NormalizeAPIKeyRoutingMode(mode)
	if normalizedMode == "" {
		return "", nil, ErrAPIKeyInvalidRoutingMode
	}
	ids := NormalizeAPIKeyAutoRouteGroupIDs(autoGroupIDs)
	if normalizedMode == APIKeyRoutingModeFixed {
		return APIKeyRoutingModeFixed, []int64{}, nil
	}
	if groupID == nil || *groupID <= 0 {
		return "", nil, ErrAPIKeyAutoRouteNoGroup
	}
	if s == nil || s.groupRepo == nil {
		return "", nil, ErrAPIKeyAutoRouteGroup
	}

	mainGroup, err := s.groupRepo.GetByID(ctx, *groupID)
	if err != nil {
		return "", nil, err
	}
	if mainGroup == nil || !mainGroup.IsActive() {
		return "", nil, ErrAPIKeyAutoRouteGroup
	}
	if user != nil && !s.canUserBindGroup(ctx, user, mainGroup) {
		return "", nil, ErrGroupNotAllowed
	}

	for _, id := range ids {
		group, err := s.groupRepo.GetByID(ctx, id)
		if err != nil {
			return "", nil, err
		}
		if group == nil || !group.IsActive() || group.Platform != mainGroup.Platform {
			return "", nil, ErrAPIKeyAutoRouteGroup
		}
		if user != nil && !s.canUserBindGroup(ctx, user, group) {
			return "", nil, ErrGroupNotAllowed
		}
	}
	return APIKeyRoutingModeAuto, ids, nil
}
