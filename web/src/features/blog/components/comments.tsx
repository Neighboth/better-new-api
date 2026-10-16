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
import { useMutation } from '@tanstack/react-query'
import dayjs from 'dayjs'
import { Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Avatar, AvatarFallback, AvatarImage } from '@/components/ui/avatar'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { useAuthStore } from '@/stores/auth-store'

import {
  buildCommentTree,
  createBlogComment,
  deleteBlogComment,
  type BlogComment,
} from '../api'
import { ReactionButtons } from './reaction-buttons'

type BlogCommentsProps = {
  postId: number
  comments: BlogComment[]
  reactions: Record<string, number>
  onChanged: () => void
}

export function BlogComments(props: BlogCommentsProps) {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const [newComment, setNewComment] = useState('')

  const comments = buildCommentTree(props.comments ?? [])

  const createMutation = useMutation({
    mutationFn: (input: { content: string; parentId?: number }) =>
      createBlogComment(props.postId, input.content, input.parentId),
    onSuccess: (res) => {
      if (!res.success) {
        toast.error(res.message || t('Failed to post comment'))
        return
      }
      toast.success(t('Comment posted'))
      setNewComment('')
      props.onChanged()
    },
    onError: () => toast.error(t('Failed to post comment')),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteBlogComment(props.postId, id),
    onSuccess: (res) => {
      if (!res.success) {
        toast.error(res.message || t('Failed to delete comment'))
        return
      }
      toast.success(t('Comment deleted'))
      props.onChanged()
    },
    onError: () => toast.error(t('Failed to delete comment')),
  })

  const submitNew = () => {
    if (!user) {
      toast.info(t('Please sign in to comment'))
      return
    }
    const content = newComment.trim()
    if (!content) return
    createMutation.mutate({ content })
  }

  return (
    <section className='space-y-4'>
      <h2 className='text-lg font-semibold'>
        {t('Comments')} ({comments.length})
      </h2>

      <div className='space-y-2'>
        <Textarea
          rows={3}
          placeholder={
            user
              ? t('Share your thoughts...')
              : t('Sign in to join the discussion')
          }
          value={newComment}
          onChange={(event) => setNewComment(event.target.value)}
          disabled={!user}
        />
        <div className='flex justify-end'>
          <Button
            type='button'
            size='sm'
            disabled={createMutation.isPending || !newComment.trim()}
            onClick={submitNew}
          >
            {t('Post comment')}
          </Button>
        </div>
      </div>

      {comments.length === 0 && (
        <p className='text-muted-foreground text-sm'>
          {t('No comments yet. Be the first!')}
        </p>
      )}
      {comments.length > 0 && (
        <ul className='space-y-4'>
          {comments.map((comment) => (
            <CommentItem
              key={comment.id}
              comment={comment}
              depth={0}
              postId={props.postId}
              reactions={props.reactions}
              currentUserId={user?.id}
              onReply={(parentId, content) =>
                createMutation.mutate({ content, parentId })
              }
              onDelete={(id) => deleteMutation.mutate(id)}
              onChanged={props.onChanged}
            />
          ))}
        </ul>
      )}
    </section>
  )
}

type CommentItemProps = {
  comment: BlogComment
  depth: number
  postId: number
  reactions: Record<string, number>
  currentUserId?: number
  onReply: (parentId: number, content: string) => void
  onDelete: (id: number) => void
  onChanged: () => void
}

function CommentItem(props: CommentItemProps) {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const [isReplying, setIsReplying] = useState(false)
  const [replyText, setReplyText] = useState('')

  const comment = props.comment
  const canDelete =
    props.currentUserId != null && props.currentUserId === comment.user_id

  const submitReply = () => {
    const content = replyText.trim()
    if (!content) return
    props.onReply(comment.id, content)
    setReplyText('')
    setIsReplying(false)
  }

  return (
    <li className={props.depth > 0 ? 'ms-8 border-s ps-4' : ''}>
      <div className='flex items-start gap-3'>
        <Avatar className='h-8 w-8'>
          <AvatarImage src={comment.avatar || undefined} alt={comment.username} />
          <AvatarFallback>
            {comment.username.slice(0, 2).toUpperCase()}
          </AvatarFallback>
        </Avatar>
        <div className='min-w-0 flex-1 space-y-1'>
          <div className='flex items-center gap-2 text-sm'>
            <span className='font-medium'>{comment.username}</span>
            <span className='text-muted-foreground text-xs'>
              {dayjs(comment.created_at).format('YYYY-MM-DD HH:mm')}
            </span>
          </div>
          <p className='text-sm break-words whitespace-pre-wrap'>
            {comment.content}
          </p>
          <div className='flex items-center gap-1'>
            <ReactionButtons
              targetType='comment'
              postId={props.postId}
              targetId={comment.id}
              likeCount={comment.like_count}
              dislikeCount={comment.dislike_count}
              myReaction={props.reactions[`comment:${comment.id}`] ?? 0}
              size='sm'
              onChanged={props.onChanged}
            />
            <Button
              type='button'
              variant='ghost'
              size='sm'
              className='h-7 px-2 text-xs'
              onClick={() => {
                if (!user) {
                  toast.info(t('Please sign in to reply'))
                  return
                }
                setIsReplying((open) => !open)
              }}
            >
              {t('Reply')}
            </Button>
            {canDelete && (
              <Button
                type='button'
                variant='ghost'
                size='sm'
                className='text-destructive h-7 px-2 text-xs'
                onClick={() => props.onDelete(comment.id)}
                aria-label={t('Delete comment')}
              >
                <Trash2 className='h-3.5 w-3.5' />
              </Button>
            )}
          </div>

          {isReplying && (
            <div className='space-y-2 pt-1'>
              <Textarea
                rows={2}
                value={replyText}
                onChange={(event) => setReplyText(event.target.value)}
                placeholder={t('Write a reply...')}
              />
              <div className='flex justify-end gap-2'>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  onClick={() => setIsReplying(false)}
                >
                  {t('Cancel')}
                </Button>
                <Button
                  type='button'
                  size='sm'
                  disabled={!replyText.trim()}
                  onClick={submitReply}
                >
                  {t('Reply')}
                </Button>
              </div>
            </div>
          )}

          {comment.replies && comment.replies.length > 0 && (
            <ul className='space-y-4 pt-3'>
              {comment.replies.map((reply) => (
                <CommentItem
                  key={reply.id}
                  comment={reply}
                  depth={props.depth + 1}
                  postId={props.postId}
                  reactions={props.reactions}
                  currentUserId={props.currentUserId}
                  onReply={props.onReply}
                  onDelete={props.onDelete}
                  onChanged={props.onChanged}
                />
              ))}
            </ul>
          )}
        </div>
      </div>
    </li>
  )
}
