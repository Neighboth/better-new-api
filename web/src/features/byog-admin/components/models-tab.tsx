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
import {
  ChevronDown,
  ChevronRight,
  Cpu,
  Download,
  HardDrive,
  HelpCircle,
  Loader2,
  Lock,
  Search,
  Sparkles,
} from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import {
  createAdminLora,
  createAdminModel,
  deleteAdminLora,
  deleteAdminModel,
  getAdminModels,
  updateAdminModel,
} from '../../byog/api'
import type { ByogModel, ByogModelLora } from '../../byog/types'

export function ByogModelsTab() {
  const { t } = useTranslation()
  const [models, setModels] = useState<ByogModel[]>([])
  const [loras, setLoras] = useState<ByogModelLora[]>([])
  const [loading, setLoading] = useState(true)
  const [expandedModelId, setExpandedModelId] = useState<string | null>(null)

  // Model Modal
  const [modelModalOpen, setModelModalOpen] = useState(false)
  const [editingModel, setEditingModel] = useState<Partial<ByogModel> | null>(
    null
  )

  // LoRA Modal
  const [loraModalOpen, setLoraModalOpen] = useState(false)
  const [selectedBaseModelId, setSelectedBaseModelId] = useState<string>('')
  const [selectedBaseModelDbId, setSelectedBaseModelDbId] = useState<number>(0)
  const [editingLora, setEditingLora] = useState<Partial<ByogModelLora> | null>(
    null
  )

  // Hugging Face Explorer State (Item 21)
  const [hfModalOpen, setHfModalOpen] = useState(false)
  const [hfSearchQuery, setHfSearchQuery] = useState('qwen2.5 gguf')
  const [hfToken, setHfToken] = useState('')
  const [hfSearching, setHfSearching] = useState(false)
  const [hfRepos, setHfRepos] = useState<any[]>([])
  const [hfSelectedRepo, setHfSelectedRepo] = useState<string | null>(null)
  const [hfLoadingFiles, setHfLoadingFiles] = useState(false)
  const [hfFiles, setHfFiles] = useState<any[]>([])
  const [hfGatedWarning, setHfGatedWarning] = useState('')

  async function searchHf(query?: string) {
    const q = query ?? hfSearchQuery
    setHfSearching(true)
    setHfGatedWarning('')
    try {
      const resp = await fetch(
        `/api/byog/admin/hf/search?q=${encodeURIComponent(q || 'gguf')}&token=${encodeURIComponent(hfToken)}`
      )
      const res = await resp.json()
      if (res.success) {
        setHfRepos(res.data || [])
      } else {
        toast.error(res.message || 'Hugging Face modelleri aranamadı')
      }
    } catch (e: any) {
      toast.error(String(e))
    } finally {
      setHfSearching(false)
    }
  }

  async function loadHfRepoFiles(repoId: string) {
    setHfSelectedRepo(repoId)
    setHfLoadingFiles(true)
    setHfFiles([])
    setHfGatedWarning('')
    try {
      const resp = await fetch(
        `/api/byog/admin/hf/files?repo=${encodeURIComponent(repoId)}&token=${encodeURIComponent(hfToken)}`
      )
      const res = await resp.json()
      if (res.gated) {
        setHfGatedWarning(res.message)
      } else if (res.success && res.data) {
        setHfFiles(res.data.files || [])
      } else {
        toast.error(res.message || 'Repo dosyaları listelenemedi')
      }
    } catch (e: any) {
      toast.error(String(e))
    } finally {
      setHfLoadingFiles(false)
    }
  }

  function handleSelectHfFile(file: any) {
    if (!editingModel) return
    const cleanName = file.filename
      .replace(/\.gguf$/i, '')
      .toLowerCase()
      .replace(/[^a-z0-9_-]/g, '-')
    let vendor = 'Community'
    if (hfSelectedRepo) {
      const parts = hfSelectedRepo.split('/')
      if (parts.length > 0) vendor = parts[0]
    }
    let paramSize = '7B'
    const lower = file.filename.toLowerCase()
    if (lower.includes('0.5b')) paramSize = '0.5B'
    else if (lower.includes('1.5b')) paramSize = '1.5B'
    else if (lower.includes('3b')) paramSize = '3B'
    else if (lower.includes('4b')) paramSize = '4B'
    else if (lower.includes('7b') || lower.includes('8b')) paramSize = '7B'
    else if (lower.includes('14b')) paramSize = '14B'
    else if (lower.includes('70b') || lower.includes('72b')) paramSize = '70B'

    setEditingModel({
      ...editingModel,
      model_id: cleanName,
      vendor: vendor,
      param_size: paramSize,
      download_url: file.download_url,
      file_size: file.size || file.size_mb * 1024 * 1024,
    })
    setHfModalOpen(false)
    toast.success(`${file.filename} bilgileri forma aktarıldı!`)
  }

  function fillRecommendedAntiCheat() {
    if (!editingModel) return
    setEditingModel({
      ...editingModel,
      anti_cheat_logic_prompt: 'What is 2+2?',
      anti_cheat_logic_expected: '4',
      anti_cheat_logic_temp: 0,
      anti_cheat_logic_seed: 42,
      anti_cheat_logic_logprob: -0.001,
      anti_cheat_logic_tolerance: 0.01,
      anti_cheat_sig_prompt: 'Hello world',
      anti_cheat_sig_expected: 'Hello! How can I help you today?',
      anti_cheat_sig_temp: 0,
      anti_cheat_sig_seed: 8472910482910482,
      anti_cheat_sig_logprob: -0.0124,
      anti_cheat_sig_tolerance: 0.005,
    })
    toast.success('Önerilen deterministik anti-cheat değerleri dolduruldu')
  }

  useEffect(() => {
    loadData()
  }, [])

  async function loadData() {
    try {
      setLoading(true)
      const data = await getAdminModels()
      if (data) {
        setModels(data.models || [])
        setLoras(data.loras || [])
      }
    } catch (err: any) {
      toast.error(err.message || t('byog.admin.load_models_failed'))
    } finally {
      setLoading(false)
    }
  }

  function handleOpenCreateModel() {
    setEditingModel({
      model_id: '',
      model_type: 'llm',
      vendor: 'Qwen',
      description: '',
      param_size: '7B',
      download_url: '',
      file_size: 4600000000,
      chunk_size: 10485760,
      chunk_manifest: '[]',
      modalities_input: 'text',
      modalities_output: 'text',
      vision_mmproj_url: '',
      anti_cheat_logic_prompt: '',
      anti_cheat_logic_temp: 0,
      anti_cheat_logic_seed: 42,
      anti_cheat_logic_expected: '',
      anti_cheat_logic_logprob: -0.008,
      anti_cheat_logic_tolerance: 0.004,
      anti_cheat_sig_prompt: '',
      anti_cheat_sig_temp: 0,
      anti_cheat_sig_seed: 8472910482910482,
      anti_cheat_sig_expected: '',
      anti_cheat_sig_logprob: -0.0124,
      anti_cheat_sig_tolerance: 0.002,
      is_enabled: true,
    })
    setModelModalOpen(true)
  }

  function handleOpenEditModel(m: ByogModel) {
    setEditingModel({ ...m })
    setModelModalOpen(true)
  }

  async function handleSaveModel() {
    if (!editingModel || !editingModel.model_id) {
      toast.error(t('byog.admin.model_id_required'))
      return
    }
    try {
      if (editingModel.id) {
        await updateAdminModel(editingModel.id, editingModel)
        toast.success(t('byog.admin.model_updated_success'))
      } else {
        await createAdminModel(editingModel)
        toast.success(t('byog.admin.model_created_success'))
      }
      setModelModalOpen(false)
      loadData()
    } catch (err: any) {
      toast.error(err.message || t('byog.admin.save_model_failed'))
    }
  }

  async function handleDeleteModel(id: number) {
    if (!window.confirm(t('byog.admin.confirm_delete_model'))) return
    try {
      await deleteAdminModel(id)
      toast.success(t('byog.admin.model_deleted_success'))
      loadData()
    } catch (err: any) {
      toast.error(err.message || t('byog.admin.delete_model_failed'))
    }
  }

  function handleOpenCreateLora(model: ByogModel) {
    setSelectedBaseModelId(model.model_id)
    setSelectedBaseModelDbId(model.id)
    setEditingLora({
      base_model_id: model.model_id,
      lora_id: `${model.model_id}-adapter-1`,
      download_url: '',
      file_size: 150000000,
      chunk_manifest: '[]',
      modality_tags: 'coding',
      anti_cheat_prompt: 'Write a Python function that adds two numbers.',
      anti_cheat_seed: 12345,
      anti_cheat_expected: 'def add(a, b):',
      anti_cheat_logprob: -0.02,
      anti_cheat_tolerance: 0.01,
      is_enabled: true,
    })
    setLoraModalOpen(true)
  }

  async function handleSaveLora() {
    if (!editingLora || !editingLora.lora_id) return
    try {
      await createAdminLora(selectedBaseModelDbId, editingLora)
      toast.success(t('byog.admin.lora_created_success'))
      setLoraModalOpen(false)
      loadData()
    } catch (err: any) {
      toast.error(err.message || t('byog.admin.save_lora_failed'))
    }
  }

  async function handleDeleteLora(id: number) {
    if (!window.confirm(t('byog.admin.confirm_delete_lora'))) return
    try {
      await deleteAdminLora(id)
      toast.success(t('byog.admin.lora_deleted_success'))
      loadData()
    } catch (err: any) {
      toast.error(err.message || t('byog.admin.delete_lora_failed'))
    }
  }

  return (
    <div className='space-y-6'>
      <Card>
        <CardHeader>
          <div className='flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between'>
            <div>
              <CardTitle>
                {t('byog.admin.models_loras_title')}
              </CardTitle>
              <CardDescription>
                {t('byog.admin.models_loras_desc')}
              </CardDescription>
            </div>
            <Button onClick={handleOpenCreateModel}>
              {t('byog.admin.add_base_model')}
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {loading ? (
            <div className='text-muted-foreground flex h-32 items-center justify-center'>
              {t('byog.loading')}
            </div>
          ) : models.length === 0 ? (
            <div className='text-muted-foreground py-12 text-center'>
              {t('byog.admin.no_models_found')}
            </div>
          ) : (
            <div className='space-y-4'>
              {models.map((model) => {
                const modelLoras = loras.filter(
                  (l) => l.base_model_id === model.model_id
                )
                const isExpanded = expandedModelId === model.model_id

                return (
                  <div
                    key={model.id}
                    className='bg-card overflow-hidden rounded-xl border'
                  >
                    <div className='bg-muted/20 flex flex-col justify-between gap-4 p-4 md:flex-row md:items-center'>
                      <div className='flex items-start gap-3 md:items-center'>
                        <button
                          type='button'
                          onClick={() =>
                            setExpandedModelId(
                              isExpanded ? null : model.model_id
                            )
                          }
                          className='text-muted-foreground hover:text-foreground mt-1 md:mt-0'
                        >
                          {isExpanded ? (
                            <ChevronDown className='h-5 w-5' />
                          ) : (
                            <ChevronRight className='h-5 w-5' />
                          )}
                        </button>
                        <div className='space-y-1'>
                          <div className='flex flex-wrap items-center gap-2'>
                            <span className='text-base font-bold'>
                              {model.model_id}
                            </span>
                            <Badge
                              variant='outline'
                              className='text-xs capitalize'
                            >
                              {model.model_type}
                            </Badge>
                            <Badge variant='secondary' className='text-xs'>
                              {model.param_size}
                            </Badge>
                            {modelLoras.length > 0 && (
                              <Badge variant='outline' className='text-xs font-mono'>
                                {modelLoras.length} LoRA
                              </Badge>
                            )}
                          </div>
                          <div className='text-muted-foreground text-xs'>
                            {model.vendor} •{' '}
                            {(model.file_size / (1024 * 1024 * 1024)).toFixed(
                              2
                            )}{' '}
                            GB • In: [{model.modalities_input}] Out: [
                            {model.modalities_output}]
                          </div>
                        </div>
                      </div>

                      <div className='flex items-center gap-2 self-end md:self-auto'>
                        <Button
                          variant='outline'
                          size='sm'
                          onClick={() => handleOpenCreateLora(model)}
                          className='h-7 px-2 text-xs'
                        >
                          {t('byog.admin.add_lora')}
                        </Button>
                        <Button
                          variant='outline'
                          size='sm'
                          className='h-7 px-2 text-xs'
                          onClick={() => handleOpenEditModel(model)}
                        >
                          {t('common.edit')}
                        </Button>
                        <Button
                          variant='outline'
                          size='sm'
                          className='h-7 px-2 text-xs text-destructive hover:text-destructive'
                          onClick={() => handleDeleteModel(model.id)}
                        >
                          {t('common.delete')}
                        </Button>
                      </div>
                    </div>

                    {/* Anti-cheat audit summary */}
                    <div className='bg-muted/5 text-muted-foreground flex items-center gap-4 border-t px-4 py-2.5 text-xs'>
                      <div className='text-primary flex items-center gap-1.5'>
                        <span>{t('byog.admin.anti_cheat_verified')}</span>
                      </div>
                      <div className='max-w-md truncate font-mono'>
                        Logic: "{model.anti_cheat_logic_expected}" (tol: ±
                        {model.anti_cheat_logic_tolerance}) | Sig: "
                        {model.anti_cheat_sig_expected}"
                      </div>
                    </div>

                    {/* Expanded LoRAs list */}
                    {isExpanded && (
                      <div className='bg-background space-y-3 border-t p-4'>
                        <div className='text-muted-foreground flex items-center gap-1.5 text-xs font-semibold tracking-wider uppercase'>
                          {t('byog.admin.attached_loras')} ({modelLoras.length})
                        </div>
                        {modelLoras.length === 0 ? (
                          <div className='text-muted-foreground py-2 text-xs italic'>
                            {t('byog.admin.no_loras_attached')}
                          </div>
                        ) : (
                          <div className='space-y-2'>
                            {modelLoras.map((lora) => (
                              <div
                                key={lora.id}
                                className='bg-muted/10 flex items-center justify-between rounded-lg border p-3 text-xs'
                              >
                                <div className='space-y-0.5'>
                                  <div className='flex items-center gap-2 text-sm font-semibold'>
                                    {lora.lora_id}
                                    <Badge
                                      variant='outline'
                                      className='text-[10px]'
                                    >
                                      {lora.modality_tags}
                                    </Badge>
                                  </div>
                                  <div className='text-muted-foreground max-w-lg truncate font-mono'>
                                    {(lora.file_size / (1024 * 1024)).toFixed(
                                      1
                                    )}{' '}
                                    MB • {lora.download_url}
                                  </div>
                                </div>
                                <Button
                                  variant='outline'
                                  size='sm'
                                  className='text-destructive hover:text-destructive h-6 px-2 text-xs'
                                  onClick={() => handleDeleteLora(lora.id)}
                                >
                                  {t('common.delete')}
                                </Button>
                              </div>
                            ))}
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                )
              })}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Base Model Dialog */}
      <Dialog open={modelModalOpen} onOpenChange={setModelModalOpen}>
        <DialogContent className='max-h-[90vh] max-w-3xl overflow-y-auto'>
          <DialogHeader>
            <DialogTitle>
              {editingModel?.id
                ? t('byog.admin.edit_base_model')
                : t('byog.admin.create_base_model')}
            </DialogTitle>
            <DialogDescription>
              {t('byog.admin.base_model_dialog_desc')}
            </DialogDescription>
          </DialogHeader>

          {editingModel && (
            <div className='grid grid-cols-1 gap-4 py-4 md:grid-cols-2'>
              <div className='space-y-2'>
                <Label>{t('byog.admin.model_id')}</Label>
                <Input
                  value={editingModel.model_id || ''}
                  onChange={(e) =>
                    setEditingModel({
                      ...editingModel,
                      model_id: e.target.value,
                    })
                  }
                  placeholder='e.g. qwen-2.5-7b-instruct'
                />
              </div>

              <div className='space-y-2'>
                <Label>{t('byog.admin.vendor')}</Label>
                <Input
                  value={editingModel.vendor || ''}
                  onChange={(e) =>
                    setEditingModel({ ...editingModel, vendor: e.target.value })
                  }
                  placeholder='e.g. Qwen, Mistral, Meta'
                />
              </div>

              <div className='space-y-2'>
                <Label>{t('byog.admin.model_type')}</Label>
                <Select
                  value={editingModel.model_type || 'llm'}
                  onValueChange={(val) =>
                    setEditingModel({
                      ...editingModel,
                      model_type: val || 'llm',
                    })
                  }
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='llm'>LLM (Chat / Text)</SelectItem>
                    <SelectItem value='embedding'>
                      Embedding (Vector)
                    </SelectItem>
                    <SelectItem value='reranker'>Reranker</SelectItem>
                    <SelectItem value='vision'>Vision (Multimodal)</SelectItem>
                    <SelectItem value='image_gen'>Image Generation</SelectItem>
                    <SelectItem value='video_gen'>Video Generation</SelectItem>
                    <SelectItem value='video'>Video (Text-to-Video)</SelectItem>
                    <SelectItem value='audio'>Audio / Speech</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <div className='space-y-2'>
                <Label>{t('byog.admin.param_size')}</Label>
                <Input
                  value={editingModel.param_size || ''}
                  onChange={(e) =>
                    setEditingModel({
                      ...editingModel,
                      param_size: e.target.value,
                    })
                  }
                  placeholder='e.g. 0.5B, 1.5B, 7B, 14B'
                />
              </div>

              <div className='space-y-2'>
                <Label>{t('byog.admin.file_size_bytes')}</Label>
                <Input
                  type='number'
                  value={editingModel.file_size || 0}
                  onChange={(e) =>
                    setEditingModel({
                      ...editingModel,
                      file_size: parseInt(e.target.value) || 0,
                    })
                  }
                />
                <span className='text-muted-foreground text-xs'>
                  {(
                    (editingModel.file_size || 0) /
                    (1024 * 1024 * 1024)
                  ).toFixed(2)}{' '}
                  GB
                </span>
              </div>

              <div className='space-y-2 md:col-span-2'>
                <div className='flex items-center justify-between'>
                  <Label>{t('byog.admin.download_url')}</Label>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    className='gap-1.5 text-xs text-primary h-7'
                    onClick={() => {
                      setHfModalOpen(true)
                      if (hfRepos.length === 0) searchHf('qwen2.5 gguf')
                    }}
                  >
                    <Search className='h-3.5 w-3.5' />
                    Hugging Face'ten Model Ara
                  </Button>
                </div>
                <Input
                  value={editingModel.download_url || ''}
                  onChange={(e) =>
                    setEditingModel({
                      ...editingModel,
                      download_url: e.target.value,
                    })
                  }
                  placeholder='https://.../model-q4_k_m.gguf'
                />
              </div>

              <div className='space-y-2'>
                <Label>{t('byog.admin.input_modalities')}</Label>
                <Input
                  value={editingModel.modalities_input || ''}
                  onChange={(e) =>
                    setEditingModel({
                      ...editingModel,
                      modalities_input: e.target.value,
                    })
                  }
                  placeholder='text, image'
                />
              </div>

              <div className='space-y-2'>
                <Label>{t('byog.admin.output_modalities')}</Label>
                <Input
                  value={editingModel.modalities_output || ''}
                  onChange={(e) =>
                    setEditingModel({
                      ...editingModel,
                      modalities_output: e.target.value,
                    })
                  }
                  placeholder='text'
                />
              </div>

              <div className='space-y-2 md:col-span-2'>
                <Label>{t('byog.admin.vision_mmproj_url')}</Label>
                <Input
                  value={editingModel.vision_mmproj_url || ''}
                  onChange={(e) =>
                    setEditingModel({
                      ...editingModel,
                      vision_mmproj_url: e.target.value,
                    })
                  }
                  placeholder='Optional mmproj adapter URL for vision models'
                />
              </div>

              {/* Anti-cheat Logic Exam */}
              <div className='bg-muted/20 space-y-4 rounded-xl border p-4 md:col-span-2'>
                <div className='flex flex-wrap items-center justify-between gap-2 border-b pb-2.5'>
                  <div className='flex items-center gap-2'>
                    <HelpCircle className='h-4 w-4 text-primary' />
                    <span className='text-sm font-semibold'>
                      {t('byog.admin.anti_cheat_logic_exam_title')} ({t('Optional')})
                    </span>
                  </div>
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    className='gap-1 text-xs text-primary h-7'
                    onClick={fillRecommendedAntiCheat}
                  >
                    <Sparkles className='h-3.5 w-3.5 text-amber-500' />
                    Önerilen Değerleri Uygula
                  </Button>
                </div>

                <div className='bg-muted/40 rounded-lg p-2.5 text-xs text-muted-foreground space-y-1 border'>
                  <p className='font-semibold text-foreground'>
                    💡 Seed ve Logprob Değerleri Nedir, Nasıl Öğrenilir?
                  </p>
                  <p>
                    Anti-cheat canaries, cihazların sahte çıktı üretmesini veya daha küçük modeller kullanmasını önlemek için deterministik (sıcaklık=0, sabit seed) testler yapar.
                  </p>
                  <p>
                    • <strong>Opsiyoneldir:</strong> Boş bırakırsanız bu doğrulama atlanır ve model normal çalışmaya devam eder.
                  </p>
                  <p>
                    • <strong>Öğrenme Yöntemi:</strong> Modeli llama.cpp ile test edip ilk token logprob değerini görebilirsiniz veya <em>Önerilen Değerleri Uygula</em> tuşuna basabilirsiniz.
                  </p>
                </div>
                <div className='grid grid-cols-1 gap-3 md:grid-cols-2'>
                  <div className='space-y-1 md:col-span-2'>
                    <Label className='text-xs'>
                      {t('byog.admin.test_prompt')} ({t('Optional')})
                    </Label>
                    <Input
                      placeholder={t('Optional - Leave empty to disable logic verification')}
                      value={editingModel.anti_cheat_logic_prompt || ''}
                      onChange={(e) =>
                        setEditingModel({
                          ...editingModel,
                          anti_cheat_logic_prompt: e.target.value,
                        })
                      }
                    />
                  </div>
                  <div className='space-y-1'>
                    <Label className='text-xs'>
                      {t('byog.admin.expected_response')}
                    </Label>
                    <Input
                      value={editingModel.anti_cheat_logic_expected || ''}
                      onChange={(e) =>
                        setEditingModel({
                          ...editingModel,
                          anti_cheat_logic_expected: e.target.value,
                        })
                      }
                    />
                  </div>
                  <div className='space-y-1'>
                    <Label className='text-xs'>{t('byog.admin.seed')}</Label>
                    <Input
                      type='number'
                      value={editingModel.anti_cheat_logic_seed || 42}
                      onChange={(e) =>
                        setEditingModel({
                          ...editingModel,
                          anti_cheat_logic_seed: parseInt(e.target.value) || 42,
                        })
                      }
                    />
                  </div>
                  <div className='space-y-1'>
                    <Label className='text-xs'>
                      {t('byog.admin.expected_logprob')}
                    </Label>
                    <Input
                      type='number'
                      step='0.0001'
                      value={editingModel.anti_cheat_logic_logprob || -0.008}
                      onChange={(e) =>
                        setEditingModel({
                          ...editingModel,
                          anti_cheat_logic_logprob:
                            parseFloat(e.target.value) || 0,
                        })
                      }
                    />
                  </div>
                  <div className='space-y-1'>
                    <Label className='text-xs'>
                      {t('byog.admin.tolerance')}
                    </Label>
                    <Input
                      type='number'
                      step='0.0001'
                      value={editingModel.anti_cheat_logic_tolerance || 0.004}
                      onChange={(e) =>
                        setEditingModel({
                          ...editingModel,
                          anti_cheat_logic_tolerance:
                            parseFloat(e.target.value) || 0,
                        })
                      }
                    />
                  </div>
                </div>
              </div>

              {/* Anti-cheat Signature Exam */}
              <div className='bg-muted/20 space-y-4 rounded-xl border p-4 md:col-span-2'>
                <div className='flex items-center gap-2'>
                  <span className='text-sm font-semibold'>
                    {t('byog.admin.anti_cheat_sig_exam_title')} ({t('Optional')})
                  </span>
                </div>
                <div className='grid grid-cols-1 gap-3 md:grid-cols-2'>
                  <div className='space-y-1 md:col-span-2'>
                    <Label className='text-xs'>
                      {t('byog.admin.test_prompt')} ({t('Optional')})
                    </Label>
                    <Input
                      placeholder={t('Optional - Leave empty to disable canary verification')}
                      value={editingModel.anti_cheat_sig_prompt || ''}
                      onChange={(e) =>
                        setEditingModel({
                          ...editingModel,
                          anti_cheat_sig_prompt: e.target.value,
                        })
                      }
                    />
                  </div>
                  <div className='space-y-1'>
                    <Label className='text-xs'>
                      {t('byog.admin.expected_response')}
                    </Label>
                    <Input
                      value={editingModel.anti_cheat_sig_expected || ''}
                      onChange={(e) =>
                        setEditingModel({
                          ...editingModel,
                          anti_cheat_sig_expected: e.target.value,
                        })
                      }
                    />
                  </div>
                  <div className='space-y-1'>
                    <Label className='text-xs'>
                      {t('byog.admin.deterministic_64bit_seed')}
                    </Label>
                    <Input
                      type='number'
                      value={
                        editingModel.anti_cheat_sig_seed || 8472910482910482
                      }
                      onChange={(e) =>
                        setEditingModel({
                          ...editingModel,
                          anti_cheat_sig_seed: parseInt(e.target.value) || 0,
                        })
                      }
                    />
                  </div>
                  <div className='space-y-1'>
                    <Label className='text-xs'>
                      {t('byog.admin.expected_logprob')}
                    </Label>
                    <Input
                      type='number'
                      step='0.0001'
                      value={editingModel.anti_cheat_sig_logprob || -0.0124}
                      onChange={(e) =>
                        setEditingModel({
                          ...editingModel,
                          anti_cheat_sig_logprob:
                            parseFloat(e.target.value) || 0,
                        })
                      }
                    />
                  </div>
                  <div className='space-y-1'>
                    <Label className='text-xs'>
                      {t('byog.admin.tolerance')}
                    </Label>
                    <Input
                      type='number'
                      step='0.0001'
                      value={editingModel.anti_cheat_sig_tolerance || 0.002}
                      onChange={(e) =>
                        setEditingModel({
                          ...editingModel,
                          anti_cheat_sig_tolerance:
                            parseFloat(e.target.value) || 0,
                        })
                      }
                    />
                  </div>
                </div>
              </div>
            </div>
          )}

          <DialogFooter>
            <Button variant='outline' onClick={() => setModelModalOpen(false)}>
              {t('byog.cancel')}
            </Button>
            <Button onClick={handleSaveModel}>{t('byog.save')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* LoRA Dialog */}
      <Dialog open={loraModalOpen} onOpenChange={setLoraModalOpen}>
        <DialogContent className='max-w-xl'>
          <DialogHeader>
            <DialogTitle>
              {t('byog.admin.add_lora_for', { modelId: selectedBaseModelId })}
            </DialogTitle>
            <DialogDescription>
              {t('byog.admin.lora_dialog_desc')}
            </DialogDescription>
          </DialogHeader>

          {editingLora && (
            <div className='space-y-4 py-2'>
              <div className='space-y-2'>
                <Label>{t('byog.admin.lora_id')}</Label>
                <Input
                  value={editingLora.lora_id || ''}
                  onChange={(e) =>
                    setEditingLora({ ...editingLora, lora_id: e.target.value })
                  }
                  placeholder='e.g. qwen-2.5-7b-coder-pro'
                />
              </div>

              <div className='space-y-2'>
                <Label>{t('byog.admin.download_url')}</Label>
                <Input
                  value={editingLora.download_url || ''}
                  onChange={(e) =>
                    setEditingLora({
                      ...editingLora,
                      download_url: e.target.value,
                    })
                  }
                  placeholder='https://.../adapter.bin'
                />
              </div>

              <div className='space-y-2'>
                <Label>{t('byog.admin.modality_tags')}</Label>
                <Input
                  value={editingLora.modality_tags || ''}
                  onChange={(e) =>
                    setEditingLora({
                      ...editingLora,
                      modality_tags: e.target.value,
                    })
                  }
                  placeholder='coding, sql, creative'
                />
              </div>
            </div>
          )}

          <DialogFooter>
            <Button variant='outline' onClick={() => setLoraModalOpen(false)}>
              {t('byog.cancel')}
            </Button>
            <Button onClick={handleSaveLora}>{t('byog.save')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Hugging Face Model Explorer Modal (Item 21) */}
      <Dialog open={hfModalOpen} onOpenChange={setHfModalOpen}>
        <DialogContent className='max-w-4xl max-h-[85vh] flex flex-col overflow-hidden'>
          <DialogHeader>
            <DialogTitle className='flex items-center gap-2'>
              <Search className='h-5 w-5 text-primary' />
              Hugging Face Model & Quantization Gezgini
            </DialogTitle>
            <DialogDescription>
              Hugging Face üzerindeki tüm açık kaynak modelleri arayın, quantize GGUF dosyalarını inceleyin ve doğrudan ekleyin.
            </DialogDescription>
          </DialogHeader>

          <div className='space-y-4 py-2 flex-1 overflow-y-auto'>
            {/* Search Input & Gated Token Input */}
            <div className='grid grid-cols-1 md:grid-cols-3 gap-2'>
              <div className='md:col-span-2 flex gap-2'>
                <Input
                  value={hfSearchQuery}
                  onChange={(e) => setHfSearchQuery(e.target.value)}
                  onKeyDown={(e) => e.key === 'Enter' && searchHf()}
                  placeholder='Model adı arayın (örn. gemma-3, qwen2.5, llama-3)...'
                />
                <Button onClick={() => searchHf()} disabled={hfSearching}>
                  {hfSearching ? <Loader2 className='h-4 w-4 animate-spin' /> : <Search className='h-4 w-4' />}
                  Ara
                </Button>
              </div>
              <div className='flex gap-1.5 items-center'>
                <Input
                  type='password'
                  value={hfToken}
                  onChange={(e) => setHfToken(e.target.value)}
                  placeholder='HF Token (Gated modeller için)'
                  className='text-xs'
                />
              </div>
            </div>

            {/* Gated warning banner */}
            {hfGatedWarning ? (
              <div className='bg-amber-500/10 border border-amber-500/30 text-amber-500 rounded-lg p-3 text-xs flex items-center gap-2'>
                <Lock className='h-4 w-4 shrink-0' />
                <span>{hfGatedWarning}</span>
              </div>
            ) : null}

            {/* Split view: Left Repos, Right Quantized Files */}
            <div className='grid grid-cols-1 md:grid-cols-2 gap-4 h-[420px]'>
              {/* Repos list */}
              <div className='border rounded-xl p-2 overflow-y-auto space-y-2'>
                <div className='text-xs font-semibold text-muted-foreground px-1 pb-1 border-b'>
                  Bulunan Model Repoları ({hfRepos.length})
                </div>
                {hfSearching ? (
                  <div className='flex items-center justify-center h-48 text-muted-foreground text-xs gap-2'>
                    <Loader2 className='h-4 w-4 animate-spin' /> Hugging Face taranıyor...
                  </div>
                ) : hfRepos.length === 0 ? (
                  <div className='text-center text-muted-foreground text-xs py-8'>
                    Model bulunamadı. Arama kutusuna bir model adı yazıp 'Ara' butonuna basın.
                  </div>
                ) : (
                  hfRepos.map((repo) => (
                    <div
                      key={repo.id}
                      onClick={() => loadHfRepoFiles(repo.id)}
                      className={`p-2.5 rounded-lg border text-xs cursor-pointer transition-colors ${
                        hfSelectedRepo === repo.id
                          ? 'border-primary bg-primary/10'
                          : 'hover:bg-muted/50 border-border'
                      }`}
                    >
                      <div className='font-bold truncate text-foreground'>{repo.id}</div>
                      <div className='flex items-center gap-3 text-muted-foreground text-[10px] mt-1'>
                        <span>İndirme: {repo.downloads?.toLocaleString() || 0}</span>
                        <span>Beğeni: {repo.likes?.toLocaleString() || 0}</span>
                        {repo.gated ? (
                          <Badge variant='outline' className='text-[9px] border-amber-500/50 text-amber-500'>
                            Gated (Şartlı)
                          </Badge>
                        ) : null}
                      </div>
                    </div>
                  ))
                )}
              </div>

              {/* Quantization Files list */}
              <div className='border rounded-xl p-2 overflow-y-auto space-y-2'>
                <div className='text-xs font-semibold text-muted-foreground px-1 pb-1 border-b'>
                  {hfSelectedRepo ? `${hfSelectedRepo} Dosyaları` : 'Bir repo seçin'}
                </div>

                {hfLoadingFiles ? (
                  <div className='flex items-center justify-center h-48 text-muted-foreground text-xs gap-2'>
                    <Loader2 className='h-4 w-4 animate-spin' /> Dosyalar taranıyor...
                  </div>
                ) : !hfSelectedRepo ? (
                  <div className='text-center text-muted-foreground text-xs py-8'>
                    Soldaki listeden bir modele tıklayarak içindeki quantize GGUF dosyalarını görüntüleyin.
                  </div>
                ) : hfFiles.length === 0 ? (
                  <div className='text-center text-muted-foreground text-xs py-8'>
                    Bu repoda doğrudan kullanılabilir GGUF dosyası bulunamadı veya model gated token gerektiriyor.
                  </div>
                ) : (
                  hfFiles.map((file, idx) => (
                    <div
                      key={idx}
                      className='p-3 rounded-lg border bg-card/60 hover:border-primary/60 transition-all space-y-2 text-xs'
                    >
                      <div className='flex items-start justify-between gap-2'>
                        <div className='font-mono font-bold truncate text-foreground max-w-[240px]' title={file.filename}>
                          {file.filename}
                        </div>
                        <Badge variant='secondary' className='text-[10px] shrink-0 font-bold'>
                          {file.quantization}
                        </Badge>
                      </div>

                      <div className='flex items-center gap-3 text-[11px] text-muted-foreground'>
                        <span>Boyut: <strong>{file.size_gb} GB</strong></span>
                      </div>

                      {/* Hardware recommendations badge */}
                      <div className='bg-muted/40 rounded-md p-2 text-[11px] text-muted-foreground flex flex-wrap gap-2 border'>
                        <span className='flex items-center gap-1'>
                          <HardDrive className='h-3 w-3 text-primary' />
                          Min RAM: <strong>{file.recommended_ram_gb} GB</strong>
                        </span>
                        <span className='flex items-center gap-1'>
                          <Cpu className='h-3 w-3 text-emerald-500' />
                          Min VRAM: <strong>{file.recommended_vram_gb} GB</strong>
                        </span>
                        <span className='flex items-center gap-1'>
                          Çekirdek: <strong>{file.recommended_cores}</strong>
                        </span>
                      </div>

                      <Button
                        size='sm'
                        className='w-full text-xs h-7 gap-1 mt-1'
                        onClick={() => handleSelectHfFile(file)}
                      >
                        <Download className='h-3.5 w-3.5' />
                        Bu Modeli Seç ve Ekle
                      </Button>
                    </div>
                  ))
                )}
              </div>
            </div>
          </div>

          <DialogFooter>
            <Button variant='outline' onClick={() => setHfModalOpen(false)}>
              Kapat
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
