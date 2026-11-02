// Package markdownrtf turns the policies of the installation into the Rich Text
// Format of the page of the installation.
//
// The page of the policies of the installer of Windows is a RichEdit control:
// it shows RTF and it does not show Markdown. The administrator of the
// installation writes the policies in Markdown, the same text of the page of
// the first start of the client, therefore the text is converted here and the
// installer shows the result as it is.
package markdownrtf

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// Table column widths are measured in twips, the unit of the format.
const (
	tableWidthTwips = 6200
	minimumColumn   = 700
	twipsPerChar    = 110
	ruleCharacters  = 58
)

var gm = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
		extension.Table,
	),
)

// Convert renders a Markdown document as a Rich Text Format document using
// the official goldmark CommonMark & GFM parser.
func Convert(markdown string) string {
	var body strings.Builder
	src := []byte(strings.ReplaceAll(markdown, "\r\n", "\n"))
	reader := text.NewReader(src)
	doc := gm.Parser().Parse(reader)

	for child := doc.FirstChild(); child != nil; child = child.NextSibling() {
		renderBlock(&body, child, src)
	}

	return `{\rtf1\ansi\ansicpg1252\deff0\uc1` +
		`{\fonttbl{\f0\fnil\fcharset0 Segoe UI;}{\f1\fnil\fcharset0 Consolas;}}` +
		`{\colortbl;\red37\green99\blue235;\red90\green98\blue115;}` +
		`\f0\fs20 ` + body.String() + `}`
}

func renderBlock(body *strings.Builder, node ast.Node, src []byte) {
	switch n := node.(type) {
	case *ast.Heading:
		sizes := [6]int{26, 22, 20, 18, 18, 18}
		size := sizes[0]
		if n.Level >= 1 && n.Level <= 6 {
			size = sizes[n.Level-1]
		}
		body.WriteString(`\pard\li0\fi0\b\fs` + strconv.Itoa(size) + ` `)
		renderInlines(body, n, src)
		body.WriteString(`\b0\fs20\par `)

	case *ast.ThematicBreak:
		body.WriteString(`\pard\li0\fi0{\f1 ` + strings.Repeat(escapeRune('─'), ruleCharacters) + `}\par `)

	case *ast.Paragraph:
		body.WriteString(`\pard\li0\fi0 `)
		renderInlines(body, n, src)
		body.WriteString(`\par `)

	case *ast.Blockquote:
		body.WriteString(`\pard\li360\ri360\i `)
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			renderInlines(body, child, src)
		}
		body.WriteString(`\i0\par `)

	case *ast.FencedCodeBlock:
		renderCodeBlock(body, n, src)

	case *ast.CodeBlock:
		renderCodeBlock(body, n, src)

	case *ast.List:
		renderList(body, n, src, 0)

	case *extast.Table:
		renderTable(body, n, src)

	default:
		for child := n.FirstChild(); child != nil; child = child.NextSibling() {
			renderBlock(body, child, src)
		}
	}
}

func renderCodeBlock(body *strings.Builder, node ast.Node, src []byte) {
	lines := node.Lines()
	for i := 0; i < lines.Len(); i++ {
		line := lines.At(i)
		body.WriteString(`\pard\li360\f1\fs18 ` + escapeString(string(line.Value(src))) + `\par `)
	}
}

func renderList(body *strings.Builder, list *ast.List, src []byte, level int) {
	itemIndex := list.Start
	for item := list.FirstChild(); item != nil; item = item.NextSibling() {
		indent := 360 + level*360
		marker := escapeRune('\u2022')
		if list.IsOrdered() {
			marker = strconv.Itoa(itemIndex) + "."
			itemIndex++
		}
		body.WriteString(`\pard\li` + strconv.Itoa(indent) + `\fi-180 ` + marker + `\tab `)
		for child := item.FirstChild(); child != nil; child = child.NextSibling() {
			if subList, ok := child.(*ast.List); ok {
				renderList(body, subList, src, level+1)
			} else {
				renderInlines(body, child, src)
			}
		}
		body.WriteString(`\par `)
	}
}

func renderTable(body *strings.Builder, table *extast.Table, src []byte) {
	var rows [][]string
	for rowNode := table.FirstChild(); rowNode != nil; rowNode = rowNode.NextSibling() {
		var cells []string
		for cellNode := rowNode.FirstChild(); cellNode != nil; cellNode = cellNode.NextSibling() {
			var cellBuilder strings.Builder
			renderInlines(&cellBuilder, cellNode, src)
			cells = append(cells, cellBuilder.String())
		}
		if len(cells) > 0 {
			rows = append(rows, cells)
		}
	}

	if len(rows) == 0 {
		return
	}

	columns := 0
	for _, cells := range rows {
		if len(cells) > columns {
			columns = len(cells)
		}
	}
	if columns == 0 {
		return
	}

	widths := make([]int, columns)
	for col := range widths {
		widths[col] = 4
	}
	for _, cells := range rows {
		for col := 0; col < columns && col < len(cells); col++ {
			cleanText := stripRtfControl(cells[col])
			if l := runeLen(cleanText); l > widths[col] {
				widths[col] = l
			}
		}
	}

	total := 0
	for col := range widths {
		widths[col] = widths[col]*twipsPerChar + minimumColumn
		total += widths[col]
	}
	stops := make([]int, columns)
	pos := 0
	for col := range widths {
		width := widths[col]
		if total > tableWidthTwips {
			width = width * tableWidthTwips / total
		}
		pos += width
		stops[col] = pos
	}

	var tabs strings.Builder
	for _, stop := range stops {
		tabs.WriteString(`\tx` + strconv.Itoa(stop))
	}
	tabStops := tabs.String()

	for rowIndex, cells := range rows {
		body.WriteString(`\pard\li0\fi0` + tabStops + ` `)
		for col := 0; col < columns; col++ {
			if col > 0 {
				body.WriteString(`\tab `)
			}
			if col >= len(cells) {
				continue
			}
			content := cells[col]
			if rowIndex == 0 {
				body.WriteString(`{\b ` + content + `}`)
			} else {
				body.WriteString(content)
			}
		}
		body.WriteString(`\par `)
	}
}

func unescapeMarkdown(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			next := s[i+1]
			if (next >= 0x21 && next <= 0x2F) || (next >= 0x3A && next <= 0x40) ||
				(next >= 0x5B && next <= 0x60) || (next >= 0x7B && next <= 0x7E) {
				b.WriteByte(next)
				i++
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func renderInlines(body *strings.Builder, node ast.Node, src []byte) {
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		switch n := child.(type) {
		case *ast.Text:
			segment := unescapeMarkdown(string(n.Segment.Value(src)))
			body.WriteString(escapeString(segment))
			if n.SoftLineBreak() {
				body.WriteString(" ")
			} else if n.HardLineBreak() {
				body.WriteString(`\line `)
			}

		case *ast.String:
			body.WriteString(escapeString(unescapeMarkdown(string(n.Value))))

		case *ast.CodeSpan:
			var codeText strings.Builder
			for c := n.FirstChild(); c != nil; c = c.NextSibling() {
				if t, ok := c.(*ast.Text); ok {
					codeText.Write(t.Segment.Value(src))
				}
			}
			body.WriteString(`{\f1 ` + escapeString(codeText.String()) + `}`)

		case *ast.Emphasis:
			tag := `\i`
			if n.Level == 2 {
				tag = `\b`
			}
			body.WriteString(`{` + tag + ` `)
			renderInlines(body, n, src)
			body.WriteString(`}`)

		case *ast.Link:
			target := string(n.Destination)
			var labelBuilder strings.Builder
			renderInlines(&labelBuilder, n, src)
			label := labelBuilder.String()
			body.WriteString(linkRTF(label, target))

		case *ast.AutoLink:
			target := string(n.URL(src))
			body.WriteString(linkRTF(escapeString(target), target))

		default:
			renderInlines(body, child, src)
		}
	}
}

func linkRTF(visibleLabel string, target string) string {
	cleanLabel := strings.TrimSpace(stripRtfControl(visibleLabel))
	cleanTarget := strings.TrimSpace(escapeString(target))
	visible := visibleLabel
	if cleanLabel != "" && cleanLabel != target && cleanLabel != cleanTarget &&
		cleanLabel != strings.TrimPrefix(target, "https://") &&
		cleanLabel != strings.TrimPrefix(target, "http://") {
		visible += ` (` + cleanTarget + `)`
	} else {
		visible = escapeString(cleanLabel)
		if visible == "" {
			visible = cleanTarget
		}
	}
	return `{\field{\*\fldinst{HYPERLINK "` + escapeFieldTarget(target) + `"}}{\fldrslt{\ul\cf1 ` + visible + `}}}`
}

func escapeFieldTarget(target string) string {
	target = strings.ReplaceAll(target, `"`, "%22")
	return escapeString(target)
}

func stripRtfControl(rtf string) string {
	var b strings.Builder
	inControl := false
	for i := 0; i < len(rtf); i++ {
		c := rtf[i]
		if c == '\\' {
			inControl = true
			continue
		}
		if inControl {
			if c == ' ' || c == ';' || c == '}' || c == '{' {
				inControl = false
			}
			continue
		}
		if c == '{' || c == '}' {
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

func escapeString(text string) string {
	var out strings.Builder
	for index := 0; index < len(text); {
		character, size := decodeRune(text[index:])
		out.WriteString(escapeRune(character))
		index += size
	}
	return out.String()
}

func escapeRune(character rune) string {
	switch character {
	case '\\', '{', '}':
		return `\` + string(character)
	case '\t':
		return `\tab `
	case '\n':
		return `\line `
	}
	if character < 0x20 {
		return ""
	}
	if character < 0x7f {
		return string(character)
	}
	if character > 0xffff {
		high, low := surrogatePair(character)
		return escapeUnit(high) + escapeUnit(low)
	}
	return escapeUnit(character)
}

func escapeUnit(unit rune) string {
	value := int(unit)
	if value > 32767 {
		value -= 65536
	}
	return `\u` + strconv.Itoa(value) + `?`
}

func decodeRune(text string) (rune, int) {
	return utf8.DecodeRuneInString(text)
}

func surrogatePair(character rune) (rune, rune) {
	value := character - 0x10000
	return 0xD800 + (value >> 10), 0xDC00 + (value & 0x3FF)
}

func runeLen(text string) int {
	count := 0
	for range text {
		count++
	}
	return count
}
