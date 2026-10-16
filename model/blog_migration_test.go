package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// The very first blog build shipped a schema with blog_id NOT NULL, foreign
// keys, a unique index that also covered blog_id, and columns the canonical
// schema no longer has (avatar_url, type). That combination broke reactions
// and comment deletes, and a naive column-list copy breaks on the extra
// columns. The startup normalization must rebuild those tables into the
// canonical schema while preserving rows and mapping legacy columns.
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
		avatar_url varchar(512),
		avatar varchar(512),
		parent_id integer,
		content text,
		like_count integer,
		dislike_count integer,
		created_at datetime,
		updated_at datetime
	)`).Error)
	require.NoError(t, db.Exec(`CREATE TABLE blog_reactions (
		id integer PRIMARY KEY AUTOINCREMENT,
		blog_id integer NOT NULL REFERENCES blog_posts(id) ON DELETE CASCADE,
		type varchar(16),
		target_type varchar(16),
		target_id integer,
		user_id integer,
		value integer,
		created_at datetime
	)`).Error)
	require.NoError(t, db.Exec(`CREATE UNIQUE INDEX idx_blog_reaction_unique ON blog_reactions (blog_id, target_type, target_id, user_id)`).Error)

	post := BlogPost{Title: "t", Content: "c", Published: true}
	require.NoError(t, db.Create(&post).Error)
	// Legacy row keeps its real avatar in avatar_url; the later AutoMigrate
	// added an empty avatar column next to it.
	require.NoError(t, db.Exec(`INSERT INTO blog_comments (blog_id, post_id, user_id, username, avatar_url, avatar, content) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		post.Id, post.Id, 7, "legacy", "https://img/avatar.png", "", "kept").Error)
	require.NoError(t, db.Exec(`INSERT INTO blog_reactions (blog_id, type, target_type, target_id, user_id, value) VALUES (?, ?, ?, ?, ?, ?)`,
		post.Id, "post", "", post.Id, 7, 1).Error)

	require.NoError(t, normalizeLegacyBlogTables())

	// Rows survived the rebuild, with legacy columns mapped.
	var comment BlogComment
	require.NoError(t, db.First(&comment).Error)
	require.Equal(t, "kept", comment.Content)
	require.Equal(t, "https://img/avatar.png", comment.Avatar)

	var reaction BlogReaction
	require.NoError(t, db.First(&reaction).Error)
	require.Equal(t, "post", reaction.TargetType)
	require.Equal(t, post.Id, reaction.BlogId)

	// Reactions on the post and on two different comments now succeed.
	newComment := &BlogComment{BlogId: post.Id, PostId: post.Id, UserId: 1, Username: "u", Content: "a"}
	require.NoError(t, CreateBlogComment(newComment))
	_, err = SetBlogReaction(1, post.Id, "post", post.Id, 1)
	require.NoError(t, err)
	_, err = SetBlogReaction(1, post.Id, "comment", newComment.Id, 1)
	require.NoError(t, err)

	// Comment delete (with its reaction cleanup) works.
	require.NoError(t, DeleteBlogComment(newComment.Id))
}

// A rebuild interrupted mid-copy must be completed on the next start: rows
// in the leftover "<table>_legacy_*" table are merged into the canonical
// table and the leftover is dropped.
func TestNormalizeLegacyBlogTablesFinishesCrashedRebuild(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	originalDB := DB
	DB = db
	t.Cleanup(func() { DB = originalDB })

	require.NoError(t, db.AutoMigrate(&BlogPost{}, &BlogComment{}))
	post := BlogPost{Title: "t", Content: "c", Published: true}
	require.NoError(t, db.Create(&post).Error)

	require.NoError(t, db.Exec(`CREATE TABLE blog_comments_legacy_999 (
		id integer PRIMARY KEY AUTOINCREMENT,
		blog_id integer NOT NULL,
		post_id integer,
		user_id integer,
		username varchar(64),
		avatar_url varchar(512),
		content text,
		created_at datetime
	)`).Error)
	require.NoError(t, db.Exec(`INSERT INTO blog_comments_legacy_999 (blog_id, post_id, user_id, username, avatar_url, content) VALUES (?, ?, ?, ?, ?, ?)`,
		post.Id, post.Id, 3, "rescued", "https://img/a.png", "stranded row").Error)

	require.NoError(t, normalizeLegacyBlogTables())

	var comment BlogComment
	require.NoError(t, db.First(&comment).Error)
	require.Equal(t, "stranded row", comment.Content)
	require.Equal(t, "https://img/a.png", comment.Avatar)
	require.False(t, db.Migrator().HasTable("blog_comments_legacy_999"))
}
