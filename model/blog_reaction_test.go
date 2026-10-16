package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// SetBlogReaction must work against databases created by earlier builds where
// blog_reactions.blog_id was declared NOT NULL. AutoMigrate never drops or
// relaxes that legacy column, so inserts must always populate it.
func TestSetBlogReactionWithLegacyNotNullBlogId(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	originalDB := DB
	DB = db
	t.Cleanup(func() { DB = originalDB })

	require.NoError(t, db.AutoMigrate(&BlogPost{}, &BlogComment{}))
	// Legacy schema: blog_id NOT NULL, as created by the first blog release.
	require.NoError(t, db.Exec(`CREATE TABLE blog_reactions (
		id integer PRIMARY KEY AUTOINCREMENT,
		blog_id integer NOT NULL,
		target_type varchar(16),
		target_id integer,
		user_id integer,
		value integer,
		created_at datetime
	)`).Error)

	post := BlogPost{Title: "t", Content: "c", Published: true}
	require.NoError(t, db.Create(&post).Error)

	value, err := SetBlogReaction(1, post.Id, "post", post.Id, 1)
	require.NoError(t, err)
	require.Equal(t, 1, value)

	var reaction BlogReaction
	require.NoError(t, db.First(&reaction).Error)
	require.Equal(t, post.Id, reaction.BlogId)

	var updated BlogPost
	require.NoError(t, db.First(&updated, post.Id).Error)
	require.Equal(t, 1, updated.LikeCount)

	// Toggling the same reaction off removes the row and the count.
	value, err = SetBlogReaction(1, post.Id, "post", post.Id, 1)
	require.NoError(t, err)
	require.Equal(t, 0, value)
	require.NoError(t, db.First(&updated, post.Id).Error)
	require.Equal(t, 0, updated.LikeCount)
}

// Comment inserts must also populate the legacy blog_id NOT NULL column.
func TestCreateBlogCommentWithLegacyNotNullBlogId(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	originalDB := DB
	DB = db
	t.Cleanup(func() { DB = originalDB })

	require.NoError(t, db.AutoMigrate(&BlogPost{}, &BlogReaction{}))
	require.NoError(t, db.Exec(`CREATE TABLE blog_comments (
		id integer PRIMARY KEY AUTOINCREMENT,
		blog_id integer NOT NULL,
		post_id integer,
		user_id integer,
		username varchar(64),
		avatar varchar(512),
		parent_id integer,
		content text,
		like_count integer,
		dislike_count integer,
		created_at datetime
	)`).Error)

	post := BlogPost{Title: "t", Content: "c", Published: true}
	require.NoError(t, db.Create(&post).Error)

	comment := &BlogComment{BlogId: post.Id, PostId: post.Id, UserId: 1, Username: "u", Content: "hi"}
	require.NoError(t, CreateBlogComment(comment))

	var stored BlogComment
	require.NoError(t, db.First(&stored).Error)
	require.Equal(t, post.Id, stored.BlogId)
	require.Equal(t, post.Id, stored.PostId)
}
