package service

import (
	"context"
	"strings"
	"sync/atomic"

	"github.com/Wei-Shaw/sub2api/internal/pkg/requestmeta"
	openaiwsv2 "github.com/Wei-Shaw/sub2api/internal/service/openai_ws_v2"
	coderws "github.com/coder/websocket"
	"github.com/tidwall/gjson"
)

type openAIWSAttemptCountingFrameConn struct {
	openaiwsv2.FrameConn
	firstTurnContext context.Context
	firstDispatched  atomic.Bool
	turnAttempts     atomic.Int64
}

func (c *openAIWSAttemptCountingFrameConn) WriteFrame(ctx context.Context, msgType coderws.MessageType, payload []byte) error {
	if (msgType == coderws.MessageText || msgType == coderws.MessageBinary) && strings.TrimSpace(gjson.GetBytes(payload, "type").String()) == "response.create" {
		attempts := 1
		if !c.firstDispatched.Swap(true) {
			// Only the retained first turn can be retried across account connections.
			requestmeta.RecordAttempt(c.firstTurnContext)
			if captured := requestmeta.FromContext(c.firstTurnContext).AttemptCount; captured != nil {
				attempts = *captured
			}
		}
		c.turnAttempts.Store(int64(attempts))
	}
	return c.FrameConn.WriteFrame(ctx, msgType, payload)
}

func (c *openAIWSAttemptCountingFrameConn) attemptCount() *int {
	attempts := int(c.turnAttempts.Load())
	if attempts == 0 {
		return nil
	}
	return &attempts
}
