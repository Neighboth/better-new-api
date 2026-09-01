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
import { Languages, Loader2, MessageSquare, Pencil, Plus, Sparkles, Trash2, Wand2 } from 'lucide-react'
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { INTERFACE_LANGUAGE_OPTIONS } from '@/i18n/languages'
import { api } from '@/lib/api'

import {
  fetchBlogPost,
  deleteBlogComment,
  type BlogComment,
} from '@/features/blog/api'
import { sendChatCompletion } from '@/features/playground/api'

import {
  BLOG_LOCALE_CODES,
  blogLocalizationsFromPayload,
  emptyBlogPostForm,
  hasAnyLocalizedContent,
  hasAnyTitleAndContent,
  type BlogLocalizedMap,
  type BlogPostForm,
} from '../lib/blog-post-form'
import {
  buildBlogAiSystemPrompt,
  parseBlogAiResponse,
} from '../lib/blog-ai'

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
  localizations?: {
    titles?: Record<string, string>
    summaries?: Record<string, string>
    contents?: Record<string, string>
    tags_list?: Record<string, string>
    seo_descriptions?: Record<string, string>
  }
}

export function BlogManager() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const [blogEnabled, setBlogEnabled] = useState<boolean | null>(null)
  const [editingPost, setEditingPost] = useState<BlogPostItem | null>(null)
  const [isEditorOpen, setIsEditorOpen] = useState(false)
  const [postForm, setPostForm] = useState<BlogPostForm>(emptyBlogPostForm())
  const [activeLanguage, setActiveLanguage] = useState('en')
  const [commentsPostId, setCommentsPostId] = useState<number | null>(null)
  const [isAiOpen, setIsAiOpen] = useState(false)
  const [aiPrompt, setAiPrompt] = useState('')
  const [aiRefinePrompt, setAiRefinePrompt] = useState('')
  const [aiModel, setAiModel] = useState('')
  const [aiGenerating, setAiGenerating] = useState(false)
  const [aiTranslating, setAiTranslating] = useState(false)

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
      const payload = buildSavePayload(postForm)
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
      setPostForm(emptyBlogPostForm())
      setActiveLanguage('en')
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
    setPostForm(emptyBlogPostForm())
    setActiveLanguage('en')
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
      ...blogLocalizationsFromPayload(post.localizations),
    })
    setActiveLanguage('en')
    setIsEditorOpen(true)
  }

  const posts = postsData?.items ?? []

  // All user-visible models (across every group the admin belongs to) for
  // the blog AI generator; no group param means the backend aggregates groups.
  const { data: aiModels } = useQuery({
    queryKey: ['admin-blog-ai-models'],
    queryFn: async () => {
      const res = await api.get('/api/user/models')
      if (!res.data?.success || !Array.isArray(res.data.data)) {
        return [] as string[]
      }
      return res.data.data as string[]
    },
    staleTime: 5 * 60 * 1000,
  })

  /** Run the AI generator: full generate for a brand-new post, refined edit
  * when a draft already exists. */
  const runAiGeneration = async (kind: 'generate' | 'refine') => {
    const prompt = kind === 'generate' ? aiPrompt.trim() : aiRefinePrompt.trim()
    const model = aiModel || aiModels?.[0] || ''
    if (!prompt) {
      toast.error(t('Enter a prompt first'))
      return
    }
    if (!model) {
      toast.error(t('No AI model available'))
      return
    }
    setAiGenerating(true)
    try {
      const hasDraft = hasAnyLocalizedContent(postForm)
      const systemParts: string[] = [buildBlogAiSystemPrompt()]
      if (kind === 'refine') {
        systemParts.push(
          'The admin already has a draft below (JSON, same schema. Edit/improve ONLY the requested parts of that draft and return the complete updated JSON (no missing keys).'
        )
        systemParts.push(`CURRENT DRAFT:${JSON.stringify(draftToAiJson(postForm))}`)
      } else {
        if (hasDraft) {
          systemParts.push(
            'You are also given a current draft below (JSON, same schema. Improve it per the user instructions and return the complete updated JSON with no missing keys.'
          )
          systemParts.push(`CURRENT DRAFT:${JSON.stringify(draftToAiJson(postForm))}`)
        }
      }
      let userRequest = prompt
      if (kind === 'generate') {
        userRequest = prompt.includes('\n') ? prompt : `Write a blog post about: ${prompt}`
      }

      const res = await sendChatCompletion({
        model,
        messages: [
          { role: 'system', content: systemParts.join('\n\n') },
          { role: 'user', content: userRequest },
        ],
        stream: false,
        temperature: 0.7,
      })
      const content = res.choices?.[0]?.message?.content
      const parsed = content ? parseBlogAiResponse(content) : null
      if (!parsed) {
        toast.error(t('AI returned an unparseable response. Try again.'))
        return
      }
      setPostForm((current) => ({
        ...current,
        ...parsed,
        title: parsed.titles.en || current.title,
        summary: parsed.summaries.en || current.summary,
        content: parsed.contents.en || current.content,
        tags: parsed.tags_list.en || current.tags,
        seo_description: parsed.seo_descriptions.en || current.seo_description,
      }))
      toast.success(kind === 'generate' ? t('Draft generated. Review or edit it below.') : t('Draft updated. Review or edit it below.'))
      if (kind === 'generate') {
        setAiPrompt('')
        setAiRefinePrompt('')
        setIsAiOpen(false)
        setIsEditorOpen(true)
      } else {
        setAiRefinePrompt('')
      }
    } catch {
      toast.error(t('AI generation failed'))
    } finally {
      setAiGenerating(false)
    }
  }

  const runAiTranslation = async () => {
    const model = aiModel || aiModels?.[0] || ''
    if (!model) {
      toast.error(t('No AI model available'))
      return
    }

    // Determine source language
    let srcLang = activeLanguage
    const getVal = (code: string, key: 'titles' | 'summaries' | 'contents' | 'tags_list' | 'seo_descriptions') =>
      (postForm[key][code] ?? '').trim()

    if (!getVal(srcLang, 'titles') && !getVal(srcLang, 'contents')) {
      // Find any language with content
      const found = BLOG_LOCALE_CODES.find((code) => getVal(code, 'titles') || getVal(code, 'contents'))
      if (found) {
        srcLang = found
      } else if (postForm.title || postForm.content) {
        srcLang = 'en'
      } else {
        toast.error(t('Please enter Title and Content in at least one language before translating.'))
        return
      }
    }

    const srcTitle = getVal(srcLang, 'titles') || postForm.title
    const srcSummary = getVal(srcLang, 'summaries') || postForm.summary
    const srcContent = getVal(srcLang, 'contents') || postForm.content
    const srcTags = getVal(srcLang, 'tags_list') || postForm.tags
    const srcSeo = getVal(srcLang, 'seo_descriptions') || postForm.seo_description

    if (!srcTitle || !srcContent) {
      toast.error(t('Please enter Title and Content in at least one language before translating.'))
      return
    }

    const sourcePayload: Record<string, string> = { title: srcTitle, content: srcContent }
    if (srcSummary) sourcePayload.summary = srcSummary
    if (srcTags) sourcePayload.tags = srcTags
    if (srcSeo) sourcePayload.seo_description = srcSeo

    const targetLangs = BLOG_LOCALE_CODES.filter((code) => code !== srcLang)

    setAiTranslating(true)
    try {
      const systemPrompt = [
        `You are an expert translator. The user provides blog post fields in source language code "${srcLang}".`,
        `Translate the provided fields into all target languages: ${targetLangs.join(', ')}.`,
        'CRITICAL RULES:',
        '1. ONLY translate the fields present in the source object. If a field (e.g. summary, tags, or seo_description) is NOT provided in the source object, DO NOT generate or translate it for target languages.',
        '2. Return ONLY a single valid JSON object matching this structure:',
        JSON.stringify({
          titles: targetLangs.reduce((acc, code) => ({ ...acc, [code]: '...' }), {}),
          summaries: srcSummary ? targetLangs.reduce((acc, code) => ({ ...acc, [code]: '...' }), {}) : undefined,
          contents: targetLangs.reduce((acc, code) => ({ ...acc, [code]: '...' }), {}),
          tags_list: srcTags ? targetLangs.reduce((acc, code) => ({ ...acc, [code]: '...' }), {}) : undefined,
          seo_descriptions: srcSeo ? targetLangs.reduce((acc, code) => ({ ...acc, [code]: '...' }), {}) : undefined,
        }),
      ].join('\n\n')

      const res = await sendChatCompletion({
        model,
        messages: [
          { role: 'system', content: systemPrompt },
          { role: 'user', content: JSON.stringify(sourcePayload) },
        ],
        stream: false,
        temperature: 0.3,
      })

      const content = res.choices?.[0]?.message?.content
      if (!content) {
        toast.error(t('Translation failed'))
        return
      }

      let text = content.trim()
      const fenceMatch = text.match(/```(?:json)?\s*([\s\S]*?)```/)
      if (fenceMatch) text = fenceMatch[1].trim()
      const parsed = JSON.parse(text)

      setPostForm((current) => {
        const nextTitles = { ...current.titles }
        const nextSummaries = { ...current.summaries }
        const nextContents = { ...current.contents }
        const nextTags = { ...current.tags_list }
        const nextSeo = { ...current.seo_descriptions }

        if (parsed.titles && typeof parsed.titles === 'object') {
          for (const code of targetLangs) {
            if (typeof parsed.titles[code] === 'string' && parsed.titles[code].trim()) {
              nextTitles[code] = parsed.titles[code].trim()
            }
          }
        }
        if (srcSummary && parsed.summaries && typeof parsed.summaries === 'object') {
          for (const code of targetLangs) {
            if (typeof parsed.summaries[code] === 'string' && parsed.summaries[code].trim()) {
              nextSummaries[code] = parsed.summaries[code].trim()
            }
          }
        }
        if (parsed.contents && typeof parsed.contents === 'object') {
          for (const code of targetLangs) {
            if (typeof parsed.contents[code] === 'string' && parsed.contents[code].trim()) {
              nextContents[code] = parsed.contents[code].trim()
            }
          }
        }
        if (srcTags && parsed.tags_list && typeof parsed.tags_list === 'object') {
          for (const code of targetLangs) {
            if (typeof parsed.tags_list[code] === 'string' && parsed.tags_list[code].trim()) {
              nextTags[code] = parsed.tags_list[code].trim()
            }
          }
        }
        if (srcSeo && parsed.seo_descriptions && typeof parsed.seo_descriptions === 'object') {
          for (const code of targetLangs) {
            if (typeof parsed.seo_descriptions[code] === 'string' && parsed.seo_descriptions[code].trim()) {
              nextSeo[code] = parsed.seo_descriptions[code].trim()
            }
          }
        }

        return {
          ...current,
          titles: nextTitles,
          summaries: nextSummaries,
          contents: nextContents,
          tags_list: nextTags,
          seo_descriptions: nextSeo,
        }
      })

      toast.success(t('Translation completed! You can review or edit each language tab before saving.'))
    } catch {
      toast.error(t('Translation failed'))
    } finally {
      setAiTranslating(false)
    }
  }

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
        <div className='flex items-center gap-2'>
        <Button type='button' size='sm' variant='outline' onClick={() => { setEditingPost(null); setPostForm(emptyBlogPostForm()); setActiveLanguage('en'); setIsAiOpen(true); }}>
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
          <div className='flex w-full flex-col-reverse gap-2 sm:flex-row sm:items-center sm:justify-between'>
            <Button
              type='button'
              variant='outline'
              className='w-full sm:w-auto'
              disabled={aiTranslating || savePost.isPending}
              onClick={runAiTranslation}
            >
              {aiTranslating ? (
                <Loader2 className='me-1 h-4 w-4 animate-spin' />
              ) : (
                <Languages className='me-1 h-4 w-4' />
              )}
              {t('Translate to other languages with AI')}
            </Button>
            <div className='flex w-full items-center justify-end gap-2 sm:w-auto'>
              <Button
                type='button'
                variant='outline'
                className='flex-1 sm:flex-none'
                onClick={() => setIsEditorOpen(false)}
              >
                {t('Cancel')}
              </Button>
              <Button
                type='button'
                className='flex-1 sm:flex-none'
                disabled={
                  savePost.isPending ||
                  !hasAnyTitleAndContent(postForm)
                }
                onClick={() => savePost.mutate()}
              >
                {t('Save')}
              </Button>
            </div>
          </div>
        }
      >
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
          <Label>{t('Published')}</Label>
          <div className='flex items-center gap-2'>
            <Switch
              checked={postForm.published}
              onCheckedChange={(checked) =>
                setPostForm({ ...postForm, published: checked })
              }
            />
            <span className='text-muted-foreground text-sm'>{t('Draft posts are hidden from the public site.')}</span>
          </div>
        </div>

        <Tabs value={activeLanguage} onValueChange={setActiveLanguage}>
          <TabsList className='flex w-full flex-wrap gap-1'>
            {INTERFACE_LANGUAGE_OPTIONS.map((lang) => (
              <TabsTrigger key={lang.code} value={lang.code} className='text-xs'>
                {lang.flag} {lang.code}
              </TabsTrigger>
            ))}
          </TabsList>

          {INTERFACE_LANGUAGE_OPTIONS.map((lang) => (
            <TabsContent key={lang.code} value={lang.code} className='mt-4 space-y-4'>
              <PostLanguageFields
                langCode={lang.code}
                languageLabel={lang.label}
                form={postForm}
                onChange={(patch: Partial<BlogPostForm>) =>
                  setPostForm((current) => ({ ...current, ...patch }))
                }
              />
            </TabsContent>
          ))}
        </Tabs>
      </Dialog>

      <Dialog
        open={isAiOpen}
        onOpenChange={setIsAiOpen}
        title={t('Generate blog post with AI')}
        contentClassName='max-w-xl'
        headerClassName='text-left'
        contentHeight='auto'
        bodyClassName='space-y-4'
        footer={
          <>
            <Button
              type='button'
              variant='ghost'
              onClick={() => setIsAiOpen(false)}
            >
              {t('Cancel')}
            </Button>
            {hasAnyLocalizedContent(postForm) && (
              <Button
                type='button'
                variant='outline'
                disabled={aiGenerating}
                onClick={() => runAiGeneration('refine')}
              >
                {aiGenerating ? (
                  <Loader2 className='h-4 w-4 animate-spin' />
                ) : (
                  <Wand2 className='h-4 w-4' />
                )}
                {t('Regenerate draft')}
              </Button>
            )}
            <Button
              type='button'
              disabled={aiGenerating}
              onClick={() => runAiGeneration('generate')}
            >
              {aiGenerating ? (
                <>
                  <Loader2 className='me-1 h-4 w-4 animate-spin' />
                  {t('Generating...')}
                </>
              ) : (
                <>
                  <Sparkles className='me-1 h-4 w-4' />
                  {t('Generate')}
                </>
              )}
            </Button>
          </>
        }
      >
        <div className='grid gap-2'>
          <Label>{t('Model')}</Label>
          <Select value={aiModel} onValueChange={(value) => setAiModel(value ?? '')}>
            <SelectTrigger>
              <SelectValue placeholder={t('Select a model')} />
            </SelectTrigger>
            <SelectContent>
              {(aiModels ?? []).map((model) => (
                <SelectItem key={model} value={model}>
                  {model}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className='grid gap-2'>
          <Label>{t('Prompt')}</Label>
          <Textarea
            rows={6}
            value={aiPrompt}
            placeholder={t('Describe the blog post you want, including topic, audience, tone, and any specific points to cover.')}
            onChange={(event) => setAiPrompt(event.target.value)}
          />
          <p className='text-muted-foreground text-xs'>
            {t('The AI writes title, summary, tags, SEO description,and full content for every supported language.')}
          </p>
        </div>
        {hasAnyLocalizedContent(postForm) && (
          <div className='grid gap-2'>
            <Label>{t('Refine draft')}</Label>
            <Textarea
              rows={3}
              value={aiRefinePrompt}
              placeholder={t('Optional: tell the AI what to change in the current draft, e.g. "Make the intro more technical".')}
              onChange={(event) => setAiRefinePrompt(event.target.value)}
            />
          </div>
        )}
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

/** Per-language editor fields for one blog post locale. */
function PostLanguageFields(props: {
  langCode: string
  languageLabel: string
  form: BlogPostForm
  onChange: (patch: Partial<BlogPostForm>) => void
}) {
  const { t } = useTranslation()

  const setLocalized = (key: 'titles' | 'summaries' | 'contents' | 'tags_list' | 'seo_descriptions', value: string) => {
    props.onChange({
      [key]: {
        ...props.form[key],
        [props.langCode]: value,
      },
    } as Partial<BlogPostForm>)
  }

  const langFlag =
    INTERFACE_LANGUAGE_OPTIONS.find((lang) => lang.code === props.langCode)?.flag ?? ''

  return (
    <div className='space-y-4'>
      <div className='grid gap-2'>
        <Label>
          <span className='me-1'>{langFlag}</span>
          {t('Title')}
        </Label>
        <Input
          value={props.form.titles[props.langCode] ?? ''}
          onChange={(event) => setLocalized('titles', event.target.value)}
        />
      </div>
      <div className='grid gap-2'>
        <Label>{t('Summary')}</Label>
        <Textarea
          rows={2}
          value={props.form.summaries[props.langCode] ?? ''}
          onChange={(event) => setLocalized('summaries', event.target.value)}
        />
      </div>
      <div className='grid gap-2'>
        <Label>{t('Tags (comma separated)')}</Label>
        <Input
          value={props.form.tags_list[props.langCode] ?? ''}
          placeholder={t('news, update')}
          onChange={(event) => setLocalized('tags_list', event.target.value)}
        />
      </div>
      <div className='grid gap-2'>
        <Label>{t('SEO description')}</Label>
        <Textarea
          rows={2}
          value={props.form.seo_descriptions[props.langCode] ?? ''}
          onChange={(event) => setLocalized('seo_descriptions', event.target.value)}
        />
      </div>
      <div className='grid gap-2'>
        <Label>{t('Content (Markdown)')}</Label>
        <Textarea
          rows={12}
          value={props.form.contents[props.langCode] ?? ''}
          className='font-mono text-xs'
          onChange={(event) => setLocalized('contents', event.target.value)}
        />
      </div>
      <p className='text-muted-foreground text-xs'>
        {props.langCode === 'en'
          ? t('The English version also acts as the fallback shown to readers when no localized version exists.') : t('Filled content is shown to readers who browse the site in this language.')}
      </p>
    </div>
  )
}

/** Serialize the form into the admin API payload, keeping the English values
  * mirrored into the legacy scalar fields so older consumers still work. */
function buildSavePayload(form: BlogPostForm) {
  const getFirstNonEmpty = (key: 'titles' | 'summaries' | 'contents' | 'tags_list' | 'seo_descriptions', scalar: string) => {
    if (form[key].en && form[key].en.trim()) return form[key].en.trim()
    if (scalar && scalar.trim()) return scalar.trim()
    for (const code of BLOG_LOCALE_CODES) {
      if (form[key][code] && form[key][code].trim()) return form[key][code].trim()
    }
    return ''
  }

  const title = getFirstNonEmpty('titles', form.title)
  const summary = getFirstNonEmpty('summaries', form.summary)
  const content = getFirstNonEmpty('contents', form.content)
  const tags = getFirstNonEmpty('tags_list', form.tags)
  const seo_description = getFirstNonEmpty('seo_descriptions', form.seo_description)
  const titleMap: BlogLocalizedMap = {}
  const summaryMap: BlogLocalizedMap = {}
  const contentMap: BlogLocalizedMap = {}
  const tagsMap: BlogLocalizedMap = {}
  const seoMap: BlogLocalizedMap = {}
  for (const code of BLOG_LOCALE_CODES) {
    const v = (field: 'titles' | 'summaries' | 'contents' | 'tags_list' | 'seo_descriptions') =>
      (form[field][code] ?? '').trim()
    if (v('titles')) titleMap[code] = v('titles')
    if (v('summaries')) summaryMap[code] = v('summaries')
    if (v('contents')) contentMap[code] = v('contents')
    if (v('tags_list')) tagsMap[code] = v('tags_list')
    if (v('seo_descriptions')) seoMap[code] = v('seo_descriptions')
  }

  return {
    title,
    summary,
    content,
    tags,
    seo_description,
    published: form.published,
    cover_image: form.cover_image,
    localizations: {
      titles: titleMap,
      summaries: summaryMap,
      contents: contentMap,
      tags_list: tagsMap,
      seo_descriptions: seoMap,
    },
  }
}

/** Snapshot the current localized draft into the JSON schema the AI consumes. */
function draftToAiJson(form: BlogPostForm) {
  const title = form.titles.en ?? form.title ?? ''
  const summary = form.summaries.en ?? form.summary ?? ''
  const content = form.contents.en ?? form.content ?? ''
  const tags = form.tags_list.en ?? form.tags ?? ''
  const seo_description = form.seo_descriptions.en ?? form.seo_description ?? ''
  const titleMap: BlogLocalizedMap = {}
  const summaryMap: BlogLocalizedMap = {}
  const contentMap: BlogLocalizedMap = {}
  const tagsMap: BlogLocalizedMap = {}
  const seoMap: BlogLocalizedMap = {}
  for (const code of BLOG_LOCALE_CODES) {
    titleMap[code] = form.titles[code] ?? (code === 'en' ? title : '')
    summaryMap[code] = form.summaries[code] ?? (code === 'en' ? summary : '')
    contentMap[code] = form.contents[code] ?? (code === 'en' ? content : '')
    tagsMap[code] = form.tags_list[code] ?? (code === 'en' ? tags : '')
    seoMap[code] = form.seo_descriptions[code] ?? (code === 'en' ? seo_description : '')
  }

  return {
    title: titleMap,
    summary: summaryMap,
    content: contentMap,
    tags: tagsMap,
    seo_description: seoMap,
  }
}
