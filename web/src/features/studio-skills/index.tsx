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

/**
 * Admin console for Studio Skills.
 *
 * A skill is a named creative direction the Studio's prompt optimizer
 * applies when the user picks it in the composer (e.g. "GTA 6-style crime
 * saga", "A24 psychological trailer"). Skills live installation-wide; every
 * authenticated Studio user sees the enabled ones.
 */
import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import {
  ArrowLeft,
  CheckCircle2,
  FileText,
  Loader2,
  Pencil,
  PlusIcon,
  Trash2,
  Upload,
  Wand2,
} from 'lucide-react'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { SectionPageLayout } from '@/components/layout'

import {
  createSkill,
  deleteSkill,
  listSkills,
  updateSkill,
  uploadSkillImage,
  type StudioSkill,
} from './api'

const TEMPLATE_SKILL = {
  title: 'GTA 6 — Open World Crime Saga',
  description:
    'A gritty open-world crime story in the style of Grand Theft Auto VI: neon-drenched city, cinematic camera work, gang politics and high-stakes chases.',
  image: '',
  content: `You are recreating the visual language of Grand Theft Auto VI for this video. Rules:
- Setting: a sun-bleached coastal metropolis with Art Deco neon, luxury penthouses and grimy backstreets (a Vice-City-like vibe, never named after a real district).
- Camera: slow cinematic drifts, crane flyovers of the skyline, neon reflections on wet asphalt at dusk, police-helicopter spotlight cuts, wide establishing aerials of the city grid.
- Characters: stylish opportunistic criminals, sharp dialogue, tense alliances, betrayal brewing under the surface.
- Tone: satirical but grounded; spectacle with consequences; humor and menace in equal measure.
- Physics & scale: realistic car chases with tire smoke and drifting, sparks off crash barriers, slow-motion impacts, crowd panic, echoing sirens.
- Lighting: golden-hour haze, sodium-vapor oranges, neon magentas, deep cyan shadows, thick humid air.
- Never recreate actual copyrighted designs, logos, badges or trade dress from the real game — only the broad genre parody of the look.`,
  sort_order: 0,
}

function SkillCard(props: {
  skill: StudioSkill
  deleting: boolean
  onEdit: () => void
  onDelete: () => void
  onToggle: (enabled: boolean) => void
}) {
  const { t } = useTranslation()
  const { skill } = props
  return (
    <Card className='flex flex-col overflow-hidden'>
      <div className='relative aspect-[16/8] w-full overflow-hidden bg-muted'>
        {skill.image ? (
          <img
            alt={skill.title}
            className='size-full object-cover'
            loading='lazy'
            src={skill.image}
          />
        ) : (
          <div className='flex size-full items-center justify-center'>
            <FileText className='size-10 text-muted-foreground/60' />
          </div>
        )}
        <div className='absolute end-2 top-2'>
          <Switch
            aria-label={t('Enabled')}
            checked={skill.enabled}
            onCheckedChange={(checked) => props.onToggle(checked)}
          />
        </div>
      </div>
      <CardContent className='flex flex-1 flex-col gap-2 p-3'>
        <div className='min-w-0'>
          <p className='truncate text-sm font-semibold'>{skill.title}</p>
          <p className='line-clamp-2 text-[11px] leading-snug text-muted-foreground'>
            {skill.description || '—'}
          </p>
        </div>
        <p className='line-clamp-2 font-mono text-[10px] leading-snug text-muted-foreground/80'>
          {skill.content}
        </p>
        <div className='mt-auto flex items-center justify-between gap-2 pt-1'>
          <span className='text-[10px] font-mono text-muted-foreground'>#{skill.sort_order}</span>
          <div className='flex items-center gap-1'>
            <Button
              aria-label={t('Edit skill')}
              onClick={props.onEdit}
              size='icon'
              variant='ghost'
            >
              <Pencil className='size-3.5' />
            </Button>
            <Button
              aria-label={t('Delete skill')}
              disabled={props.deleting}
              onClick={props.onDelete}
              size='icon'
              variant='ghost'
            >
              {props.deleting ? (
                <Loader2 className='size-3.5 animate-spin' />
              ) : (
                <Trash2 className='size-3.5 text-destructive' />
              )}
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

export function StudioSkillsPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  const skillsQuery = useQuery({
    queryKey: ['studio-skills'],
    queryFn: listSkills,
  })

  const [editorOpen, setEditorOpen] = useState(false)
  const [editing, setEditing] = useState<StudioSkill | null>(null)
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [image, setImage] = useState('')
  const [content, setContent] = useState('')
  const [sortOrder, setSortOrder] = useState(0)
  const [enabled, setEnabled] = useState(true)
  const [saving, setSaving] = useState(false)
  const [uploading, setUploading] = useState(false)
  const [deletingId, setDeletingId] = useState<string | null>(null)

  const openEditor = (skill: StudioSkill | null) => {
    setEditing(skill)
    setTitle(skill?.title ?? '')
    setDescription(skill?.description ?? '')
    setImage(skill?.image ?? '')
    setContent(skill?.content ?? '')
    setSortOrder(skill?.sort_order ?? 0)
    setEnabled(skill?.enabled ?? true)
    setEditorOpen(true)
  }

  const applyTemplate = () => {
    setEditing(null)
    setTitle(TEMPLATE_SKILL.title)
    setDescription(TEMPLATE_SKILL.description)
    setImage(TEMPLATE_SKILL.image)
    setContent(TEMPLATE_SKILL.content)
    setSortOrder(TEMPLATE_SKILL.sort_order)
    setEnabled(true)
  }

  const saveMutation = useMutation({
    mutationFn: async () => {
      setSaving(true)
      const payload = {
        title: title.trim(),
        description: description.trim(),
        image: image.trim(),
        content: content.trim(),
        sort_order: Number.isFinite(sortOrder) ? Math.max(0, Math.floor(sortOrder)) : 0,
      }
      if (editing) {
        await updateSkill(editing.id, { ...payload, enabled })
      } else {
        await createSkill(payload)
      }
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['studio-skills'] })
      toast.success(editing ? t('Skill updated') : t('Skill created'))
      setEditorOpen(false)
      setEditing(null)
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to save the skill'))
    },
    onSettled: () => setSaving(false),
  })

  const deleteMutation = useMutation({
    mutationFn: deleteSkill,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['studio-skills'] })
      toast.success(t('Skill deleted'))
      setDeletingId(null)
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to delete the skill'))
      setDeletingId(null)
    },
  })

  const toggleMutation = useMutation({
    mutationFn: ({ id, enabled: next }: { id: string; enabled: boolean }) =>
      updateSkill(id, { enabled: next }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['studio-skills'] })
    },
    onError: (error: Error) => {
      toast.error(error.message || t('Failed to update the skill'))
    },
  })

  const skills = useMemo(
    () =>
      [...(skillsQuery.data ?? [])].sort(
        (a, b) => a.sort_order - b.sort_order || a.title.localeCompare(b.title)
      ),
    [skillsQuery.data]
  )

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        <span className='inline-flex min-w-0 items-center gap-2'>
          <span className='truncate'>{t('Studio Skills')}</span>
          <Badge variant='outline' className='shrink-0'>
            Admin
          </Badge>
        </span>
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='mx-auto flex w-full max-w-6xl flex-col gap-4'>
          <div className='flex flex-wrap items-center justify-between gap-3'>
            <div className='min-w-0'>
              <p className='text-sm text-muted-foreground'>
                {t('Reusable creative directions for the Studio prompt optimizer — e.g. a GTA 6 visual style. Users pick one in the composer before generating (optional).')}
              </p>
            </div>
            <div className='flex items-center gap-2'>
              <Button
                onClick={() => void navigate({ to: '/system-settings/operations/$section', params: { section: 'playground' } })}
                size='sm'
                variant='ghost'
              >
                <ArrowLeft className='me-1.5 size-3.5' />
                {t('Back to System Settings')}
              </Button>
              <Button onClick={() => openEditor(null)} size='sm'>
                <PlusIcon className='me-1.5 size-4' />
                {t('New skill')}
              </Button>
            </div>
          </div>

          {skillsQuery.isLoading && (
            <div className='flex h-32 items-center justify-center text-muted-foreground'>
              <Loader2 className='me-2 size-4 animate-spin' />
              {t('Loading')}
            </div>
          )}
          {skillsQuery.isError && (
            <p className='rounded-lg border border-destructive/40 bg-destructive/5 p-4 text-sm'>
              {t('Skills could not be loaded')}
            </p>
          )}
          {!skillsQuery.isLoading && !skillsQuery.isError && skills.length === 0 && (
            <div className='flex flex-col items-center gap-3 rounded-xl border border-dashed border-border/70 py-14 text-center'>
              <Wand2 className='size-8 text-muted-foreground/60' />
              <p className='text-sm text-muted-foreground'>{t('No skills yet.')}</p>
              <Button onClick={() => openEditor(null)} size='sm' variant='outline'>
                <PlusIcon className='me-1.5 size-4' />
                {t('Create the first skill')}
              </Button>
            </div>
          )}

          <div className='grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3'>
            {skills.map((skill) => (
              <SkillCard
                deleting={deletingId === skill.id}
                key={skill.id}
                onDelete={() => {
                  if (!window.confirm(t('Delete the skill "{{title}}"?', { title: skill.title }))) {
                    return
                  }
                  setDeletingId(skill.id)
                  deleteMutation.mutate(skill.id)
                }}
                onEdit={() => openEditor(skill)}
                onToggle={(enabled) => toggleMutation.mutate({ id: skill.id, enabled })}
                skill={skill}
              />
            ))}
          </div>
        </div>
      </SectionPageLayout.Content>

      <Dialog open={editorOpen} onOpenChange={setEditorOpen}>
        <DialogContent className='max-w-2xl max-h-[92vh] overflow-y-auto'>
          <DialogHeader>
            <DialogTitle>{editing ? t('Edit skill') : t('New skill')}</DialogTitle>
            <DialogDescription>
              {t('The optimizer weaves the skill into the prompt for this generation.')}
            </DialogDescription>
          </DialogHeader>

          <div className='space-y-3'>
            <div className='flex items-center justify-between gap-2'>
              <Label className='text-xs'>{t('Skill image')}</Label>
              <div className='flex items-center gap-2'>
                <input
                  accept='image/png,image/jpeg,image/webp,image/svg+xml'
                  className='hidden'
                  id='skill-image-upload'
                  onChange={async (e) => {
                    const file = e.target.files?.[0]
                    e.target.value = ''
                    if (!file) return
                    setUploading(true)
                    try {
                      const url = await uploadSkillImage(file)
                      setImage(url)
                    } catch (error) {
                      toast.error((error as Error).message)
                    } finally {
                      setUploading(false)
                    }
                  }}
                  type='file'
                />
                <label
                  className='inline-flex cursor-pointer items-center gap-1 rounded-md border border-border/70 px-2 py-1 text-[11px] font-medium text-muted-foreground transition hover:bg-muted hover:text-foreground'
                  htmlFor='skill-image-upload'
                >
                  {uploading ? (
                    <Loader2 className='size-3.5 animate-spin' />
                  ) : (
                    <Upload className='size-3.5' />
                  )}
                  {t('Upload')}
                </label>
              </div>
            </div>
            <div className='flex items-center gap-2'>
              <Input
                className='h-8 font-mono text-[11px]'
                onChange={(e) => setImage(e.target.value)}
                placeholder='https://… or upload'
                value={image}
              />
            </div>
            {image && (
              <img
                alt={t('Skill image preview')}
                className='h-24 w-full rounded-md border border-border/60 object-cover'
                src={image}
              />
            )}

            <div className='grid gap-3 sm:grid-cols-3'>
              <div className='space-y-1 sm:col-span-2'>
                <Label className='text-xs' htmlFor='skill-title'>{t('Title')}</Label>
                <Input
                  className='h-8 text-xs'
                  id='skill-title'
                  onChange={(e) => setTitle(e.target.value)}
                  placeholder='GTA 6 — Open World Crime Saga'
                  value={title}
                />
              </div>
              <div className='space-y-1'>
                <Label className='text-xs' htmlFor='skill-sort-order'>{t('Sort order')}</Label>
                <Input
                  className='h-8 text-xs'
                  id='skill-sort-order'
                  min={0}
                  onChange={(e) => setSortOrder(Number(e.target.value))}
                  type='number'
                  value={sortOrder}
                />
              </div>
            </div>

            <div className='space-y-1'>
              <Label className='text-xs' htmlFor='skill-description'>{t('Description')}</Label>
              <Input
                className='h-8 text-xs'
                id='skill-description'
                onChange={(e) => setDescription(e.target.value)}
                placeholder={t('What the skill achieves…')}
                value={description}
              />
            </div>

            <div className='space-y-1'>
              <div className='flex items-center justify-between gap-2'>
                <Label className='text-xs' htmlFor='skill-content'>{t('Skill content')}</Label>
                <button
                  className='inline-flex items-center gap-1 text-[10px] font-medium text-primary hover:underline'
                  onClick={applyTemplate}
                  type='button'
                >
                  <Wand2 className='size-3' />
                  {t('Insert example skill')}
                </button>
              </div>
              <Textarea
                className='min-h-44 font-mono text-[11px] leading-snug'
                id='skill-content'
                onChange={(e) => setContent(e.target.value)}
                placeholder={t('Describe the visual language, camera, tone, lighting and constraints…')}
                value={content}
              />
            </div>

            <div className='flex items-center justify-between gap-2 rounded-md border border-border/60 px-3 py-2'>
              <div>
                <Label className='text-xs'>{t('Enabled')}</Label>
                <p className='text-[10px] text-muted-foreground'>
                  {t('Disabled skills stay editable but are hidden from the picker.')}
                </p>
              </div>
              <Switch checked={enabled} onCheckedChange={setEnabled} />
            </div>
          </div>

          <DialogFooter>
            <Button onClick={() => setEditorOpen(false)} variant='outline'>
              {t('Cancel')}
            </Button>
            <Button
              disabled={saving || !title.trim() || !content.trim()}
              onClick={() => saveMutation.mutate()}
            >
              {saving && <Loader2 className='me-1.5 size-3.5 animate-spin' />}
              <CheckCircle2 className='me-1.5 size-3.5' />
              {t('Save skill')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </SectionPageLayout>
  )
}
