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
import { AlertCircle, KeyRound, Languages, Loader2, MessageSquare, Pencil, Plus, RefreshCw, Sparkles, Trash2 } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { ScopedApiDialog } from '@/components/scoped-api-dialog'
import { BlogMarkdown } from '@/features/blog/components/blog-markdown'
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
import { sendChatCompletion, getPlaygroundRuntimeConfig, searchWeb, fetchWebpage } from '@/features/playground/api'

import { createStreamRequestController } from '@/features/playground/hooks/use-stream-request'
import { getFreshAuthHeaders } from '@/lib/api'
import { API_ENDPOINTS } from '@/features/playground/constants'
import { SSE } from 'sse.js'
import type { StreamEventSource } from '@/features/playground/hooks/use-stream-request'

import {
  BLOG_LOCALE_CODES,
  blogLocalizationsFromPayload,
  emptyBlogPostForm,
  hasAnyTitleAndContent,
  type BlogLocalizedMap,
  type BlogPostForm,
} from '../lib/blog-post-form'
import {
  applyUserCasingGuarantees,
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
  const postFormRef = useRef<BlogPostForm>(postForm)
  useEffect(() => {
    postFormRef.current = postForm
  }, [postForm])
  const researchLogRef = useRef<HTMLDivElement | null>(null)
  const [activeLanguage, setActiveLanguage] = useState('en')
  const [commentsPostId, setCommentsPostId] = useState<number | null>(null)
  const [isBlogApiOpen, setIsBlogApiOpen] = useState(false)
  const [isAiOpen, setIsAiOpen] = useState(false)
  const [isAiGenerated, setIsAiGenerated] = useState(false)
  const [isRefineOpen, setIsRefineOpen] = useState(false)
  const [isRefining, setIsRefining] = useState(false)
  const [aiPrompt, setAiPrompt] = useState('')
  const [aiRefinePrompt, setAiRefinePrompt] = useState('')
  const [aiModel, setAiModel] = useState('')
  const [aiGenerating, setAiGenerating] = useState(false)
  const [aiTranslating, setAiTranslating] = useState(false)
  const [aiGenerationStep, setAiGenerationStep] = useState(0)
  const [aiGenerationError, setAiGenerationError] = useState(false)
  const [aiErrorMessage, setAiErrorMessage] = useState('')
  const [webResearch, setWebResearch] = useState(false)
  const [researchLogs, setResearchLogs] = useState<Array<{ id: string; kind: 'search' | 'fetch'; detail: string; ts: number }>>([])

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

  // The administrator decides whether the blog AI writer may research the web.
  const { data: runtimeConfig } = useQuery({
    queryKey: ['playground-runtime-config'],
    queryFn: () => getPlaygroundRuntimeConfig(),
    staleTime: 60_000,
  })
  const webResearchEnabled = Boolean(
    runtimeConfig?.web_search?.blog && runtimeConfig?.web_search?.configured
  )

  useEffect(() => {
    if (!webResearchEnabled) {
      setWebResearch(false)
      setResearchLogs([])
    }
  }, [webResearchEnabled])

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
    setIsAiGenerated(false)
    setIsEditorOpen(true)
  }

  const openEditPost = (post: BlogPostItem) => {
    setEditingPost(post)
    setIsAiGenerated(false)
    const localizations = blogLocalizationsFromPayload(post.localizations)
    if (!localizations.titles.en && post.title) localizations.titles.en = post.title
    if (!localizations.summaries.en && post.summary) localizations.summaries.en = post.summary
    if (!localizations.contents.en && post.content) localizations.contents.en = post.content
    if (!localizations.tags_list.en && post.tags) localizations.tags_list.en = post.tags
    if (!localizations.seo_descriptions.en && post.seo_description) localizations.seo_descriptions.en = post.seo_description

    setPostForm({
      title: post.title,
      summary: post.summary,
      content: post.content,
      cover_image: post.cover_image,
      tags: post.tags,
      seo_description: post.seo_description,
      published: post.published,
      ...localizations,
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
  const runAiStreamWithController = async (
    payload: any,
    onStart: () => void,
    onSuccess: (fullContent: string) => void,
    onError: (errStr?: string) => void,
    silent = false
  ) => {
    let text = ''
    let isFinished = false
    let inactivityTimer: ReturnType<typeof setTimeout> | null = null

    const clearTimer = () => {
      if (inactivityTimer) {
        clearTimeout(inactivityTimer)
        inactivityTimer = null
      }
    }

    onStart()
    const controller = createStreamRequestController({
      getHeaders: getFreshAuthHeaders,
      createSource: (reqPayload, headers) =>
        new SSE(API_ENDPOINTS.CHAT_COMPLETIONS, {
          headers,
          method: 'POST',
          payload: JSON.stringify(reqPayload),
        }) as StreamEventSource,
      setStreaming: () => {}, // unused
    })

    const resetTimer = () => {
      clearTimer()
      inactivityTimer = setTimeout(() => {
        if (!isFinished) {
          isFinished = true
          try {
            controller.stop()
          } catch {
            // ignore
          }
          const err = t('Generation timed out due to stream inactivity.')
          if (!silent) toast.error(err)
          onError(err)
        }
      }, 45000)
    }

    resetTimer()

    try {
      await controller.send(payload, {
        onUpdate: (type, chunk) => {
          resetTimer()
          if (type === 'content') {
            text += chunk
          }
        },
        onComplete: () => {
          if (isFinished) return
          isFinished = true
          clearTimer()
          onSuccess(text)
        },
        onError: (errStr) => {
          if (isFinished) return
          isFinished = true
          clearTimer()
          if (!silent) toast.error(errStr || t('Generation failed.'))
          onError(errStr)
        },
      })
    } catch (e: any) {
      if (isFinished) return
      isFinished = true
      clearTimer()
      if (!silent) toast.error(e.message || t('Generation failed.'))
      onError(e.message)
    }
  }

  const pushResearchLog = (kind: 'search' | 'fetch', detail: string) => {
    setResearchLogs((prev) =>
      [...prev, { id: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`, kind, detail, ts: Date.now() }].slice(-120)
    )
  }

  const parseQueryBounds = (raw: string): string[] => {
    try {
      const cleaned = raw
        .trim()
        .replace(/^```(?:json)?\s*/i, '')
        .replace(/```\s*$/i, '')
        .trim()
      const parsed = JSON.parse(cleaned) as unknown
      if (Array.isArray(parsed)) {
        return parsed.map((entry) => String(entry ?? '').trim()).filter((entry) => entry.length > 2)
      }
      if (parsed && typeof parsed === 'object') {
        for (const key of ['queries', 'search_queries', 'items', 'results']) {
          const value = (parsed as Record<string, unknown>)[key]
          if (Array.isArray(value)) {
            return value.map((entry) => String(entry ?? '').trim()).filter((entry) => entry.length > 2)
          }
        }
      }
      return []
    } catch {
      return []
    }
  }

  const runResearchPhase = async (prompt: string, model: string): Promise<string> => {
    setResearchLogs([])
    setAiGenerating(true)
    const trimmed = prompt.trim()

    // Phase 1 — ask the model to design its own research plan. The model
    // chooses how many queries to run and which angles to cover; the harness
    // only enforces a hard cap so the research cannot run away.
    const year = new Date().getFullYear()
    let queries: string[] = []
    try {
      const res = await sendChatCompletion(
        {
          model,
          messages: [
            {
              role: 'system',
              content: 'You output only valid JSON. No commentary. No markdown. No code fences.',
            },
            {
              role: 'user',
              content: `Topic: ${trimmed}\n\nYou are preparing to research the web for an authoritative, up-to-date blog post about this topic (year: ${year}). Design the search plan yourself: choose whatever queries you genuinely need, in your own words, at whatever angle you think a careful researcher would use. There is no fixed count and no checklist to fill. Reply with ONLY a JSON object shaped like {"queries": ["...", ...]}; each string is a single search query.`,
            },
          ],
          stream: false,
          temperature: 0.7,
          response_format: { type: 'json_object' },
        },
        undefined,
        60000
      )
      const raw = res?.choices?.[0]?.message?.content?.trim() ?? ''
      queries = parseQueryBounds(raw)
    } catch (error) {
      if (import.meta.env.DEV) console.warn('research query generation failed', error)
    }
    if (queries.length === 0) {
      // Last-resort fallback: just search the raw topic, once.
      queries = [trimmed]
    }
    queries = queries.slice(0, 3)

    // Phase 2 — search queries and gather candidate URLs.
    const candidates: Array<{ title: string; url: string; content: string }> = []
    const seen = new Set<string>()
    for (const query of queries) {
      pushResearchLog('search', query)
      try {
        const response = await searchWeb(query, 5)
        const hits = Array.isArray(response?.results) ? response.results : []
        for (const hit of hits) {
          const url = String(hit.url ?? '').trim()
          if (!url || seen.has(url) || !/^https?:\/\//i.test(url)) continue
          seen.add(url)
          candidates.push({
            title: String(hit.title ?? '').trim(),
            url,
            content: String(hit.content ?? '').trim(),
          })
        }
      } catch (error) {
        if (import.meta.env.DEV) console.warn('search failed for', query, error)
      }
    }

    // Phase 3 — fetch qualifying pages concurrently with 6s timeout, capped at 4 pages.
    const MAX_FETCH = 4
    const selectedHits = candidates.slice(0, MAX_FETCH)
    const pages: Array<{ title: string; url: string; markdown: string }> = []

    const fetchWithTimeout = async (hit: { title: string; url: string }) => {
      pushResearchLog('fetch', hit.url)
      const fetchPromise = fetchWebpage(hit.url)
      const timeoutPromise = new Promise<{ markdown?: string; title?: string }>((_, reject) =>
        setTimeout(() => reject(new Error('Fetch timeout')), 6000)
      )
      return Promise.race([fetchPromise, timeoutPromise])
    }

    const settled = await Promise.allSettled(
      selectedHits.map(async (hit) => {
        const page = await fetchWithTimeout(hit)
        const md = String(page.markdown ?? '')
        if (md.trim()) {
          return { title: page.title || hit.title || hit.url, url: hit.url, markdown: md.slice(0, 6000) }
        }
        return null
      })
    )

    for (const result of settled) {
      if (result.status === 'fulfilled' && result.value) {
        pages.push(result.value)
      }
    }

    // Phase 4 — compile the digest that every step of the draft receives.
    setAiGenerating(false)
    if (pages.length === 0) return ''
    const digest = pages
      .map(
        (page, index) =>
          `### Source ${index + 1}: ${page.title || page.url}\nURL: ${page.url}\n\n${page.markdown.slice(0, 5000)}`
      )
      .join('\n\n')
    return digest.slice(0, 30000)
  }

  const doAiSequenceStep = async (step: number, currentPrompt: string, kind: 'generate' | 'refine') => {
    const model = aiModel || aiModels?.[0] || ''
    setAiGenerationError(false)
    setAiErrorMessage('')
    setAiGenerationStep(step)

    let researchNote = ''
    if (webResearch && step === 1) {
      try {
        const digest = await runResearchPhase(currentPrompt, model)
        if (digest) {
          researchNote = `\n\nWeb research notes (fresh facts, prefer them over memory, cite these sources in the article):\n${digest.slice(0, 14000)}`
        }
      } catch (error: any) {
        // Research is best-effort: the post is still generated without it.
        console.error('web research failed', error)
      }
    }

    let currentDraft: BlogPostForm = {
      ...postFormRef.current,
      titles: { ...postFormRef.current.titles },
      summaries: { ...postFormRef.current.summaries },
      contents: { ...postFormRef.current.contents },
      tags_list: { ...postFormRef.current.tags_list },
      seo_descriptions: { ...postFormRef.current.seo_descriptions },
    }

    const doStream = async (systemMsg: string, userMsg: string, onUpdateObj: (parsed: any) => void): Promise<void> => {
      const attempt = async (silent: boolean): Promise<void> =>
        new Promise<void>((resolve, reject) => {
          const payload = {
            model,
            messages: [
              { role: 'system', content: systemMsg },
              { role: 'user', content: userMsg },
            ],
            stream: true,
            temperature: 0.7,
            response_format: { type: 'json_object' },
          }
          runAiStreamWithController(
            payload,
            () => setAiGenerating(true),
            (content) => {
              setAiGenerating(false)
              const parsed = content ? parseBlogAiResponse(content) : null
              if (!parsed) {
                reject(new Error('AI returned an unparseable response.'))
                return
              }
              onUpdateObj(parsed)
              resolve()
            },
            (errStr) => {
              setAiGenerating(false)
              reject(new Error(errStr || 'Generation failed.'))
            },
            silent
          )
        })

      let lastError: unknown = new Error('Generation failed.')
      for (let attemptNumber = 1; attemptNumber <= 3; attemptNumber++) {
        try {
          await attempt(attemptNumber < 3)
          return
        } catch (error) {
          lastError = error
          if (attemptNumber < 3) {
            await new Promise((resolve) => setTimeout(resolve, 1200 * attemptNumber))
          }
        }
      }
      throw lastError
    }

    const withRetry = async (fn: (attempt: number, silent: boolean) => Promise<void>, attempts = 3, delayBase = 1200): Promise<void> => {
      let lastError: unknown = new Error('Generation failed.')
      for (let attempt = 1; attempt <= attempts; attempt++) {
        try {
          await fn(attempt, attempt < attempts)
          return
        } catch (error) {
          lastError = error
          if (attempt < attempts) {
            await new Promise((resolve) => setTimeout(resolve, delayBase * attempt))
          }
        }
      }
      throw lastError
    }

    let curStep = step
    try {
      if (curStep <= 1) { // titles
        const sys = buildBlogAiSystemPrompt('titles')
        let req = (currentPrompt.includes("\n") ? currentPrompt : `Write a blog post about: ${currentPrompt}`) + researchNote
        if (kind === 'refine') {
           req = `Current Draft:\n${JSON.stringify(draftToAiJson(currentDraft))}\n\nInstructions:\n${currentPrompt}` + researchNote
        }
        await doStream(sys, req, (parsed) => {
          currentDraft = {
            ...currentDraft,
            titles: { ...currentDraft.titles, ...parsed.titles },
            title: parsed.titles?.en || currentDraft.title,
          }
          postFormRef.current = currentDraft
          setPostForm({ ...currentDraft })
        })
        curStep = 2
        setAiGenerationStep(2)
      }

      if (curStep <= 2) { // summaries
        const sys = buildBlogAiSystemPrompt('summaries')
        let req = `English Title: ${currentDraft.titles.en || currentDraft.title}\n\nUser Prompt: ${currentPrompt}` + researchNote

        if (kind === 'refine') {
           req = `Current Draft:\n${JSON.stringify(draftToAiJson(currentDraft))}\n\nInstructions:\n${currentPrompt}` + researchNote
        }
        await doStream(sys, req, (parsed) => {
          currentDraft = {
            ...currentDraft,
            summaries: { ...currentDraft.summaries, ...parsed.summaries },
            summary: parsed.summaries?.en || currentDraft.summary,
          }
          postFormRef.current = currentDraft
          setPostForm({ ...currentDraft })
        })
        curStep = 3
        setAiGenerationStep(3)
      }

      if (curStep <= 3) { // tags
        const sys = buildBlogAiSystemPrompt('tags')
        let req = `English Title: ${currentDraft.titles.en || currentDraft.title}\nEnglish Summary: ${currentDraft.summaries.en || currentDraft.summary}\n\nUser Prompt: ${currentPrompt}` + researchNote

        if (kind === 'refine') {
           req = `Current Draft:\n${JSON.stringify(draftToAiJson(currentDraft))}\n\nInstructions:\n${currentPrompt}` + researchNote
        }
        await doStream(sys, req, (parsed) => {
          currentDraft = {
            ...currentDraft,
            tags_list: { ...currentDraft.tags_list, ...parsed.tags_list },
            tags: parsed.tags_list?.en || currentDraft.tags,
          }
          postFormRef.current = currentDraft
          setPostForm({ ...currentDraft })
        })
        curStep = 4
        setAiGenerationStep(4)
      }

      if (curStep <= 4) { // seo
        const sys = buildBlogAiSystemPrompt('seo')
        let req = `English Title: ${currentDraft.titles.en || currentDraft.title}\nEnglish Summary: ${currentDraft.summaries.en || currentDraft.summary}\nEnglish Tags: ${currentDraft.tags_list.en || currentDraft.tags}\n\nUser Prompt: ${currentPrompt}` + researchNote

        if (kind === 'refine') {
           req = `Current Draft:\n${JSON.stringify(draftToAiJson(currentDraft))}\n\nInstructions:\n${currentPrompt}` + researchNote
        }
        await doStream(sys, req, (parsed) => {
          currentDraft = {
            ...currentDraft,
            seo_descriptions: { ...currentDraft.seo_descriptions, ...parsed.seo_descriptions },
            seo_description: parsed.seo_descriptions?.en || currentDraft.seo_description,
          }
          postFormRef.current = currentDraft
          setPostForm({ ...currentDraft })
        })
        curStep = 5
        setAiGenerationStep(5)
      }

      if (curStep <= 5) { // content_en
        const sys = buildBlogAiSystemPrompt('content_en')
        let req = `Title: ${currentDraft.titles.en || currentDraft.title}\nSummary: ${currentDraft.summaries.en || currentDraft.summary}\nTags: ${currentDraft.tags_list.en || currentDraft.tags}\nSEO Description: ${currentDraft.seo_descriptions.en || currentDraft.seo_description}\n\nUser Prompt: ${currentPrompt}` + researchNote

        if (kind === 'refine') {
           req = `Current Draft:\n${JSON.stringify(draftToAiJson(currentDraft))}\n\nInstructions:\n${currentPrompt}` + researchNote
        }
        await withRetry(async (_attempt, silent) => {
          await new Promise<void>((resolve, reject) => {
            const payload = {
              model,
              messages: [
                { role: 'system', content: sys },
                { role: 'user', content: req },
              ],
              stream: true,
              temperature: 0.7,
            }
            runAiStreamWithController(
              payload,
              () => setAiGenerating(true),
              (content) => {
                setAiGenerating(false)
                let text = (content || '').trim()
                text = text.replace(/<think>[\s\S]*?<\/think>/g, '').trim()
                const fenceMatch = text.match(/^```(?:markdown)?\s*([\s\S]*?)```$/)
                if (fenceMatch) text = fenceMatch[1].trim()

                if (!text) {
                  reject(new Error('AI returned an empty response.'))
                  return
                }

                const guaranteedText = applyUserCasingGuarantees(text, currentPrompt, 'en')

                currentDraft = {
                  ...currentDraft,
                  contents: { ...currentDraft.contents, en: guaranteedText },
                  content: guaranteedText,
                }
                postFormRef.current = currentDraft
                setPostForm({ ...currentDraft })
                resolve()
              },
              (errStr) => {
                setAiGenerating(false)
                reject(new Error(errStr || 'Generation failed.'))
              },
              silent
            )
          })
        })
        curStep = 6
        setAiGenerationStep(6)
      }

      if (curStep >= 6) { // content_translate loop
        const targetLangs = BLOG_LOCALE_CODES.filter((code) => code !== 'en')
        let currentLangIdx = curStep > 6 ? curStep - 6 : 0
        const enContent = currentDraft.contents.en || currentDraft.content

        if (enContent) {
          for (let i = currentLangIdx; i < targetLangs.length; i++) {
            const langCode = targetLangs[i]
            setAiGenerationStep(6 + i)
            const sys = buildBlogAiSystemPrompt('content_translate')
            const req = `Target Language Code: ${langCode}\nUser Formatting / Casing Instructions: ${currentPrompt}\n\nEnglish Content:\n${enContent}`

            try {
              await withRetry(async (_attempt, silent) => {
                await new Promise<void>((resolve, reject) => {
                const payload = {
                  model,
                  messages: [
                    { role: 'system', content: sys },
                    { role: 'user', content: req },
                  ],
                  stream: true,
                  temperature: 0.3,
                }
                runAiStreamWithController(
                  payload,
                  () => setAiGenerating(true),
                  (content) => {
                    setAiGenerating(false)
                    let text = (content || '').trim()
                    text = text.replace(/<think>[\s\S]*?<\/think>/g, '').trim()
                    const fenceMatch = text.match(/^```(?:markdown)?\s*([\s\S]*?)```$/)
                    if (fenceMatch) text = fenceMatch[1].trim()

                    if (!text) {
                      reject(new Error(`AI returned an empty response for ${langCode}.`))
                      return
                    }

                    const guaranteedText = applyUserCasingGuarantees(text, currentPrompt, langCode)

                    currentDraft = {
                      ...currentDraft,
                      contents: { ...currentDraft.contents, [langCode]: guaranteedText },
                    }
                    postFormRef.current = currentDraft
                    setPostForm({ ...currentDraft })
                    resolve()
                  },
                  (errStr) => {
                    setAiGenerating(false)
                    reject(new Error(errStr || `Generation failed for ${langCode}`))
                  },
                  silent
                )
                })
              })
            } catch (err: any) {
              console.warn(`Translation for ${langCode} failed, continuing with generated content:`, err)
              toast.warning(t('Translation for {{lang}} could not be completed, continuing...', { lang: langCode }))
              if (!currentDraft.contents?.[langCode] && enContent) {
                currentDraft = {
                  ...currentDraft,
                  contents: { ...currentDraft.contents, [langCode]: enContent },
                }
                postFormRef.current = currentDraft
                setPostForm({ ...currentDraft })
              }
            }

            if (i < targetLangs.length - 1) {
              setAiGenerationStep(6 + i + 1)
            }
          }
        }
      }

      // Finished
      toast.success(kind === 'generate' ? t('Draft generated. Review or edit it below.') : t('Draft updated. Review or edit it below.'))
      setAiGenerationStep(0)
      setAiGenerationError(false)
      setAiErrorMessage('')
      setAiPrompt('')
      setAiRefinePrompt('')
      setIsAiGenerated(true)
      setIsAiOpen(false)
      // The run is finished: open the editor with the drafted post.
      setIsEditorOpen(true)
    } catch (e: any) {
      // 3 attempts already ran silently before this point; surface the failure once.
      const message = (e && e.message) ? String(e.message) : t('Generation failed.')
      setAiGenerationError(true)
      setAiErrorMessage(message)
      toast.error(message)
    }
  }

  const runAiRefine = async () => {
    const promptInstruction = aiRefinePrompt.trim()
    if (!promptInstruction) {
      toast.error(t('Enter instructions first'))
      return
    }
    const model = aiModel || aiModels?.[0] || 'gpt-4o'
    setIsRefineOpen(false)
    setIsRefining(true)
    try {
      const currentDraft = postFormRef.current
      const currentEnContent = currentDraft.contents.en || currentDraft.content || ''
      const currentEnTitle = currentDraft.titles.en || currentDraft.title || ''
      const currentEnDesc = currentDraft.summaries.en || currentDraft.summary || ''
      const currentEnTags = currentDraft.tags_list.en || currentDraft.tags || ''

      const sys = `You are an expert multilingual blog post writer and editor. Modify the existing blog post draft based on the user's instructions.
CRITICAL STYLE, CASING & PUNCTUATION RULE: If the user instructions specify casing rules (such as ALL UPPERCASE/tamamen büyük harf, all lowercase) or punctuation rules (no punctuation/noktalama olmasın), you MUST strictly and aggressively enforce it on the English title, content, summary, tags, and on ALL localized translations in the translations object! For example, if all uppercase is requested, every character in title, content, description, tags, and every localized field must be UPPERCASE without exception. Never revert to normal sentence casing in summaries or titles!
CRITICAL COMPLETENESS RULE: Output the FULL complete content. Never truncate and never use ellipses (...) to omit text!
CRITICAL LANGUAGE RULE: Each localized title, description, and tags in "translations" must be written in that specific NATIVE language.
Return ONLY a valid JSON object matching this structure:
{
  "title": "refined english title",
  "content": "refined markdown content in english",
  "description": "refined short summary in english",
  "tags": "refined comma separated tags in english",
  "translations": {
    "tr": { "title": "...", "description": "...", "tags": "..." },
    "zh": { "title": "...", "description": "...", "tags": "..." },
    "zh-TW": { "title": "...", "description": "...", "tags": "..." },
    "fr": { "title": "...", "description": "...", "tags": "..." },
    "ru": { "title": "...", "description": "...", "tags": "..." },
    "ja": { "title": "...", "description": "...", "tags": "..." },
    "vi": { "title": "...", "description": "...", "tags": "..." }
  }
}`
      const req = `Current Title:\n${currentEnTitle}\n\nCurrent Summary:\n${currentEnDesc}\n\nCurrent Content:\n${currentEnContent}\n\nCurrent Tags:\n${currentEnTags}\n\nUser Instructions for Changes:\n${promptInstruction}`

      const res = await sendChatCompletion({
        model,
        stream: false,
        temperature: 0.7,
        messages: [
          { role: 'system', content: sys },
          { role: 'user', content: req },
        ],
        response_format: { type: 'json_object' },
      })
      const content = res.choices?.[0]?.message?.content
      if (content) {
        const parsed = JSON.parse(content)
        const refinedEnContent = parsed.content || currentEnContent

        const updatedPost: BlogPostForm = {
          ...postFormRef.current,
          titles: { ...postFormRef.current.titles },
          summaries: { ...postFormRef.current.summaries },
          contents: { ...postFormRef.current.contents },
          tags_list: { ...postFormRef.current.tags_list },
          seo_descriptions: { ...postFormRef.current.seo_descriptions },
        }

        if (parsed.title) {
          const t = applyUserCasingGuarantees(parsed.title, promptInstruction, 'en')
          updatedPost.title = t
          updatedPost.titles.en = t
        }
        if (refinedEnContent) {
          const c = applyUserCasingGuarantees(refinedEnContent, promptInstruction, 'en')
          updatedPost.content = c
          updatedPost.contents.en = c
        }
        if (parsed.description) {
          const d = applyUserCasingGuarantees(parsed.description, promptInstruction, 'en')
          updatedPost.summary = d
          updatedPost.summaries.en = d
          updatedPost.seo_description = d
          updatedPost.seo_descriptions.en = d
        }
        if (parsed.tags) {
          const tg = applyUserCasingGuarantees(parsed.tags, promptInstruction, 'en')
          updatedPost.tags = tg
          updatedPost.tags_list.en = tg
        }

        if (parsed.translations && typeof parsed.translations === 'object') {
          for (const code of BLOG_LOCALE_CODES) {
            if (code === 'en') continue
            const tData = parsed.translations[code]
            if (tData) {
              if (tData.title) updatedPost.titles[code] = applyUserCasingGuarantees(tData.title, promptInstruction, code)
              if (tData.description) {
                const desc = applyUserCasingGuarantees(tData.description, promptInstruction, code)
                updatedPost.summaries[code] = desc
                updatedPost.seo_descriptions[code] = desc
              }
              if (tData.tags) updatedPost.tags_list[code] = applyUserCasingGuarantees(tData.tags, promptInstruction, code)
            }
          }
        }

        // Translate the full refined content for each target language sequentially so token limits are never exceeded
        const nonEnLangs = BLOG_LOCALE_CODES.filter((code) => code !== 'en')
        for (const code of nonEnLangs) {
          try {
            const transRes = await sendChatCompletion({
              model,
              stream: false,
              temperature: 0.3,
              messages: [
                {
                  role: 'system',
                  content: `You are an expert technical translator. Translate the given markdown article into the target language (${code}).
CRITICAL RULES:
1. Output MUST be entirely in the target language (${code}). Never keep it in English or Turkish unless the target is that language.
2. The user has given these refinement instructions: "${promptInstruction}". You MUST strictly follow these rules (for example, if instructed to make ALL UPPERCASE, all lowercase, or no punctuation, apply it strictly to this entire translated article without exception!).
3. Generate the FULL complete markdown article without omitting any section or paragraph. NEVER truncate and NEVER output ellipses (...) to skip text!
4. Output ONLY the raw Markdown article directly without any code fences or conversational prefaces/postfaces.`,
                },
                {
                  role: 'user',
                  content: `Target Language Code: ${code}\nRefinement Rule: ${promptInstruction}\n\nEnglish Content:\n${refinedEnContent}`,
                },
              ],
            })
            let translated = transRes.choices?.[0]?.message?.content?.trim() || ''
            translated = translated.replace(/<think>[\s\S]*?<\/think>/g, '').trim()
            const fenceMatch = translated.match(/^```(?:markdown)?\s*([\s\S]*?)```$/)
            if (fenceMatch) translated = fenceMatch[1].trim()
            if (translated) {
              updatedPost.contents[code] = applyUserCasingGuarantees(translated, promptInstruction, code)
            }
          } catch (err) {
            console.error(`Failed to refine/translate content for ${code}:`, err)
          }
        }

        postFormRef.current = updatedPost
        setPostForm({ ...updatedPost })
        toast.success(t('Draft updated successfully and applied to all languages'))
        setAiRefinePrompt('')
      }
    } catch (err: any) {
      toast.error(err?.message || t('Failed to update draft'))
    } finally {
      setIsRefining(false)
    }
  }

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

    // The editor opens only after the run finishes (or refines in-place), so
    // the AI dialog with the live web research log stays visible the whole
    // time the research is running.
    await doAiSequenceStep(1, prompt, kind)
  }

  const retryAiGeneration = async () => {
    const kind = aiPrompt.trim() && !aiRefinePrompt.trim() ? 'generate' : 'refine'
    const prompt =
      (kind === 'generate' ? aiPrompt.trim() : aiRefinePrompt.trim()) ||
      postFormRef.current.title ||
      'Continue blog generation'
    // Smart resume: if the failed step was before the content step but the
    // content was already written (e.g. a later translation step failed),
    // jump straight there instead of starting from titles again.
    let step = aiGenerationStep || 1
    const enContent = (postFormRef.current.contents.en || postFormRef.current.content || '').trim()
    if (step <= 4 && enContent) {
      step = 5
    }
    if (step >= 6) {
      step = 6
    }
    await doAiSequenceStep(step, prompt, kind)
  }

  const getStepDescription = (step: number) => {
    const targetLangs = BLOG_LOCALE_CODES.filter((code) => code !== 'en')
    switch (step) {
      case 1:
        return t('Titles')
      case 2:
        return t('Summaries')
      case 3:
        return t('Tags')
      case 4:
        return t('SEO Descriptions')
      case 5:
        return t('English Article Content')
      default:
        if (step >= 6) {
          const langIdx = step - 6
          const lang = targetLangs[langIdx] || `lang #${langIdx + 1}`
          return t('Translation to {{lang}}', { lang })
        }
        return t('Step {{step}}', { step })
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

    const sourceMetadata: Record<string, string> = { title: srcTitle }
    if (srcSummary) sourceMetadata.summary = srcSummary
    if (srcTags) sourceMetadata.tags = srcTags
    if (srcSeo) sourceMetadata.seo_description = srcSeo

    const targetLangs = BLOG_LOCALE_CODES.filter((code) => code !== srcLang)

    setAiTranslating(true)
    try {
      const metadataPrompt = [
        `You are an expert translator. The user provides blog post metadata in source language code "${srcLang}".`,
        `Translate the provided fields into all target languages: ${targetLangs.join(', ')}.`,
        'CRITICAL RULES:',
        '1. ONLY translate the fields present in the source object.',
        '2. Return ONLY a single valid JSON object matching this structure:',
        JSON.stringify({
          titles: targetLangs.reduce((acc, code) => ({ ...acc, [code]: '...' }), {}),
          summaries: srcSummary ? targetLangs.reduce((acc, code) => ({ ...acc, [code]: '...' }), {}) : undefined,
          tags_list: srcTags ? targetLangs.reduce((acc, code) => ({ ...acc, [code]: '...' }), {}) : undefined,
          seo_descriptions: srcSeo ? targetLangs.reduce((acc, code) => ({ ...acc, [code]: '...' }), {}) : undefined,
        }),
      ].join('\n\n')

      let metaContent = ''
      try {
        const res = await sendChatCompletion({
          model,
          messages: [
            { role: 'system', content: metadataPrompt },
            { role: 'user', content: JSON.stringify(sourceMetadata) },
          ],
          stream: false,
          temperature: 0.7,
          response_format: { type: 'json_object' }
        }, undefined, 900000)
        metaContent = res.choices?.[0]?.message?.content || ''
      } catch (err: any) {
        toast.error(err.response?.data?.error?.message || err.message || t('Metadata translation failed.'))
      }

      let parsedMeta: any = {}
      if (metaContent) {
        try {
          let text = metaContent.trim()
          const fenceMatch = text.match(/```(?:json)?\s*([\s\S]*?)```/)
          if (fenceMatch) text = fenceMatch[1].trim()
          parsedMeta = JSON.parse(text)
        } catch (e) {
          console.error('Failed to parse metadata translation:', e)
        }
      }

      const nextContents: Record<string, string> = { ...postForm.contents }

      // Translate contents directly as raw markdown language-by-language without JSON
      for (const targetLang of targetLangs) {
        try {
          const contentPrompt = [
            `You are an expert technical translator.`,
            `Translate the following blog article from language "${srcLang}" to language "${targetLang}".`,
            `Output ONLY the translated Markdown article directly.`,
            `Do NOT output JSON. Do NOT wrap the entire text in code fences (no \`\`\`markdown wrapper). Do NOT add conversational commentary, prefaces, thinking tags, or postfaces.`,
            `Maintain all headings, lists, links, code snippets, and markdown formatting.`,
          ].join('\n')

          const contentRes = await sendChatCompletion({
            model,
            messages: [
              { role: 'system', content: contentPrompt },
              { role: 'user', content: srcContent },
            ],
            stream: false,
            temperature: 0.7,
          }, undefined, 900000)

          let rawContent = contentRes.choices?.[0]?.message?.content || ''
          rawContent = rawContent.replace(/<think>[\s\S]*?<\/think>/g, '').trim()
          const fence = rawContent.match(/^```(?:markdown)?\s*([\s\S]*?)```$/)
          if (fence) rawContent = fence[1].trim()

          if (rawContent) {
            nextContents[targetLang] = rawContent
          }
        } catch (err: any) {
          console.error(`Failed to translate content for ${targetLang}:`, err)
        }
      }

      setPostForm((current) => {
        const nextTitles = { ...current.titles }
        const nextSummaries = { ...current.summaries }
        const nextTags = { ...current.tags_list }
        const nextSeo = { ...current.seo_descriptions }

        if (parsedMeta.titles && typeof parsedMeta.titles === 'object') {
          for (const code of targetLangs) {
            if (typeof parsedMeta.titles[code] === 'string' && parsedMeta.titles[code].trim()) {
              nextTitles[code] = parsedMeta.titles[code].trim()
            }
          }
        }
        if (srcSummary && parsedMeta.summaries && typeof parsedMeta.summaries === 'object') {
          for (const code of targetLangs) {
            if (typeof parsedMeta.summaries[code] === 'string' && parsedMeta.summaries[code].trim()) {
              nextSummaries[code] = parsedMeta.summaries[code].trim()
            }
          }
        }
        if (srcTags && parsedMeta.tags_list && typeof parsedMeta.tags_list === 'object') {
          for (const code of targetLangs) {
            if (typeof parsedMeta.tags_list[code] === 'string' && parsedMeta.tags_list[code].trim()) {
              nextTags[code] = parsedMeta.tags_list[code].trim()
            }
          }
        }
        if (srcSeo && parsedMeta.seo_descriptions && typeof parsedMeta.seo_descriptions === 'object') {
          for (const code of targetLangs) {
            if (typeof parsedMeta.seo_descriptions[code] === 'string' && parsedMeta.seo_descriptions[code].trim()) {
              nextSeo[code] = parsedMeta.seo_descriptions[code].trim()
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
      <div className='flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3'>
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
        <div className='flex flex-col sm:flex-row items-stretch sm:items-center gap-2 w-full sm:w-auto'>
          <Button
            type='button'
            size='sm'
            variant='outline'
            className='w-full sm:w-auto justify-center'
            onClick={() => setIsBlogApiOpen(true)}
          >
            <KeyRound className='me-1 h-4 w-4' />
            {t('Blog API')}
          </Button>
          <Button type='button' size='sm' variant='outline' className='w-full sm:w-auto justify-center' onClick={() => { setEditingPost(null); setPostForm(emptyBlogPostForm()); setActiveLanguage('en'); setIsAiOpen(true); }}>
            <Sparkles className='me-1 h-4 w-4' />
            {t('Generate with AI')}
          </Button>
          <Button type='button' size='sm' className='w-full sm:w-auto justify-center' onClick={openNewPost}>
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
            <div className='flex flex-wrap items-center gap-2'>
              {!isAiGenerated && (
                <Button
                  type='button'
                  variant='outline'
                  className='w-full sm:w-auto'
                  disabled={aiTranslating || savePost.isPending || isRefining}
                  onClick={runAiTranslation}
                >
                  {aiTranslating ? (
                    <Loader2 className='me-1 h-4 w-4 animate-spin' />
                  ) : (
                    <Languages className='me-1 h-4 w-4' />
                  )}
                  {t('Translate to other languages with AI')}
                </Button>
              )}
              <Button
                type='button'
                variant='outline'
                className='w-full sm:w-auto'
                disabled={isRefining || savePost.isPending}
                onClick={() => setIsRefineOpen(true)}
              >
                {isRefining ? (
                  <Loader2 className='me-1 h-4 w-4 animate-spin' />
                ) : (
                  <Sparkles className='me-1 h-4 w-4' />
                )}
                {t('Edit Draft')}
              </Button>
            </div>
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
                variant='outline'
                className='flex-1 sm:flex-none text-destructive hover:text-destructive'
                onClick={retryAiGeneration}
                disabled={aiGenerating}
                style={{ display: aiGenerationError ? 'block' : 'none' }}
              >
                {aiGenerating ? (
                  <Loader2 className='me-1 h-4 w-4 animate-spin' />
                ) : (
                  <RefreshCw className='me-1 h-4 w-4' />
                )}
                {t('Resume generation')}
              </Button>
              <Button
                type='button'
                className='flex-1 sm:flex-none'
                disabled={
                  savePost.isPending || aiGenerating || isRefining ||
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
            disabled={aiGenerating || isRefining}
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
              disabled={aiGenerating || isRefining}
              checked={postForm.published}
              onCheckedChange={(checked) =>
                setPostForm({ ...postForm, published: checked })
              }
            />
            <span className='text-muted-foreground text-sm'>{t('Draft posts are hidden from the public site.')}</span>
          </div>
        </div>

        {isRefining && (
          <div className='flex items-center gap-2 rounded-md bg-muted/60 p-3 text-sm text-muted-foreground'>
            <Loader2 className='h-4 w-4 animate-spin text-primary' />
            <span>{t('Refining draft with AI, please wait...')}</span>
          </div>
        )}

        {aiGenerating && (
          <div className='flex items-center gap-2 rounded-md bg-muted/60 p-3 text-sm text-muted-foreground'>
            <Loader2 className='h-4 w-4 animate-spin text-primary' />
            <span>
              {t('Generating {{step}}, please wait...', {
                step: getStepDescription(aiGenerationStep || 1),
              })}
            </span>
          </div>
        )}

        {aiGenerationError && (
          <div className='rounded-lg border border-destructive/40 bg-destructive/10 p-4 space-y-3'>
            <div className='flex items-start gap-3'>
              <AlertCircle className='h-5 w-5 text-destructive shrink-0 mt-0.5' />
              <div className='space-y-1 flex-1'>
                <p className='text-sm font-semibold text-destructive'>
                  {t('AI Generation interrupted at {{step}}', {
                    step: getStepDescription(aiGenerationStep || 1),
                  })}
                </p>
                <p className='text-xs text-muted-foreground'>
                  {aiErrorMessage || t('The AI request failed. You can switch to another model below and resume without losing previously generated content.')}
                </p>
              </div>
            </div>

            <div className='flex flex-col sm:flex-row items-stretch sm:items-center gap-2 pt-1'>
              <div className='flex-1'>
                <Select value={aiModel} onValueChange={(val) => setAiModel(val ?? '')}>
                  <SelectTrigger className='h-9 text-xs bg-background'>
                    <SelectValue placeholder={t('Select fallback model')} />
                  </SelectTrigger>
                  <SelectContent>
                    {(aiModels ?? []).map((m) => (
                      <SelectItem key={m} value={m} className='text-xs'>
                        {m}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <Button
                type='button'
                size='sm'
                variant='destructive'
                disabled={aiGenerating}
                onClick={retryAiGeneration}
                className='gap-1.5'
              >
                {aiGenerating ? (
                  <Loader2 className='h-3.5 w-3.5 animate-spin' />
                ) : (
                  <RefreshCw className='h-3.5 w-3.5' />
                )}
                <span>{t('Resume from {{step}}', { step: getStepDescription(aiGenerationStep || 1) })}</span>
              </Button>
            </div>
          </div>
        )}

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
                readOnly={aiGenerating || isRefining}
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
        {webResearchEnabled && (
          <div className='grid gap-2'>
            <div className='flex items-center justify-between gap-3'>
              <div className='min-w-0'>
                <Label>{t('Web research')}</Label>
                <p className='text-muted-foreground text-xs'>
                  {t('The model searches the web and reads pages before writing, so the post carries fresh facts. Watch the live log below.')}
                </p>
              </div>
              <Switch
                checked={webResearch}
                onCheckedChange={(checked) => setWebResearch(checked)}
                disabled={aiGenerating}
              />
            </div>
            {webResearch && researchLogs.length > 0 && (
              <div
                ref={researchLogRef}
                className='max-h-40 overflow-y-auto rounded-md border bg-background/80 p-2 text-[11px] font-mono leading-5'
              >
                {researchLogs.map((log) => (
                  <div key={log.id} className='flex items-center gap-2 whitespace-nowrap'>
                    <span className='text-muted-foreground shrink-0'>
                      {new Date(log.ts).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}
                    </span>
                    {log.kind === 'search' ? (
                      <span className='text-sky-500 shrink-0'>search</span>
                    ) : (
                      <span className='text-emerald-500 shrink-0'>fetch</span>
                    )}
                    <span className='truncate text-foreground/80' title={log.detail}>{log.detail}</span>
                  </div>
                ))}
                {aiGenerating && (
                  <div className='flex items-center gap-2 text-muted-foreground'>
                    <Loader2 className='h-3 w-3 animate-spin' />
                    {t('Researching the web…')}
                  </div>
                )}
              </div>
            )}
          </div>
        )}
      </Dialog>

      <Dialog
        open={isRefineOpen}
        onOpenChange={setIsRefineOpen}
        title={t('Edit Draft')}
        contentClassName='max-w-md'
        headerClassName='text-left'
        contentHeight='auto'
        bodyClassName='space-y-4'
        footer={
          <>
            <Button
              type='button'
              variant='ghost'
              onClick={() => setIsRefineOpen(false)}
            >
              {t('Cancel')}
            </Button>
            <Button
              type='button'
              disabled={isRefining || !aiRefinePrompt.trim()}
              onClick={runAiRefine}
            >
              {t('Apply Changes')}
            </Button>
          </>
        }
      >
        <div className='grid gap-2'>
          <Label>{t('Requested Changes')}</Label>
          <Textarea
            rows={4}
            value={aiRefinePrompt}
            placeholder={t('Describe your requested changes to the draft (e.g., make the introduction more technical, expand the conclusion)...')}
            onChange={(e) => setAiRefinePrompt(e.target.value)}
          />
        </div>
      </Dialog>

      <CommentsDialog
        postId={commentsPostId}
        onClose={() => setCommentsPostId(null)}
      />

      <ScopedApiDialog
        scope='blog'
        open={isBlogApiOpen}
        onOpenChange={setIsBlogApiOpen}
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
  readOnly?: boolean
  form: BlogPostForm
  onChange: (patch: Partial<BlogPostForm>) => void
}) {
  const { t } = useTranslation()
  const [isPreview, setIsPreview] = useState(false)

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
          readOnly={props.readOnly}
        />
      </div>
      <div className='grid gap-2'>
        <Label>{t('Summary')}</Label>
        <Textarea
          rows={2}
          value={props.form.summaries[props.langCode] ?? ''}
          onChange={(event) => setLocalized('summaries', event.target.value)}
          readOnly={props.readOnly}
        />
      </div>
      <div className='grid gap-2'>
        <Label>{t('Tags (comma separated)')}</Label>
        <Input
          value={props.form.tags_list[props.langCode] ?? ''}
          placeholder={t('news, update')}
          onChange={(event) => setLocalized('tags_list', event.target.value)}
          readOnly={props.readOnly}
        />
      </div>
      <div className='grid gap-2'>
        <Label>{t('SEO description')}</Label>
        <Textarea
          rows={2}
          value={props.form.seo_descriptions[props.langCode] ?? ''}
          onChange={(event) => setLocalized('seo_descriptions', event.target.value)}
          readOnly={props.readOnly}
        />
      </div>
      <div className='grid gap-2'>
        <div className='flex items-center justify-between'>
          <Label>{t('Content (Markdown)')}</Label>
          <Button type='button' variant='ghost' size='sm' onClick={() => setIsPreview(!isPreview)}>
            {isPreview ? t('Edit') : t('Preview')}
          </Button>
        </div>
        {isPreview ? (
          <div className='min-h-[200px] max-h-[500px] overflow-y-auto rounded-md border p-4 bg-muted/20'>
            <BlogMarkdown content={props.form.contents[props.langCode] ?? ''} />
          </div>
        ) : (
          <Textarea
            rows={12}
            value={props.form.contents[props.langCode] ?? ''}
            className='font-mono text-xs'
            onChange={(event) => setLocalized('contents', event.target.value)}
            readOnly={props.readOnly}
          />
        )}
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
