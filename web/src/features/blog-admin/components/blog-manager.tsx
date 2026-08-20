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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import dayjs from 'dayjs'
import { MessageSquare, Pencil, Plus, Trash2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { api } from '@/lib/api'

import {
  fetchBlogPost,
  deleteBlogComment,
  type BlogComment,
} from '@/features/blog/api'

export type BlogPostItem = {
  id: number
  title: string
  summary: string
  content: string
  cover_image: string
  tags: string
  seo_description: string
  published: boolean
  created_at: string
  updated_at: string
}

const emptyPostForm = {
  title: '',
  summary: '',
  content: '',
  cover_image: '',
  tags: '',
  seo_description: '',
  published: true,
}

export function BlogManager() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const [blogEnabled, setBlogEnabled] = useState<boolean | null>(null)
  const [editingPost, setEditingPost] = useState<BlogPostItem | null>(null)
  const [isEditorOpen, setIsEditorOpen] = useState(false)
  const [postForm, setPostForm] = useState(emptyPostForm)
  const [commentsPostId, setCommentsPostId] = useState<number | null>(null)

  // Blog on/off toggle lives on the blog admin API so regular admins (not
  // only the root user) can manage the blog.
  const { data: settings } = useQuery({
    queryKey: ['admin-blog-settings'],
    queryFn: async () => {
      const res = await api.get('/api/blog/manage/settings')
      if (!res.data?.success) {
        throw new Error(res.data?.message || 'failed')
      }
      return res.data.data as { enabled: boolean }
    },
  })

  useEffect(() => {
    if (settings) {
      setBlogEnabled(settings.enabled)
    }
  }, [settings])

  const toggleBlog = useMutation({
    mutationFn: async (enabled: boolean) =>
      api.put('/api/blog/manage/settings', { enabled }),
    onSuccess: (res, enabled) => {
      if (!res.data?.success) {
        toast.error(res.data?.message || t('Update failed'))
        return
      }
      setBlogEnabled(enabled)
      toast.success(enabled ? t('Blog enabled') : t('Blog disabled'))
      void queryClient.invalidateQueries({ queryKey: ['admin-blog-settings'] })
    },
    onError: () => toast.error(t('Update failed')),
  })

  const { data: postsData, isLoading: isLoadingPosts } = useQuery({
    queryKey: ['admin-blog-posts'],
    queryFn: async () => {
      const res = await api.get('/api/blog/manage/posts?page=1&page_size=100')
      if (!res.data?.success) {
        throw new Error(res.data?.message || 'failed')
      }
      return res.data.data as { items: BlogPostItem[]; total: number }
    },
  })

  const invalidatePosts = () =>
    queryClient.invalidateQueries({ queryKey: ['admin-blog-posts'] })

  const savePost = useMutation({
    mutationFn: async () => {
      const payload = { ...postForm }
      if (editingPost) {
        return api.put(`/api/blog/manage/posts/${editingPost.id}`, payload)
      }
      return api.post('/api/blog/manage/posts', payload)
    },
    onSuccess: (res) => {
      if (!res.data?.success) {
        toast.error(res.data?.message || t('Failed to save post'))
        return
      }
      toast.success(t('Post saved'))
      setIsEditorOpen(false)
      setEditingPost(null)
      setPostForm(emptyPostForm)
      void invalidatePosts()
    },
    onError: () => toast.error(t('Failed to save post')),
  })

  const deletePost = useMutation({
    mutationFn: (id: number) => api.delete(`/api/blog/manage/posts/${id}`),
    onSuccess: (res) => {
      if (!res.data?.success) {
        toast.error(res.data?.message || t('Failed to delete post'))
        return
      }
      toast.success(t('Post deleted'))
      void invalidatePosts()
    },
    onError: () => toast.error(t('Failed to delete post')),
  })

  const openNewPost = () => {
    setEditingPost(null)
    setPostForm(emptyPostForm)
    setIsEditorOpen(true)
  }

  const openEditPost = (post: BlogPostItem) => {
    setEditingPost(post)
    setPostForm({
      title: post.title,
      summary: post.summary,
      content: post.content,
      cover_image: post.cover_image,
      tags: post.tags,
      seo_description: post.seo_description,
      published: post.published,
    })
    setIsEditorOpen(true)
  }

  const posts = postsData?.items ?? []

  return (
    <div className='space-y-4 py-2'>
      <div className='flex items-center justify-between gap-3'>
        <div className='flex items-center gap-3'>
          <Switch
            checked={blogEnabled === true}
            disabled={blogEnabled === null || toggleBlog.isPending}
            onCheckedChange={(checked) => toggleBlog.mutate(checked)}
            aria-label={t('Enable blog')}
          />
          <div>
            <Label>{t('Enable blog')}</Label>
            <p className='text-muted-foreground text-sm'>
              {t('When off, public blog pages return an error.')}
            </p>
          </div>
        </div>
        <Button type='button' size='sm' onClick={openNewPost}>
          <Plus className='me-1 h-4 w-4' />
          {t('New post')}
        </Button>
      </div>

      {isLoadingPosts && (
        <p className='text-muted-foreground text-sm'>{t('Loading...')}</p>
      )}
      {!isLoadingPosts && posts.length === 0 && (
        <p className='text-muted-foreground text-sm'>{t('No posts yet.')}</p>
      )}
      {!isLoadingPosts && posts.length > 0 && (
        <div className='divide-y rounded-md border'>
          {posts.map((post) => (
            <div
              key={post.id}
              className='flex items-center justify-between gap-3 px-3 py-2'
            >
              <div className='min-w-0 flex-1'>
                <div className='flex items-center gap-2'>
                  <span className='truncate font-medium'>{post.title}</span>
                  <Badge variant={post.published ? 'default' : 'secondary'}>
                    {post.published ? t('Published') : t('Draft')}
                  </Badge>
                </div>
                <p className='text-muted-foreground truncate text-xs'>
                  {dayjs(post.created_at).format('YYYY-MM-DD HH:mm')}
                  {post.tags ? ` · ${post.tags}` : ''}
                </p>
              </div>
              <div className='flex items-center gap-1'>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  onClick={() => setCommentsPostId(post.id)}
                  aria-label={t('Comments')}
                >
                  <MessageSquare className='h-4 w-4' />
                </Button>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  onClick={() => openEditPost(post)}
                  aria-label={t('Edit post')}
                >
                  <Pencil className='h-4 w-4' />
                </Button>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  onClick={() => {
                    if (window.confirm(t('Delete this post?'))) {
                      deletePost.mutate(post.id)
                    }
                  }}
                  aria-label={t('Delete post')}
                >
                  <Trash2 className='h-4 w-4' />
                </Button>
              </div>
            </div>
          ))}
        </div>
      )}

      <Dialog
        open={isEditorOpen}
        onOpenChange={setIsEditorOpen}
        title={editingPost ? t('Edit post') : t('New post')}
        contentClassName='max-w-2xl'
        headerClassName='text-left'
        contentHeight='auto'
        bodyClassName='space-y-4'
        footer={
          <>
            <Button
              type='button'
              variant='outline'
              onClick={() => setIsEditorOpen(false)}
            >
              {t('Cancel')}
            </Button>
            <Button
              type='button'
              disabled={
                savePost.isPending ||
                !postForm.title.trim() ||
                !postForm.content.trim()
              }
              onClick={() => savePost.mutate()}
            >
              {t('Save')}
            </Button>
          </>
        }
      >
        <div className='grid gap-2'>
          <Label>{t('Title')}</Label>
          <Input
            value={postForm.title}
            onChange={(event) =>
              setPostForm({ ...postForm, title: event.target.value })
            }
          />
        </div>
        <div className='grid gap-2'>
          <Label>{t('Summary')}</Label>
          <Textarea
            rows={2}
            value={postForm.summary}
            onChange={(event) =>
              setPostForm({ ...postForm, summary: event.target.value })
            }
          />
        </div>
        <div className='grid gap-2'>
          <Label>{t('Cover image URL')}</Label>
          <Input
            value={postForm.cover_image}
            placeholder='https://...'
            onChange={(event) =>
              setPostForm({ ...postForm, cover_image: event.target.value })
            }
          />
        </div>
        <div className='grid gap-2'>
          <Label>{t('Tags (comma separated)')}</Label>
          <Input
            value={postForm.tags}
            placeholder={t('news, update')}
            onChange={(event) =>
              setPostForm({ ...postForm, tags: event.target.value })
            }
          />
        </div>
        <div className='grid gap-2'>
          <Label>{t('SEO description')}</Label>
          <Textarea
            rows={2}
            value={postForm.seo_description}
            onChange={(event) =>
              setPostForm({ ...postForm, seo_description: event.target.value })
            }
          />
        </div>
        <div className='grid gap-2'>
          <Label>{t('Content (Markdown)')}</Label>
          <Textarea
            rows={12}
            value={postForm.content}
            className='font-mono text-xs'
            onChange={(event) =>
              setPostForm({ ...postForm, content: event.target.value })
            }
          />
        </div>
        <div className='flex items-center gap-2'>
          <Switch
            checked={postForm.published}
            onCheckedChange={(checked) =>
              setPostForm({ ...postForm, published: checked })
            }
          />
          <Label>{t('Published')}</Label>
        </div>
      </Dialog>

      <CommentsDialog
        postId={commentsPostId}
        onClose={() => setCommentsPostId(null)}
      />
    </div>
  )
}

/** Admin comment moderation for one post: list + delete. */
function CommentsDialog(props: { postId: number | null; onClose: () => void }) {
  const { t } = useTranslation()

  const { data, isLoading, refetch } = useQuery({
    queryKey: ['admin-blog-comments', props.postId],
    queryFn: () => fetchBlogPost(String(props.postId)),
    enabled: props.postId !== null,
  })

  const deleteComment = useMutation({
    mutationFn: (commentId: number) =>
      deleteBlogComment(props.postId ?? 0, commentId),
    onSuccess: (res) => {
      if (!res.success) {
        toast.error(res.message || t('Failed to delete comment'))
        return
      }
      toast.success(t('Comment deleted'))
      void refetch()
    },
    onError: () => toast.error(t('Failed to delete comment')),
  })

  const comments: BlogComment[] = data?.success ? (data.data.comments ?? []) : []

  return (
    <Dialog
      open={props.postId !== null}
      onOpenChange={(open) => {
        if (!open) props.onClose()
      }}
      title={t('Comments')}
      contentClassName='max-w-xl'
      headerClassName='text-left'
      contentHeight='auto'
      bodyClassName='space-y-3'
    >
      {isLoading && (
        <p className='text-muted-foreground text-sm'>{t('Loading...')}</p>
      )}
      {!isLoading && comments.length === 0 && (
        <p className='text-muted-foreground text-sm'>
          {t('No comments yet. Be the first!')}
        </p>
      )}
      {comments.map((comment) => (
        <div
          key={comment.id}
          className='flex items-start justify-between gap-3 rounded-md border p-2'
        >
          <div className='min-w-0 flex-1'>
            <p className='text-xs font-medium'>
              {comment.username}
              {comment.parent_id ? ` · ${t('Reply')} #${comment.parent_id}` : ''}
              <span className='text-muted-foreground font-normal'>
                {' '}
                · {dayjs(comment.created_at).format('YYYY-MM-DD HH:mm')}
              </span>
            </p>
            <p className='mt-1 text-sm break-words whitespace-pre-wrap'>
              {comment.content}
            </p>
          </div>
          <Button
            type='button'
            variant='ghost'
            size='icon'
            disabled={deleteComment.isPending}
            onClick={() => {
              if (window.confirm(t('Delete comment'))) {
                deleteComment.mutate(comment.id)
              }
            }}
            aria-label={t('Delete comment')}
          >
            <Trash2 className='h-4 w-4' />
          </Button>
        </div>
      ))}
    </Dialog>
  )
}
