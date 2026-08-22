package common

import "strings"

// ContentLanguage describes one language the site's public content (blog,
// legal pages, SEO metadata, model descriptions) can be served in. The base
// language is English; every other language is optional and falls back to
// English (or an on-the-fly machine translation) when missing.
type ContentLanguage struct {
	Code   string `json:"code"`   // BCP-47 tag, e.g. "tr", "zh-CN"
	Name   string `json:"name"`   // English name
	Native string `json:"native"` // native name shown in the language picker
	Flag   string `json:"flag"`
}

const DefaultContentLanguage = "en"

// ContentLanguages is the canonical list of languages offered for public
// content. The frontend keeps a mirror of this list for boot-time URL prefix
// detection; keep both in sync.
var ContentLanguages = []ContentLanguage{
	{Code: "en", Name: "English", Native: "English", Flag: "🇬🇧"},
	{Code: "zh-CN", Name: "Chinese (Simplified)", Native: "简体中文", Flag: "🇨🇳"},
	{Code: "zh-TW", Name: "Chinese (Traditional)", Native: "繁體中文", Flag: "🇹🇼"},
	{Code: "tr", Name: "Turkish", Native: "Türkçe", Flag: "🇹🇷"},
	{Code: "fr", Name: "French", Native: "Français", Flag: "🇫🇷"},
	{Code: "de", Name: "German", Native: "Deutsch", Flag: "🇩🇪"},
	{Code: "es", Name: "Spanish", Native: "Español", Flag: "🇪🇸"},
	{Code: "pt", Name: "Portuguese", Native: "Português", Flag: "🇵🇹"},
	{Code: "ru", Name: "Russian", Native: "Русский", Flag: "🇷🇺"},
	{Code: "ja", Name: "Japanese", Native: "日本語", Flag: "🇯🇵"},
	{Code: "ko", Name: "Korean", Native: "한국어", Flag: "🇰🇷"},
	{Code: "ar", Name: "Arabic", Native: "العربية", Flag: "🇸🇦"},
	{Code: "hi", Name: "Hindi", Native: "हिन्दी", Flag: "🇮🇳"},
	{Code: "id", Name: "Indonesian", Native: "Bahasa Indonesia", Flag: "🇮🇩"},
	{Code: "vi", Name: "Vietnamese", Native: "Tiếng Việt", Flag: "🇻🇳"},
	{Code: "th", Name: "Thai", Native: "ไทย", Flag: "🇹🇭"},
	{Code: "it", Name: "Italian", Native: "Italiano", Flag: "🇮🇹"},
	{Code: "nl", Name: "Dutch", Native: "Nederlands", Flag: "🇳🇱"},
	{Code: "pl", Name: "Polish", Native: "Polski", Flag: "🇵🇱"},
	{Code: "uk", Name: "Ukrainian", Native: "Українська", Flag: "🇺🇦"},
	{Code: "cs", Name: "Czech", Native: "Čeština", Flag: "🇨🇿"},
	{Code: "sk", Name: "Slovak", Native: "Slovenčina", Flag: "🇸🇰"},
	{Code: "sl", Name: "Slovenian", Native: "Slovenščina", Flag: "🇸🇮"},
	{Code: "hr", Name: "Croatian", Native: "Hrvatski", Flag: "🇭🇷"},
	{Code: "sr", Name: "Serbian", Native: "Српски", Flag: "🇷🇸"},
	{Code: "bs", Name: "Bosnian", Native: "Bosanski", Flag: "🇧🇦"},
	{Code: "bg", Name: "Bulgarian", Native: "Български", Flag: "🇧🇬"},
	{Code: "mk", Name: "Macedonian", Native: "Македонски", Flag: "🇲🇰"},
	{Code: "el", Name: "Greek", Native: "Ελληνικά", Flag: "🇬🇷"},
	{Code: "ro", Name: "Romanian", Native: "Română", Flag: "🇷🇴"},
	{Code: "hu", Name: "Hungarian", Native: "Magyar", Flag: "🇭🇺"},
	{Code: "da", Name: "Danish", Native: "Dansk", Flag: "🇩🇰"},
	{Code: "sv", Name: "Swedish", Native: "Svenska", Flag: "🇸🇪"},
	{Code: "no", Name: "Norwegian", Native: "Norsk", Flag: "🇳🇴"},
	{Code: "fi", Name: "Finnish", Native: "Suomi", Flag: "🇫🇮"},
	{Code: "et", Name: "Estonian", Native: "Eesti", Flag: "🇪🇪"},
	{Code: "lv", Name: "Latvian", Native: "Latviešu", Flag: "🇱🇻"},
	{Code: "lt", Name: "Lithuanian", Native: "Lietuvių", Flag: "🇱🇹"},
	{Code: "is", Name: "Icelandic", Native: "Íslenska", Flag: "🇮🇸"},
	{Code: "ga", Name: "Irish", Native: "Gaeilge", Flag: "🇮🇪"},
	{Code: "mt", Name: "Maltese", Native: "Malti", Flag: "🇲🇹"},
	{Code: "sq", Name: "Albanian", Native: "Shqip", Flag: "🇦🇱"},
	{Code: "ca", Name: "Catalan", Native: "Català", Flag: "🇪🇸"},
	{Code: "eu", Name: "Basque", Native: "Euskara", Flag: "🇪🇸"},
	{Code: "gl", Name: "Galician", Native: "Galego", Flag: "🇪🇸"},
	{Code: "he", Name: "Hebrew", Native: "עברית", Flag: "🇮🇱"},
	{Code: "fa", Name: "Persian", Native: "فارسی", Flag: "🇮🇷"},
	{Code: "ur", Name: "Urdu", Native: "اردو", Flag: "🇵🇰"},
	{Code: "bn", Name: "Bengali", Native: "বাংলা", Flag: "🇧🇩"},
	{Code: "ta", Name: "Tamil", Native: "தமிழ்", Flag: "🇮🇳"},
	{Code: "te", Name: "Telugu", Native: "తెలుగు", Flag: "🇮🇳"},
	{Code: "mr", Name: "Marathi", Native: "मराठी", Flag: "🇮🇳"},
	{Code: "gu", Name: "Gujarati", Native: "ગુજરાતી", Flag: "🇮🇳"},
	{Code: "kn", Name: "Kannada", Native: "ಕನ್ನಡ", Flag: "🇮🇳"},
	{Code: "ml", Name: "Malayalam", Native: "മലയാളം", Flag: "🇮🇳"},
	{Code: "pa", Name: "Punjabi", Native: "ਪੰਜਾਬੀ", Flag: "🇮🇳"},
	{Code: "sw", Name: "Swahili", Native: "Kiswahili", Flag: "🇰🇪"},
	{Code: "am", Name: "Amharic", Native: "አማርኛ", Flag: "🇪🇹"},
	{Code: "ms", Name: "Malay", Native: "Bahasa Melayu", Flag: "🇲🇾"},
	{Code: "tl", Name: "Filipino", Native: "Filipino", Flag: "🇵🇭"},
	{Code: "my", Name: "Burmese", Native: "မြန်မာ", Flag: "🇲🇲"},
	{Code: "km", Name: "Khmer", Native: "ខ្មែរ", Flag: "🇰🇭"},
	{Code: "lo", Name: "Lao", Native: "ລາວ", Flag: "🇱🇦"},
	{Code: "ka", Name: "Georgian", Native: "ქართული", Flag: "🇬🇪"},
	{Code: "hy", Name: "Armenian", Native: "Հայերեն", Flag: "🇦🇲"},
	{Code: "az", Name: "Azerbaijani", Native: "Azərbaycan", Flag: "🇦🇿"},
	{Code: "kk", Name: "Kazakh", Native: "Қазақша", Flag: "🇰🇿"},
	{Code: "uz", Name: "Uzbek", Native: "Oʻzbek", Flag: "🇺🇿"},
	{Code: "mn", Name: "Mongolian", Native: "Монгол", Flag: "🇲🇳"},
	{Code: "ne", Name: "Nepali", Native: "नेपाली", Flag: "🇳🇵"},
	{Code: "si", Name: "Sinhala", Native: "සිංහල", Flag: "🇱🇰"},
	{Code: "af", Name: "Afrikaans", Native: "Afrikaans", Flag: "🇿🇦"},
}

var contentLanguageByCode = func() map[string]ContentLanguage {
	m := make(map[string]ContentLanguage, len(ContentLanguages))
	for _, lang := range ContentLanguages {
		m[strings.ToLower(lang.Code)] = lang
	}
	return m
}()

// NormalizeContentLanguage canonicalizes a language tag ("TR", "zh_cn",
// "zh-tw" -> "tr", "zh-CN", "zh-TW"). Returns "" for unknown codes.
func NormalizeContentLanguage(code string) string {
	lang, ok := LookupContentLanguage(code)
	if !ok {
		return ""
	}
	return lang.Code
}

// LookupContentLanguage resolves a user-supplied tag (any casing, "_" or "-"
// separators) to a supported content language.
func LookupContentLanguage(code string) (ContentLanguage, bool) {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(code), "_", "-"))
	if normalized == "" {
		return ContentLanguage{}, false
	}
	lang, ok := contentLanguageByCode[normalized]
	return lang, ok
}

// IsContentLanguage reports whether the code is a supported content language.
func IsContentLanguage(code string) bool {
	_, ok := LookupContentLanguage(code)
	return ok
}
