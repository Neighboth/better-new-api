package service

import (
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

type EmailTemplate struct {
	Subject string
	Body    string
}

// Built-in professional, responsive HTML email templates with anti-spam layout
var defaultEmailTemplates = map[string]map[string]EmailTemplate{
	"tr": {
		"verification": {
			Subject: "{{system_name}} - E-posta Doğrulama Kodunuz",
			Body: `<!DOCTYPE html>
<html lang="tr">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{system_name}} Doğrulama Kodu</title>
</head>
<body style="margin: 0; padding: 0; background-color: #f4f7fb; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; color: #1e293b;">
  <table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="min-width: 100%; background-color: #f4f7fb; padding: 40px 15px;">
    <tr>
      <td align="center">
        <table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="max-width: 560px; background-color: #ffffff; border-radius: 12px; box-shadow: 0 4px 16px rgba(0, 0, 0, 0.06); overflow: hidden; border: 1px solid #e2e8f0;">
          <tr>
            <td style="background: linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%); padding: 32px 40px; text-align: center;">
              <h1 style="color: #ffffff; margin: 0; font-size: 24px; font-weight: 700; letter-spacing: -0.5px;">{{system_name}}</h1>
            </td>
          </tr>
          <tr>
            <td style="padding: 40px;">
              <h2 style="margin: 0 0 16px 0; font-size: 20px; font-weight: 600; color: #0f172a;">E-posta Adresinizi Doğrulayın</h2>
              <p style="margin: 0 0 24px 0; font-size: 15px; line-height: 24px; color: #475569;">
                Merhaba,<br>
                <strong>{{system_name}}</strong> hesabınız için e-posta doğrulama talebinde bulundunuz. Aşağıdaki tek kullanımlık doğrulama kodunu kullanarak işleminizi tamamlayabilirsiniz.
              </p>
              <div style="background-color: #f8fafc; border: 2px dashed #cbd5e1; border-radius: 8px; padding: 20px; text-align: center; margin-bottom: 24px;">
                <span style="font-size: 32px; font-weight: 800; letter-spacing: 8px; color: #1d4ed8; font-family: monospace;">{{code}}</span>
              </div>
              <p style="margin: 0 0 16px 0; font-size: 14px; line-height: 22px; color: #64748b;">
                ⏱️ Bu kod <strong>{{valid_minutes}} dakika</strong> boyunca geçerlidir.
              </p>
              <p style="margin: 0; font-size: 13px; line-height: 20px; color: #94a3b8;">
                Eğer bu işlemi siz başlatmadıysanız, bu e-postayı güvenle göz ardı edebilirsiniz. Hesabınız güvendedir.
              </p>
            </td>
          </tr>
          <tr>
            <td style="background-color: #f8fafc; padding: 24px 40px; border-top: 1px solid #e2e8f0; text-align: center; font-size: 12px; color: #94a3b8; line-height: 18px;">
              © {{year}} {{system_name}}. Tüm hakları saklıdır.<br>
              Bu otomatik bir güvenlik bildirimidir, lütfen bu e-postayı yanıtlamayınız.
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`,
		},
		"password_reset": {
			Subject: "{{system_name}} - Şifre Sıfırlama Talebi",
			Body: `<!DOCTYPE html>
<html lang="tr">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{system_name}} Şifre Sıfırlama</title>
</head>
<body style="margin: 0; padding: 0; background-color: #f4f7fb; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; color: #1e293b;">
  <table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="min-width: 100%; background-color: #f4f7fb; padding: 40px 15px;">
    <tr>
      <td align="center">
        <table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="max-width: 560px; background-color: #ffffff; border-radius: 12px; box-shadow: 0 4px 16px rgba(0, 0, 0, 0.06); overflow: hidden; border: 1px solid #e2e8f0;">
          <tr>
            <td style="background: linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%); padding: 32px 40px; text-align: center;">
              <h1 style="color: #ffffff; margin: 0; font-size: 24px; font-weight: 700; letter-spacing: -0.5px;">{{system_name}}</h1>
            </td>
          </tr>
          <tr>
            <td style="padding: 40px;">
              <h2 style="margin: 0 0 16px 0; font-size: 20px; font-weight: 600; color: #0f172a;">Şifrenizi Sıfırlayın</h2>
              <p style="margin: 0 0 24px 0; font-size: 15px; line-height: 24px; color: #475569;">
                Merhaba,<br>
                <strong>{{system_name}}</strong> hesabınız için bir şifre sıfırlama talebinde bulunuldu. Yeni şifrenizi belirlemek için aşağıdaki butona tıklayabilirsiniz.
              </p>
              <div style="text-align: center; margin: 32px 0;">
                <a href="{{link}}" target="_blank" style="display: inline-block; background-color: #2563eb; color: #ffffff; font-size: 16px; font-weight: 600; text-decoration: none; padding: 14px 32px; border-radius: 8px; box-shadow: 0 2px 8px rgba(37, 99, 235, 0.25);">Şifremi Sıfırla</a>
              </div>
              <p style="margin: 0 0 16px 0; font-size: 13px; line-height: 20px; color: #64748b;">
                Butona tıklayamıyorsanız, aşağıdaki bağlantıyı kopyalayıp tarayıcınızın adres çubuğuna yapıştırabilirsiniz:<br>
                <a href="{{link}}" style="color: #2563eb; word-break: break-all;">{{link}}</a>
              </p>
              <p style="margin: 0 0 16px 0; font-size: 14px; line-height: 22px; color: #64748b;">
                ⏱️ Bu bağlantı <strong>{{valid_minutes}} dakika</strong> geçerlidir.
              </p>
              <p style="margin: 0; font-size: 13px; line-height: 20px; color: #94a3b8;">
                Bu talebi siz yapmadıysanız lütfen endişelenmeyin; şifreniz değişmedi ve hesabınız güvendedir.
              </p>
            </td>
          </tr>
          <tr>
            <td style="background-color: #f8fafc; padding: 24px 40px; border-top: 1px solid #e2e8f0; text-align: center; font-size: 12px; color: #94a3b8; line-height: 18px;">
              © {{year}} {{system_name}}. Tüm hakları saklıdır.<br>
              Bu otomatik bir güvenlik bildirimidir, lütfen bu e-postayı yanıtlamayınız.
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`,
		},
	},
	"en": {
		"verification": {
			Subject: "{{system_name}} - Your Verification Code",
			Body: `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{system_name}} Verification Code</title>
</head>
<body style="margin: 0; padding: 0; background-color: #f4f7fb; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; color: #1e293b;">
  <table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="min-width: 100%; background-color: #f4f7fb; padding: 40px 15px;">
    <tr>
      <td align="center">
        <table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="max-width: 560px; background-color: #ffffff; border-radius: 12px; box-shadow: 0 4px 16px rgba(0, 0, 0, 0.06); overflow: hidden; border: 1px solid #e2e8f0;">
          <tr>
            <td style="background: linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%); padding: 32px 40px; text-align: center;">
              <h1 style="color: #ffffff; margin: 0; font-size: 24px; font-weight: 700; letter-spacing: -0.5px;">{{system_name}}</h1>
            </td>
          </tr>
          <tr>
            <td style="padding: 40px;">
              <h2 style="margin: 0 0 16px 0; font-size: 20px; font-weight: 600; color: #0f172a;">Verify Your Email Address</h2>
              <p style="margin: 0 0 24px 0; font-size: 15px; line-height: 24px; color: #475569;">
                Hello,<br>
                You requested a verification code for your <strong>{{system_name}}</strong> account. Use the one-time code below to complete your verification.
              </p>
              <div style="background-color: #f8fafc; border: 2px dashed #cbd5e1; border-radius: 8px; padding: 20px; text-align: center; margin-bottom: 24px;">
                <span style="font-size: 32px; font-weight: 800; letter-spacing: 8px; color: #1d4ed8; font-family: monospace;">{{code}}</span>
              </div>
              <p style="margin: 0 0 16px 0; font-size: 14px; line-height: 22px; color: #64748b;">
                ⏱️ This code is valid for <strong>{{valid_minutes}} minutes</strong>.
              </p>
              <p style="margin: 0; font-size: 13px; line-height: 20px; color: #94a3b8;">
                If you did not request this verification, please safely ignore this email. Your account remains secure.
              </p>
            </td>
          </tr>
          <tr>
            <td style="background-color: #f8fafc; padding: 24px 40px; border-top: 1px solid #e2e8f0; text-align: center; font-size: 12px; color: #94a3b8; line-height: 18px;">
              © {{year}} {{system_name}}. All rights reserved.<br>
              This is an automated security notification. Please do not reply to this email.
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`,
		},
		"password_reset": {
			Subject: "{{system_name}} - Password Reset Request",
			Body: `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{system_name}} Password Reset</title>
</head>
<body style="margin: 0; padding: 0; background-color: #f4f7fb; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; color: #1e293b;">
  <table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="min-width: 100%; background-color: #f4f7fb; padding: 40px 15px;">
    <tr>
      <td align="center">
        <table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="max-width: 560px; background-color: #ffffff; border-radius: 12px; box-shadow: 0 4px 16px rgba(0, 0, 0, 0.06); overflow: hidden; border: 1px solid #e2e8f0;">
          <tr>
            <td style="background: linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%); padding: 32px 40px; text-align: center;">
              <h1 style="color: #ffffff; margin: 0; font-size: 24px; font-weight: 700; letter-spacing: -0.5px;">{{system_name}}</h1>
            </td>
          </tr>
          <tr>
            <td style="padding: 40px;">
              <h2 style="margin: 0 0 16px 0; font-size: 20px; font-weight: 600; color: #0f172a;">Reset Your Password</h2>
              <p style="margin: 0 0 24px 0; font-size: 15px; line-height: 24px; color: #475569;">
                Hello,<br>
                We received a request to reset the password for your <strong>{{system_name}}</strong> account. Click the button below to choose a new password.
              </p>
              <div style="text-align: center; margin: 32px 0;">
                <a href="{{link}}" target="_blank" style="display: inline-block; background-color: #2563eb; color: #ffffff; font-size: 16px; font-weight: 600; text-decoration: none; padding: 14px 32px; border-radius: 8px; box-shadow: 0 2px 8px rgba(37, 99, 235, 0.25);">Reset Password</a>
              </div>
              <p style="margin: 0 0 16px 0; font-size: 13px; line-height: 20px; color: #64748b;">
                If the button doesn't work, copy and paste the following link into your browser:<br>
                <a href="{{link}}" style="color: #2563eb; word-break: break-all;">{{link}}</a>
              </p>
              <p style="margin: 0 0 16px 0; font-size: 14px; line-height: 22px; color: #64748b;">
                ⏱️ This link will expire in <strong>{{valid_minutes}} minutes</strong>.
              </p>
              <p style="margin: 0; font-size: 13px; line-height: 20px; color: #94a3b8;">
                If you did not request a password reset, you can safely ignore this email.
              </p>
            </td>
          </tr>
          <tr>
            <td style="background-color: #f8fafc; padding: 24px 40px; border-top: 1px solid #e2e8f0; text-align: center; font-size: 12px; color: #94a3b8; line-height: 18px;">
              © {{year}} {{system_name}}. All rights reserved.<br>
              This is an automated security notification. Please do not reply to this email.
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`,
		},
	},
	"zh_CN": {
		"verification": {
			Subject: "{{system_name}} - 邮箱验证码",
			Body: `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{system_name}} 验证码</title>
</head>
<body style="margin: 0; padding: 0; background-color: #f4f7fb; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'PingFang SC', 'Microsoft YaHei', sans-serif; color: #1e293b;">
  <table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="min-width: 100%; background-color: #f4f7fb; padding: 40px 15px;">
    <tr>
      <td align="center">
        <table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="max-width: 560px; background-color: #ffffff; border-radius: 12px; box-shadow: 0 4px 16px rgba(0, 0, 0, 0.06); overflow: hidden; border: 1px solid #e2e8f0;">
          <tr>
            <td style="background: linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%); padding: 32px 40px; text-align: center;">
              <h1 style="color: #ffffff; margin: 0; font-size: 24px; font-weight: 700;">{{system_name}}</h1>
            </td>
          </tr>
          <tr>
            <td style="padding: 40px;">
              <h2 style="margin: 0 0 16px 0; font-size: 20px; font-weight: 600; color: #0f172a;">验证您的邮箱</h2>
              <p style="margin: 0 0 24px 0; font-size: 15px; line-height: 24px; color: #475569;">
                您好，<br>
                您正在进行 <strong>{{system_name}}</strong> 账号的邮箱验证，请在验证页面输入以下验证码：
              </p>
              <div style="background-color: #f8fafc; border: 2px dashed #cbd5e1; border-radius: 8px; padding: 20px; text-align: center; margin-bottom: 24px;">
                <span style="font-size: 32px; font-weight: 800; letter-spacing: 8px; color: #1d4ed8; font-family: monospace;">{{code}}</span>
              </div>
              <p style="margin: 0 0 16px 0; font-size: 14px; line-height: 22px; color: #64748b;">
                ⏱️ 验证码有效期为 <strong>{{valid_minutes}} 分钟</strong>。
              </p>
              <p style="margin: 0; font-size: 13px; line-height: 20px; color: #94a3b8;">
                如果这不是您的操作，请忽略此邮件，您的账号依然安全。
              </p>
            </td>
          </tr>
          <tr>
            <td style="background-color: #f8fafc; padding: 24px 40px; border-top: 1px solid #e2e8f0; text-align: center; font-size: 12px; color: #94a3b8; line-height: 18px;">
              © {{year}} {{system_name}} 保留所有权利。<br>
              此邮件由系统自动发送，请勿直接回复。
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`,
		},
		"password_reset": {
			Subject: "{{system_name}} - 密码重置",
			Body: `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{system_name}} 密码重置</title>
</head>
<body style="margin: 0; padding: 0; background-color: #f4f7fb; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'PingFang SC', 'Microsoft YaHei', sans-serif; color: #1e293b;">
  <table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="min-width: 100%; background-color: #f4f7fb; padding: 40px 15px;">
    <tr>
      <td align="center">
        <table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="max-width: 560px; background-color: #ffffff; border-radius: 12px; box-shadow: 0 4px 16px rgba(0, 0, 0, 0.06); overflow: hidden; border: 1px solid #e2e8f0;">
          <tr>
            <td style="background: linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%); padding: 32px 40px; text-align: center;">
              <h1 style="color: #ffffff; margin: 0; font-size: 24px; font-weight: 700;">{{system_name}}</h1>
            </td>
          </tr>
          <tr>
            <td style="padding: 40px;">
              <h2 style="margin: 0 0 16px 0; font-size: 20px; font-weight: 600; color: #0f172a;">重置您的密码</h2>
              <p style="margin: 0 0 24px 0; font-size: 15px; line-height: 24px; color: #475569;">
                您好，<br>
                我们收到了您的 <strong>{{system_name}}</strong> 账号密码重置请求。请点击下方按钮完成重置：
              </p>
              <div style="text-align: center; margin: 32px 0;">
                <a href="{{link}}" target="_blank" style="display: inline-block; background-color: #2563eb; color: #ffffff; font-size: 16px; font-weight: 600; text-decoration: none; padding: 14px 32px; border-radius: 8px; box-shadow: 0 2px 8px rgba(37, 99, 235, 0.25);">重置密码</a>
              </div>
              <p style="margin: 0 0 16px 0; font-size: 13px; line-height: 20px; color: #64748b;">
                如果按钮无法点击，请复制以下链接到浏览器打开：<br>
                <a href="{{link}}" style="color: #2563eb; word-break: break-all;">{{link}}</a>
              </p>
              <p style="margin: 0 0 16px 0; font-size: 14px; line-height: 22px; color: #64748b;">
                ⏱️ 链接有效期为 <strong>{{valid_minutes}} 分钟</strong>。
              </p>
              <p style="margin: 0; font-size: 13px; line-height: 20px; color: #94a3b8;">
                如果这不是您的操作，请忽略此邮件。
              </p>
            </td>
          </tr>
          <tr>
            <td style="background-color: #f8fafc; padding: 24px 40px; border-top: 1px solid #e2e8f0; text-align: center; font-size: 12px; color: #94a3b8; line-height: 18px;">
              © {{year}} {{system_name}} 保留所有权利。<br>
              此邮件由系统自动发送，请勿直接回复。
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`,
		},
	},
	"zh_TW": {
		"verification": {
			Subject: "{{system_name}} - 您的驗證碼",
			Body: `<!DOCTYPE html>
<html lang="zh-TW">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{system_name}} 驗證碼</title>
</head>
<body style="margin: 0; padding: 0; background-color: #f4f7fb; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'PingFang TC', 'Microsoft JhengHei', sans-serif; color: #1e293b;">
  <table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="min-width: 100%; background-color: #f4f7fb; padding: 40px 15px;">
    <tr>
      <td align="center">
        <table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="max-width: 560px; background-color: #ffffff; border-radius: 12px; box-shadow: 0 4px 16px rgba(0, 0, 0, 0.06); overflow: hidden; border: 1px solid #e2e8f0;">
          <tr>
            <td style="background: linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%); padding: 32px 40px; text-align: center;">
              <h1 style="color: #ffffff; margin: 0; font-size: 24px; font-weight: 700;">{{system_name}}</h1>
            </td>
          </tr>
          <tr>
            <td style="padding: 40px;">
              <h2 style="margin: 0 0 16px 0; font-size: 20px; font-weight: 600; color: #0f172a;">驗證您的電子郵件</h2>
              <p style="margin: 0 0 24px 0; font-size: 15px; line-height: 24px; color: #475569;">
                您好，<br>
                您正在進行 <strong>{{system_name}}</strong> 帳號的信箱驗證，請在驗證頁面輸入以下驗證碼：
              </p>
              <div style="background-color: #f8fafc; border: 2px dashed #cbd5e1; border-radius: 8px; padding: 20px; text-align: center; margin-bottom: 24px;">
                <span style="font-size: 32px; font-weight: 800; letter-spacing: 8px; color: #1d4ed8; font-family: monospace;">{{code}}</span>
              </div>
              <p style="margin: 0 0 16px 0; font-size: 14px; line-height: 22px; color: #64748b;">
                ⏱️ 驗證碼有效期限為 <strong>{{valid_minutes}} 分鐘</strong>。
              </p>
              <p style="margin: 0; font-size: 13px; line-height: 20px; color: #94a3b8;">
                如果這不是您的操作，請忽略此郵件，您的帳號依然安全。
              </p>
            </td>
          </tr>
          <tr>
            <td style="background-color: #f8fafc; padding: 24px 40px; border-top: 1px solid #e2e8f0; text-align: center; font-size: 12px; color: #94a3b8; line-height: 18px;">
              © {{year}} {{system_name}} 保留所有權利。<br>
              此郵件由系統自動發送，請勿直接回覆。
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`,
		},
		"password_reset": {
			Subject: "{{system_name}} - 密碼重置請求",
			Body: `<!DOCTYPE html>
<html lang="zh-TW">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{system_name}} 密碼重置</title>
</head>
<body style="margin: 0; padding: 0; background-color: #f4f7fb; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'PingFang TC', 'Microsoft JhengHei', sans-serif; color: #1e293b;">
  <table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="min-width: 100%; background-color: #f4f7fb; padding: 40px 15px;">
    <tr>
      <td align="center">
        <table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="max-width: 560px; background-color: #ffffff; border-radius: 12px; box-shadow: 0 4px 16px rgba(0, 0, 0, 0.06); overflow: hidden; border: 1px solid #e2e8f0;">
          <tr>
            <td style="background: linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%); padding: 32px 40px; text-align: center;">
              <h1 style="color: #ffffff; margin: 0; font-size: 24px; font-weight: 700;">{{system_name}}</h1>
            </td>
          </tr>
          <tr>
            <td style="padding: 40px;">
              <h2 style="margin: 0 0 16px 0; font-size: 20px; font-weight: 600; color: #0f172a;">重置您的密碼</h2>
              <p style="margin: 0 0 24px 0; font-size: 15px; line-height: 24px; color: #475569;">
                您好，<br>
                我們收到了您的 <strong>{{system_name}}</strong> 帳號密碼重設請求。請點選下方按鈕完成重設：
              </p>
              <div style="text-align: center; margin: 32px 0;">
                <a href="{{link}}" target="_blank" style="display: inline-block; background-color: #2563eb; color: #ffffff; font-size: 16px; font-weight: 600; text-decoration: none; padding: 14px 32px; border-radius: 8px; box-shadow: 0 2px 8px rgba(37, 99, 235, 0.25);">重設密碼</a>
              </div>
              <p style="margin: 0 0 16px 0; font-size: 13px; line-height: 20px; color: #64748b;">
                若無法點選按鈕，請複製以下連結至瀏覽器開啟：<br>
                <a href="{{link}}" style="color: #2563eb; word-break: break-all;">{{link}}</a>
              </p>
              <p style="margin: 0 0 16px 0; font-size: 14px; line-height: 22px; color: #64748b;">
                ⏱️ 連結有效期限為 <strong>{{valid_minutes}} 分鐘</strong>。
              </p>
              <p style="margin: 0; font-size: 13px; line-height: 20px; color: #94a3b8;">
                如果這不是您的操作，請忽略此郵件。
              </p>
            </td>
          </tr>
          <tr>
            <td style="background-color: #f8fafc; padding: 24px 40px; border-top: 1px solid #e2e8f0; text-align: center; font-size: 12px; color: #94a3b8; line-height: 18px;">
              © {{year}} {{system_name}} 保留所有權利。<br>
              此郵件由系統自動發送，請勿直接回覆。
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`,
		},
	},
	"fr": {
		"verification": {
			Subject: "{{system_name}} - Votre code de vérification",
			Body: `<!DOCTYPE html>
<html lang="fr">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{system_name}} Code de vérification</title>
</head>
<body style="margin: 0; padding: 0; background-color: #f4f7fb; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; color: #1e293b;">
  <table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="min-width: 100%; background-color: #f4f7fb; padding: 40px 15px;">
    <tr>
      <td align="center">
        <table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="max-width: 560px; background-color: #ffffff; border-radius: 12px; box-shadow: 0 4px 16px rgba(0, 0, 0, 0.06); overflow: hidden; border: 1px solid #e2e8f0;">
          <tr>
            <td style="background: linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%); padding: 32px 40px; text-align: center;">
              <h1 style="color: #ffffff; margin: 0; font-size: 24px; font-weight: 700;">{{system_name}}</h1>
            </td>
          </tr>
          <tr>
            <td style="padding: 40px;">
              <h2 style="margin: 0 0 16px 0; font-size: 20px; font-weight: 600; color: #0f172a;">Vérifiez votre adresse e-mail</h2>
              <p style="margin: 0 0 24px 0; font-size: 15px; line-height: 24px; color: #475569;">
                Bonjour,<br>
                Vous avez demandé un code de vérification pour votre compte <strong>{{system_name}}</strong>. Utilisez le code unique ci-dessous :
              </p>
              <div style="background-color: #f8fafc; border: 2px dashed #cbd5e1; border-radius: 8px; padding: 20px; text-align: center; margin-bottom: 24px;">
                <span style="font-size: 32px; font-weight: 800; letter-spacing: 8px; color: #1d4ed8; font-family: monospace;">{{code}}</span>
              </div>
              <p style="margin: 0 0 16px 0; font-size: 14px; line-height: 22px; color: #64748b;">
                ⏱️ Ce code est valable pendant <strong>{{valid_minutes}} minutes</strong>.
              </p>
              <p style="margin: 0; font-size: 13px; line-height: 20px; color: #94a3b8;">
                Si vous n'avez pas demandé ce code, ignorez cet e-mail. Votre compte reste sécurisé.
              </p>
            </td>
          </tr>
          <tr>
            <td style="background-color: #f8fafc; padding: 24px 40px; border-top: 1px solid #e2e8f0; text-align: center; font-size: 12px; color: #94a3b8; line-height: 18px;">
              © {{year}} {{system_name}}. Tous droits réservés.<br>
              Ceci est un message automatique, veuillez ne pas y répondre.
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`,
		},
		"password_reset": {
			Subject: "{{system_name}} - Réinitialisation de votre mot de passe",
			Body: `<!DOCTYPE html>
<html lang="fr">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{system_name}} Réinitialisation de mot de passe</title>
</head>
<body style="margin: 0; padding: 0; background-color: #f4f7fb; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; color: #1e293b;">
  <table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="min-width: 100%; background-color: #f4f7fb; padding: 40px 15px;">
    <tr>
      <td align="center">
        <table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="max-width: 560px; background-color: #ffffff; border-radius: 12px; box-shadow: 0 4px 16px rgba(0, 0, 0, 0.06); overflow: hidden; border: 1px solid #e2e8f0;">
          <tr>
            <td style="background: linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%); padding: 32px 40px; text-align: center;">
              <h1 style="color: #ffffff; margin: 0; font-size: 24px; font-weight: 700;">{{system_name}}</h1>
            </td>
          </tr>
          <tr>
            <td style="padding: 40px;">
              <h2 style="margin: 0 0 16px 0; font-size: 20px; font-weight: 600; color: #0f172a;">Réinitialisez votre mot de passe</h2>
              <p style="margin: 0 0 24px 0; font-size: 15px; line-height: 24px; color: #475569;">
                Bonjour,<br>
                Nous avons reçu une demande de réinitialisation de mot de passe pour votre compte <strong>{{system_name}}</strong>. Cliquez ci-dessous :
              </p>
              <div style="text-align: center; margin: 32px 0;">
                <a href="{{link}}" target="_blank" style="display: inline-block; background-color: #2563eb; color: #ffffff; font-size: 16px; font-weight: 600; text-decoration: none; padding: 14px 32px; border-radius: 8px; box-shadow: 0 2px 8px rgba(37, 99, 235, 0.25);">Réinitialiser mon mot de passe</a>
              </div>
              <p style="margin: 0 0 16px 0; font-size: 13px; line-height: 20px; color: #64748b;">
                Si le bouton ne fonctionne pas, copiez ce lien dans votre navigateur :<br>
                <a href="{{link}}" style="color: #2563eb; word-break: break-all;">{{link}}</a>
              </p>
              <p style="margin: 0 0 16px 0; font-size: 14px; line-height: 22px; color: #64748b;">
                ⏱️ Ce lien est valide pendant <strong>{{valid_minutes}} minutes</strong>.
              </p>
              <p style="margin: 0; font-size: 13px; line-height: 20px; color: #94a3b8;">
                Si vous n'êtes pas à l'origine de cette demande, ignorez cet e-mail.
              </p>
            </td>
          </tr>
          <tr>
            <td style="background-color: #f8fafc; padding: 24px 40px; border-top: 1px solid #e2e8f0; text-align: center; font-size: 12px; color: #94a3b8; line-height: 18px;">
              © {{year}} {{system_name}}. Tous droits réservés.<br>
              Ceci est un message automatique, veuillez ne pas y répondre.
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`,
		},
	},
	"ru": {
		"verification": {
			Subject: "{{system_name}} - Ваш код подтверждения",
			Body: `<!DOCTYPE html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{system_name}} Код подтверждения</title>
</head>
<body style="margin: 0; padding: 0; background-color: #f4f7fb; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; color: #1e293b;">
  <table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="min-width: 100%; background-color: #f4f7fb; padding: 40px 15px;">
    <tr>
      <td align="center">
        <table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="max-width: 560px; background-color: #ffffff; border-radius: 12px; box-shadow: 0 4px 16px rgba(0, 0, 0, 0.06); overflow: hidden; border: 1px solid #e2e8f0;">
          <tr>
            <td style="background: linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%); padding: 32px 40px; text-align: center;">
              <h1 style="color: #ffffff; margin: 0; font-size: 24px; font-weight: 700;">{{system_name}}</h1>
            </td>
          </tr>
          <tr>
            <td style="padding: 40px;">
              <h2 style="margin: 0 0 16px 0; font-size: 20px; font-weight: 600; color: #0f172a;">Подтверждение адреса эл. почты</h2>
              <p style="margin: 0 0 24px 0; font-size: 15px; line-height: 24px; color: #475569;">
                Здравствуйте,<br>
                Вы запросили код подтверждения для аккаунта <strong>{{system_name}}</strong>. Введите одноразовый код:
              </p>
              <div style="background-color: #f8fafc; border: 2px dashed #cbd5e1; border-radius: 8px; padding: 20px; text-align: center; margin-bottom: 24px;">
                <span style="font-size: 32px; font-weight: 800; letter-spacing: 8px; color: #1d4ed8; font-family: monospace;">{{code}}</span>
              </div>
              <p style="margin: 0 0 16px 0; font-size: 14px; line-height: 22px; color: #64748b;">
                ⏱️ Код действителен в течение <strong>{{valid_minutes}} минут</strong>.
              </p>
              <p style="margin: 0; font-size: 13px; line-height: 20px; color: #94a3b8;">
                Если вы не совершали этот запрос, проигнорируйте письмо.
              </p>
            </td>
          </tr>
          <tr>
            <td style="background-color: #f8fafc; padding: 24px 40px; border-top: 1px solid #e2e8f0; text-align: center; font-size: 12px; color: #94a3b8; line-height: 18px;">
              © {{year}} {{system_name}}. Все права защищены.<br>
              Это автоматическое уведомление, не отвечайте на него.
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`,
		},
		"password_reset": {
			Subject: "{{system_name}} - Сброс пароля",
			Body: `<!DOCTYPE html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{system_name}} Сброс пароля</title>
</head>
<body style="margin: 0; padding: 0; background-color: #f4f7fb; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; color: #1e293b;">
  <table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="min-width: 100%; background-color: #f4f7fb; padding: 40px 15px;">
    <tr>
      <td align="center">
        <table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="max-width: 560px; background-color: #ffffff; border-radius: 12px; box-shadow: 0 4px 16px rgba(0, 0, 0, 0.06); overflow: hidden; border: 1px solid #e2e8f0;">
          <tr>
            <td style="background: linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%); padding: 32px 40px; text-align: center;">
              <h1 style="color: #ffffff; margin: 0; font-size: 24px; font-weight: 700;">{{system_name}}</h1>
            </td>
          </tr>
          <tr>
            <td style="padding: 40px;">
              <h2 style="margin: 0 0 16px 0; font-size: 20px; font-weight: 600; color: #0f172a;">Сброс вашего пароля</h2>
              <p style="margin: 0 0 24px 0; font-size: 15px; line-height: 24px; color: #475569;">
                Здравствуйте,<br>
                Поступил запрос на сброс пароля для аккаунта <strong>{{system_name}}</strong>. Нажмите кнопку ниже:
              </p>
              <div style="text-align: center; margin: 32px 0;">
                <a href="{{link}}" target="_blank" style="display: inline-block; background-color: #2563eb; color: #ffffff; font-size: 16px; font-weight: 600; text-decoration: none; padding: 14px 32px; border-radius: 8px; box-shadow: 0 2px 8px rgba(37, 99, 235, 0.25);">Сбросить пароль</a>
              </div>
              <p style="margin: 0 0 16px 0; font-size: 13px; line-height: 20px; color: #64748b;">
                Если кнопка не нажимается, скопируйте ссылку в адресную строку браузера:<br>
                <a href="{{link}}" style="color: #2563eb; word-break: break-all;">{{link}}</a>
              </p>
              <p style="margin: 0 0 16px 0; font-size: 14px; line-height: 22px; color: #64748b;">
                ⏱️ Ссылка действительна в течение <strong>{{valid_minutes}} минут</strong>.
              </p>
              <p style="margin: 0; font-size: 13px; line-height: 20px; color: #94a3b8;">
                Если вы не отправляли этот запрос, проигнорируйте письмо.
              </p>
            </td>
          </tr>
          <tr>
            <td style="background-color: #f8fafc; padding: 24px 40px; border-top: 1px solid #e2e8f0; text-align: center; font-size: 12px; color: #94a3b8; line-height: 18px;">
              © {{year}} {{system_name}}. Все права защищены.<br>
              Это автоматическое уведомление, не отвечайте на него.
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`,
		},
	},
	"ja": {
		"verification": {
			Subject: "{{system_name}} - 認証コードのご案内",
			Body: `<!DOCTYPE html>
<html lang="ja">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{system_name}} 認証コード</title>
</head>
<body style="margin: 0; padding: 0; background-color: #f4f7fb; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Hiragino Sans', 'Meiryo', sans-serif; color: #1e293b;">
  <table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="min-width: 100%; background-color: #f4f7fb; padding: 40px 15px;">
    <tr>
      <td align="center">
        <table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="max-width: 560px; background-color: #ffffff; border-radius: 12px; box-shadow: 0 4px 16px rgba(0, 0, 0, 0.06); overflow: hidden; border: 1px solid #e2e8f0;">
          <tr>
            <td style="background: linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%); padding: 32px 40px; text-align: center;">
              <h1 style="color: #ffffff; margin: 0; font-size: 24px; font-weight: 700;">{{system_name}}</h1>
            </td>
          </tr>
          <tr>
            <td style="padding: 40px;">
              <h2 style="margin: 0 0 16px 0; font-size: 20px; font-weight: 600; color: #0f172a;">メールアドレスの認証</h2>
              <p style="margin: 0 0 24px 0; font-size: 15px; line-height: 24px; color: #475569;">
                こんにちは、<br>
                <strong>{{system_name}}</strong> アカウントの認証リクエストを受け付けました。以下の認証コードを入力してください：
              </p>
              <div style="background-color: #f8fafc; border: 2px dashed #cbd5e1; border-radius: 8px; padding: 20px; text-align: center; margin-bottom: 24px;">
                <span style="font-size: 32px; font-weight: 800; letter-spacing: 8px; color: #1d4ed8; font-family: monospace;">{{code}}</span>
              </div>
              <p style="margin: 0 0 16px 0; font-size: 14px; line-height: 22px; color: #64748b;">
                ⏱️ この認証コードの有効期限は <strong>{{valid_minutes}} 分間</strong> です。
              </p>
              <p style="margin: 0; font-size: 13px; line-height: 20px; color: #94a3b8;">
                お心当たりがない場合は、このメールを破棄してください。アカウントは保護されています。
              </p>
            </td>
          </tr>
          <tr>
            <td style="background-color: #f8fafc; padding: 24px 40px; border-top: 1px solid #e2e8f0; text-align: center; font-size: 12px; color: #94a3b8; line-height: 18px;">
              © {{year}} {{system_name}}. All rights reserved.<br>
              本メールは送信専用アドレスより配信されています。返信は受け付けておりません。
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`,
		},
		"password_reset": {
			Subject: "{{system_name}} - パスワード再設定のご案内",
			Body: `<!DOCTYPE html>
<html lang="ja">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{system_name}} パスワード再設定</title>
</head>
<body style="margin: 0; padding: 0; background-color: #f4f7fb; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Hiragino Sans', 'Meiryo', sans-serif; color: #1e293b;">
  <table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="min-width: 100%; background-color: #f4f7fb; padding: 40px 15px;">
    <tr>
      <td align="center">
        <table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="max-width: 560px; background-color: #ffffff; border-radius: 12px; box-shadow: 0 4px 16px rgba(0, 0, 0, 0.06); overflow: hidden; border: 1px solid #e2e8f0;">
          <tr>
            <td style="background: linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%); padding: 32px 40px; text-align: center;">
              <h1 style="color: #ffffff; margin: 0; font-size: 24px; font-weight: 700;">{{system_name}}</h1>
            </td>
          </tr>
          <tr>
            <td style="padding: 40px;">
              <h2 style="margin: 0 0 16px 0; font-size: 20px; font-weight: 600; color: #0f172a;">パスワードの再設定</h2>
              <p style="margin: 0 0 24px 0; font-size: 15px; line-height: 24px; color: #475569;">
                こんにちは、<br>
                <strong>{{system_name}}</strong> アカウントのパスワード再設定リクエストを受け付けました。下のボタンをクリックしてください：
              </p>
              <div style="text-align: center; margin: 32px 0;">
                <a href="{{link}}" target="_blank" style="display: inline-block; background-color: #2563eb; color: #ffffff; font-size: 16px; font-weight: 600; text-decoration: none; padding: 14px 32px; border-radius: 8px; box-shadow: 0 2px 8px rgba(37, 99, 235, 0.25);">パスワードを再設定する</a>
              </div>
              <p style="margin: 0 0 16px 0; font-size: 13px; line-height: 20px; color: #64748b;">
                ボタンをクリックできない場合は、以下のURLをブラウザに貼り付けてアクセスしてください：<br>
                <a href="{{link}}" style="color: #2563eb; word-break: break-all;">{{link}}</a>
              </p>
              <p style="margin: 0 0 16px 0; font-size: 14px; line-height: 22px; color: #64748b;">
                ⏱️ このリンクの有効期限は <strong>{{valid_minutes}} 分間</strong> です。
              </p>
              <p style="margin: 0; font-size: 13px; line-height: 20px; color: #94a3b8;">
                心当たりがない場合は、このメールを破棄してください。
              </p>
            </td>
          </tr>
          <tr>
            <td style="background-color: #f8fafc; padding: 24px 40px; border-top: 1px solid #e2e8f0; text-align: center; font-size: 12px; color: #94a3b8; line-height: 18px;">
              © {{year}} {{system_name}}. All rights reserved.<br>
              本メールは送信専用アドレスより配信されています。返信は受け付けておりません。
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`,
		},
	},
	"vi": {
		"verification": {
			Subject: "{{system_name}} - Mã xác thực của bạn",
			Body: `<!DOCTYPE html>
<html lang="vi">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{system_name}} Mã xác thực</title>
</head>
<body style="margin: 0; padding: 0; background-color: #f4f7fb; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; color: #1e293b;">
  <table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="min-width: 100%; background-color: #f4f7fb; padding: 40px 15px;">
    <tr>
      <td align="center">
        <table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="max-width: 560px; background-color: #ffffff; border-radius: 12px; box-shadow: 0 4px 16px rgba(0, 0, 0, 0.06); overflow: hidden; border: 1px solid #e2e8f0;">
          <tr>
            <td style="background: linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%); padding: 32px 40px; text-align: center;">
              <h1 style="color: #ffffff; margin: 0; font-size: 24px; font-weight: 700;">{{system_name}}</h1>
            </td>
          </tr>
          <tr>
            <td style="padding: 40px;">
              <h2 style="margin: 0 0 16px 0; font-size: 20px; font-weight: 600; color: #0f172a;">Xác thực địa chỉ email</h2>
              <p style="margin: 0 0 24px 0; font-size: 15px; line-height: 24px; color: #475569;">
                Xin chào,<br>
                Bạn đã yêu cầu mã xác thực cho tài khoản <strong>{{system_name}}</strong>. Vui lòng nhập mã bên dưới:
              </p>
              <div style="background-color: #f8fafc; border: 2px dashed #cbd5e1; border-radius: 8px; padding: 20px; text-align: center; margin-bottom: 24px;">
                <span style="font-size: 32px; font-weight: 800; letter-spacing: 8px; color: #1d4ed8; font-family: monospace;">{{code}}</span>
              </div>
              <p style="margin: 0 0 16px 0; font-size: 14px; line-height: 22px; color: #64748b;">
                ⏱️ Mã này có hiệu lực trong vòng <strong>{{valid_minutes}} phút</strong>.
              </p>
              <p style="margin: 0; font-size: 13px; line-height: 20px; color: #94a3b8;">
                Nếu bạn không thực hiện yêu cầu này, vui lòng bỏ qua email.
              </p>
            </td>
          </tr>
          <tr>
            <td style="background-color: #f8fafc; padding: 24px 40px; border-top: 1px solid #e2e8f0; text-align: center; font-size: 12px; color: #94a3b8; line-height: 18px;">
              © {{year}} {{system_name}}. Bản quyền đã được bảo lưu.<br>
              Đây là thư thông báo tự động, vui lòng không phản hồi thư này.
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`,
		},
		"password_reset": {
			Subject: "{{system_name}} - Yêu cầu đặt lại mật khẩu",
			Body: `<!DOCTYPE html>
<html lang="vi">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{system_name}} Đặt lại mật khẩu</title>
</head>
<body style="margin: 0; padding: 0; background-color: #f4f7fb; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; color: #1e293b;">
  <table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="min-width: 100%; background-color: #f4f7fb; padding: 40px 15px;">
    <tr>
      <td align="center">
        <table role="presentation" border="0" cellpadding="0" cellspacing="0" width="100%" style="max-width: 560px; background-color: #ffffff; border-radius: 12px; box-shadow: 0 4px 16px rgba(0, 0, 0, 0.06); overflow: hidden; border: 1px solid #e2e8f0;">
          <tr>
            <td style="background: linear-gradient(135deg, #3b82f6 0%, #1d4ed8 100%); padding: 32px 40px; text-align: center;">
              <h1 style="color: #ffffff; margin: 0; font-size: 24px; font-weight: 700;">{{system_name}}</h1>
            </td>
          </tr>
          <tr>
            <td style="padding: 40px;">
              <h2 style="margin: 0 0 16px 0; font-size: 20px; font-weight: 600; color: #0f172a;">Đặt lại mật khẩu của bạn</h2>
              <p style="margin: 0 0 24px 0; font-size: 15px; line-height: 24px; color: #475569;">
                Xin chào,<br>
                Chúng tôi đã nhận được yêu cầu đặt lại mật khẩu cho tài khoản <strong>{{system_name}}</strong>. Nhấp vào nút bên dưới:
              </p>
              <div style="text-align: center; margin: 32px 0;">
                <a href="{{link}}" target="_blank" style="display: inline-block; background-color: #2563eb; color: #ffffff; font-size: 16px; font-weight: 600; text-decoration: none; padding: 14px 32px; border-radius: 8px; box-shadow: 0 2px 8px rgba(37, 99, 235, 0.25);">Đặt lại mật khẩu</a>
              </div>
              <p style="margin: 0 0 16px 0; font-size: 13px; line-height: 20px; color: #64748b;">
                Nếu không nhấp được vào nút, vui lòng sao chép đường liên kết sau vào trình duyệt:<br>
                <a href="{{link}}" style="color: #2563eb; word-break: break-all;">{{link}}</a>
              </p>
              <p style="margin: 0 0 16px 0; font-size: 14px; line-height: 22px; color: #64748b;">
                ⏱️ Đường dẫn này có hiệu lực trong <strong>{{valid_minutes}} phút</strong>.
              </p>
              <p style="margin: 0; font-size: 13px; line-height: 20px; color: #94a3b8;">
                Nếu bạn không gửi yêu cầu này, vui lòng bỏ qua thư này.
              </p>
            </td>
          </tr>
          <tr>
            <td style="background-color: #f8fafc; padding: 24px 40px; border-top: 1px solid #e2e8f0; text-align: center; font-size: 12px; color: #94a3b8; line-height: 18px;">
              © {{year}} {{system_name}}. Bản quyền đã được bảo lưu.<br>
              Đây là thư thông báo tự động, vui lòng không phản hồi thư này.
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`,
		},
	},
}

func NormalizeEmailLang(lang string) string {
	l := strings.ToLower(strings.TrimSpace(lang))
	if strings.HasPrefix(l, "tr") {
		return "tr"
	}
	if strings.HasPrefix(l, "zh") {
		if strings.Contains(l, "tw") || strings.Contains(l, "hk") || strings.Contains(l, "hant") {
			return "zh_TW"
		}
		return "zh_CN"
	}
	if strings.HasPrefix(l, "fr") {
		return "fr"
	}
	if strings.HasPrefix(l, "ru") {
		return "ru"
	}
	if strings.HasPrefix(l, "ja") {
		return "ja"
	}
	if strings.HasPrefix(l, "vi") {
		return "vi"
	}
	return "en"
}

// RenderEmail resolves the subject and HTML body for a given template and language, replacing variables
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

	// Fallback to built-in defaults
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

	// Prepare standard placeholders
	allVars := map[string]string{
		"system_name":   sysName,
		"year":          fmt.Sprintf("%d", time.Now().Year()),
		"valid_minutes": fmt.Sprintf("%d", common.VerificationValidMinutes),
	}
	for k, v := range vars {
		allVars[k] = v
	}

	subject = customSubj
	body = customBody

	for k, v := range allVars {
		placeholder := "{{" + k + "}}"
		subject = strings.ReplaceAll(subject, placeholder, v)
		body = strings.ReplaceAll(body, placeholder, html.EscapeString(v))
	}
	// For link, allow unescaped in href if replaced
	if link, ok := allVars["link"]; ok {
		body = strings.ReplaceAll(body, "{{link}}", link)
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
