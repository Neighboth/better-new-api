/*
Copyright (C) 2023-2026 QuantumNous
This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

For commercial licensing, please contact support@quantumnous.com
*/

package controller

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
)

// fetchWebpageMaxBodyBytes caps the body of one fetched page.
const fetchWebpageMaxBodyBytes = 512 * 1024

// fetchWebpageTimeout is the total timeout of one fetch.
const fetchWebpageTimeout = 15 * time.Second

var (
	fetchScriptStyleRe = regexp.MustCompile(`(?is)<(script|style|noscript|template|svg)[^>]*>.*?</(script|style|noscript|template|svg)>`)
	fetchBlockBreakRe  = regexp.MustCompile(`(?is)</?(p|div|br|hr|li|ul|ol|dl|table|tr|td|th|h1|h2|h3|h4|h5|h6|section|article|header|footer|nav|aside|blockquote|pre|figure|figcaption)[^>]*>`)
	fetchListItemRe    = regexp.MustCompile(`(?is)<li(?:\s[^>]*)?>`)
	fetchTagRe         = regexp.MustCompile(`(?is)<[^>]+>`)
	fetchSpaceRe       = regexp.MustCompile(`[\t\x{00a0} ]+`)
	fetchManyNewlines  = regexp.MustCompile(`\n{3,}`)
	fetchTitleRe       = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	fetchCommentRe     = regexp.MustCompile(`(?is)<!--.*?-->`)
	fetchHeadingRe     = regexp.MustCompile(`(?is)<h([1-6])[^>]*>(.*?)</h[1-6]>`)
)

type fetchWebpageRequest struct {
	URL string `json:"url" binding:"required"`
}

// blockTagToNewline converts the block-level tags to newlines so the raw
// text keeps the paragraph structure of the source page.
func blockTagToNewline(htmlText string) string {
	withMark := fetchBlockBreakRe.ReplaceAllString(htmlText, "\n")
	withMark = fetchListItemRe.ReplaceAllString(withMark, "\n- ")
	return withMark
}

// htmlToMarkdown converts a page into readable markdown-ish plain text. The
// goal is giving a model the content of a page, not reproducing the styling.
func htmlToMarkdown(htmlText string) string {
	text := fetchCommentRe.ReplaceAllString(htmlText, "")
	text = fetchScriptStyleRe.ReplaceAllString(text, "")

	// Extract and normalize the page title for the header line.
	title := ""
	if match := fetchTitleRe.FindStringSubmatch(text); len(match) == 2 {
		title = strings.TrimSpace(html.UnescapeString(fetchTagRe.ReplaceAllString(match[1], "")))
	}

	text = blockTagToNewline(text)

	// Headings become markdown headings so the model can see the outline.
	text = fetchHeadingRe.ReplaceAllStringFunc(text, func(match string) string {
		parts := fetchHeadingRe.FindStringSubmatch(match)
		level := len(parts[1])
		inner := strings.TrimSpace(html.UnescapeString(fetchTagRe.ReplaceAllString(parts[2], "")))
		if inner == "" {
			return ""
		}
		return "\n\n" + strings.Repeat("#", level) + " " + inner + "\n\n"
	})

	text = fetchTagRe.ReplaceAllString(text, "")
	text = html.UnescapeString(text)

	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = fetchSpaceRe.ReplaceAllString(strings.TrimSpace(line), " ")
		if line == "" {
			out = append(out, "")
			continue
		}
		out = append(out, line)
	}
	text = strings.Join(out, "\n")
	text = fetchManyNewlines.ReplaceAllString(text, "\n\n")
	text = strings.TrimSpace(text)

	var sb strings.Builder
	if title != "" {
		sb.WriteString("# ")
		sb.WriteString(title)
		sb.WriteString("\n\n")
	}
	sb.WriteString(text)
	return strings.TrimSpace(sb.String())
}

// PlaygroundFetchWebpage fetches one public web page and returns readable
// markdown. The endpoint exists for the `fetch_webpage` tool of the blog AI
// assistant so the model can read many pages in one session. It is available
// only when the installation enables the blog research toggle.
func PlaygroundFetchWebpage(c *gin.Context) {
	setting := operation_setting.GetPlaygroundSetting()
	blogEnabled := setting != nil && setting.WebSearchBlogEnabled
	if !blogEnabled {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "fetch_webpage is disabled on this installation"})
		return
	}

	var req fetchWebpageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "url is required"})
		return
	}
	target := strings.TrimSpace(req.URL)
	if target == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "url is required"})
		return
	}
	if err := service.ValidateSSRFProtectedFetchURL(target); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "url is not allowed: " + err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), fetchWebpageTimeout)
	defer cancel()

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "url is not valid"})
		return
	}
	httpRequest.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain,text/markdown")
	httpRequest.Header.Set("User-Agent", "Mozilla/5.0 (compatible; NewAPIFetch/1.0; +https://github.com/QuantumNous/new-api)")

	client := service.GetSSRFProtectedHTTPClient()
	resp, err := client.Do(httpRequest)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "could not fetch the page: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": fmt.Sprintf("the page answered %d", resp.StatusCode)})
		return
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	textTypes := []string{"text/html", "application/xhtml+xml", "text/plain", "text/markdown", "application/xml", "text/xml"}
	contentOK := false
	for _, part := range textTypes {
		if strings.Contains(contentType, part) {
			contentOK = true
			break
		}
	}
	if contentType != "" && !contentOK {
		c.JSON(http.StatusUnsupportedMediaType, gin.H{"success": false, "message": "content type is not text: " + contentType})
		return
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, fetchWebpageMaxBodyBytes))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "could not read the page body"})
		return
	}

	markdown := htmlToMarkdown(string(body))
	if len(markdown) > 24000 {
		markdown = markdown[:24000] + "\n\n[...truncated...]"
	}
	title := ""
	if match := fetchTitleRe.FindStringSubmatch(string(body)); len(match) == 2 {
		title = strings.TrimSpace(html.UnescapeString(fetchTagRe.ReplaceAllString(match[1], "")))
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"url":      target,
			"title":    title,
			"markdown": markdown,
			"bytes":    len(body),
			"fetched":  time.Now().Unix(),
		},
	})
}
