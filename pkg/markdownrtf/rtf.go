// Package markdownrtf turns the policies of the installation into the Rich Text
// Format of the page of the installation.
//
// The page of the policies of the installer of Windows is a RichEdit control:
// it shows RTF and it does not show Markdown. The administrator of the
// installation writes the policies in Markdown, the same text of the page of
// the first start of the client, therefore the text is converted here and the
// installer shows the result as it is.
//
// The renderer covers the subset of Markdown that a policy uses: headings,
// paragraphs, lists, quotes, code, tables, horizontal rules and the inline
// emphasis, code and links.
package markdownrtf

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// Table column widths are measured in twips, the unit of the format. The page
// of the policies of the installer is about 430 pixels wide, a value that is
// larger than the page is scaled down by the control.
const (
	tableWidthTwips = 6200
	minimumColumn   = 700
	twipsPerChar    = 110
	ruleCharacters  = 58
)

// Convert renders a Markdown document as a Rich Text Format document.
//
// The result is an ASCII document: every character outside of the ASCII range
// is written as an RTF escape, therefore the document keeps its text whatever
// the encoding of the transport is.
func Convert(markdown string) string {
	var body strings.Builder
	lines := splitLines(markdown)

	for index := 0; index < len(lines); {
		line := strings.TrimSpace(lines[index])
		if line == "" {
			body.WriteString(`\par `)
			index++
			continue
		}

		if _, fenced := fenceOf(line); fenced {
			index = writeCodeBlock(&body, lines, index)
			continue
		}

		if isTableLine(line) {
			index = writeTable(&body, lines, index)
			continue
		}

		if level := headingLevel(line); level > 0 {
			writeHeading(&body, line, level)
			index++
			continue
		}

		if isRule(line) {
			body.WriteString(`\pard\li0\fi0{\f1 ` + strings.Repeat(escapeRune('─'), ruleCharacters) + `}\par `)
			index++
			continue
		}

		if level, marker, content, ok := listItem(line); ok {
			writeListItem(&body, level, marker, content)
			index++
			continue
		}

		if quote, ok := quoteLine(line); ok {
			body.WriteString(`\pard\li360\ri360\i ` + inline(quote) + `\i0\par `)
			index++
			continue
		}

		index = writeParagraph(&body, lines, index)
	}

	return `{\rtf1\ansi\ansicpg1252\deff0\uc1` +
		`{\fonttbl{\f0\fnil\fcharset0 Segoe UI;}{\f1\fnil\fcharset0 Consolas;}}` +
		`{\colortbl;\red37\green99\blue235;\red90\green98\blue115;}` +
		`\f0\fs20 ` + body.String() + `}`
}

// splitLines splits the document into the lines of the renderer.
func splitLines(markdown string) []string {
	normalized := strings.ReplaceAll(markdown, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	return strings.Split(normalized, "\n")
}

// fenceOf reports the marker of a fence of a code block.
func fenceOf(line string) (string, bool) {
	for _, marker := range []string{"```", "~~~"} {
		if strings.HasPrefix(line, marker) {
			return marker, true
		}
	}
	return "", false
}

// writeCodeBlock writes the lines of a code block and returns the line after it.
func writeCodeBlock(body *strings.Builder, lines []string, index int) int {
	marker, _ := fenceOf(strings.TrimSpace(lines[index]))
	index++
	for index < len(lines) {
		line := strings.TrimSpace(lines[index])
		if strings.HasPrefix(line, marker) {
			index++
			break
		}
		body.WriteString(`\pard\li360\f1\fs18 ` + escapeString(lines[index]) + `\par `)
		index++
	}
	return index
}

// headingLevel reports how many number signs start a heading, zero when the
// line is not a heading.
func headingLevel(line string) int {
	level := 0
	for level < len(line) && line[level] == '#' {
		level++
	}
	if level == 0 || level > 6 || level >= len(line) || line[level] != ' ' {
		return 0
	}
	return level
}

// writeHeading writes a heading of the given level.
func writeHeading(body *strings.Builder, line string, level int) {
	text := strings.TrimSpace(line[level:])
	sizes := [6]int{26, 22, 20, 18, 18, 18}
	size := sizes[level-1]
	body.WriteString(`\pard\li0\fi0\b\fs` + strconv.Itoa(size) + ` ` + inline(text) + `\b0\fs20\par `)
}

// isRule reports whether a line is a horizontal rule.
func isRule(line string) bool {
	if len(line) < 3 {
		return false
	}
	character := line[0]
	if character != '-' && character != '*' && character != '_' {
		return false
	}
	count := 0
	for index := 0; index < len(line); index++ {
		switch line[index] {
		case character:
			count++
		case ' ', '\t':
		default:
			return false
		}
	}
	return count >= 3
}

// listItem reports the level, the marker and the content of a list item.
func listItem(line string) (int, string, string, bool) {
	level := 0
	for level*2+2 < len(line) && line[level*2] == ' ' && line[level*2+1] == ' ' {
		level++
	}
	rest := line[level*2:]
	for _, marker := range []string{"- ", "* ", "+ "} {
		if strings.HasPrefix(rest, marker) {
			return level, escapeRune('\u2022'), strings.TrimSpace(rest[len(marker):]), true
		}
	}
	digits := 0
	for digits < len(rest) && rest[digits] >= '0' && rest[digits] <= '9' {
		digits++
	}
	if digits > 0 && digits+1 < len(rest) && (rest[digits] == '.' || rest[digits] == ')') && rest[digits+1] == ' ' {
		return level, rest[:digits] + string(rest[digits]), strings.TrimSpace(rest[digits+2:]), true
	}
	return 0, "", "", false
}

// writeListItem writes an item of a list.
//
// The item of an unordered list carries a bullet, the item of an ordered list
// keeps the number that the author of the policy wrote.
func writeListItem(body *strings.Builder, level int, marker string, content string) {
	indent := 360 + level*360
	body.WriteString(`\pard\li` + strconv.Itoa(indent) + `\fi-180 ` + marker + `\tab ` + inline(content) + `\par `)
}

// quoteLine reports the content of a quoted line.
func quoteLine(line string) (string, bool) {
	if !strings.HasPrefix(line, ">") {
		return "", false
	}
	content := strings.TrimSpace(strings.TrimPrefix(line, ">"))
	return content, true
}

// writeParagraph writes a paragraph and returns the line after it.
//
// The lines of a paragraph are joined with a space, a line break inside of a
// paragraph of Markdown is a space in the text that the reader sees.
func writeParagraph(body *strings.Builder, lines []string, index int) int {
	parts := make([]string, 0, 2)
	for index < len(lines) {
		line := strings.TrimSpace(lines[index])
		if line == "" {
			break
		}
		if _, fenced := fenceOf(line); fenced {
			break
		}
		if isTableLine(line) || headingLevel(line) > 0 || isRule(line) {
			break
		}
		if _, _, _, ok := listItem(line); ok {
			break
		}
		if _, ok := quoteLine(line); ok {
			break
		}
		parts = append(parts, line)
		index++
	}
	body.WriteString(`\pard\li0\fi0 ` + inline(strings.Join(parts, " ")) + `\par `)
	return index
}

// isTableLine reports whether a line is a row of a table.
func isTableLine(line string) bool {
	return len(line) >= 3 && strings.HasPrefix(line, "|") && strings.HasSuffix(line, "|")
}

// isDivider reports whether a row of a table separates the head of the table
// from its body.
func isDivider(cells []string) bool {
	dashes := 0
	for _, cell := range cells {
		for _, character := range cell {
			switch character {
			case '-':
				dashes++
			case ':', ' ', '\t':
			default:
				return false
			}
		}
	}
	return dashes > 0
}

// splitRow splits a row of a table into the text of its cells.
func splitRow(line string) []string {
	trimmed := strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|")
	cells := strings.Split(trimmed, "|")
	for index, cell := range cells {
		cells[index] = strings.TrimSpace(cell)
	}
	return cells
}

// writeTable writes the rows of a table as aligned columns.
//
// The columns are the tab stops of the paragraphs of the rows, a RichEdit
// control of every version of Windows aligns the columns with them. The head of
// the table is written in bold, the separator of the head is not a row of the
// text.
func writeTable(body *strings.Builder, lines []string, index int) int {
	rows := make([][]string, 0, 8)
	divider := -1
	for index < len(lines) {
		line := strings.TrimSpace(lines[index])
		if !isTableLine(line) {
			break
		}
		cells := splitRow(line)
		if divider < 0 && isDivider(cells) {
			divider = len(rows)
			index++
			continue
		}
		rows = append(rows, cells)
		index++
	}
	if len(rows) == 0 {
		return index
	}

	columns := 0
	for _, cells := range rows {
		if len(cells) > columns {
			columns = len(cells)
		}
	}
	if columns == 0 {
		return index
	}

	widths := make([]int, columns)
	for column := range widths {
		widths[column] = 4
	}
	for _, cells := range rows {
		for column := 0; column < columns && column < len(cells); column++ {
			if length := runeLen(cells[column]); length > widths[column] {
				widths[column] = length
			}
		}
	}

	// The columns share the width of the page, a wide column takes the space
	// that its text needs and a narrow one keeps the space of its head.
	total := 0
	for column := range widths {
		widths[column] = widths[column]*twipsPerChar + minimumColumn
		total += widths[column]
	}
	stops := make([]int, columns)
	position := 0
	for column := range widths {
		width := widths[column]
		if total > tableWidthTwips {
			width = width * tableWidthTwips / total
		}
		position += width
		stops[column] = position
	}

	var tabs strings.Builder
	for _, stop := range stops {
		tabs.WriteString(`\tx` + strconv.Itoa(stop))
	}
	tabStops := tabs.String()

	for rowIndex, cells := range rows {
		body.WriteString(`\pard\li0\fi0` + tabStops + ` `)
		for column := 0; column < columns; column++ {
			if column > 0 {
				body.WriteString(`\tab `)
			}
			if column >= len(cells) {
				continue
			}
			content := inline(cells[column])
			if rowIndex < divider || (divider < 0 && rowIndex == 0) {
				body.WriteString(`{\b ` + content + `}`)
				continue
			}
			body.WriteString(content)
		}
		body.WriteString(`\par `)
	}
	return index
}

// inline renders the inline markup of a line of text.
func inline(text string) string {
	var out strings.Builder
	index := 0
	for index < len(text) {
		rest := text[index:]

		if strings.HasPrefix(rest, "**") || strings.HasPrefix(rest, "__") {
			marker := rest[:2]
			if end := strings.Index(rest[2:], marker); end > 0 {
				out.WriteString(`{\b ` + inline(rest[2:2+end]) + `}`)
				index += 2 + end + 2
				continue
			}
		}

		if rest[0] == '*' || rest[0] == '_' {
			if end := strings.IndexByte(rest[1:], rest[0]); end > 0 {
				out.WriteString(`{\i ` + inline(rest[1:1+end]) + `}`)
				index += 1 + end + 1
				continue
			}
		}

		if rest[0] == '`' {
			if end := strings.IndexByte(rest[1:], '`'); end >= 0 {
				out.WriteString(`{\f1 ` + escapeString(rest[1:1+end]) + `}`)
				index += 1 + end + 1
				continue
			}
		}

		if rest[0] == '[' {
			if label, target, length, ok := parseLink(rest); ok {
				out.WriteString(linkRTF(label, target))
				index += length
				continue
			}
		}

		if rest[0] == '<' {
			if end := strings.IndexByte(rest, '>'); end > 1 {
				target := rest[1:end]
				if isLinkTarget(target) {
					out.WriteString(linkRTF(target, target))
					index += end + 1
					continue
				}
			}
		}

		if rest[0] == '\\' && len(rest) > 1 {
			character, size := decodeRune(rest[1:])
			out.WriteString(escapeRune(character))
			index += 1 + size
			continue
		}

		character, size := decodeRune(rest)
		out.WriteString(escapeRune(character))
		index += size
	}
	return out.String()
}

// parseLink reads a link of the form `[label](target)` at the start of the
// text. It returns the label, the target and the number of bytes it read.
func parseLink(text string) (label string, target string, length int, ok bool) {
	if len(text) < 4 || text[0] != '[' {
		return "", "", 0, false
	}
	closing := strings.IndexByte(text, ']')
	if closing < 0 || closing+1 >= len(text) || text[closing+1] != '(' {
		return "", "", 0, false
	}
	end := strings.IndexByte(text[closing+2:], ')')
	if end < 0 {
		return "", "", 0, false
	}
	label = text[1:closing]
	target = text[closing+2 : closing+2+end]
	if target == "" {
		return "", "", 0, false
	}
	return label, target, closing + 2 + end + 1, true
}

// isLinkTarget reports whether the target of an autolink is a link.
func isLinkTarget(target string) bool {
	for _, prefix := range []string{"http://", "https://", "mailto:"} {
		if strings.HasPrefix(target, prefix) {
			return true
		}
	}
	return false
}

// linkRTF writes a link. The address follows the text, a policy that is printed
// keeps the address that the reader has to open.
func linkRTF(label string, target string) string {
	visible := inline(label)
	if label != target {
		visible += ` (` + escapeString(target) + `)`
	}
	return `{\field{\*\fldinst{HYPERLINK "` + escapeFieldTarget(target) + `"}}{\fldrslt{\ul\cf1 ` + visible + `}}}`
}

// escapeFieldTarget writes the target of a link as the argument of a field.
func escapeFieldTarget(target string) string {
	target = strings.ReplaceAll(target, `"`, "%22")
	return escapeString(target)
}

// escapeString writes a text without markup.
func escapeString(text string) string {
	var out strings.Builder
	for index := 0; index < len(text); {
		character, size := decodeRune(text[index:])
		out.WriteString(escapeRune(character))
		index += size
	}
	return out.String()
}

// escapeRune writes a single character of the format.
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

// escapeUnit writes a code unit of UTF-16 as an escape of the format.
//
// The value of an escape is a signed number, the code units of the second
// plane are negative numbers.
func escapeUnit(unit rune) string {
	value := int(unit)
	if value > 32767 {
		value -= 65536
	}
	return `\u` + strconv.Itoa(value) + `?`
}

// decodeRune reads the first character of the text.
func decodeRune(text string) (rune, int) {
	return utf8.DecodeRuneInString(text)
}

// surrogatePair returns the code units of a character outside of the first
// plane of UTF-16, the unit of the RichEdit control.
func surrogatePair(character rune) (rune, rune) {
	value := character - 0x10000
	return 0xD800 + (value >> 10), 0xDC00 + (value & 0x3FF)
}

// runeLen returns the number of characters of a text.
func runeLen(text string) int {
	count := 0
	for range text {
		count++
	}
	return count
}
