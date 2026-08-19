package model

import (
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// BlogPost is an admin-authored article rendered on the public blog pages.
type BlogPost struct {
	Id             int64     `json:"id" gorm:"primaryKey"`
	Title          string    `json:"title" gorm:"type:varchar(255);not null"`
	Summary        string    `json:"summary" gorm:"type:text"`
	Content        string    `json:"content" gorm:"type:text"`
	CoverImage     string    `json:"cover_image" gorm:"type:varchar(512)"`
	Tags           string    `json:"tags" gorm:"type:varchar(512)"` // comma separated
	SeoDescription string    `json:"seo_description" gorm:"type:varchar(512)"`
	Published      bool      `json:"published" gorm:"default:false;index"`
	LikeCount      int       `json:"like_count" gorm:"default:0"`
	DislikeCount   int       `json:"dislike_count" gorm:"default:0"`
	CreatedAt      time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt      time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// BlogComment is a member comment on a blog post; ParentId enables replies.
type BlogComment struct {
	Id int64 `json:"id" gorm:"primaryKey"`
	// BlogId mirrors PostId; kept for legacy DB schemas where the blog_id
	// column was declared NOT NULL.
	BlogId       int64     `json:"blog_id" gorm:"index"`
	PostId       int64     `json:"post_id" gorm:"index"`
	UserId       int       `json:"user_id" gorm:"index"`
	Username     string    `json:"username" gorm:"type:varchar(64)"`
	Avatar       string    `json:"avatar" gorm:"type:varchar(512)"`
	ParentId     int64     `json:"parent_id" gorm:"default:0;index"`
	Content      string    `json:"content" gorm:"type:text"`
	LikeCount    int       `json:"like_count" gorm:"default:0"`
	DislikeCount int       `json:"dislike_count" gorm:"default:0"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// BlogReaction stores one user's like/dislike on a post or a comment.
type BlogReaction struct {
	Id         int64     `json:"id" gorm:"primaryKey"`
	BlogId     int64     `json:"blog_id" gorm:"index"`                                                      // owning post id, kept for legacy DB schemas where the column is NOT NULL
	TargetType string    `json:"target_type" gorm:"type:varchar(16);index:idx_blog_reaction_unique,unique"` // post | comment
	TargetId   int64     `json:"target_id" gorm:"index:idx_blog_reaction_unique,unique"`
	UserId     int       `json:"user_id" gorm:"index:idx_blog_reaction_unique,unique"`
	Value      int       `json:"value"` // 1 like, -1 dislike
	CreatedAt  time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (post *BlogPost) TagList() []string {
	var tags []string
	for _, tag := range strings.Split(post.Tags, ",") {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}

func GetBlogPostCount(publishedOnly bool) (int64, error) {
	var count int64
	tx := DB.Model(&BlogPost{})
	if publishedOnly {
		tx = tx.Where("published = ?", true)
	}
	return count, tx.Count(&count).Error
}

func GetBlogPosts(page int, pageSize int, publishedOnly bool) ([]*BlogPost, error) {
	var posts []*BlogPost
	tx := DB.Model(&BlogPost{}).Order("created_at DESC")
	if publishedOnly {
		tx = tx.Where("published = ?", true)
	}
	err := tx.Offset((page - 1) * pageSize).Limit(pageSize).Find(&posts).Error
	return posts, err
}

func GetBlogPostById(id int64) (*BlogPost, error) {
	var post BlogPost
	err := DB.First(&post, id).Error
	return &post, err
}

func CreateBlogPost(post *BlogPost) error {
	return DB.Create(post).Error
}

func UpdateBlogPost(post *BlogPost) error {
	return DB.Model(&BlogPost{}).Where("id = ?", post.Id).Updates(map[string]any{
		"title":           post.Title,
		"summary":         post.Summary,
		"content":         post.Content,
		"cover_image":     post.CoverImage,
		"tags":            post.Tags,
		"seo_description": post.SeoDescription,
		"published":       post.Published,
	}).Error
}

func DeleteBlogPost(id int64) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("post_id = ?", id).Delete(&BlogComment{}).Error; err != nil {
			return err
		}
		commentIDs := tx.Model(&BlogComment{}).Where("post_id = ?", id).Select("id")
		if err := tx.Where("target_type = 'post' AND target_id = ?", id).
			Or("target_type = 'comment' AND target_id IN (?)", commentIDs).
			Delete(&BlogReaction{}).Error; err != nil {
			return err
		}
		return tx.Delete(&BlogPost{}, id).Error
	})
}

func GetBlogComments(postId int64) ([]*BlogComment, error) {
	var comments []*BlogComment
	err := DB.Where("post_id = ?", postId).Order("created_at ASC").Find(&comments).Error
	return comments, err
}

func CreateBlogComment(comment *BlogComment) error {
	return DB.Create(comment).Error
}

func GetBlogCommentById(id int64) (*BlogComment, error) {
	var comment BlogComment
	err := DB.First(&comment, id).Error
	return &comment, err
}

func DeleteBlogComment(id int64) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ? OR parent_id = ?", id, id).Delete(&BlogComment{}).Error; err != nil {
			return err
		}
		return tx.Where("target_type = 'comment' AND (target_id = ? OR target_id IN (SELECT id FROM blog_comments WHERE parent_id = ?))", id, id).
			Delete(&BlogReaction{}).Error
	})
}

// GetUserBlogReactions returns the reactions a user left on a post and all of
// its comments, keyed by "type:id" for easy client lookup.
func GetUserBlogReactions(userId int, postId int64) (map[string]int, error) {
	commentIDs := DB.Model(&BlogComment{}).Where("post_id = ?", postId).Select("id")
	var reactions []BlogReaction
	err := DB.Where("user_id = ?", userId).
		Where("(target_type = 'post' AND target_id = ?) OR (target_type = 'comment' AND target_id IN (?))", postId, commentIDs).
		Find(&reactions).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]int, len(reactions))
	for _, r := range reactions {
		result[r.TargetType+":"+strconv.FormatInt(r.TargetId, 10)] = r.Value
	}
	return result, nil
}

// SetBlogReaction upserts/toggles a reaction and keeps the denormalized
// counters on the target row in sync. Returns the user's resulting value
// (0 when the reaction was removed).
func SetBlogReaction(userId int, postId int64, targetType string, targetId int64, value int) (int, error) {
	result := 0
	err := DB.Transaction(func(tx *gorm.DB) error {
		var existing BlogReaction
		findErr := tx.Where("target_type = ? AND target_id = ? AND user_id = ?", targetType, targetId, userId).
			First(&existing).Error
		if findErr != nil && findErr != gorm.ErrRecordNotFound {
			return findErr
		}

		var counterColumn string
		if value == 1 {
			counterColumn = "like_count"
		} else {
			counterColumn = "dislike_count"
		}
		target := blogReactionTarget(targetType)

		if findErr == gorm.ErrRecordNotFound {
			if err := tx.Create(&BlogReaction{BlogId: postId, TargetType: targetType, TargetId: targetId, UserId: userId, Value: value}).Error; err != nil {
				return err
			}
			if err := tx.Model(target).Where("id = ?", targetId).
				UpdateColumn(counterColumn, gorm.Expr(counterColumn+" + 1")).Error; err != nil {
				return err
			}
			result = value
			return nil
		}

		if existing.Value == value {
			// Toggle off.
			if err := tx.Delete(&existing).Error; err != nil {
				return err
			}
			return tx.Model(target).Where("id = ?", targetId).
				UpdateColumn(counterColumn, gorm.Expr(counterColumn+" - 1")).Error
		}

		// Switch sides.
		if err := tx.Model(&existing).Update("value", value).Error; err != nil {
			return err
		}
		oldColumn := "like_count"
		if existing.Value == -1 {
			oldColumn = "dislike_count"
		}
		if err := tx.Model(target).Where("id = ?", targetId).
			UpdateColumn(oldColumn, gorm.Expr(oldColumn+" - 1")).Error; err != nil {
			return err
		}
		if err := tx.Model(target).Where("id = ?", targetId).
			UpdateColumn(counterColumn, gorm.Expr(counterColumn+" + 1")).Error; err != nil {
			return err
		}
		result = value
		return nil
	})
	return result, err
}

func blogReactionTarget(targetType string) any {
	if targetType == "comment" {
		return &BlogComment{}
	}
	return &BlogPost{}
}

// ListPublishedBlogPostsForSitemap returns id+updated_at for every published
// post so the sitemap can reference them without loading full bodies.
func ListPublishedBlogPostsForSitemap() ([]*BlogPost, error) {
	var posts []*BlogPost
	err := DB.Model(&BlogPost{}).Where("published = ?", true).
		Select("id", "updated_at").Order("created_at DESC").Find(&posts).Error
	return posts, err
}
