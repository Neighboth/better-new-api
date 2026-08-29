package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFallbackStateChain(t *testing.T) {
	s := &fallbackState{
		models: []string{"m1", "m2"},
		all:    []string{"x1", "x2", "x3"},
		idx:    0,
		orig:   "orig-model",
		on:     true,
	}

	m, ok := s.currentModel()
	require.True(t, ok)
	assert.Equal(t, "m1", m)

	s.advance()
	m, ok = s.currentModel()
	require.True(t, ok)
	assert.Equal(t, "m2", m)

	// Exhaust the configured list; the chain moves into the arbitrary phase.
	assert.True(t, s.advance())
	m, ok = s.currentModel()
	require.True(t, ok)
	assert.Contains(t, []string{"x1", "x2", "x3"}, m)

	s.advance()
	m, ok = s.currentModel()
	require.True(t, ok)
	assert.Contains(t, []string{"x1", "x2", "x3"}, m)

	// Last arbitrary candidate is consumed; advancing further reports failure.
	assert.True(t, s.advance())
	s.advance()
	_, ok = s.currentModel()
	assert.False(t, ok)
}

func TestFallbackStateDisabled(t *testing.T) {
	s := &fallbackState{on: false}
	_, ok := s.currentModel()
	assert.False(t, ok)
	assert.False(t, s.advance())
}
