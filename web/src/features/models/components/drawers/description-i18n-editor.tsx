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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
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
} from '@/i18n/content-languages'
import { api } from '@/lib/api'

function parseI18nMap(raw: string): Record<string, string> {
  if (!raw) return {}
  try {
    const parsed = JSON.parse(raw)
    return parsed && typeof parsed === 'object' ? parsed : {}
  } catch {
    return {}
  }
}

/**
 * Per-language description editor for model metadata. The value is a JSON
 * string keyed by language code; empty entries are dropped on change so the
 * stored value stays compact.
 */
export function DescriptionI18nEditor(props: {
  value: string
  baseText: string
  onChange: (value: string) => void
}) {
  const { t } = useTranslation()
  const map = parseI18nMap(props.value)
  const languages = Object.keys(map).sort()
  const [selected, setSelected] = useState('')
  const [translating, setTranslating] = useState(false)

  const activeLang =
    selected && languages.includes(selected) ? selected : (languages[0] ?? '')

  const commit = (next: Record<string, string>) => {
    const cleaned: Record<string, string> = {}
    for (const [lang, text] of Object.entries(next)) {
      if (text.trim()) cleaned[lang] = text
    }
    props.onChange(Object.keys(cleaned).length ? JSON.stringify(cleaned) : '')
  }

  const addLanguage = (lang: string) => {
    commit({ ...map, [lang]: map[lang] ?? '' })
    setSelected(lang)
  }

  const removeLanguage = (lang: string) => {
    const next = { ...map }
    delete next[lang]
    commit(next)
  }

  const autoTranslate = async (lang: string) => {
    if (!props.baseText.trim()) {
      toast.error(t('Write the English version first'))
      return
    }
    setTranslating(true)
    try {
      const res = await api.post('/api/translate', {
        source: DEFAULT_CONTENT_LANGUAGE,
        target: lang,
        texts: [props.baseText],
      })
      if (!res.data?.success) throw new Error(res.data?.message || 'failed')
      commit({ ...map, [lang]: res.data.data.texts[0] ?? '' })
      toast.success(t('Translation filled'))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t('Translation failed'))
    } finally {
      setTranslating(false)
    }
  }

  const available = CONTENT_LANGUAGES.filter(
    (lang) =>
      lang.code !== DEFAULT_CONTENT_LANGUAGE && !languages.includes(lang.code)
  )

  return (
    <div className='flex flex-col gap-2 rounded-md border p-3'>
      <div className='flex items-center gap-2'>
        <Select value='' onValueChange={(lang) => lang && addLanguage(lang)}>
          <SelectTrigger className='h-8 w-48'>
            <SelectValue placeholder={t('Add language')} />
          </SelectTrigger>
          <SelectContent className='max-h-72'>
            {available.map((lang) => (
              <SelectItem key={lang.code} value={lang.code}>
                {lang.flag} {lang.native}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {languages.length === 0 && (
          <p className='text-muted-foreground text-xs'>
            {t(
              'No translations yet. Visitors in other languages see the main description.'
            )}
          </p>
        )}
      </div>
      {languages.length > 0 && (
        <div className='flex flex-wrap items-center gap-1'>
          {languages.map((lang) => {
            const meta = CONTENT_LANGUAGES.find((l) => l.code === lang)
            return (
              <Button
                key={lang}
                type='button'
                size='sm'
                variant={activeLang === lang ? 'default' : 'outline'}
                onClick={() => setSelected(lang)}
              >
                {meta ? `${meta.flag} ${meta.native}` : lang}
              </Button>
            )
          })}
        </div>
      )}
      {activeLang && (
        <div className='flex flex-col gap-2'>
          <Textarea
            rows={3}
            value={map[activeLang] ?? ''}
            placeholder={t('Translated description...')}
            onChange={(event) =>
              commit({ ...map, [activeLang]: event.target.value })
            }
          />
          <div className='flex items-center gap-1'>
            <Button
              type='button'
              size='sm'
              variant='outline'
              disabled={translating}
              onClick={() => autoTranslate(activeLang)}
            >
              <Languages className='me-1 h-4 w-4' />
              {translating ? t('Translating...') : t('Auto-translate')}
            </Button>
            <Button
              type='button'
              size='sm'
              variant='ghost'
              onClick={() => removeLanguage(activeLang)}
            >
              {t('Remove language')}
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
