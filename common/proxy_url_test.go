package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeProxyLine_IPv6(t *testing.T) {
	// Bracketed IPv6 with port
	u, err := NormalizeProxyLine("[2001:db8::1]:1080")
	require.NoError(t, err)
	assert.Equal(t, "socks5://[2001:db8::1]:1080", u)

	// Bracketed IPv6 with port, user, pass
	u, err = NormalizeProxyLine("[2001:db8::1]:8080:alice:secret123")
	require.NoError(t, err)
	assert.Equal(t, "socks5://alice:secret123@[2001:db8::1]:8080", u)

	// Standard URL with IPv6 and user:pass
	u, err = NormalizeProxyLine("http://alice:secret123@[2001:db8::1]:8080")
	require.NoError(t, err)
	assert.Equal(t, "http://alice:secret123@[2001:db8::1]:8080", u)

	// User:pass@bracketed host
	u, err = NormalizeProxyLine("user:pass@[2001:db8::1]:1080")
	require.NoError(t, err)
	assert.Equal(t, "socks5://user:pass@[2001:db8::1]:1080", u)

	// Bare IPv6
	u, err = NormalizeProxyLine("2001:db8::1")
	require.NoError(t, err)
	assert.Equal(t, "socks5://[2001:db8::1]:1080", u)
}
