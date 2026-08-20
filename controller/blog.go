package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

func blogEnabled() bool {
	return common.OptionMap["BlogEnabled"] == "true"
}

func blogDisabled(c *gin.Context) bool {
	if blogEnabled() {
		return false
	}
	c.JSON(http.StatusOK, gin.H{"success": false, "message": "blog is not enabled"})
	return true
}

// GetBlogPosts lists published posts for the public blog page.
func GetBlogPosts(c *gin.Context) {
	if blogDisabled(c) {
		return
	}
	posts, err := model.GetPublishedBlogPostSummaries()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	// Oldest first in the payload; the client reverses it so the newest post
	// renders top-left.
	items := make([]gin.H, 0, len(posts))
	for _, post := range posts {
		items = append(items, gin.H{
			"id":         post.Id,
			"title":      post.Title,
			"summary":    post.Summary,
			"cover":      post.CoverImage,
			"tags":       post.TagList(),
			"created_at": post.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"items": items,
			"total": len(items),
		},
	})
}

// GetBlogPost returns one published post with its comments and, for signed-in
// users, their reactions.
func GetBlogPost(c *gin.Context) {
	if blogDisabled(c) {
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid id"})
		return
	}
	post, err := model.GetBlogPostById(id)
	if err != nil || !post.Published {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "post not found"})
		return
	}
	comments, err := model.GetBlogComments(post.Id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	commentItems := make([]gin.H, 0, len(comments))
	for _, comment := range comments {
		commentItems = append(commentItems, gin.H{
			"id":            comment.Id,
			"post_id":       comment.PostId,
			"user_id":       comment.UserId,
			"username":      comment.Username,
			"avatar":        comment.Avatar,
			"parent_id":     comment.ParentId,
			"content":       comment.Content,
			"like_count":    comment.LikeCount,
			"dislike_count": comment.DislikeCount,
			"created_at":    comment.CreatedAt,
		})
	}

	reactions := map[string]int{}
	if userId := c.GetInt("id"); userId > 0 {
		if userReactions, err := model.GetUserBlogReactions(userId, post.Id); err == nil {
			reactions = userReactions
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"post": gin.H{
				"id":              post.Id,
				"title":           post.Title,
				"summary":         post.Summary,
				"content":         post.Content,
				"cover_image":     post.CoverImage,
				"tags":            post.TagList(),
				"seo_description": post.SeoDescription,
				"like_count":      post.LikeCount,
				"dislike_count":   post.DislikeCount,
				"created_at":      post.CreatedAt,
				"updated_at":      post.UpdatedAt,
			},
			"comments":  commentItems,
			"reactions": reactions,
		},
	})
}

type blogPostRequest struct {
	Title          string `json:"title"`
	Summary        string `json:"summary"`
	Content        string `json:"content"`
	CoverImage     string `json:"cover_image"`
	Tags           string `json:"tags"`
	SeoDescription string `json:"seo_description"`
	Published      bool   `json:"published"`
}

func sanitizeBlogPostRequest(req *blogPostRequest) {
	req.Title = strings.TrimSpace(req.Title)
	req.Summary = strings.TrimSpace(req.Summary)
	req.Tags = strings.TrimSpace(req.Tags)
	req.CoverImage = strings.TrimSpace(req.CoverImage)
	req.SeoDescription = strings.TrimSpace(req.SeoDescription)
}

// GetAllBlogPosts lists every post (including drafts) for administrators.
func GetAllBlogPosts(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	posts, err := model.GetBlogPosts(page, pageSize, false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	total, _ := model.GetBlogPostCount(false)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"items":     posts,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

func CreateBlogPost(c *gin.Context) {
	var req blogPostRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	sanitizeBlogPostRequest(&req)
	if req.Title == "" || strings.TrimSpace(req.Content) == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "title and content are required"})
		return
	}
	post := &model.BlogPost{
		Title:          req.Title,
		Summary:        req.Summary,
		Content:        req.Content,
		CoverImage:     req.CoverImage,
		Tags:           req.Tags,
		SeoDescription: req.SeoDescription,
		Published:      req.Published,
	}
	if err := model.CreateBlogPost(post); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": post})
}

func UpdateBlogPost(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid id"})
		return
	}
	var req blogPostRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	sanitizeBlogPostRequest(&req)
	if req.Title == "" || strings.TrimSpace(req.Content) == "" {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "title and content are required"})
		return
	}
	post := &model.BlogPost{
		Id:             id,
		Title:          req.Title,
		Summary:        req.Summary,
		Content:        req.Content,
		CoverImage:     req.CoverImage,
		Tags:           req.Tags,
		SeoDescription: req.SeoDescription,
		Published:      req.Published,
	}
	if err := model.UpdateBlogPost(post); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "updated"})
}

func DeleteBlogPost(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid id"})
		return
	}
	if err := model.DeleteBlogPost(id); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "deleted"})
}

type blogCommentRequest struct {
	Content  string `json:"content"`
	ParentId int64  `json:"parent_id"`
}

const maxBlogCommentLength = 2000

// CreateBlogComment adds a comment or a reply for a signed-in member.
func CreateBlogComment(c *gin.Context) {
	if blogDisabled(c) {
		return
	}
	postId, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid id"})
		return
	}
	post, err := model.GetBlogPostById(postId)
	if err != nil || !post.Published {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "post not found"})
		return
	}
	var req blogCommentRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	content := strings.TrimSpace(req.Content)
	if content == "" || len(content) > maxBlogCommentLength {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "comment must be 1-2000 characters"})
		return
	}
	if req.ParentId != 0 {
		parent, err := model.GetBlogCommentById(req.ParentId)
		if err != nil || parent.PostId != postId {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "parent comment not found"})
			return
		}
	}
	userId := c.GetInt("id")
	user, err := model.GetUserById(userId, false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	comment := &model.BlogComment{
		BlogId:   postId,
		PostId:   postId,
		UserId:   userId,
		Username: user.Username,
		Avatar:   user.AvatarUrl,
		ParentId: req.ParentId,
		Content:  content,
	}
	if err := model.CreateBlogComment(comment); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": comment})
}

// DeleteBlogComment lets members remove their own comments; admins can remove
// any comment. Replies go with the deleted comment.
func DeleteBlogComment(c *gin.Context) {
	commentId, err := strconv.ParseInt(c.Param("commentId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid id"})
		return
	}
	comment, err := model.GetBlogCommentById(commentId)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "comment not found"})
		return
	}
	userId := c.GetInt("id")
	if comment.UserId != userId && c.GetInt("role") < common.RoleAdminUser {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "permission denied"})
		return
	}
	if err := model.DeleteBlogComment(commentId); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "deleted"})
}

type blogReactionRequest struct {
	TargetType string `json:"target_type"` // post | comment
	Value      int    `json:"value"`       // 1 like, -1 dislike
}

// ReactBlog sets or toggles the signed-in member's reaction on a post or one
// of its comments.
func ReactBlog(c *gin.Context) {
	if blogDisabled(c) {
		return
	}
	postId, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid id"})
		return
	}
	var req blogReactionRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	if req.Value != 1 && req.Value != -1 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid reaction"})
		return
	}
	var targetId int64
	switch req.TargetType {
	case "post":
		post, err := model.GetBlogPostById(postId)
		if err != nil || !post.Published {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "post not found"})
			return
		}
		targetId = postId
	case "comment":
		commentId, err := strconv.ParseInt(c.Param("commentId"), 10, 64)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid id"})
			return
		}
		comment, err := model.GetBlogCommentById(commentId)
		if err != nil || comment.PostId != postId {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "comment not found"})
			return
		}
		targetId = commentId
	default:
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "invalid target"})
		return
	}

	userId := c.GetInt("id")
	newValue, err := model.SetBlogReaction(userId, postId, req.TargetType, targetId, req.Value)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"value": newValue}})
}
