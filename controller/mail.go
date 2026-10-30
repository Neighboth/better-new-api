package controller

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/mail"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
)

// The limits of a message of the administrator.
const (
	maxMailSubjectLength = 255
	maxMailContentLength = 200000
	maxMailRecipients    = 50000
)

// MailCampaignRequest is the message of the page of the users.
type MailCampaignRequest struct {
	Subject string `json:"subject"`
	Content string `json:"content"`
	// IsHtml tells whether the content of the message is HTML. A message that
	// is not HTML is sent as text: it is escaped and the reader sees the text,
	// the message of the installation is never a page of broken markup.
	IsHtml bool `json:"is_html"`
	// Promotional adds the headers of the unsubscribe (RFC 8058) and limits the
	// message to the users that accept the announcements of the installation.
	Promotional bool `json:"promotional"`
	// AllUsers sends the message to every user of the installation.
	AllUsers bool `json:"all_users"`
	// UserIds sends the message to the selected users.
	UserIds []int `json:"user_ids"`
	// Test sends the message to the address of the administrator only.
	Test bool `json:"test"`
	// TestEmail is the address of the test message. An empty value sends the
	// test to the address of the administrator.
	TestEmail string `json:"test_email"`
}

// GetMailAudience returns the number of the readers of a message.
//
// The page of the users shows the number next to the switch of the promotional
// message: the administrator sees how many users accept the announcements
// before the message is written.
func GetMailAudience(c *gin.Context) {
	audience, err := model.CountMailAudience()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var selected int
	if ids := parseIds(c.Query("ids")); len(ids) > 0 {
		counted, err := model.CountSelectedMailRecipients(ids, false)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		selected = counted
		promotional, err := model.CountSelectedMailRecipients(ids, true)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"addresses":          audience.Addresses,
				"marketing":          audience.Marketing,
				"selected":           selected,
				"selected_marketing": promotional,
			},
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"addresses": audience.Addresses,
			"marketing": audience.Marketing,
		},
	})
}

// SearchMailRecipients searches the readers for the selector of the page.
//
// The search runs on the server and the page loads the next page of the result
// only when the administrator scrolls: an installation of a hundred thousand
// users does not send its users to the browser.
func SearchMailRecipients(c *gin.Context) {
	var recipients []model.MailRecipient
	var err error
	if ids := parseIds(c.Query("ids")); len(ids) > 0 {
		// The page of the users sends the ids of the rows that the
		// administrator selected and shows the users with their names.
		recipients, err = model.MailRecipientsByIds(ids)
	} else {
		recipients, err = model.SearchMailRecipients(c.Query("keyword"), parseLimit(c.Query("limit")))
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]gin.H, 0, len(recipients))
	for _, recipient := range recipients {
		items = append(items, gin.H{
			"id":           recipient.Id,
			"username":     recipient.Username,
			"display_name": recipient.DisplayName,
			"email":        recipient.Email,
			"marketing":    recipient.MarketingEnabled,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"items": items,
			"count": len(items),
		},
	})
}

// SendMailCampaign writes a message to the users of the installation.
//
// The message is written into the store first and the sending runs in the
// background: the page of the administrator answers at once and it polls the
// progress of the campaign.
func SendMailCampaign(c *gin.Context) {
	var req MailCampaignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "The message is not valid.")
		return
	}

	subject := strings.TrimSpace(req.Subject)
	if subject == "" {
		common.ApiErrorMsg(c, "The subject of the message is empty.")
		return
	}
	if len(subject) > maxMailSubjectLength {
		common.ApiErrorMsg(c, "The subject of the message is too long.")
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		common.ApiErrorMsg(c, "The message is empty.")
		return
	}
	if len(req.Content) > maxMailContentLength {
		common.ApiErrorMsg(c, "The message is too long.")
		return
	}

	userId := c.GetInt("id")

	if req.Test {
		address := strings.TrimSpace(req.TestEmail)
		if address == "" {
			user, err := model.GetUserById(userId, true)
			if err != nil || strings.TrimSpace(user.Email) == "" {
				common.ApiErrorMsg(c, "Your account has no address: enter the address of the test message.")
				return
			}
			address = user.Email
		}
		if err := common.Validate.Var(address, "required,email"); err != nil {
			common.ApiErrorMsg(c, "The address of the test message is not valid.")
			return
		}
		if err := mail.SendTest(subject, req.Content, req.IsHtml, req.Promotional, address, userId); err != nil {
			common.ApiError(c, err)
			return
		}
		common.ApiSuccess(c, gin.H{"test": true, "emails": []string{address}})
		return
	}

	ids := normalizeIds(req.UserIds)
	if !req.AllUsers {
		if len(ids) == 0 {
			common.ApiErrorMsg(c, "Select at least one user.")
			return
		}
		if len(ids) > maxMailRecipients {
			common.ApiErrorMsg(c, "Too many users are selected.")
			return
		}
	}

	audience := "selected"
	userIds := ""
	if req.AllUsers {
		audience = "all"
	}
	userIds = mail.FormatUserIds(ids)

	total := 0
	if req.AllUsers {
		audienceCount, err := model.CountMailAudience()
		if err != nil {
			common.ApiError(c, err)
			return
		}
		total = int(audienceCount.Addresses)
		if req.Promotional {
			total = int(audienceCount.Marketing)
		}
	} else {
		count, err := model.CountSelectedMailRecipients(ids, req.Promotional)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		total = count
	}
	if total == 0 {
		common.ApiErrorMsg(c, "No user accepts this message.")
		return
	}

	campaign := &model.MailCampaign{
		Subject:       subject,
		Content:       req.Content,
		IsHtml:        req.IsHtml,
		IsPromotional: req.Promotional,
		Audience:      audience,
		UserIds:       userIds,
		Status:        model.MailCampaignQueued,
		Total:         total,
		CreatedBy:     userId,
	}
	if err := model.CreateMailCampaign(campaign); err != nil {
		common.ApiError(c, err)
		return
	}
	mail.SendCampaign(campaign.Id)
	common.ApiSuccess(c, gin.H{"campaign_id": campaign.Id, "total": total})
}

// GetMailCampaign reports the progress of a message.
func GetMailCampaign(c *gin.Context) {
	id := parseLimit(c.Param("id"))
	campaign, err := model.GetMailCampaign(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    campaign,
	})
}

// ListMailCampaigns returns the last messages of the installation.
func ListMailCampaigns(c *gin.Context) {
	campaigns, err := model.ListMailCampaigns(parseLimit(c.Query("limit")))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if campaigns == nil {
		campaigns = []model.MailCampaign{}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    campaigns,
	})
}

// GetMailDeliverability reports the DNS of the domain of the sender.
func GetMailDeliverability(c *gin.Context) {
	report := mail.CheckDeliverability()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    report,
	})
}

// GetMailStatus reports whether the installation can send a message.
func GetMailStatus(c *gin.Context) {
	configured := common.SMTPFrom != "" || common.SMTPAccount != ""
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"configured": configured,
			"server":     common.SMTPServer,
			"from":       common.SMTPFrom,
			"encrypted":  common.SMTPSSLEnabled || common.SMTPStartTLSEnabled,
			"sending":    mail.IsRunning(),
		},
	})
}

// Unsubscribe removes a reader from the announcements of the installation.
//
// The endpoint answers the one click button of the mailbox providers (RFC 8058)
// and the link of the body of the message. The token identifies the reader, the
// request carries no session: the provider sends the POST of the one click
// without a cookie and the reader that opens the link of the message is not
// asked for a password before the messages stop.
func Unsubscribe(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		token = c.PostForm("token")
	}
	if token == "" && c.Request.Method == http.MethodPost {
		// RFC 8058 sends `List-Unsubscribe=One-Click` as the body, the token is
		// in the address of the endpoint.
		_ = c.Request.ParseForm()
		token = c.Request.Form.Get("token")
	}
	userId, err := mail.ParseUnsubscribeToken(token)
	if err != nil {
		respondUnsubscribe(c, false, err)
		return
	}
	if err := model.SetUserMarketingEmail(userId, false); err != nil {
		respondUnsubscribe(c, false, errors.New("the settings of the reader could not be written"))
		return
	}
	respondUnsubscribe(c, true, nil)
}

// respondUnsubscribe answers the request of the reader.
//
// A browser that opens the link of the message sees a page, the provider that
// sends the one click POST reads the status of the answer.
func respondUnsubscribe(c *gin.Context, ok bool, err error) {
	if c.Request.Method == http.MethodPost {
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
			return
		}
		// RFC 8058 expects a 200 answer of the one click endpoint.
		c.JSON(http.StatusOK, gin.H{"success": true})
		return
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	status := http.StatusOK
	if !ok {
		status = http.StatusBadRequest
	}
	c.Status(status)
	target := strings.TrimRight(system_setting.ServerAddress, "/")
	if target == "" {
		target = "/"
	}
	_, _ = c.Writer.WriteString(unsubscribePage(ok, target))
}

// unsubscribePage is the page that a reader sees after the link.
func unsubscribePage(ok bool, target string) string {
	title := "Unsubscribed"
	message := "You will not receive the announcements of this site any more."
	if !ok {
		title = "The link is not valid"
		message = "The link of the message is not valid or it is too old. Write to the administrator of the site."
	}
	return `<!DOCTYPE html><html><head><meta charset="UTF-8">` +
		`<meta name="viewport" content="width=device-width,initial-scale=1">` +
		`<title>` + title + `</title></head>` +
		`<body style="font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0;background:#f6f7f9;color:#111827">` +
		`<div style="background:#fff;border-radius:12px;padding:32px;max-width:420px;text-align:center">` +
		`<h1 style="font-size:18px;margin:0 0 12px">` + title + `</h1>` +
		`<p style="margin:0;color:#4b5563;font-size:14px;line-height:1.6">` + message + `</p>` +
		`<p style="margin:16px 0 0"><a href="` + target + `" style="color:#2563eb;font-size:13px;text-decoration:none">` +
		common.SystemName + `</a></p>` +
		`</div></body></html>`
}

// parseIds reads a comma separated list of ids.
func parseIds(raw string) []int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var ids []int
	for _, part := range strings.Split(raw, ",") {
		value := 0
		for _, character := range strings.TrimSpace(part) {
			if character < '0' || character > '9' {
				value = 0
				break
			}
			value = value*10 + int(character-'0')
		}
		if value > 0 {
			ids = append(ids, value)
		}
	}
	return ids
}

// normalizeIds removes the duplicates and the empty ids of a selection.
func normalizeIds(ids []int) []int {
	seen := make(map[int]struct{}, len(ids))
	normalized := make([]int, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	return normalized
}

// parseLimit reads a positive number of a query.
func parseLimit(raw string) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || value < 0 || value > 1_000_000 {
		return 0
	}
	return value
}
