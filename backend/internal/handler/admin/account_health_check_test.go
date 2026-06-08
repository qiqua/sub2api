package admin

import "testing"

func TestClassifyAccountHealthCheckError(t *testing.T) {
	tests := []struct {
		name       string
		status     string
		errMsg     string
		wantStatus string
		wantCat    string
		wantHTTP   int
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
			wantStatus: AccountHealthStatusUnavailable,
			wantCat:    AccountHealthCategoryQuotaExhausted,
			wantHTTP:   403,
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
			gotStatus, gotCat, gotHTTP, _ := classifyAccountHealthCheckError(tt.status, tt.errMsg)
			if gotStatus != tt.wantStatus {
				t.Fatalf("status = %q, want %q", gotStatus, tt.wantStatus)
			}
			if gotCat != tt.wantCat {
				t.Fatalf("category = %q, want %q", gotCat, tt.wantCat)
			}
			if gotHTTP != tt.wantHTTP {
				t.Fatalf("http status = %d, want %d", gotHTTP, tt.wantHTTP)
			}
		})
	}
}
