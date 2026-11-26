import {
  AlertCircle,
  Check,
  Download,
  Layers,
  Loader2,
  Lock,
  Search,
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
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { api } from '@/lib/api'

export type HfTaskType = 'text' | 'image' | 'video' | 'audio' | 'embeddings'

export interface HfPickedModel {
  repoId: string
  filename?: string
  downloadUrl?: string
  pipelineTag?: string
  parameterSize?: string
  taskType: HfTaskType
}

export function mapPipelineTagToTask(tag?: string): HfTaskType {
	const t = (tag ?? '').toLowerCase()
	if (t.includes('video')) return 'video'
	if (
		t === 'image-text-to-text' ||
		t.includes('text-generation') ||
		t.includes('question-answering') ||
		t.includes('conversational')
	) {
		return 'text'
	}
	if (t.includes('text-to-image') || t.includes('image-to-image') || t.includes('diffusion')) return 'image'
	if (t.includes('audio') || t.includes('speech') || t.includes('tts')) return 'audio'
	if (t.includes('embed') || t.includes('feature-extraction')) return 'embeddings'
	return 'text'
}

interface HfRepoResult {
  id: string
  author?: string
  downloads?: number
  likes?: number
  gated?: boolean
  private?: boolean
  parameter_size?: string
  pipeline_tag?: string
}

interface HfFileResult {
  filename: string
  size?: number
  size_mb?: number
  size_gb?: number
  download_url?: string
  quantization?: string
  recommended_ram_gb?: number
  recommended_vram_gb?: number
  recommended_cores?: number
}

export function HfModelPickerDialog({
  open,
  onOpenChange,
  onSelect,
  multiSelect = false,
  onSelectMultiple,
  initialQuery = 'qwen2.5',
  apiBasePath = '/api/deployments/hf',
  excludeGguf = false,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSelect: (picked: HfPickedModel) => void
  multiSelect?: boolean
  onSelectMultiple?: (picked: HfPickedModel[]) => void
  initialQuery?: string
  apiBasePath?: string
  excludeGguf?: boolean
}) {
  const { t } = useTranslation()
  const [searchQuery, setSearchQuery] = useState(initialQuery)
  const [hfToken, setHfToken] = useState('')
  const [searching, setSearching] = useState(false)
  const [repos, setRepos] = useState<HfRepoResult[]>([])
  const [selectedRepo, setSelectedRepo] = useState<HfRepoResult | null>(null)
  const [loadingFiles, setLoadingFiles] = useState(false)
  const [files, setFiles] = useState<HfFileResult[]>([])
  const [gatedWarning, setGatedWarning] = useState('')
  const [selectedItems, setSelectedItems] = useState<HfPickedModel[]>([])

  function pickedKey(p: HfPickedModel) {
    return `${p.repoId}::${p.filename ?? ''}`
  }

  function isPicked(p: HfPickedModel) {
    const key = pickedKey(p)
    return selectedItems.some((item) => pickedKey(item) === key)
  }

  function toggleMultiItem(item: HfPickedModel) {
    const key = pickedKey(item)
    setSelectedItems((prev) =>
      prev.some((x) => pickedKey(x) === key)
        ? prev.filter((x) => pickedKey(x) !== key)
        : [...prev, item]
    )
  }

  async function searchHf(query?: string) {
    const q = (query ?? searchQuery).trim()
    setSearching(true)
    setGatedWarning('')
    try {
      const resp = await api.get(`${apiBasePath}/search`, {
        params: {
          q: q || 'qwen',
          token: hfToken,
        },
      })
      const res = resp.data
      if (res.success) {
        setRepos(res.data || [])
      } else {
        toast.error(res.message || t('Failed to search Hugging Face models'))
      }
    } catch (e: any) {
      toast.error(e?.response?.data?.message || String(e))
    } finally {
      setSearching(false)
    }
  }

  async function loadRepoFiles(repo: HfRepoResult) {
    setSelectedRepo(repo)
    setLoadingFiles(true)
    setFiles([])
    setGatedWarning('')
    try {
      const resp = await api.get(`${apiBasePath}/files`, {
        params: {
          repo: repo.id,
          token: hfToken,
        },
      })
      const res = resp.data
      if (res.gated) {
        setGatedWarning(res.message)
      } else if (res.success && res.data) {
        setFiles(res.data.files || [])
      } else {
        toast.error(res.message || t('Failed to list repository files'))
      }
    } catch (e: any) {
      toast.error(e?.response?.data?.message || String(e))
    } finally {
      setLoadingFiles(false)
    }
  }

  function handleSelectRepoDirectly(repo: HfRepoResult) {
    if (repo.gated && !hfToken.trim()) {
      toast.error(
        t('This model is gated. Please enter a Hugging Face Access Token above.')
      )
      return
    }
    const picked: HfPickedModel = {
      repoId: repo.id,
      pipelineTag: repo.pipeline_tag,
      parameterSize: repo.parameter_size,
      taskType: mapPipelineTagToTask(repo.pipeline_tag),
    }
    if (multiSelect) {
      toggleMultiItem(picked)
      return
    }
    onSelect(picked)
    onOpenChange(false)
  }

  function handleSelectFile(file: HfFileResult) {
    if (!selectedRepo) return
    if (excludeGguf && file.filename.toLowerCase().endsWith('.gguf')) {
      toast.error(
        t(
          'GGUF files are inference-only and cannot be fine-tuned. Select a .safetensors file or use the full repository.'
        )
      )
      return
    }
    if (selectedRepo.gated && !hfToken.trim()) {
      toast.error(
        t('This model is gated. Please enter a Hugging Face Access Token above.')
      )
      return
    }
    const picked: HfPickedModel = {
      repoId: selectedRepo.id,
      filename: file.filename,
      downloadUrl: file.download_url,
      pipelineTag: selectedRepo.pipeline_tag,
      parameterSize: selectedRepo.parameter_size,
      taskType: mapPipelineTagToTask(selectedRepo.pipeline_tag),
    }
    if (multiSelect) {
      toggleMultiItem(picked)
      return
    }
    onSelect(picked)
    onOpenChange(false)
  }

  const visibleFiles = excludeGguf
    ? files.filter((f) => !f.filename.toLowerCase().endsWith('.gguf'))
    : files
  const filteredOutGgufCount = excludeGguf
    ? files.length - visibleFiles.length
    : 0

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-w-5xl w-[95vw] h-[82vh] flex flex-col'>
        <DialogHeader>
          <DialogTitle>{t('Select Model from Hugging Face')}</DialogTitle>
          <DialogDescription>
            {t(
              'Search Hugging Face repositories (LLMs, image/video diffusion, audio, embeddings) and pick a repository or specific weight file.'
            )}
          </DialogDescription>
        </DialogHeader>

        <div className='flex flex-col gap-3 py-2 flex-1 overflow-hidden min-h-0'>
          <div className='grid grid-cols-1 md:grid-cols-12 gap-2 shrink-0'>
            <div className='md:col-span-8 flex gap-2'>
              <div className='relative flex-1'>
                <Search className='absolute left-3 top-2.5 h-4 w-4 text-muted-foreground' />
                <Input
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  onKeyDown={(e) => e.key === 'Enter' && searchHf()}
                  placeholder={t(
                    'Search model name (e.g. qwen2.5, wan2.1, flux, whisper)...'
                  )}
                  className='pl-9 h-10 text-xs sm:text-sm'
                />
              </div>
              <Button
                type='button'
                onClick={() => searchHf()}
                disabled={searching}
                className='h-10 px-4 gap-1.5 font-medium shrink-0 text-xs sm:text-sm'
              >
                {searching ? (
                  <Loader2 className='h-4 w-4 animate-spin' />
                ) : (
                  <Search className='h-4 w-4' />
                )}
                {t('Search')}
              </Button>
            </div>
            <div className='md:col-span-4'>
              <Input
                type='password'
                value={hfToken}
                onChange={(e) => setHfToken(e.target.value)}
                placeholder={t('HF Token (Optional - for gated models)')}
                className='h-10 text-xs'
              />
            </div>
          </div>

          {gatedWarning ? (
            <div className='bg-amber-500/10 border border-amber-500/30 text-amber-500 rounded-lg p-3 text-xs flex items-center gap-2 shrink-0'>
              <Lock className='h-4 w-4 shrink-0' />
              <span>{gatedWarning}</span>
            </div>
          ) : null}

          <div className='grid grid-cols-1 md:grid-cols-2 gap-3 sm:gap-4 flex-1 min-h-0 overflow-y-auto md:overflow-hidden'>
            {/* Left: Repositories */}
            <div className='border rounded-xl p-3 flex flex-col bg-muted/10 min-h-[260px] md:min-h-0 overflow-hidden'>
              <div className='flex items-center justify-between text-xs font-semibold text-muted-foreground pb-2 border-b mb-2 shrink-0'>
                <span>
                  {t('Repositories Found')} ({repos.length})
                </span>
                {searching && (
                  <span className='text-primary animate-pulse text-[11px]'>
                    {t('Searching...')}
                  </span>
                )}
              </div>
              <div className='flex-1 overflow-y-auto space-y-2 pr-1'>
                {searching ? (
                  <div className='flex flex-col items-center justify-center h-56 text-muted-foreground text-xs gap-2'>
                    <Loader2 className='h-6 w-6 animate-spin text-primary' />
                    <span>{t('Scanning Hugging Face...')}</span>
                  </div>
                ) : repos.length === 0 ? (
                  <div className='flex flex-col items-center justify-center h-56 text-muted-foreground text-xs text-center px-4'>
                    <Search className='h-8 w-8 text-muted-foreground/40 mb-2' />
                    <p className='font-medium text-foreground'>
                      {t('No models found')}
                    </p>
                    <p className='text-[11px] mt-1'>
                      {t(
                        'Enter a model or architecture name and click Search.'
                      )}
                    </p>
                  </div>
                ) : (
                  repos.map((repo) => (
                    <div
                      key={repo.id}
                      onClick={() => loadRepoFiles(repo)}
                      className={`p-3 rounded-xl border text-xs cursor-pointer transition-all space-y-2 ${
                        selectedRepo?.id === repo.id
                          ? 'border-primary bg-primary/10 shadow-sm'
                          : 'hover:bg-muted/60 bg-card border-border'
                      }`}
                    >
                      <div className='flex items-start justify-between gap-2'>
                        <div className='font-bold text-sm text-foreground break-all'>
                          {repo.id}
                        </div>
                        <Button
                          type='button'
                          size='sm'
                          variant={
                            multiSelect &&
                            isPicked({
                              repoId: repo.id,
                              taskType: mapPipelineTagToTask(repo.pipeline_tag),
                            })
                              ? 'default'
                              : 'secondary'
                          }
                          className='h-7 text-[11px] shrink-0 gap-1'
                          onClick={(e) => {
                            e.stopPropagation()
                            handleSelectRepoDirectly(repo)
                          }}
                        >
                          <Check className='h-3 w-3' />
                          {multiSelect &&
                          isPicked({
                            repoId: repo.id,
                            taskType: mapPipelineTagToTask(repo.pipeline_tag),
                          })
                            ? t('Selected')
                            : t('Use Repo')}
                        </Button>
                      </div>
                      <div className='flex flex-wrap items-center gap-2 text-muted-foreground text-[11px]'>
                        <span>
                          {t('Downloads')}:{' '}
                          <strong>
                            {repo.downloads?.toLocaleString() || 0}
                          </strong>
                        </span>
                        <span>
                          {t('Likes')}:{' '}
                          <strong>{repo.likes?.toLocaleString() || 0}</strong>
                        </span>
                        {repo.parameter_size ? (
                          <span>
                            {t('Size')}: <strong>{repo.parameter_size}</strong>
                          </span>
                        ) : null}
                        {repo.pipeline_tag ? (
                          <Badge variant='secondary' className='text-[10px]'>
                            {repo.pipeline_tag}
                          </Badge>
                        ) : null}
                        {repo.gated ? (
                          <Badge
                            variant='outline'
                            className='text-[10px] border-amber-500/50 text-amber-500 bg-amber-500/10'
                          >
                            {t('Gated')}
                          </Badge>
                        ) : (
                          <Badge
                            variant='outline'
                            className='text-[10px] border-emerald-500/40 text-emerald-500 bg-emerald-500/10'
                          >
                            {t('Open Source')}
                          </Badge>
                        )}
                      </div>
                    </div>
                  ))
                )}
              </div>
            </div>

            {/* Right: Weight Files */}
            <div className='border rounded-xl p-3 flex flex-col bg-muted/10 min-h-[260px] md:min-h-0 overflow-hidden'>
              <div className='text-xs font-semibold text-muted-foreground pb-2 border-b mb-2 shrink-0 truncate'>
                {selectedRepo ? (
                  <span className='text-foreground font-bold'>
                    {selectedRepo.id}{' '}
                    <span className='font-normal text-muted-foreground'>
                      ({visibleFiles.length} {t('files')})
                    </span>
                  </span>
                ) : (
                  t('Model Weight Files')
                )}
              </div>

              {excludeGguf && filteredOutGgufCount > 0 && (
                <div className='mb-2 rounded-lg border border-amber-500/30 bg-amber-500/10 px-2.5 py-2 text-[11px] text-amber-600 dark:text-amber-400'>
                  {t(
                    '{{count}} GGUF file(s) hidden because GGUF is quantized for inference only and cannot be fine-tuned. Choose a .safetensors file or click "Use Repo".',
                    { count: filteredOutGgufCount }
                  )}
                </div>
              )}

              <div className='flex-1 overflow-y-auto space-y-2 pr-1'>
                {loadingFiles ? (
                  <div className='flex flex-col items-center justify-center h-56 text-muted-foreground text-xs gap-2'>
                    <Loader2 className='h-6 w-6 animate-spin text-primary' />
                    <span>{t('Scanning repository files...')}</span>
                  </div>
                ) : !selectedRepo ? (
                  <div className='flex flex-col items-center justify-center h-56 text-muted-foreground text-xs text-center px-4'>
                    <Layers className='h-8 w-8 text-muted-foreground/40 mb-2' />
                    <p className='font-medium text-foreground'>
                      {t('Select a Repository')}
                    </p>
                    <p className='text-[11px] mt-1'>
                      {t(
                        'Click a repository on the left to inspect its weight files, or click "Use Repo" to deploy the full repository.'
                      )}
                    </p>
                  </div>
                ) : visibleFiles.length === 0 ? (
                  <div className='flex flex-col items-center justify-center h-56 text-muted-foreground text-xs text-center px-4 gap-3'>
                    <AlertCircle className='h-8 w-8 text-amber-500/60' />
                    <div>
                      <p className='font-medium text-foreground'>
                        {excludeGguf && filteredOutGgufCount > 0
                          ? t('No fine-tunable weight files in this repository')
                          : t('No standalone weight files listed')}
                      </p>
                      <p className='text-[11px] mt-1'>
                        {excludeGguf && filteredOutGgufCount > 0
                          ? t(
                              'This repository only contains GGUF inference files. Pick a base repository with .safetensors weights.'
                            )
                          : t(
                              'Diffusers, video, and full-repo models can be deployed directly by repository ID.'
                            )}
                      </p>
                    </div>
                    {!(excludeGguf && filteredOutGgufCount > 0) && (
                      <Button
                        type='button'
                        size='sm'
                        onClick={() => handleSelectRepoDirectly(selectedRepo)}
                      >
                        <Check className='h-3.5 w-3.5 mr-1.5' />
                        {t('Use Repository')} ({selectedRepo.id})
                      </Button>
                    )}
                  </div>
                ) : (
                  visibleFiles.map((file, idx) => (
                    <div
                      key={idx}
                      className='p-3.5 rounded-xl border bg-card hover:border-primary/60 transition-all space-y-2.5 shadow-xs'
                    >
                      <div className='flex items-start justify-between gap-2'>
                        <div
                          className='font-mono font-bold text-xs text-foreground break-all'
                          title={file.filename}
                        >
                          {file.filename}
                        </div>
                        <Badge
                          variant='secondary'
                          className='text-[11px] shrink-0 font-bold px-2 py-0.5'
                        >
                          {file.quantization || 'Standard'}
                        </Badge>
                      </div>

                      <div className='text-xs text-muted-foreground font-medium'>
                        {t('File Size')}:{' '}
                        <strong className='text-foreground'>
                          {file.size ? `${file.size_gb} GB` : '—'}
                        </strong>
                      </div>

                      <Button
                        type='button'
                        size='sm'
                        variant={
                          multiSelect &&
                          isPicked({
                            repoId: selectedRepo.id,
                            filename: file.filename,
                            taskType: mapPipelineTagToTask(
                              selectedRepo.pipeline_tag
                            ),
                          })
                            ? 'secondary'
                            : 'default'
                        }
                        className='w-full text-xs h-8 gap-1.5 font-medium'
                        onClick={() => handleSelectFile(file)}
                      >
                        {multiSelect &&
                        isPicked({
                          repoId: selectedRepo.id,
                          filename: file.filename,
                          taskType: mapPipelineTagToTask(
                            selectedRepo.pipeline_tag
                          ),
                        }) ? (
                          <>
                            <Check className='h-3.5 w-3.5' />
                            {t('Selected')}
                          </>
                        ) : (
                          <>
                            <Download className='h-3.5 w-3.5' />
                            {t('Select This Model')}
                          </>
                        )}
                      </Button>
                    </div>
                  ))
                )}
              </div>
            </div>
          </div>
        </div>

        <DialogFooter className='pt-2 border-t flex items-center justify-between gap-2'>
          {multiSelect ? (
            <div className='text-xs text-muted-foreground mr-auto'>
              {t('Selected models')}: <strong>{selectedItems.length}</strong>
            </div>
          ) : null}
          <Button
            type='button'
            variant='outline'
            onClick={() => onOpenChange(false)}
          >
            {t('Close')}
          </Button>
          {multiSelect && (
            <Button
              type='button'
              disabled={selectedItems.length === 0}
              onClick={() => {
                onSelectMultiple?.(selectedItems)
                setSelectedItems([])
                onOpenChange(false)
              }}
            >
              <Check className='h-4 w-4 mr-1.5' />
              {t('Add Selected ({{count}})', { count: selectedItems.length })}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
