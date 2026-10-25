package discordbot

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/bwmarrin/discordgo"
)

// Owner panel actions open a modal for the required input and then run the same
// model-layer operations the web dashboard uses. Every action re-checks the
// session permission, so a stale or forged component id cannot escalate.
const ownerModalPrefix = ownerPanelPrefix + "modal:"

func (b *Bot) handleOwnerUserAction(s *discordgo.Session, i *discordgo.InteractionCreate, session *ownerSession, parts []string) {
	if len(parts) < 2 {
		_ = respondPanel(s, i, textDisplay("Geçersiz işlem parametresi."))
		return
	}
	switch parts[1] {
	case "create":
		if !session.can(authz.UserWrite) {
			b.denyOwnerAction(s, i)
			return
		}
		safeOpenModal(s, i, ownerModalPrefix+"user_create", "Kullanıcı oluştur",
			textInput("username", "Kullanıcı adı", "ornek_kullanici", true, discordgo.TextInputShort),
			textInput("display_name", "Görünen ad", "Örnek Kullanıcı", false, discordgo.TextInputShort),
			textInput("password", "Şifre", "en az 8 karakter", true, discordgo.TextInputShort),
			textInput("quota", "Başlangıç bakiyesi (quota, boş = varsayılan)", "100000", false, discordgo.TextInputShort),
		)
	case "quota":
		if !session.can(authz.UserWrite) {
			b.denyOwnerAction(s, i)
			return
		}
		safeOpenModal(s, i, ownerModalPrefix+"user_quota", "Bakiye güncelle",
			textInput("target", "Kullanıcı (id veya kullanıcı adı)", "42", true, discordgo.TextInputShort),
			textInput("delta", "Değişim (negatif düşürür)", "-5000", true, discordgo.TextInputShort),
		)
	case "status":
		if !session.can(authz.UserWrite) {
			b.denyOwnerAction(s, i)
			return
		}
		safeOpenModal(s, i, ownerModalPrefix+"user_status", "Durum değiştir",
			textInput("target", "Kullanıcı (id veya kullanıcı adı)", "42", true, discordgo.TextInputShort),
			textInput("status", "Durum (aktif / pasif)", "aktif", true, discordgo.TextInputShort),
		)
	case "delete":
		if !session.can(authz.UserDelete) {
			b.denyOwnerAction(s, i)
			return
		}
		safeOpenModal(s, i, ownerModalPrefix+"user_delete", "Kullanıcı sil",
			textInput("target", "Kullanıcı (id veya kullanıcı adı)", "42", true, discordgo.TextInputShort),
			textInput("confirm", "Onay için SİL yazın", "SİL", true, discordgo.TextInputShort),
		)
	default:
		_ = respondPanel(s, i, textDisplay("Bilinmeyen kullanıcı işlemi."))
	}
}

func (b *Bot) handleOwnerBlogAction(s *discordgo.Session, i *discordgo.InteractionCreate, session *ownerSession, parts []string) {
	if !session.can(authz.BlogWrite) {
		b.denyOwnerAction(s, i)
		return
	}
	if len(parts) < 2 {
		_ = respondPanel(s, i, textDisplay("Geçersiz işlem parametresi."))
		return
	}
	switch parts[1] {
	case "create":
		safeOpenModal(s, i, ownerModalPrefix+"blog_create", "Blog yazısı ekle",
			textInput("title", "Başlık", "Duyuru", true, discordgo.TextInputShort),
			textInput("summary", "Özet", "", false, discordgo.TextInputShort),
			textInput("content", "İçerik (markdown)", "", true, discordgo.TextInputParagraph),
			textInput("published", "Yayınla (evet/hayır)", "hayır", false, discordgo.TextInputShort),
		)
	case "edit":
		safeOpenModal(s, i, ownerModalPrefix+"blog_edit", "Blog yazısını düzenle",
			textInput("id", "Yazı id", "1", true, discordgo.TextInputShort),
			textInput("title", "Yeni başlık (boş = değişmez)", "", false, discordgo.TextInputShort),
			textInput("content", "Yeni içerik (boş = değişmez)", "", false, discordgo.TextInputParagraph),
			textInput("published", "Yayın durumu (evet/hayır/boş)", "", false, discordgo.TextInputShort),
		)
	case "delete":
		safeOpenModal(s, i, ownerModalPrefix+"blog_delete", "Blog yazısını sil",
			textInput("id", "Yazı id", "1", true, discordgo.TextInputShort),
			textInput("confirm", "Onay için SİL yazın", "SİL", true, discordgo.TextInputShort),
		)
	default:
		_ = respondPanel(s, i, textDisplay("Bilinmeyen blog işlemi."))
	}
}

func (b *Bot) handleOwnerTicketAction(s *discordgo.Session, i *discordgo.InteractionCreate, session *ownerSession, parts []string) {
	if len(parts) < 2 {
		_ = respondPanel(s, i, textDisplay("Geçersiz işlem parametresi."))
		return
	}
	switch parts[1] {
	case "reply":
		if !session.can(authz.TicketWrite) {
			b.denyOwnerAction(s, i)
			return
		}
		safeOpenModal(s, i, ownerModalPrefix+"ticket_reply", "Talebi yanıtla",
			textInput("id", "Talep id", "1", true, discordgo.TextInputShort),
			textInput("message", "Yanıt", "", true, discordgo.TextInputParagraph),
		)
	case "status":
		if !session.can(authz.TicketWrite) {
			b.denyOwnerAction(s, i)
			return
		}
		safeOpenModal(s, i, ownerModalPrefix+"ticket_status", "Talep durumu",
			textInput("id", "Talep id", "1", true, discordgo.TextInputShort),
			textInput("status", "Durum (open / answered / closed)", "closed", true, discordgo.TextInputShort),
		)
	case "delete":
		if !session.can(authz.TicketDelete) {
			b.denyOwnerAction(s, i)
			return
		}
		safeOpenModal(s, i, ownerModalPrefix+"ticket_delete", "Talebi sil",
			textInput("id", "Talep id", "1", true, discordgo.TextInputShort),
			textInput("confirm", "Onay için SİL yazın", "SİL", true, discordgo.TextInputShort),
		)
	default:
		_ = respondPanel(s, i, textDisplay("Bilinmeyen talep işlemi."))
	}
}

func (b *Bot) handleOwnerCouponAction(s *discordgo.Session, i *discordgo.InteractionCreate, session *ownerSession, parts []string) {
	if len(parts) < 2 {
		_ = respondPanel(s, i, textDisplay("Geçersiz işlem parametresi."))
		return
	}
	if !session.can(authz.RedemptionWrite) {
		b.denyOwnerAction(s, i)
		return
	}
	switch parts[1] {
	case "create":
		safeOpenModal(s, i, ownerModalPrefix+"coupon_create", "Kupon oluştur",
			textInput("name", "Kupon Adı / Açıklama", "Promosyon Kuponu", true, discordgo.TextInputShort),
			textInput("quota", "Bakiye Miktarı (quota, boş = 500000)", "500000", false, discordgo.TextInputShort),
		)
	case "delete":
		safeOpenModal(s, i, ownerModalPrefix+"coupon_delete", "Kuponu sil",
			textInput("id", "Kupon id", "1", true, discordgo.TextInputShort),
			textInput("confirm", "Onay için SİL yazın", "SİL", true, discordgo.TextInputShort),
		)
	default:
		_ = respondPanel(s, i, textDisplay("Geçersiz kupon işlemi."))
	}
}

func (b *Bot) denyOwnerAction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	_ = respondPanel(s, i, textDisplay(" **Yetki yok**\nBu işlem için izniniz bulunmuyor."))
}

// handleOwnerPanelModal runs the action a modal collected input for.
func (b *Bot) handleOwnerPanelModal(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.ModalSubmitInteractionData) {
	actionName := strings.TrimPrefix(data.CustomID, ownerModalPrefix)
	if actionName == "auth" {
		userID := interactionUserID(i)
		b.ownerMu.Lock()
		req, exists := b.ownerAuth[userID]
		b.ownerMu.Unlock()
		if !exists || time.Now().After(req.expiresAt) {
			_ = respondPanel(s, i, textDisplay("⚠️ Doğrulama süresi doldu. Lütfen `!owner` komutunu tekrar çalıştırın."))
			return
		}

		account, err := model.GetUserById(req.userID, true)
		if err != nil || account.Status != common.UserStatusEnabled {
			_ = respondPanel(s, i, textDisplay("❌ Hesap bulunamadı veya devre dışı."))
			return
		}

		pwd := strings.TrimSpace(modalValue(data, "password"))
		if !common.ValidatePasswordAndHash(pwd, account.Password) {
			req.attempts++
			if req.attempts >= ownerMaxAuthAttempts {
				b.ownerMu.Lock()
				delete(b.ownerAuth, userID)
				b.ownerMu.Unlock()
				_ = respondPanel(s, i, textDisplay(fmt.Sprintf("❌ **%d hatalı deneme.** Güvenlik nedeniyle oturum kilitlendi. Lütfen komutu yeniden çalıştırın.", req.attempts)))
				return
			}
			req.expiresAt = time.Now().Add(ownerAuthTTL)
			b.ownerMu.Lock()
			b.ownerAuth[userID] = req
			b.ownerMu.Unlock()

			_ = respondPanel(s, i,
				textDisplay(fmt.Sprintf("❌ **Şifre hatalı.** Kalan hak: %d", ownerMaxAuthAttempts-req.attempts)),
				actionRow(primaryButton(ownerPanelPrefix+"auth:prompt", "🔑 Tekrar Dene")),
			)
			return
		}

		b.ownerMu.Lock()
		delete(b.ownerAuth, userID)
		session := &ownerSession{
			userID:      account.Id,
			username:    account.Username,
			displayName: account.DisplayName,
			role:        account.Role,
			permissions: authz.Capabilities(account.Id, account.Role),
			section:     ownerSectionOverview,
			page:        1,
		}
		session.touch()
		b.ownerSessions[userID] = session
		b.ownerMu.Unlock()

		body, actions := b.renderOwnerSection(session)
		_ = replyPanel(s, i, ownerPanelLayout(session, body, actions, "✅ Giriş başarılı! Hoş geldiniz.")...)
		return
	}

	session := b.sessionFor(interactionUserID(i))
	if session == nil {
		_ = respondPanel(s, i, textDisplay(" **Oturum kapandı.** DM'den komutu yeniden çalıştırın."))
		return
	}

	var notice string
	switch actionName {
	case "user_create":
		notice = b.runOwnerUserCreate(session, data)
	case "user_quota":
		notice = b.runOwnerUserQuota(session, data)
	case "user_status":
		notice = b.runOwnerUserStatus(session, data)
	case "user_delete":
		notice = b.runOwnerUserDelete(session, data)
	case "blog_create":
		notice = b.runOwnerBlogCreate(session, data)
	case "blog_edit":
		notice = b.runOwnerBlogEdit(session, data)
	case "blog_delete":
		notice = b.runOwnerBlogDelete(session, data)
	case "ticket_reply":
		notice = b.runOwnerTicketReply(session, data)
	case "ticket_status":
		notice = b.runOwnerTicketStatus(session, data)
	case "ticket_delete":
		notice = b.runOwnerTicketDelete(session, data)
	case "coupon_create":
		notice = b.runOwnerCouponCreate(session, data)
	case "coupon_delete":
		notice = b.runOwnerCouponDelete(session, data)
	default:
		notice = " **Bilinmeyen işlem.**"
	}

	body, actions := b.renderOwnerSection(session)
	if err := replyPanel(s, i, ownerPanelLayout(session, body, actions, notice)...); err != nil {
		if ackErr := respondPanel(s, i, ownerPanelLayout(session, body, actions, notice)...); ackErr != nil {
			_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: notice,
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			b.sendOwnerMessage(s, i.ChannelID, ownerPanelLayout(session, body, actions, notice)...)
		}
	}
}

func (b *Bot) runOwnerUserCreate(session *ownerSession, data discordgo.ModalSubmitInteractionData) string {
	if !session.can(authz.UserWrite) {
		return " **Yetki yok.**"
	}
	username := modalValue(data, "username")
	password := modalValue(data, "password")
	if len(username) < 2 || len(username) > 20 {
		return " Kullanıcı adı 2-20 karakter olmalı."
	}
	if len(password) < 8 || len(password) > 20 {
		return " Şifre 8-20 karakter olmalı."
	}

	user := &model.User{
		Username:    username,
		Password:    password,
		DisplayName: modalValue(data, "display_name"),
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
	}
	if user.DisplayName == "" {
		user.DisplayName = username
	}
	if err := user.Insert(0); err != nil {
		return " **Kullanıcı oluşturulamadı**\n" + err.Error()
	}

	created, err := resolveOwnerTargetUser(username)
	if err != nil {
		return fmt.Sprintf(" **%s** oluşturuldu (id çözümlenemedi: %v)", username, err)
	}

	if raw := modalValue(data, "quota"); raw != "" {
		quota, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Sprintf(" **%s** (`#%d`) oluşturuldu, ancak bakiye okunamadı: %s", username, created.Id, raw)
		}
		if err := model.DeltaUpdateUserQuota(created.Id, quota); err != nil {
			return fmt.Sprintf(" **%s** (`#%d`) oluşturuldu, ancak bakiye eklenemedi: %v", username, created.Id, err)
		}
	}
	return fmt.Sprintf(" **%s** (`#%d`) oluşturuldu.", username, created.Id)
}

func (b *Bot) runOwnerUserQuota(session *ownerSession, data discordgo.ModalSubmitInteractionData) string {
	if !session.can(authz.UserWrite) {
		return " **Yetki yok.**"
	}
	user, err := resolveOwnerTargetUser(modalValue(data, "target"))
	if err != nil {
		return " **Kullanıcı bulunamadı**\n" + err.Error()
	}
	delta, err := strconv.Atoi(modalValue(data, "delta"))
	if err != nil || delta == 0 {
		return " Geçerli, sıfırdan farklı bir sayı girin."
	}
	if err := model.DeltaUpdateUserQuota(user.Id, delta); err != nil {
		return " **Bakiye güncellenemedi**\n" + err.Error()
	}
	updated, err := model.GetUserById(user.Id, false)
	if err != nil {
		return fmt.Sprintf(" **%s** bakiyesi güncellendi.", user.Username)
	}
	return fmt.Sprintf(" **%s** (`#%d`) yeni bakiye: **%s**", updated.Username, updated.Id, logger.FormatQuota(updated.Quota))
}

func (b *Bot) runOwnerUserStatus(session *ownerSession, data discordgo.ModalSubmitInteractionData) string {
	if !session.can(authz.UserWrite) {
		return " **Yetki yok.**"
	}
	user, err := resolveOwnerTargetUser(modalValue(data, "target"))
	if err != nil {
		return " **Kullanıcı bulunamadı**\n" + err.Error()
	}
	if user.Id == session.userID {
		return " Kendi hesabınızın durumunu buradan değiştiremezsiniz."
	}

	switch strings.ToLower(modalValue(data, "status")) {
	case "aktif", "active", "acik", "açık", "enable", "enabled", "1":
		user.Status = common.UserStatusEnabled
	case "pasif", "inactive", "kapali", "kapalı", "disable", "disabled", "0":
		user.Status = common.UserStatusDisabled
	default:
		return " Durum `aktif` veya `pasif` olmalı."
	}
	if err := user.Update(false); err != nil {
		return " **Durum güncellenemedi**\n" + err.Error()
	}
	return fmt.Sprintf(" **%s** (`#%d`) durumu: **%s**", user.Username, user.Id, describeStatus(user.Status))
}

func (b *Bot) runOwnerUserDelete(session *ownerSession, data discordgo.ModalSubmitInteractionData) string {
	if !session.can(authz.UserDelete) {
		return " **Yetki yok.**"
	}
	if !strings.EqualFold(modalValue(data, "confirm"), "sil") {
		return " Onay için tam olarak `SİL` yazmalısınız."
	}
	user, err := resolveOwnerTargetUser(modalValue(data, "target"))
	if err != nil {
		return " **Kullanıcı bulunamadı**\n" + err.Error()
	}
	if user.Id == session.userID {
		return " Kendi hesabınızı silemezsiniz."
	}
	if user.Role >= common.RoleRootUser {
		return " Kurucu hesabı bot üzerinden silinemez."
	}
	if err := model.DeleteUserById(user.Id); err != nil {
		return " **Kullanıcı silinemedi**\n" + err.Error()
	}
	return fmt.Sprintf(" **%s** (`#%d`) silindi.", user.Username, user.Id)
}

func (b *Bot) runOwnerBlogCreate(session *ownerSession, data discordgo.ModalSubmitInteractionData) string {
	if !session.can(authz.BlogWrite) {
		return " **Yetki yok.**"
	}
	title := modalValue(data, "title")
	content := modalValue(data, "content")
	if title == "" || content == "" {
		return " Başlık ve içerik zorunludur."
	}

	post := &model.BlogPost{
		Title:     title,
		Summary:   modalValue(data, "summary"),
		Content:   content,
		Published: parseOwnerBool(modalValue(data, "published")),
	}
	if err := model.CreateBlogPost(post); err != nil {
		return " **Yazı eklenemedi**\n" + err.Error()
	}
	return fmt.Sprintf(" Blog yazısı eklendi: **%s** (`#%d`)", post.Title, post.Id)
}

func (b *Bot) runOwnerBlogEdit(session *ownerSession, data discordgo.ModalSubmitInteractionData) string {
	if !session.can(authz.BlogWrite) {
		return " **Yetki yok.**"
	}
	id, err := strconv.ParseInt(modalValue(data, "id"), 10, 64)
	if err != nil {
		return " Geçerli bir yazı id girin."
	}
	post, err := model.GetBlogPostById(id)
	if err != nil {
		return " **Yazı bulunamadı**\n" + err.Error()
	}
	if title := modalValue(data, "title"); title != "" {
		post.Title = title
	}
	if content := modalValue(data, "content"); content != "" {
		post.Content = content
	}
	if published := modalValue(data, "published"); published != "" {
		post.Published = parseOwnerBool(published)
	}
	if err := model.UpdateBlogPost(post); err != nil {
		return " **Yazı güncellenemedi**\n" + err.Error()
	}
	return fmt.Sprintf(" Blog yazısı güncellendi: **%s** (`#%d`)", post.Title, post.Id)
}

func (b *Bot) runOwnerBlogDelete(session *ownerSession, data discordgo.ModalSubmitInteractionData) string {
	if !session.can(authz.BlogWrite) {
		return " **Yetki yok.**"
	}
	if !strings.EqualFold(modalValue(data, "confirm"), "sil") {
		return " Onay için tam olarak `SİL` yazmalısınız."
	}
	id, err := strconv.ParseInt(modalValue(data, "id"), 10, 64)
	if err != nil {
		return " Geçerli bir yazı id girin."
	}
	if err := model.DeleteBlogPost(id); err != nil {
		return " **Yazı silinemedi**\n" + err.Error()
	}
	return fmt.Sprintf(" Blog yazısı `#%d` silindi.", id)
}

func (b *Bot) runOwnerTicketReply(session *ownerSession, data discordgo.ModalSubmitInteractionData) string {
	if !session.can(authz.TicketWrite) {
		return " **Yetki yok.**"
	}
	id, err := strconv.Atoi(modalValue(data, "id"))
	if err != nil || id <= 0 {
		return " Geçerli bir talep id girin."
	}
	message := modalValue(data, "message")
	if message == "" {
		return " Yanıt boş olamaz."
	}
	if _, err := model.AddTicketMessage(id, session.userID, true, message, ""); err != nil {
		return " **Yanıt gönderilemedi**\n" + err.Error()
	}
	return fmt.Sprintf(" `#%d` numaralı talebe yanıt eklendi.", id)
}

func (b *Bot) runOwnerTicketStatus(session *ownerSession, data discordgo.ModalSubmitInteractionData) string {
	if !session.can(authz.TicketWrite) {
		return " **Yetki yok.**"
	}
	id, err := strconv.Atoi(modalValue(data, "id"))
	if err != nil || id <= 0 {
		return " Geçerli bir talep id girin."
	}
	status := strings.ToLower(modalValue(data, "status"))
	switch status {
	case model.TicketStatusOpen, model.TicketStatusAnswered, model.TicketStatusClosed:
	default:
		return " Durum `open`, `answered` veya `closed` olmalı."
	}
	if err := model.UpdateTicketStatus(id, status); err != nil {
		return " **Durum güncellenemedi**\n" + err.Error()
	}
	return fmt.Sprintf(" `#%d` numaralı talebin durumu: **%s**", id, status)
}

func (b *Bot) runOwnerTicketDelete(session *ownerSession, data discordgo.ModalSubmitInteractionData) string {
	if !session.can(authz.TicketDelete) {
		return " **Yetki yok.**"
	}
	if !strings.EqualFold(modalValue(data, "confirm"), "sil") {
		return " Onay için tam olarak `SİL` yazmalısınız."
	}
	id, err := strconv.Atoi(modalValue(data, "id"))
	if err != nil || id <= 0 {
		return " Geçerli bir talep id girin."
	}
	if err := model.DeleteTicket(id); err != nil {
		return " **Talep silinemedi**\n" + err.Error()
	}
	return fmt.Sprintf(" `#%d` numaralı talep silindi.", id)
}

func (b *Bot) runOwnerCouponDelete(session *ownerSession, data discordgo.ModalSubmitInteractionData) string {
	if !session.can(authz.RedemptionWrite) {
		return " **Yetki yok.**"
	}
	if !strings.EqualFold(modalValue(data, "confirm"), "sil") {
		return " Onay için tam olarak `SİL` yazmalısınız."
	}
	id, err := strconv.Atoi(modalValue(data, "id"))
	if err != nil || id <= 0 {
		return " Geçerli bir kupon id girin."
	}
	coupon := model.Redemption{Id: id}
	if err := coupon.Delete(); err != nil {
		return " **Kupon silinemedi**\n" + err.Error()
	}
	return fmt.Sprintf(" Kupon `#%d` silindi.", id)
}

func (b *Bot) runOwnerCouponCreate(session *ownerSession, data discordgo.ModalSubmitInteractionData) string {
	if !session.can(authz.RedemptionWrite) {
		return "⚠️ Yetki yok."
	}
	name := strings.TrimSpace(modalValue(data, "name"))
	if name == "" {
		name = "Discord Kupon"
	}
	quota := 500000
	if qStr := strings.TrimSpace(modalValue(data, "quota")); qStr != "" {
		if parsed, err := strconv.Atoi(qStr); err == nil && parsed > 0 {
			quota = parsed
		}
	}
	redemption := &model.Redemption{
		UserId:      session.userID,
		Name:        name,
		Key:         common.GetUUID(),
		Status:      common.RedemptionCodeStatusEnabled,
		Quota:       quota,
		CreatedTime: time.Now().Unix(),
	}
	if err := redemption.Insert(); err != nil {
		return "⚠️ Kupon oluşturulamadı: " + err.Error()
	}
	return fmt.Sprintf("✅ **Kupon oluşturuldu!**\n🎟️ Kod: `%s`\n💰 Değer: `%s`", redemption.Key, logger.FormatQuota(redemption.Quota))
}

// resolveOwnerTargetUser accepts either a numeric user id or a username.
func resolveOwnerTargetUser(value string) (*model.User, error) {
	value = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(value), "#"))
	if value == "" {
		return nil, errors.New("kullanıcı belirtilmedi")
	}
	if id, err := strconv.Atoi(value); err == nil {
		return model.GetUserById(id, true)
	}

	var user model.User
	if err := model.DB.Where("username = ?", value).First(&user).Error; err != nil {
		return nil, fmt.Errorf("`%s` kullanıcısı bulunamadı", value)
	}
	return &user, nil
}

func parseOwnerBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "evet", "e", "yes", "y", "true", "1", "açık", "acik", "on":
		return true
	default:
		return false
	}
}
