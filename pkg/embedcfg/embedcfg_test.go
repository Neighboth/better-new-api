package embedcfg

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAppendAndExtractRoundTrip(t *testing.T) {
	program := []byte("MZ-program")
	settings := []byte(`{"server_url":"https://example.com","system_name":"Example"}`)

	combined := Append(program, settings)
	require.Equal(t, program, combined[:len(program)])

	extracted, ok := Extract(combined)
	require.True(t, ok)
	require.Equal(t, settings, extracted)
}

func TestExtractRejectsProgramWithoutSettings(t *testing.T) {
	_, ok := Extract([]byte("MZ-program"))
	require.False(t, ok)
}
