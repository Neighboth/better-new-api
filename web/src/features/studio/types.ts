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
export type StudioMode = 'image' | 'video'

export type StudioItemStatus = 'pending' | 'ready' | 'error'

export type StudioItem = {
  id: string
  mode: StudioMode
  prompt: string
  createdAt: number
  status: StudioItemStatus
  /** Image or video source. */
  urls: string[]
  error?: string
  /** Raw progress string of a video task ("0%", "42%", "100%"). */
  progress?: string
  /** Settings used for the generation, shown under the result. */
  settings: Record<string, string | number | undefined>
}
