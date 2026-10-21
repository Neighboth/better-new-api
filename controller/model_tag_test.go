package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseModelTagRouting(t *testing.T) {
	cases := []struct {
		raw    string
		base   string
		floor  bool
		nitro  bool
		online bool
	}{
		{"gpt-4o", "gpt-4o", false, false, false},
		{"gpt-4o:floor", "gpt-4o", true, false, false},
		{"gpt-4o:nitro", "gpt-4o", false, true, false},
		{"gpt-4o:online", "gpt-4o", false, false, true},
		{"gpt-4o:floor:online", "gpt-4o", true, false, true},
		{"gpt-4o:NITRO", "gpt-4o", false, true, false},
		{"gpt-4o:unknown", "gpt-4o:unknown", false, false, false},
		{"", "", false, false, false},
	}
	for _, tc := range cases {
		r := parseModelTagRouting(tc.raw)
		assert.Equal(t, tc.base, r.base, "raw=%q", tc.raw)
		assert.Equal(t, tc.floor, r.floor, "raw=%q", tc.raw)
		assert.Equal(t, tc.nitro, r.nitro, "raw=%q", tc.raw)
		assert.Equal(t, tc.online, r.online, "raw=%q", tc.raw)
	}
	require.Equal(t, "gpt-4o", parseModelTagRouting("gpt-4o:floor").base)
}
