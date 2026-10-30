package common

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

// MailMessage is one message that is written to the SMTP server.
//
// The message is built here and not by a template of the callers: the headers
// that keep a message out of the spam folder (a plain text alternative, a
// Message-ID of the domain of the sender, the headers of a bulk message) are
// the same for every message of the installation.
type MailMessage struct {
	// To is the address of the reader.
	To string
	// ToName is the display name of the reader, it is optional.
	ToName string
	// Subject is the subject as the reader sees it, the encoding of the
	// transport is written here.
	Subject string
	// HTML is the body of the message, it is always present.
	HTML string
	// Text is the plain text alternative of the body. An empty value is
	// derived from the HTML: a message without a text part is a message that
	// the filters of the mail providers score as bulk mail.
	Text string
	// Headers are the additional headers of the message, for example
	// `List-Unsubscribe`.
	Headers map[string]string
	// ReplyTo is the address that answers the message.
	ReplyTo string
	// UnsubscribeURL is the address that removes the reader from the list. A
	// promotional message carries the headers of RFC 8058 next to the link.
	UnsubscribeURL string
	// Bulk marks a message that is sent to a list. The providers treat a
	// message that says that it is bulk better than a message that looks like
	// a personal one.
	Bulk bool
}

// BuildMailMessage writes the message with its headers.
//
// The body is written as a multipart message with a plain text part and an HTML
// part, both encoded with base64. The encoding keeps the lines of the message
// short and every character of the text safe, whatever the encoding of the
// transport is.
func BuildMailMessage(message MailMessage) ([]byte, error) {
	if strings.TrimSpace(message.To) == "" {
		return nil, fmt.Errorf("the receiver of the message is missing")
	}
	from := SMTPFrom
	if from == "" {
		from = SMTPAccount
	}
	if from == "" {
		return nil, fmt.Errorf("SMTP 服务器未配置")
	}

	messageId, err := generateMessageID()
	if err != nil {
		return nil, err
	}

	boundary := fmt.Sprintf("byog-%d-%s", time.Now().UnixNano(), GetRandomString(16))
	text := strings.TrimSpace(message.Text)
	if text == "" {
		text = htmlToText(message.HTML)
	}
	if strings.TrimSpace(text) == "" {
		text = " "
	}

	var builder strings.Builder
	writeHeader(&builder, "From", addressHeader(SystemName, from))
	writeHeader(&builder, "To", addressHeader(message.ToName, message.To))
	if message.ReplyTo != "" {
		writeHeader(&builder, "Reply-To", addressHeader("", message.ReplyTo))
	}
	// The subject is always encoded: the text of a subject is written by the
	// administrator of the installation and the readers of every language has
	// to see it, whatever the encoding of the transport is.
	writeHeader(&builder, "Subject", encodeHeaderText(message.Subject))
	writeHeader(&builder, "Date", time.Now().Format(time.RFC1123Z))
	writeHeader(&builder, "Message-ID", messageId)
	writeHeader(&builder, "MIME-Version", "1.0")
	// A message that answers itself is not expected: the header keeps the out
	// of office answers and the vacation messages of the readers away.
	writeHeader(&builder, "Auto-Submitted", "auto-generated")
	writeHeader(&builder, "X-Auto-Response-Suppress", "OOF, AutoReply")
	if message.Bulk {
		writeHeader(&builder, "Precedence", "bulk")
	}
	if message.UnsubscribeURL != "" {
		writeHeader(&builder, "List-Unsubscribe", fmt.Sprintf("<%s>", message.UnsubscribeURL))
		// RFC 8058: the mailbox provider removes the reader with a POST and
		// the reader does not have to open the message. The providers require
		// the pair of the headers for a message that is sent to a list.
		writeHeader(&builder, "List-Unsubscribe-Post", "List-Unsubscribe=One-Click")
	}
	for name, value := range message.Headers {
		writeHeader(&builder, name, value)
	}
	writeHeader(&builder, "Content-Type", fmt.Sprintf("multipart/alternative; boundary=\"%s\"", boundary))

	builder.WriteString("\r\n")
	builder.WriteString("--" + boundary + "\r\n")
	builder.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	builder.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
	builder.WriteString(encodeBase64Body(text))
	builder.WriteString("\r\n--" + boundary + "\r\n")
	builder.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	builder.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
	builder.WriteString(encodeBase64Body(message.HTML))
	builder.WriteString("\r\n--" + boundary + "--\r\n")

	return []byte(builder.String()), nil
}

// writeHeader writes one header of the message.
func writeHeader(builder *strings.Builder, name string, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	// A header never carries a line break, an injected break would let the
	// text of the message write headers of its own.
	value = strings.NewReplacer("\r", " ", "\n", " ").Replace(value)
	builder.WriteString(name + ": " + value + "\r\n")
}

// encodeHeaderText encodes a text of a header with the encoding of the format.
func encodeHeaderText(text string) string {
	return "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(text)) + "?="
}

// addressHeader writes a name and an address of the format.
func addressHeader(name string, address string) string {
	address = strings.TrimSpace(address)
	if strings.TrimSpace(name) == "" {
		return "<" + address + ">"
	}
	return encodeHeaderText(name) + " <" + address + ">"
}

// encodeBase64Body encodes the body of a part with the line length of the
// format.
func encodeBase64Body(body string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(body))
	lines := make([]string, 0, len(encoded)/76+1)
	for len(encoded) > 76 {
		lines = append(lines, encoded[:76])
		encoded = encoded[76:]
	}
	lines = append(lines, encoded)
	return strings.Join(lines, "\r\n")
}

// htmlToText writes the plain text of a message from its HTML.
//
// The text part is the copy that a reader without an HTML viewer sees and the
// copy that the filters read: a message whose two parts say the same thing is a
// message of a sender that writes both parts.
func htmlToText(document string) string {
	replaced := document
	for _, tag := range []string{"</p>", "<br>", "<br/>", "<br />", "</div>", "</tr>", "</h1>", "</h2>", "</h3>", "</li>"} {
		replaced = strings.ReplaceAll(replaced, tag, "\n")
	}
	replaced = strings.ReplaceAll(replaced, "<li>", "• ")

	var text strings.Builder
	depth := 0
	for _, character := range replaced {
		switch character {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 {
				text.WriteRune(character)
			}
		}
	}

	lines := strings.Split(text.String(), "\n")
	cleaned := make([]string, 0, len(lines))
	previousBlank := false
	for _, line := range lines {
		line = strings.TrimSpace(strings.ReplaceAll(line, "&nbsp;", " "))
		if line == "" {
			if previousBlank {
				continue
			}
			previousBlank = true
			cleaned = append(cleaned, "")
			continue
		}
		previousBlank = false
		cleaned = append(cleaned, line)
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}
