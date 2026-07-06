//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateEndpointAllowsHTTPAndHTTPSOrigins(t *testing.T) {
	t.Parallel()

	for _, endpoint := range []string{
		"http://1.1.1.1:8080",
		"https://1.1.1.1",
	} {
		require.NoError(t, validateEndpoint(endpoint))
	}
}

func TestValidateEndpointRejectsUnsupportedScheme(t *testing.T) {
	t.Parallel()

	err := validateEndpoint("ftp://1.1.1.1")

	require.ErrorIs(t, err, ErrChannelMonitorEndpointScheme)
}

func TestValidateEndpointKeepsOriginAndSSRFGuards(t *testing.T) {
	t.Parallel()

	require.ErrorIs(t, validateEndpoint("http://1.1.1.1/v1"), ErrChannelMonitorEndpointPath)
	require.ErrorIs(t, validateEndpoint("http://127.0.0.1:8080"), ErrChannelMonitorEndpointPrivate)
}
