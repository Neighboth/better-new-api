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
	"/login",
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

		path := c.Request.URL.Path

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

		message := s.Message
		if message == "" {
			message = "We'll be back shortly. The installation is undergoing scheduled maintenance."
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

// renderMaintenancePage emits a self-contained, dependency-free maintenance
// screen: inline CSS + inline JS, no external requests (the installation is
// fully down, hitting a CDN would leak the outage to the visitor).
func renderMaintenancePage(message string, eta int64) string {
	etaBlock := ""
	etaSeconds := int64(0)
	if eta > 0 {
		etaSeconds = eta
		etaBlock = `<div class="eta" id="eta-box">
        <div class="eta-value" id="eta-value">--:--:--</div>
        <div class="eta-label">Expected back <span id="eta-date"></span></div>
      </div>`
	}

	siteName := strings.TrimSpace(common.OptionMap["SystemName"])
	if siteName == "" {
		siteName = "PixRouter"
	}
	logo := strings.TrimSpace(common.OptionMap["Logo"])
	if logo == "" {
		logo = strings.TrimSpace(common.OptionMap["LogoDark"])
	}
	logoTag := ""
	if logo != "" {
		logoTag = `<img class="logo" src="` + htmlEscape(logo) + `" alt="` + htmlEscape(siteName) + `" onerror="this.style.display='none'">`
	}

	page := `<!DOCTYPE html>
<html lang="tr">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex">
<title>` + htmlEscape(siteName) + ` — Bakım</title>
<style>
:root {
  --bg-from: #0b1020;
  --bg-to: #171a35;
  --panel: rgba(255,255,255,0.04);
  --panel-border: rgba(255,255,255,0.10);
  --text: #e7e9f5;
  --muted: #9aa0bd;
  --accent: #8b5cf6;
  --accent-2: #38bdf8;
  --accent-soft: rgba(139,92,246,0.25);
  --logo-shadow: rgba(139,92,246,0.45);
}
@media (prefers-color-scheme: light) {
  :root {
    --bg-from: #eef2ff;
    --bg-to: #f5f7ff;
    --panel: #ffffff;
    --panel-border: rgba(15, 23, 42, 0.08);
    --text: #1e293b;
    --muted: #64748b;
    --accent: #6d28d9;
    --accent-2: #0284c7;
    --accent-soft: rgba(109,40,217,0.12);
    --logo-shadow: rgba(109,40,217,0.25);
  }
}
* { box-sizing: border-box; }
html, body { height: 100%; margin: 0; }
body {
  min-height: 100vh;
  font-family: ui-sans-serif, system-ui, -apple-system, "Segoe UI", sans-serif;
  color: var(--text);
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  background:
    radial-gradient(900px 520px at 15% -10%, var(--accent-soft), transparent 60%),
    radial-gradient(800px 520px at 110% 110%, rgba(56,189,248,0.18), transparent 55%),
    linear-gradient(160deg, var(--bg-from), var(--bg-to));
}
body::before {
  content: "";
  position: absolute; inset: 0;
  background-image:
    linear-gradient(rgba(127,127,127,0.06) 1px, transparent 1px),
    linear-gradient(90deg, rgba(127,127,127,0.06) 1px, transparent 1px);
  background-size: 44px 44px;
  mask-image: radial-gradient(ellipse 90% 80% at 50% 40%, black 35%, transparent 78%);
  pointer-events: none;
}
.card {
  position: relative;
  width: min(560px, calc(100% - 40px));
  text-align: center;
  padding: 44px 36px 30px;
  border-radius: 24px;
  background: var(--panel);
  border: 1px solid var(--panel-border);
  backdrop-filter: blur(18px);
  box-shadow: 0 40px 90px rgba(0,0,0,0.35), 0 0 0 1px var(--panel-border);
  animation: rise .7s cubic-bezier(.2,.8,.25,1.15) both;
}
@keyframes rise { from { opacity: 0; transform: translateY(22px) scale(.97); } to { opacity: 1; transform: none; } }
.brand { display: flex; align-items: center; justify-content: center; gap: 12px; margin-bottom: 22px; }
.logo { height: 40px; width: auto; max-width: 190px; object-fit: contain; filter: drop-shadow(0 6px 18px var(--logo-shadow)); }
.brand-name { font-size: 1.35rem; font-weight: 700; letter-spacing: -0.01em; color: var(--text); }
.brand-mark {
  width: 58px; height: 58px; border-radius: 17px; display: flex; align-items: center; justify-content: center;
  background: linear-gradient(145deg, var(--accent), var(--accent-2));
  border: 1px solid rgba(255,255,255,0.25);
  box-shadow: 0 12px 34px var(--logo-shadow), inset 0 1px 0 rgba(255,255,255,0.35);
}
.brand-mark svg { width: 28px; height: 28px; stroke: #fff; }
.badge {
  display: inline-flex; align-items: center; gap: 8px;
  padding: 6px 14px; border-radius: 999px;
  font-size: .72rem; font-weight: 600; letter-spacing: .12em; text-transform: uppercase;
  color: #fbbf24; background: rgba(251,191,36,0.10); border: 1px solid rgba(251,191,36,0.30);
  margin-bottom: 14px;
}
.badge .dot { width: 7px; height: 7px; border-radius: 50%; background: #f59e0b; box-shadow: 0 0 10px rgba(245,158,11,.9); animation: blink 1.2s steps(2) infinite; }
@keyframes blink { 0%,60% {opacity:1} 61%,100% {opacity:.2} }
h1 {
  margin: 0 0 8px; font-size: clamp(1.5rem, 4.4vw, 2.05rem); font-weight: 800; letter-spacing: -0.03em;
  background: linear-gradient(180deg, var(--text) 30%, var(--accent-2) 100%);
  -webkit-background-clip: text; background-clip: text; color: transparent;
}
.msg { color: var(--muted); font-size: .98rem; line-height: 1.65; margin: 0 0 22px; }
.eta {
  display: flex; flex-direction: column; align-items: center; gap: 4px;
  padding: 14px 26px; margin: 0 auto 20px; border-radius: 16px; width: fit-content;
  background: rgba(127,127,127,0.10); border: 1px solid var(--panel-border);
}
.eta-value { font-variant-numeric: tabular-nums; font-size: 1.55rem; font-weight: 700; letter-spacing: .03em; color: var(--accent-2); }
.eta-label { font-size: .68rem; color: var(--muted); text-transform: uppercase; letter-spacing: .16em; }
.divider { height: 1px; background: var(--panel-border); margin: 6px 0 16px; }
.hint { font-size: .8rem; color: var(--muted); margin: 0 0 12px; }
.admin-link {
  display: inline-flex; align-items: center; gap: 10px;
  width: 100%; justify-content: center;
  padding: 12px 18px; border-radius: 14px;
  font-size: .9rem; font-weight: 600; text-decoration: none;
  color: #fff; border: 1px solid rgba(255,255,255,0.18);
  background: linear-gradient(145deg, var(--accent), #6d28d9);
  box-shadow: 0 12px 30px rgba(109,40,217,0.35), inset 0 1px 0 rgba(255,255,255,0.25);
  transition: transform .18s ease, box-shadow .18s ease, filter .18s ease;
}
.admin-link:hover { transform: translateY(-1px); filter: brightness(1.06); box-shadow: 0 16px 38px rgba(109,40,217,0.45), inset 0 1px 0 rgba(255,255,255,0.3); }
.admin-link svg { width: 18px; height: 18px; }
.foot { margin-top: 16px; font-size: .7rem; color: var(--muted); opacity: .8; }
.spinner {
  position: absolute; top: 22px; right: 22px; opacity: .45;
  animation: spin 9s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }
@media (max-width: 480px) { .card { padding: 34px 22px 24px; border-radius: 20px; } }
</style>
</head>
<body>
<main class="card" role="main">
  <svg class="spinner" width="34" height="34" viewBox="0 0 34 34" fill="none" aria-hidden="true">
    <circle cx="17" cy="17" r="15" stroke="var(--accent-2)" stroke-width="2" stroke-dasharray="40 55" stroke-linecap="round"/>
  </svg>
  <div class="brand">
    ` + logoTag + `
    <div class="brand-name">` + htmlEscape(siteName) + `</div>
  </div>
  <div class="badge"><span class="dot"></span>Bakım Çalışması</div>
  <h1>Kısa süreliğine bakımdayız</h1>
  <p class="msg">` + htmlEscape(message) + `</p>
  ` + etaBlock + `
  <div class="divider"></div>
  <p class="hint">Bu ekranın üzerinden giriş yaparak siteyi yönetin.</p>
  <a class="admin-link" href="/login" aria-label="Admin girişi">
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      <circle cx="12" cy="8" r="4"/><path d="M4 21a8 8 0 0 1 16 0"/><path d="M12 13v4"/><path d="M9.5 15l-1.5 1.5M14.5 15l1.5 1.5"/>
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
