package handler

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func applyAPIKeyAutoRoute(c *gin.Context, router *service.APIKeyAutoRouter, reqLog *zap.Logger, apiKey *service.APIKey, requestedModel string, options ...service.APIKeyAutoRouteOption) *service.APIKey {
	if c == nil || router == nil || apiKey == nil {
		return apiKey
	}
	group, decision, err := router.Resolve(c.Request.Context(), apiKey, requestedModel, options...)
	if err != nil {
		if reqLog != nil {
			reqLog.Warn("api_key.auto_route.resolve_failed",
				zap.Int64("api_key_id", apiKey.ID),
				zap.Int64("primary_group_id", decision.PrimaryGroupID),
				zap.String("reason", decision.Reason),
				zap.Error(err),
			)
		}
		return apiKey
	}
	if !decision.Enabled {
		return apiKey
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
	if !decision.Routed || group == nil {
		return apiKey
	}
	routed := cloneAPIKeyWithGroup(apiKey, group)
	c.Set(string(middleware2.ContextKeyAPIKey), routed)
	if service.IsGroupContextValid(group) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.Group, group))
	}
	return routed
}
