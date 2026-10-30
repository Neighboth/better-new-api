// Package mail sends the messages of the administrator of the installation.
//
// The package owns three things that the plain SMTP client of the
// `common` package does not: the campaign that writes many messages with one
// connection, the throttling that keeps the reputation of the sending domain,
// and the token of the one click unsubscribe of RFC 8058.
package mail

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

// DefaultThrottle is the pause between two messages of a campaign.
//
// A provider counts the messages of a connection and the messages of a domain
// per second: a burst of a thousand messages of a new domain is the fastest way
// into the spam folder. Ten messages per second is slow enough for the filters
// and fast enough for a list of a few thousand users.
const DefaultThrottle = 100 * time.Millisecond

// campaignMu serializes the campaigns of one installation.
//
// The SMTP server of a shared host allows one session at a time, two campaigns
// of two administrators would compete for the connections and the server would
// throttle both.
var campaignMu sync.Mutex

// running reports whether a campaign is running now.
var running = struct {
	sync.Mutex
	value bool
}{}

// IsRunning reports whether a campaign is being sent now.
func IsRunning() bool {
	running.Lock()
	defer running.Unlock()
	return running.value
}

func setRunning(value bool) {
	running.Lock()
	running.value = value
	running.Unlock()
}

// UnsubscribeToken returns the token of the one click unsubscribe of a user.
//
// The token is an HMAC of the user and of the settings of the installation: the
// address of the reader never travels in the URL, a forwarded message does not
// publish the address of its reader, and the token cannot be computed by a
// third party. The token of a replaced CRYPTO_SECRET stops working, the reader
// uses the link of the next message then.
func UnsubscribeToken(userId int) string {
	raw := fmt.Sprintf("mail-unsubscribe:%d", userId)
	mac := hmac.New(sha256.New, []byte(common.CryptoSecret))
	mac.Write([]byte(raw))
	signature := mac.Sum(nil)[:18]
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(userId))) +
		"." + base64.RawURLEncoding.EncodeToString(signature)
}

// ParseUnsubscribeToken returns the user of an unsubscribe token.
func ParseUnsubscribeToken(token string) (int, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return 0, errors.New("the link of the unsubscribe is not valid")
	}
	rawId, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return 0, errors.New("the link of the unsubscribe is not valid")
	}
	userId, err := strconv.Atoi(string(rawId))
	if err != nil || userId <= 0 {
		return 0, errors.New("the link of the unsubscribe is not valid")
	}
	expected := UnsubscribeToken(userId)
	if !hmac.Equal([]byte(expected), []byte(token)) {
		return 0, errors.New("the link of the unsubscribe is not valid")
	}
	return userId, nil
}

// UnsubscribeURL returns the address of the one click unsubscribe of a user.
//
// The address has to be HTTPS: the mailbox providers ignore an insecure address
// of the header and the one click button of the reader stops working.
func UnsubscribeURL(userId int) string {
	base := strings.TrimRight(system_setting.ServerAddress, "/")
	return fmt.Sprintf("%s/api/user/unsubscribe?token=%s", base, UnsubscribeToken(userId))
}

// UnsubscribePageURL returns the address of the page that a reader opens from
// the body of the message.
func UnsubscribePageURL(userId int) string {
	base := strings.TrimRight(system_setting.ServerAddress, "/")
	return fmt.Sprintf("%s/api/user/unsubscribe?token=%s&page=1", base, UnsubscribeToken(userId))
}

// UnsubscribeFooter is the footer of a promotional message.
//
// The body of a bulk message has to carry the link of the unsubscribe as well
// as the header: a reader who does not see the button of the provider looks for
// the link, a reader who does not find the link reports the message as spam.
func UnsubscribeFooter(userId int, language string) string {
	texts := footerTexts(language)
	return fmt.Sprintf(`<hr style="border:none;border-top:1px solid #e5e7eb;margin:24px 0 12px">
<p style="color:#6b7280;font-size:12px;line-height:1.6;margin:0">
%s<br>
<a href="%s" style="color:#6b7280">%s</a>
</p>`, texts.reason, UnsubscribePageURL(userId), texts.link)
}

type footerStrings struct {
	reason string
	link   string
}

func footerTexts(language string) footerStrings {
	switch strings.ToLower(strings.Split(language, "-")[0]) {
	case "tr":
		return footerStrings{
			reason: fmt.Sprintf("Bu ileti %s tarafından gönderilen duyuru ve kampanya postasıdır.", common.SystemName),
			link:   "Bu e-postaları almak istemiyorsanız buradan çıkabilirsiniz.",
		}
	case "zh", "zh_cn", "zh_tw":
		return footerStrings{
			reason: fmt.Sprintf("此邮件来自 %s 的公告与推广信息。", common.SystemName),
			link:   "如果您不想再收到此类邮件，请点击此处退订。",
		}
	default:
		return footerStrings{
			reason: fmt.Sprintf("This message is an announcement from %s.", common.SystemName),
			link:   "You can stop these emails at any time.",
		}
	}
}

// PlainBody writes the text of the message.
//
// The text part of a message is not a copy of the HTML that a filter can see
// through: the administrator writes the text that a reader without an HTML
// viewer reads, the HTML is generated from it.
func PlainBody(content string) string {
	escaped := html.EscapeString(content)
	paragraphs := strings.Split(strings.ReplaceAll(escaped, "\r\n", "\n"), "\n\n")
	var builder strings.Builder
	for _, paragraph := range paragraphs {
		trimmed := strings.TrimSpace(paragraph)
		if trimmed == "" {
			continue
		}
		builder.WriteString("<p style=\"margin:0 0 14px;line-height:1.65\">")
		builder.WriteString(strings.ReplaceAll(trimmed, "\n", "<br>"))
		builder.WriteString("</p>\n")
	}
	return builder.String()
}

// WrapHTML writes the document of the message.
//
// Every message of the installation is written with the same frame: a reader
// who recognizes the frame of the messages of the installation trusts the next
// message, an unstyled page of raw HTML is a page that the filters of the
// providers score as a message of a sender that does not care.
func WrapHTML(body string, language string, promotional bool, userId int) string {
	direction := "ltr"
	if strings.HasPrefix(strings.ToLower(language), "ar") || strings.HasPrefix(strings.ToLower(language), "he") {
		direction = "rtl"
	}
	footer := ""
	if promotional {
		footer = UnsubscribeFooter(userId, language)
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html dir="%s">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>%s</title>
</head>
<body style="margin:0;padding:0;background:#f6f7f9">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background:#f6f7f9;padding:24px 12px">
<tr><td align="center">
<table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="max-width:600px;background:#ffffff;border-radius:12px;padding:28px;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;font-size:15px;color:#111827">
<tr><td>
<div style="font-weight:700;font-size:17px;margin:0 0 18px;color:#111827">%s</div>
%s
%s
</td></tr>
</table>
</td></tr>
</table>
</body>
</html>`, direction, html.EscapeString(common.SystemName), html.EscapeString(common.SystemName), body, footer)
}

// BuildMessage writes the message of one reader.
func BuildMessage(subject string, body string, isHtml bool, promotional bool, recipient model.MailRecipient) common.MailMessage {
	htmlBody := body
	textBody := ""
	if isHtml {
		htmlBody = WrapHTML(body, recipient.Language, promotional, recipient.Id)
	} else {
		textBody = body
		htmlBody = WrapHTML(PlainBody(body), recipient.Language, promotional, recipient.Id)
	}

	message := common.MailMessage{
		To:      recipient.Email,
		ToName:  recipientDisplayName(recipient),
		Subject: subject,
		HTML:    htmlBody,
		Text:    textBody,
		Bulk:    promotional,
	}
	if promotional {
		message.UnsubscribeURL = UnsubscribeURL(recipient.Id)
	}
	return message
}

func recipientDisplayName(recipient model.MailRecipient) string {
	name := strings.TrimSpace(recipient.DisplayName)
	if name == "" {
		name = strings.TrimSpace(recipient.Username)
	}
	return name
}

// SendCampaign writes a campaign to its readers.
//
// The sending runs in the background: the page of the administrator answers
// as soon as the campaign is written into the store and the page polls the
// progress. A campaign never blocks a request of the panel.
func SendCampaign(campaignId int) {
	go runCampaign(campaignId)
}

func runCampaign(campaignId int) {
	campaign, err := model.GetMailCampaign(campaignId)
	if err != nil {
		common.SysError(fmt.Sprintf("mail campaign %d: %s", campaignId, err.Error()))
		return
	}
	if campaign.Status != model.MailCampaignQueued {
		return
	}

	campaignMu.Lock()
	defer campaignMu.Unlock()
	setRunning(true)
	defer setRunning(false)

	startedAt := time.Now().Unix()
	_ = model.UpdateMailCampaign(campaign.Id, map[string]any{
		"status":     model.MailCampaignActive,
		"started_at": startedAt,
	})

	ids := parseUserIds(campaign.UserIds)
	sender, senderErr := common.NewMailSender()
	throttle := DefaultThrottle

	sent := 0
	failed := 0
	skipped := 0
	failures := make([]string, 0, 10)
	lastError := ""

	flush := func() {
		fields := map[string]any{
			"sent":       sent,
			"failed":     failed,
			"skipped":    skipped,
			"last_error": truncate(lastError, 500),
			"failures":   strings.Join(failures, "\n"),
		}
		_ = model.UpdateMailCampaign(campaign.Id, fields)
	}

	var iterErr error
	_, iterErr = model.IterateMailRecipients(ids, campaign.IsPromotional, 200, func(batch []model.MailRecipient) error {
		for index, recipient := range batch {
			if senderErr != nil || sender == nil || sender.Exhausted() {
				if sender != nil {
					sender.Close()
				}
				sender, senderErr = common.NewMailSender()
				if senderErr != nil {
					// The connection of the server is lost: the messages of the
					// batch that did not leave the installation are reported as
					// failed, the campaign stops and the administrator sees why.
					failed += len(batch) - index
					lastError = "the connection to the mail server failed: " + senderErr.Error()
					flush()
					return errors.New(lastError)
				}
			}

			message := BuildMessage(campaign.Subject, campaign.Content, campaign.IsHtml, campaign.IsPromotional, recipient)
			if err := sender.Send(message); err != nil {
				failed++
				lastError = err.Error()
				if len(failures) < 10 {
					failures = append(failures, recipient.Email+": "+truncate(err.Error(), 160))
				}
				// A refused session is retried once with a new connection: the
				// provider closes an old session and the next message of the
				// campaign has to leave the installation.
				sender.Close()
				sender = nil
				if !isPermanentError(err) {
					if retrySender, retryErr := common.NewMailSender(); retryErr == nil {
						if retryErr := retrySender.Send(message); retryErr == nil {
							sent++
							failed--
							lastError = ""
						}
						retrySender.Close()
					}
				}
			} else {
				sent++
			}
			flush()
			time.Sleep(throttle)
		}
		return nil
	})
	if sender != nil {
		sender.Close()
	}

	endedAt := time.Now().Unix()
	status := model.MailCampaignDone
	if iterErr != nil {
		status = model.MailCampaignFailed
		lastError = iterErr.Error()
	} else if sent == 0 && failed > 0 {
		status = model.MailCampaignFailed
	}
	flush()
	_ = model.UpdateMailCampaign(campaign.Id, map[string]any{
		"status":   status,
		"ended_at": endedAt,
	})
}

// isPermanentError reports whether the server refused the message for a reason
// that the next attempt of the same address cannot change.
func isPermanentError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	for _, permanent := range []string{
		"550", "551", "553", "554",
		"mailbox unavailable", "no such user", "recipient address rejected",
		"invalid recipient", "user unknown",
	} {
		if strings.Contains(text, permanent) {
			return true
		}
	}
	return false
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

// parseUserIds reads the ids of the audience of a campaign.
func parseUserIds(raw string) []int {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, ",")
	ids := make([]int, 0, len(parts))
	for _, part := range parts {
		value, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || value <= 0 {
			continue
		}
		ids = append(ids, value)
	}
	return ids
}

// FormatUserIds writes the ids of the audience of a campaign.
func FormatUserIds(ids []int) string {
	unique := make([]string, 0, len(ids))
	seen := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, strconv.Itoa(id))
	}
	return strings.Join(unique, ",")
}

// SendTest writes one message to the address of the administrator.
//
// The test message is the message of the campaign: the administrator sees the
// frame, the footer and the link of the unsubscribe before the campaign reaches
// a reader.
func SendTest(subject string, content string, isHtml bool, promotional bool, email string, userId int) error {
	recipient := model.MailRecipient{
		Id:          userId,
		Username:    "test",
		DisplayName: common.SystemName,
		Email:       email,
		Language:    "",
	}
	message := BuildMessage(subject, content, isHtml, promotional, recipient)
	return common.SendMailMessage(message)
}
