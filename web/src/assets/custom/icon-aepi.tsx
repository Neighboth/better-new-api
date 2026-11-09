/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import type { ImgHTMLAttributes } from 'react'

type IconAepiProps = Omit<ImgHTMLAttributes<HTMLImageElement>, 'src'> & {
  size?: number
}

export function IconAepi({ size = 20, className, style, ...props }: IconAepiProps) {
  return (
    <img
      src='/aepi-mark.svg'
      alt='Aepi Media'
      width={size}
      height={size}
      className={className}
      style={{
        width: size,
        height: size,
        objectFit: 'contain',
        display: 'inline-block',
        verticalAlign: 'middle',
        borderRadius: 4,
        ...style,
      }}
      {...props}
    />
  )
}
