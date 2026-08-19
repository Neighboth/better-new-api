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
import { ThumbsDown, ThumbsUp } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { useAuthStore } from '@/stores/auth-store'
import { cn } from '@/lib/utils'

import { reactToBlogComment, reactToBlogPost } from '../api'

type ReactionButtonsProps = {
  targetType: 'post' | 'comment'
  postId: number
  targetId: number
  likeCount: number
  dislikeCount: number
  myReaction: number
  size?: 'default' | 'sm'
  onChanged?: () => void
}

export function ReactionButtons(props: ReactionButtonsProps) {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)

  const react = async (value: 1 | -1) => {
    if (!user) {
      toast.info(t('Please sign in to react'))
      return
    }
    try {
      const res =
        props.targetType === 'post'
          ? await reactToBlogPost(props.postId, value)
          : await reactToBlogComment(props.postId, props.targetId, value)
      if (!res.success) {
        toast.error(res.message || t('Operation failed'))
        return
      }
      props.onChanged?.()
    } catch {
      toast.error(t('Operation failed'))
    }
  }

  const buttonSize = props.size === 'sm' ? 'h-7 gap-1 px-2 text-xs' : 'gap-1.5'
  const iconSize = props.size === 'sm' ? 'h-3.5 w-3.5' : 'h-4 w-4'

  return (
    <div className='flex items-center gap-1'>
      <Button
        type='button'
        variant='ghost'
        size='sm'
        className={cn(
          buttonSize,
          props.myReaction === 1 && 'text-primary'
        )}
        onClick={() => void react(1)}
        aria-label={t('Like')}
        aria-pressed={props.myReaction === 1}
      >
        <ThumbsUp
          className={cn(iconSize, props.myReaction === 1 && 'fill-current')}
        />
        {props.likeCount}
      </Button>
      <Button
        type='button'
        variant='ghost'
        size='sm'
        className={cn(
          buttonSize,
          props.myReaction === -1 && 'text-destructive'
        )}
        onClick={() => void react(-1)}
        aria-label={t('Dislike')}
        aria-pressed={props.myReaction === -1}
      >
        <ThumbsDown
          className={cn(iconSize, props.myReaction === -1 && 'fill-current')}
        />
        {props.dislikeCount}
      </Button>
    </div>
  )
}
