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
import { Check, Languages } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

import {
  CONTENT_LANGUAGES,
  contentToInterfaceLanguage,
  interfaceToContentLanguage,
} from '../i18n/content-languages'

export function LanguageSwitcher() {
  const { i18n, t } = useTranslation()
  const [query, setQuery] = useState('')
  const currentContentCode = interfaceToContentLanguage(i18n.language)

  const filtered = useMemo(() => {
    const needle = query.trim().toLowerCase()
    if (!needle) return CONTENT_LANGUAGES
    return CONTENT_LANGUAGES.filter(
      (lang) =>
        lang.name.toLowerCase().includes(needle) ||
        lang.native.toLowerCase().includes(needle) ||
        lang.code.toLowerCase().includes(needle)
    )
  }, [query])

  const handleChange = (contentCode: string) => {
    if (contentCode === currentContentCode) return
    const interfaceCode = contentToInterfaceLanguage(contentCode)
    try {
      localStorage.setItem('i18nextLng', interfaceCode)
    } catch {
      // storage unavailable; the URL prefix still pins the language
    }
    // Navigate to the language-prefixed URL so SEO stays language-scoped and
    // the server renders localized meta tags. The boot code in i18n/config.ts
    // strips the prefix again for the router.
    const path = window.location.pathname || '/'
    window.location.assign(
      `/${contentCode}${path}${window.location.search}${window.location.hash}`
    )
  }

  return (
    <DropdownMenu onOpenChange={(open) => !open && setQuery('')}>
      <DropdownMenuTrigger
        render={<Button variant='ghost' size='icon' className='rounded-full' />}
      >
        <Languages className='h-5 w-5' aria-hidden='true' />
        <span className='sr-only'>{t('Change language')}</span>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end' className='w-56'>
        <div className='p-1'>
          <input
            type='text'
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder={t('Search language')}
            aria-label={t('Search language')}
            className='border-input bg-background placeholder:text-muted-foreground focus:border-primary mb-1 w-full rounded-md border px-2 py-1 text-sm outline-none'
          />
        </div>
        <div className='max-h-72 overflow-y-auto'>
          {filtered.map((language) => (
            <DropdownMenuItem
              key={language.code}
              onClick={() => handleChange(language.code)}
            >
              <span aria-hidden='true' className='me-2'>
                {language.flag}
              </span>
              <span className='flex-1 truncate'>{language.native}</span>
              {language.code === currentContentCode && (
                <Check className='ms-2 h-4 w-4' aria-hidden='true' />
              )}
            </DropdownMenuItem>
          ))}
          {filtered.length === 0 && (
            <div className='text-muted-foreground px-2 py-3 text-center text-sm'>
              {t('No results found.')}
            </div>
          )}
        </div>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
