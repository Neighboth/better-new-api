/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import type { SVGProps } from 'react'

type IconGroqProps = SVGProps<SVGSVGElement> & {
  size?: number
}

export function IconGroq({ size = 20, className, ...props }: IconGroqProps) {
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
        rx='5.25'
        fill='#F55036'
      />
      {/* The Groq "G" mark. */}
      <path
        d='M17.6 9.15A6.2 6.2 0 0 0 12.05 6.4a6.35 6.35 0 1 0 0 11.2 6.25 6.25 0 0 0 5.55-2.72'
        stroke='#FFFFFF'
        strokeWidth='1.9'
        strokeLinecap='round'
      />
      <path
        d='M13.1 10.9H17.4'
        stroke='#FFFFFF'
        strokeWidth='1.9'
        strokeLinecap='round'
      />
    </svg>
  )
}
