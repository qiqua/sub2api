package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func annotateOpenAILargeContextRequest(c *gin.Context, reqLog *zap.Logger, cfg *config.Config, endpoint string, stream bool, body []byte) *zap.Logger {
	info := service.NewOpenAILargeContextInfo(cfg, len(body), endpoint, stream)
	if c != nil && c.Request != nil {
		c.Request = c.Request.WithContext(service.WithOpenAILargeContextInfo(c.Request.Context(), info))
	}
	fields := []zap.Field{
		zap.Int("request_body_bytes", info.BodyBytes),
		zap.String("large_context_tier", info.Tier),
		zap.String("large_context_endpoint", info.Endpoint),
	}
	if reqLog == nil {
		return reqLog
	}
	reqLog = reqLog.With(fields...)
	if info.Tier != service.OpenAILargeContextTierNormal {
		reqLog.Info("large_context.request_detected")
	}
	return reqLog
}
