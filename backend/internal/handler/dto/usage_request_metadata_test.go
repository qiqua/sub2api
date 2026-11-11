package dto

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUsageLogRequestMetadataSurvivesUserAndAdminJSON(t *testing.T) {
	var log service.UsageLog
	require.NoError(t, json.Unmarshal([]byte(`{"HTTPMethod":"POST","StatusCode":201,"AttemptCount":3}`), &log))
	for _, mapped := range []any{UsageLogFromService(&log), UsageLogFromServiceAdmin(&log)} {
		encoded, err := json.Marshal(mapped)
		require.NoError(t, err)
		var fields map[string]any
		require.NoError(t, json.Unmarshal(encoded, &fields))
		require.Equal(t, "POST", fields["http_method"])
		require.Equal(t, float64(201), fields["status_code"])
		require.Equal(t, float64(3), fields["attempt_count"])
	}
}

func TestUsageLogHistoricalRequestMetadataStaysUnknown(t *testing.T) {
	encoded, err := json.Marshal(UsageLogFromService(&service.UsageLog{}))
	require.NoError(t, err)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(encoded, &fields))
	for _, name := range []string{"http_method", "status_code", "attempt_count"} {
		require.Nil(t, fields[name], "historical rows must not fabricate %s", name)
	}
}
