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
import { Languages } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import {
  CONTENT_LANGUAGES,
  DEFAULT_CONTENT_LANGUAGE,
  normalizeContentLanguage,
} from '@/i18n/content-languages'
import { api } from '@/lib/api'

import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

type LocalizedSEO = {
  title_prefix: string
  description: string
  keywords: string
}

type MultilingualDefaults = {
  seoLanguages: string
  seoLocalized: string
  aboutBase: string
  aboutI18n: string
  userAgreementBase: string
  userAgreementI18n: string
  privacyPolicyBase: string
  privacyPolicyI18n: string
}

function parseMap(raw: string): Record<string, string> {
  if (!raw) return {}
  try {
    const parsed = JSON.parse(raw)
    return parsed && typeof parsed === 'object' ? parsed : {}
  } catch {
    return {}
  }
}

function parseLocalizedSEO(raw: string): Record<string, LocalizedSEO> {
  if (!raw) return {}
  try {
    const parsed = JSON.parse(raw)
    return parsed && typeof parsed === 'object' ? parsed : {}
  } catch {
    return {}
  }
}

function parseLanguages(raw: string): string[] {
  const langs = raw
    ? raw.split(',')
    : ['en', 'zh-CN', 'zh-TW', 'fr', 'ru', 'ja', 'vi']
  const out: string[] = [DEFAULT_CONTENT_LANGUAGE]
  for (const code of langs) {
    const normalized = normalizeContentLanguage(code)
    if (normalized && !out.includes(normalized)) out.push(normalized)
  }
  return out
}

/**
 * Language-scoped content: which languages the site advertises (hreflang,
 * AI blog translations), plus per-language SEO meta and legal/About texts.
 * Everything is optional; missing languages fall back to English and, for
 * public pages, to on-the-fly machine translation.
 */
export function MultilingualContentSection(props: {
  defaultValues: MultilingualDefaults
}) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const defaults = props.defaultValues

  const [languagesInput, setLanguagesInput] = useState(defaults.seoLanguages)
  const [seoMap, setSeoMap] = useState(() =>
    parseLocalizedSEO(defaults.seoLocalized)
  )
  const [aboutMap, setAboutMap] = useState(() => parseMap(defaults.aboutI18n))
  const [uaMap, setUaMap] = useState(() => parseMap(defaults.userAgreementI18n))
  const [ppMap, setPpMap] = useState(() => parseMap(defaults.privacyPolicyI18n))

  const languages = useMemo(
    () => parseLanguages(languagesInput),
    [languagesInput]
  )
  const [selectedLang, setSelectedLang] = useState('')

  const editableLanguages = useMemo(() => {
    const out = new Set(languages.filter((l) => l !== DEFAULT_CONTENT_LANGUAGE))
    for (const code of [
      ...Object.keys(seoMap),
      ...Object.keys(aboutMap),
      ...Object.keys(uaMap),
      ...Object.keys(ppMap),
    ]) {
      if (code !== DEFAULT_CONTENT_LANGUAGE) out.add(code)
    }
    return [...out].sort()
  }, [languages, seoMap, aboutMap, uaMap, ppMap])

  const activeLang =
    selectedLang && editableLanguages.includes(selectedLang)
      ? selectedLang
      : (editableLanguages[0] ?? '')

  const seoEntry: LocalizedSEO | undefined = seoMap[activeLang]

  const updateSeoField = (field: keyof LocalizedSEO, value: string) => {
    if (!activeLang) return
    setSeoMap((prev) => ({
      ...prev,
      [activeLang]: {
        title_prefix: prev[activeLang]?.title_prefix ?? '',
        description: prev[activeLang]?.description ?? '',
        keywords: prev[activeLang]?.keywords ?? '',
        [field]: value,
      },
    }))
  }

  const [translating, setTranslating] = useState<string | null>(null)

  const autoTranslate = async (field: 'seo' | 'about' | 'ua' | 'pp') => {
    if (!activeLang) return
    const docSources: Record<
      string,
      { source: string; apply: (text: string) => void }
    > = {
      about: {
        source: defaults.aboutBase,
        apply: (text) =>
          setAboutMap((prev) => ({ ...prev, [activeLang]: text })),
      },
      ua: {
        source: defaults.userAgreementBase,
        apply: (text) => setUaMap((prev) => ({ ...prev, [activeLang]: text })),
      },
      pp: {
        source: defaults.privacyPolicyBase,
        apply: (text) => setPpMap((prev) => ({ ...prev, [activeLang]: text })),
      },
    }

    setTranslating(field)
    try {
      if (field === 'seo') {
        const res = await api.post('/api/translate', {
          source: DEFAULT_CONTENT_LANGUAGE,
          target: activeLang,
          texts: [
            seoEntry?.title_prefix ?? '',
            seoEntry?.description ?? '',
            seoEntry?.keywords ?? '',
          ],
        })
        if (!res.data?.success) throw new Error(res.data?.message || 'failed')
        const texts = res.data.data.texts as string[]
        setSeoMap((prev) => ({
          ...prev,
          [activeLang]: {
            title_prefix: texts[0] ?? '',
            description: texts[1] ?? '',
            keywords: texts[2] ?? '',
          },
        }))
      } else {
        const entry = docSources[field]
        if (!entry.source.trim()) {
          toast.error(t('Write the English version first'))
          return
        }
        const res = await api.post('/api/translate', {
          source: DEFAULT_CONTENT_LANGUAGE,
          target: activeLang,
          texts: [entry.source],
        })
        if (!res.data?.success) throw new Error(res.data?.message || 'failed')
        entry.apply(res.data.data.texts[0] ?? '')
      }
      toast.success(t('Translation filled'))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('Translation failed'))
    } finally {
      setTranslating(null)
    }
  }

  const [saving, setSaving] = useState(false)
  const handleSave = async () => {
    setSaving(true)
    try {
      const cleanSeo: Record<string, LocalizedSEO> = {}
      for (const [lang, entry] of Object.entries(seoMap)) {
        if (entry.title_prefix || entry.description || entry.keywords) {
          cleanSeo[lang] = entry
        }
      }
      const cleanMap = (map: Record<string, string>) => {
        const out: Record<string, string> = {}
        for (const [lang, value] of Object.entries(map)) {
          if (value.trim()) out[lang] = value
        }
        return out
      }
      const updates: [string, string][] = [
        ['SEOLanguages', languagesInput.trim()],
        ['SEOLocalized', JSON.stringify(cleanSeo)],
        ['AboutI18n', JSON.stringify(cleanMap(aboutMap))],
        ['legal.user_agreement_i18n', JSON.stringify(cleanMap(uaMap))],
        ['legal.privacy_policy_i18n', JSON.stringify(cleanMap(ppMap))],
      ]
      for (const [key, value] of updates) {
        await updateOption.mutateAsync({ key, value })
      }
      toast.success(t('Settings saved'))
    } catch {
      toast.error(t('Failed to save settings'))
    } finally {
      setSaving(false)
    }
  }

  const langMeta = CONTENT_LANGUAGES.find((l) => l.code === activeLang)

  return (
    <SettingsSection title={t('Multilingual content')}>
      <div className='flex flex-col gap-4'>
        <div className='flex flex-col gap-2'>
          <Label>{t('Content languages')}</Label>
          <Input
            value={languagesInput}
            placeholder='en,zh-CN,zh-TW,fr,ru,ja,vi'
            onChange={(event) => setLanguagesInput(event.target.value)}
          />
          <p className='text-muted-foreground text-sm'>
            {t(
              'Comma-separated language codes the site advertises. Each gets a language-prefixed URL (e.g. /tr/...) with hreflang alternates, and AI blog drafts are translated into all of them. English is always the required base; everything else falls back to English when missing.'
            )}
          </p>
        </div>

        <div className='flex items-center gap-2'>
          <Label>{t('Language')}</Label>
          <Select
            value={activeLang}
            onValueChange={(value) => setSelectedLang(value ?? '')}
          >
            <SelectTrigger className='w-56'>
              <SelectValue placeholder={t('Select language')} />
            </SelectTrigger>
            <SelectContent className='max-h-72'>
              {editableLanguages.map((code) => {
                const meta = CONTENT_LANGUAGES.find((l) => l.code === code)
                return (
                  <SelectItem key={code} value={code}>
                    {meta ? `${meta.flag} ${meta.native}` : code}
                  </SelectItem>
                )
              })}
            </SelectContent>
          </Select>
        </div>

        {activeLang ? (
          <div className='flex flex-col gap-4 rounded-md border p-4'>
            <div className='flex items-center justify-between gap-2'>
              <p className='text-sm font-medium'>
                {langMeta ? `${langMeta.flag} ${langMeta.native}` : activeLang}
              </p>
              <p className='text-muted-foreground text-xs'>
                {t('Leave a field empty to fall back to English.')}
              </p>
            </div>

            <div className='flex flex-col gap-2'>
              <div className='flex items-center justify-between'>
                <Label>{t('SEO')}</Label>
                <Button
                  type='button'
                  size='sm'
                  variant='outline'
                  disabled={translating !== null}
                  onClick={() => autoTranslate('seo')}
                >
                  <Languages className='me-1 h-4 w-4' />
                  {translating === 'seo'
                    ? t('Translating...')
                    : t('Auto-translate')}
                </Button>
              </div>
              <Input
                value={seoEntry?.title_prefix ?? ''}
                placeholder={t('Title prefix (optional)')}
                onChange={(event) =>
                  updateSeoField('title_prefix', event.target.value)
                }
              />
              <Textarea
                rows={2}
                value={seoEntry?.description ?? ''}
                placeholder={t('Meta description')}
                onChange={(event) =>
                  updateSeoField('description', event.target.value)
                }
              />
              <Input
                value={seoEntry?.keywords ?? ''}
                placeholder={t('comma, separated, keywords')}
                onChange={(event) =>
                  updateSeoField('keywords', event.target.value)
                }
              />
            </div>

            <div className='flex flex-col gap-2'>
              <div className='flex items-center justify-between'>
                <Label>{t('About')}</Label>
                <Button
                  type='button'
                  size='sm'
                  variant='outline'
                  disabled={translating !== null}
                  onClick={() => autoTranslate('about')}
                >
                  <Languages className='me-1 h-4 w-4' />
                  {translating === 'about'
                    ? t('Translating...')
                    : t('Auto-translate')}
                </Button>
              </div>
              <Textarea
                rows={5}
                value={aboutMap[activeLang] ?? ''}
                placeholder={
                  defaults.aboutBase
                    ? t('Leave empty to use the English version')
                    : ''
                }
                onChange={(event) =>
                  setAboutMap((prev) => ({
                    ...prev,
                    [activeLang]: event.target.value,
                  }))
                }
              />
            </div>

            <div className='flex flex-col gap-2'>
              <div className='flex items-center justify-between'>
                <Label>{t('User Agreement')}</Label>
                <Button
                  type='button'
                  size='sm'
                  variant='outline'
                  disabled={translating !== null}
                  onClick={() => autoTranslate('ua')}
                >
                  <Languages className='me-1 h-4 w-4' />
                  {translating === 'ua'
                    ? t('Translating...')
                    : t('Auto-translate')}
                </Button>
              </div>
              <Textarea
                rows={5}
                value={uaMap[activeLang] ?? ''}
                onChange={(event) =>
                  setUaMap((prev) => ({
                    ...prev,
                    [activeLang]: event.target.value,
                  }))
                }
              />
            </div>

            <div className='flex flex-col gap-2'>
              <div className='flex items-center justify-between'>
                <Label>{t('Privacy Policy')}</Label>
                <Button
                  type='button'
                  size='sm'
                  variant='outline'
                  disabled={translating !== null}
                  onClick={() => autoTranslate('pp')}
                >
                  <Languages className='me-1 h-4 w-4' />
                  {translating === 'pp'
                    ? t('Translating...')
                    : t('Auto-translate')}
                </Button>
              </div>
              <Textarea
                rows={5}
                value={ppMap[activeLang] ?? ''}
                onChange={(event) =>
                  setPpMap((prev) => ({
                    ...prev,
                    [activeLang]: event.target.value,
                  }))
                }
              />
            </div>
          </div>
        ) : (
          <p className='text-muted-foreground text-sm'>
            {t('No additional languages configured yet.')}
          </p>
        )}

        <div>
          <Button
            type='button'
            onClick={handleSave}
            disabled={saving || updateOption.isPending}
          >
            {saving || updateOption.isPending ? t('Saving...') : t('Save')}
          </Button>
        </div>
      </div>
    </SettingsSection>
  )
}
