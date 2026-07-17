package service

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestEffectiveOpenAILargeContextConfig_ZeroConfigUsesDefaults(t *testing.T) {
	cfg := &config.Config{}

	got := effectiveOpenAILargeContextConfig(cfg)

	require.True(t, got.Enabled)
	require.Equal(t, int64(1*1024*1024), got.MediumBytes)
	require.Equal(t, int64(10*1024*1024), got.LargeBytes)
	require.Equal(t, int64(30*1024*1024), got.HugeBytes)
}

func TestEffectiveOpenAILargeContextConfig_CanExplicitlyDisable(t *testing.T) {
	cfg := &config.Config{}
	cfg.Gateway.OpenAIScheduler.LargeContext.Enabled = false
	cfg.Gateway.OpenAIScheduler.LargeContext.MediumBytes = 1 * 1024 * 1024
	cfg.Gateway.OpenAIScheduler.LargeContext.LargeBytes = 10 * 1024 * 1024
	cfg.Gateway.OpenAIScheduler.LargeContext.HugeBytes = 30 * 1024 * 1024
	cfg.Gateway.OpenAIScheduler.LargeContext.LargePoolBonus = 0.8
	cfg.Gateway.OpenAIScheduler.LargeContext.NonLargePoolPenalty = 0.4
	cfg.Gateway.OpenAIScheduler.LargeContext.NormalToLargePoolPenalty = 0.25

	got := effectiveOpenAILargeContextConfig(cfg)

	require.False(t, got.Enabled)
}
