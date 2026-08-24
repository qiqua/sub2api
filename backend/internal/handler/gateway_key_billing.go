package handler

import (
	"net/http"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const keyBillingInfoSchemaVersion = 1

type keyBillingInfoResponse struct {
	Object                  string                   `json:"object"`
	SchemaVersion           int                      `json:"schema_version"`
	BillingScope            string                   `json:"billing_scope"`
	GroupRateMultiplier     float64                  `json:"group_rate_multiplier"`
	UserRateMultiplier      *float64                 `json:"user_rate_multiplier,omitempty"`
	ResolvedRateMultiplier  float64                  `json:"resolved_rate_multiplier"`
	PeakRateEnabled         bool                     `json:"peak_rate_enabled"`
	PeakStart               *string                  `json:"peak_start,omitempty"`
	PeakEnd                 *string                  `json:"peak_end,omitempty"`
	PeakRateMultiplier      *float64                 `json:"peak_rate_multiplier,omitempty"`
	AppliedPeakMultiplier   *float64                 `json:"applied_peak_multiplier,omitempty"`
	EffectiveRateMultiplier float64                  `json:"effective_rate_multiplier"`
	Timezone                *string                  `json:"timezone,omitempty"`
	ObservedAt              time.Time                `json:"observed_at"`
	Quota                   *keyBillingQuotaSnapshot `json:"quota,omitempty"`
}

type keyBillingQuotaSnapshot struct {
	Currency    string                         `json:"currency"`
	BillingMode string                         `json:"billing_mode,omitempty"`
	APIKey      keyBillingAPIKeyQuotaSnapshot  `json:"api_key"`
	UserBalance *keyBillingUserBalanceSnapshot `json:"user_balance,omitempty"`
	RateLimits  []keyBillingRateLimitSnapshot  `json:"rate_limits,omitempty"`
}

type keyBillingAPIKeyQuotaSnapshot struct {
	Limited   bool       `json:"limited"`
	Limit     *float64   `json:"limit,omitempty"`
	Used      float64    `json:"used"`
	Remaining *float64   `json:"remaining,omitempty"`
	Exhausted bool       `json:"exhausted"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Expired   bool       `json:"expired"`
}

type keyBillingUserBalanceSnapshot struct {
	Balance float64 `json:"balance"`
}

type keyBillingRateLimitSnapshot struct {
	Window      string     `json:"window"`
	Limit       float64    `json:"limit"`
	Used        float64    `json:"used"`
	Remaining   float64    `json:"remaining"`
	WindowStart *time.Time `json:"window_start,omitempty"`
	ResetAt     *time.Time `json:"reset_at,omitempty"`
}

// KeyBillingInfo returns the token billing multiplier effective for the authenticated API key.
// GET /v1/sub2api/billing
func (h *GatewayHandler) KeyBillingInfo(c *gin.Context) {
	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	if h.cfg != nil && h.cfg.RunMode == config.RunModeSimple {
		h.errorResponse(c, http.StatusNotFound, "not_found_error", "Billing information is not supported in simple mode")
		return
	}
	if apiKey.GroupID == nil {
		h.errorResponse(c, http.StatusForbidden, "permission_error", "API key is not assigned to a group")
		return
	}
	if apiKey.Group == nil {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "Billing information is unavailable")
		return
	}
	apiKey = h.refreshKeyBillingInfoAPIKey(c, apiKey)

	resolvedRate, ok := h.resolveKeyBillingRate(c, apiKey)
	if !ok {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "Billing information is unavailable")
		return
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, buildKeyBillingInfo(apiKey, resolvedRate, timezone.Now()))
}

func (h *GatewayHandler) refreshKeyBillingInfoAPIKey(c *gin.Context, apiKey *service.APIKey) *service.APIKey {
	if h == nil || h.apiKeyService == nil || apiKey == nil || apiKey.ID <= 0 {
		return apiKey
	}
	fresh, err := h.apiKeyService.GetByID(c.Request.Context(), apiKey.ID)
	if err != nil || fresh == nil || fresh.GroupID == nil || fresh.Group == nil {
		return apiKey
	}
	return fresh
}

func (h *GatewayHandler) resolveKeyBillingRate(c *gin.Context, apiKey *service.APIKey) (float64, bool) {
	groupRate := apiKey.Group.RateMultiplier
	switch apiKey.Group.Platform {
	case service.PlatformOpenAI, service.PlatformGrok:
		if h.openAIGatewayService == nil {
			return 0, false
		}
		return h.openAIGatewayService.ResolveUserGroupRateMultiplier(c.Request.Context(), apiKey.UserID, *apiKey.GroupID, groupRate), true
	default:
		if h.gatewayService == nil {
			return 0, false
		}
		return h.gatewayService.ResolveUserGroupRateMultiplier(c.Request.Context(), apiKey.UserID, *apiKey.GroupID, groupRate), true
	}
}

func buildKeyBillingInfo(apiKey *service.APIKey, resolvedRate float64, now time.Time) keyBillingInfoResponse {
	groupRate := apiKey.Group.RateMultiplier
	var userRate *float64
	if resolvedRate != groupRate {
		userRate = &resolvedRate
	}
	appliedPeak := apiKey.Group.PeakMultiplierAt(now)

	response := keyBillingInfoResponse{
		Object:                  "sub2api.key_billing",
		SchemaVersion:           keyBillingInfoSchemaVersion,
		BillingScope:            "token",
		GroupRateMultiplier:     groupRate,
		UserRateMultiplier:      userRate,
		ResolvedRateMultiplier:  resolvedRate,
		PeakRateEnabled:         apiKey.Group.PeakRateEnabled,
		EffectiveRateMultiplier: resolvedRate * appliedPeak,
		ObservedAt:              now.UTC(),
		Quota:                   buildKeyBillingQuotaSnapshot(apiKey, now),
	}
	if apiKey.Group.PeakRateEnabled {
		response.PeakStart = &apiKey.Group.PeakStart
		response.PeakEnd = &apiKey.Group.PeakEnd
		response.PeakRateMultiplier = &apiKey.Group.PeakRateMultiplier
		response.AppliedPeakMultiplier = &appliedPeak
		tz := timezone.Location().String()
		response.Timezone = &tz
	}
	return response
}

func buildKeyBillingQuotaSnapshot(apiKey *service.APIKey, now time.Time) *keyBillingQuotaSnapshot {
	if apiKey == nil {
		return nil
	}
	quota := &keyBillingQuotaSnapshot{
		Currency: "USD",
		APIKey: keyBillingAPIKeyQuotaSnapshot{
			Limited:   apiKey.Quota > 0,
			Used:      nonNegative(apiKey.QuotaUsed),
			Exhausted: apiKey.IsQuotaExhausted(),
			ExpiresAt: apiKey.ExpiresAt,
			Expired:   apiKey.ExpiresAt != nil && now.After(*apiKey.ExpiresAt),
		},
		RateLimits: buildKeyBillingRateLimitSnapshots(apiKey, now),
	}
	if apiKey.Group != nil && apiKey.Group.SubscriptionType != "" {
		quota.BillingMode = apiKey.Group.SubscriptionType
	}
	if apiKey.Quota > 0 {
		limit := nonNegative(apiKey.Quota)
		remaining := nonNegative(apiKey.GetQuotaRemaining())
		quota.APIKey.Limit = &limit
		quota.APIKey.Remaining = &remaining
	}
	if apiKey.User != nil && (apiKey.Group == nil || !apiKey.Group.IsSubscriptionType()) {
		quota.UserBalance = &keyBillingUserBalanceSnapshot{Balance: apiKey.User.Balance}
	}
	return quota
}

func buildKeyBillingRateLimitSnapshots(apiKey *service.APIKey, now time.Time) []keyBillingRateLimitSnapshot {
	if apiKey == nil {
		return nil
	}
	windows := make([]keyBillingRateLimitSnapshot, 0, 3)
	add := func(name string, limit float64, used float64, start *time.Time, duration time.Duration) {
		if limit <= 0 {
			return
		}
		effectiveUsed := nonNegative(used)
		var windowStart *time.Time
		var resetAt *time.Time
		if start != nil && now.Sub(*start) < duration {
			startCopy := start.UTC()
			resetCopy := start.Add(duration).UTC()
			windowStart = &startCopy
			resetAt = &resetCopy
		} else {
			effectiveUsed = 0
		}
		windows = append(windows, keyBillingRateLimitSnapshot{
			Window:      name,
			Limit:       nonNegative(limit),
			Used:        effectiveUsed,
			Remaining:   nonNegative(limit - effectiveUsed),
			WindowStart: windowStart,
			ResetAt:     resetAt,
		})
	}
	add("5h", apiKey.RateLimit5h, apiKey.Usage5h, apiKey.Window5hStart, service.RateLimitWindow5h)
	add("1d", apiKey.RateLimit1d, apiKey.Usage1d, apiKey.Window1dStart, service.RateLimitWindow1d)
	add("7d", apiKey.RateLimit7d, apiKey.Usage7d, apiKey.Window7dStart, service.RateLimitWindow7d)
	return windows
}

func nonNegative(value float64) float64 {
	if value < 0 {
		return 0
	}
	return value
}
