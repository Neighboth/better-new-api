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
import { useEffect, useRef } from 'react'
import type { DefaultValues, FieldValues, UseFormReturn } from 'react-hook-form'

/**
 * Reset a react-hook-form instance whenever the provided default values change.
 * Guards against naively resetting on every render by tracking the last
 * serialized snapshot of the defaults.
 *
 * A settings card refetches its options right after a save, and that refetch
 * used to reset the form even while the user was still editing: the fields
 * jumped back to the values of the previous response (the "save bounces back"
 * report). The form is now only reset when it carries no unsaved edit, i.e.
 * when its current values still match the previous defaults.
 */
export function useResetForm<TFieldValues extends FieldValues>(
  form: UseFormReturn<TFieldValues>,
  values: DefaultValues<TFieldValues> | undefined
) {
  const lastSerializedDefaults = useRef<string | null>(null)

  useEffect(() => {
    if (!values) return

    const serializedDefaults = JSON.stringify(values)
    if (serializedDefaults === lastSerializedDefaults.current) {
      return
    }

    const currentValues = form.getValues()
    const hasUnsavedEdits =
      lastSerializedDefaults.current !== null &&
      JSON.stringify(currentValues) !== lastSerializedDefaults.current

    lastSerializedDefaults.current = serializedDefaults

    if (hasUnsavedEdits && form.formState.isDirty) {
      // The user typed something that is not stored yet: keep it.
      return
    }

    form.reset(values)
  }, [values, form])
}
