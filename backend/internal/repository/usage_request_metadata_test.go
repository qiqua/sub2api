//go:build unit

package repository

import (
	"context"
	"database/sql/driver"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/requestmeta"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUsageRequestMetadataCountsDispatchedRetries(t *testing.T) {
	attempt := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempt++
		if attempt == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer upstream.Close()
	ctx, capture := requestmeta.Begin(context.Background(), http.MethodPost)
	for i := 0; i < 2; i++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, upstream.URL, nil)
		require.NoError(t, err)
		response, err := doUpstreamRequest(upstream.Client(), req)
		require.NoError(t, err)
		_, err = io.Copy(io.Discard, response.Body)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		if i == 1 {
			capture.RecordStatus(response.StatusCode)
		}
	}
	metadata := requestmeta.FromContext(ctx)
	require.Equal(t, http.MethodPost, *metadata.HTTPMethod)
	require.Equal(t, http.StatusCreated, *metadata.StatusCode)
	require.Equal(t, 2, *metadata.AttemptCount)
}

func TestUsageRequestMetadataPersistsThroughInsertAndRead(t *testing.T) {
	for _, historical := range []bool{false, true} {
		for _, bestEffort := range []bool{false, true} {
			t.Run(map[bool]string{false: "captured", true: "historical"}[historical]+map[bool]string{false: "/create", true: "/best_effort"}[bestEffort], func(t *testing.T) {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				defer db.Close()
				repo := &usageLogRepository{sql: db}
				ctx := context.Background()
				if !historical {
					var capture *requestmeta.Capture
					ctx, capture = requestmeta.Begin(ctx, "POST")
					capture.RecordStatus(201)
					requestmeta.RecordAttempt(ctx)
					requestmeta.RecordAttempt(ctx)
					ctx = requestmeta.SnapshotContext(ctx, context.Background())
				}
				log := &service.UsageLog{UserID: 1, APIKeyID: 2, AccountID: 3, Model: "model", CreatedAt: time.Now().UTC()}
				mock.ExpectQuery("INSERT INTO usage_logs").WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(7, log.CreatedAt))
				if bestEffort {
					require.NoError(t, repo.CreateBestEffort(ctx, log))
				} else {
					_, err = repo.Create(ctx, log)
					require.NoError(t, err)
				}
				prepared := prepareUsageLogInsert(log)
				require.Len(t, prepared.args, len(usageLogInsertArgTypes))
				values := []driver.Value{int64(7)}
				for _, arg := range prepared.args {
					value, err := driver.DefaultParameterConverter.ConvertValue(arg)
					require.NoError(t, err)
					values = append(values, value)
				}
				mock.ExpectQuery("SELECT .* FROM usage_logs WHERE id").WithArgs(7).
					WillReturnRows(sqlmock.NewRows(strings.Split(usageLogSelectColumns, ", ")).AddRow(values...))
				stored, err := repo.GetByID(context.Background(), 7)
				require.NoError(t, err)
				if historical {
					require.Nil(t, stored.HTTPMethod)
					require.Nil(t, stored.StatusCode)
					require.Nil(t, stored.AttemptCount)
				} else {
					require.Equal(t, "POST", *stored.HTTPMethod)
					require.Equal(t, 201, *stored.StatusCode)
					require.Equal(t, 2, *stored.AttemptCount)
				}
				require.NoError(t, mock.ExpectationsWereMet())
			})
		}
	}
}

func TestUsageRequestMetadataPreservesWebSocketTurnAttemptCount(t *testing.T) {
	ctx, capture := requestmeta.Begin(context.Background(), "GET")
	capture.RecordStatus(101)
	for i := 0; i < 5; i++ {
		requestmeta.RecordAttempt(ctx)
	}
	count := 1
	log := &service.UsageLog{OpenAIWSMode: true, AttemptCount: &count}
	applyUsageRequestMetadata(ctx, log)
	require.Equal(t, 1, *log.AttemptCount, "turn metadata takes priority over connection totals")
	require.Equal(t, "GET", *log.HTTPMethod)
	require.Equal(t, 101, *log.StatusCode)
	unknown := &service.UsageLog{OpenAIWSMode: true}
	applyUsageRequestMetadata(ctx, unknown)
	require.Nil(t, unknown.AttemptCount, "an uninstrumented turn must not inherit the connection total")
}
