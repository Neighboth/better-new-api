package markdownrtf

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConvertWritesTheTableOfThePolicy checks that the cells of a table keep
// their order and that the row of the dashes of the head is not a row of the
// text that the reader sees.
func TestConvertWritesTheTableOfThePolicy(t *testing.T) {
	document := Convert("| Ad | Süre |\n|:---|---:|\n| İndirme | 2 saat |\n| Yükleme | 1 saat |\n")
	text := visibleText(document)

	assert.Contains(t, document, `\tx`, "the columns of the table are the tab stops of the rows")
	assert.Contains(t, document, `{\b `, "the head of the table is written in bold")
	assert.NotContains(t, text, "---", "the separator of the head is not a row of the text")

	rows := nonEmptyLines(text)
	require.Len(t, rows, 3)
	assert.Equal(t, "Ad\tSüre", rows[0])
	assert.Equal(t, "İndirme\t2 saat", rows[1])
	assert.Equal(t, "Yükleme\t1 saat", rows[2])
}

// TestConvertWritesAHorizontalRule checks that a rule of Markdown is a visible
// line and not an empty paragraph.
func TestConvertWritesAHorizontalRule(t *testing.T) {
	lines := nonEmptyLines(visibleText(Convert("Birinci\n\n---\n\nİkinci\n")))

	require.Len(t, lines, 3)
	assert.Equal(t, "Birinci", lines[0])
	assert.Equal(t, strings.Repeat("─", ruleCharacters), lines[1])
	assert.Equal(t, "İkinci", lines[2])
}

// TestConvertWritesLinks checks that a link keeps the text and the address and
// that the characters outside of the ASCII range are escapes of the format.
func TestConvertWritesLinks(t *testing.T) {
	document := Convert("Metni [KVKK metni](https://ornek.com/kvkk) okuyun.")

	assert.Contains(t, document, `HYPERLINK "https://ornek.com/kvkk"`)
	assert.Contains(t, document, `\ul\cf1 `)
	assert.Contains(t, visibleText(document), "KVKK metni (https://ornek.com/kvkk)")
}

// TestConvertWritesTheCharactersOfEveryLanguage checks the escapes of the
// characters that the document carries.
func TestConvertWritesTheCharactersOfEveryLanguage(t *testing.T) {
	document := Convert("Gizlilik — ş, ğ, ü, ç, ö, ı\n")

	assert.Contains(t, document, `\u351?`, "s of the Turkish alphabet")
	assert.Contains(t, document, `\u287?`, "g of the Turkish alphabet")
	assert.Contains(t, document, `\u8212?`, "the em dash")
	assert.Contains(t, visibleText(document), "Gizlilik — ş, ğ, ü, ç, ö, ı")
}

// TestConvertWritesTheInlineMarkup checks the emphasis and the code.
func TestConvertWritesTheInlineMarkup(t *testing.T) {
	markdown := "Bu **koyu**, bu *eğik*, bu " + "`kod`" + ", bu da {süslü} ve " + "\\\\" + "ters bölü.\n"
	document := Convert(markdown)

	assert.Contains(t, document, `{\b koyu}`)
	assert.Contains(t, document, `{\i e`)
	assert.Contains(t, document, `{\f1 kod}`)
	assert.Contains(t, document, `\{s`)
	assert.Contains(t, document, `\\ters`, "a backslash of the text is an escape of the format")
	assert.Contains(t, visibleText(document), `Bu koyu, bu eğik, bu kod, bu da {süslü} ve \ters bölü.`)
}

// TestConvertWritesHeadingsAndLists checks the blocks of a policy.
func TestConvertWritesHeadingsAndLists(t *testing.T) {
	document := Convert("# Gizlilik\n\n## Toplanan veriler\n\n- İşlem kaydı\n- Cihaz bilgisi\n\n1. Birinci\n2. İkinci\n")
	lines := nonEmptyLines(visibleText(document))

	assert.Contains(t, document, `\b\fs26 `)
	assert.Contains(t, document, `\b\fs22 `)

	require.Len(t, lines, 6)
	assert.Equal(t, "Gizlilik", lines[0])
	assert.Equal(t, "Toplanan veriler", lines[1])
	assert.Equal(t, "•\tİşlem kaydı", lines[2])
	assert.Equal(t, "•\tCihaz bilgisi", lines[3])
	assert.Equal(t, "1.\tBirinci", lines[4])
	assert.Equal(t, "2.\tİkinci", lines[5])
}

// TestConvertKeepsTheTextOfAPlainPolicy checks that a policy that is written
// without markup is not changed by the renderer.
func TestConvertKeepsTheTextOfAPlainPolicy(t *testing.T) {
	assert.Contains(t, visibleText(Convert("Gizlilik politikası\nİkinci satır\n")), "Gizlilik politikası İkinci satır")
}

// TestConvertWritesAValidDocument checks the structure of the document: the
// groups are balanced and every character outside of the ASCII range is an
// escape, whatever the encoding of the transport is.
func TestConvertWritesAValidDocument(t *testing.T) {
	document := Convert("# Başlık 🙂\n\n| A | B |\n|---|---|\n| 🙂 | ş |\n\n- ğ\n\n---\n")

	depth := 0
	for _, character := range document {
		switch character {
		case '{':
			depth++
		case '}':
			depth--
			require.GreaterOrEqual(t, depth, 0, "the document closes a group that it did not open")
		}
	}
	require.Zero(t, depth, "the document leaves a group open")
	assert.True(t, isASCII(document), "the document is written with ASCII characters only")
	assert.Contains(t, document, `\u-10179?\u-8638?`, "a character outside of the first plane is a surrogate pair")
}

// visibleText reads the text that the RichEdit control shows.
//
// The reader walks the document, it skips the destinations of the format (the
// font table, the colour table and the parts that start with an asterisk, the
// instruction of a field among them) and it decodes the escapes of a character.
func visibleText(document string) string {
	var out strings.Builder
	stack := make([]bool, 0, 8)
	skipping := false

	push := func(skip bool) {
		stack = append(stack, skipping || skip)
		skipping = stack[len(stack)-1]
	}

	for index := 0; index < len(document); index++ {
		switch character := document[index]; character {
		case '{':
			destination := index+2 < len(document) && document[index+1] == '\\' && document[index+2] == '*'
			for _, table := range []string{`\fonttbl`, `\colortbl`} {
				if strings.HasPrefix(document[index+1:], table) {
					destination = true
				}
			}
			push(destination)
		case '}':
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
			skipping = len(stack) > 0 && stack[len(stack)-1]
		case '\\':
			if index+1 >= len(document) {
				continue
			}
			if next := document[index+1]; next == '\\' || next == '{' || next == '}' {
				if !skipping {
					out.WriteByte(next)
				}
				index++
				continue
			}
			end := index + 1
			for end < len(document) && isControlCharacter(document[end]) {
				end++
			}
			word := document[index+1 : end]
			parameterStart := end
			if end < len(document) && document[end] == '-' {
				end++
			}
			for end < len(document) && document[end] >= '0' && document[end] <= '9' {
				end++
			}
			parameterEnd := end
			if end < len(document) && document[end] == ' ' {
				end++
			}
			if !skipping {
				switch word {
				case "par", "row", "line":
					out.WriteString("\n")
				case "tab", "cell":
					out.WriteString("\t")
				case "u":
					value, _ := strconv.Atoi(document[parameterStart:parameterEnd])
					if value < 0 {
						value += 65536
					}
					out.WriteRune(rune(value))
					end++
				}
			}
			index = end - 1
		default:
			if !skipping {
				out.WriteByte(character)
			}
		}
	}
	return out.String()
}

// isControlCharacter reports whether a character belongs to the name of a
// control word of the format.
func isControlCharacter(character byte) bool {
	return character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z'
}

// nonEmptyLines returns the lines of a text, the empty ones are dropped.
func nonEmptyLines(text string) []string {
	lines := make([]string, 0, 8)
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, " \t")
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// isASCII reports whether a text has characters of the ASCII range only.
func isASCII(text string) bool {
	for _, character := range text {
		if character > 0x7f {
			return false
		}
	}
	return true
}
