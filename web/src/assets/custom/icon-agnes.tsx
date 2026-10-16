/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import type { SVGProps } from 'react'

type IconAgnesProps = SVGProps<SVGSVGElement> & {
  size?: number
}

export function IconAgnes({ size = 20, className, ...props }: IconAgnesProps) {
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
      <circle cx='12' cy='12' r='12' fill='#000000' />
      <path
        fill='#FFFFFF'
        fillRule='evenodd'
        d='M5.72 14.513s-1.183 3.606.999 3.969c2.182.362 5.8-.673 9.798-8.228 0 0-.494 2.373-.762 3.732a.884.884 0 01-.308.512.905.905 0 01-.567.204h-.533a.157.157 0 00-.088.288c.506.345 1.154 1.093 1.258 2.68a.486.486 0 00.239.387.502.502 0 00.459.029.824.824 0 00.38-.352c.085-.154.132-.325.136-.5l-.073-.661a.753.753 0 01.117-.48.769.769 0 01.39-.31c.474-.169 1.114-.524 1.427-1.251a.108.108 0 00-.076-.147.113.113 0 00-.062.003c-.317.112-.918.298-1.443.295a.26.26 0 01-.244-.185.253.253 0 01-.007-.108l.728-5.013a.482.482 0 00-.273-.512.5.5 0 00-.202-.048c-.2-.003-.4.026-.59.084-.51.153-.991.404-1.256.905-1.248 2.369-7.222 9.47-9.446 4.707zM17.604 7.027a1.203 1.203 0 10-2.376-.383 1.203 1.203 0 002.376.383z'
      />
    </svg>
  )
}
