package service

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/requestmeta"
	coderws "github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestPassthroughLifecycle_AttemptsBelongToEachDispatchedTurn(t *testing.T) {
	gin.SetMode(gin.TestMode)
	requestCtx, _ := requestmeta.Begin(context.Background(), http.MethodGet)
	requestmeta.RecordAttempt(requestCtx)
	requestmeta.RecordAttempt(requestCtx)
	controlCtx, cancelControl := context.WithCancelCause(requestCtx)
	defer cancelControl(context.Canceled)
	upstream := newStagedPassthroughConn()
	svc := newPassthroughLifecycleService(passthroughLifecycleConfig(), upstream)
	results := make(chan *OpenAIForwardResult, 2)
	server, _ := startPassthroughLifecycleServerWithHooks(t, controlCtx, svc, passthroughLifecycleAccount(), func(*gin.Context) *OpenAIWSIngressHooks {
		return &OpenAIWSIngressHooks{AfterTurn: func(_ int, result *OpenAIForwardResult, err error) {
			if err == nil && result != nil {
				results <- result
			}
		}}
	})
	defer server.Close()
	clientConn := dialPassthroughLifecycleClient(t, server)
	defer func() { _ = clientConn.CloseNow() }()
	first := requirePassthroughUpstreamWrite(t, upstream, time.Second)
	require.Equal(t, "response.create", gjson.GetBytes(first, "type").String())

	for turn := range 2 {
		upstream.Send(fmt.Sprintf(`{"type":"response.completed","response":{"id":"resp_attempt_%d","model":"gpt-5.1","usage":{"input_tokens":1,"output_tokens":1}}}`, turn))
		frame, err := readPassthroughLifecycleFrame(t, clientConn, time.Second)
		require.NoError(t, err)
		require.Equal(t, "response.completed", gjson.GetBytes(frame, "type").String())
		if turn == 0 {
			for _, payload := range []string{`{"type":"session.update","session":{"model":"gpt-5.1"}}`, `{"type":"response.create","model":"gpt-5.1"}`} {
				writeCtx, cancelWrite := context.WithTimeout(context.Background(), time.Second)
				err = clientConn.Write(writeCtx, coderws.MessageText, []byte(payload))
				cancelWrite()
				require.NoError(t, err)
				written := requirePassthroughUpstreamWrite(t, upstream, time.Second)
				require.Equal(t, gjson.Get(payload, "type").String(), gjson.GetBytes(written, "type").String())
			}
		}
	}
	var recorded []*OpenAIForwardResult
	for _, expected := range []int{3, 1} {
		select {
		case result := <-results:
			require.NotNil(t, result.AttemptCount)
			require.Equal(t, expected, *result.AttemptCount)
			recorded = append(recorded, result)
		case <-time.After(time.Second):
			t.Fatal("terminal turn did not produce a usage result")
		}
	}
	require.Equal(t, 3, *recorded[0].AttemptCount, "the next turn must not mutate the prior result")
}
