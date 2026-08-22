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
import {
  Languages,
  MessageSquare,
  Pencil,
  Plus,
  Sparkles,
  Trash2,
} from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import {
  fetchBlogPost,
  deleteBlogComment,
  type BlogComment,
} from '@/features/blog/api'
import {
  CONTENT_LANGUAGES,
  DEFAULT_CONTENT_LANGUAGE,
} from '@/i18n/content-languages'
import { api } from '@/lib/api'

export type BlogTranslationForm = {
  title: string
  summary: string
  content: string
  seo_description: string
  tags: string
}

export type BlogPostItem = {
  id: number
  title: string
  summary: string
  content: string
  cover_image: string
  tags: string
  seo_description: string
  published: boolean
  translations?: string
  created_at: string
  updated_at: string
}

const emptyTranslation: BlogTranslationForm = {
  title: '',
  summary: '',
  content: '',
  seo_description: '',
  tags: '',
}

const emptyPostForm = {
  title: '',
  summary: '',
  content: '',
  cover_image: '',
  tags: '',
  seo_description: '',
  published: true,
  translations: {} as Record<string, BlogTranslationForm>,
}

function parseTranslations(raw?: string): Record<string, BlogTranslationForm> {
  if (!raw) return {}
  try {
    const parsed = JSON.parse(raw) as Record<
      string,
      Partial<BlogTranslationForm>
    >
    const out: Record<string, BlogTranslationForm> = {}
    for (const [lang, fields] of Object.entries(parsed)) {
      out[lang] = { ...emptyTranslation, ...fields }
    }
    return out
  } catch {
    return {}
  }
}

// Strip languages whose fields are all empty before persisting.
function compactTranslations(map: Record<string, BlogTranslationForm>) {
  const out: Record<string, BlogTranslationForm> = {}
  for (const [lang, fields] of Object.entries(map)) {
    if (Object.values(fields).some((value) => value.trim() !== '')) {
      out[lang] = fields
    }
  }
  return out
}

export function BlogManager() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const [blogEnabled, setBlogEnabled] = useState<boolean | null>(null)
  const [editingPost, setEditingPost] = useState<BlogPostItem | null>(null)
  const [isEditorOpen, setIsEditorOpen] = useState(false)
  const [postForm, setPostForm] = useState(emptyPostForm)
  const [activeLang, setActiveLang] = useState<string>(DEFAULT_CONTENT_LANGUAGE)
  const [commentsPostId, setCommentsPostId] = useState<number | null>(null)
  const [isAIDialogOpen, setIsAIDialogOpen] = useState(false)

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
      const payload = {
        ...postForm,
        translations: compactTranslations(postForm.translations),
      }
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
      setActiveLang(DEFAULT_CONTENT_LANGUAGE)
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
    setActiveLang(DEFAULT_CONTENT_LANGUAGE)
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
      translations: parseTranslations(post.translations),
    })
    setActiveLang(DEFAULT_CONTENT_LANGUAGE)
    setIsEditorOpen(true)
  }

  const setTranslationField = (
    lang: string,
    field: keyof BlogTranslationForm,
    value: string
  ) => {
    setPostForm({
      ...postForm,
      translations: {
        ...postForm.translations,
        [lang]: {
          ...(postForm.translations[lang] ?? emptyTranslation),
          [field]: value,
        },
      },
    })
  }

  const addLanguage = (lang: string) => {
    if (!postForm.translations[lang]) {
      setPostForm({
        ...postForm,
        translations: {
          ...postForm.translations,
          [lang]: { ...emptyTranslation },
        },
      })
    }
    setActiveLang(lang)
  }

  const removeLanguage = (lang: string) => {
    const next = { ...postForm.translations }
    delete next[lang]
    setPostForm({ ...postForm, translations: next })
    setActiveLang(DEFAULT_CONTENT_LANGUAGE)
  }

  const autoTranslateLang = useMutation({
    mutationFn: async (lang: string) => {
      const res = await api.post('/api/translate', {
        source: DEFAULT_CONTENT_LANGUAGE,
        target: lang,
        texts: [
          postForm.title,
          postForm.summary,
          postForm.content,
          postForm.seo_description,
          postForm.tags,
        ],
      })
      if (!res.data?.success) {
        throw new Error(res.data?.message || 'translate failed')
      }
      return res.data.data.texts as string[]
    },
    onSuccess: (texts, lang) => {
      setPostForm((prev) => ({
        ...prev,
        translations: {
          ...prev.translations,
          [lang]: {
            title: texts[0] ?? '',
            summary: texts[1] ?? '',
            content: texts[2] ?? '',
            seo_description: texts[3] ?? '',
            tags: texts[4] ?? '',
          },
        },
      }))
      toast.success(t('Translation filled'))
    },
    onError: (err) =>
      toast.error(err instanceof Error ? err.message : t('Translation failed')),
  })

  const availableLanguages = CONTENT_LANGUAGES.filter(
    (lang) => lang.code !== DEFAULT_CONTENT_LANGUAGE
  )
  const addedLanguages = Object.keys(postForm.translations)

  const posts = postsData?.items ?? []

  return (
    <div className='space-y-4 py-2'>
      <div className='flex flex-wrap items-center justify-between gap-3'>
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
        <div className='flex items-center gap-2'>
          <Button
            type='button'
            size='sm'
            variant='outline'
            onClick={() => setIsAIDialogOpen(true)}
          >
            <Sparkles className='me-1 h-4 w-4' />
            {t('Generate with AI')}
          </Button>
          <Button type='button' size='sm' onClick={openNewPost}>
            <Plus className='me-1 h-4 w-4' />
            {t('New post')}
          </Button>
        </div>
      </div>

      {isLoadingPosts && (
        <p className='text-muted-foreground text-sm'>{t('Loading...')}</p>
      )}
      {!isLoadingPosts && posts.length === 0 && (
        <p className='text-muted-foreground text-sm'>{t('No posts yet.')}</p>
      )}
      {!isLoadingPosts && posts.length > 0 && (
        <div className='divide-y rounded-md border'>
          {posts.map((post) => {
            const langCount = Object.keys(
              parseTranslations(post.translations)
            ).length
            return (
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
                    {langCount > 0 && (
                      <Badge variant='outline'>
                        <Languages className='me-1 h-3 w-3' />
                        {langCount}
                      </Badge>
                    )}
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
            )
          })}
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
        <div className='flex flex-wrap items-center gap-2'>
          <Button
            type='button'
            size='sm'
            variant={
              activeLang === DEFAULT_CONTENT_LANGUAGE ? 'default' : 'outline'
            }
            onClick={() => setActiveLang(DEFAULT_CONTENT_LANGUAGE)}
          >
            {t('English (base)')}
          </Button>
          {addedLanguages.map((lang) => {
            const meta = CONTENT_LANGUAGES.find((l) => l.code === lang)
            return (
              <Button
                key={lang}
                type='button'
                size='sm'
                variant={activeLang === lang ? 'default' : 'outline'}
                onClick={() => setActiveLang(lang)}
              >
                {meta ? `${meta.flag} ${meta.native}` : lang}
              </Button>
            )
          })}
          <Select value='' onValueChange={(lang) => lang && addLanguage(lang)}>
            <SelectTrigger className='h-8 w-40'>
              <SelectValue placeholder={t('Add language')} />
            </SelectTrigger>
            <SelectContent className='max-h-72'>
              {availableLanguages
                .filter((lang) => !addedLanguages.includes(lang.code))
                .map((lang) => (
                  <SelectItem key={lang.code} value={lang.code}>
                    {lang.flag} {lang.native}
                  </SelectItem>
                ))}
            </SelectContent>
          </Select>
        </div>

        {activeLang === DEFAULT_CONTENT_LANGUAGE ? (
          <>
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
                  setPostForm({
                    ...postForm,
                    seo_description: event.target.value,
                  })
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
          </>
        ) : (
          <>
            <div className='flex flex-wrap items-center justify-between gap-2'>
              <p className='text-muted-foreground text-sm'>
                {t(
                  'Optional translation. Empty fields fall back to the English version automatically.'
                )}
              </p>
              <div className='flex items-center gap-1'>
                <Button
                  type='button'
                  size='sm'
                  variant='outline'
                  disabled={autoTranslateLang.isPending}
                  onClick={() => autoTranslateLang.mutate(activeLang)}
                >
                  <Languages className='me-1 h-4 w-4' />
                  {autoTranslateLang.isPending
                    ? t('Translating...')
                    : t('Auto-translate from English')}
                </Button>
                <Button
                  type='button'
                  size='sm'
                  variant='ghost'
                  onClick={() => removeLanguage(activeLang)}
                >
                  <Trash2 className='me-1 h-4 w-4' />
                  {t('Remove language')}
                </Button>
              </div>
            </div>
            <div className='grid gap-2'>
              <Label>{t('Title')}</Label>
              <Input
                value={postForm.translations[activeLang]?.title ?? ''}
                onChange={(event) =>
                  setTranslationField(activeLang, 'title', event.target.value)
                }
              />
            </div>
            <div className='grid gap-2'>
              <Label>{t('Summary')}</Label>
              <Textarea
                rows={2}
                value={postForm.translations[activeLang]?.summary ?? ''}
                onChange={(event) =>
                  setTranslationField(activeLang, 'summary', event.target.value)
                }
              />
            </div>
            <div className='grid gap-2'>
              <Label>{t('Tags (comma separated)')}</Label>
              <Input
                value={postForm.translations[activeLang]?.tags ?? ''}
                onChange={(event) =>
                  setTranslationField(activeLang, 'tags', event.target.value)
                }
              />
            </div>
            <div className='grid gap-2'>
              <Label>{t('SEO description')}</Label>
              <Textarea
                rows={2}
                value={postForm.translations[activeLang]?.seo_description ?? ''}
                onChange={(event) =>
                  setTranslationField(
                    activeLang,
                    'seo_description',
                    event.target.value
                  )
                }
              />
            </div>
            <div className='grid gap-2'>
              <Label>{t('Content (Markdown)')}</Label>
              <Textarea
                rows={12}
                className='font-mono text-xs'
                value={postForm.translations[activeLang]?.content ?? ''}
                onChange={(event) =>
                  setTranslationField(activeLang, 'content', event.target.value)
                }
              />
            </div>
          </>
        )}
      </Dialog>

      <AIGenerateDialog
        open={isAIDialogOpen}
        onClose={() => setIsAIDialogOpen(false)}
        onDraft={(draft) => {
          setEditingPost(null)
          setPostForm({
            title: draft.title,
            summary: draft.summary,
            content: draft.content,
            cover_image: '',
            tags: draft.tags,
            seo_description: draft.seo_description,
            published: true,
            translations: draft.translations,
          })
          setActiveLang(DEFAULT_CONTENT_LANGUAGE)
          setIsAIDialogOpen(false)
          setIsEditorOpen(true)
        }}
      />

      <CommentsDialog
        postId={commentsPostId}
        onClose={() => setCommentsPostId(null)}
      />
    </div>
  )
}

type AIDraft = {
  title: string
  summary: string
  content: string
  seo_description: string
  tags: string
  translations: Record<string, BlogTranslationForm>
}

/** AI drafting dialog: pick a model, describe the post, review in the editor. */
function AIGenerateDialog(props: {
  open: boolean
  onClose: () => void
  onDraft: (draft: AIDraft) => void
}) {
  const { t } = useTranslation()
  const [model, setModel] = useState('')
  const [prompt, setPrompt] = useState('')

  const { data: models, isLoading: isLoadingModels } = useQuery({
    queryKey: ['admin-blog-ai-models'],
    queryFn: async () => {
      const res = await api.get('/api/blog/manage/ai/models')
      if (!res.data?.success) {
        throw new Error(res.data?.message || 'failed')
      }
      return res.data.data as string[]
    },
    enabled: props.open,
    staleTime: 5 * 60 * 1000,
  })

  const generate = useMutation({
    mutationFn: async () => {
      const res = await api.post('/api/blog/manage/ai/generate', {
        model,
        prompt,
      })
      if (!res.data?.success) {
        throw new Error(res.data?.message || t('Generation failed'))
      }
      return res.data.data as {
        title: string
        summary: string
        content: string
        seo_description: string
        tags: string
        translations?: Record<string, Partial<BlogTranslationForm>>
      }
    },
    onSuccess: (data) => {
      const translations: Record<string, BlogTranslationForm> = {}
      for (const [lang, fields] of Object.entries(data.translations ?? {})) {
        translations[lang] = { ...emptyTranslation, ...fields }
      }
      props.onDraft({
        title: data.title,
        summary: data.summary,
        content: data.content,
        seo_description: data.seo_description,
        tags: data.tags,
        translations,
      })
      toast.success(t('Draft generated. Review it before publishing.'))
    },
    onError: (err) =>
      toast.error(err instanceof Error ? err.message : t('Generation failed')),
  })

  return (
    <Dialog
      open={props.open}
      onOpenChange={(open) => !open && props.onClose()}
      title={t('Generate with AI')}
      contentClassName='max-w-xl'
      headerClassName='text-left'
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button type='button' variant='outline' onClick={props.onClose}>
            {t('Cancel')}
          </Button>
          <Button
            type='button'
            disabled={generate.isPending || !model || !prompt.trim()}
            onClick={() => generate.mutate()}
          >
            <Sparkles className='me-1 h-4 w-4' />
            {generate.isPending ? t('Generating...') : t('Generate')}
          </Button>
        </>
      }
    >
      <div className='grid gap-2'>
        <Label>{t('Model')}</Label>
        <Select value={model} onValueChange={(value) => setModel(value ?? '')}>
          <SelectTrigger>
            <SelectValue
              placeholder={
                isLoadingModels ? t('Loading...') : t('Select a model')
              }
            />
          </SelectTrigger>
          <SelectContent className='max-h-72'>
            {(models ?? []).map((name) => (
              <SelectItem key={name} value={name}>
                {name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <p className='text-muted-foreground text-xs'>
          {t('Lists the models served by your enabled channels.')}
        </p>
      </div>
      <div className='grid gap-2'>
        <Label>{t('What should the post cover?')}</Label>
        <Textarea
          rows={8}
          value={prompt}
          placeholder={t(
            'Describe the article in detail: topic, audience, tone, key points, things to mention or avoid...'
          )}
          onChange={(event) => setPrompt(event.target.value)}
        />
        <p className='text-muted-foreground text-xs'>
          {t(
            'The model writes the English post plus a full translation for every configured SEO language. You can edit everything before saving.'
          )}
        </p>
      </div>
    </Dialog>
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

  const comments: BlogComment[] = data?.success
    ? (data.data.comments ?? [])
    : []

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
              {comment.parent_id
                ? ` · ${t('Reply')} #${comment.parent_id}`
                : ''}
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
