package middleware

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// maintenanceAllowedPrefixes are reachable while the installation is in
// maintenance mode: static assets needed by the maintenance screen itself,
// the login page and the login API, and the public status probes that
// clients read before deciding to show anything.
var maintenanceAllowedPrefixes = []string{
	"/sign-in",
	"/sign-up",
	"/login",
	"/register",
	"/api/user/login",
	"/api/user/auth/refresh",
	"/api/user/logout",
	"/api/user/register",
	"/api/status",
	"/api/notice",
	"/api/oauth",
	"/api/captcha",
	"/api/health",
	"/health",
	"/.well-known/",
	"/favicon",
	"/logo.png",
	"/robots.txt",
	"/manifest.webmanifest",
	"/sw.js",
	"/sitemap.xml",
	"/maintenance",
}

// maintenanceAllowedPaths are exact matches that should remain reachable
// (e.g. assets that would be served under the maintenance screen).
var maintenanceAllowedPaths = map[string]bool{
	"/manifest.json":    true,
	"/site.webmanifest": true,
}

// MaintenanceMode blocks the whole dashboard in maintenance mode: public
// pages serve a built-in HTML maintenance screen and API callers receive a
// 503 JSON body. The administrator is identified the same way as everywhere
// else in the codebase (dashboard JWT or PAT with an admin role); admins
// always keep full access so they can flip the toggle back off.
func MaintenanceMode() gin.HandlerFunc {
	return func(c *gin.Context) {
		s := operation_setting.GetMaintenanceModeSetting()
		if !s.Enabled {
			c.Next()
			return
		}

		message := s.Message
		if message == "" {
			message = "We'll be back shortly. The installation is undergoing scheduled maintenance."
		}

		path := c.Request.URL.Path

		// If visitor explicitly navigates to root `/`, ALWAYS render the maintenance page!
		// Even for admins, so an admin can see the maintenance page without random SPA re-hydration,
		// header flashes, or 503 toast errors.
		if path == "/" || path == "" {
			c.Abort()
			c.Header("Retry-After", "300")
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.String(http.StatusServiceUnavailable, renderMaintenancePage(message, s.ETA))
			return
		}

		// Administrators bypass the block, either with the Authorization
		// header (the SPA keeps the JWT in memory) or with the refresh
		// cookie (the SPA is re-hydrated by the refresh endpoint).
		user, _, _, err := classifyDashboardCredential(c)
		if err == nil && user != nil && user.Role >= common.RoleAdminUser {
			c.Next()
			return
		}
		if maintenanceAdminFromCookie(c) {
			c.Next()
			return
		}

		if maintenanceAllowedPaths[path] {
			c.Next()
			return
		}

		for _, prefix := range maintenanceAllowedPrefixes {
			if strings.HasPrefix(path, prefix) {
				c.Next()
				return
			}
		}
		if isMaintenanceAsset(path) {
			c.Next()
			return
		}

		if wantsJSON(c) {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
				"success": false,
				"message": message,
				"error": gin.H{
					"code":    "maintenance_mode",
					"message": message,
					"type":    "maintenance",
				},
			})
			return
		}

		c.Abort()
		c.Header("Retry-After", "300")
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusServiceUnavailable, renderMaintenancePage(message, s.ETA))
	}
}

// maintenanceAdminFromCookie maps the refresh cookie to a user row and
// reports whether it belongs to an administrator. Anything that fails to
// resolve — missing cookie, stale session, malformed payload, non-admin —
// returns false so the visitor sees the maintenance screen.
func maintenanceAdminFromCookie(c *gin.Context) bool {
	raw, err := c.Cookie(service.RefreshCookieName)
	if err != nil || raw == "" {
		return false
	}
	sid, ok := service.RefreshTokenSID(raw)
	if !ok || sid == "" {
		return false
	}
	session, err := model.GetUserSessionCached(sid)
	if err != nil || session == nil {
		return false
	}
	now := time.Now().Unix()
	if session.ExpiresAt <= now || session.Status != model.UserSessionStatusActive {
		return false
	}
	user, err := model.GetUserById(session.UserID, false)
	if err != nil || user == nil {
		return false
	}
	return user.Role >= common.RoleAdminUser
}

// isMaintenanceAsset reports whether a static asset file exists under the
// site's root directory: the maintenance screen itself fetches a few of
// these (logo, favicon, webmanifest, fonts).
func isMaintenanceAsset(path string) bool {
	matched, err := regexp.MatchString(`^(/static/.+|/assets/.+|/fonts/.+|/img/.+|/images/.+|/files/.+)$`, strings.ToLower(path))
	if err != nil {
		return false
	}
	return matched
}

func wantsJSON(c *gin.Context) bool {
	if strings.Contains(c.Request.URL.Path, "/api/") || strings.HasPrefix(c.Request.URL.Path, "/v1") {
		return true
	}
	accept := c.GetHeader("Accept")
	return strings.Contains(accept, "application/json") && !strings.Contains(accept, "text/html")
}

// renderMaintenancePage emits a clean, self-contained maintenance screen
// styled identically to the application's clean dashboard theme (zero AI slop,
// exact button radius, borders, font weights and colors).
func renderMaintenancePage(message string, eta int64) string {
	etaBlock := ""
	etaSeconds := int64(0)
	if eta > 0 {
		etaSeconds = eta
		etaBlock = `<div class="eta" id="eta-box">
        <div class="eta-value" id="eta-value">--:--:--</div>
        <div class="eta-label">Tahmini Tamamlanma: <span id="eta-date"></span></div>
      </div>`
	}

	siteName := strings.TrimSpace(common.OptionMap["SystemName"])
	if siteName == "" {
		siteName = "PixRouter"
	}
	logoLight := strings.TrimSpace(common.OptionMap["Logo"])
	logoDark := strings.TrimSpace(common.OptionMap["LogoDark"])
	if logoLight == "" && logoDark != "" {
		logoLight = logoDark
	}
	if logoDark == "" && logoLight != "" {
		logoDark = logoLight
	}

	logoHeight := 40
	if hStr := strings.TrimSpace(common.OptionMap["HeaderLogoHeight"]); hStr != "" {
		if h, err := strconv.Atoi(hStr); err == nil && h >= 10 && h <= 200 {
			logoHeight = h
		}
	}

	logoTag := ""
	if logoLight != "" && logoDark != "" {
		if logoLight == logoDark {
			logoTag = `<img class="logo" src="` + htmlEscape(logoLight) + `" alt="` + htmlEscape(siteName) + `" onerror="this.style.display='none'">`
		} else {
			logoTag = `<picture class="logo-picture">
        <source media="(prefers-color-scheme: light)" srcset="` + htmlEscape(logoLight) + `">
        <img class="logo" src="` + htmlEscape(logoDark) + `" alt="` + htmlEscape(siteName) + `" onerror="this.style.display='none'">
      </picture>`
		}
	}

	hideSiteName := common.HideHeaderSiteName || common.OptionMap["HideHeaderSiteName"] == "true" || common.OptionMap["HideHeaderSiteName"] == "1"
	siteNameTag := ""
	if !hideSiteName || logoTag == "" {
		siteNameTag = `<div class="brand-name">` + htmlEscape(siteName) + `</div>`
	}

	page := `<!DOCTYPE html>
<html lang="tr">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex, nofollow">
<title>` + htmlEscape(siteName) + ` — Bakım Modu</title>
<style>
:root {
  --bg: #09090b;
  --card: #18181b;
  --card-border: #27272a;
  --text: #f4f4f5;
  --text-muted: #a1a1aa;
  --primary: #2563eb;
  --primary-hover: #1d4ed8;
  --primary-foreground: #ffffff;
  --badge-bg: rgba(37, 99, 235, 0.1);
  --badge-border: rgba(37, 99, 235, 0.25);
  --badge-text: #60a5fa;
  --eta-bg: #111114;
}
@media (prefers-color-scheme: light) {
  :root {
    --bg: #f8fafc;
    --card: #ffffff;
    --card-border: #e2e8f0;
    --text: #0f172a;
    --text-muted: #64748b;
    --primary: #2563eb;
    --primary-hover: #1d4ed8;
    --primary-foreground: #ffffff;
    --badge-bg: #eff6ff;
    --badge-border: #bfdbfe;
    --badge-text: #2563eb;
    --eta-bg: #f1f5f9;
  }
}
* { box-sizing: border-box; }
html, body {
  height: 100%;
  margin: 0;
  font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
  background-color: var(--bg);
  color: var(--text);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
  -webkit-font-smoothing: antialiased;
}
.card {
  width: 100%;
  max-width: 440px;
  background-color: var(--card);
  border: 1px solid var(--card-border);
  border-radius: 16px;
  padding: 36px 32px;
  text-align: center;
  box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.15), 0 8px 10px -6px rgba(0, 0, 0, 0.1);
}
.brand {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  margin-bottom: 24px;
}
.logo-picture {
  display: block;
}
.logo {
  height: ` + strconv.Itoa(logoHeight) + `px;
  max-width: 220px;
  object-fit: contain;
  display: block;
}
.brand-name {
  font-size: 1.125rem;
  font-weight: 600;
  letter-spacing: -0.015em;
  color: var(--text);
}
.badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 12px;
  font-size: 0.75rem;
  font-weight: 500;
  border-radius: 9999px;
  background-color: var(--badge-bg);
  border: 1px solid var(--badge-border);
  color: var(--badge-text);
  margin-bottom: 16px;
}
.badge .dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background-color: #3b82f6;
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.25);
}
h1 {
  font-size: 1.25rem;
  font-weight: 600;
  margin: 0 0 8px;
  letter-spacing: -0.02em;
  color: var(--text);
}
.msg {
  color: var(--text-muted);
  font-size: 0.875rem;
  line-height: 1.5;
  margin: 0 0 20px;
}
.eta {
  background: var(--eta-bg);
  border: 1px solid var(--card-border);
  border-radius: 12px;
  padding: 14px;
  margin: 0 0 20px;
}
.eta-value {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 1.25rem;
  font-weight: 600;
  color: #3b82f6;
  margin-bottom: 4px;
}
.eta-label {
  font-size: 0.75rem;
  color: var(--text-muted);
}
.divider {
  height: 1px;
  background-color: var(--card-border);
  margin: 20px 0;
}
.hint {
  font-size: 0.8125rem;
  color: var(--text-muted);
  margin: 0 0 12px;
}
.admin-link {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  width: 100%;
  padding: 10px 16px;
  border-radius: 10px;
  background-color: var(--primary);
  color: var(--primary-foreground);
  font-size: 0.875rem;
  font-weight: 500;
  text-decoration: none;
  transition: background-color 0.15s ease, opacity 0.15s ease;
}
.admin-link:hover {
  background-color: var(--primary-hover);
}
.admin-link svg {
  width: 15px;
  height: 15px;
}
.foot {
  margin-top: 18px;
  font-size: 0.75rem;
  color: var(--text-muted);
}
@media (max-width: 480px) {
  .card {
    padding: 24px 20px;
    border-radius: 14px;
  }
}
</style>
</head>
<body>
<main class="card" role="main">
  <div class="brand">
    ` + logoTag + `
    ` + siteNameTag + `
  </div>
  <div class="badge"><span class="dot"></span>Bakım Çalışması</div>
  <h1>Kısa süreliğine bakımdayız</h1>
  <p class="msg">` + htmlEscape(message) + `</p>
  ` + etaBlock + `
  <div class="divider"></div>
  <p class="hint">Yönetici paneline erişmek için giriş yapın.</p>
  <a class="admin-link" href="/sign-in" aria-label="Admin girişi">
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      <rect x="3" y="11" width="18" height="11" rx="2" ry="2"/>
      <path d="M7 11V7a5 5 0 0 1 10 0v4"/>
    </svg>
    Admin Girişi
  </a>
  <div class="foot">503 — Service Unavailable</div>
</main>
<script>
(function () {
  var END = ` + strconv.FormatInt(etaSeconds, 10) + `;
  if (!END || END <= 0) {
    var etaBox = document.getElementById('eta-box');
    if (etaBox) etaBox.style.display = 'none';
    return;
  }
  var end = END * 1000;
  var etaDate = document.getElementById('eta-date');
  var valueEl = document.getElementById('eta-value');
  if (etaDate) {
    etaDate.textContent = new Date(end).toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });
  }
  function tick() {
    var left = Math.max(0, end - Date.now());
    if (left === 0) {
      if (valueEl) valueEl.textContent = '00:00:00';
      return;
    }
    var s = Math.floor(left / 1000);
    var h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60), sec = s % 60;
    if (valueEl) {
      valueEl.textContent = (h > 0 ? (h < 10 ? '0' + h : String(h)) + ':' : '') +
        String(m).padStart(2, '0') + ':' + String(sec).padStart(2, '0');
    }
  }
  tick();
  setInterval(tick, 1000);
})();
</script>
</body>
</html>`

	return page
}

func htmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&#39;",
	)
	return r.Replace(s)
}
