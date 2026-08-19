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
import { Pencil, Plus, Trash2 } from 'lucide-react'
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
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

type BlogPostItem = {
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

type BlogSectionProps = {
  defaultValues: {
    BlogEnabled: boolean
    AdSenseClientId: string
    AdSenseSlotId: string
  }
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

export function BlogSection({ defaultValues }: BlogSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const queryClient = useQueryClient()

  const [blogEnabled, setBlogEnabled] = useState(defaultValues.BlogEnabled)
  const [adsenseClientId, setAdsenseClientId] = useState(
    defaultValues.AdSenseClientId
  )
  const [adsenseSlotId, setAdsenseSlotId] = useState(defaultValues.AdSenseSlotId)

  useEffect(() => {
    setBlogEnabled(defaultValues.BlogEnabled)
    setAdsenseClientId(defaultValues.AdSenseClientId)
    setAdsenseSlotId(defaultValues.AdSenseSlotId)
  }, [defaultValues])

  const [editingPost, setEditingPost] = useState<BlogPostItem | null>(null)
  const [isEditorOpen, setIsEditorOpen] = useState(false)
  const [postForm, setPostForm] = useState(emptyPostForm)

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

  const saveSetting = async (key: string, value: string) => {
    await updateOption.mutateAsync({ key, value })
  }

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
    <SettingsSection title={t('Blog & Ads')}>
      <SettingsForm onSubmit={(event) => event.preventDefault()}>
        <SettingsSwitchItem>
          <SettingsSwitchContent>
            <Label>{t('Enable blog')}</Label>
            <p className='text-muted-foreground text-sm'>
              {t(
                'Show a Blogs link in the header and serve public blog pages at /blog. Reading posts requires no account.'
              )}
            </p>
          </SettingsSwitchContent>
          <Switch
            checked={blogEnabled}
            onCheckedChange={(checked) => {
              setBlogEnabled(checked)
              void saveSetting('BlogEnabled', String(checked))
            }}
          />
        </SettingsSwitchItem>

        <div className='grid gap-2'>
          <Label htmlFor='adsense-client-id'>
            {t('AdSense publisher ID (ca-pub-...)')}
          </Label>
          <Input
            id='adsense-client-id'
            value={adsenseClientId}
            onChange={(event) => setAdsenseClientId(event.target.value)}
            placeholder='ca-pub-xxxxxxxxxxxxxxxx'
            onBlur={() => {
              if (adsenseClientId !== defaultValues.AdSenseClientId) {
                void saveSetting('AdSenseClientId', adsenseClientId)
              }
            }}
          />
          <p className='text-muted-foreground text-sm'>
            {t(
              'Ads show at the top and bottom of each blog post. Leave empty to disable ads.'
            )}
          </p>
        </div>

        <div className='grid gap-2'>
          <Label htmlFor='adsense-slot-id'>{t('AdSense ad slot ID')}</Label>
          <Input
            id='adsense-slot-id'
            value={adsenseSlotId}
            onChange={(event) => setAdsenseSlotId(event.target.value)}
            placeholder='1234567890'
            onBlur={() => {
              if (adsenseSlotId !== defaultValues.AdSenseSlotId) {
                void saveSetting('AdSenseSlotId', adsenseSlotId)
              }
            }}
          />
        </div>

        <div className='flex items-center justify-between pt-4'>
          <Label className='text-base'>{t('Blog posts')}</Label>
          <Button type='button' size='sm' onClick={openNewPost}>
            <Plus className='me-1 h-4 w-4' />
            {t('New post')}
          </Button>
        </div>

        {isLoadingPosts && (
          <p className='text-muted-foreground text-sm'>{t('Loading...')}</p>
        )}
        {!isLoadingPosts && posts.length === 0 && (
          <p className='text-muted-foreground text-sm'>
            {t('No posts yet.')}
          </p>
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
      </SettingsForm>

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
    </SettingsSection>
  )
}
