package mail

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/require"
)

func init() {
	common.CryptoSecret = "test-secret-of-the-mail-package"
	common.SystemName = "New API"
}

func TestUnsubscribeTokenRoundTrip(t *testing.T) {
	token := UnsubscribeToken(42)
	require.NotEmpty(t, token)
	// The address of the reader never travels in the link.
	require.NotContains(t, token, "@")
	require.NotContains(t, token, "42.")

	userId, err := ParseUnsubscribeToken(token)
	require.NoError(t, err)
	require.Equal(t, 42, userId)
}

func TestUnsubscribeTokenRejectsForeignToken(t *testing.T) {
	token := UnsubscribeToken(42)
	other := UnsubscribeToken(43)

	_, err := ParseUnsubscribeToken(other)
	require.NoError(t, err)

	// A token whose payload is changed must not be accepted.
	parts := strings.Split(token, ".")
	require.Len(t, parts, 2)
	_, err = ParseUnsubscribeToken(other[:len(other)-2] + parts[1])
	require.Error(t, err)

	_, err = ParseUnsubscribeToken("")
	require.Error(t, err)

	_, err = ParseUnsubscribeToken("garbage")
	require.Error(t, err)
}

func TestBuildMessageCarriesTheHeadersOfTheUnsubscribe(t *testing.T) {
	common.SMTPFrom = "sender@example.com"
	common.SMTPAccount = "sender@example.com"
	recipient := model.MailRecipient{
		Id:    7,
		Email: "reader@example.com",
	}

	message := BuildMessage("Hello", "The body", false, true, recipient)
	require.Equal(t, "reader@example.com", message.To)
	require.Contains(t, message.UnsubscribeURL, "token=")

	raw, err := common.BuildMailMessage(message)
	require.NoError(t, err)
	body := string(raw)

	require.Contains(t, body, "List-Unsubscribe: <http")
	require.Contains(t, body, "List-Unsubscribe-Post: List-Unsubscribe=One-Click")
	require.Contains(t, body, "Precedence: bulk")
	require.Contains(t, body, "multipart/alternative")
	require.Contains(t, body, "Message-ID: <")
}

func TestPromotionalMessageHasNoUnsubscribeHeader(t *testing.T) {
	common.SMTPFrom = "sender@example.com"
	common.SMTPAccount = "sender@example.com"
	recipient := model.MailRecipient{Id: 9, Email: "reader@example.com"}

	raw, err := common.BuildMailMessage(BuildMessage("News", "The body", true, false, recipient))
	require.NoError(t, err)
	require.NotContains(t, string(raw), "List-Unsubscribe")
}

func TestPlainBodyEscapesMarkup(t *testing.T) {
	body := PlainBody("<script>alert(1)</script>\n\nFirst line\nSecond line")
	require.NotContains(t, body, "<script>")
	require.Contains(t, body, "&lt;script&gt;")
	require.Contains(t, body, "<p style=")
	// A line break of the text becomes the break of the paragraph.
	require.Contains(t, body, "<br>")
}

func TestHTMLMessageKeepsTheMarkupOfTheAdministrator(t *testing.T) {
	common.SMTPFrom = "sender@example.com"
	recipient := model.MailRecipient{Id: 3, Email: "reader@example.com"}
	message := BuildMessage("Subject", "<b>bold</b>", true, false, recipient)
	require.Contains(t, message.HTML, "<b>bold</b>")
	require.Empty(t, message.Text, "the text part of an HTML message is derived")
}

func TestIsPermanentError(t *testing.T) {
	require.True(t, isPermanentError(assertError("550 5.1.1 user unknown")))
	require.True(t, isPermanentError(assertError("553 5.7.1 recipient address rejected")))
	require.False(t, isPermanentError(assertError("451 4.7.1 greylisted, try again later")))
	require.False(t, isPermanentError(nil))
}

func TestParseUserIdsAndFormat(t *testing.T) {
	require.Empty(t, parseUserIds(""))
	require.Equal(t, []int{3, 8}, parseUserIds("3, 8"))
	require.Equal(t, []int{3}, parseUserIds("3, 0, x"))
	require.Equal(t, "3,8", FormatUserIds([]int{3, 8, 3, 0}))
}

type stringError string

func (e stringError) Error() string { return string(e) }

func assertError(text string) error { return stringError(text) }

func TestFooterTextsFollowTheLanguage(t *testing.T) {
	require.Contains(t, footerTexts("tr").link, "çıkabilirsiniz")
	require.Contains(t, footerTexts("en").link, "stop these emails")
	require.Contains(t, footerTexts("").link, "stop these emails")
}
