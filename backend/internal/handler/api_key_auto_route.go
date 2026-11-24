package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const autoRoutePrimaryGroupKey = "api_key_auto_route_primary_group"

func autoRoutePrimaryModelAllowlist(c *gin.Context) service.GroupModelAllowlist {
	value, _ := c.Get(autoRoutePrimaryGroupKey)
	if primary, ok := value.(*service.Group); ok && primary != nil {
		return primary.ModelAllowlist
	}
	return service.GroupModelAllowlist{}
}

func applyAPIKeyAutoRoute(c *gin.Context, router *service.APIKeyAutoRouter, reqLog *zap.Logger, apiKey *service.APIKey, requestedModel string, options ...service.APIKeyAutoRouteOption) *service.APIKey {
	routed, err := resolveAPIKeyAutoRoute(c, router, reqLog, apiKey, requestedModel, options...)
	if err == nil {
		return routed
	}
	status, message := infraerrors.Code(err), infraerrors.Message(err)
	if status >= 500 {
		message = "Unable to verify API key routing permissions"
	}
	c.Abort()
	switch path := c.Request.URL.Path; {
	case strings.Contains(path, "/v1beta/"):
		middleware2.GoogleErrorWriter(c, status, message)
	case strings.Contains(path, "/messages"):
		middleware2.AnthropicErrorWriter(c, status, message)
	default:
		c.JSON(status, gin.H{"error": gin.H{"type": "api_error", "message": message}})
	}
	return nil
}

// resolveAPIKeyAutoRoute installs the same decision for scheduling, header expansion and settlement.
// WebSocket callers translate errors into a close frame after the HTTP exchange is upgraded.
func resolveAPIKeyAutoRoute(c *gin.Context, router *service.APIKeyAutoRouter, reqLog *zap.Logger, apiKey *service.APIKey, requestedModel string, options ...service.APIKeyAutoRouteOption) (*service.APIKey, error) {
	if c == nil || c.Request == nil || apiKey == nil {
		return nil, infraerrors.ServiceUnavailable("AUTO_ROUTE_UNAVAILABLE", "Automatic group routing is unavailable")
	}
	if service.NormalizeAPIKeyRoutingMode(apiKey.RoutingMode) != service.APIKeyRoutingModeAuto {
		// Resolve also checks an explicit worker/turn pin when the current key is fixed.
		_, _, err := router.Resolve(c.Request.Context(), apiKey, requestedModel, options...)
		if err != nil {
			return nil, err
		}
		return apiKey, nil
	}
	publicModels := middleware2.AutoRoutePublicModels(c)
	if len(publicModels) == 0 {
		publicModels = []string{clientRequestedModel(c, requestedModel)}
	}
	// More specific options (e.g. a WebSocket turn's original frame) take precedence.
	opts := []service.APIKeyAutoRouteOption{service.WithAPIKeyAutoRoutePublicModels(publicModels...)}
	if apiKey.Group != nil && apiKey.Group.Platform == service.PlatformComposite {
		opts = append(opts, service.WithAPIKeyAutoRouteCompositeEndpoint(service.CompositeRouteEndpointForPath(c.Request.URL.Path)))
	}
	group, decision, err := router.Resolve(c.Request.Context(), apiKey, requestedModel, append(opts, options...)...)
	if err != nil {
		if reqLog != nil {
			reqLog.Warn("api_key.auto_route.resolve_failed",
				zap.Int64("api_key_id", apiKey.ID),
				zap.Int64("primary_group_id", decision.PrimaryGroupID),
				zap.String("reason", decision.Reason),
				zap.Error(err),
			)
		}
		return nil, err
	}
	if !decision.Enabled || group == nil || group.ID != decision.SelectedGroupID {
		return nil, infraerrors.New(http.StatusForbidden, "AUTO_ROUTE_DENIED", "No eligible group is available for this API key")
	}
	if reqLog != nil {
		reqLog.Info("api_key.auto_route.decision",
			zap.Int64("api_key_id", apiKey.ID),
			zap.Bool("routed", decision.Routed),
			zap.String("reason", decision.Reason),
			zap.Int64("primary_group_id", decision.PrimaryGroupID),
			zap.Int64("selected_group_id", decision.SelectedGroupID),
			zap.Any("candidate_group_ids", decision.CandidateGroupIDs),
			zap.Int("selected_concurrency_used", decision.Capacity.ConcurrencyUsed),
			zap.Int("selected_concurrency_max", decision.Capacity.ConcurrencyMax),
		)
	}
	if sub := decision.Subscription; sub != nil && (sub.GroupID != group.ID || sub.UserID != apiKey.UserID || !group.IsSubscriptionType()) {
		return nil, infraerrors.ServiceUnavailable("AUTO_ROUTE_BILLING_MISMATCH", "Unable to verify the selected group's subscription")
	}
	routed := cloneAPIKeyWithGroup(apiKey, group)
	if decision.Routed && requestedModel == "" {
		c.Set(autoRoutePrimaryGroupKey, decision.PrimaryGroup)
	}
	c.Set(string(middleware2.ContextKeyAPIKey), routed)
	// Setting an untyped nil clears the primary subscription for a balance-billed route.
	if decision.Subscription == nil {
		c.Set(string(middleware2.ContextKeySubscription), nil)
	} else {
		c.Set(string(middleware2.ContextKeySubscription), decision.Subscription)
	}
	ctx := context.WithValue(c.Request.Context(), ctxkey.Group, group)
	ctx = service.WithHeaderOverrideRequestScope(ctx, routed.UserID, routed.ID, routed.GroupID)
	c.Request = c.Request.WithContext(ctx)
	return routed, nil
}
