package requestmeta

import (
	"context"
	"sync"
	"testing"
)

func TestCaptureConcurrentAttemptsAndFrozenSnapshots(t *testing.T) {
	ctx, capture := Begin(context.Background(), "POST")
	capture.RecordStatus(201)
	RecordAttempt(ctx)
	frozen := SnapshotContext(ctx, context.Background())
	const workers, perWorker = 8, 100
	var tasks sync.WaitGroup
	for range workers {
		tasks.Add(1)
		go func() {
			defer tasks.Done()
			for range perWorker {
				RecordAttempt(ctx)
				capture.RecordStatus(503)
				_ = FromContext(ctx)
				if *FromContext(frozen).AttemptCount != 1 {
					t.Error("async snapshot changed after later dispatch")
				}
			}
		}()
	}
	tasks.Wait()
	latest := FromContext(ctx)
	if *latest.AttemptCount != 1+workers*perWorker {
		t.Fatalf("lost concurrent attempts: got %d", *latest.AttemptCount)
	}
	for _, values := range []Values{latest, FromContext(frozen)} {
		if *values.HTTPMethod != "POST" || *values.StatusCode != 201 {
			t.Fatalf("committed request identity changed: %+v", values)
		}
	}
	copy := SnapshotContext(frozen, context.Background())
	if *FromContext(copy).AttemptCount != 1 {
		t.Fatal("copying the worker context lost its request snapshot")
	}
}

func TestMissingCaptureAndUnwrittenStatusStayUnknown(t *testing.T) {
	for _, ctx := range []context.Context{nil, context.Background()} {
		RecordAttempt(ctx)
		values := FromContext(ctx)
		if values.HTTPMethod != nil || values.StatusCode != nil || values.AttemptCount != nil {
			t.Fatal("unobserved metadata was invented")
		}
	}
	ctx, capture := Begin(context.Background(), "GET")
	for _, status := range []int{0, 100, 103, 600} {
		capture.RecordStatus(status)
	}
	if FromContext(ctx).StatusCode != nil {
		t.Fatal("interim headers must not become the final response status")
	}
	capture.RecordStatus(101)
	if *FromContext(ctx).StatusCode != 101 {
		t.Fatal("committed WebSocket handshake was not captured")
	}
}
