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
import { Check, Loader2, RotateCcw, Save } from 'lucide-react'
import {
  createContext,
  useContext,
  useEffect,
  useRef,
  useState,
  type ComponentProps,
  type ReactNode,
  type RefObject,
} from 'react'
import { createPortal } from 'react-dom'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

type SettingsPageContextValue = {
  actionsContainer: HTMLDivElement | null
  titleStatusContainer: HTMLSpanElement | null
  suppressSectionHeader: boolean
}

const SettingsPageContext = createContext<SettingsPageContextValue>({
  actionsContainer: null,
  titleStatusContainer: null,
  suppressSectionHeader: false,
})

type SettingsPageProviderProps = {
  actionsContainer: HTMLDivElement | null
  titleStatusContainer?: HTMLSpanElement | null
  children: ReactNode
  suppressSectionHeader?: boolean
}

export function SettingsPageProvider(props: SettingsPageProviderProps) {
  return (
    <SettingsPageContext.Provider
      value={{
        actionsContainer: props.actionsContainer,
        titleStatusContainer: props.titleStatusContainer ?? null,
        suppressSectionHeader: props.suppressSectionHeader ?? true,
      }}
    >
      {props.children}
    </SettingsPageContext.Provider>
  )
}

export function useSuppressSettingsSectionHeader() {
  return useContext(SettingsPageContext).suppressSectionHeader
}

type SettingsPageTitleStatusPortalProps = {
  children: ReactNode
}

export function SettingsPageTitleStatusPortal(
  props: SettingsPageTitleStatusPortalProps
) {
  const { titleStatusContainer } = useContext(SettingsPageContext)

  if (!titleStatusContainer) return null

  return createPortal(props.children, titleStatusContainer)
}

type SettingsPageActionsPortalProps = {
  children: ReactNode
}

export function SettingsPageActionsPortal(
  props: SettingsPageActionsPortalProps
) {
  const { actionsContainer } = useContext(SettingsPageContext)

  if (!actionsContainer) return null

  return createPortal(
    <div className='flex flex-wrap items-center justify-end gap-2'>
      {props.children}
    </div>,
    actionsContainer
  )
}

type SettingsPageFormActionsProps = {
  onSave: () => void | Promise<unknown>
  onReset?: () => void
  isSaving?: boolean
  isSaveDisabled?: boolean
  isResetDisabled?: boolean
  saveLabel?: string
  savingLabel?: string
  savedLabel?: string
  resetLabel?: string
  resetVariant?: ComponentProps<typeof Button>['variant']
  saveButtonRef?: RefObject<HTMLButtonElement | null>
}

export function SettingsPageFormActions(props: SettingsPageFormActionsProps) {
  const { t } = useTranslation()
  const [pending, setPending] = useState(false)
  const [justSaved, setJustSaved] = useState(false)
  const savedTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(
    () => () => {
      if (savedTimer.current) clearTimeout(savedTimer.current)
    },
    []
  )

  const isSaving = props.isSaving || pending
  const saveLabel = isSaving
    ? (props.savingLabel ?? 'Saving...')
    : justSaved
      ? (props.savedLabel ?? 'Saved')
      : (props.saveLabel ?? 'Save Changes')

  const handleSave = async () => {
    if (isSaving) return
    setPending(true)
    try {
      // The card's handler usually returns the mutation promise; waiting for it
      // lets the button show "Saved" only after the request really finished,
      // instead of flashing back to its idle label.
      await props.onSave()
      setJustSaved(true)
      if (savedTimer.current) clearTimeout(savedTimer.current)
      savedTimer.current = setTimeout(() => setJustSaved(false), 2500)
    } finally {
      setPending(false)
    }
  }

  return (
    <SettingsPageActionsPortal>
      {props.onReset && (
        <Button
          type='button'
          size='sm'
          variant={props.resetVariant ?? 'outline'}
          onClick={props.onReset}
          disabled={props.isResetDisabled || isSaving}
        >
          <RotateCcw data-icon='inline-start' />
          <span>{t(props.resetLabel ?? 'Reset')}</span>
        </Button>
      )}
      <Button
        ref={props.saveButtonRef}
        type='button'
        size='sm'
        onClick={() => void handleSave()}
        disabled={isSaving || props.isSaveDisabled}
        aria-live='polite'
      >
        {isSaving ? (
          <Loader2 className='animate-spin' data-icon='inline-start' />
        ) : justSaved ? (
          <Check data-icon='inline-start' />
        ) : (
          <Save data-icon='inline-start' />
        )}
        <span>{t(saveLabel)}</span>
      </Button>
    </SettingsPageActionsPortal>
  )
}
