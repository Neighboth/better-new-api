/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
// Content languages offered for public content (blog, legal pages, SEO,
// model descriptions). Mirrors common/content_languages.go on the backend;
// keep both in sync. English is the required base language, everything else
// is optional and falls back to English (or a machine translation).

export type ContentLanguage = {
  code: string
  name: string
  native: string
  flag: string
}

export const DEFAULT_CONTENT_LANGUAGE = 'en'

export const CONTENT_LANGUAGES: ContentLanguage[] = [
  { code: 'en', name: 'English', native: 'English', flag: '🇬🇧' },
  {
    code: 'zh-CN',
    name: 'Chinese (Simplified)',
    native: '简体中文',
    flag: '🇨🇳',
  },
  {
    code: 'zh-TW',
    name: 'Chinese (Traditional)',
    native: '繁體中文',
    flag: '🇹🇼',
  },
  { code: 'tr', name: 'Turkish', native: 'Türkçe', flag: '🇹🇷' },
  { code: 'fr', name: 'French', native: 'Français', flag: '🇫🇷' },
  { code: 'de', name: 'German', native: 'Deutsch', flag: '🇩🇪' },
  { code: 'es', name: 'Spanish', native: 'Español', flag: '🇪🇸' },
  { code: 'pt', name: 'Portuguese', native: 'Português', flag: '🇵🇹' },
  { code: 'ru', name: 'Russian', native: 'Русский', flag: '🇷🇺' },
  { code: 'ja', name: 'Japanese', native: '日本語', flag: '🇯🇵' },
  { code: 'ko', name: 'Korean', native: '한국어', flag: '🇰🇷' },
  { code: 'ar', name: 'Arabic', native: 'العربية', flag: '🇸🇦' },
  { code: 'hi', name: 'Hindi', native: 'हिन्दी', flag: '🇮🇳' },
  { code: 'id', name: 'Indonesian', native: 'Bahasa Indonesia', flag: '🇮🇩' },
  { code: 'vi', name: 'Vietnamese', native: 'Tiếng Việt', flag: '🇻🇳' },
  { code: 'th', name: 'Thai', native: 'ไทย', flag: '🇹🇭' },
  { code: 'it', name: 'Italian', native: 'Italiano', flag: '🇮🇹' },
  { code: 'nl', name: 'Dutch', native: 'Nederlands', flag: '🇳🇱' },
  { code: 'pl', name: 'Polish', native: 'Polski', flag: '🇵🇱' },
  { code: 'uk', name: 'Ukrainian', native: 'Українська', flag: '🇺🇦' },
  { code: 'cs', name: 'Czech', native: 'Čeština', flag: '🇨🇿' },
  { code: 'sk', name: 'Slovak', native: 'Slovenčina', flag: '🇸🇰' },
  { code: 'sl', name: 'Slovenian', native: 'Slovenščina', flag: '🇸🇮' },
  { code: 'hr', name: 'Croatian', native: 'Hrvatski', flag: '🇭🇷' },
  { code: 'sr', name: 'Serbian', native: 'Српски', flag: '🇷🇸' },
  { code: 'bs', name: 'Bosnian', native: 'Bosanski', flag: '🇧🇦' },
  { code: 'bg', name: 'Bulgarian', native: 'Български', flag: '🇧🇬' },
  { code: 'mk', name: 'Macedonian', native: 'Македонски', flag: '🇲🇰' },
  { code: 'el', name: 'Greek', native: 'Ελληνικά', flag: '🇬🇷' },
  { code: 'ro', name: 'Romanian', native: 'Română', flag: '🇷🇴' },
  { code: 'hu', name: 'Hungarian', native: 'Magyar', flag: '🇭🇺' },
  { code: 'da', name: 'Danish', native: 'Dansk', flag: '🇩🇰' },
  { code: 'sv', name: 'Swedish', native: 'Svenska', flag: '🇸🇪' },
  { code: 'no', name: 'Norwegian', native: 'Norsk', flag: '🇳🇴' },
  { code: 'fi', name: 'Finnish', native: 'Suomi', flag: '🇫🇮' },
  { code: 'et', name: 'Estonian', native: 'Eesti', flag: '🇪🇪' },
  { code: 'lv', name: 'Latvian', native: 'Latviešu', flag: '🇱🇻' },
  { code: 'lt', name: 'Lithuanian', native: 'Lietuvių', flag: '🇱🇹' },
  { code: 'is', name: 'Icelandic', native: 'Íslenska', flag: '🇮🇸' },
  { code: 'ga', name: 'Irish', native: 'Gaeilge', flag: '🇮🇪' },
  { code: 'mt', name: 'Maltese', native: 'Malti', flag: '🇲🇹' },
  { code: 'sq', name: 'Albanian', native: 'Shqip', flag: '🇦🇱' },
  { code: 'ca', name: 'Catalan', native: 'Català', flag: '🇪🇸' },
  { code: 'eu', name: 'Basque', native: 'Euskara', flag: '🇪🇸' },
  { code: 'gl', name: 'Galician', native: 'Galego', flag: '🇪🇸' },
  { code: 'he', name: 'Hebrew', native: 'עברית', flag: '🇮🇱' },
  { code: 'fa', name: 'Persian', native: 'فارسی', flag: '🇮🇷' },
  { code: 'ur', name: 'Urdu', native: 'اردو', flag: '🇵🇰' },
  { code: 'bn', name: 'Bengali', native: 'বাংলা', flag: '🇧🇩' },
  { code: 'ta', name: 'Tamil', native: 'தமிழ்', flag: '🇮🇳' },
  { code: 'te', name: 'Telugu', native: 'తెలుగు', flag: '🇮🇳' },
  { code: 'mr', name: 'Marathi', native: 'मराठी', flag: '🇮🇳' },
  { code: 'gu', name: 'Gujarati', native: 'ગુજરાતી', flag: '🇮🇳' },
  { code: 'kn', name: 'Kannada', native: 'ಕನ್ನಡ', flag: '🇮🇳' },
  { code: 'ml', name: 'Malayalam', native: 'മലയാളം', flag: '🇮🇳' },
  { code: 'pa', name: 'Punjabi', native: 'ਪੰਜਾਬੀ', flag: '🇮🇳' },
  { code: 'sw', name: 'Swahili', native: 'Kiswahili', flag: '🇰🇪' },
  { code: 'am', name: 'Amharic', native: 'አማርኛ', flag: '🇪🇹' },
  { code: 'ms', name: 'Malay', native: 'Bahasa Melayu', flag: '🇲🇾' },
  { code: 'tl', name: 'Filipino', native: 'Filipino', flag: '🇵🇭' },
  { code: 'my', name: 'Burmese', native: 'မြန်မာ', flag: '🇲🇲' },
  { code: 'km', name: 'Khmer', native: 'ខ្មែរ', flag: '🇰🇭' },
  { code: 'lo', name: 'Lao', native: 'ລາວ', flag: '🇱🇦' },
  { code: 'ka', name: 'Georgian', native: 'ქართული', flag: '🇬🇪' },
  { code: 'hy', name: 'Armenian', native: 'Հայերեն', flag: '🇦🇲' },
  { code: 'az', name: 'Azerbaijani', native: 'Azərbaycan', flag: '🇦🇿' },
  { code: 'kk', name: 'Kazakh', native: 'Қазақша', flag: '🇰🇿' },
  { code: 'uz', name: 'Uzbek', native: 'Oʻzbek', flag: '🇺🇿' },
  { code: 'mn', name: 'Mongolian', native: 'Монгол', flag: '🇲🇳' },
  { code: 'ne', name: 'Nepali', native: 'नेपाली', flag: '🇳🇵' },
  { code: 'si', name: 'Sinhala', native: 'සිංහල', flag: '🇱🇰' },
  { code: 'af', name: 'Afrikaans', native: 'Afrikaans', flag: '🇿🇦' },
]

const contentLanguageByCode = new Map(
  CONTENT_LANGUAGES.map((lang) => [lang.code.toLowerCase(), lang])
)

/** Canonicalize a language tag ("TR", "zh_cn") to a content language code. */
export function normalizeContentLanguage(value?: string | null): string {
  if (!value) return ''
  const normalized = value.trim().replaceAll('_', '-').toLowerCase()
  return contentLanguageByCode.get(normalized)?.code ?? ''
}

/** Map an i18next interface code (zhCN/zhTW) to a content language code. */
export function interfaceToContentLanguage(code?: string | null): string {
  if (code === 'zhCN') return 'zh-CN'
  if (code === 'zhTW') return 'zh-TW'
  return normalizeContentLanguage(code) || DEFAULT_CONTENT_LANGUAGE
}

/** Map a content language code back to the i18next interface code. */
export function contentToInterfaceLanguage(code?: string | null): string {
  const normalized = normalizeContentLanguage(code)
  if (normalized === 'zh-CN') return 'zhCN'
  if (normalized === 'zh-TW') return 'zhTW'
  return normalized || DEFAULT_CONTENT_LANGUAGE
}

/** Extract a language prefix from a URL path: "/tr/blog" -> { lang: "tr", strippedPath: "/blog" }. */
export function extractLanguagePrefix(pathname: string): {
  lang: string
  strippedPath: string
} {
  const match = pathname.match(/^\/([A-Za-z-]{2,5})(\/|$)/)
  if (!match) return { lang: '', strippedPath: pathname }
  const lang = normalizeContentLanguage(match[1])
  if (!lang) return { lang: '', strippedPath: pathname }
  const strippedPath =
    match[2] === '/' ? pathname.slice(match[1].length + 1) : '/'
  return { lang, strippedPath: strippedPath || '/' }
}
