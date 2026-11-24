package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUserModelAlias_CRUDAndPrivateIsolation(t *testing.T) {
	origDB := DB
	t.Cleanup(func() { DB = origDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&UserModelAlias{}))
	DB = db

	// Create alias for user 10
	alias, err := CreateUserModelAlias(10, "auto-myapp", "opus-5.5", "Production default")
	require.NoError(t, err)
	require.NotZero(t, alias.Id)

	// Resolves for user 10
	target, ok := ResolveUserModelAlias(10, "auto-myapp")
	assert.True(t, ok)
	assert.Equal(t, "opus-5.5", target)

	// Private isolation: user 20 MUST NOT resolve user 10's alias
	targetOther, okOther := ResolveUserModelAlias(20, "auto-myapp")
	assert.False(t, okOther)
	assert.Empty(t, targetOther)

	// User 20 CAN create their own alias with the same name pointing to another model
	_, err = CreateUserModelAlias(20, "auto-myapp", "gpt-5.4", "")
	require.NoError(t, err)
	target20, ok20 := ResolveUserModelAlias(20, "auto-myapp")
	assert.True(t, ok20)
	assert.Equal(t, "gpt-5.4", target20)

	// Duplicate alias for the same user fails
	_, err = CreateUserModelAlias(10, "auto-myapp", "claude-sonnet", "")
	assert.Error(t, err)

	// Alias equal to target model fails
	_, err = CreateUserModelAlias(10, "same-name", "same-name", "")
	assert.Error(t, err)

	// Update target model for user 10
	updated, err := UpdateUserModelAlias(10, alias.Id, "auto-myapp", "opus-5.6", "Updated")
	require.NoError(t, err)
	assert.Equal(t, "opus-5.6", updated.TargetModel)
	target, ok = ResolveUserModelAlias(10, "auto-myapp")
	assert.True(t, ok)
	assert.Equal(t, "opus-5.6", target)

	// Delete alias for user 10 does not affect user 20
	require.NoError(t, DeleteUserModelAlias(10, alias.Id))
	_, ok = ResolveUserModelAlias(10, "auto-myapp")
	assert.False(t, ok)

	list10, err := ListUserModelAliases(10)
	require.NoError(t, err)
	assert.Empty(t, list10)

	list20, err := ListUserModelAliases(20)
	require.NoError(t, err)
	assert.Len(t, list20, 1)
}
