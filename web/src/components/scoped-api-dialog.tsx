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
  BookOpen,
  Check,
  Copy,
  KeyRound,
  Loader2,
  Plus,
  ShieldCheck,
  Trash2,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { api } from '@/lib/api'

export type ScopedApiScope = 'blog' | 'file_manager'

interface ScopedApiKeyRecord {
  id: number
  scope: string
  name: string
  key_prefix: string
  created_by: number
  created_at: number
  last_used_at: number
}

interface ScopedApiDialogProps {
  scope: ScopedApiScope
  open: boolean
  onOpenChange: (open: boolean) => void
}

function CopySnippetButton({ text }: { text: string }) {
  const { t } = useTranslation()
  const [copied, setCopied] = useState(false)

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(text)
      setCopied(true)
      toast.success(t('Copied'))
      setTimeout(() => setCopied(false), 1500)
    } catch {
      toast.error(t('Failed to copy'))
    }
  }

  return (
    <Button
      type='button'
      variant='ghost'
      size='icon'
      className='h-7 w-7 shrink-0'
      onClick={() => void handleCopy()}
      title={t('Copy')}
    >
      {copied ? (
        <Check className='h-3.5 w-3.5 text-emerald-500' />
      ) : (
        <Copy className='h-3.5 w-3.5' />
      )}
    </Button>
  )
}

export function ScopedApiDialog({
  scope,
  open,
  onOpenChange,
}: ScopedApiDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [keyName, setKeyName] = useState('')
  const [newRawKey, setNewRawKey] = useState('')

  const isBlog = scope === 'blog'
  const manageEndpoint = isBlog
    ? '/api/blog/manage/api-keys'
    : '/api/files/api-keys'
  const queryKey = ['scoped-api-keys', scope]

  const keysQuery = useQuery({
    queryKey,
    enabled: open,
    queryFn: async () => {
      const res = await api.get<{
        success: boolean
        message?: string
        data?: { keys: ScopedApiKeyRecord[]; base_url?: string }
      }>(manageEndpoint)
      if (!res.data?.success) {
        throw new Error(res.data?.message || 'Failed to load API keys')
      }
      return res.data.data ?? { keys: [], base_url: '' }
    },
  })

  const createKeyMutation = useMutation({
    mutationFn: async () => {
      const res = await api.post<{
        success: boolean
        message?: string
        data?: { key: ScopedApiKeyRecord; raw_key: string }
      }>(manageEndpoint, { name: keyName.trim() })
      if (!res.data?.success || !res.data.data?.raw_key) {
        throw new Error(res.data?.message || t('Failed to create API key'))
      }
      return res.data.data
    },
    onSuccess: (data) => {
      setNewRawKey(data.raw_key)
      setKeyName('')
      toast.success(t('API key created'))
      void queryClient.invalidateQueries({ queryKey })
    },
    onError: (err: Error) => {
      toast.error(err.message || t('Failed to create API key'))
    },
  })

  const deleteKeyMutation = useMutation({
    mutationFn: async (id: number) => {
      const res = await api.delete<{ success: boolean; message?: string }>(
        `${manageEndpoint}/${id}`
      )
      if (!res.data?.success) {
        throw new Error(res.data?.message || t('Failed to delete API key'))
      }
    },
    onSuccess: () => {
      toast.success(t('API key revoked'))
      void queryClient.invalidateQueries({ queryKey })
    },
    onError: (err: Error) => {
      toast.error(err.message || t('Failed to delete API key'))
    },
  })

  const origin =
    keysQuery.data?.base_url?.replace(/\/+$/, '') ||
    (typeof window !== 'undefined'
      ? window.location.origin
      : 'https://your-domain.com')
  const apiBase = isBlog ? `${origin}/api/v1/blog` : `${origin}/api/v1/files`
  const keys = keysQuery.data?.keys ?? []
  const sampleKey =
    newRawKey || (isBlog ? 'sk-blog-YOUR_KEY' : 'sk-files-YOUR_KEY')

  const blogCreateExample = `curl -X POST "${apiBase}/posts" \\
  -H "Authorization: Bearer ${sampleKey}" \\
  -H "Content-Type: application/json" \\
  -d '{
    "cover_image": "https://cdn.example.com/banners/launch.png",
    "published": true,
    "translations": {
      "en": {
        "title": "Introducing Our New Release",
        "summary": "Overview of what is new.",
        "tags": ["release", "guide"],
        "seo_description": "Release overview and walkthrough.",
        "content": "# Hello\\n\\n::youtube[https://www.youtube.com/watch?v=dQw4w9WgXcQ]\\n\\n::video[https://cdn.example.com/videos/demo.mp4]"
      },
      "tr": {
        "title": "Yeni Sürümümüz Yayında",
        "summary": "Yeniliklere hızlı bakış.",
        "tags": ["duyuru", "rehber"],
        "content": "# Merhaba\\n\\nTürkçe içerik buraya yazılır."
      }
    }
  }'`

  const fileUploadExample = `curl -X POST "${apiBase}/upload" \\
  -H "Authorization: Bearer ${sampleKey}" \\
  -F "file=@./promo.mp4" \\
  -F "dir=media/2026" \\
  -F "filename=launch-video.mp4"`

  const fileOpsExample = `# Create folder
curl -X POST "${apiBase}/mkdir" -H "Authorization: Bearer ${sampleKey}" -H "Content-Type: application/json" -d '{"path":"assets/banners"}'

# List folder (add &recursive=true for all nested items)
curl "${apiBase}?path=assets" -H "Authorization: Bearer ${sampleKey}"

# Get direct public URL & metadata
curl "${apiBase}/info?path=media/2026/launch-video.mp4" -H "Authorization: Bearer ${sampleKey}"

# Create / edit text file
curl -X PUT "${apiBase}/content" -H "Authorization: Bearer ${sampleKey}" -H "Content-Type: application/json" -d '{"path":"docs/notes.md","content":"# Notes"}'

# Rename or move file/folder
curl -X POST "${apiBase}/rename" -H "Authorization: Bearer ${sampleKey}" -H "Content-Type: application/json" -d '{"old_path":"docs/notes.md","new_path":"docs/changelog.md"}'

# Download file
curl -L "${apiBase}/download?path=media/2026/launch-video.mp4" -H "Authorization: Bearer ${sampleKey}" -o launch-video.mp4

# Delete file or folder
curl -X DELETE "${apiBase}?path=docs/changelog.md" -H "Authorization: Bearer ${sampleKey}"`

  const endpoints = isBlog
    ? [
        {
          method: 'GET',
          path: '/posts',
          desc: t(
            'List all posts with full URLs & translations (?page, ?page_size, ?published, ?search)'
          ),
        },
        {
          method: 'GET',
          path: '/posts/:id',
          desc: t('Get single post details, URL, and all 8 language fields'),
        },
        {
          method: 'POST',
          path: '/posts',
          desc: t(
            'Create post with cover image URL, 8 locales, and up to 50 MB Markdown content'
          ),
        },
        {
          method: 'PATCH',
          path: '/posts/:id',
          desc: t(
            'Partially update post fields or add/edit specific languages'
          ),
        },
        {
          method: 'PUT',
          path: '/posts/:id',
          desc: t('Update existing post'),
        },
        {
          method: 'DELETE',
          path: '/posts/:id',
          desc: t('Permanently delete a post'),
        },
      ]
    : [
        {
          method: 'GET',
          path: '/',
          desc: t(
            'List files & folders with public URLs (?path=folder&recursive=true)'
          ),
        },
        {
          method: 'GET',
          path: '/info',
          desc: t(
            'Get metadata, public link, and download link (?path=...)'
          ),
        },
        {
          method: 'POST',
          path: '/upload',
          desc: t(
            'Upload file to any folder with custom filename (multipart or JSON base64)'
          ),
        },
        {
          method: 'POST',
          path: '/mkdir',
          desc: t(
            'Create folder and missing parent folders ({"path":"a/b"})'
          ),
        },
        {
          method: 'GET',
          path: '/content',
          desc: t('Read text file content (?path=...)'),
        },
        {
          method: 'PUT',
          path: '/content',
          desc: t(
            'Create or edit text file content ({"path":"...","content":"..."})'
          ),
        },
        {
          method: 'POST',
          path: '/rename',
          desc: t(
            'Rename or move file/folder recursively ({"old_path":"...","new_path":"..."})'
          ),
        },
        {
          method: 'GET',
          path: '/download',
          desc: t('Download raw file bytes (?path=...)'),
        },
        {
          method: 'DELETE',
          path: '/',
          desc: t('Delete file or folder recursively (?path=...)'),
        },
      ]

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) setNewRawKey('')
        onOpenChange(next)
      }}
    >
      <DialogContent className='flex max-h-[85vh] w-[calc(100vw-2rem)] flex-col overflow-hidden p-0 sm:max-w-3xl'>
        <DialogHeader className='shrink-0 border-b px-6 pt-5 pb-4'>
          <DialogTitle className='flex items-center gap-2'>
            <KeyRound className='h-4 w-4' />
            {isBlog ? t('Blog API') : t('File Manager API')}
          </DialogTitle>
          <DialogDescription>
            {isBlog
              ? t(
                  'Manage external Blog API keys and integrate automated multi-language publishing.'
                )
              : t(
                  'Manage external File Manager API keys for programmatic uploads, folders, editing, and links.'
                )}
          </DialogDescription>
        </DialogHeader>

        <Tabs
          defaultValue='keys'
          className='flex min-h-0 flex-1 flex-col overflow-hidden'
        >
          <div className='shrink-0 px-6 pt-4'>
            <TabsList>
              <TabsTrigger value='keys' className='gap-1.5'>
                <KeyRound className='h-3.5 w-3.5' />
                {t('API Keys')} ({keys.length})
              </TabsTrigger>
              <TabsTrigger value='docs' className='gap-1.5'>
                <BookOpen className='h-3.5 w-3.5' />
                {t('API Documentation')}
              </TabsTrigger>
            </TabsList>
          </div>

          <div className='min-h-0 flex-1 overflow-x-hidden overflow-y-auto px-6 py-4'>
            <TabsContent value='keys' className='mt-0 space-y-4'>
              <div className='bg-muted/40 flex items-start gap-2.5 rounded-lg border px-3.5 py-2.5 text-xs'>
                <ShieldCheck className='text-primary mt-0.5 h-4 w-4 shrink-0' />
                <p className='text-muted-foreground leading-relaxed'>
                  {t(
                    'External endpoints are active only when authenticated with a valid key below (Authorization: Bearer <key>). Deleting a key revokes access immediately.'
                  )}
                </p>
              </div>

              <form
                className='flex flex-col gap-2 sm:flex-row sm:items-end'
                onSubmit={(e) => {
                  e.preventDefault()
                  createKeyMutation.mutate()
                }}
              >
                <div className='flex-1 space-y-1.5'>
                  <Label htmlFor={`scoped-key-name-${scope}`}>
                    {t('Key Name')}
                  </Label>
                  <Input
                    id={`scoped-key-name-${scope}`}
                    placeholder={
                      isBlog
                        ? t('e.g. AI Blog Publisher')
                        : t('e.g. Asset Uploader Bot')
                    }
                    value={keyName}
                    onChange={(e) => setKeyName(e.target.value)}
                  />
                </div>
                <Button
                  type='submit'
                  size='sm'
                  disabled={createKeyMutation.isPending}
                >
                  {createKeyMutation.isPending ? (
                    <Loader2 className='mr-1.5 h-4 w-4 animate-spin' />
                  ) : (
                    <Plus className='mr-1.5 h-4 w-4' />
                  )}
                  {t('Create API Key')}
                </Button>
              </form>

              {newRawKey && (
                <div className='space-y-2 rounded-lg border border-emerald-500/40 bg-emerald-500/10 p-3'>
                  <div className='text-xs font-medium text-emerald-700 dark:text-emerald-400'>
                    {t(
                      'Copy your new API key now — it will not be shown again.'
                    )}
                  </div>
                  <div className='bg-background flex min-w-0 items-center justify-between gap-2 rounded-md border px-2.5 py-1.5 font-mono text-xs'>
                    <span className='min-w-0 truncate select-all'>
                      {newRawKey}
                    </span>
                    <CopySnippetButton text={newRawKey} />
                  </div>
                </div>
              )}

              <div className='divide-y rounded-lg border'>
                {keysQuery.isLoading ? (
                  <div className='flex items-center justify-center py-8'>
                    <Loader2 className='text-muted-foreground h-4 w-4 animate-spin' />
                  </div>
                ) : keys.length === 0 ? (
                  <div className='text-muted-foreground py-8 text-center text-sm'>
                    {t(
                      'No API keys created yet. External API access is currently inactive.'
                    )}
                  </div>
                ) : (
                  keys.map((item) => (
                    <div
                      key={item.id}
                      className='flex items-center justify-between gap-3 px-3.5 py-2.5'
                    >
                      <div className='min-w-0 space-y-0.5'>
                        <div className='flex flex-wrap items-center gap-2'>
                          <span className='truncate text-sm font-medium'>
                            {item.name}
                          </span>
                          <Badge
                            variant='secondary'
                            className='font-mono text-[11px] font-normal'
                          >
                            {item.key_prefix}
                          </Badge>
                        </div>
                        <div className='text-muted-foreground text-xs'>
                          {t('Created')}:{' '}
                          {dayjs.unix(item.created_at).format('YYYY-MM-DD HH:mm')}
                          {item.last_used_at > 0
                            ? ` · ${t('Last used')}: ${dayjs.unix(item.last_used_at).format('YYYY-MM-DD HH:mm')}`
                            : ` · ${t('Never used')}`}
                        </div>
                      </div>
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon'
                        className='text-destructive hover:text-destructive shrink-0'
                        disabled={deleteKeyMutation.isPending}
                        title={t('Delete')}
                        onClick={() => {
                          if (
                            window.confirm(t('Revoke and delete this API key?'))
                          ) {
                            deleteKeyMutation.mutate(item.id)
                          }
                        }}
                      >
                        <Trash2 className='h-4 w-4' />
                      </Button>
                    </div>
                  ))
                )}
              </div>
            </TabsContent>

            <TabsContent value='docs' className='mt-0 min-w-0 space-y-4'>
              {/* Base URL & Auth */}
              <div className='grid min-w-0 gap-2 rounded-lg border p-3 text-xs'>
                <div className='flex flex-wrap items-center justify-between gap-2'>
                  <span className='text-muted-foreground shrink-0 font-medium'>
                    {t('Base URL')}
                  </span>
                  <div className='flex min-w-0 max-w-full items-center gap-1'>
                    <code className='bg-muted truncate rounded px-2 py-0.5 font-mono'>
                      {apiBase}
                    </code>
                    <CopySnippetButton text={apiBase} />
                  </div>
                </div>
                <div className='flex flex-wrap items-center justify-between gap-2'>
                  <span className='text-muted-foreground shrink-0 font-medium'>
                    {t('Authentication Header')}
                  </span>
                  <code className='bg-muted max-w-full break-all rounded px-2 py-0.5 font-mono'>
                    Authorization: Bearer {isBlog ? 'sk-blog-...' : 'sk-files-...'}
                  </code>
                </div>
                {isBlog && (
                  <div className='flex flex-wrap items-center justify-between gap-2'>
                    <span className='text-muted-foreground shrink-0 font-medium'>
                      {t('Supported Locales')}
                    </span>
                    <code className='bg-muted max-w-full break-all rounded px-2 py-0.5 font-mono'>
                      en, tr, zhCN, zhTW, fr, ja, ru, vi
                    </code>
                  </div>
                )}
              </div>

              {/* Endpoints list */}
              <div className='min-w-0 space-y-1.5'>
                <h4 className='text-muted-foreground text-xs font-semibold tracking-wide uppercase'>
                  {t('Endpoints')}
                </h4>
                <div className='divide-y rounded-lg border text-xs'>
                  {endpoints.map((ep) => (
                    <div
                      key={`${ep.method}-${ep.path}`}
                      className='flex flex-col gap-1 px-3 py-2 sm:flex-row sm:items-center sm:gap-3'
                    >
                      <div className='flex shrink-0 items-center gap-2'>
                        <Badge
                          variant='outline'
                          className='w-16 justify-center font-mono text-[10px]'
                        >
                          {ep.method}
                        </Badge>
                        <code className='font-mono font-medium'>{ep.path}</code>
                      </div>
                      <span className='text-muted-foreground min-w-0 break-words'>
                        {ep.desc}
                      </span>
                    </div>
                  ))}
                </div>
              </div>

              {isBlog ? (
                <>
                  {/* Markdown YouTube & Studio Video Player guide */}
                  <div className='min-w-0 space-y-2 rounded-lg border p-3 text-xs'>
                    <div className='font-semibold'>
                      {t('Markdown Media Embeds (YouTube & Video.js Player)')}
                    </div>
                    <p className='text-muted-foreground'>
                      {t(
                        'Pass banner images via cover_image URL (or upload via File Manager API). Place media embeds on their own line inside Markdown — videos use the full Studio Video.js player.'
                      )}
                    </p>
                    <div className='grid min-w-0 gap-1.5 font-mono text-[11px]'>
                      <div className='bg-muted flex min-w-0 items-center justify-between gap-2 rounded px-2.5 py-1.5'>
                        <code className='min-w-0 truncate'>
                          ::youtube[https://www.youtube.com/watch?v=VIDEO_ID]
                        </code>
                        <CopySnippetButton text='::youtube[https://www.youtube.com/watch?v=VIDEO_ID]' />
                      </div>
                      <div className='bg-muted flex min-w-0 items-center justify-between gap-2 rounded px-2.5 py-1.5'>
                        <code className='min-w-0 truncate'>
                          ::video[https://cdn.example.com/videos/demo.mp4]
                        </code>
                        <CopySnippetButton text='::video[https://cdn.example.com/videos/demo.mp4]' />
                      </div>
                    </div>
                  </div>

                  {/* Create Post Example */}
                  <div className='min-w-0 space-y-1.5'>
                    <div className='flex items-center justify-between gap-2'>
                      <h4 className='text-muted-foreground text-xs font-semibold'>
                        {t('Example: Create Multi-Language Post')}
                      </h4>
                      <CopySnippetButton text={blogCreateExample} />
                    </div>
                    <pre className='bg-muted/60 max-w-full overflow-x-auto rounded-lg border p-3 font-mono text-[11px] leading-relaxed whitespace-pre'>
                      {blogCreateExample}
                    </pre>
                  </div>
                </>
              ) : (
                <>
                  <div className='min-w-0 space-y-1.5'>
                    <div className='flex items-center justify-between gap-2'>
                      <h4 className='text-muted-foreground text-xs font-semibold'>
                        {t('Example: Upload File to Custom Folder & Filename')}
                      </h4>
                      <CopySnippetButton text={fileUploadExample} />
                    </div>
                    <pre className='bg-muted/60 max-w-full overflow-x-auto rounded-lg border p-3 font-mono text-[11px] leading-relaxed whitespace-pre'>
                      {fileUploadExample}
                    </pre>
                  </div>

                  <div className='min-w-0 space-y-1.5'>
                    <div className='flex items-center justify-between gap-2'>
                      <h4 className='text-muted-foreground text-xs font-semibold'>
                        {t(
                          'Example: Folder, List, Link, Edit, Rename, Download & Delete'
                        )}
                      </h4>
                      <CopySnippetButton text={fileOpsExample} />
                    </div>
                    <pre className='bg-muted/60 max-w-full overflow-x-auto rounded-lg border p-3 font-mono text-[11px] leading-relaxed whitespace-pre'>
                      {fileOpsExample}
                    </pre>
                  </div>
                </>
              )}
            </TabsContent>
          </div>
        </Tabs>
      </DialogContent>
    </Dialog>
  )
}
