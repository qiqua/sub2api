package admin

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
)

const (
	AccountHealthStatusAvailable   = "available"
	AccountHealthStatusRateLimited = "rate_limited"
	AccountHealthStatusUnavailable = "unavailable"
	AccountHealthStatusPending     = "pending"
	AccountHealthStatusChecking    = "checking"

	AccountHealthCategoryAvailable      = "available"
	AccountHealthCategoryRateLimited    = "rate_limited"
	AccountHealthCategoryQuotaExhausted = "quota_exhausted"
	AccountHealthCategoryAuthInvalid    = "auth_invalid"
	AccountHealthCategoryProxyError     = "proxy_error"
	AccountHealthCategoryModelError     = "model_error"
	AccountHealthCategoryUpstreamError  = "upstream_error"
	AccountHealthCategoryConfigError    = "config_error"
	AccountHealthCategoryUnknownError   = "unknown_error"

	accountHealthJobStatusQueued    = "queued"
	accountHealthJobStatusRunning   = "running"
	accountHealthJobStatusCompleted = "completed"
	accountHealthJobStatusCanceled  = "canceled"
	accountHealthJobStatusFailed    = "failed"
)

const (
	defaultAccountHealthCheckConcurrency = 2
	maxAccountHealthCheckConcurrency     = 5
	defaultAccountHealthCheckLimit       = 200
	maxAccountHealthCheckLimit           = 500
	maxAccountHealthCheckJobs            = 30
	accountHealthCheckPageSize           = 1000
)

var (
	accountHealthCheckJobs       = newAccountHealthCheckJobStore(maxAccountHealthCheckJobs)
	accountHealthHTTPStatusRegex = regexp.MustCompile(`\b([1-5][0-9]{2})\b`)
	accountHealthErrorCodeRegex  = regexp.MustCompile(`\b[a-z][a-z0-9]+(?:_[a-z0-9]+)+\b`)
)

type AccountHealthCheckJobRequest struct {
	AccountIDs             []int64                   `json:"account_ids"`
	Filters                *BulkUpdateAccountFilters `json:"filters"`
	ModelID                string                    `json:"model_id"`
	Model                  string                    `json:"model"`
	Concurrency            int                       `json:"concurrency"`
	Limit                  int                       `json:"limit"`
	Cursor                 int64                     `json:"cursor"`
	IncludeUnschedulable   *bool                     `json:"include_unschedulable"`
}

type AccountHealthCheckSummary struct {
	Total       int            `json:"total"`
	Pending     int            `json:"pending"`
	Checking    int            `json:"checking"`
	Available   int            `json:"available"`
	RateLimited int            `json:"rate_limited"`
	Unavailable int            `json:"unavailable"`
	ByCategory  map[string]int `json:"by_category"`
}

type AccountHealthCheckJob struct {
	ID         string                    `json:"id"`
	Status     string                    `json:"status"`
	Summary    AccountHealthCheckSummary `json:"summary"`
	Limit      int                       `json:"limit"`
	Cursor     int64                     `json:"cursor"`
	NextCursor int64                     `json:"next_cursor"`
	HasMore    bool                      `json:"has_more"`
	Error      string                    `json:"error,omitempty"`
	CreatedAt  time.Time                 `json:"created_at"`
	StartedAt  *time.Time                `json:"started_at,omitempty"`
	FinishedAt *time.Time                `json:"finished_at,omitempty"`
}

type AccountHealthCheckResult struct {
	AccountID  int64      `json:"account_id"`
	Name       string     `json:"name"`
	Platform   string     `json:"platform"`
	Type       string     `json:"type"`
	Status     string     `json:"status"`
	Category   string     `json:"category"`
	HTTPStatus int        `json:"http_status,omitempty"`
	ErrorCode  string     `json:"error_code,omitempty"`
	Message    string     `json:"message,omitempty"`
	LatencyMs  int64      `json:"latency_ms,omitempty"`
	CheckedAt  *time.Time `json:"checked_at,omitempty"`
}

type accountHealthCheckJobState struct {
	job     AccountHealthCheckJob
	results map[int64]AccountHealthCheckResult
	order   []int64
	cancel  context.CancelFunc
}

type accountHealthCheckBatch struct {
	Accounts   []service.Account
	Cursor     int64
	Limit      int
	NextCursor int64
	HasMore    bool
}

type accountHealthCheckJobStore struct {
	mu      sync.RWMutex
	maxJobs int
	jobs    map[string]*accountHealthCheckJobState
	order   []string
}

func newAccountHealthCheckJobStore(maxJobs int) *accountHealthCheckJobStore {
	return &accountHealthCheckJobStore{
		maxJobs: maxJobs,
		jobs:    make(map[string]*accountHealthCheckJobState),
		order:   make([]string, 0, maxJobs),
	}
}

func (s *accountHealthCheckJobStore) create(batch accountHealthCheckBatch) AccountHealthCheckJob {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	jobID := fmt.Sprintf("%d", now.UnixNano())
	state := &accountHealthCheckJobState{
		job: AccountHealthCheckJob{
			ID:         jobID,
			Status:     accountHealthJobStatusQueued,
			Limit:      batch.Limit,
			Cursor:     batch.Cursor,
			NextCursor: batch.Cursor,
			HasMore:    batch.HasMore,
			CreatedAt:  now,
		},
		results: make(map[int64]AccountHealthCheckResult, len(batch.Accounts)),
		order:   make([]int64, 0, len(batch.Accounts)),
	}
	for _, account := range batch.Accounts {
		if _, exists := state.results[account.ID]; exists {
			continue
		}
		state.order = append(state.order, account.ID)
		state.results[account.ID] = AccountHealthCheckResult{
			AccountID: account.ID,
			Name:      account.Name,
			Platform:  account.Platform,
			Type:      account.Type,
			Status:    AccountHealthStatusPending,
		}
	}
	state.job.Summary = summarizeAccountHealthCheckResultsLocked(state)

	s.jobs[jobID] = state
	s.order = append(s.order, jobID)
	s.pruneLocked()
	return state.job
}

func (s *accountHealthCheckJobStore) setCancel(jobID string, cancel context.CancelFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if state, ok := s.jobs[jobID]; ok {
		state.cancel = cancel
	}
}

func (s *accountHealthCheckJobStore) get(jobID string) (AccountHealthCheckJob, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, ok := s.jobs[jobID]
	if !ok {
		return AccountHealthCheckJob{}, false
	}
	return state.job, true
}

func (s *accountHealthCheckJobStore) listResults(jobID, status, category string) ([]AccountHealthCheckResult, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, ok := s.jobs[jobID]
	if !ok {
		return nil, false
	}

	results := make([]AccountHealthCheckResult, 0, len(state.order))
	for _, accountID := range state.order {
		item, exists := state.results[accountID]
		if !exists {
			continue
		}
		if status != "" && item.Status != status {
			continue
		}
		if category != "" && item.Category != category {
			continue
		}
		results = append(results, item)
	}
	return results, true
}

func (s *accountHealthCheckJobStore) markStarted(jobID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if state, ok := s.jobs[jobID]; ok {
		if state.job.Status == accountHealthJobStatusCanceled {
			return
		}
		now := time.Now()
		state.job.Status = accountHealthJobStatusRunning
		state.job.StartedAt = &now
	}
}

func (s *accountHealthCheckJobStore) updateResult(jobID string, result AccountHealthCheckResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.jobs[jobID]
	if !ok {
		return
	}
	if state.job.Status == accountHealthJobStatusCanceled {
		return
	}
	state.results[result.AccountID] = result
	advanceAccountHealthCheckCursorLocked(state)
	state.job.Summary = summarizeAccountHealthCheckResultsLocked(state)
}

func (s *accountHealthCheckJobStore) finish(jobID, status, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.jobs[jobID]
	if !ok {
		return
	}
	if state.job.Status == accountHealthJobStatusCanceled && status == accountHealthJobStatusCompleted {
		return
	}
	now := time.Now()
	state.job.Status = status
	state.job.Error = errMsg
	state.job.FinishedAt = &now
	advanceAccountHealthCheckCursorLocked(state)
	state.job.Summary = summarizeAccountHealthCheckResultsLocked(state)
	if status == accountHealthJobStatusCanceled && (state.job.Summary.Pending > 0 || state.job.Summary.Checking > 0) {
		state.job.HasMore = true
	}
}

func (s *accountHealthCheckJobStore) cancel(jobID string) (AccountHealthCheckJob, bool) {
	s.mu.Lock()
	state, ok := s.jobs[jobID]
	if !ok {
		s.mu.Unlock()
		return AccountHealthCheckJob{}, false
	}
	if state.job.Status == accountHealthJobStatusCompleted || state.job.Status == accountHealthJobStatusFailed {
		job := state.job
		s.mu.Unlock()
		return job, true
	}
	cancel := state.cancel
	now := time.Now()
	state.job.Status = accountHealthJobStatusCanceled
	state.job.FinishedAt = &now
	advanceAccountHealthCheckCursorLocked(state)
	state.job.Summary = summarizeAccountHealthCheckResultsLocked(state)
	if state.job.Summary.Pending > 0 || state.job.Summary.Checking > 0 {
		state.job.HasMore = true
	}
	job := state.job
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	return job, true
}

func (s *accountHealthCheckJobStore) pruneLocked() {
	for s.maxJobs > 0 && len(s.order) > s.maxJobs {
		oldest := s.order[0]
		s.order = s.order[1:]
		delete(s.jobs, oldest)
	}
}

func summarizeAccountHealthCheckResultsLocked(state *accountHealthCheckJobState) AccountHealthCheckSummary {
	summary := AccountHealthCheckSummary{
		Total:      len(state.order),
		ByCategory: make(map[string]int),
	}
	for _, accountID := range state.order {
		result := state.results[accountID]
		switch result.Status {
		case AccountHealthStatusPending:
			summary.Pending++
		case AccountHealthStatusChecking:
			summary.Checking++
		case AccountHealthStatusAvailable:
			summary.Available++
		case AccountHealthStatusRateLimited:
			summary.RateLimited++
		case AccountHealthStatusUnavailable:
			summary.Unavailable++
		}
		if result.Category != "" {
			summary.ByCategory[result.Category]++
		}
	}
	return summary
}

func advanceAccountHealthCheckCursorLocked(state *accountHealthCheckJobState) {
	nextCursor := state.job.Cursor
	for _, accountID := range state.order {
		result := state.results[accountID]
		if !isAccountHealthCheckFinalStatus(result.Status) {
			break
		}
		nextCursor = accountID
	}
	state.job.NextCursor = nextCursor
}

func isAccountHealthCheckFinalStatus(status string) bool {
	switch status {
	case AccountHealthStatusAvailable, AccountHealthStatusRateLimited, AccountHealthStatusUnavailable:
		return true
	default:
		return false
	}
}

func (h *AccountHandler) CreateHealthCheckJob(c *gin.Context) {
	if h.accountTestService == nil {
		response.Error(c, http.StatusServiceUnavailable, "account test service unavailable")
		return
	}

	var req AccountHealthCheckJobRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	limit := normalizeAccountHealthCheckLimit(req.Limit)
	batch, err := h.resolveAccountHealthCheckTargets(c.Request.Context(), &req, limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if len(batch.Accounts) == 0 {
		if req.Cursor > 0 {
			response.BadRequest(c, "no accounts matched after cursor")
			return
		}
		response.BadRequest(c, "no accounts matched")
		return
	}

	concurrency := normalizeAccountHealthCheckConcurrency(req.Concurrency)
	modelID := strings.TrimSpace(req.ModelID)
	if modelID == "" {
		modelID = strings.TrimSpace(req.Model)
	}

	job := accountHealthCheckJobs.create(batch)
	jobCtx, cancel := context.WithCancel(context.Background())
	accountHealthCheckJobs.setCancel(job.ID, cancel)

	go h.runAccountHealthCheckJob(jobCtx, job.ID, batch.Accounts, modelID, concurrency)

	response.Accepted(c, job)
}

func (h *AccountHandler) GetHealthCheckJob(c *gin.Context) {
	jobID := c.Param("job_id")
	job, ok := accountHealthCheckJobs.get(jobID)
	if !ok {
		response.NotFound(c, "health check job not found")
		return
	}
	response.Success(c, job)
}

func (h *AccountHandler) ListHealthCheckJobResults(c *gin.Context) {
	jobID := c.Param("job_id")
	status := strings.TrimSpace(c.Query("status"))
	category := strings.TrimSpace(c.Query("category"))
	results, ok := accountHealthCheckJobs.listResults(jobID, status, category)
	if !ok {
		response.NotFound(c, "health check job not found")
		return
	}
	response.Success(c, gin.H{
		"items": results,
		"total": len(results),
	})
}

func (h *AccountHandler) CancelHealthCheckJob(c *gin.Context) {
	jobID := c.Param("job_id")
	job, ok := accountHealthCheckJobs.cancel(jobID)
	if !ok {
		response.NotFound(c, "health check job not found")
		return
	}
	response.Success(c, job)
}

func (h *AccountHandler) BatchDelete(c *gin.Context) {
	var req struct {
		AccountIDs []int64 `json:"account_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	accountIDs := dedupeAccountHealthCheckIDs(req.AccountIDs)
	if len(accountIDs) == 0 {
		response.BadRequest(c, "account_ids is required")
		return
	}

	g, gctx := errgroup.WithContext(c.Request.Context())
	g.SetLimit(10)

	var mu sync.Mutex
	successIDs := make([]int64, 0, len(accountIDs))
	failedIDs := make([]int64, 0)
	results := make([]service.BulkUpdateAccountResult, 0, len(accountIDs))

	for _, id := range accountIDs {
		accountID := id
		g.Go(func() error {
			err := h.adminService.DeleteAccount(gctx, accountID)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failedIDs = append(failedIDs, accountID)
				results = append(results, service.BulkUpdateAccountResult{
					AccountID: accountID,
					Success:   false,
					Error:     err.Error(),
				})
			} else {
				successIDs = append(successIDs, accountID)
				results = append(results, service.BulkUpdateAccountResult{
					AccountID: accountID,
					Success:   true,
				})
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].AccountID < results[j].AccountID
	})
	response.Success(c, gin.H{
		"total":       len(accountIDs),
		"success":     len(successIDs),
		"failed":      len(failedIDs),
		"success_ids": successIDs,
		"failed_ids":  failedIDs,
		"results":     results,
	})
}

func (h *AccountHandler) runAccountHealthCheckJob(ctx context.Context, jobID string, accounts []service.Account, modelID string, concurrency int) {
	accountHealthCheckJobs.markStarted(jobID)

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
accountLoop:
	for _, account := range accounts {
		if ctx.Err() != nil {
			break
		}
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			break accountLoop
		}

		accountCopy := account
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			if ctx.Err() != nil {
				return
			}

			checking := AccountHealthCheckResult{
				AccountID: accountCopy.ID,
				Name:      accountCopy.Name,
				Platform:  accountCopy.Platform,
				Type:      accountCopy.Type,
				Status:    AccountHealthStatusChecking,
			}
			accountHealthCheckJobs.updateResult(jobID, checking)

			testResult, err := h.accountTestService.RunTestBackground(ctx, accountCopy.ID, modelID)
			if ctx.Err() != nil {
				return
			}
			finishedAt := time.Now()
			message := ""
			latencyMs := int64(0)
			testStatus := "failed"
			if testResult != nil {
				message = testResult.ErrorMessage
				latencyMs = testResult.LatencyMs
				testStatus = testResult.Status
			}
			if err != nil && message == "" {
				message = err.Error()
			}

			status, category, httpStatus, errorCode := classifyAccountHealthCheckError(testStatus, message)
			if status == AccountHealthStatusAvailable && h.rateLimitService != nil {
				_, _ = h.rateLimitService.RecoverAccountAfterSuccessfulTest(ctx, accountCopy.ID)
			}

			accountHealthCheckJobs.updateResult(jobID, AccountHealthCheckResult{
				AccountID:  accountCopy.ID,
				Name:       accountCopy.Name,
				Platform:   accountCopy.Platform,
				Type:       accountCopy.Type,
				Status:     status,
				Category:   category,
				HTTPStatus: httpStatus,
				ErrorCode:  errorCode,
				Message:    message,
				LatencyMs:  latencyMs,
				CheckedAt:  &finishedAt,
			})
		}()
	}
	wg.Wait()

	if ctx.Err() != nil {
		accountHealthCheckJobs.finish(jobID, accountHealthJobStatusCanceled, "")
		return
	}
	accountHealthCheckJobs.finish(jobID, accountHealthJobStatusCompleted, "")
}

func (h *AccountHandler) resolveAccountHealthCheckTargets(ctx context.Context, req *AccountHealthCheckJobRequest, limit int) (accountHealthCheckBatch, error) {
	includeUnschedulable := false
	if req.IncludeUnschedulable != nil {
		includeUnschedulable = *req.IncludeUnschedulable
	}

	if len(req.AccountIDs) > 0 {
		ids := dedupeAccountHealthCheckIDs(req.AccountIDs)
		accounts, err := h.adminService.GetAccountsByIDs(ctx, ids)
		if err != nil {
			return accountHealthCheckBatch{}, err
		}
		byID := make(map[int64]*service.Account, len(accounts))
		for _, account := range accounts {
			if account != nil {
				byID[account.ID] = account
			}
		}
		out := make([]service.Account, 0, len(byID))
		for _, id := range ids {
			account, ok := byID[id]
			if !ok || account == nil {
				continue
			}
			out = append(out, *account)
		}
		return buildAccountHealthCheckBatch(out, req.Cursor, limit, includeUnschedulable), nil
	}

	if req.Filters == nil {
		return accountHealthCheckBatch{Cursor: req.Cursor, Limit: limit}, nil
	}

	groupID, err := accountHealthCheckGroupID(req.Filters.Group)
	if err != nil {
		return accountHealthCheckBatch{}, err
	}

	out := make([]service.Account, 0, limit+1)
	for page := 1; ; page++ {
		accounts, total, err := h.adminService.ListAccounts(
			ctx,
			page,
			accountHealthCheckPageSize,
			req.Filters.Platform,
			req.Filters.Type,
			req.Filters.Status,
			req.Filters.Search,
			groupID,
			req.Filters.PrivacyMode,
			"id",
			"asc",
		)
		if err != nil {
			return accountHealthCheckBatch{}, err
		}
		for _, account := range accounts {
			if account.ID <= req.Cursor {
				continue
			}
			if !includeUnschedulable && !account.Schedulable {
				continue
			}
			out = append(out, account)
			if len(out) > limit {
				break
			}
		}
		if len(out) > limit {
			break
		}
		if int64(page*accountHealthCheckPageSize) >= total || len(accounts) == 0 {
			break
		}
	}
	return buildAccountHealthCheckBatch(out, req.Cursor, limit, true), nil
}

func accountHealthCheckGroupID(group string) (int64, error) {
	group = strings.TrimSpace(group)
	if group == "" {
		return 0, nil
	}
	if group == accountListGroupUngroupedQueryValue {
		return service.AccountListGroupUngrouped, nil
	}
	groupID, err := strconv.ParseInt(group, 10, 64)
	if err != nil || groupID < 0 {
		return 0, infraerrors.BadRequest("INVALID_GROUP_FILTER", "invalid group filter")
	}
	return groupID, nil
}

func normalizeAccountHealthCheckConcurrency(concurrency int) int {
	if concurrency <= 0 {
		return defaultAccountHealthCheckConcurrency
	}
	if concurrency > maxAccountHealthCheckConcurrency {
		return maxAccountHealthCheckConcurrency
	}
	return concurrency
}

func normalizeAccountHealthCheckLimit(limit int) int {
	if limit <= 0 {
		return defaultAccountHealthCheckLimit
	}
	if limit > maxAccountHealthCheckLimit {
		return maxAccountHealthCheckLimit
	}
	return limit
}

func buildAccountHealthCheckBatch(accounts []service.Account, cursor int64, limit int, includeUnschedulable bool) accountHealthCheckBatch {
	limit = normalizeAccountHealthCheckLimit(limit)
	sortedAccounts := append([]service.Account(nil), accounts...)
	sort.SliceStable(sortedAccounts, func(i, j int) bool {
		return sortedAccounts[i].ID < sortedAccounts[j].ID
	})

	out := make([]service.Account, 0, limit)
	nextCursor := cursor
	hasMore := false
	for _, account := range sortedAccounts {
		if account.ID <= cursor {
			continue
		}
		if !includeUnschedulable && !account.Schedulable {
			continue
		}
		if len(out) >= limit {
			hasMore = true
			break
		}
		out = append(out, account)
		nextCursor = account.ID
	}

	return accountHealthCheckBatch{
		Accounts:   out,
		Cursor:     cursor,
		Limit:      limit,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}
}

func dedupeAccountHealthCheckIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func classifyAccountHealthCheckError(testStatus, errMsg string) (status, category string, httpStatus int, errorCode string) {
	if strings.EqualFold(testStatus, "success") || strings.EqualFold(testStatus, AccountHealthStatusAvailable) {
		return AccountHealthStatusAvailable, AccountHealthCategoryAvailable, 0, ""
	}

	errMsg = strings.TrimSpace(errMsg)
	lower := strings.ToLower(errMsg)
	httpStatus = extractAccountHealthHTTPStatus(lower)
	errorCode = extractAccountHealthErrorCode(lower)

	if containsAny(lower, "insufficient_quota", "insufficient quota", "quota exhausted", "quota_exhausted", "quota exceeded", "quota_exceeded", "check quota", "usage limit", "usage_limit", "credit", "credits", "billing hard limit", "billing_hard_limit") {
		return AccountHealthStatusRateLimited, AccountHealthCategoryQuotaExhausted, httpStatus, errorCode
	}
	if httpStatus == http.StatusTooManyRequests || containsAny(lower, "rate_limit", "rate limit", "too many requests", "ratelimited", "rate-limited") {
		return AccountHealthStatusRateLimited, AccountHealthCategoryRateLimited, httpStatus, errorCode
	}
	if containsAny(lower, "no access token available", "no api key available", "missing refresh token", "missing api key", "api key is required", "credential is required", "credentials are required", "unsupported account", "unsupported platform") {
		return AccountHealthStatusUnavailable, AccountHealthCategoryConfigError, httpStatus, errorCode
	}
	if httpStatus == http.StatusUnauthorized || httpStatus == http.StatusForbidden || containsAny(lower, "invalid token", "invalid_refresh_token", "refresh_token_invalid", "refresh token invalid", "refresh_token_expired", "refresh token expired", "refresh_token_reused", "invalid_grant", "invalid_client", "invalid api key", "unauthorized", "unauthorized_client", "unauthenticated", "forbidden", "access_denied", "access denied", "authentication failed", "permission denied", "permission_denied") {
		return AccountHealthStatusUnavailable, AccountHealthCategoryAuthInvalid, httpStatus, errorCode
	}
	if containsAny(lower, "model not found", "unsupported model", "invalid model", "model_not_found") || (httpStatus == http.StatusNotFound && strings.Contains(lower, "model")) {
		return AccountHealthStatusUnavailable, AccountHealthCategoryModelError, httpStatus, errorCode
	}
	if containsAny(lower, "proxy", "connection refused", "connect: refused", "deadline exceeded", "timeout", "no such host", "tls", "certificate", "network is unreachable", "eof") {
		return AccountHealthStatusUnavailable, AccountHealthCategoryProxyError, httpStatus, errorCode
	}
	if httpStatus == http.StatusInternalServerError ||
		httpStatus == http.StatusBadGateway ||
		httpStatus == http.StatusServiceUnavailable ||
		httpStatus == http.StatusGatewayTimeout ||
		httpStatus == 529 ||
		containsAny(lower, "upstream", "server error", "temporarily unavailable", "overloaded", "bad gateway") {
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

func containsAny(s string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}
