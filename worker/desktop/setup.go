package main

// The first start of the client.
//
// The client asks the language of the user and it shows the privacy policy and
// the terms of service of the installation. The page is served by the client
// itself, therefore it works before the installation is reached and it uses the
// design of the installation. The user has to accept the policies before the
// client is used.

import (
	_ "embed"
	"encoding/json"
	"html/template"
	"net/http"
	"strings"
	"sync"
	"time"
)

//go:embed assets/marked.js
var markedJs []byte

func (b *localBridge) handleMarkedJS(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	writer.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = writer.Write(markedJs)
}

// setupLanguage is a language of the installation.
type setupLanguage struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// setupPolicy holds the privacy policy and the terms of service.
type setupPolicy struct {
	Privacy string `json:"privacy"`
	Terms   string `json:"terms"`
}

// setupBootstrap is the answer of the installation.
type setupBootstrap struct {
	Languages      []setupLanguage        `json:"languages"`
	SystemLanguage string                 `json:"system_language"`
	Policies       map[string]setupPolicy `json:"policies"`
}

// setupCache keeps the answer of the installation for the page, the page is
// opened more than once and the installation is not asked again and again.
var setupCache struct {
	sync.Mutex
	serverUrl string
	value     setupBootstrap
	fetched   time.Time
}

// setupLanguageNames are the names of the languages in the language itself.
var setupLanguageNames = map[string]string{
	"tr":    "Türkçe",
	"en":    "English",
	"zh_CN": "简体中文",
	"zh_TW": "繁體中文",
	"fr":    "Français",
	"ru":    "Русский",
	"ja":    "日本語",
	"vi":    "Tiếng Việt",
}

// fetchBootstrap reads the languages and the policies of the installation.
//
// The answer is kept for five minutes, the page of the setup is opened again
// when the language changes.
func fetchBootstrap(serverUrl string) setupBootstrap {
	if serverUrl == "" {
		return setupBootstrap{}
	}
	setupCache.Lock()
	if setupCache.serverUrl == serverUrl && time.Since(setupCache.fetched) < 5*time.Minute {
		value := setupCache.value
		setupCache.Unlock()
		return value
	}
	setupCache.Unlock()

	var bootstrap setupBootstrap
	client := &http.Client{Timeout: 6 * time.Second}
	response, err := client.Get(strings.TrimRight(serverUrl, "/") + "/api/v1/worker/bootstrap")
	if err == nil {
		defer response.Body.Close()
		if response.StatusCode == http.StatusOK {
			if err := json.NewDecoder(response.Body).Decode(&bootstrap); err != nil {
				bootstrap = setupBootstrap{}
			}
		}
	}

	setupCache.Lock()
	setupCache.serverUrl = serverUrl
	setupCache.value = bootstrap
	setupCache.fetched = time.Now()
	setupCache.Unlock()
	return bootstrap
}

// setupLanguages returns the languages of the installation.
func setupLanguages(bootstrap setupBootstrap) []setupLanguage {
	languages := bootstrap.Languages
	if len(languages) == 0 {
		languages = make([]setupLanguage, 0, len(clientLanguages))
		for _, code := range clientLanguages {
			languages = append(languages, setupLanguage{Code: code, Name: setupLanguageNames[code]})
		}
	}
	return languages
}

// setupSelected returns the language of the page of the setup.
//
// The language of the address is used first, the language of the user and the
// language of the system are used after that. English is used when the
// installation does not know any of them.
func setupSelected(request *http.Request, bootstrap setupBootstrap, configured string) string {
	languages := setupLanguages(bootstrap)
	known := func(code string) bool {
		for _, item := range languages {
			if item.Code == code {
				return true
			}
		}
		return false
	}
	for _, candidate := range []string{
		request.URL.Query().Get("lang"),
		configured,
		systemLanguage(),
		bootstrap.SystemLanguage,
	} {
		candidate = strings.TrimSpace(candidate)
		if candidate != "" && known(candidate) {
			return candidate
		}
	}
	return "en"
}

// setupPolicyOf returns the policies of the language with field-by-field fallback.
// If the chosen language has an empty Privacy or Terms, it falls back to English ("en"),
// and then to any available language.
func setupPolicyOf(bootstrap setupBootstrap, language string) setupPolicy {
	var result setupPolicy
	// 1. Try selected language
	if policy, ok := bootstrap.Policies[language]; ok {
		result.Privacy = strings.TrimSpace(policy.Privacy)
		result.Terms = strings.TrimSpace(policy.Terms)
	}
	// 2. Fall back to English for missing fields
	if enPolicy, ok := bootstrap.Policies["en"]; ok {
		if result.Privacy == "" {
			result.Privacy = strings.TrimSpace(enPolicy.Privacy)
		}
		if result.Terms == "" {
			result.Terms = strings.TrimSpace(enPolicy.Terms)
		}
	}
	// 3. Fall back to any language for still-missing fields
	if result.Privacy == "" || result.Terms == "" {
		for _, anyPolicy := range bootstrap.Policies {
			if result.Privacy == "" && strings.TrimSpace(anyPolicy.Privacy) != "" {
				result.Privacy = strings.TrimSpace(anyPolicy.Privacy)
			}
			if result.Terms == "" && strings.TrimSpace(anyPolicy.Terms) != "" {
				result.Terms = strings.TrimSpace(anyPolicy.Terms)
			}
		}
	}
	return result
}

// setupPage is the state of the page of the setup.
type setupPage struct {
	Title       string
	Subtitle    string
	Logo        template.URL
	Languages   []setupLanguage
	Language    string
	LanguageKey string
	Privacy     string
	Terms       string
	TextLang    string
	TextPrivacy string
	TextTerms   string
	TextAccept  string
	TextTermsA  string
	TextCont    string
	TextIntro   string
	TextEmpty   string
	ServerUrl   string
}

// handleSetupPage redirects directly to the site /client page.
func (b *localBridge) handleSetupPage(writer http.ResponseWriter, request *http.Request) {
	State.Mu.Lock()
	serverUrl := State.Config.ServerUrl
	if serverUrl == "" {
		serverUrl = DefaultServerUrl
	}
	State.Config.LegalAccepted = true
	State.Mu.Unlock()

	target := b.pageAddress(serverUrl)
	if target == "" {
		target = strings.TrimRight(serverUrl, "/") + "/client"
	}
	http.Redirect(writer, request, target, http.StatusSeeOther)
}

// saveSetup keeps the language and redirects to the site /client page.
func (b *localBridge) saveSetup(writer http.ResponseWriter, request *http.Request) {
	State.Mu.Lock()
	serverUrl := State.Config.ServerUrl
	if serverUrl == "" {
		serverUrl = DefaultServerUrl
	}
	State.Config.LegalAccepted = true
	State.Mu.Unlock()

	target := b.pageAddress(serverUrl)
	if target == "" {
		target = strings.TrimRight(serverUrl, "/") + "/client"
	}
	http.Redirect(writer, request, target, http.StatusSeeOther)
}

// setupTemplate is the page of the first start.
//
// The page follows the theme of the system, it shows the name and the icon of
// the installation and it asks the language and the policies of the user.
var setupTemplate = template.Must(template.New("setup").Parse(`<!doctype html>
<html lang="{{.LanguageKey}}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}} — {{.Subtitle}}</title>
<style>
  :root {
    color-scheme: light dark;
    --bg: #f4f6fb;
    --panel: #ffffff;
    --text: #0f172a;
    --muted: #5b6780;
    --line: #e2e8f2;
    --field: #f7f9fd;
    --accent: #2f6df6;
    --accent-text: #ffffff;
    --soft: #eef3ff;
  }
  @media (prefers-color-scheme: dark) {
    :root {
      --bg: #0b1020;
      --panel: #121a2e;
      --text: #e8eefc;
      --muted: #9aa8c4;
      --line: #223052;
      --field: #0d1528;
      --accent: #5b8cff;
      --accent-text: #06122b;
      --soft: #16223c;
    }
  }
  * { box-sizing: border-box; }
  body {
    margin: 0;
    min-height: 100vh;
    background: var(--bg);
    color: var(--text);
    font-family: "DM Sans", "Segoe UI", system-ui, -apple-system, sans-serif;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 32px 18px;
  }
  main {
    width: 100%;
    max-width: 720px;
    background: var(--panel);
    border: 1px solid var(--line);
    border-radius: 20px;
    box-shadow: 0 24px 70px rgba(15, 23, 42, .14);
    overflow: hidden;
  }
  header {
    display: flex;
    align-items: center;
    gap: 16px;
    padding: 26px 30px 20px;
    border-bottom: 1px solid var(--line);
  }
  header img { width: 48px; height: 48px; border-radius: 12px; object-fit: contain; background: var(--soft); padding: 6px; }
  header .name { font-size: 20px; font-weight: 700; letter-spacing: -.01em; }
  header .step { font-size: 12px; color: var(--muted); text-transform: uppercase; letter-spacing: .12em; margin-top: 3px; }
  section { padding: 22px 30px; }
  p.intro { margin: 0 0 18px; color: var(--muted); font-size: 14px; line-height: 1.6; }
  label.field { display: block; font-size: 12px; font-weight: 700; letter-spacing: .06em; text-transform: uppercase; color: var(--muted); margin-bottom: 8px; }
  select {
    width: 100%; padding: 12px 14px; border-radius: 11px; border: 1px solid var(--line);
    background: var(--field); color: var(--text); font: inherit; font-size: 15px;
  }
  .policy { margin-top: 20px; border: 1px solid var(--line); border-radius: 14px; overflow: hidden; background: var(--field); }
  .policy + .policy { margin-top: 12px; }
  .policy summary {
    cursor: pointer; padding: 13px 16px; font-size: 14px; font-weight: 700;
    list-style: none; display: flex; justify-content: space-between; align-items: center;
  }
  .policy summary::-webkit-details-marker { display: none; }
  .policy summary::after { content: "›"; font-size: 18px; color: var(--muted); transform: rotate(90deg); transition: transform .15s; }
  .policy[open] summary::after { transform: rotate(-90deg); }
  .policy .text {
    max-height: 200px; overflow: auto; padding: 0 16px 14px; font-size: 13px; line-height: 1.65;
    color: var(--muted); border-top: 1px solid var(--line); padding-top: 12px;
  }
  .markdown-body h1, .markdown-body h2, .markdown-body h3, .markdown-body h4 { color: var(--text); margin: 12px 0 6px; font-size: 14px; font-weight: 700; }
  .markdown-body p { margin: 6px 0; }
  .markdown-body ul, .markdown-body ol { margin: 6px 0; padding-left: 20px; }
  .markdown-body li { margin: 2px 0; }
  .markdown-body a { color: var(--accent); text-decoration: underline; }
  .markdown-body table { width: 100%; border-collapse: collapse; margin: 8px 0; font-size: 12px; }
  .markdown-body th, .markdown-body td { border: 1px solid var(--line); padding: 6px 10px; text-align: left; }
  .markdown-body th { background: var(--panel); color: var(--text); font-weight: 600; }
  .markdown-body blockquote { border-left: 3px solid var(--line); margin: 8px 0; padding-left: 10px; color: var(--muted); font-style: italic; }
  .markdown-body code { font-family: monospace; font-size: 12px; background: var(--panel); padding: 2px 4px; border-radius: 4px; }
  .checks { margin-top: 18px; display: grid; gap: 10px; }
  .check { display: flex; gap: 11px; align-items: flex-start; font-size: 13.5px; line-height: 1.5; cursor: pointer; }
  .check input { width: 18px; height: 18px; margin: 1px 0 0; accent-color: var(--accent); flex: 0 0 auto; }
  footer { display: flex; align-items: center; justify-content: space-between; gap: 16px; padding: 18px 30px 24px; border-top: 1px solid var(--line); }
  footer .site { color: var(--muted); font-size: 12px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  button {
    border: 0; border-radius: 11px; padding: 12px 24px; background: var(--accent); color: var(--accent-text);
    font: inherit; font-size: 15px; font-weight: 700; cursor: pointer; flex: 0 0 auto;
  }
  button:hover { filter: brightness(1.06); }
  .empty { color: var(--muted); font-size: 13px; line-height: 1.6; }
  .empty a { color: var(--accent); }
  @media (max-width: 560px) {
    header, section, footer { padding-left: 20px; padding-right: 20px; }
    footer { flex-direction: column; align-items: stretch; }
  }
</style>
</head>
<body>
<main>
  <header>
    {{if .Logo}}<img src="{{.Logo}}" alt="">{{end}}
    <div>
      <div class="name">{{.Title}}</div>
      <div class="step">{{.Subtitle}}</div>
    </div>
  </header>
  <form method="post" action="/setup">
    <section>
      <p class="intro">{{.TextIntro}}</p>
      <label class="field" for="language">{{.TextLang}}</label>
      <select id="language" name="language" onchange="location.href='/?lang='+encodeURIComponent(this.value)">
        {{range .Languages}}<option value="{{.Code}}" {{if eq .Code $.Language}}selected{{end}}>{{.Name}}</option>{{end}}
      </select>

      <details class="policy" open>
        <summary>{{.TextPrivacy}}</summary>
        {{if .Privacy}}
          <template class="raw-markdown">{{.Privacy}}</template>
          <div class="text markdown-body"></div>
        {{else}}
          <div class="text empty">{{.TextEmpty}} {{if .ServerUrl}}<a href="{{.ServerUrl}}" target="_blank" rel="noreferrer">{{.ServerUrl}}</a>{{end}}</div>
        {{end}}
      </details>
      <details class="policy" open>
        <summary>{{.TextTerms}}</summary>
        {{if .Terms}}
          <template class="raw-markdown">{{.Terms}}</template>
          <div class="text markdown-body"></div>
        {{else}}
          <div class="text empty">{{.TextEmpty}} {{if .ServerUrl}}<a href="{{.ServerUrl}}" target="_blank" rel="noreferrer">{{.ServerUrl}}</a>{{end}}</div>
        {{end}}
      </details>

      <div class="checks">
        <label class="check"><input type="checkbox" name="privacy" value="1" required> <span>{{.TextAccept}}</span></label>
        <label class="check"><input type="checkbox" name="terms" value="1" required> <span>{{.TextTermsA}}</span></label>
      </div>
    </section>
    <footer>
      <div class="site">{{.ServerUrl}}</div>
      <button type="submit">{{.TextCont}}</button>
    </footer>
  </form>
</main>
<script src="/marked.js"></script>
<script>
  (function() {
    document.querySelectorAll('.policy').forEach(function(p) {
      var tmpl = p.querySelector('.raw-markdown');
      var body = p.querySelector('.markdown-body');
      if (tmpl && body) {
        var raw = tmpl.innerHTML;
        // Decode HTML entities if any
        var txt = document.createElement('textarea');
        txt.innerHTML = raw;
        var decoded = txt.value;
        try {
          if (window.marked && typeof window.marked.parse === 'function') {
            body.innerHTML = window.marked.parse(decoded);
          } else {
            body.textContent = decoded;
          }
        } catch (e) {
          body.textContent = decoded;
        }
      }
    });
  })();
</script>
</body>
</html>`))
