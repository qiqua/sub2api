package requestmeta

import (
	"context"
	"strings"
	"sync/atomic"
)

type captureKey struct{}
type snapshotKey struct{}

// Values contains only observations made for this request. Nil means unknown.
type Values struct {
	HTTPMethod   *string
	StatusCode   *int
	AttemptCount *int
}

type Capture struct {
	method   string
	status   atomic.Int64
	attempts atomic.Int64
}

func Begin(ctx context.Context, method string) (context.Context, *Capture) {
	method = strings.TrimSpace(method)
	if len(method) > 16 {
		method = ""
	}
	capture := &Capture{method: method}
	return context.WithValue(ctx, captureKey{}, capture), capture
}

// RecordStatus receives the committed client-facing HTTP status. An SSE error
// after headers are sent does not change that wire status.
func (c *Capture) RecordStatus(status int) {
	if c != nil && (status == 101 || (status >= 200 && status <= 599)) {
		c.status.CompareAndSwap(0, int64(status))
	}
}

// RecordAttempt counts upstream HTTP requests and WebSocket response.create
// dispatches, including account failover and retries.
func RecordAttempt(ctx context.Context) {
	if capture := fromContext(ctx); capture != nil {
		capture.attempts.Add(1)
	}
}

func fromContext(ctx context.Context) *Capture {
	if ctx == nil {
		return nil
	}
	capture, _ := ctx.Value(captureKey{}).(*Capture)
	return capture
}

func FromContext(ctx context.Context) Values {
	if ctx == nil {
		return Values{}
	}
	if values, ok := ctx.Value(snapshotKey{}).(Values); ok {
		return values
	}
	capture := fromContext(ctx)
	if capture == nil {
		return Values{}
	}
	values := Values{}
	if capture.method != "" {
		method := capture.method
		values.HTTPMethod = &method
	}
	if status := int(capture.status.Load()); status > 0 {
		values.StatusCode = &status
	}
	if attempts := int(capture.attempts.Load()); attempts > 0 {
		values.AttemptCount = &attempts
	}
	return values
}

// SnapshotContext detaches the async billing record from mutable request state.
// In particular, a later WebSocket turn must not change an earlier record.
func SnapshotContext(parent, destination context.Context) context.Context {
	return context.WithValue(destination, snapshotKey{}, FromContext(parent))
}
