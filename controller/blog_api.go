package controller

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

const (
	maxBlogAPIJSONBytes = 50 << 20 // 50 MB for very long multi-language blog articles
)

var blogAPIKnownLocales = []string{"en", "tr", "zhCN", "zhTW", "fr", "ja", "ru", "vi"}

// flexTags accepts either a comma-separated string ("ai, guide") or a string array (["ai", "guide"]).
type flexTags string

func (t *flexTags) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		*t = ""
		return nil
	}
	var s string
	if err := common.Unmarshal(data, &s); err == nil {
		*t = flexTags(strings.TrimSpace(s))
		return nil
	}
	var arr []string
	if err := common.Unmarshal(data, &arr); err == nil {
		cleaned := make([]string, 0, len(arr))
		for _, item := range arr {
			if v := strings.TrimSpace(item); v != "" {
				cleaned = append(cleaned, v)
			}
		}
		*t = flexTags(strings.Join(cleaned, ", "))
		return nil
	}
	return fmt.Errorf("tags must be a string or string array")
}

type blogAPITranslationEntry struct {
	Title          *string   `json:"title"`
	Summary        *string   `json:"summary"`
	Content        *string   `json:"content"`
	Tags           *flexTags `json:"tags"`
	SeoDescription *string   `json:"seo_description"`
}

type blogAPIPostPayload struct {
	Title          *string                            `json:"title"`
	Summary        *string                            `json:"summary"`
	Content        *string                            `json:"content"`
	CoverImage     *string                            `json:"cover_image"`
	Banner         *string                            `json:"banner"`
	Tags           *flexTags                          `json:"tags"`
	SeoDescription *string                            `json:"seo_description"`
	Published      *bool                              `json:"published"`
	Localizations  *blogAPILocalizationsInput         `json:"localizations"`
	Translations   map[string]blogAPITranslationEntry `json:"translations"`
}

type blogAPILocalizationsInput struct {
	Titles          map[string]string `json:"titles"`
	Summaries       map[string]string `json:"summaries"`
	Contents        map[string]string `json:"contents"`
	TagsList        map[string]string `json:"tags_list"`
	SeoDescriptions map[string]string `json:"seo_descriptions"`
}

func serializeBlogAPIPost(c *gin.Context, post *model.BlogPost, includeContent bool) gin.H {
	baseURL := resolveExternalBaseURL(c)
	postPath := fmt.Sprintf("/blog/%d", post.Id)
	fullURL := postPath
	if baseURL != "" {
		fullURL = baseURL + postPath
	}

	titles := mustUnmarshalBlogLocalized(post.Titles)
	summaries := mustUnmarshalBlogLocalized(post.Summaries)
	contents := mustUnmarshalBlogLocalized(post.Contents)
	tagsList := mustUnmarshalBlogLocalized(post.TagsList)
	seoDescs := mustUnmarshalBlogLocalized(post.SeoDescriptions)

	if titles["en"] == "" && post.Title != "" {
		titles["en"] = post.Title
	}
	if summaries["en"] == "" && post.Summary != "" {
		summaries["en"] = post.Summary
	}
	if contents["en"] == "" && post.Content != "" {
		contents["en"] = post.Content
	}
	if tagsList["en"] == "" && post.Tags != "" {
		tagsList["en"] = post.Tags
	}
	if seoDescs["en"] == "" && post.SeoDescription != "" {
		seoDescs["en"] = post.SeoDescription
	}

	localeSet := map[string]struct{}{}
	for k := range titles {
		localeSet[k] = struct{}{}
	}
	for k := range summaries {
		localeSet[k] = struct{}{}
	}
	for k := range contents {
		localeSet[k] = struct{}{}
	}
	for k := range tagsList {
		localeSet[k] = struct{}{}
	}
	for k := range seoDescs {
		localeSet[k] = struct{}{}
	}

	translations := map[string]gin.H{}
	for loc := range localeSet {
		entry := gin.H{
			"title":           titles[loc],
			"summary":         summaries[loc],
			"tags":            splitBlogTags(tagsList[loc]),
			"seo_description": seoDescs[loc],
		}
		if includeContent {
			entry["content"] = contents[loc]
		}
		translations[loc] = entry
	}

	locPayload := gin.H{
		"titles":           titles,
		"summaries":        summaries,
		"tags_list":        tagsList,
		"seo_descriptions": seoDescs,
	}
	if includeContent {
		locPayload["contents"] = contents
	}

	out := gin.H{
		"id":              post.Id,
		"url":             fullURL,
		"path":            postPath,
		"title":           post.Title,
		"summary":         post.Summary,
		"cover_image":     post.CoverImage,
		"tags":            splitBlogTags(post.Tags),
		"seo_description": post.SeoDescription,
		"published":       post.Published,
		"like_count":      post.LikeCount,
		"dislike_count":   post.DislikeCount,
		"created_at":      post.CreatedAt,
		"updated_at":      post.UpdatedAt,
		"localizations":   locPayload,
		"translations":    translations,
	}
	if includeContent {
		out["content"] = post.Content
	}
	return out
}

func applyBlogAPIPayload(post *model.BlogPost, req *blogAPIPostPayload) {
	titles := mustUnmarshalBlogLocalized(post.Titles)
	summaries := mustUnmarshalBlogLocalized(post.Summaries)
	contents := mustUnmarshalBlogLocalized(post.Contents)
	tagsList := mustUnmarshalBlogLocalized(post.TagsList)
	seoDescs := mustUnmarshalBlogLocalized(post.SeoDescriptions)

	if req.Title != nil {
		post.Title = strings.TrimSpace(*req.Title)
		if post.Title != "" {
			titles["en"] = post.Title
		}
	}
	if req.Summary != nil {
		post.Summary = strings.TrimSpace(*req.Summary)
		summaries["en"] = post.Summary
	}
	if req.Content != nil {
		post.Content = strings.TrimSpace(*req.Content)
		if post.Content != "" {
			contents["en"] = post.Content
		}
	}
	if req.CoverImage != nil {
		post.CoverImage = strings.TrimSpace(*req.CoverImage)
	} else if req.Banner != nil {
		post.CoverImage = strings.TrimSpace(*req.Banner)
	}
	if req.Tags != nil {
		post.Tags = strings.TrimSpace(string(*req.Tags))
		tagsList["en"] = post.Tags
	}
	if req.SeoDescription != nil {
		post.SeoDescription = strings.TrimSpace(*req.SeoDescription)
		seoDescs["en"] = post.SeoDescription
	}
	if req.Published != nil {
		post.Published = *req.Published
	}

	if req.Localizations != nil {
		for k, v := range req.Localizations.Titles {
			titles[model.NormalizeBlogLocale(k)] = strings.TrimSpace(v)
		}
		for k, v := range req.Localizations.Summaries {
			summaries[model.NormalizeBlogLocale(k)] = strings.TrimSpace(v)
		}
		for k, v := range req.Localizations.Contents {
			contents[model.NormalizeBlogLocale(k)] = strings.TrimSpace(v)
		}
		for k, v := range req.Localizations.TagsList {
			tagsList[model.NormalizeBlogLocale(k)] = strings.TrimSpace(v)
		}
		for k, v := range req.Localizations.SeoDescriptions {
			seoDescs[model.NormalizeBlogLocale(k)] = strings.TrimSpace(v)
		}
	}

	for rawLocale, tr := range req.Translations {
		loc := model.NormalizeBlogLocale(rawLocale)
		if tr.Title != nil {
			titles[loc] = strings.TrimSpace(*tr.Title)
		}
		if tr.Summary != nil {
			summaries[loc] = strings.TrimSpace(*tr.Summary)
		}
		if tr.Content != nil {
			contents[loc] = strings.TrimSpace(*tr.Content)
		}
		if tr.Tags != nil {
			tagsList[loc] = strings.TrimSpace(string(*tr.Tags))
		}
		if tr.SeoDescription != nil {
			seoDescs[loc] = strings.TrimSpace(*tr.SeoDescription)
		}
	}

	// Ensure scalar fallback columns stay populated even when caller only sent non-English translations.
	pickFallback := func(m map[string]string) string {
		if v := strings.TrimSpace(m["en"]); v != "" {
			return v
		}
		for _, loc := range blogAPIKnownLocales {
			if v := strings.TrimSpace(m[loc]); v != "" {
				return v
			}
		}
		for _, v := range m {
			if tv := strings.TrimSpace(v); tv != "" {
				return tv
			}
		}
		return ""
	}
	if post.Title == "" {
		post.Title = pickFallback(titles)
	}
	if post.Summary == "" {
		post.Summary = pickFallback(summaries)
	}
	if post.Content == "" {
		post.Content = pickFallback(contents)
	}
	if post.Tags == "" {
		post.Tags = pickFallback(tagsList)
	}
	if post.SeoDescription == "" {
		post.SeoDescription = pickFallback(seoDescs)
	}

	if len(titles)+len(summaries)+len(contents)+len(tagsList)+len(seoDescs) > 0 {
		post.SetLocalized(titles, summaries, contents, tagsList, seoDescs)
		if post.Title == "" {
			post.Title = pickFallback(titles)
		}
		if post.Content == "" {
			post.Content = pickFallback(contents)
		}
	}
}

func decodeLargeJSON(r io.Reader, limit int64, dst any) error {
	raw, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return err
	}
	if int64(len(raw)) > limit {
		return fmt.Errorf("request body exceeds maximum allowed size (%d MB)", limit/(1<<20))
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return fmt.Errorf("request body is empty")
	}
	return common.Unmarshal(raw, dst)
}

// BlogAPIListPosts lists blog posts with full URLs and localization maps.
// GET /api/v1/blog/posts?page=1&page_size=50&published=all|true|false&include_content=true&search=
func BlogAPIListPosts(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	includeContent := !strings.EqualFold(strings.TrimSpace(c.Query("include_content")), "false")
	pubFilter := strings.ToLower(strings.TrimSpace(c.DefaultQuery("published", "all")))
	search := strings.TrimSpace(c.Query("search"))

	tx := model.DB.Model(&model.BlogPost{})
	switch pubFilter {
	case "true", "1":
		tx = tx.Where("published = ?", true)
	case "false", "0":
		tx = tx.Where("published = ?", false)
	}
	if search != "" {
		like := "%" + search + "%"
		tx = tx.Where("title LIKE ? OR summary LIKE ? OR tags LIKE ? OR titles LIKE ?", like, like, like, like)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	var posts []*model.BlogPost
	if err := tx.Order("created_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&posts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	items := make([]gin.H, 0, len(posts))
	for _, p := range posts {
		items = append(items, serializeBlogAPIPost(c, p, includeContent))
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"items":             items,
			"total":             total,
			"page":              page,
			"page_size":         pageSize,
			"supported_locales": blogAPIKnownLocales,
		},
	})
}

// BlogAPIGetPost returns a single blog post with full URL and all localizations.
// GET /api/v1/blog/posts/:id
func BlogAPIGetPost(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid post ID"})
		return
	}
	post, err := model.GetBlogPostById(id)
	if err != nil || post == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Post not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    serializeBlogAPIPost(c, post, true),
	})
}

// BlogAPICreatePost creates a new multi-language blog post (up to 50 MB body).
// POST /api/v1/blog/posts
func BlogAPICreatePost(c *gin.Context) {
	var req blogAPIPostPayload
	if err := decodeLargeJSON(c.Request.Body, maxBlogAPIJSONBytes, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	post := &model.BlogPost{}
	applyBlogAPIPayload(post, &req)
	if strings.TrimSpace(post.Title) == "" || strings.TrimSpace(post.Content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "At least one title and content (top-level or in translations/localizations) is required"})
		return
	}
	if err := model.CreateBlogPost(post); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    serializeBlogAPIPost(c, post, true),
	})
}

// BlogAPIUpdatePost updates an existing blog post (supports partial PATCH or full PUT).
// PUT/PATCH /api/v1/blog/posts/:id
func BlogAPIUpdatePost(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid post ID"})
		return
	}
	existing, err := model.GetBlogPostById(id)
	if err != nil || existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Post not found"})
		return
	}
	var req blogAPIPostPayload
	if err := decodeLargeJSON(c.Request.Body, maxBlogAPIJSONBytes, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	applyBlogAPIPayload(existing, &req)
	if strings.TrimSpace(existing.Title) == "" || strings.TrimSpace(existing.Content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Title and content cannot be empty"})
		return
	}
	if err := model.UpdateBlogPost(existing); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	updated, err := model.GetBlogPostById(id)
	if err != nil {
		updated = existing
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    serializeBlogAPIPost(c, updated, true),
	})
}

// BlogAPIDeletePost permanently removes a blog post.
// DELETE /api/v1/blog/posts/:id
func BlogAPIDeletePost(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid post ID"})
		return
	}
	if _, err := model.GetBlogPostById(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "Post not found"})
		return
	}
	if err := model.DeleteBlogPost(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "deleted",
	})
}
