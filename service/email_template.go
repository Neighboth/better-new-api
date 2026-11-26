package service

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

type EmailTemplate struct {
	Subject string
	Body    string
}

type cleanEmailSpec struct {
	Lang       string
	Title      string
	Greeting   string
	Paragraphs []string
	Code       string
	Details    [][2]string
	CTALabel   string
	CTAURL     string
	Footnote   string
	Closing    string
}

// buildCleanEmailHTML renders a minimal, modern, anti-spam responsive HTML email
// matching the clean solid-indigo card layout (periwinkle outer canvas, solid
// #4D62E5 header with site name only — no header icon, no avatar, no social
// icons, no gradients, no emojis, clean high-contrast typography).
func buildCleanEmailHTML(spec cleanEmailSpec) string {
	var b strings.Builder
	b.WriteString(`<!DOCTYPE html>
<html lang="` + spec.Lang + `">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<meta name="color-scheme" content="light">
<meta name="supported-color-schemes" content="light">
<title>` + spec.Title + `</title>
<style>
@media only screen and (max-width: 620px) {
  .email-outer { padding: 16px 10px !important; }
  .email-body { padding: 28px 22px !important; }
  .email-header { padding: 24px 20px !important; }
  .email-footer { padding: 18px 22px !important; }
}
</style>
</head>
<body style="margin:0;padding:0;background-color:#9EB7EE;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;color:#374151;-webkit-font-smoothing:antialiased;">
<table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" class="email-outer" style="background-color:#9EB7EE;padding:36px 16px;">
<tr>
<td align="center">
<table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="max-width:600px;background-color:#ffffff;border-radius:8px;overflow:hidden;">
<tr>
<td class="email-header" align="center" style="background-color:#4D62E5;padding:28px 32px;">
<span style="display:inline-block;color:#ffffff;font-size:21px;font-weight:600;line-height:28px;letter-spacing:-0.2px;">{{system_name}}</span>
</td>
</tr>
<tr>
<td class="email-body" style="padding:36px 40px 28px 40px;font-size:15px;line-height:24px;color:#374151;">
<p style="margin:0 0 18px 0;">` + spec.Greeting + `</p>
`)

	for _, p := range spec.Paragraphs {
		b.WriteString(`<p style="margin:0 0 16px 0;">` + p + `</p>` + "\n")
	}

	if spec.Code != "" {
		b.WriteString(`<table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="margin:22px 0;">
<tr>
<td align="center" style="background-color:#F3F5FD;border:1px solid #DCE3F9;border-radius:6px;padding:16px 20px;">
<span style="font-family:ui-monospace,SFMono-Regular,Menlo,Monaco,Consolas,monospace;font-size:26px;font-weight:700;letter-spacing:6px;color:#1F2937;">` + spec.Code + `</span>
</td>
</tr>
</table>
`)
	}

	if len(spec.Details) > 0 {
		b.WriteString(`<table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="margin:20px 0;background-color:#F8FAFC;border:1px solid #E5E7EB;border-radius:6px;">
`)
		for i, kv := range spec.Details {
			borderTop := ""
			if i > 0 {
				borderTop = "border-top:1px solid #E5E7EB;"
			}
			b.WriteString(`<tr>
<td style="padding:10px 14px;font-size:13px;color:#6B7280;width:36%;vertical-align:top;` + borderTop + `">` + kv[0] + `</td>
<td style="padding:10px 14px;font-size:13px;color:#111827;font-weight:600;word-break:break-word;` + borderTop + `">` + kv[1] + `</td>
</tr>
`)
		}
		b.WriteString("</table>\n")
	}

	if spec.CTALabel != "" && spec.CTAURL != "" {
		b.WriteString(`<table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="margin:26px 0 22px 0;">
<tr>
<td align="center">
<a href="` + spec.CTAURL + `" target="_blank" style="display:inline-block;background-color:#4D62E5;color:#ffffff;font-size:14px;font-weight:600;letter-spacing:0.4px;text-decoration:none;padding:14px 32px;border-radius:6px;">` + spec.CTALabel + `</a>
</td>
</tr>
</table>
`)
	}

	if spec.Footnote != "" {
		b.WriteString(`<p style="margin:0 0 18px 0;font-size:13px;line-height:20px;color:#6B7280;">` + spec.Footnote + `</p>` + "\n")
	}

	b.WriteString(`<p style="margin:20px 0 0 0;font-size:14px;line-height:22px;color:#374151;">` + spec.Closing + `</p>
</td>
</tr>
<tr>
<td class="email-footer" style="padding:18px 40px 26px 40px;border-top:1px solid #EEF2F7;font-size:12px;line-height:18px;color:#6B7280;">
{{system_name}} &middot; &copy; {{year}}
</td>
</tr>
</table>
</td>
</tr>
</table>
</body>
</html>`)
	return b.String()
}

var defaultEmailTemplates = buildAllDefaultEmailTemplates()

func buildAllDefaultEmailTemplates() map[string]map[string]EmailTemplate {
	return map[string]map[string]EmailTemplate{
		"tr": {
			"verification": {
				Subject: "{{system_name}} - E-posta doğrulama kodunuz",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "tr",
					Title:    "{{system_name}} - E-posta Doğrulama",
					Greeting: "Merhaba,",
					Paragraphs: []string{
						"<strong>{{system_name}}</strong> hesabınız için bir e-posta doğrulama isteği aldık. İşlemi tamamlamak için aşağıdaki kodu girin:",
						"<strong>Geçerlilik süresi:</strong> Bu kod {{valid_minutes}} dakika boyunca geçerlidir.",
					},
					Code:     "{{code}}",
					CTALabel: "DOĞRULAMA SAYFASINI AÇ",
					CTAURL:   "{{link}}",
					Footnote: "Bu isteği siz oluşturmadıysanız herhangi bir işlem yapmanız gerekmez.",
					Closing:  "Saygılarımızla,<br><strong>{{system_name}}</strong>",
				}),
			},
			"password_reset": {
				Subject: "{{system_name}} - Şifre sıfırlama bağlantısı",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "tr",
					Title:    "{{system_name}} - Şifre Sıfırlama",
					Greeting: "Merhaba,",
					Paragraphs: []string{
						"<strong>{{system_name}}</strong> hesabınız için şifre sıfırlama talebi alındı. Yeni şifrenizi belirlemek için aşağıdaki bağlantıyı kullanabilirsiniz.",
						"<strong>Geçerlilik süresi:</strong> Bu bağlantı güvenlik nedeniyle {{valid_minutes}} dakika içinde geçerliliğini yitirir.",
					},
					CTALabel: "ŞİFREMİ SIFIRLA",
					CTAURL:   "{{link}}",
					Footnote: "Bağlantı çalışmıyorsa tarayıcınıza yapıştırın: {{link}}<br>Bu talebi siz yapmadıysanız şifreniz değişmeden kalır.",
					Closing:  "Saygılarımızla,<br><strong>{{system_name}}</strong>",
				}),
			},
			"system_error": {
				Subject: "{{system_name}} - Sistem hatası bildirimi",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "tr",
					Title:    "{{system_name}} - Sistem Hatası",
					Greeting: "Merhaba,",
					Paragraphs: []string{
						"<strong>{{system_name}}</strong> üzerinde inceleme gerektiren bir sistem hatası kaydedildi.",
					},
					Details: [][2]string{
						{"Zaman", "{{time}}"},
						{"Hata", "{{error_message}}"},
					},
					Footnote: "Ayrıntılar için sunucu kayıtlarını ve yönetim panelini kontrol edin.",
					Closing:  "İyi çalışmalar,<br><strong>{{system_name}}</strong>",
				}),
			},
			"quota_warning": {
				Subject: "{{system_name}} - Bakiye uyarısı",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "tr",
					Title:    "{{system_name}} - Bakiye Uyarısı",
					Greeting: "Merhaba {{user_name}},",
					Paragraphs: []string{
						"<strong>{{system_name}}</strong> hesabınızdaki kalan bakiye belirlenen uyarı eşiğinin altına düştü.",
						"<strong>Kalan bakiye:</strong> {{quota_remaining}}. API isteklerinizin kesintiye uğramaması için cüzdanınızdan bakiye yükleyebilirsiniz.",
					},
					CTALabel: "BAKİYE YÜKLE",
					CTAURL:   "{{topup_link}}",
					Closing:  "Saygılarımızla,<br><strong>{{system_name}}</strong>",
				}),
			},
			"ticket_created": {
				Subject: "{{system_name}} - Yeni destek talebi #{{ticket_id}}",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "tr",
					Title:    "{{system_name}} - Yeni Destek Talebi",
					Greeting: "Merhaba,",
					Paragraphs: []string{
						"Yeni bir destek talebi oluşturuldu. Özet bilgiler aşağıdadır:",
					},
					Details: [][2]string{
						{"Talep No", "#{{ticket_id}}"},
						{"Konu", "{{ticket_title}}"},
						{"Kullanıcı", "{{user_name}} ({{user_email}})"},
						{"Kategori / Öncelik", "{{category}} / {{priority}}"},
						{"Mesaj", "{{message}}"},
					},
					Closing: "İyi çalışmalar,<br><strong>{{system_name}}</strong>",
				}),
			},
			"ticket_replied": {
				Subject: "{{system_name}} - Destek talebinize yanıt verildi (#{{ticket_id}})",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "tr",
					Title:    "{{system_name}} - Destek Yanıtı",
					Greeting: "Merhaba,",
					Paragraphs: []string{
						"<strong>#{{ticket_id}} — {{ticket_title}}</strong> numaralı destek talebinize yeni bir yanıt eklendi:",
					},
					Details: [][2]string{
						{"Yanıt", "{{reply_message}}"},
					},
					CTALabel: "TALEBİ GÖRÜNTÜLE",
					CTAURL:   "{{dashboard_link}}",
					Closing:  "Saygılarımızla,<br><strong>{{system_name}}</strong>",
				}),
			},
			"channel_disabled": {
				Subject: "{{system_name}} - Kanal devre dışı bırakıldı (#{{channel_id}})",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "tr",
					Title:    "{{system_name}} - Kanal Devre Dışı",
					Greeting: "Merhaba,",
					Paragraphs: []string{
						"Bir upstream API kanalı otomatik sağlık kontrolü nedeniyle devre dışı bırakıldı.",
					},
					Details: [][2]string{
						{"Kanal", "{{channel_name}} (#{{channel_id}})"},
						{"Neden", "{{reason}}"},
						{"Zaman", "{{time}}"},
					},
					Footnote: "Kanal ayarlarını ve API anahtarını yönetim panelinden kontrol edebilirsiniz.",
					Closing:  "İyi çalışmalar,<br><strong>{{system_name}}</strong>",
				}),
			},
			"video_expiry": {
				Subject: "{{system_name}} - Video dosyalarınız yakında silinecek",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "tr",
					Title:    "{{system_name}} - Video Süre Hatırlatması",
					Greeting: "Merhaba {{username}},",
					Paragraphs: []string{
						"Sunucuda saklanan video dosyanızın saklama süresi <strong>{{expires_at}}</strong> tarihinde sona erecek ve otomatik olarak silinecektir.",
						"Videoyu saklamak istiyorsanız belirtilen tarihten önce cihazınıza indirebilirsiniz.",
					},
					CTALabel: "VİDEOYU AÇ",
					CTAURL:   "{{video_url}}",
					Closing:  "Saygılarımızla,<br><strong>{{system_name}}</strong>",
				}),
			},
			"fine_tuning_expiry": {
				Subject: "{{system_name}} - İnce ayarlı modeliniz {{days_left}} gün içinde silinecek",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "tr",
					Title:    "{{system_name}} - İnce Ayar Model Hatırlatması",
					Greeting: "Merhaba {{username}},",
					Paragraphs: []string{
						"<strong>{{model}}</strong> ince ayarlı modelinizin sunucu saklama süresi <strong>{{days_left}} gün</strong> içinde (<strong>{{expires_at}}</strong>) dolacaktır.",
						"Model ağırlıklarını, kontrol noktalarını ve eğitim kayıtlarını kaybetmemek için son tarihten önce .zip arşivini indirin.",
					},
					Details: [][2]string{
						{"Model", "{{model}}"},
						{"İş ID", "{{job_id}}"},
						{"Son Tarih", "{{expires_at}}"},
					},
					CTALabel: "MODELİ İNDİR",
					CTAURL:   "{{download_url}}",
					Closing:  "Saygılarımızla,<br><strong>{{system_name}}</strong>",
				}),
			},
		},
		"en": {
			"verification": {
				Subject: "{{system_name}} - Your verification code",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "en",
					Title:    "{{system_name}} - Email Verification",
					Greeting: "Hi,",
					Paragraphs: []string{
						"We received a request to verify your email address for <strong>{{system_name}}</strong>. Enter the code below to continue:",
						"<strong>Expiration:</strong> This code remains valid for {{valid_minutes}} minutes.",
					},
					Code:     "{{code}}",
					CTALabel: "VERIFY EMAIL",
					CTAURL:   "{{link}}",
					Footnote: "If you did not request this verification, you can safely ignore this message.",
					Closing:  "Best regards,<br><strong>{{system_name}}</strong>",
				}),
			},
			"password_reset": {
				Subject: "{{system_name}} - Password reset request",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "en",
					Title:    "{{system_name}} - Password Reset",
					Greeting: "Hi,",
					Paragraphs: []string{
						"A password reset was requested for your <strong>{{system_name}}</strong> account. Use the button below to choose a new password.",
						"<strong>Expiration:</strong> For your security, this link expires in {{valid_minutes}} minutes.",
					},
					CTALabel: "RESET PASSWORD",
					CTAURL:   "{{link}}",
					Footnote: "If the button does not open, paste this link into your browser: {{link}}<br>If you did not request a reset, your password has not been changed.",
					Closing:  "Best regards,<br><strong>{{system_name}}</strong>",
				}),
			},
			"system_error": {
				Subject: "{{system_name}} - System error alert",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "en",
					Title:    "{{system_name}} - System Error",
					Greeting: "Hi,",
					Paragraphs: []string{
						"A system error was recorded on <strong>{{system_name}}</strong> and requires review.",
					},
					Details: [][2]string{
						{"Time", "{{time}}"},
						{"Error", "{{error_message}}"},
					},
					Footnote: "Check server logs and the admin dashboard for full diagnostic details.",
					Closing:  "Regards,<br><strong>{{system_name}}</strong>",
				}),
			},
			"quota_warning": {
				Subject: "{{system_name}} - Low balance notice",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "en",
					Title:    "{{system_name}} - Quota Warning",
					Greeting: "Hi {{user_name}},",
					Paragraphs: []string{
						"Your remaining balance on <strong>{{system_name}}</strong> has dropped below your alert threshold.",
						"<strong>Remaining balance:</strong> {{quota_remaining}}. Add funds to keep your API requests running without interruption.",
					},
					CTALabel: "ADD BALANCE",
					CTAURL:   "{{topup_link}}",
					Closing:  "Best regards,<br><strong>{{system_name}}</strong>",
				}),
			},
			"ticket_created": {
				Subject: "{{system_name}} - New support ticket #{{ticket_id}}",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "en",
					Title:    "{{system_name}} - New Support Ticket",
					Greeting: "Hi,",
					Paragraphs: []string{
						"A new support ticket has been submitted with the following details:",
					},
					Details: [][2]string{
						{"Ticket", "#{{ticket_id}}"},
						{"Subject", "{{ticket_title}}"},
						{"User", "{{user_name}} ({{user_email}})"},
						{"Category / Priority", "{{category}} / {{priority}}"},
						{"Message", "{{message}}"},
					},
					Closing: "Regards,<br><strong>{{system_name}}</strong>",
				}),
			},
			"ticket_replied": {
				Subject: "{{system_name}} - New reply on ticket #{{ticket_id}}",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "en",
					Title:    "{{system_name}} - Ticket Reply",
					Greeting: "Hi,",
					Paragraphs: []string{
						"Our team posted a new reply to your support ticket <strong>#{{ticket_id}} — {{ticket_title}}</strong>:",
					},
					Details: [][2]string{
						{"Reply", "{{reply_message}}"},
					},
					CTALabel: "VIEW TICKET",
					CTAURL:   "{{dashboard_link}}",
					Closing:  "Best regards,<br><strong>{{system_name}}</strong>",
				}),
			},
			"channel_disabled": {
				Subject: "{{system_name}} - Channel disabled (#{{channel_id}})",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "en",
					Title:    "{{system_name}} - Channel Disabled",
					Greeting: "Hi,",
					Paragraphs: []string{
						"An upstream API channel was automatically disabled after failing health checks.",
					},
					Details: [][2]string{
						{"Channel", "{{channel_name}} (#{{channel_id}})"},
						{"Reason", "{{reason}}"},
						{"Time", "{{time}}"},
					},
					Footnote: "Review channel credentials and upstream status in the admin panel.",
					Closing:  "Regards,<br><strong>{{system_name}}</strong>",
				}),
			},
			"video_expiry": {
				Subject: "{{system_name}} - Your stored video will expire soon",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "en",
					Title:    "{{system_name}} - Video Expiry Reminder",
					Greeting: "Hi {{username}},",
					Paragraphs: []string{
						"A video stored in your account is scheduled for automatic deletion on <strong>{{expires_at}}</strong>.",
						"If you want to keep a copy, please download it before the expiration date.",
					},
					CTALabel: "OPEN VIDEO",
					CTAURL:   "{{video_url}}",
					Closing:  "Best regards,<br><strong>{{system_name}}</strong>",
				}),
			},
			"fine_tuning_expiry": {
				Subject: "{{system_name}} - Fine-tuned model expires in {{days_left}} days",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "en",
					Title:    "{{system_name}} - Fine-Tuning Expiry Reminder",
					Greeting: "Hi {{username}},",
					Paragraphs: []string{
						"Your fine-tuned model <strong>{{model}}</strong> will be removed from server storage in <strong>{{days_left}} days</strong> (<strong>{{expires_at}}</strong>).",
						"Download the .zip archive before expiration to keep your trained weights, checkpoints, and training logs.",
					},
					Details: [][2]string{
						{"Model", "{{model}}"},
						{"Job ID", "{{job_id}}"},
						{"Expires On", "{{expires_at}}"},
					},
					CTALabel: "DOWNLOAD MODEL",
					CTAURL:   "{{download_url}}",
					Closing:  "Best regards,<br><strong>{{system_name}}</strong>",
				}),
			},
		},
		"zh_CN": {
			"verification": {
				Subject: "{{system_name}} - 邮箱验证码",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "zh-CN",
					Title:    "{{system_name}} - 邮箱验证",
					Greeting: "您好，",
					Paragraphs: []string{
						"我们收到了您的 <strong>{{system_name}}</strong> 邮箱验证请求。请输入以下验证码完成验证：",
						"<strong>有效期：</strong>该验证码在 {{valid_minutes}} 分钟内有效。",
					},
					Code:     "{{code}}",
					CTALabel: "打开验证页面",
					CTAURL:   "{{link}}",
					Footnote: "如果这不是您本人的操作，请忽略此邮件。",
					Closing:  "此致，<br><strong>{{system_name}}</strong>",
				}),
			},
			"password_reset": {
				Subject: "{{system_name}} - 密码重置请求",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "zh-CN",
					Title:    "{{system_name}} - 密码重置",
					Greeting: "您好，",
					Paragraphs: []string{
						"我们收到了您的 <strong>{{system_name}}</strong> 账户密码重置请求。请点击下方按钮设置新密码。",
						"<strong>有效期：</strong>为了您的账户安全，该链接将在 {{valid_minutes}} 分钟后失效。",
					},
					CTALabel: "重置密码",
					CTAURL:   "{{link}}",
					Footnote: "如果按钮无法打开，请复制以下链接到浏览器：{{link}}<br>如非本人操作，您的密码不会发生任何更改。",
					Closing:  "此致，<br><strong>{{system_name}}</strong>",
				}),
			},
			"system_error": {
				Subject: "{{system_name}} - 系统异常通知",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "zh-CN",
					Title:    "{{system_name}} - 系统异常",
					Greeting: "您好，",
					Paragraphs: []string{
						"<strong>{{system_name}}</strong> 检测到需要处理的系统错误。",
					},
					Details: [][2]string{
						{"时间", "{{time}}"},
						{"错误信息", "{{error_message}}"},
					},
					Footnote: "请前往管理后台和服务端日志查看详细信息。",
					Closing:  "此致，<br><strong>{{system_name}}</strong>",
				}),
			},
			"quota_warning": {
				Subject: "{{system_name}} - 余额不足提醒",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "zh-CN",
					Title:    "{{system_name}} - 余额提醒",
					Greeting: "您好 {{user_name}}，",
					Paragraphs: []string{
						"您的 <strong>{{system_name}}</strong> 账户剩余额度已低于预警阈值。",
						"<strong>当前剩余额度：</strong>{{quota_remaining}}。为避免 API 请求中断，请及时充值。",
					},
					CTALabel: "前往充值",
					CTAURL:   "{{topup_link}}",
					Closing:  "此致，<br><strong>{{system_name}}</strong>",
				}),
			},
			"ticket_created": {
				Subject: "{{system_name}} - 新工单通知 #{{ticket_id}}",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "zh-CN",
					Title:    "{{system_name}} - 新工单",
					Greeting: "您好，",
					Paragraphs: []string{
						"收到一条新的支持工单，详情如下：",
					},
					Details: [][2]string{
						{"工单编号", "#{{ticket_id}}"},
						{"主题", "{{ticket_title}}"},
						{"用户", "{{user_name}} ({{user_email}})"},
						{"分类 / 优先级", "{{category}} / {{priority}}"},
						{"内容", "{{message}}"},
					},
					Closing: "此致，<br><strong>{{system_name}}</strong>",
				}),
			},
			"ticket_replied": {
				Subject: "{{system_name}} - 工单 #{{ticket_id}} 有新回复",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "zh-CN",
					Title:    "{{system_name}} - 工单回复",
					Greeting: "您好，",
					Paragraphs: []string{
						"您的工单 <strong>#{{ticket_id}} — {{ticket_title}}</strong> 收到了新回复：",
					},
					Details: [][2]string{
						{"回复内容", "{{reply_message}}"},
					},
					CTALabel: "查看工单",
					CTAURL:   "{{dashboard_link}}",
					Closing:  "此致，<br><strong>{{system_name}}</strong>",
				}),
			},
			"channel_disabled": {
				Subject: "{{system_name}} - 渠道已自动禁用 (#{{channel_id}})",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "zh-CN",
					Title:    "{{system_name}} - 渠道禁用通知",
					Greeting: "您好，",
					Paragraphs: []string{
						"检测到上游 API 渠道异常，系统已自动将其禁用。",
					},
					Details: [][2]string{
						{"渠道", "{{channel_name}} (#{{channel_id}})"},
						{"原因", "{{reason}}"},
						{"时间", "{{time}}"},
					},
					Footnote: "请在管理后台检查该渠道的密钥与上游状态。",
					Closing:  "此致，<br><strong>{{system_name}}</strong>",
				}),
			},
			"video_expiry": {
				Subject: "{{system_name}} - 视频文件即将过期",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "zh-CN",
					Title:    "{{system_name}} - 视频过期提醒",
					Greeting: "您好 {{username}}，",
					Paragraphs: []string{
						"您存储在服务器上的视频文件将于 <strong>{{expires_at}}</strong> 自动清理。",
						"如需保留，请在到期前下载到本地。",
					},
					CTALabel: "查看视频",
					CTAURL:   "{{video_url}}",
					Closing:  "此致，<br><strong>{{system_name}}</strong>",
				}),
			},
			"fine_tuning_expiry": {
				Subject: "{{system_name}} - 微调模型将在 {{days_left}} 天后清理",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "zh-CN",
					Title:    "{{system_name}} - 微调模型到期提醒",
					Greeting: "您好 {{username}}，",
					Paragraphs: []string{
						"您的微调模型 <strong>{{model}}</strong> 将在 <strong>{{days_left}} 天</strong> 后（<strong>{{expires_at}}</strong>）从服务器存储中自动删除。",
						"请在此之前下载 .zip 压缩包以保存权重、检查点和训练日志。",
					},
					Details: [][2]string{
						{"模型", "{{model}}"},
						{"任务 ID", "{{job_id}}"},
						{"到期日期", "{{expires_at}}"},
					},
					CTALabel: "下载模型",
					CTAURL:   "{{download_url}}",
					Closing:  "此致，<br><strong>{{system_name}}</strong>",
				}),
			},
		},
		"zh_TW": {
			"verification": {
				Subject: "{{system_name}} - 電子郵件驗證碼",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "zh-TW",
					Title:    "{{system_name}} - 電子郵件驗證",
					Greeting: "您好，",
					Paragraphs: []string{
						"我們收到了您的 <strong>{{system_name}}</strong> 電子郵件驗證請求。請輸入以下驗證碼完成驗證：",
						"<strong>有效期限：</strong>此驗證碼在 {{valid_minutes}} 分鐘內有效。",
					},
					Code:     "{{code}}",
					CTALabel: "開啟驗證頁面",
					CTAURL:   "{{link}}",
					Footnote: "若這並非您本人的操作，請放心忽略此郵件。",
					Closing:  "敬上，<br><strong>{{system_name}}</strong>",
				}),
			},
			"password_reset": {
				Subject: "{{system_name}} - 密碼重設請求",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "zh-TW",
					Title:    "{{system_name}} - 密碼重設",
					Greeting: "您好，",
					Paragraphs: []string{
						"我們收到了您的 <strong>{{system_name}}</strong> 帳戶密碼重設請求。請點擊下方按鈕設定新密碼。",
						"<strong>有效期限：</strong>為了保護您的帳戶安全，此連結將於 {{valid_minutes}} 分鐘後失效。",
					},
					CTALabel: "重設密碼",
					CTAURL:   "{{link}}",
					Footnote: "若按鈕無法開啟，請複製以下連結至瀏覽器：{{link}}<br>如非本人操作，您的密碼不會有任何變更。",
					Closing:  "敬上，<br><strong>{{system_name}}</strong>",
				}),
			},
			"system_error": {
				Subject: "{{system_name}} - 系統錯誤通知",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "zh-TW",
					Title:    "{{system_name}} - 系統錯誤",
					Greeting: "您好，",
					Paragraphs: []string{
						"<strong>{{system_name}}</strong> 偵測到需要檢查的系統錯誤。",
					},
					Details: [][2]string{
						{"時間", "{{time}}"},
						{"錯誤訊息", "{{error_message}}"},
					},
					Footnote: "請查看伺服器日誌與管理後台以取得完整資訊。",
					Closing:  "敬上，<br><strong>{{system_name}}</strong>",
				}),
			},
			"quota_warning": {
				Subject: "{{system_name}} - 餘額不足提醒",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "zh-TW",
					Title:    "{{system_name}} - 餘額提醒",
					Greeting: "您好 {{user_name}}，",
					Paragraphs: []string{
						"您的 <strong>{{system_name}}</strong> 帳戶剩餘額度已低於提醒門檻。",
						"<strong>目前剩餘額度：</strong>{{quota_remaining}}。為避免 API 請求中斷，請儘速儲值。",
					},
					CTALabel: "立即儲值",
					CTAURL:   "{{topup_link}}",
					Closing:  "敬上，<br><strong>{{system_name}}</strong>",
				}),
			},
			"ticket_created": {
				Subject: "{{system_name}} - 新客服工單 #{{ticket_id}}",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "zh-TW",
					Title:    "{{system_name}} - 新工單通知",
					Greeting: "您好，",
					Paragraphs: []string{
						"系統收到一筆新的客服工單，詳情如下：",
					},
					Details: [][2]string{
						{"工單編號", "#{{ticket_id}}"},
						{"主旨", "{{ticket_title}}"},
						{"使用者", "{{user_name}} ({{user_email}})"},
						{"分類 / 優先級", "{{category}} / {{priority}}"},
						{"內容", "{{message}}"},
					},
					Closing: "敬上，<br><strong>{{system_name}}</strong>",
				}),
			},
			"ticket_replied": {
				Subject: "{{system_name}} - 工單 #{{ticket_id}} 有新回覆",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "zh-TW",
					Title:    "{{system_name}} - 工單回覆",
					Greeting: "您好，",
					Paragraphs: []string{
						"您的工單 <strong>#{{ticket_id}} — {{ticket_title}}</strong> 收到了新回覆：",
					},
					Details: [][2]string{
						{"回覆內容", "{{reply_message}}"},
					},
					CTALabel: "檢視工單",
					CTAURL:   "{{dashboard_link}}",
					Closing:  "敬上，<br><strong>{{system_name}}</strong>",
				}),
			},
			"channel_disabled": {
				Subject: "{{system_name}} - 管道已停用 (#{{channel_id}})",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "zh-TW",
					Title:    "{{system_name}} - 管道停用通知",
					Greeting: "您好，",
					Paragraphs: []string{
						"上游 API 管道因健康檢查失敗已被自動停用。",
					},
					Details: [][2]string{
						{"管道", "{{channel_name}} (#{{channel_id}})"},
						{"原因", "{{reason}}"},
						{"時間", "{{time}}"},
					},
					Footnote: "請至管理後台確認管道金鑰與上游狀態。",
					Closing:  "敬上，<br><strong>{{system_name}}</strong>",
				}),
			},
			"video_expiry": {
				Subject: "{{system_name}} - 影片檔案即將到期",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "zh-TW",
					Title:    "{{system_name}} - 影片到期提醒",
					Greeting: "您好 {{username}}，",
					Paragraphs: []string{
						"您儲存於伺服器上的影片檔案將於 <strong>{{expires_at}}</strong> 自動刪除。",
						"若需保留檔案，請於到期日前下載保存。",
					},
					CTALabel: "開啟影片",
					CTAURL:   "{{video_url}}",
					Closing:  "敬上，<br><strong>{{system_name}}</strong>",
				}),
			},
			"fine_tuning_expiry": {
				Subject: "{{system_name}} - 微調模型將於 {{days_left}} 天後刪除",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "zh-TW",
					Title:    "{{system_name}} - 微調模型到期提醒",
					Greeting: "您好 {{username}}，",
					Paragraphs: []string{
						"您的微調模型 <strong>{{model}}</strong> 將於 <strong>{{days_left}} 天</strong> 內（<strong>{{expires_at}}</strong>）自伺服器儲存空間自動移除。",
						"請於到期前下載 .zip 封存檔以保留訓練權重、檢查點與日誌。",
					},
					Details: [][2]string{
						{"模型", "{{model}}"},
						{"任務 ID", "{{job_id}}"},
						{"到期日期", "{{expires_at}}"},
					},
					CTALabel: "下載模型",
					CTAURL:   "{{download_url}}",
					Closing:  "敬上，<br><strong>{{system_name}}</strong>",
				}),
			},
		},
		"fr": {
			"verification": {
				Subject: "{{system_name}} - Votre code de vérification",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "fr",
					Title:    "{{system_name}} - Vérification d'e-mail",
					Greeting: "Bonjour,",
					Paragraphs: []string{
						"Nous avons reçu une demande de vérification d'adresse e-mail pour votre compte <strong>{{system_name}}</strong>. Saisissez le code ci-dessous pour continuer :",
						"<strong>Validité :</strong> Ce code expire dans {{valid_minutes}} minutes.",
					},
					Code:     "{{code}}",
					CTALabel: "VÉRIFIER L'E-MAIL",
					CTAURL:   "{{link}}",
					Footnote: "Si vous n'êtes pas à l'origine de cette demande, vous pouvez ignorer cet e-mail.",
					Closing:  "Cordialement,<br><strong>{{system_name}}</strong>",
				}),
			},
			"password_reset": {
				Subject: "{{system_name}} - Réinitialisation de mot de passe",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "fr",
					Title:    "{{system_name}} - Réinitialisation du mot de passe",
					Greeting: "Bonjour,",
					Paragraphs: []string{
						"Une réinitialisation de mot de passe a été demandée pour votre compte <strong>{{system_name}}</strong>. Cliquez ci-dessous pour définir un nouveau mot de passe.",
						"<strong>Validité :</strong> Ce lien expire dans {{valid_minutes}} minutes.",
					},
					CTALabel: "RÉINITIALISER LE MOT DE PASSE",
					CTAURL:   "{{link}}",
					Footnote: "Si le bouton ne fonctionne pas, copiez ce lien dans votre navigateur : {{link}}<br>Si vous n'avez pas fait cette demande, votre mot de passe reste inchangé.",
					Closing:  "Cordialement,<br><strong>{{system_name}}</strong>",
				}),
			},
			"system_error": {
				Subject: "{{system_name}} - Alerte d'erreur système",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "fr",
					Title:    "{{system_name}} - Erreur système",
					Greeting: "Bonjour,",
					Paragraphs: []string{
						"Une erreur système a été enregistrée sur <strong>{{system_name}}</strong> et nécessite une vérification.",
					},
					Details: [][2]string{
						{"Heure", "{{time}}"},
						{"Erreur", "{{error_message}}"},
					},
					Footnote: "Consultez les journaux du serveur et le tableau de bord d'administration pour plus de détails.",
					Closing:  "Cordialement,<br><strong>{{system_name}}</strong>",
				}),
			},
			"quota_warning": {
				Subject: "{{system_name}} - Alerte de solde faible",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "fr",
					Title:    "{{system_name}} - Alerte de quota",
					Greeting: "Bonjour {{user_name}},",
					Paragraphs: []string{
						"Le solde restant sur votre compte <strong>{{system_name}}</strong> est passé sous le seuil d'alerte.",
						"<strong>Solde restant :</strong> {{quota_remaining}}. Rechargez votre compte pour éviter toute interruption de service.",
					},
					CTALabel: "RECHARGER MON SOLDE",
					CTAURL:   "{{topup_link}}",
					Closing:  "Cordialement,<br><strong>{{system_name}}</strong>",
				}),
			},
			"ticket_created": {
				Subject: "{{system_name}} - Nouveau ticket #{{ticket_id}}",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "fr",
					Title:    "{{system_name}} - Nouveau ticket",
					Greeting: "Bonjour,",
					Paragraphs: []string{
						"Un nouveau ticket d'assistance a été créé :",
					},
					Details: [][2]string{
						{"Ticket", "#{{ticket_id}}"},
						{"Sujet", "{{ticket_title}}"},
						{"Utilisateur", "{{user_name}} ({{user_email}})"},
						{"Catégorie / Priorité", "{{category}} / {{priority}}"},
						{"Message", "{{message}}"},
					},
					Closing: "Cordialement,<br><strong>{{system_name}}</strong>",
				}),
			},
			"ticket_replied": {
				Subject: "{{system_name}} - Nouvelle réponse au ticket #{{ticket_id}}",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "fr",
					Title:    "{{system_name}} - Réponse au ticket",
					Greeting: "Bonjour,",
					Paragraphs: []string{
						"Une nouvelle réponse a été ajoutée à votre ticket <strong>#{{ticket_id}} — {{ticket_title}}</strong> :",
					},
					Details: [][2]string{
						{"Réponse", "{{reply_message}}"},
					},
					CTALabel: "VOIR LE TICKET",
					CTAURL:   "{{dashboard_link}}",
					Closing:  "Cordialement,<br><strong>{{system_name}}</strong>",
				}),
			},
			"channel_disabled": {
				Subject: "{{system_name}} - Canal désactivé (#{{channel_id}})",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "fr",
					Title:    "{{system_name}} - Canal désactivé",
					Greeting: "Bonjour,",
					Paragraphs: []string{
						"Un canal API amont a été désactivé automatiquement suite à un échec de vérification.",
					},
					Details: [][2]string{
						{"Canal", "{{channel_name}} (#{{channel_id}})"},
						{"Motif", "{{reason}}"},
						{"Heure", "{{time}}"},
					},
					Footnote: "Vérifiez la configuration et la clé du canal dans le panneau d'administration.",
					Closing:  "Cordialement,<br><strong>{{system_name}}</strong>",
				}),
			},
			"video_expiry": {
				Subject: "{{system_name}} - Votre vidéo expire bientôt",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "fr",
					Title:    "{{system_name}} - Rappel d'expiration vidéo",
					Greeting: "Bonjour {{username}},",
					Paragraphs: []string{
						"Une vidéo stockée sur votre compte sera supprimée automatiquement le <strong>{{expires_at}}</strong>.",
						"Téléchargez-la avant cette date si vous souhaitez en conserver une copie.",
					},
					CTALabel: "OUVRIR LA VIDÉO",
					CTAURL:   "{{video_url}}",
					Closing:  "Cordialement,<br><strong>{{system_name}}</strong>",
				}),
			},
			"fine_tuning_expiry": {
				Subject: "{{system_name}} - Votre modèle affiné expire dans {{days_left}} jours",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "fr",
					Title:    "{{system_name}} - Expiration du modèle affiné",
					Greeting: "Bonjour {{username}},",
					Paragraphs: []string{
						"Votre modèle affiné <strong>{{model}}</strong> sera supprimé du stockage serveur dans <strong>{{days_left}} jours</strong> (<strong>{{expires_at}}</strong>).",
						"Téléchargez l'archive .zip avant expiration pour conserver vos poids, points de contrôle et journaux.",
					},
					Details: [][2]string{
						{"Modèle", "{{model}}"},
						{"ID de tâche", "{{job_id}}"},
						{"Expiration", "{{expires_at}}"},
					},
					CTALabel: "TÉLÉCHARGER LE MODÈLE",
					CTAURL:   "{{download_url}}",
					Closing:  "Cordialement,<br><strong>{{system_name}}</strong>",
				}),
			},
		},
		"ru": {
			"verification": {
				Subject: "{{system_name}} - Код подтверждения почты",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "ru",
					Title:    "{{system_name}} - Подтверждение почты",
					Greeting: "Здравствуйте,",
					Paragraphs: []string{
						"Мы получили запрос на подтверждение адреса электронной почты для <strong>{{system_name}}</strong>. Введите код ниже:",
						"<strong>Срок действия:</strong> код действителен в течение {{valid_minutes}} минут.",
					},
					Code:     "{{code}}",
					CTALabel: "ПОДТВЕРДИТЬ ПОЧТУ",
					CTAURL:   "{{link}}",
					Footnote: "Если вы не отправляли этот запрос, просто проигнорируйте письмо.",
					Closing:  "С уважением,<br><strong>{{system_name}}</strong>",
				}),
			},
			"password_reset": {
				Subject: "{{system_name}} - Сброс пароля",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "ru",
					Title:    "{{system_name}} - Сброс пароля",
					Greeting: "Здравствуйте,",
					Paragraphs: []string{
						"Получен запрос на сброс пароля для вашей учетной записи <strong>{{system_name}}</strong>. Нажмите кнопку ниже, чтобы задать новый пароль.",
						"<strong>Срок действия:</strong> ссылка действительна {{valid_minutes}} минут.",
					},
					CTALabel: "СБРОСИТЬ ПАРОЛЬ",
					CTAURL:   "{{link}}",
					Footnote: "Если кнопка не открывается, вставьте ссылку в браузер: {{link}}<br>Если вы не запрашивали сброс, пароль не изменен.",
					Closing:  "С уважением,<br><strong>{{system_name}}</strong>",
				}),
			},
			"system_error": {
				Subject: "{{system_name}} - Уведомление о системной ошибке",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "ru",
					Title:    "{{system_name}} - Системная ошибка",
					Greeting: "Здравствуйте,",
					Paragraphs: []string{
						"На платформе <strong>{{system_name}}</strong> зафиксирована системная ошибка, требующая проверки.",
					},
					Details: [][2]string{
						{"Время", "{{time}}"},
						{"Ошибка", "{{error_message}}"},
					},
					Footnote: "Проверьте журналы сервера и панель администратора для диагностики.",
					Closing:  "С уважением,<br><strong>{{system_name}}</strong>",
				}),
			},
			"quota_warning": {
				Subject: "{{system_name}} - Низкий остаток баланса",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "ru",
					Title:    "{{system_name}} - Предупреждение о балансе",
					Greeting: "Здравствуйте, {{user_name}},",
					Paragraphs: []string{
						"Остаток баланса в вашем аккаунте <strong>{{system_name}}</strong> опустился ниже заданного порога.",
						"<strong>Текущий остаток:</strong> {{quota_remaining}}. Пополните баланс, чтобы избежать перебоев в работе API.",
					},
					CTALabel: "ПОПОЛНИТЬ БАЛАНС",
					CTAURL:   "{{topup_link}}",
					Closing:  "С уважением,<br><strong>{{system_name}}</strong>",
				}),
			},
			"ticket_created": {
				Subject: "{{system_name}} - Новое обращение #{{ticket_id}}",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "ru",
					Title:    "{{system_name}} - Новое обращение",
					Greeting: "Здравствуйте,",
					Paragraphs: []string{
						"Создано новое обращение в поддержку:",
					},
					Details: [][2]string{
						{"Обращение", "#{{ticket_id}}"},
						{"Тема", "{{ticket_title}}"},
						{"Пользователь", "{{user_name}} ({{user_email}})"},
						{"Категория / Приоритет", "{{category}} / {{priority}}"},
						{"Сообщение", "{{message}}"},
					},
					Closing: "С уважением,<br><strong>{{system_name}}</strong>",
				}),
			},
			"ticket_replied": {
				Subject: "{{system_name}} - Новый ответ по обращению #{{ticket_id}}",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "ru",
					Title:    "{{system_name}} - Ответ поддержки",
					Greeting: "Здравствуйте,",
					Paragraphs: []string{
						"По вашему обращению <strong>#{{ticket_id}} — {{ticket_title}}</strong> получен новый ответ:",
					},
					Details: [][2]string{
						{"Ответ", "{{reply_message}}"},
					},
					CTALabel: "ОТКРЫТЬ ОБРАЩЕНИЕ",
					CTAURL:   "{{dashboard_link}}",
					Closing:  "С уважением,<br><strong>{{system_name}}</strong>",
				}),
			},
			"channel_disabled": {
				Subject: "{{system_name}} - Канал отключен (#{{channel_id}})",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "ru",
					Title:    "{{system_name}} - Канал отключен",
					Greeting: "Здравствуйте,",
					Paragraphs: []string{
						"Внешний API-канал был автоматически отключен по результатам проверки состояния.",
					},
					Details: [][2]string{
						{"Канал", "{{channel_name}} (#{{channel_id}})"},
						{"Причина", "{{reason}}"},
						{"Время", "{{time}}"},
					},
					Footnote: "Проверьте настройки канала и API-ключ в панели администратора.",
					Closing:  "С уважением,<br><strong>{{system_name}}</strong>",
				}),
			},
			"video_expiry": {
				Subject: "{{system_name}} - Срок хранения видео скоро истекает",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "ru",
					Title:    "{{system_name}} - Напоминание о видео",
					Greeting: "Здравствуйте, {{username}},",
					Paragraphs: []string{
						"Видеофайл на сервере будет автоматически удален <strong>{{expires_at}}</strong>.",
						"Если вы хотите сохранить его, скачайте файл до указанной даты.",
					},
					CTALabel: "ОТКРЫТЬ ВИДЕО",
					CTAURL:   "{{video_url}}",
					Closing:  "С уважением,<br><strong>{{system_name}}</strong>",
				}),
			},
			"fine_tuning_expiry": {
				Subject: "{{system_name}} - Дообученная модель будет удалена через {{days_left}} дн.",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "ru",
					Title:    "{{system_name}} - Срок хранения модели",
					Greeting: "Здравствуйте, {{username}},",
					Paragraphs: []string{
						"Срок хранения дообученной модели <strong>{{model}}</strong> истекает через <strong>{{days_left}} дн.</strong> (<strong>{{expires_at}}</strong>).",
						"Скачайте архив .zip заранее, чтобы сохранить веса, чекпоинты и журнал обучения.",
					},
					Details: [][2]string{
						{"Модель", "{{model}}"},
						{"ID задачи", "{{job_id}}"},
						{"Дата удаления", "{{expires_at}}"},
					},
					CTALabel: "СКАЧАТЬ МОДЕЛЬ",
					CTAURL:   "{{download_url}}",
					Closing:  "С уважением,<br><strong>{{system_name}}</strong>",
				}),
			},
		},
		"ja": {
			"verification": {
				Subject: "{{system_name}} - メール認証コード",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "ja",
					Title:    "{{system_name}} - メール認証",
					Greeting: "こんにちは、",
					Paragraphs: []string{
						"<strong>{{system_name}}</strong> のメールアドレス認証リクエストを受け付けました。以下のコードを入力してください。",
						"<strong>有効期限：</strong>このコードは {{valid_minutes}} 分間有効です。",
					},
					Code:     "{{code}}",
					CTALabel: "認証ページを開く",
					CTAURL:   "{{link}}",
					Footnote: "心当たりがない場合は、このメールを破棄してください。",
					Closing:  "よろしくお願いいたします。<br><strong>{{system_name}}</strong>",
				}),
			},
			"password_reset": {
				Subject: "{{system_name}} - パスワード再設定のご案内",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "ja",
					Title:    "{{system_name}} - パスワード再設定",
					Greeting: "こんにちは、",
					Paragraphs: []string{
						"<strong>{{system_name}}</strong> アカウントのパスワード再設定がリクエストされました。下のボタンから新しいパスワードを設定できます。",
						"<strong>有効期限：</strong>セキュリティ保護のため、このリンクは {{valid_minutes}} 分後に失効します。",
					},
					CTALabel: "パスワードを再設定",
					CTAURL:   "{{link}}",
					Footnote: "ボタンが開かない場合は次のURLをブラウザに貼り付けてください：{{link}}<br>ご本人による操作でない場合、パスワードは変更されません。",
					Closing:  "よろしくお願いいたします。<br><strong>{{system_name}}</strong>",
				}),
			},
			"system_error": {
				Subject: "{{system_name}} - システムエラー通知",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "ja",
					Title:    "{{system_name}} - システムエラー",
					Greeting: "管理者様、",
					Paragraphs: []string{
						"<strong>{{system_name}}</strong> で確認が必要なシステムエラーが記録されました。",
					},
					Details: [][2]string{
						{"発生時刻", "{{time}}"},
						{"エラー内容", "{{error_message}}"},
					},
					Footnote: "詳細はサーバーログおよび管理画面をご確認ください。",
					Closing:  "<strong>{{system_name}}</strong>",
				}),
			},
			"quota_warning": {
				Subject: "{{system_name}} - 残高低下のお知らせ",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "ja",
					Title:    "{{system_name}} - 残高アラート",
					Greeting: "{{user_name}} 様、",
					Paragraphs: []string{
						"<strong>{{system_name}}</strong> アカウントの残高が通知しきい値を下回りました。",
						"<strong>現在の残高：</strong>{{quota_remaining}}。API利用の中断を防ぐため、必要に応じてチャージしてください。",
					},
					CTALabel: "残高をチャージ",
					CTAURL:   "{{topup_link}}",
					Closing:  "よろしくお願いいたします。<br><strong>{{system_name}}</strong>",
				}),
			},
			"ticket_created": {
				Subject: "{{system_name}} - 新規サポートチケット #{{ticket_id}}",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "ja",
					Title:    "{{system_name}} - 新規チケット",
					Greeting: "管理者様、",
					Paragraphs: []string{
						"新しいサポートチケットが作成されました。",
					},
					Details: [][2]string{
						{"チケット番号", "#{{ticket_id}}"},
						{"件名", "{{ticket_title}}"},
						{"ユーザー", "{{user_name}} ({{user_email}})"},
						{"カテゴリ / 優先度", "{{category}} / {{priority}}"},
						{"内容", "{{message}}"},
					},
					Closing: "<strong>{{system_name}}</strong>",
				}),
			},
			"ticket_replied": {
				Subject: "{{system_name}} - チケット #{{ticket_id}} に返信があります",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "ja",
					Title:    "{{system_name}} - チケット返信",
					Greeting: "こんにちは、",
					Paragraphs: []string{
						"サポートチケット <strong>#{{ticket_id}} — {{ticket_title}}</strong> に新しい返信が投稿されました。",
					},
					Details: [][2]string{
						{"返信内容", "{{reply_message}}"},
					},
					CTALabel: "チケットを確認",
					CTAURL:   "{{dashboard_link}}",
					Closing:  "よろしくお願いいたします。<br><strong>{{system_name}}</strong>",
				}),
			},
			"channel_disabled": {
				Subject: "{{system_name}} - チャネルが無効化されました (#{{channel_id}})",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "ja",
					Title:    "{{system_name}} - チャネル無効化",
					Greeting: "管理者様、",
					Paragraphs: []string{
						"ヘルスチェック失敗により、アップストリームAPIチャネルが自動的に無効化されました。",
					},
					Details: [][2]string{
						{"チャネル", "{{channel_name}} (#{{channel_id}})"},
						{"理由", "{{reason}}"},
						{"時刻", "{{time}}"},
					},
					Footnote: "管理画面でチャネル設定とAPIキーをご確認ください。",
					Closing:  "<strong>{{system_name}}</strong>",
				}),
			},
			"video_expiry": {
				Subject: "{{system_name}} - 動画ファイルの保管期限が近づいています",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "ja",
					Title:    "{{system_name}} - 動画保管期限リマインダー",
					Greeting: "{{username}} 様、",
					Paragraphs: []string{
						"サーバーに保存されている動画ファイルは <strong>{{expires_at}}</strong> に自動削除されます。",
						"保持が必要な場合は、期限前にダウンロードしてください。",
					},
					CTALabel: "動画を開く",
					CTAURL:   "{{video_url}}",
					Closing:  "よろしくお願いいたします。<br><strong>{{system_name}}</strong>",
				}),
			},
			"fine_tuning_expiry": {
				Subject: "{{system_name}} - ファインチューニングモデルの期限まで残り {{days_left}} 日",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "ja",
					Title:    "{{system_name}} - モデル保管期限リマインダー",
					Greeting: "{{username}} 様、",
					Paragraphs: []string{
						"ファインチューニング済みモデル <strong>{{model}}</strong> は、あと <strong>{{days_left}} 日</strong>（<strong>{{expires_at}}</strong>）でサーバーストレージから自動削除されます。",
						"学習済みウェイト、チェックポイント、ログを保存するため、期限前に .zip アーカイブをダウンロードしてください。",
					},
					Details: [][2]string{
						{"モデル", "{{model}}"},
						{"ジョブID", "{{job_id}}"},
						{"削除予定日", "{{expires_at}}"},
					},
					CTALabel: "モデルをダウンロード",
					CTAURL:   "{{download_url}}",
					Closing:  "よろしくお願いいたします。<br><strong>{{system_name}}</strong>",
				}),
			},
		},
		"vi": {
			"verification": {
				Subject: "{{system_name}} - Mã xác minh email của bạn",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "vi",
					Title:    "{{system_name}} - Xác minh email",
					Greeting: "Xin chào,",
					Paragraphs: []string{
						"Chúng tôi đã nhận được yêu cầu xác minh địa chỉ email cho tài khoản <strong>{{system_name}}</strong> của bạn. Nhập mã bên dưới để tiếp tục:",
						"<strong>Thời hạn:</strong> Mã có hiệu lực trong {{valid_minutes}} phút.",
					},
					Code:     "{{code}}",
					CTALabel: "XÁC MINH EMAIL",
					CTAURL:   "{{link}}",
					Footnote: "Nếu bạn không thực hiện yêu cầu này, bạn có thể bỏ qua email này.",
					Closing:  "Trân trọng,<br><strong>{{system_name}}</strong>",
				}),
			},
			"password_reset": {
				Subject: "{{system_name}} - Yêu cầu đặt lại mật khẩu",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "vi",
					Title:    "{{system_name}} - Đặt lại mật khẩu",
					Greeting: "Xin chào,",
					Paragraphs: []string{
						"Chúng tôi nhận được yêu cầu đặt lại mật khẩu cho tài khoản <strong>{{system_name}}</strong> của bạn. Nhấn vào nút bên dưới để tạo mật khẩu mới.",
						"<strong>Thời hạn:</strong> Liên kết này sẽ hết hạn sau {{valid_minutes}} phút.",
					},
					CTALabel: "ĐẶT LẠI MẬT KHẨU",
					CTAURL:   "{{link}}",
					Footnote: "Nếu nút không mở được, hãy dán liên kết này vào trình duyệt: {{link}}<br>Nếu không phải bạn yêu cầu, mật khẩu của bạn sẽ không thay đổi.",
					Closing:  "Trân trọng,<br><strong>{{system_name}}</strong>",
				}),
			},
			"system_error": {
				Subject: "{{system_name}} - Cảnh báo lỗi hệ thống",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "vi",
					Title:    "{{system_name}} - Lỗi hệ thống",
					Greeting: "Xin chào,",
					Paragraphs: []string{
						"Hệ thống <strong>{{system_name}}</strong> vừa ghi nhận một lỗi cần được kiểm tra.",
					},
					Details: [][2]string{
						{"Thời gian", "{{time}}"},
						{"Lỗi", "{{error_message}}"},
					},
					Footnote: "Vui lòng kiểm tra nhật ký máy chủ và trang quản trị để biết thêm chi tiết.",
					Closing:  "Trân trọng,<br><strong>{{system_name}}</strong>",
				}),
			},
			"quota_warning": {
				Subject: "{{system_name}} - Cảnh báo số dư thấp",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "vi",
					Title:    "{{system_name}} - Cảnh báo số dư",
					Greeting: "Xin chào {{user_name}},",
					Paragraphs: []string{
						"Số dư còn lại trong tài khoản <strong>{{system_name}}</strong> của bạn đã giảm xuống dưới ngưỡng cảnh báo.",
						"<strong>Số dư còn lại:</strong> {{quota_remaining}}. Hãy nạp thêm số dư để tránh gián đoạn yêu cầu API.",
					},
					CTALabel: "NẠP SỐ DƯ",
					CTAURL:   "{{topup_link}}",
					Closing:  "Trân trọng,<br><strong>{{system_name}}</strong>",
				}),
			},
			"ticket_created": {
				Subject: "{{system_name}} - Yêu cầu hỗ trợ mới #{{ticket_id}}",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "vi",
					Title:    "{{system_name}} - Yêu cầu hỗ trợ mới",
					Greeting: "Xin chào,",
					Paragraphs: []string{
						"Một yêu cầu hỗ trợ mới vừa được gửi:",
					},
					Details: [][2]string{
						{"Mã yêu cầu", "#{{ticket_id}}"},
						{"Tiêu đề", "{{ticket_title}}"},
						{"Người dùng", "{{user_name}} ({{user_email}})"},
						{"Danh mục / Ưu tiên", "{{category}} / {{priority}}"},
						{"Nội dung", "{{message}}"},
					},
					Closing: "Trân trọng,<br><strong>{{system_name}}</strong>",
				}),
			},
			"ticket_replied": {
				Subject: "{{system_name}} - Phản hồi mới cho yêu cầu #{{ticket_id}}",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "vi",
					Title:    "{{system_name}} - Phản hồi hỗ trợ",
					Greeting: "Xin chào,",
					Paragraphs: []string{
						"Đội ngũ hỗ trợ đã phản hồi yêu cầu <strong>#{{ticket_id}} — {{ticket_title}}</strong> của bạn:",
					},
					Details: [][2]string{
						{"Phản hồi", "{{reply_message}}"},
					},
					CTALabel: "XEM YÊU CẦU",
					CTAURL:   "{{dashboard_link}}",
					Closing:  "Trân trọng,<br><strong>{{system_name}}</strong>",
				}),
			},
			"channel_disabled": {
				Subject: "{{system_name}} - Kênh đã bị tắt (#{{channel_id}})",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "vi",
					Title:    "{{system_name}} - Kênh bị tắt",
					Greeting: "Xin chào,",
					Paragraphs: []string{
						"Một kênh API upstream đã tự động bị tắt do không vượt qua kiểm tra trạng thái.",
					},
					Details: [][2]string{
						{"Kênh", "{{channel_name}} (#{{channel_id}})"},
						{"Lý do", "{{reason}}"},
						{"Thời gian", "{{time}}"},
					},
					Footnote: "Vui lòng kiểm tra khóa API và cấu hình kênh trong bảng quản trị.",
					Closing:  "Trân trọng,<br><strong>{{system_name}}</strong>",
				}),
			},
			"video_expiry": {
				Subject: "{{system_name}} - Video của bạn sắp hết hạn lưu trữ",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "vi",
					Title:    "{{system_name}} - Nhắc hết hạn video",
					Greeting: "Xin chào {{username}},",
					Paragraphs: []string{
						"Tệp video lưu trên máy chủ của bạn sẽ tự động bị xóa vào ngày <strong>{{expires_at}}</strong>.",
						"Nếu muốn giữ lại, hãy tải video về máy trước ngày hết hạn.",
					},
					CTALabel: "MỞ VIDEO",
					CTAURL:   "{{video_url}}",
					Closing:  "Trân trọng,<br><strong>{{system_name}}</strong>",
				}),
			},
			"fine_tuning_expiry": {
				Subject: "{{system_name}} - Mô hình tinh chỉnh sẽ bị xóa sau {{days_left}} ngày",
				Body: buildCleanEmailHTML(cleanEmailSpec{
					Lang:     "vi",
					Title:    "{{system_name}} - Nhắc hết hạn mô hình tinh chỉnh",
					Greeting: "Xin chào {{username}},",
					Paragraphs: []string{
						"Mô hình tinh chỉnh <strong>{{model}}</strong> của bạn sẽ tự động bị xóa khỏi máy chủ sau <strong>{{days_left}} ngày</strong> (<strong>{{expires_at}}</strong>).",
						"Hãy tải tệp nén .zip trước thời hạn để lưu trọng số, điểm kiểm tra và nhật ký huấn luyện.",
					},
					Details: [][2]string{
						{"Mô hình", "{{model}}"},
						{"Mã tác vụ", "{{job_id}}"},
						{"Ngày hết hạn", "{{expires_at}}"},
					},
					CTALabel: "TẢI MÔ HÌNH",
					CTAURL:   "{{download_url}}",
					Closing:  "Trân trọng,<br><strong>{{system_name}}</strong>",
				}),
			},
		},
	}
}

// NormalizeEmailLang maps frontend/browser language tags to one of our 8 template keys.
func NormalizeEmailLang(lang string) string {
	l := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(lang, "_", "-")))
	switch {
	case strings.HasPrefix(l, "tr"):
		return "tr"
	case l == "zh-tw" || l == "zh-hk" || l == "zh-mo" || l == "zh-hant":
		return "zh_TW"
	case strings.HasPrefix(l, "zh"):
		return "zh_CN"
	case strings.HasPrefix(l, "fr"):
		return "fr"
	case strings.HasPrefix(l, "ru"):
		return "ru"
	case strings.HasPrefix(l, "ja"):
		return "ja"
	case strings.HasPrefix(l, "vi"):
		return "vi"
	default:
		return "en"
	}
}

// RenderEmail resolves the subject and HTML body for a given template and language, replacing variables.
func RenderEmail(templateType string, lang string, vars map[string]string) (subject string, body string) {
	normLang := NormalizeEmailLang(lang)

	common.OptionMapRWMutex.RLock()
	customSubj := common.OptionMap[fmt.Sprintf("EmailSubject_%s_%s", templateType, normLang)]
	customBody := common.OptionMap[fmt.Sprintf("EmailBody_%s_%s", templateType, normLang)]
	common.OptionMapRWMutex.RUnlock()

	sysName := common.SystemName
	if sysName == "" {
		sysName = "New API"
	}

	if customSubj == "" || customBody == "" {
		tplMap, ok := defaultEmailTemplates[normLang]
		if !ok {
			tplMap = defaultEmailTemplates["en"]
		}
		tpl, ok := tplMap[templateType]
		if !ok {
			tpl = defaultEmailTemplates["en"][templateType]
		}
		if customSubj == "" {
			customSubj = tpl.Subject
		}
		if customBody == "" {
			customBody = tpl.Body
		}
	}

	allVars := map[string]string{
		"system_name":   sysName,
		"year":          fmt.Sprintf("%d", time.Now().Year()),
		"valid_minutes": fmt.Sprintf("%d", common.VerificationValidMinutes),
	}
	for k, v := range vars {
		allVars[k] = v
	}
	if _, ok := allVars["link"]; !ok {
		if c, hasCode := allVars["code"]; hasCode && c != "" {
			allVars["link"] = fmt.Sprintf("%s/verify-code?code=%s", strings.TrimRight(system_setting.ServerAddress, "/"), c)
		}
	}

	subject = customSubj
	body = customBody

	for k, v := range allVars {
		placeholder := "{{" + k + "}}"
		subject = strings.ReplaceAll(subject, placeholder, v)
		body = strings.ReplaceAll(body, placeholder, html.EscapeString(v))
	}

	return subject, body
}

func GetDefaultEmailTemplate(templateType, lang string) EmailTemplate {
	normLang := NormalizeEmailLang(lang)
	if tpls, ok := defaultEmailTemplates[normLang]; ok {
		if t, ok := tpls[templateType]; ok {
			return t
		}
	}
	return defaultEmailTemplates["en"][templateType]
}

func GetAllDefaultEmailTemplates() map[string]map[string]EmailTemplate {
	return defaultEmailTemplates
}
