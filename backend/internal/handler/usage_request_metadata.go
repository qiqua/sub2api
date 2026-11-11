package handler

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/requestmeta"
	"github.com/gin-gonic/gin"
)

// installUsageRequestMetadata runs before the existing gateway response capture.
// The queued usage task receives a value snapshot, never this response writer.
func installUsageRequestMetadata(c *gin.Context) {
	ctx, capture := requestmeta.Begin(c.Request.Context(), c.Request.Method)
	c.Request = c.Request.WithContext(ctx)
	c.Writer = &usageMetadataWriter{ResponseWriter: c.Writer, capture: capture}
}

type usageMetadataWriter struct {
	gin.ResponseWriter
	capture *requestmeta.Capture
}

func (w *usageMetadataWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *usageMetadataWriter) recordStatus() {
	if w.Written() {
		w.capture.RecordStatus(w.Status())
	}
}

func (w *usageMetadataWriter) WriteHeader(status int) {
	w.ResponseWriter.WriteHeader(status)
	w.recordStatus()
}

func (w *usageMetadataWriter) WriteHeaderNow() {
	w.ResponseWriter.WriteHeaderNow()
	w.recordStatus()
}

func (w *usageMetadataWriter) Write(data []byte) (int, error) {
	n, err := w.ResponseWriter.Write(data)
	w.recordStatus()
	return n, err
}

func (w *usageMetadataWriter) WriteString(data string) (int, error) {
	n, err := w.ResponseWriter.WriteString(data)
	w.recordStatus()
	return n, err
}

func (w *usageMetadataWriter) Flush() {
	w.ResponseWriter.Flush()
	w.recordStatus()
}
