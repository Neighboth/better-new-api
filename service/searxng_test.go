package service

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunOnlineSearchRequiresURL(t *testing.T) {
	prev := operation_setting.GetOnlineSearchSetting().SearXNGURL
	operation_setting.GetOnlineSearchSetting().SearXNGURL = ""
	defer func() { operation_setting.GetOnlineSearchSetting().SearXNGURL = prev }()
	_, _, err := RunOnlineSearch("hello", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "searxng")
}
