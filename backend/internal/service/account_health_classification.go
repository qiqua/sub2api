package service

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

const (
	AccountHealthStatusAvailable   = "available"
	AccountHealthStatusRateLimited = "rate_limited"
	AccountHealthStatusUnavailable = "unavailable"
	AccountHealthStatusPending     = "pending"
	AccountHealthStatusChecking    = "checking"

	AccountHealthCategoryAvailable       = "available"
	AccountHealthCategoryRateLimited     = "rate_limited"
	AccountHealthCategoryQuotaExhausted  = "quota_exhausted"
	AccountHealthCategoryAuthInvalid     = "auth_invalid"
	AccountHealthCategoryPaymentRequired = "payment_required"
	AccountHealthCategoryProxyError      = "proxy_error"
	AccountHealthCategoryModelError      = "model_error"
	AccountHealthCategoryUpstreamError   = "upstream_error"
	AccountHealthCategoryConfigError     = "config_error"
	AccountHealthCategoryUnknownError    = "unknown_error"
)

var (
	accountHealthHTTPStatusRegex = regexp.MustCompile(`\b([1-5][0-9]{2})\b`)
	accountHealthErrorCodeRegex  = regexp.MustCompile(`\b[a-z][a-z0-9]+(?:_[a-z0-9]+)+\b`)
)

func ClassifyAccountHealthCheckError(testStatus, errMsg string) (status, category string, httpStatus int, errorCode string) {
	if strings.EqualFold(testStatus, "success") || strings.EqualFold(testStatus, AccountHealthStatusAvailable) {
		return AccountHealthStatusAvailable, AccountHealthCategoryAvailable, 0, ""
	}

	errMsg = strings.TrimSpace(errMsg)
	lower := strings.ToLower(errMsg)
	httpStatus = extractAccountHealthHTTPStatus(lower)
	errorCode = extractAccountHealthErrorCode(lower)

	if accountHealthContainsAny(lower, "insufficient_quota", "insufficient quota", "quota exhausted", "quota_exhausted", "quota exceeded", "quota_exceeded", "check quota", "usage limit", "usage_limit", "credit", "credits", "billing hard limit", "billing_hard_limit") {
		return AccountHealthStatusRateLimited, AccountHealthCategoryQuotaExhausted, httpStatus, errorCode
	}
	if httpStatus == http.StatusPaymentRequired || accountHealthContainsAny(lower, "payment required", "payment_required", "billing required", "billing_required") {
		return AccountHealthStatusUnavailable, AccountHealthCategoryPaymentRequired, httpStatus, errorCode
	}
	if httpStatus == http.StatusTooManyRequests || accountHealthContainsAny(lower, "rate_limit", "rate limit", "too many requests", "ratelimited", "rate-limited") {
		return AccountHealthStatusRateLimited, AccountHealthCategoryRateLimited, httpStatus, errorCode
	}
	if accountHealthContainsAny(lower, "no access token available", "no api key available", "missing refresh token", "missing api key", "api key is required", "credential is required", "credentials are required", "unsupported account", "unsupported platform") {
		return AccountHealthStatusUnavailable, AccountHealthCategoryConfigError, httpStatus, errorCode
	}
	if httpStatus == http.StatusUnauthorized || httpStatus == http.StatusForbidden || accountHealthContainsAny(lower, "invalid token", "invalid_refresh_token", "refresh_token_invalid", "refresh token invalid", "refresh_token_expired", "refresh token expired", "refresh_token_reused", "invalid_grant", "invalid_client", "invalid api key", "unauthorized", "unauthorized_client", "unauthenticated", "forbidden", "access_denied", "access denied", "authentication failed", "permission denied", "permission_denied") {
		return AccountHealthStatusUnavailable, AccountHealthCategoryAuthInvalid, httpStatus, errorCode
	}
	if accountHealthContainsAny(lower, "model not found", "unsupported model", "invalid model", "model_not_found") || (httpStatus == http.StatusNotFound && strings.Contains(lower, "model")) {
		return AccountHealthStatusUnavailable, AccountHealthCategoryModelError, httpStatus, errorCode
	}
	if accountHealthContainsAny(lower, "proxy", "connection refused", "connect: refused", "deadline exceeded", "timeout", "no such host", "tls", "certificate", "network is unreachable", "eof") {
		return AccountHealthStatusUnavailable, AccountHealthCategoryProxyError, httpStatus, errorCode
	}
	if httpStatus == http.StatusInternalServerError ||
		httpStatus == http.StatusBadGateway ||
		httpStatus == http.StatusServiceUnavailable ||
		httpStatus == http.StatusGatewayTimeout ||
		httpStatus == 529 ||
		accountHealthContainsAny(lower, "upstream", "server error", "temporarily unavailable", "overloaded", "bad gateway") {
		return AccountHealthStatusUnavailable, AccountHealthCategoryUpstreamError, httpStatus, errorCode
	}

	return AccountHealthStatusUnavailable, AccountHealthCategoryUnknownError, httpStatus, errorCode
}

func extractAccountHealthHTTPStatus(lowerErr string) int {
	match := accountHealthHTTPStatusRegex.FindStringSubmatch(lowerErr)
	if len(match) < 2 {
		return 0
	}
	status, err := strconv.Atoi(match[1])
	if err != nil {
		return 0
	}
	return status
}

func extractAccountHealthErrorCode(lowerErr string) string {
	codes := accountHealthErrorCodeRegex.FindAllString(lowerErr, -1)
	priorities := []string{
		"usage_limit_reached",
		"insufficient_quota",
		"quota_exhausted",
		"quota_exceeded",
		"billing_hard_limit_reached",
		"payment_required",
		"billing_required",
		"invalid_refresh_token",
		"refresh_token_invalid",
		"refresh_token_expired",
		"refresh_token_reused",
		"invalid_grant",
		"invalid_client",
		"unauthorized_client",
		"access_denied",
		"invalid_api_key",
		"model_not_found",
		"rate_limit_exceeded",
		"permission_denied",
	}
	for _, priority := range priorities {
		for _, code := range codes {
			if code == priority {
				return code
			}
		}
	}
	for _, code := range codes {
		switch code {
		case "access_token", "refresh_token", "api_key", "invalid_request_error", "resource_exhausted", "openai_oauth_token_refresh_failed":
			continue
		default:
			return code
		}
	}
	return ""
}

func accountHealthContainsAny(s string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}
