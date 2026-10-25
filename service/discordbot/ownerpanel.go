package discordbot

import (
	"fmt"
	"log"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/authz"
	"github.com/bwmarrin/discordgo"
)

// The owner panel is a DM-only administration surface. It requires the caller's
// Discord account to be linked to a site administrator and re-verifies the site
// password on every invocation. Sections and actions are gated by the same authz
// permissions the web dashboard uses, so the bot can never expose more than the
// web UI would.
const (
	ownerPanelPrefix = "owner:"

	ownerSessionTTL      = 30 * time.Minute
	ownerAuthTTL         = 3 * time.Minute
	ownerMaxAuthAttempts = 3

	ownerPageSize = 8

	ownerSectionOverview = "overview"
	ownerSectionUsers    = "users"
	ownerSectionBlog     = "blog"
	ownerSectionTickets  = "tickets"
	ownerSectionCoupons  = "coupons"
	ownerSectionSettings = "settings"
)

type ownerSession struct {
	userID      int
	username    string
	displayName string
	role        int
	permissions authz.PermissionsMap
	expiresAt   time.Time
	section     string
	page        int
}

func (s *ownerSession) can(permission authz.Permission) bool {
	if s == nil {
		return false
	}
	return s.permissions[permission.Resource][permission.Action]
}

func (s *ownerSession) touch() {
	s.expiresAt = time.Now().Add(ownerSessionTTL)
	if s.page < 1 {
		s.page = 1
	}
}

type ownerAuthRequest struct {
	userID    int
	username  string
	expiresAt time.Time
	attempts  int
}

type ownerSection struct {
	id    string
	label string
}

// availableSections lists the panel sections the session's permissions unlock.
func (s *ownerSession) availableSections() []ownerSection {
	sections := []ownerSection{{ownerSectionOverview, "Genel Bakış"}}
	if s.can(authz.UserRead) {
		sections = append(sections, ownerSection{ownerSectionUsers, "Kullanıcılar"})
	}
	if s.can(authz.BlogRead) {
		sections = append(sections, ownerSection{ownerSectionBlog, "Blog"})
	}
	if s.can(authz.TicketRead) {
		sections = append(sections, ownerSection{ownerSectionTickets, "Talepler"})
	}
	if s.can(authz.RedemptionRead) {
		sections = append(sections, ownerSection{ownerSectionCoupons, "Kuponlar"})
	}
	if s.can(authz.SystemSettingRead) {
		sections = append(sections, ownerSection{ownerSectionSettings, "Sistem"})
	}
	return sections
}

func (s *ownerSession) allowsSection(section string) bool {
	for _, candidate := range s.availableSections() {
		if candidate.id == section {
			return true
		}
	}
	return false
}

// handleOwnerPanelCommand starts the DM authentication challenge. The panel is
// never opened from a guild channel.
func (b *Bot) handleOwnerPanelCommand(s *discordgo.Session, m *discordgo.MessageCreate) {
	settings := currentSettings()

	if m.GuildID != "" {
		b.sendOwnerMessage(s, m.ChannelID, textDisplay(
			" **Yönetici paneli yalnızca özel mesaj (DM) üzerinden kullanılabilir.**\n"+
				"Bota DM gönderip "+inlineCode(settings.Prefix+"owner")+" yazın."))
		return
	}

	account, err := findUserByDiscordID(m.Author.ID)
	if err != nil {
		b.sendOwnerMessage(s, m.ChannelID, textDisplay(
			" **Hesap bağlı değil**\nBu Discord hesabı siteye bağlı bir kullanıcıya ait değil.\n"+
				"Önce site üzerinden **Discord ile bağla** işlemini tamamlayın."))
		return
	}
	if account.Role < common.RoleAdminUser {
		b.sendOwnerMessage(s, m.ChannelID, textDisplay(
			" **Yetki yok**\nBağlı hesabın yönetici yetkisi bulunmuyor."))
		return
	}
	if account.Status != common.UserStatusEnabled {
		b.sendOwnerMessage(s, m.ChannelID, textDisplay(" **Hesap devre dışı**\nBağlı site hesabı etkin değil."))
		return
	}

	session := b.sessionFor(m.Author.ID)
	if session != nil {
		body, actions := b.renderOwnerSection(session)
		b.sendOwnerMessage(s, m.ChannelID, ownerPanelLayout(session, body, actions, "👋 Oturumunuz aktif.")...)
		return
	}

	b.ownerMu.Lock()
	delete(b.ownerSessions, m.Author.ID)
	b.ownerAuth[m.Author.ID] = &ownerAuthRequest{
		userID:    account.Id,
		username:  account.Username,
		expiresAt: time.Now().Add(ownerAuthTTL),
	}
	b.ownerMu.Unlock()

	b.sendOwnerMessage(s, m.ChannelID,
		textDisplay(fmt.Sprintf("🛡️ **Yönetici Doğrulaması**\nHesap: `%s` (`#%d`)\nRol: %s",
			account.Username, account.Id, describeRole(account.Role))),
		separator(),
		textDisplay(fmt.Sprintf(
			"Güvenliğiniz için site şifrenizi aşağıdaki butona tıklayarak açılan form penceresinden girin.\n"+
				"Şifreniz sohbette görünmez ve kaydedilmez.\n"+
				"Oturum süresi: **%s** | Kalan hak: %d deneme",
			formatDuration(ownerAuthTTL), ownerMaxAuthAttempts)),
		actionRow(primaryButton(ownerPanelPrefix+"auth:prompt", "🔑 Şifre ile Giriş Yap")),
	)
}

// consumeOwnerAuthReply handles the DM that is expected to contain the password.
// It reports whether the message was consumed, so it is never treated as a
// command or an auto-reply prompt.
func (b *Bot) consumeOwnerAuthReply(s *discordgo.Session, m *discordgo.MessageCreate) bool {
	b.ownerMu.Lock()
	request, pending := b.ownerAuth[m.Author.ID]
	if !pending {
		b.ownerMu.Unlock()
		return false
	}
	delete(b.ownerAuth, m.Author.ID)
	b.ownerMu.Unlock()

	// Remove the password message immediately; a failed delete is logged without
	// ever logging the message body.
	if err := s.ChannelMessageDelete(m.ChannelID, m.ID); err != nil {
		log.Printf("Discord owner panel could not delete the password message: %v", err)
	}

	if time.Now().After(request.expiresAt) {
		b.sendOwnerMessage(s, m.ChannelID, textDisplay(" **Doğrulama süresi doldu.** Komutu yeniden çalıştırın."))
		return true
	}

	account, err := model.GetUserById(request.userID, true)
	if err != nil {
		b.sendOwnerMessage(s, m.ChannelID, textDisplay(" **Hesap bulunamadı.**"))
		return true
	}
	if account.Status != common.UserStatusEnabled {
		b.sendOwnerMessage(s, m.ChannelID, textDisplay(" **Hesap devre dışı.**"))
		return true
	}
	if !common.ValidatePasswordAndHash(strings.TrimSpace(m.Content), account.Password) {
		request.attempts++
		if request.attempts >= ownerMaxAuthAttempts {
			b.sendOwnerMessage(s, m.ChannelID, textDisplay(fmt.Sprintf(
				" **%d hatalı deneme.** Oturum kapatıldı, komutu yeniden çalıştırın.", request.attempts)))
			return true
		}
		request.expiresAt = time.Now().Add(ownerAuthTTL)
		b.ownerMu.Lock()
		b.ownerAuth[m.Author.ID] = request
		b.ownerMu.Unlock()
		b.sendOwnerMessage(s, m.ChannelID, textDisplay(fmt.Sprintf(
			"❌ **Şifre hatalı.** Kalan deneme: %d", ownerMaxAuthAttempts-request.attempts)))
		return true
	}

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

	b.ownerMu.Lock()
	b.ownerSessions[m.Author.ID] = session
	b.ownerMu.Unlock()

	b.sendOwnerMessage(s, m.ChannelID, b.renderOwnerPanel(session)...)
	return true
}

// sessionFor returns the live session for a Discord user, dropping it when it
// expired.
func (b *Bot) sessionFor(discordUserID string) *ownerSession {
	b.ownerMu.Lock()
	defer b.ownerMu.Unlock()

	session, ok := b.ownerSessions[discordUserID]
	if !ok {
		return nil
	}
	if time.Now().After(session.expiresAt) {
		delete(b.ownerSessions, discordUserID)
		return nil
	}
	session.touch()
	return session
}

func (b *Bot) dropSession(discordUserID string) {
	b.ownerMu.Lock()
	delete(b.ownerSessions, discordUserID)
	b.ownerMu.Unlock()
}

func (b *Bot) sendOwnerMessage(s *discordgo.Session, channelID string, components ...discordgo.MessageComponent) {
	if _, err := sendPanel(s, channelID, components...); err != nil {
		log.Printf("Discord owner panel message failed: %v", err)
	}
}

// renderOwnerPanel builds the Components V2 tree for the session's section.
func (b *Bot) renderOwnerPanel(session *ownerSession) []discordgo.MessageComponent {
	session.normalizeSection()
	body, actions := b.renderOwnerSection(session)
	return ownerPanelLayout(session, body, actions, "")
}

// ownerPanelLayout assembles the shared panel chrome: header, body, section
// selector, section actions and the footer.
func ownerPanelLayout(session *ownerSession, body string, actions []discordgo.MessageComponent, notice string) []discordgo.MessageComponent {
	components := make([]discordgo.MessageComponent, 0, 8)
	if notice != "" {
		components = append(components, textDisplay(notice), separator())
	}
	headerText := fmt.Sprintf("🛡️ **Yönetici Kontrol Paneli**\n👤 **Yönetici:** `%s` (#%d) • **Rol:** `%s`\n⏳ **Oturum:** `%s` kaldı",
		session.username, session.userID, describeRole(session.role),
		formatDuration(time.Until(session.expiresAt)))
	components = append(components,
		textDisplay(headerText),
		separator(),
		textDisplay(body),
		separator(),
		actionRow(ownerSectionSelect(session)),
	)
	if len(actions) > 0 {
		var currentButtons []discordgo.MessageComponent
		flushButtons := func() {
			if len(currentButtons) > 0 {
				components = append(components, actionRow(currentButtons...))
				currentButtons = nil
			}
		}
		for _, act := range actions {
			switch val := act.(type) {
			case discordgo.SelectMenu, *discordgo.SelectMenu:
				flushButtons()
				components = append(components, actionRow(val))
			case discordgo.ActionsRow:
				flushButtons()
				components = append(components, val)
			case *discordgo.ActionsRow:
				if val != nil {
					flushButtons()
					components = append(components, *val)
				}
			default:
				currentButtons = append(currentButtons, act)
				if len(currentButtons) == 5 {
					flushButtons()
				}
			}
		}
		flushButtons()
	}
	return append(components, actionRow(
		secondaryButton(ownerPanelPrefix+"refresh", "🔄 Yenile"),
		dangerButton(ownerPanelPrefix+"exit", "🚪 Çıkış"),
	))
}

// normalizeSection keeps the session on a section the permissions still allow.
func (s *ownerSession) normalizeSection() {
	if s.section == "" || !s.allowsSection(s.section) {
		s.section = ownerSectionOverview
	}
	if s.page < 1 {
		s.page = 1
	}
}

func ownerSectionSelect(session *ownerSession) discordgo.SelectMenu {
	options := make([]discordgo.SelectMenuOption, 0, 6)
	for _, section := range session.availableSections() {
		options = append(options, discordgo.SelectMenuOption{
			Label:   section.label,
			Value:   section.id,
			Default: section.id == session.section,
		})
	}
	return discordgo.SelectMenu{
		MenuType:    discordgo.StringSelectMenu,
		CustomID:    ownerPanelPrefix + "nav",
		Placeholder: "Bölüm seçin",
		Options:     options,
	}
}

func (b *Bot) renderOwnerSection(session *ownerSession) (string, []discordgo.MessageComponent) {
	switch session.section {
	case ownerSectionUsers:
		return b.renderOwnerUsers(session)
	case ownerSectionBlog:
		return b.renderOwnerBlog(session)
	case ownerSectionTickets:
		return b.renderOwnerTickets(session)
	case ownerSectionCoupons:
		return b.renderOwnerCoupons(session)
	case ownerSectionSettings:
		return b.renderOwnerSettings(session)
	default:
		return b.renderOwnerOverview(session)
	}
}

func (b *Bot) renderOwnerOverview(session *ownerSession) (string, []discordgo.MessageComponent) {
	lines := []string{"📊 **Genel Bakış & Sistem Durumu**", "", "**Kullanılabilir Yönetim Bölümleri:**"}
	for _, section := range session.availableSections() {
		lines = append(lines, "  📁 "+section.label)
	}

	counts := []string{}
	if session.can(authz.UserRead) {
		if _, total, err := model.GetAllUsers(&common.PageInfo{Page: 1, PageSize: 1}); err == nil {
			counts = append(counts, fmt.Sprintf("• 👥 Kullanıcılar: **%d**", total))
		}
	}
	if session.can(authz.ChannelRead) {
		if total, err := model.CountAllChannels(); err == nil {
			counts = append(counts, fmt.Sprintf("• ⚡ AI Kanalları: **%d**", total))
		}
	}
	if session.can(authz.BlogRead) {
		if total, err := model.GetBlogPostCount(false); err == nil {
			counts = append(counts, fmt.Sprintf("• 📝 Blog Yazıları: **%d**", total))
		}
	}
	if session.can(authz.TicketRead) {
		if _, total, err := model.GetAllTickets(1, 1, "", "", "", ""); err == nil {
			counts = append(counts, fmt.Sprintf("• 🎫 Destek Talepleri: **%d**", total))
		}
	}
	if session.can(authz.RedemptionRead) {
		if _, total, err := model.GetAllRedemptions(0, 1); err == nil {
			counts = append(counts, fmt.Sprintf("• 🎟️ Hediye Kuponları: **%d**", total))
		}
	}
	if len(counts) > 0 {
		lines = append(lines, "", "**📈 Kayıt & Kaynak Özeti:**")
		lines = append(lines, counts...)
	}

	var mStats runtime.MemStats
	runtime.ReadMemStats(&mStats)
	lines = append(lines, "", fmt.Sprintf("🟢 **Sistem:** Aktif & Sağlıklı | **Goroutine:** `%d` | **RAM:** `%.1f MB`",
		runtime.NumGoroutine(), float64(mStats.Alloc)/1024/1024))

	return strings.Join(lines, "\n"), nil
}
func ownerActionSelect(section string, placeholder string, options ...discordgo.SelectMenuOption) discordgo.SelectMenu {
	return discordgo.SelectMenu{
		MenuType:    discordgo.StringSelectMenu,
		CustomID:    ownerPanelPrefix + "action:" + section,
		Placeholder: placeholder,
		Options:     options,
	}
}

func (b *Bot) renderOwnerUsers(session *ownerSession) (string, []discordgo.MessageComponent) {
	users, total, err := model.GetAllUsers(&common.PageInfo{Page: session.page, PageSize: ownerPageSize})
	if err != nil {
		return "Kullanicilar okunamadi:\n" + err.Error(), nil
	}

	entries := make([]string, 0, len(users))
	for _, user := range users {
		entries = append(entries, fmt.Sprintf("`#%d` %s - %s - %s - kota %s",
			user.Id, user.Username, describeRole(user.Role), describeStatus(user.Status), logger.FormatQuota(user.Quota)))
	}
	body := fmt.Sprintf("Kullanicilar - sayfa %d - toplam %d\n\n%s",
		session.page, total, bulletList(entries, "Kayit yok."))

	actions := []discordgo.MessageComponent{}
	if session.can(authz.UserWrite) || session.can(authz.UserDelete) {
		opts := []discordgo.SelectMenuOption{}
		if session.can(authz.UserWrite) {
			opts = append(opts,
				discordgo.SelectMenuOption{Label: "Kullanici olustur", Value: "user:create"},
				discordgo.SelectMenuOption{Label: "Bakiye islemi", Value: "user:quota"},
				discordgo.SelectMenuOption{Label: "Durum degistir", Value: "user:status"},
			)
		}
		if session.can(authz.UserDelete) {
			opts = append(opts, discordgo.SelectMenuOption{Label: "Kullanici sil", Value: "user:delete"})
		}
		actions = append(actions, ownerActionSelect(ownerSectionUsers, "Kullanici islemi secin", opts...))
	}
	actions = append(actions, paginationActions(session, total)...)
	return body, actions
}

func (b *Bot) renderOwnerBlog(session *ownerSession) (string, []discordgo.MessageComponent) {
	posts, err := model.GetBlogPosts(session.page, ownerPageSize, false)
	if err != nil {
		return "Blog okunamadi:\n" + err.Error(), nil
	}
	total, _ := model.GetBlogPostCount(false)

	entries := make([]string, 0, len(posts))
	for _, post := range posts {
		state := "taslak"
		if post.Published {
			state = "yayinda"
		}
		entries = append(entries, fmt.Sprintf("`#%d` %s - %s", post.Id, truncateText(post.Title, 80), state))
	}
	body := fmt.Sprintf("Blog yazilari - sayfa %d - toplam %d\n\n%s",
		session.page, total, bulletList(entries, "Kayit yok."))

	actions := []discordgo.MessageComponent{}
	if session.can(authz.BlogWrite) {
		actions = append(actions, ownerActionSelect(ownerSectionBlog, "Blog islemi secin",
			discordgo.SelectMenuOption{Label: "Yazi ekle", Value: "blog:create"},
			discordgo.SelectMenuOption{Label: "Yazi duzenle", Value: "blog:edit"},
			discordgo.SelectMenuOption{Label: "Yazi sil", Value: "blog:delete"},
		))
	}
	actions = append(actions, paginationActions(session, total)...)
	return body, actions
}

func (b *Bot) renderOwnerTickets(session *ownerSession) (string, []discordgo.MessageComponent) {
	tickets, total, err := model.GetAllTickets(session.page, ownerPageSize, "", "", "", "")
	if err != nil {
		return "Talepler okunamadi:\n" + err.Error(), nil
	}

	entries := make([]string, 0, len(tickets))
	for _, ticket := range tickets {
		entries = append(entries, fmt.Sprintf("`#%d` %s - %s - %s",
			ticket.Id, truncateText(ticket.Title, 70), ticket.Status, ticket.Priority))
	}
	body := fmt.Sprintf("Talepler - sayfa %d - toplam %d\n\n%s",
		session.page, total, bulletList(entries, "Kayit yok."))

	actions := []discordgo.MessageComponent{}
	opts := []discordgo.SelectMenuOption{}
	if session.can(authz.TicketWrite) {
		opts = append(opts,
			discordgo.SelectMenuOption{Label: "Talebi yanitla", Value: "ticket:reply"},
			discordgo.SelectMenuOption{Label: "Durum degistir", Value: "ticket:status"},
		)
	}
	if session.can(authz.TicketDelete) {
		opts = append(opts, discordgo.SelectMenuOption{Label: "Talebi sil", Value: "ticket:delete"})
	}
	if len(opts) > 0 {
		actions = append(actions, ownerActionSelect(ownerSectionTickets, "Talep islemi secin", opts...))
	}
	actions = append(actions, paginationActions(session, total)...)
	return body, actions
}

func (b *Bot) renderOwnerCoupons(session *ownerSession) (string, []discordgo.MessageComponent) {
	coupons, total, err := model.GetAllRedemptions((session.page-1)*ownerPageSize, ownerPageSize)
	if err != nil {
		return "Kuponlar okunamadi:\n" + err.Error(), nil
	}

	entries := make([]string, 0, len(coupons))
	for _, coupon := range coupons {
		entries = append(entries, fmt.Sprintf("`#%d` %s - %s - %s",
			coupon.Id, truncateText(coupon.Name, 40), describeRedemptionStatus(coupon.Status), logger.FormatQuota(coupon.Quota)))
	}
	body := fmt.Sprintf("Kuponlar - sayfa %d - toplam %d\n\n%s",
		session.page, total, bulletList(entries, "Kayit yok."))

	actions := []discordgo.MessageComponent{}
	if session.can(authz.RedemptionWrite) {
		actions = append(actions, ownerActionSelect(ownerSectionCoupons, "Kupon islemi secin",
			discordgo.SelectMenuOption{Label: "Kupon olustur", Value: "coupon:create"},
			discordgo.SelectMenuOption{Label: "Kupon sil", Value: "coupon:delete"},
		))
	}
	actions = append(actions, paginationActions(session, total)...)
	return body, actions
}

func (b *Bot) renderOwnerSettings(session *ownerSession) (string, []discordgo.MessageComponent) {
	settings := currentSettings()

	lines := []string{
		" **Sistem**",
		"",
		field("Site adı", common.SystemName),
		field("Kayıt açık", describeBool(common.RegisterEnabled)),
		field("E-posta doğrulaması", describeBool(common.EmailVerificationEnabled)),
		field("Discord bot", describeBool(settings.Enabled)),
		field("Bot token", maskSecret(settings.BotToken)),
		field("Komut öneki", settings.Prefix),
		field("Otomatik yanıt kanalı", settings.AutoReplyChannelID),
		field("Otomatik yanıt modeli", settings.AutoReplyModel),
		field("Görsel modeli", settings.ImageModel),
	}
	return strings.Join(lines, "\n"), nil
}

func paginationActions(session *ownerSession, total int64) []discordgo.MessageComponent {
	if total <= int64(ownerPageSize) {
		return nil
	}
	actions := []discordgo.MessageComponent{}
	if session.page > 1 {
		actions = append(actions, secondaryButton(ownerPanelPrefix+"page:"+strconv.Itoa(session.page-1), "Geri"))
	}
	if int64(session.page)*int64(ownerPageSize) < total {
		actions = append(actions, secondaryButton(ownerPanelPrefix+"page:"+strconv.Itoa(session.page+1), "Ileri"))
	}
	return actions
}

// handleOwnerPanelComponent routes a button or select-menu click.
func (b *Bot) handleOwnerPanelComponent(s *discordgo.Session, i *discordgo.InteractionCreate, data discordgo.MessageComponentInteractionData) {
	if data.CustomID == ownerPanelPrefix+"auth:prompt" {
		b.ownerMu.Lock()
		req, exists := b.ownerAuth[interactionUserID(i)]
		b.ownerMu.Unlock()
		if !exists || time.Now().After(req.expiresAt) {
			_ = respondPanel(s, i, textDisplay("⚠️ Doğrulama süresi doldu. Lütfen `!owner` komutunu tekrar çalıştırın."))
			return
		}
		safeOpenModal(s, i, ownerModalPrefix+"auth", "Yönetici Girişi",
			textInput("password", "Site Şifreniz", "Yönetici şifrenizi girin", true, discordgo.TextInputShort),
		)
		return
	}

	session := b.sessionFor(interactionUserID(i))
	if session == nil {
		_ = respondPanel(s, i, textDisplay("Oturum kapandi. DM'den komutu yeniden calistirin."))
		return
	}

	trimmed := strings.TrimPrefix(data.CustomID, ownerPanelPrefix)
	parts := strings.Split(trimmed, ":")
	switch {
	case data.CustomID == ownerPanelPrefix+"nav" && len(data.Values) > 0:
		if session.allowsSection(data.Values[0]) {
			session.section = data.Values[0]
		}
		session.page = 1
		b.updateOwnerPanel(s, i, session, "")
	case data.CustomID == ownerPanelPrefix+"refresh":
		b.updateOwnerPanel(s, i, session, "")
	case data.CustomID == ownerPanelPrefix+"exit":
		b.dropSession(interactionUserID(i))
		_ = replyPanel(s, i, textDisplay("Yonetici oturumu kapatildi."))
	case len(parts) >= 2 && parts[0] == "page":
		if page, err := strconv.Atoi(parts[1]); err == nil && page > 0 {
			session.page = page
		}
		b.updateOwnerPanel(s, i, session, "")
	case len(parts) >= 2 && parts[0] == "action" && len(data.Values) > 0:
		b.handleOwnerActionSelect(s, i, session, parts[1], data.Values[0])
	case parts[0] == "user":
		b.handleOwnerUserAction(s, i, session, parts)
	case parts[0] == "blog":
		b.handleOwnerBlogAction(s, i, session, parts)
	case parts[0] == "ticket":
		b.handleOwnerTicketAction(s, i, session, parts)
	case parts[0] == "coupon":
		b.handleOwnerCouponAction(s, i, session, parts)
	default:
		_ = respondPanel(s, i, textDisplay("Bilinmeyen islem. Paneli yenileyin."))
	}
}

func (b *Bot) handleOwnerActionSelect(s *discordgo.Session, i *discordgo.InteractionCreate, session *ownerSession, section, value string) {
	parts := strings.Split(value, ":")
	if len(parts) < 2 {
		_ = respondPanel(s, i, textDisplay("Bilinmeyen islem."))
		return
	}
	switch parts[0] {
	case "user":
		b.handleOwnerUserAction(s, i, session, parts)
	case "blog":
		b.handleOwnerBlogAction(s, i, session, parts)
	case "ticket":
		b.handleOwnerTicketAction(s, i, session, parts)
	case "coupon":
		b.handleOwnerCouponAction(s, i, session, parts)
	default:
		_ = respondPanel(s, i, textDisplay("Bilinmeyen islem."))
	}
}

func (b *Bot) updateOwnerPanel(s *discordgo.Session, i *discordgo.InteractionCreate, session *ownerSession, notice string) {
	session.normalizeSection()
	body, actions := b.renderOwnerSection(session)
	if err := replyPanel(s, i, ownerPanelLayout(session, body, actions, notice)...); err != nil {
		log.Printf("Discord owner panel update failed: %v", err)
	}
}

func interactionUserID(i *discordgo.InteractionCreate) string {
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User.ID
	}
	if i.User != nil {
		return i.User.ID
	}
	return ""
}

func modalValue(data discordgo.ModalSubmitInteractionData, customID string) string {
	for _, row := range data.Components {
		var inputs []discordgo.MessageComponent
		switch typed := row.(type) {
		case *discordgo.ActionsRow:
			inputs = typed.Components
		case discordgo.ActionsRow:
			inputs = typed.Components
		default:
			continue
		}

		for _, component := range inputs {
			switch input := component.(type) {
			case *discordgo.TextInput:
				if input.CustomID == customID {
					return strings.TrimSpace(input.Value)
				}
			case discordgo.TextInput:
				if input.CustomID == customID {
					return strings.TrimSpace(input.Value)
				}
			}
		}
	}
	return ""
}

func describeRole(role int) string {
	switch {
	case role >= common.RoleRootUser:
		return "Kurucu"
	case role >= common.RoleAdminUser:
		return "Yönetici"
	case role >= common.RoleResellerUser:
		return "Bayi"
	case role >= common.RoleCommonUser:
		return "Kullanıcı"
	default:
		return "Misafir"
	}
}

func describeStatus(status int) string {
	if status == common.UserStatusEnabled {
		return "aktif"
	}
	return "devre dışı"
}

func describeBool(value bool) string {
	if value {
		return "açık"
	}
	return "kapalı"
}

func describeRedemptionStatus(status int) string {
	switch status {
	case 1:
		return "kullanılabilir"
	case 2:
		return "devre dışı"
	case 3:
		return "kullanılmış"
	default:
		return "bilinmiyor"
	}
}

func formatDuration(duration time.Duration) string {
	if duration < 0 {
		return "0 dk"
	}
	minutes := int(duration.Minutes())
	if minutes < 1 {
		return "1 dk'dan az"
	}
	if minutes < 60 {
		return fmt.Sprintf("%d dk", minutes)
	}
	return fmt.Sprintf("%d sa %d dk", minutes/60, minutes%60)
}
