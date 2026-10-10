/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useId, type SVGProps } from 'react'

type IconAgnesProps = SVGProps<SVGSVGElement> & {
  size?: number
}

export function IconAgnes({ size = 20, ...props }: IconAgnesProps) {
  const gradientId = useId()
  const glowId = useId()

  return (
    <svg
      xmlns='http://www.w3.org/2000/svg'
      viewBox='0 0 32 32'
      width={size}
      height={size}
      fill='none'
      {...props}
    >
      <defs>
        <linearGradient
          id={gradientId}
          x1='4'
          y1='28'
          x2='28'
          y2='4'
          gradientUnits='userSpaceOnUse'
        >
          <stop stopColor='#8B5CF6' />
          <stop offset='0.5' stopColor='#EC4899' />
          <stop offset='1' stopColor='#3B82F6' />
        </linearGradient>
        <filter id={glowId} x='-20%' y='-20%' width='140%' height='140%'>
          <feGaussianBlur stdDeviation='1.5' result='blur' />
          <feComposite in='SourceGraphic' in2='blur' operator='over' />
        </filter>
      </defs>
      {/* Outer rounded container with subtle border */}
      <rect
        x='2'
        y='2'
        width='28'
        height='28'
        rx='7'
        fill='#0F172A'
      />
      {/* Agnes Stylized A with glowing gradient */}
      <path
        d='M16 6L7 25h4.2l2.1-4.8h5.4L20.8 25H25L16 6zm0 6.6l2.1 4.6h-4.2L16 12.6z'
        fill={`url(#${gradientId})`}
        filter={`url(#${glowId})`}
      />
      {/* Modern orbital ring arc symbolizing video generation & AI */}
      <path
        d='M9 13.5C9.8 11.2 12.6 9.5 16 9.5c4.1 0 7.4 2.4 7.9 5.5'
        stroke='#A855F7'
        strokeWidth='1.5'
        strokeLinecap='round'
        strokeDasharray='1 3'
        opacity='0.8'
      />
      <circle cx='23.5' cy='15' r='1.5' fill='#38BDF8' />
    </svg>
  )
}
