package model

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/QuantumNous/new-api/common"
)

// BlogPost is an admin-authored article rendered on the public blog pages.
// BlogPostI18n holds the localized variant of a post's text fields. Empty
// fields fall back to the English base on the post itself.
type BlogPostI18n struct {
	Title          string `json:"title,omitempty"`
	Summary        string `json:"summary,omitempty"`
	Content        string `json:"content,omitempty"`
	SeoDescription string `json:"seo_description,omitempty"`
	Tags           string `json:"tags,omitempty"`
}

type BlogPost struct {
	Id             int64  `json:"id" gorm:"primaryKey"`
	Title          string `json:"title" gorm:"type:varchar(255);not null"`
	Summary        string `json:"summary" gorm:"type:text"`
	Content        string `json:"content" gorm:"type:text"`
	CoverImage     string `json:"cover_image" gorm:"type:varchar(512)"`
	Tags           string `json:"tags" gorm:"type:varchar(512)"` // comma separated
	SeoDescription string `json:"seo_description" gorm:"type:varchar(512)"`
	// Translations is a JSON object keyed by content language code
	// ("tr", "fr", ...) with BlogPostI18n values. English stays in the base
	// columns above so existing single-language posts keep working unchanged.
	Translations string    `json:"translations" gorm:"type:text"`
	Published    bool      `json:"published" gorm:"default:false;index"`
	LikeCount    int       `json:"like_count" gorm:"default:0"`
	DislikeCount int       `json:"dislike_count" gorm:"default:0"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// TranslationMap decodes the Translations JSON column.
func (post *BlogPost) TranslationMap() map[string]BlogPostI18n {
	out := map[string]BlogPostI18n{}
	if strings.TrimSpace(post.Translations) == "" {
		return out
	}
	if err := common.Unmarshal([]byte(post.Translations), &out); err != nil {
		common.SysError("failed to decode blog translations for post " + strconv.FormatInt(post.Id, 10) + ": " + err.Error())
		return map[string]BlogPostI18n{}
	}
	return out
}

// EncodeTranslations serializes the map back into the Translations column.
func (post *BlogPost) EncodeTranslations(translations map[string]BlogPostI18n) {
	cleaned := map[string]BlogPostI18n{}
	for lang, tr := range translations {
		code := common.NormalizeContentLanguage(lang)
		if code == "" || code == common.DefaultContentLanguage {
			continue
		}
		tr.Title = strings.TrimSpace(tr.Title)
		tr.Summary = strings.TrimSpace(tr.Summary)
		tr.SeoDescription = strings.TrimSpace(tr.SeoDescription)
		tr.Tags = strings.TrimSpace(tr.Tags)
		if tr.Title == "" && tr.Summary == "" && strings.TrimSpace(tr.Content) == "" && tr.SeoDescription == "" && tr.Tags == "" {
			continue
		}
		cleaned[code] = tr
	}
	if len(cleaned) == 0 {
		post.Translations = ""
		return
	}
	data, err := common.Marshal(cleaned)
	if err != nil {
		common.SysError("failed to encode blog translations: " + err.Error())
		post.Translations = ""
		return
	}
	post.Translations = string(data)
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

// AdImpression logs one shown ad (custom image or AdSense unit) so admins can
// audit counts and download the rows as CSV.
type AdImpression struct {
	Id        int64     `json:"id" gorm:"primaryKey"`
	AdId      string    `json:"ad_id" gorm:"type:varchar(128);index"`
	IsAdsense bool      `json:"is_adsense"`
	Ip        string    `json:"ip" gorm:"type:varchar(64)"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func RecordAdImpression(adId string, isAdsense bool, ip string) {
	if len(adId) > 128 || len(ip) > 64 {
		return
	}
	if err := DB.Create(&AdImpression{AdId: adId, IsAdsense: isAdsense, Ip: ip}).Error; err != nil {
		common.SysError("failed to record ad impression: " + err.Error())
	}
}

func GetPublishedBlogPostSummaries() ([]*BlogPost, error) {
	var posts []*BlogPost
	err := DB.Where("published = ?", true).Order("id ASC").Find(&posts).Error
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
		"translations":    post.Translations,
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

// SetBlogReaction is serialized process-wide: two rapid clicks (like then
// dislike) must never read the same pre-update state and double-apply the
// counter deltas. Counters are recomputed from the reaction rows at the end
// of every mutation, so they stay correct (and self-heal) no matter what.
var blogReactionMutex sync.Mutex

// SetBlogReaction upserts/toggles a reaction and keeps the denormalized
// counters on the target row in sync. Returns the user's resulting value
// (0 when the reaction was removed).
func SetBlogReaction(userId int, postId int64, targetType string, targetId int64, value int) (int, error) {
	blogReactionMutex.Lock()
	defer blogReactionMutex.Unlock()

	result := 0
	err := DB.Transaction(func(tx *gorm.DB) error {
		var existing BlogReaction
		findErr := tx.Where("target_type = ? AND target_id = ? AND user_id = ?", targetType, targetId, userId).
			First(&existing).Error
		if findErr != nil && findErr != gorm.ErrRecordNotFound {
			return findErr
		}

		switch {
		case findErr == gorm.ErrRecordNotFound:
			if err := tx.Create(&BlogReaction{BlogId: postId, TargetType: targetType, TargetId: targetId, UserId: userId, Value: value}).Error; err != nil {
				return err
			}
			result = value
		case existing.Value == value:
			// Toggle off.
			if err := tx.Delete(&existing).Error; err != nil {
				return err
			}
		default:
			// Switch sides.
			if err := tx.Model(&existing).Update("value", value).Error; err != nil {
				return err
			}
			result = value
		}

		var likeCount, dislikeCount int64
		if err := tx.Model(&BlogReaction{}).
			Where("target_type = ? AND target_id = ? AND value = 1", targetType, targetId).
			Count(&likeCount).Error; err != nil {
			return err
		}
		if err := tx.Model(&BlogReaction{}).
			Where("target_type = ? AND target_id = ? AND value = -1", targetType, targetId).
			Count(&dislikeCount).Error; err != nil {
			return err
		}
		return tx.Model(blogReactionTarget(targetType)).Where("id = ?", targetId).
			Updates(map[string]any{"like_count": likeCount, "dislike_count": dislikeCount}).Error
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
