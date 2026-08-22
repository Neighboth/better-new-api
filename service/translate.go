package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// Machine translation via the unofficial Google Translate "gtx" endpoint:
//
//	GET translate.googleapis.com/translate_a/single?client=gtx&sl=<src>&tl=<dst>&dt=t&q=<text>
//
// The response is a nested JSON array whose [0] element lists translated
// sentence segments; each segment's [0] element holds the translated text.
const googleTranslateEndpoint = "https://translate.googleapis.com/translate_a/single"

const (
	translateRequestTimeout = 30 * time.Second
	// gtx rejects overly long q values; split long texts at sentence
	// boundaries before calling it.
	translateMaxChunkChars = 1500
	translateCacheMaxSize  = 20000
	// TranslateTexts fans out with this many workers so translating a full
	// markdown post (one request per paragraph) stays interactive.
	translateConcurrency = 4
)

var (
	translateCache      sync.Map // cacheKey -> string
	translateCacheCount atomic.Int64
)

func translateCacheKey(sl, tl, text string) string {
	sum := sha256.Sum256([]byte(text))
	return sl + "|" + tl + "|" + hex.EncodeToString(sum[:])
}

func translateCacheGet(sl, tl, text string) (string, bool) {
	value, ok := translateCache.Load(translateCacheKey(sl, tl, text))
	if !ok {
		return "", false
	}
	return value.(string), true
}

func translateCacheSet(sl, tl, text, translated string) {
	if translateCacheCount.Add(1) > translateCacheMaxSize {
		// Cheap eviction: drop everything and start over. Translation results
		// are deterministic, so a full reset only costs a warm-up round.
		translateCache.Clear()
		translateCacheCount.Store(1)
	}
	translateCache.Store(translateCacheKey(sl, tl, text), translated)
}

// Patterns for content that must pass through translation untouched:
// placeholders (i18next {{var}}), URLs, emails, fenced/inline code.
var translateProtectedPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?s)` + "```" + `.*?` + "```"),
	regexp.MustCompile("`[^`\n]*`"),
	regexp.MustCompile(`\{\{[^{}]*\}\}`),
	regexp.MustCompile(`https?://[^\s)\]"'>]+`),
	regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`),
}

// technicalToken matches tokens such as model names ("gpt-4o",
// "claude-sonnet-4"), endpoints ("api.example.com/v1") and version strings
// that must never be translated. It requires the token to carry a digit,
// dash, dot or slash so plain words never match.
var technicalToken = regexp.MustCompile(`^[A-Za-z0-9_./+:-]*[0-9./:-][A-Za-z0-9_./+:-]*$`)

type translateSegment struct {
	text      string
	protected bool
}

// splitTranslateSegments partitions text into protected spans (kept verbatim)
// and plain prose spans (sent to the translator).
func splitTranslateSegments(text string) []translateSegment {
	type span struct{ start, end int }
	var spans []span
	for _, pattern := range translateProtectedPatterns {
		for _, loc := range pattern.FindAllStringIndex(text, -1) {
			spans = append(spans, span{loc[0], loc[1]})
		}
	}
	if len(spans) == 0 {
		return []translateSegment{{text: text}}
	}
	// Sort and drop overlaps (earlier patterns win).
	for i := 1; i < len(spans); i++ {
		for j := i; j > 0 && spans[j-1].start > spans[j].start; j-- {
			spans[j-1], spans[j] = spans[j], spans[j-1]
		}
	}
	merged := spans[:0]
	for _, sp := range spans {
		if len(merged) > 0 && sp.start < merged[len(merged)-1].end {
			continue
		}
		merged = append(merged, sp)
	}

	var segments []translateSegment
	cursor := 0
	for _, sp := range merged {
		if sp.start > cursor {
			segments = append(segments, translateSegment{text: text[cursor:sp.start]})
		}
		segments = append(segments, translateSegment{text: text[sp.start:sp.end], protected: true})
		cursor = sp.end
	}
	if cursor < len(text) {
		segments = append(segments, translateSegment{text: text[cursor:]})
	}
	return segments
}

// isTranslatable reports whether a text carries prose worth sending to the
// translator. Pure technical tokens (model names, endpoints, versions,
// numbers) and whitespace-only fragments are returned verbatim.
func isTranslatable(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	hasLetter := false
	for _, r := range trimmed {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r > 127 {
			hasLetter = true
			break
		}
	}
	if !hasLetter {
		return false
	}
	// A whole-text technical token (e.g. "gpt-4o-mini", "v1/chat/completions")
	// is kept as-is; such tokens inside a sentence are protected per-word in
	// protectInlineTokens.
	if !strings.ContainsAny(trimmed, " \t\n") && technicalToken.MatchString(trimmed) {
		return false
	}
	return true
}

// protectInlineTokens shields technical single tokens inside a prose segment
// by splitting them out as protected sub-segments.
func protectInlineTokens(segment string) []translateSegment {
	fields := strings.Fields(segment)
	needsSplit := false
	for _, field := range fields {
		trimmed := strings.Trim(field, "()[]{}<>,;:!?\"'")
		if len(trimmed) > 2 && technicalToken.MatchString(trimmed) && strings.ContainsAny(trimmed, ".-/:") {
			needsSplit = true
			break
		}
	}
	if !needsSplit {
		return []translateSegment{{text: segment}}
	}
	var out []translateSegment
	remaining := segment
	for _, field := range fields {
		idx := strings.Index(remaining, field)
		if idx < 0 {
			continue
		}
		prefix := remaining[:idx]
		core := strings.Trim(field, "()[]{}<>,;:!?\"'")
		leadLen := strings.Index(field, core)
		if len(core) > 2 && technicalToken.MatchString(core) && strings.ContainsAny(core, ".-/:") {
			lead := field[:leadLen]
			trail := field[leadLen+len(core):]
			out = append(out, translateSegment{text: prefix + lead})
			out = append(out, translateSegment{text: core, protected: true})
			remaining = trail + remaining[idx+len(field):]
		} else {
			out = append(out, translateSegment{text: prefix + field})
			remaining = remaining[idx+len(field):]
		}
	}
	if remaining != "" {
		out = append(out, translateSegment{text: remaining})
	}
	// Merge adjacent non-protected pieces back together.
	merged := out[:0]
	for _, seg := range out {
		if len(merged) > 0 && !seg.protected && !merged[len(merged)-1].protected {
			merged[len(merged)-1].text += seg.text
			continue
		}
		merged = append(merged, seg)
	}
	return merged
}

// splitLongText breaks text into chunks under limit characters, preferring
// paragraph, newline and sentence boundaries.
func splitLongText(text string, limit int) []string {
	if len(text) <= limit {
		return []string{text}
	}
	var chunks []string
	remaining := text
	for len(remaining) > limit {
		window := remaining[:limit]
		cut := strings.LastIndex(window, "\n\n")
		if cut < limit/2 {
			cut = strings.LastIndex(window, "\n")
		}
		if cut < limit/2 {
			if idx := strings.LastIndex(window, ". "); idx >= limit/2 {
				cut = idx + 1
			}
		}
		if cut < limit/2 {
			cut = strings.LastIndex(window, " ")
		}
		if cut <= 0 {
			cut = limit
		}
		chunks = append(chunks, remaining[:cut])
		remaining = remaining[cut:]
	}
	if remaining != "" {
		chunks = append(chunks, remaining)
	}
	return chunks
}

// googleTranslateOnce performs one gtx request for a single text.
func googleTranslateOnce(sl, tl, text string) (string, error) {
	if cached, ok := translateCacheGet(sl, tl, text); ok {
		return cached, nil
	}
	query := url.Values{}
	query.Set("client", "gtx")
	query.Set("sl", sl)
	query.Set("tl", tl)
	query.Set("dt", "t")
	query.Set("q", text)

	ctx, cancel := context.WithTimeout(context.Background(), translateRequestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleTranslateEndpoint+"?"+query.Encode(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; new-api translator)")

	resp, err := GetHttpClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("translate upstream returned status %d", resp.StatusCode)
	}

	var parsed []any
	if err := common.Unmarshal(body, &parsed); err != nil {
		return "", err
	}
	if len(parsed) == 0 {
		return "", fmt.Errorf("translate upstream returned an empty result")
	}
	segments, ok := parsed[0].([]any)
	if !ok {
		return "", fmt.Errorf("translate upstream returned an unexpected payload")
	}
	var b strings.Builder
	for _, segment := range segments {
		parts, ok := segment.([]any)
		if !ok || len(parts) == 0 {
			continue
		}
		if text, ok := parts[0].(string); ok {
			b.WriteString(text)
		}
	}
	translated := b.String()
	if translated == "" {
		return "", fmt.Errorf("translate upstream returned no text")
	}
	translateCacheSet(sl, tl, text, translated)
	return translated, nil
}

// TranslateText translates one text, preserving placeholders, URLs, code
// spans and technical tokens (model names, endpoints). On any failure the
// original text is returned, so callers always get displayable output.
func TranslateText(sl, tl, text string) string {
	if text == "" || sl == tl {
		return text
	}
	var b strings.Builder
	for _, segment := range splitTranslateSegments(text) {
		if segment.protected || !isTranslatable(segment.text) {
			b.WriteString(segment.text)
			continue
		}
		for _, sub := range protectInlineTokens(segment.text) {
			if sub.protected || !isTranslatable(sub.text) {
				b.WriteString(sub.text)
				continue
			}
			for _, chunk := range splitLongText(sub.text, translateMaxChunkChars) {
				translated, err := googleTranslateOnce(sl, tl, chunk)
				if err != nil {
					common.SysError("machine translation failed: " + err.Error())
					b.WriteString(chunk)
					continue
				}
				b.WriteString(translated)
			}
		}
	}
	return b.String()
}

// batchableText reports whether a text can safely ride in a newline-joined
// multi-line translation request: short, single-line, no protected spans.
func batchableText(text string) bool {
	if len(text) > 300 || strings.ContainsAny(text, "\n\r") {
		return false
	}
	for _, pattern := range translateProtectedPatterns {
		if pattern.MatchString(text) {
			return false
		}
	}
	return true
}

// translateLineBatch translates newline-joined short texts in one request.
// The translator preserves line structure; if the line count ever mismatches
// the caller falls back to per-text translation, so results stay correct.
func translateLineBatch(sl, tl string, texts []string) ([]string, bool) {
	joined := strings.Join(texts, "\n")
	translated, err := googleTranslateOnce(sl, tl, joined)
	if err != nil {
		return nil, false
	}
	lines := strings.Split(translated, "\n")
	if len(lines) != len(texts) {
		return nil, false
	}
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
		if lines[i] == "" {
			return nil, false
		}
	}
	return lines, true
}

// TranslateTexts translates a batch with bounded concurrency. Short UI
// strings are grouped into newline-joined requests (one upstream call per
// ~1200 chars); longer or structured texts go one by one. Untranslatable
// entries and failures fall back to the source text.
func TranslateTexts(sl, tl string, texts []string) []string {
	out := make([]string, len(texts))
	if sl == tl {
		copy(out, texts)
		return out
	}

	type batchItem struct {
		index int
		text  string
	}
	var singles []batchItem
	var group []batchItem
	var groups [][]batchItem
	groupChars := 0
	flushGroup := func() {
		if len(group) > 0 {
			groups = append(groups, group)
			group = nil
			groupChars = 0
		}
	}
	for i, text := range texts {
		if !isTranslatable(text) {
			out[i] = text
			continue
		}
		if !batchableText(text) {
			singles = append(singles, batchItem{i, text})
			continue
		}
		if groupChars+len(text)+1 > 1200 || len(group) >= 60 {
			flushGroup()
		}
		group = append(group, batchItem{i, text})
		groupChars += len(text) + 1
	}
	flushGroup()

	sem := make(chan struct{}, translateConcurrency)
	var wg sync.WaitGroup
	run := func(fn func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			fn()
		}()
	}
	for _, item := range singles {
		run(func() { out[item.index] = TranslateText(sl, tl, item.text) })
	}
	for _, grp := range groups {
		run(func() {
			grpTexts := make([]string, len(grp))
			for j, item := range grp {
				grpTexts[j] = item.text
			}
			translated, ok := translateLineBatch(sl, tl, grpTexts)
			if !ok {
				for j, item := range grp {
					out[item.index] = TranslateText(sl, tl, grpTexts[j])
				}
				return
			}
			for j, item := range grp {
				out[item.index] = translated[j]
			}
		})
	}
	wg.Wait()
	return out
}
