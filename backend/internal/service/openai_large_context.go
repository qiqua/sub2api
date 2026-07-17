package service

import (
	"context"
	"math"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

const (
	OpenAILargeContextTierNormal = "normal"
	OpenAILargeContextTierMedium = "medium"
	OpenAILargeContextTierLarge  = "large"
	OpenAILargeContextTierHuge   = "huge"
)

type openAILargeContextInfoKey struct{}

// OpenAILargeContextInfo describes request-size routing metadata.
// It is observational/scheduling metadata only: request bodies are still forwarded unchanged.
type OpenAILargeContextInfo struct {
	BodyBytes int
	Tier      string
	Endpoint  string
	Stream    bool
}

func WithOpenAILargeContextInfo(ctx context.Context, info OpenAILargeContextInfo) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	info.Tier = NormalizeOpenAILargeContextTier(info.Tier)
	return context.WithValue(ctx, openAILargeContextInfoKey{}, info)
}

func OpenAILargeContextInfoFromContext(ctx context.Context) (OpenAILargeContextInfo, bool) {
	if ctx == nil {
		return OpenAILargeContextInfo{}, false
	}
	info, ok := ctx.Value(openAILargeContextInfoKey{}).(OpenAILargeContextInfo)
	if !ok {
		return OpenAILargeContextInfo{}, false
	}
	info.Tier = NormalizeOpenAILargeContextTier(info.Tier)
	return info, true
}

func NewOpenAILargeContextInfo(cfg *config.Config, bodyBytes int, endpoint string, stream bool) OpenAILargeContextInfo {
	if bodyBytes < 0 {
		bodyBytes = 0
	}
	return OpenAILargeContextInfo{
		BodyBytes: bodyBytes,
		Tier:      ClassifyOpenAILargeContextTier(cfg, bodyBytes),
		Endpoint:  strings.TrimSpace(endpoint),
		Stream:    stream,
	}
}

func ClassifyOpenAILargeContextTier(cfg *config.Config, bodyBytes int) string {
	lc := effectiveOpenAILargeContextConfig(cfg)
	size := int64(bodyBytes)
	if size >= lc.HugeBytes {
		return OpenAILargeContextTierHuge
	}
	if size >= lc.LargeBytes {
		return OpenAILargeContextTierLarge
	}
	if size >= lc.MediumBytes {
		return OpenAILargeContextTierMedium
	}
	return OpenAILargeContextTierNormal
}

func NormalizeOpenAILargeContextTier(tier string) string {
	switch strings.ToLower(strings.TrimSpace(tier)) {
	case OpenAILargeContextTierHuge:
		return OpenAILargeContextTierHuge
	case OpenAILargeContextTierLarge:
		return OpenAILargeContextTierLarge
	case OpenAILargeContextTierMedium:
		return OpenAILargeContextTierMedium
	case OpenAILargeContextTierNormal, "":
		return OpenAILargeContextTierNormal
	default:
		return OpenAILargeContextTierNormal
	}
}

func IsOpenAILargeContextTierLarge(tier string) bool {
	tier = NormalizeOpenAILargeContextTier(tier)
	return tier == OpenAILargeContextTierLarge || tier == OpenAILargeContextTierHuge
}

func effectiveOpenAILargeContextConfig(cfg *config.Config) config.GatewayOpenAILargeContextConfig {
	lc := config.GatewayOpenAILargeContextConfig{
		Enabled:                  true,
		MediumBytes:              1 * 1024 * 1024,
		LargeBytes:               10 * 1024 * 1024,
		HugeBytes:                30 * 1024 * 1024,
		LargePoolBonus:           0.8,
		NonLargePoolPenalty:      0.4,
		NormalToLargePoolPenalty: 0.25,
	}
	if cfg != nil {
		custom := cfg.Gateway.OpenAIScheduler.LargeContext
		if !custom.Enabled &&
			custom.MediumBytes == 0 &&
			custom.LargeBytes == 0 &&
			custom.HugeBytes == 0 &&
			custom.LargePoolBonus == 0 &&
			custom.NonLargePoolPenalty == 0 &&
			custom.NormalToLargePoolPenalty == 0 {
			return lc
		}
		lc.Enabled = custom.Enabled
		if custom.MediumBytes > 0 {
			lc.MediumBytes = custom.MediumBytes
		}
		if custom.LargeBytes > lc.MediumBytes {
			lc.LargeBytes = custom.LargeBytes
		}
		if custom.HugeBytes > lc.LargeBytes {
			lc.HugeBytes = custom.HugeBytes
		}
		if custom.LargePoolBonus >= 0 && !math.IsNaN(custom.LargePoolBonus) && !math.IsInf(custom.LargePoolBonus, 0) {
			lc.LargePoolBonus = custom.LargePoolBonus
		}
		if custom.NonLargePoolPenalty >= 0 && !math.IsNaN(custom.NonLargePoolPenalty) && !math.IsInf(custom.NonLargePoolPenalty, 0) {
			lc.NonLargePoolPenalty = custom.NonLargePoolPenalty
		}
		if custom.NormalToLargePoolPenalty >= 0 && !math.IsNaN(custom.NormalToLargePoolPenalty) && !math.IsInf(custom.NormalToLargePoolPenalty, 0) {
			lc.NormalToLargePoolPenalty = custom.NormalToLargePoolPenalty
		}
	}
	return lc
}

func openAILargeContextRoutingEnabled(cfg *config.Config) bool {
	return effectiveOpenAILargeContextConfig(cfg).Enabled
}

func openAILargeContextScoreAdjustment(cfg *config.Config, req OpenAIAccountScheduleRequest, account *Account) float64 {
	if account == nil || !openAILargeContextRoutingEnabled(cfg) {
		return 0
	}
	if req.RequestBodyBytes <= 0 && strings.TrimSpace(req.LargeContextTier) == "" {
		return 0
	}

	lc := effectiveOpenAILargeContextConfig(cfg)
	tier := NormalizeOpenAILargeContextTier(req.LargeContextTier)
	pool := isOpenAILargeContextPoolAccount(account)

	switch tier {
	case OpenAILargeContextTierHuge, OpenAILargeContextTierLarge:
		if pool {
			return lc.LargePoolBonus
		}
		return -lc.NonLargePoolPenalty
	case OpenAILargeContextTierMedium:
		if pool {
			return lc.LargePoolBonus * 0.5
		}
		return -lc.NonLargePoolPenalty * 0.25
	case OpenAILargeContextTierNormal:
		if pool {
			return -lc.NormalToLargePoolPenalty
		}
	}
	return 0
}

func compareOpenAILargeContextPoolPreference(ctx context.Context, cfg *config.Config, left, right *Account) int {
	if left == nil || right == nil || !openAILargeContextRoutingEnabled(cfg) {
		return 0
	}
	info, ok := OpenAILargeContextInfoFromContext(ctx)
	if !ok || (info.BodyBytes <= 0 && strings.TrimSpace(info.Tier) == "") {
		return 0
	}
	tier := NormalizeOpenAILargeContextTier(info.Tier)
	leftPool := isOpenAILargeContextPoolAccount(left)
	rightPool := isOpenAILargeContextPoolAccount(right)
	if leftPool == rightPool {
		return 0
	}

	preferPool := tier == OpenAILargeContextTierMedium || tier == OpenAILargeContextTierLarge || tier == OpenAILargeContextTierHuge
	if preferPool {
		if leftPool {
			return -1
		}
		return 1
	}
	if leftPool {
		return 1
	}
	return -1
}

func isOpenAILargeContextPoolAccount(account *Account) bool {
	if account == nil || len(account.Extra) == 0 {
		return false
	}
	for _, key := range []string{"large_context_pool", "large_context", "large_context_dedicated"} {
		if truthyAccountExtraValue(account.Extra[key]) {
			return true
		}
	}
	return false
}

func truthyAccountExtraValue(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "t", "true", "yes", "y", "on", "large", "dedicated":
			return true
		default:
			parsed, err := strconv.ParseBool(strings.TrimSpace(v))
			return err == nil && parsed
		}
	case int:
		return v != 0
	case int8:
		return v != 0
	case int16:
		return v != 0
	case int32:
		return v != 0
	case int64:
		return v != 0
	case uint:
		return v != 0
	case uint8:
		return v != 0
	case uint16:
		return v != 0
	case uint32:
		return v != 0
	case uint64:
		return v != 0
	case float32:
		return v != 0
	case float64:
		return v != 0
	default:
		return false
	}
}
