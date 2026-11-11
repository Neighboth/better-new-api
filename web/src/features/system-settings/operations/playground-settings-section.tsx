import { zodResolver } from '@hookform/resolvers/zod'
import { useForm, type Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'

import {
  SettingsForm,
  SettingsFormGrid,
  SettingsSwitchContent,
  SettingsSwitchRow,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const schema = z.object({
  imageEnabled: z.boolean().optional(),
  imageModel: z.string().optional(),
  imageSizes: z.string().optional(),
  imageDefaultSize: z.string().optional(),
  imageQualities: z.string().optional(),
  imageDefaultQuality: z.string().optional(),
  imageDefaultCount: z.coerce.number().int().min(1).max(4).optional(),

  videoEnabled: z.boolean().optional(),
  videoModel: z.string().optional(),
  videoDurations: z.string().optional(),
  videoDefaultDuration: z.coerce.number().int().min(1).max(120).optional(),
  videoMinDuration: z.coerce.number().int().min(1).max(120).optional(),
  videoMaxDuration: z.coerce.number().int().min(1).max(600).optional(),
  videoRatios: z.string().optional(),
  videoDefaultRatio: z.string().optional(),
  videoQualities: z.string().optional(),
  videoDefaultQuality: z.string().optional(),
  videoReferenceImage: z.boolean().optional(),

  studioEnabled: z.boolean().optional(),
})

type Values = z.infer<typeof schema>

export type PlaygroundSettingsDefaults = {
  imageEnabled?: boolean
  imageModel?: string
  imageSizes?: string
  imageDefaultSize?: string
  imageQualities?: string
  imageDefaultQuality?: string
  imageDefaultCount?: number
  videoEnabled?: boolean
  videoModel?: string
  videoDurations?: string
  videoDefaultDuration?: number
  videoMinDuration?: number
  videoMaxDuration?: number
  videoRatios?: string
  videoDefaultRatio?: string
  videoQualities?: string
  videoDefaultQuality?: string
  videoReferenceImage?: boolean
  studioEnabled?: boolean
}

export function PlaygroundSettingsSection({
  defaultValues,
}: {
  defaultValues: PlaygroundSettingsDefaults
}) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const form = useForm<Values>({
    resolver: zodResolver(schema) as unknown as Resolver<Values>,
    defaultValues: {
      imageEnabled: defaultValues.imageEnabled ?? true,
      imageModel: defaultValues.imageModel ?? '',
      imageSizes: defaultValues.imageSizes ?? '1024x1024,1024x1792,1792x1024',
      imageDefaultSize: defaultValues.imageDefaultSize ?? '1024x1024',
      imageQualities: defaultValues.imageQualities ?? 'standard,hd',
      imageDefaultQuality: defaultValues.imageDefaultQuality ?? 'standard',
      imageDefaultCount: defaultValues.imageDefaultCount ?? 1,
      videoEnabled: defaultValues.videoEnabled ?? false,
      videoModel: defaultValues.videoModel ?? '',
      videoDurations: defaultValues.videoDurations ?? '5,10',
      videoDefaultDuration: defaultValues.videoDefaultDuration ?? 5,
      videoMinDuration: defaultValues.videoMinDuration ?? 5,
      videoMaxDuration: defaultValues.videoMaxDuration ?? 10,
      videoRatios: defaultValues.videoRatios ?? '16:9,9:16,1:1',
      videoDefaultRatio: defaultValues.videoDefaultRatio ?? '16:9',
      videoQualities: defaultValues.videoQualities ?? 'standard,high',
      videoDefaultQuality: defaultValues.videoDefaultQuality ?? 'standard',
      videoReferenceImage: defaultValues.videoReferenceImage ?? true,
      studioEnabled: defaultValues.studioEnabled ?? true,
    },
  })

  const onSubmit = async (values: Values) => {
    const entries: Array<[string, string]> = [
      ['playground_setting.image_enabled', String(Boolean(values.imageEnabled))],
      ['playground_setting.image_model', String(values.imageModel ?? '')],
      ['playground_setting.image_sizes', String(values.imageSizes ?? '')],
      [
        'playground_setting.image_default_size',
        String(values.imageDefaultSize ?? ''),
      ],
      ['playground_setting.image_qualities', String(values.imageQualities ?? '')],
      [
        'playground_setting.image_default_quality',
        String(values.imageDefaultQuality ?? ''),
      ],
      [
        'playground_setting.image_default_count',
        String(values.imageDefaultCount ?? 1),
      ],
      ['playground_setting.video_enabled', String(Boolean(values.videoEnabled))],
      ['playground_setting.video_model', String(values.videoModel ?? '')],
      ['playground_setting.video_durations', String(values.videoDurations ?? '')],
      [
        'playground_setting.video_default_duration',
        String(values.videoDefaultDuration ?? 5),
      ],
      [
        'playground_setting.video_min_duration',
        String(values.videoMinDuration ?? 1),
      ],
      [
        'playground_setting.video_max_duration',
        String(values.videoMaxDuration ?? 60),
      ],
      ['playground_setting.video_ratios', String(values.videoRatios ?? '')],
      [
        'playground_setting.video_default_ratio',
        String(values.videoDefaultRatio ?? ''),
      ],
      ['playground_setting.video_qualities', String(values.videoQualities ?? '')],
      [
        'playground_setting.video_default_quality',
        String(values.videoDefaultQuality ?? ''),
      ],
      [
        'playground_setting.video_reference_image',
        String(Boolean(values.videoReferenceImage)),
      ],
      ['playground_setting.studio_enabled', String(Boolean(values.studioEnabled))],
    ]

    try {
      for (const [key, value] of entries) {
        await updateOption.mutateAsync({ key, value })
      }
      form.reset(values)
      toast.success(t('Playground settings saved successfully'))
    } catch {
      toast.error(t('Failed to save settings'))
    }
  }

  const imageEnabled = form.watch('imageEnabled')
  const videoEnabled = form.watch('videoEnabled')

  return (
    <SettingsSection title={t('Playground & Studio')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <div className='space-y-3'>
            <SettingsSwitchRow className='rounded-lg border px-3'>
              <SettingsSwitchContent>
                <Label className='text-sm font-medium'>
                  {t('Image generation')}
                </Label>
                <p className='text-muted-foreground text-xs'>
                  {t(
                    'Offers the image tool in the playground and the image tab of the studio.'
                  )}
                </p>
              </SettingsSwitchContent>
              <Switch
                checked={Boolean(imageEnabled)}
                onCheckedChange={(checked) =>
                  form.setValue('imageEnabled', checked, { shouldDirty: true })
                }
              />
            </SettingsSwitchRow>

            <SettingsFormGrid>
              <FormField
                control={form.control}
                name='imageModel'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Image model')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder='gpt-image-1'
                        {...field}
                        value={field.value ?? ''}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Model id used by the image tool and the studio. It must exist in your model catalog.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='imageDefaultSize'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Default image size')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder='1024x1024'
                        {...field}
                        value={field.value ?? ''}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Must be one of the sizes below. Used when the visitor does not pick one.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='imageSizes'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Image sizes')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder='1024x1024,1024x1792,1792x1024'
                        {...field}
                        value={field.value ?? ''}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Comma separated list of offered sizes, e.g. 1024x1024,1536x1024.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='imageQualities'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Image qualities')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder='standard,hd'
                        {...field}
                        value={field.value ?? ''}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Comma separated quality options offered by the studio.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='imageDefaultQuality'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Default image quality')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder='standard'
                        {...field}
                        value={field.value ?? ''}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('One of the quality options above.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='imageDefaultCount'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Images per request')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        max={4}
                        {...field}
                        value={field.value ?? 1}
                        onChange={(e) => field.onChange(Number(e.target.value))}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Default number of images generated at once (1-4).')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SettingsFormGrid>
          </div>

          <div className='space-y-3 pt-2'>
            <SettingsSwitchRow className='rounded-lg border px-3'>
              <SettingsSwitchContent>
                <Label className='text-sm font-medium'>
                  {t('Video generation')}
                </Label>
                <p className='text-muted-foreground text-xs'>
                  {t(
                    'Offers the video tool in the playground and the video tab of the studio. Video models are billed per task.'
                  )}
                </p>
              </SettingsSwitchContent>
              <Switch
                checked={Boolean(videoEnabled)}
                onCheckedChange={(checked) =>
                  form.setValue('videoEnabled', checked, { shouldDirty: true })
                }
              />
            </SettingsSwitchRow>

            <SettingsFormGrid>
              <FormField
                control={form.control}
                name='videoModel'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Video model')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder='sora-2'
                        {...field}
                        value={field.value ?? ''}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Model id of the video channel. Leave empty to disable the video tool even when the switch is on.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='videoDurations'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Video durations (seconds)')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder='5,10'
                        {...field}
                        value={field.value ?? ''}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Comma separated duration presets offered by the studio.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='videoMinDuration'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Minimum duration (seconds)')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        {...field}
                        value={field.value ?? 1}
                        onChange={(e) => field.onChange(Number(e.target.value))}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Shortest clip the tools may request.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='videoMaxDuration'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Maximum duration (seconds)')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        {...field}
                        value={field.value ?? 60}
                        onChange={(e) => field.onChange(Number(e.target.value))}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Longest clip the tools may request.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='videoDefaultDuration'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Default duration (seconds)')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        {...field}
                        value={field.value ?? 5}
                        onChange={(e) => field.onChange(Number(e.target.value))}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Preselected clip length of the studio.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='videoRatios'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Aspect ratios')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder='16:9,9:16,1:1'
                        {...field}
                        value={field.value ?? ''}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Comma separated ratios offered by the studio.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='videoDefaultRatio'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Default aspect ratio')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder='16:9'
                        {...field}
                        value={field.value ?? ''}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('One of the ratios above.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='videoQualities'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Video qualities')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder='standard,high'
                        {...field}
                        value={field.value ?? ''}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Comma separated quality options offered by the studio.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='videoDefaultQuality'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Default video quality')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder='standard'
                        {...field}
                        value={field.value ?? ''}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('One of the video qualities above.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SettingsFormGrid>

            <SettingsSwitchRow className='rounded-lg border px-3'>
              <SettingsSwitchContent>
                <Label className='text-sm font-medium'>
                  {t('Reference image (image to video)')}
                </Label>
                <p className='text-muted-foreground text-xs'>
                  {t(
                    'Lets visitors upload a reference image that the video model animates.'
                  )}
                </p>
              </SettingsSwitchContent>
              <Switch
                checked={Boolean(form.watch('videoReferenceImage'))}
                onCheckedChange={(checked) =>
                  form.setValue('videoReferenceImage', checked, {
                    shouldDirty: true,
                  })
                }
              />
            </SettingsSwitchRow>
          </div>

          <div className='space-y-3 pt-2'>
            <SettingsSwitchRow className='rounded-lg border px-3'>
              <SettingsSwitchContent>
                <Label className='text-sm font-medium'>{t('Studio page')}</Label>
                <p className='text-muted-foreground text-xs'>
                  {t(
                    'Shows the studio entry in the sidebar. The visibility can additionally be changed in the sidebar modules.'
                  )}
                </p>
              </SettingsSwitchContent>
              <Switch
                checked={Boolean(form.watch('studioEnabled'))}
                onCheckedChange={(checked) =>
                  form.setValue('studioEnabled', checked, { shouldDirty: true })
                }
              />
            </SettingsSwitchRow>
          </div>

          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            isSaveDisabled={!form.formState.isDirty}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
