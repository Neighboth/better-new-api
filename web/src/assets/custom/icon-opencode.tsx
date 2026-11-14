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
      viewBox='0 0 240 300'
      width={size}
      height={size}
      fill='none'
      className={className}
      {...props}
    >
      <path d='M180 240H60V120H180V240Z' fill='currentColor' opacity='0.45' />
      <path d='M180 60H60V240H180V60ZM240 300H0V0H240V300Z' fill='currentColor' />
    </svg>
  )
}
