package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// The very first blog build shipped a schema with blog_id NOT NULL, foreign
// keys and a unique index that also covered blog_id. That combination broke
// reactions on a second comment, and comment deletes. The startup
// normalization must rebuild those tables into the canonical schema while
// preserving rows.
func TestNormalizeLegacyBlogTablesRebuildsBrokenSchema(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	originalDB := DB
	DB = db
	t.Cleanup(func() { DB = originalDB })

	require.NoError(t, db.AutoMigrate(&BlogPost{}))
	require.NoError(t, db.Exec(`CREATE TABLE blog_comments (
		id integer PRIMARY KEY AUTOINCREMENT,
		blog_id integer NOT NULL REFERENCES blog_posts(id) ON DELETE CASCADE,
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
	require.NoError(t, db.Exec(`CREATE TABLE blog_reactions (
		id integer PRIMARY KEY AUTOINCREMENT,
		blog_id integer NOT NULL REFERENCES blog_posts(id) ON DELETE CASCADE,
		target_type varchar(16),
		target_id integer,
		user_id integer,
		value integer,
		created_at datetime
	)`).Error)
	require.NoError(t, db.Exec(`CREATE UNIQUE INDEX idx_blog_reaction_unique ON blog_reactions (blog_id, target_type, target_id, user_id)`).Error)

	post := BlogPost{Title: "t", Content: "c", Published: true}
	require.NoError(t, db.Create(&post).Error)
	require.NoError(t, db.Exec(`INSERT INTO blog_comments (blog_id, post_id, user_id, username, content) VALUES (?, ?, ?, ?, ?)`,
		post.Id, post.Id, 7, "legacy", "kept").Error)

	require.NoError(t, normalizeLegacyBlogTables())

	// Rows survived the rebuild.
	var count int64
	require.NoError(t, db.Model(&BlogComment{}).Count(&count).Error)
	require.Equal(t, int64(1), count)

	// Reactions on the post and on two different comments now succeed.
	comment := &BlogComment{BlogId: post.Id, PostId: post.Id, UserId: 1, Username: "u", Content: "a"}
	require.NoError(t, CreateBlogComment(comment))
	_, err = SetBlogReaction(1, post.Id, "post", post.Id, 1)
	require.NoError(t, err)
	_, err = SetBlogReaction(1, post.Id, "comment", comment.Id, 1)
	require.NoError(t, err)

	// Comment delete (with its reaction cleanup) works.
	require.NoError(t, DeleteBlogComment(comment.Id))
}
