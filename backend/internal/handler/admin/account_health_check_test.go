package admin

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestNormalizeAccountHealthCheckConcurrency(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{name: "default is small server safe", in: 0, want: 2},
		{name: "negative uses default", in: -10, want: 2},
		{name: "explicit low value is preserved", in: 1, want: 1},
		{name: "max is accepted", in: 5, want: 5},
		{name: "large value is capped", in: 30, want: 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeAccountHealthCheckConcurrency(tt.in); got != tt.want {
				t.Fatalf("normalizeAccountHealthCheckConcurrency(%d) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizeAccountHealthCheckLimit(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{name: "default batch is small server safe", in: 0, want: 200},
		{name: "negative uses default", in: -10, want: 200},
		{name: "explicit low value is preserved", in: 50, want: 50},
		{name: "max is accepted", in: 500, want: 500},
		{name: "large value is capped", in: 10000, want: 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeAccountHealthCheckLimit(tt.in); got != tt.want {
				t.Fatalf("normalizeAccountHealthCheckLimit(%d) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestBuildAccountHealthCheckBatchSkipsCursorAndUnschedulable(t *testing.T) {
	accounts := []service.Account{
		{ID: 1, Name: "one", Schedulable: true},
		{ID: 2, Name: "two", Schedulable: false},
		{ID: 3, Name: "three", Schedulable: true},
		{ID: 4, Name: "four", Schedulable: true},
		{ID: 5, Name: "five", Schedulable: true},
	}

	got := buildAccountHealthCheckBatch(accounts, 2, 2, false)
	if len(got.Accounts) != 2 {
		t.Fatalf("batch size = %d, want 2", len(got.Accounts))
	}
	if got.Accounts[0].ID != 3 || got.Accounts[1].ID != 4 {
		t.Fatalf("batch account IDs = [%d,%d], want [3,4]", got.Accounts[0].ID, got.Accounts[1].ID)
	}
	if got.NextCursor != 4 {
		t.Fatalf("next cursor = %d, want 4", got.NextCursor)
	}
	if !got.HasMore {
		t.Fatal("has more = false, want true")
	}
}

func TestBuildAccountHealthCheckBatchIncludesUnschedulableWhenRequested(t *testing.T) {
	accounts := []service.Account{
		{ID: 1, Name: "one", Schedulable: true},
		{ID: 2, Name: "two", Schedulable: false},
		{ID: 3, Name: "three", Schedulable: true},
	}

	got := buildAccountHealthCheckBatch(accounts, 0, 3, true)
	if len(got.Accounts) != 3 {
		t.Fatalf("batch size = %d, want 3", len(got.Accounts))
	}
	if got.Accounts[1].ID != 2 {
		t.Fatalf("second account ID = %d, want unschedulable account 2", got.Accounts[1].ID)
	}
	if got.NextCursor != 3 {
		t.Fatalf("next cursor = %d, want 3", got.NextCursor)
	}
	if got.HasMore {
		t.Fatal("has more = true, want false")
	}
}

func TestClassifyAccountHealthCheckError(t *testing.T) {
	tests := []struct {
		name       string
		status     string
		errMsg     string
		wantStatus string
		wantCat    string
		wantHTTP   int
		wantCode   string
	}{
		{
			name:       "successful test is available",
			status:     AccountHealthStatusAvailable,
			wantStatus: AccountHealthStatusAvailable,
			wantCat:    AccountHealthCategoryAvailable,
		},
		{
			name:       "429 response is rate limited",
			status:     "failed",
			errMsg:     "API returned 429: rate_limit_exceeded",
			wantStatus: AccountHealthStatusRateLimited,
			wantCat:    AccountHealthCategoryRateLimited,
			wantHTTP:   429,
		},
		{
			name:       "401 response is auth invalid",
			status:     "failed",
			errMsg:     "Authentication failed (401): invalid token",
			wantStatus: AccountHealthStatusUnavailable,
			wantCat:    AccountHealthCategoryAuthInvalid,
			wantHTTP:   401,
		},
		{
			name:       "missing local token is config error",
			status:     "failed",
			errMsg:     "No access token available",
			wantStatus: AccountHealthStatusUnavailable,
			wantCat:    AccountHealthCategoryConfigError,
		},
		{
			name:       "quota errors are grouped separately",
			status:     "failed",
			errMsg:     "API returned 403: insufficient quota",
			wantStatus: AccountHealthStatusRateLimited,
			wantCat:    AccountHealthCategoryQuotaExhausted,
			wantHTTP:   403,
		},
		{
			name:       "429 usage limit reached is rate limited quota",
			status:     "failed",
			errMsg:     "Responses API returned 429: usage_limit_reached",
			wantStatus: AccountHealthStatusRateLimited,
			wantCat:    AccountHealthCategoryQuotaExhausted,
			wantHTTP:   429,
			wantCode:   "usage_limit_reached",
		},
		{
			name:       "usage limit message without status is rate limited quota",
			status:     "failed",
			errMsg:     "The usage limit has been reached",
			wantStatus: AccountHealthStatusRateLimited,
			wantCat:    AccountHealthCategoryQuotaExhausted,
		},
		{
			name:       "resource exhausted check quota is rate limited quota",
			status:     "failed",
			errMsg:     `API returned 429: {"error":{"code":429,"message":"Resource has been exhausted (e.g. check quota).","status":"RESOURCE_EXHAUSTED"}}`,
			wantStatus: AccountHealthStatusRateLimited,
			wantCat:    AccountHealthCategoryQuotaExhausted,
			wantHTTP:   429,
		},
		{
			name:       "invalid refresh token is auth invalid",
			status:     "failed",
			errMsg:     `OPENAI_OAUTH_TOKEN_REFRESH_FAILED: token refresh failed: status 400, body: {"error":{"type":"invalid_request_error","code":"invalid_refresh_token"}}`,
			wantStatus: AccountHealthStatusUnavailable,
			wantCat:    AccountHealthCategoryAuthInvalid,
			wantHTTP:   400,
			wantCode:   "invalid_refresh_token",
		},
		{
			name:       "invalid grant is auth invalid",
			status:     "failed",
			errMsg:     "token refresh failed: invalid_grant: token revoked",
			wantStatus: AccountHealthStatusUnavailable,
			wantCat:    AccountHealthCategoryAuthInvalid,
			wantCode:   "invalid_grant",
		},
		{
			name:       "missing api key is config error",
			status:     "failed",
			errMsg:     "No API key available",
			wantStatus: AccountHealthStatusUnavailable,
			wantCat:    AccountHealthCategoryConfigError,
		},
		{
			name:       "resource exhausted with rate limit reason is rate limited",
			status:     "failed",
			errMsg:     `API returned 429: {"error":{"status":"RESOURCE_EXHAUSTED","details":[{"reason":"RATE_LIMIT_EXCEEDED"}]}}`,
			wantStatus: AccountHealthStatusRateLimited,
			wantCat:    AccountHealthCategoryRateLimited,
			wantHTTP:   429,
			wantCode:   "rate_limit_exceeded",
		},
		{
			name:       "model errors are grouped separately",
			status:     "failed",
			errMsg:     "API returned 404: model not found",
			wantStatus: AccountHealthStatusUnavailable,
			wantCat:    AccountHealthCategoryModelError,
			wantHTTP:   404,
		},
		{
			name:       "transport failures are proxy errors",
			status:     "failed",
			errMsg:     "Post https://example.invalid: proxy connection refused",
			wantStatus: AccountHealthStatusUnavailable,
			wantCat:    AccountHealthCategoryProxyError,
		},
		{
			name:       "5xx responses are upstream errors",
			status:     "failed",
			errMsg:     "Responses API returned 503",
			wantStatus: AccountHealthStatusUnavailable,
			wantCat:    AccountHealthCategoryUpstreamError,
			wantHTTP:   503,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStatus, gotCat, gotHTTP, gotCode := classifyAccountHealthCheckError(tt.status, tt.errMsg)
			if gotStatus != tt.wantStatus {
				t.Fatalf("status = %q, want %q", gotStatus, tt.wantStatus)
			}
			if gotCat != tt.wantCat {
				t.Fatalf("category = %q, want %q", gotCat, tt.wantCat)
			}
			if gotHTTP != tt.wantHTTP {
				t.Fatalf("http status = %d, want %d", gotHTTP, tt.wantHTTP)
			}
			if gotCode != tt.wantCode {
				t.Fatalf("error code = %q, want %q", gotCode, tt.wantCode)
			}
		})
	}
}
