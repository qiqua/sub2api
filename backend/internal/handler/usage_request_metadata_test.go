package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/requestmeta"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUsageRequestMetadataOnlyRecordsCommittedStatus(t *testing.T) {
	for _, writeMode := range []string{"json", "flush", "header_now", "unwritten", "websocket"} {
		t.Run(writeMode, func(t *testing.T) {
			router := gin.New()
			router.Use(OpsErrorLoggerMiddleware(nil))
			var observed requestmeta.Values
			router.POST("/request", func(c *gin.Context) {
				switch writeMode {
				case "json":
					c.JSON(http.StatusCreated, gin.H{"ok": true})
				case "flush":
					c.Writer.Flush()
				case "header_now":
					c.Writer.WriteHeader(http.StatusAccepted)
					c.Writer.WriteHeaderNow()
				case "websocket":
					c.Writer.WriteHeader(http.StatusSwitchingProtocols)
					c.Writer.WriteHeaderNow()
				case "unwritten":
					c.Status(http.StatusCreated)
				}
				observed = requestmeta.FromContext(usageRecordContext(c.Request.Context(), context.Background()))
			})
			router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/request", nil))
			require.Equal(t, http.MethodPost, *observed.HTTPMethod)
			require.Nil(t, observed.AttemptCount, "no upstream request was dispatched")
			if writeMode == "unwritten" {
				require.Nil(t, observed.StatusCode, "queued billing must not invent a response status before it is committed")
			} else {
				require.NotNil(t, observed.StatusCode)
				require.Equal(t, map[string]int{"json": 201, "flush": 200, "header_now": 202, "websocket": 101}[writeMode], *observed.StatusCode)
			}
		})
	}
}

func TestUsageRequestMetadataSnapshotSurvivesGinReuse(t *testing.T) {
	pool := newUsageRecordTestPool(t)
	blocked, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	pool.Submit(func(context.Context) { close(blocked); <-release })
	<-blocked
	handler := &OpenAIGatewayHandler{usageRecordWorkerPool: pool}
	got := make(chan requestmeta.Values, 1)
	router := gin.New()
	router.Use(OpsErrorLoggerMiddleware(nil))
	var first *gin.Context
	router.POST("/first", func(c *gin.Context) {
		first = c
		requestmeta.RecordAttempt(c.Request.Context())
		requestmeta.RecordAttempt(c.Request.Context())
		c.JSON(201, gin.H{"ok": true})
		handler.submitUsageRecordTask(c.Request.Context(), func(ctx context.Context) {
			got <- requestmeta.FromContext(ctx)
		})
		requestmeta.RecordAttempt(c.Request.Context())
	})
	router.GET("/next", func(c *gin.Context) { c.JSON(503, gin.H{"error": "second request"}) })
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/first", nil))
	first.Request = httptest.NewRequest(http.MethodGet, "/next", nil)
	router.HandleContext(first)
	unblock()
	select {
	case values := <-got:
		require.Equal(t, "POST", *values.HTTPMethod)
		require.Equal(t, 201, *values.StatusCode)
		require.Equal(t, 2, *values.AttemptCount)
	case <-time.After(2 * time.Second):
		t.Fatal("usage task did not complete")
	}
}

func TestUsageRequestMetadataCapturesWebSocketHandshake(t *testing.T) {
	got := make(chan requestmeta.Values, 1)
	router := gin.New()
	router.Use(OpsErrorLoggerMiddleware(nil))
	router.GET("/responses", func(c *gin.Context) {
		conn, err := coderws.Accept(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		got <- requestmeta.FromContext(usageRecordContext(c.Request.Context(), context.Background()))
	})
	server := httptest.NewServer(router)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, _, err := coderws.Dial(ctx, server.URL+"/responses", nil)
	require.NoError(t, err)
	defer conn.CloseNow()
	select {
	case values := <-got:
		require.Equal(t, "GET", *values.HTTPMethod)
		require.Equal(t, 101, *values.StatusCode)
	case <-ctx.Done():
		t.Fatal("handshake metadata was not captured")
	}
}
