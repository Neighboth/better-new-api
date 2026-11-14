/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import type { SVGProps } from 'react'

type IconOpenCodeProps = SVGProps<SVGSVGElement> & {
  size?: number
}

export function IconOpenCode({ size = 20, className, ...props }: IconOpenCodeProps) {
  return (
    <svg
      xmlns='http://www.w3.org/2000/svg'
      viewBox='0 0 24 24'
      width={size}
      height={size}
      fill='none'
      className={className}
      {...props}
    >
      <rect
        x='1.75'
        y='1.75'
        width='20.5'
        height='20.5'
        rx='5'
        fill='#111111'
        stroke='#FFFFFF'
        strokeOpacity='0.12'
        strokeWidth='1.5'
      />
      {/* Terminal prompt chevron + underscore: the OpenCode mark. */}
      <path
        d='M6.75 8.5L10.25 12L6.75 15.5'
        stroke='#FFFFFF'
        strokeWidth='1.7'
        strokeLinecap='round'
        strokeLinejoin='round'
      />
      <path
        d='M12.25 15.75H16.75'
        stroke='#FFFFFF'
        strokeWidth='1.7'
        strokeLinecap='round'
      />
    </svg>
  )
}
