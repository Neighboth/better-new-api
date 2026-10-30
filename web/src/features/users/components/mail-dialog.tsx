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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import {
  AlertTriangle,
  CheckCircle2,
  ChevronDown,
  Loader2,
  Mail,
  Search,
  Send,
  ShieldCheck,
  X,
} from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
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
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { useDebounce } from '@/hooks'
import { cn } from '@/lib/utils'

import {
  getMailAudience,
  getMailCampaign,
  getMailDeliverability,
  getMailRecipientsByIds,
  getMailStatus,
  searchMailRecipients,
  sendMail,
  type MailCampaign,
  type MailRecipientOption,
} from '../api'

const PAGE_SIZE = 50

type MailDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** The users that the table selected, the dialog starts with them. */
  initialUserIds?: number[]
}

/**
 * The dialog that writes a message to the users of the installation.
 *
 * The recipients are chosen on the server: the search asks the installation for
 * the next page of the users and the list of the users of the installation
 * never reaches the browser. An installation of a hundred thousand users opens
 * this page with the same payload as an installation of ten users.
 */
export function MailDialog({
  open,
  onOpenChange,
  initialUserIds = [],
}: MailDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()

  const [allUsers, setAllUsers] = useState(false)
  const [selected, setSelected] = useState<MailRecipientOption[]>([])
  const [keyword, setKeyword] = useState('')
  const [page, setPage] = useState(1)
  const [subject, setSubject] = useState('')
  const [content, setContent] = useState('')
  const [isHtml, setIsHtml] = useState(false)
  const [promotional, setPromotional] = useState(false)
  const [advancedOpen, setAdvancedOpen] = useState(false)
  const [sending, setSending] = useState(false)
  const [campaign, setCampaign] = useState<MailCampaign | null>(null)

  const debouncedKeyword = useDebounce(keyword, 400)
  const selectedIds = useMemo(() => selected.map((user) => user.id), [selected])
  const idsKey = selectedIds.join(',')
  const startedRef = useRef(false)

  const resetForm = useCallback(() => {
    setSubject('')
    setContent('')
    setIsHtml(false)
    setPromotional(false)
    setAdvancedOpen(false)
    setCampaign(null)
  }, [])

  // The dialog opens with the users of the table: the bulk action of the table
  // and the button of the header share the dialog. The names of the selected
  // users are read from the installation, the page never invents them.
  useEffect(() => {
    if (!open) {
      startedRef.current = false
      return
    }
    if (startedRef.current) return
    startedRef.current = true
    if (initialUserIds.length > 0) {
      setAllUsers(false)
      void getMailRecipientsByIds(initialUserIds).then((result) => {
        const items = result.data?.items ?? []
        if (items.length === 0) return
        setSelected(items)
      })
      return
    }
    // The button of the header opens an empty message, the selection of the
    // table does not follow the administrator into the next message.
    setSelected([])
    if (
      campaign &&
      (campaign.status === 'done' || campaign.status === 'failed')
    ) {
      resetForm()
    }
  }, [open, initialUserIds, campaign, resetForm])

  const searchQuery = useQuery({
    queryKey: ['mail-recipients', debouncedKeyword, page],
    enabled: open && !allUsers,
    queryFn: () => searchMailRecipients(debouncedKeyword, PAGE_SIZE),
    staleTime: 30 * 1000,
  })

  const audienceQuery = useQuery({
    queryKey: ['mail-audience', allUsers ? 'all' : idsKey],
    enabled: open,
    queryFn: () => getMailAudience(allUsers ? [] : selectedIds),
    staleTime: 30 * 1000,
  })

  const statusQuery = useQuery({
    queryKey: ['mail-status'],
    enabled: open,
    queryFn: getMailStatus,
  })

  const deliverabilityQuery = useQuery({
    queryKey: ['mail-deliverability'],
    enabled: open && advancedOpen,
    queryFn: getMailDeliverability,
    staleTime: 5 * 60 * 1000,
  })

  // The progress of the campaign is polled until the campaign stops.
  useEffect(() => {
    if (!campaign || campaign.status === 'done' || campaign.status === 'failed')
      return
    const timer = setTimeout(async () => {
      const result = await getMailCampaign(campaign.id)
      if (result.success && result.data) setCampaign(result.data)
    }, 1500)
    return () => clearTimeout(timer)
  }, [campaign])

  const audience = audienceQuery.data?.data
  const recipientCount = allUsers
    ? promotional
      ? (audience?.marketing ?? 0)
      : (audience?.addresses ?? 0)
    : promotional
      ? (audience?.selected_marketing ?? 0)
      : (audience?.selected ?? 0)

  const toggleRecipient = useCallback((user: MailRecipientOption) => {
    setSelected((current) => {
      const exists = current.some((item) => item.id === user.id)
      if (exists) return current.filter((item) => item.id !== user.id)
      return [...current, user]
    })
  }, [])

  const send = useCallback(
    async (test: boolean) => {
      if (!subject.trim()) {
        toast.error(t('The subject of the message is empty.'))
        return
      }
      if (!content.trim()) {
        toast.error(t('The message is empty.'))
        return
      }
      if (!test && !allUsers && selected.length === 0) {
        toast.error(t('Select at least one user.'))
        return
      }
      setSending(true)
      try {
        const result = await sendMail({
          subject: subject.trim(),
          content,
          is_html: isHtml,
          promotional,
          all_users: allUsers,
          user_ids: allUsers ? [] : selectedIds,
          test,
        })
        if (!result.success) {
          toast.error(result.message || t('The message could not be sent.'))
          return
        }
        if (test) {
          toast.success(t('The test message left the server.'))
          return
        }
        toast.success(
          t('The message is being sent to {{count}} users.', {
            count: result.data?.total ?? recipientCount,
          })
        )
        if (result.data?.campaign_id) {
          const created = await getMailCampaign(result.data.campaign_id)
          if (created.success && created.data) setCampaign(created.data)
        }
        void queryClient.invalidateQueries({ queryKey: ['mail-audience'] })
      } catch (_error) {
        toast.error(t('The message could not be sent.'))
      } finally {
        setSending(false)
      }
    },
    [
      allUsers,
      content,
      isHtml,
      promotional,
      queryClient,
      recipientCount,
      selected.length,
      selectedIds,
      subject,
      t,
    ]
  )

  const results = searchQuery.data?.data?.items ?? []
  const mailStatus = statusQuery.data?.data
  const deliverability = deliverabilityQuery.data?.data
  const finished = campaign?.status === 'done' || campaign?.status === 'failed'

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2'>
            <Mail className='size-4' />
            {t('Send mail')}
          </DialogTitle>
          <DialogDescription>
            {t('Write a message to the users of the site.')}
          </DialogDescription>
        </DialogHeader>

        {campaign ? (
          <div className='space-y-3'>
            <div className='flex items-center gap-2 text-sm'>
              {finished ? (
                campaign.status === 'done' ? (
                  <CheckCircle2 className='size-4 text-emerald-500' />
                ) : (
                  <AlertTriangle className='size-4 text-amber-500' />
                )
              ) : (
                <Loader2 className='size-4 animate-spin' />
              )}
              <span className='font-medium'>
                {campaign.status === 'queued'
                  ? t('The message is queued.')
                  : campaign.status === 'running'
                    ? t('The message is being sent.')
                    : campaign.status === 'done'
                      ? t('The message left the server.')
                      : t('The message stopped with an error.')}
              </span>
            </div>
            <div className='bg-muted h-2 overflow-hidden rounded-full'>
              <div
                className='bg-primary h-full transition-all'
                style={{
                  width: `${
                    campaign.total > 0
                      ? Math.min(
                          100,
                          Math.round(
                            ((campaign.sent + campaign.failed) /
                              campaign.total) *
                              100
                          )
                        )
                      : 0
                  }%`,
                }}
              />
            </div>
            <div className='text-muted-foreground grid grid-cols-2 gap-2 text-xs sm:grid-cols-4'>
              <div>
                {t('Recipients')}: <b>{campaign.total}</b>
              </div>
              <div>
                {t('Sent')}: <b className='text-emerald-600'>{campaign.sent}</b>
              </div>
              <div>
                {t('Failed')}: <b className='text-red-600'>{campaign.failed}</b>
              </div>
              <div>
                {t('Skipped')}: <b>{campaign.skipped}</b>
              </div>
            </div>
            {campaign.last_error ? (
              <p className='text-destructive text-xs break-words'>
                {campaign.last_error}
              </p>
            ) : null}
            <DialogFooter>
              <Button
                variant='outline'
                onClick={() => {
                  setCampaign(null)
                  resetForm()
                }}
              >
                {t('Write another message')}
              </Button>
              <Button onClick={() => onOpenChange(false)}>{t('Close')}</Button>
            </DialogFooter>
          </div>
        ) : (
          <div className='space-y-4'>
            {mailStatus && !mailStatus.configured ? (
              <div className='rounded-lg border border-amber-500/40 bg-amber-500/10 p-3 text-xs text-amber-700 dark:text-amber-400'>
                {t(
                  'The mail server is not configured. Set the SMTP settings before a message is sent.'
                )}
              </div>
            ) : null}

            {/* Recipients */}
            <div className='space-y-2'>
              <div className='flex items-center justify-between'>
                <Label>{t('Recipients')}</Label>
                <div className='text-muted-foreground text-xs'>
                  {t('{{count}} users will receive the message.', {
                    count: recipientCount,
                  })}
                </div>
              </div>

              <label className='flex items-center gap-2 text-sm'>
                <Checkbox
                  checked={allUsers}
                  onCheckedChange={(value) => setAllUsers(value === true)}
                />
                {t('All users')}
                {promotional && audience ? (
                  <Badge variant='secondary' className='ml-1 text-[10px]'>
                    {t('{{count}} of {{total}} accept the announcements', {
                      count: audience.marketing,
                      total: audience.addresses,
                    })}
                  </Badge>
                ) : null}
              </label>

              {/* The reach of the message is never a bare number: the panel
                  names the users that the message cannot reach, so an
                  administrator never reads a smaller number as a lost contact. */}
              {audience ? (
                <div className='text-muted-foreground bg-muted/40 space-y-1 rounded-lg border px-3 py-2 text-[11px]'>
                  <div>
                    {t('{{total}} users in the installation.', {
                      total: audience.total,
                    })}
                  </div>
                  <div>
                    {t('{{count}} have an address and an enabled account.', {
                      count: audience.addresses,
                    })}
                  </div>
                  {audience.disabled > 0 ? (
                    <div>
                      {t('{{count}} disabled or banned accounts are skipped.', {
                        count: audience.disabled,
                      })}
                    </div>
                  ) : null}
                  {audience.no_email > 0 ? (
                    <div>
                      {t('{{count}} accounts have no address.', {
                        count: audience.no_email,
                      })}
                    </div>
                  ) : null}
                  {audience.opted_out > 0 ? (
                    <div>
                      {t(
                        '{{count}} turned the announcements off in their profile.',
                        { count: audience.opted_out }
                      )}
                    </div>
                  ) : null}
                </div>
              ) : null}

              {!allUsers && (
                <div className='rounded-lg border'>
                  <div className='flex items-center gap-2 border-b p-2'>
                    <Search className='text-muted-foreground size-3.5' />
                    <Input
                      value={keyword}
                      onChange={(event) => {
                        setKeyword(event.target.value)
                        setPage(1)
                      }}
                      placeholder={t('Search by name, username or email…')}
                      className='h-8 border-0 shadow-none focus-visible:ring-0'
                    />
                  </div>
                  <div className='max-h-44 overflow-y-auto'>
                    {searchQuery.isLoading ? (
                      <div className='text-muted-foreground p-3 text-center text-xs'>
                        {t('Loading…')}
                      </div>
                    ) : results.length === 0 ? (
                      <div className='text-muted-foreground p-3 text-center text-xs'>
                        {t('No user matches the search.')}
                      </div>
                    ) : (
                      results.map((user) => {
                        const checked = selected.some(
                          (item) => item.id === user.id
                        )
                        return (
                          <button
                            key={user.id}
                            type='button'
                            onClick={() => toggleRecipient(user)}
                            className={cn(
                              'hover:bg-muted/50 flex w-full items-center gap-2 px-2 py-1.5 text-left text-xs',
                              checked && 'bg-primary/5'
                            )}
                          >
                            <Checkbox checked={checked} className='size-3.5' />
                            <span className='truncate font-medium'>
                              {user.display_name || user.username}
                            </span>
                            <span className='text-muted-foreground truncate'>
                              {user.email}
                            </span>
                            {!user.marketing && promotional ? (
                              <Badge
                                variant='outline'
                                className='ml-auto text-[10px]'
                              >
                                {t('No announcements')}
                              </Badge>
                            ) : null}
                          </button>
                        )
                      })
                    )}
                  </div>
                  <div className='text-muted-foreground flex items-center justify-between border-t px-2 py-1 text-[11px]'>
                    <span>
                      {t('{{count}} selected', { count: selected.length })}
                    </span>
                    <div className='flex items-center gap-1'>
                      <Button
                        type='button'
                        variant='ghost'
                        size='sm'
                        className='h-6 px-2 text-[11px]'
                        disabled={page <= 1}
                        onClick={() =>
                          setPage((current) => Math.max(1, current - 1))
                        }
                      >
                        {t('Previous')}
                      </Button>
                      <span>{page}</span>
                      <Button
                        type='button'
                        variant='ghost'
                        size='sm'
                        className='h-6 px-2 text-[11px]'
                        disabled={
                          (searchQuery.data?.data?.count ?? 0) < PAGE_SIZE
                        }
                        onClick={() => setPage((current) => current + 1)}
                      >
                        {t('Next')}
                      </Button>
                    </div>
                  </div>
                </div>
              )}

              {selected.length > 0 && !allUsers ? (
                <div className='flex flex-wrap gap-1'>
                  {selected.slice(0, 12).map((user) => (
                    <Badge
                      key={user.id}
                      variant='secondary'
                      className='gap-1 text-[10px]'
                    >
                      {user.display_name || user.username}
                      <button
                        type='button'
                        onClick={() => toggleRecipient(user)}
                        aria-label={t('Remove')}
                      >
                        <X className='size-3' />
                      </button>
                    </Badge>
                  ))}
                  {selected.length > 12 ? (
                    <Badge variant='outline' className='text-[10px]'>
                      +{selected.length - 12}
                    </Badge>
                  ) : null}
                </div>
              ) : null}
            </div>

            {/* Subject and body */}
            <div className='space-y-1.5'>
              <Label htmlFor='mail-subject'>{t('Subject')}</Label>
              <Input
                id='mail-subject'
                value={subject}
                maxLength={255}
                onChange={(event) => setSubject(event.target.value)}
                placeholder={t('A subject that tells what the message is')}
              />
            </div>

            <div className='space-y-1.5'>
              <div className='flex items-center justify-between'>
                <Label htmlFor='mail-content'>{t('Message')}</Label>
                <label className='flex items-center gap-2 text-xs'>
                  <Switch
                    checked={isHtml}
                    onCheckedChange={(value: boolean) => setIsHtml(value)}
                  />
                  {t('This is HTML')}
                </label>
              </div>
              <Textarea
                id='mail-content'
                value={content}
                onChange={(event) => setContent(event.target.value)}
                rows={8}
                className='font-mono text-xs'
                placeholder={
                  isHtml
                    ? '<p>Hello <b>world</b></p>'
                    : t('Write the text of the message…')
                }
              />
              <p className='text-muted-foreground text-[11px]'>
                {isHtml
                  ? t(
                      'The message is sent as HTML. Escape the text that the readers write.'
                    )
                  : t(
                      'The message is sent as text. The installation wraps it in the frame of the site.'
                    )}
              </p>
            </div>

            {/* Advanced options */}
            <div className='rounded-lg border'>
              <button
                type='button'
                className='flex w-full items-center justify-between px-3 py-2 text-sm font-medium'
                onClick={() => setAdvancedOpen((value) => !value)}
              >
                <span className='flex items-center gap-2'>
                  <ShieldCheck className='size-4' />
                  {t('Advanced options')}
                </span>
                <ChevronDown
                  className={cn(
                    'size-4 transition-transform',
                    advancedOpen && 'rotate-180'
                  )}
                />
              </button>
              {advancedOpen && (
                <div className='space-y-3 border-t px-3 py-3'>
                  <label className='flex items-start justify-between gap-3'>
                    <span className='space-y-0.5'>
                      <span className='block text-sm font-medium'>
                        {t('Is this a promotional message?')}
                      </span>
                      <span className='text-muted-foreground block text-xs'>
                        {t(
                          'The message carries the unsubscribe headers (RFC 8058) and it is sent only to the users that accept the announcements of the site.'
                        )}
                      </span>
                    </span>
                    <Switch
                      checked={promotional}
                      onCheckedChange={(value: boolean) =>
                        setPromotional(value)
                      }
                    />
                  </label>

                  {promotional ? (
                    <div className='rounded-md bg-emerald-500/10 p-2 text-xs text-emerald-700 dark:text-emerald-400'>
                      {t(
                        '{{count}} users accept the announcements and will receive this message.',
                        { count: recipientCount }
                      )}
                    </div>
                  ) : null}

                  {deliverability ? (
                    <div className='space-y-2 rounded-md border p-2'>
                      <div className='flex items-center justify-between text-xs font-medium'>
                        <span>
                          {t('Mail health of {{domain}}', {
                            domain: deliverability.domain,
                          })}
                        </span>
                        <Badge
                          variant={
                            deliverability.readiness >= 75
                              ? 'default'
                              : 'secondary'
                          }
                        >
                          {deliverability.readiness}%
                        </Badge>
                      </div>
                      <div className='text-muted-foreground grid grid-cols-2 gap-1 text-[11px]'>
                        <span>
                          SPF{' '}
                          <b
                            className={
                              deliverability.spf_ok
                                ? 'text-emerald-600'
                                : 'text-red-600'
                            }
                          >
                            {deliverability.spf_ok ? t('ok') : t('missing')}
                          </b>
                        </span>
                        <span>
                          DKIM{' '}
                          <b
                            className={
                              deliverability.dkim_ok
                                ? 'text-emerald-600'
                                : 'text-amber-600'
                            }
                          >
                            {deliverability.dkim_ok
                              ? deliverability.dkim_selector
                                ? `${t('ok')} (${deliverability.dkim_selector})`
                                : t('ok')
                              : t('missing')}
                          </b>
                        </span>
                        <span>
                          DMARC{' '}
                          <b
                            className={
                              deliverability.dmarc_ok
                                ? 'text-emerald-600'
                                : 'text-red-600'
                            }
                          >
                            {deliverability.dmarc_ok ? t('ok') : t('missing')}
                          </b>
                        </span>
                        <span>
                          MX{' '}
                          <b
                            className={
                              deliverability.mx_ok
                                ? 'text-emerald-600'
                                : 'text-amber-600'
                            }
                          >
                            {deliverability.mx_ok ? t('ok') : t('missing')}
                          </b>
                        </span>
                      </div>
                      <ul className='text-muted-foreground space-y-1 text-[11px]'>
                        {deliverability.hints.map((hint) => (
                          <li key={hint.message} className='flex gap-1.5'>
                            <span
                              className={cn(
                                'mt-1 size-1.5 shrink-0 rounded-full',
                                hint.level === 'ok'
                                  ? 'bg-emerald-500'
                                  : hint.level === 'warn'
                                    ? 'bg-amber-500'
                                    : 'bg-red-500'
                              )}
                            />
                            {hint.message}
                          </li>
                        ))}
                      </ul>
                    </div>
                  ) : null}
                </div>
              )}
            </div>

            <DialogFooter className='gap-2'>
              <Button
                type='button'
                variant='outline'
                disabled={sending}
                onClick={() => void send(true)}
              >
                {t('Send a test to me')}
              </Button>
              <Button
                type='button'
                disabled={sending || recipientCount === 0}
                onClick={() => void send(false)}
              >
                {sending ? (
                  <Loader2 className='size-4 animate-spin' />
                ) : (
                  <Send className='size-4' />
                )}
                {t('Send')}
              </Button>
            </DialogFooter>
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}
