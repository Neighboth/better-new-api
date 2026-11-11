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
import { useTranslation } from 'react-i18next'

/**
 * Keyboard-only "skip to main content" link.
 *
 * The link is invisible until it is focused with the keyboard. It used to be
 * hidden by translating it up by a fixed amount, which left a blue, square
 * "skip to main content" button floating over the announcement bars as soon as
 * the stack of bars grew taller than that offset. It now sits below the bars
 * (their height is published in `--announcement-bars-height`), keeps the
 * rounded shape of the other buttons and only appears on focus.
 */
export function SkipToMain() {
  const { t } = useTranslation()
  return (
    <a
      className='bg-primary text-primary-foreground hover:bg-primary/90 focus-visible:ring-ring pointer-events-none fixed start-4 z-999 -translate-y-[200%] rounded-md px-4 py-2 text-sm font-medium whitespace-nowrap opacity-0 shadow-md transition focus:pointer-events-auto focus:translate-y-2 focus:opacity-100 focus-visible:ring-2 focus-visible:outline-none'
      href='#content'
      style={{ top: 'var(--announcement-bars-height, 0px)' }}
    >
      {t('Skip to Main')}
    </a>
  )
}
