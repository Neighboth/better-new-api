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

/**
 * OVH icon (simple-icons, CC0).
 * https://simpleicons.org/icons/ovh.svg
 */
import type { SVGProps } from 'react'

type IconOvhCloudProps = SVGProps<SVGSVGElement> & { size?: number }

export function IconOvhCloud({ size = 24, ...props }: IconOvhCloudProps) {
  return (
    <svg
      role='img'
      viewBox='0 0 24 24'
      xmlns='http://www.w3.org/2000/svg'
      width={size}
      height={size}
      fill='currentColor'
      {...props}
    >
      <path d='M19.881 10.095l2.563-4.45C23.434 7.389 24 9.404 24 11.555c0 2.88-1.017 5.523-2.71 7.594h-6.62l2.04-3.541h-2.696l3.176-5.513h2.691zm-2.32-5.243L9.333 19.14l.003.009H2.709C1.014 17.077 0 14.435 0 11.555c0-2.152.57-4.17 1.561-5.918L5.855 13.1 10.6 4.852h6.961z' />
    </svg>
  )
}
