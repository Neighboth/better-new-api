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
	etaAttr := ""
	dateLine := ""
	if eta > 0 {
		etaAttr = ` data-eta="` + strconv.FormatInt(eta, 10) + `"`
		dateLine = `<div class="eta-label">Expected back <span id="eta-date"></span></div>`
	}

	page := `<!DOCTYPE html>
<html lang="en" class="dark">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex">
<title>Under maintenance</title>
<style>
:root {
  --bg: #060913;
  --panel: rgba(255,255,255,0.04);
  --panel-border: rgba(255,255,255,0.08);
  --text: #e6e8ef;
  --muted: #8b93a7;
  --accent: #60a5fa;
}
* { box-sizing: border-box; }
html, body { height: 100%; margin: 0; }
body {
  min-height: 100vh;
  font-family: ui-sans-serif, system-ui, -apple-system, sans-serif;
  color: var(--text);
  background: radial-gradient(1200px 700px at 20% -10%, #1e3a5f 0%, transparent 55%),
              radial-gradient(1100px 700px at 90% 110%, #312e81 0%, transparent 55%),
              var(--bg);
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}
body::before {
  content: "";
  position: absolute; inset: 0;
  background-image:
    radial-gradient(rgba(255,255,255,0.05) 1px, transparent 1px),
    radial-gradient(rgba(255,255,255,0.04) 1px, transparent 1px);
  background-size: 42px 42px, 90px 90px;
  background-position: 0 0, 20px 30px;
  mask-image: radial-gradient(ellipse 80% 60% at 50% 50%, black 30%, transparent 80%);
  pointer-events: none;
}
.card {
  position: relative;
  text-align: center;
  padding: 48px 36px 36px;
  max-width: 560px;
  width: calc(100% - 48px);
  border-radius: 24px;
  background: var(--panel);
  border: 1px solid var(--panel-border);
  backdrop-filter: blur(16px);
  box-shadow: 0 30px 80px rgba(0,0,0,0.45);
  animation: float-in 0.9s cubic-bezier(.2,.9,.25,1.2) both;
}
@keyframes float-in {
  from { opacity: 0; transform: translateY(24px) scale(0.96); }
  to   { opacity: 1; transform: none; }
}
.glyph {
  width: 96px; height: 96px; border-radius: 28px;
  margin: 0 auto 22px;
  display: flex; align-items: center; justify-content: center;
  background: linear-gradient(145deg, rgba(96,165,250,0.28), rgba(129,140,248,0.12));
  border: 1px solid rgba(96,165,250,0.35);
  box-shadow: 0 0 60px rgba(96,165,250,0.35), inset 0 0 30px rgba(96,165,250,0.15);
  animation: pulse 2.6s ease-in-out infinite;
}
@keyframes pulse {
  0%, 100% { box-shadow: 0 0 60px rgba(96,165,250,0.35), inset 0 0 30px rgba(96,165,250,0.15); }
  50%      { box-shadow: 0 0 90px rgba(96,165,250,0.55), inset 0 0 40px rgba(96,165,250,0.3); }
}
.glyph svg { width: 44px; height: 44px; stroke: #93c5fd; }
h1 {
  margin: 0 0 10px;
  font-size: clamp(1.5rem, 4vw, 2.1rem);
  font-weight: 800;
  letter-spacing: -0.03em;
  background: linear-gradient(180deg, #ffffff 20%, #a5b4fc 100%);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}
.msg { color: var(--muted); font-size: 1rem; line-height: 1.6; margin: 0 0 26px; }
.eta {
  display: inline-flex; flex-direction: column; align-items: center; gap: 4px;
  padding: 14px 22px; border-radius: 16px;
  background: rgba(0,0,0,0.35);
  border: 1px solid var(--panel-border);
  margin-bottom: 18px;
}
.eta-value { font-variant-numeric: tabular-nums; font-size: 1.45rem; font-weight: 700; letter-spacing: 0.02em; color: #bfdbfe; }
.eta-label { font-size: 0.72rem; color: var(--muted); text-transform: uppercase; letter-spacing: 0.14em; }
.status-dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; background: #f59e0b; margin-right: 8px; box-shadow: 0 0 10px rgba(245,158,11,0.8); animation: blink 1.1s steps(2) infinite; vertical-align: middle; }
@keyframes blink { 0%, 60% { opacity: 1; } 61%, 100% { opacity: 0.25; } }
.footer { margin-top: 22px; font-size: 0.75rem; color: #525b72; }
.spinner { position: absolute; top: 24px; right: 24px; opacity: 0.4; animation: spin 10s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }
</style>
</head>
<body>
<div class="card"` + etaAttr + `>
  <svg class="spinner" width="38" height="38" viewBox="0 0 38 38" fill="none">
    <circle cx="19" cy="19" r="17" stroke="#3b82f6" stroke-width="2" stroke-dasharray="40 70" stroke-linecap="round"/>
  </svg>
  <div class="glyph">
    <svg viewBox="0 0 24 24" fill="none" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">
      <path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z"/>
    </svg>
  </div>
  <h1>Scheduled maintenance in progress</h1>
  <p class="msg"><span class="status-dot"></span>` + htmlEscape(message) + `</p>
  ` + dateLine + `
  <div class="footer">503 — Service Unavailable · Retry later</div>
</div>
<script>
(function () {
  var eta = document.currentScript.parentNode.getAttribute('data-eta');
  if (!eta) return;
  var end = parseInt(eta, 10) * 1000;
  if (!isFinite(end) || end <= 0) return;

  var etaEl = document.getElementById('eta-date');
  if (etaEl) {
    var d = new Date(end);
    etaEl.textContent = d.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });
  }

  function tick() {
    var now = Date.now();
    var left = Math.max(0, end - now);
    var box = document.querySelector('.eta');
    if (!box) return;
    if (left === 0) {
      box.innerHTML = '<div class="eta-label">Finishing up…</div>';
      return;
    }
    var s = Math.floor(left / 1000);
    var h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60), sec = s % 60;
    var valueEl = box.querySelector('.eta-value');
    if (!valueEl) {
      var valueDiv = document.createElement('div');
      valueDiv.className = 'eta-value';
      box.insertBefore(valueDiv, box.firstChild);
      valueEl = valueDiv;
    }
    valueEl.textContent = (h > 0 ? String(h).padStart(2, '0') + ':' : '') +
      String(m).padStart(2, '0') + ':' + String(sec).padStart(2, '0');
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


