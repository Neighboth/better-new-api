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
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import {
  Download,
  ExternalLink,
  Headphones,
  Loader2,
  MessageCircle,
  PlusCircle,
  Send,
  X,
} from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Markdown } from '@/components/ui/markdown'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardFooter, CardHeader } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { useAuthStore } from '@/stores/auth-store'

import {
  addTicketMessage,
  addGuestTicketMessage,
  createGuestTicket,
  createTicket,
  downloadGuestTicketTranscript,
  getGuestTicket,
  getTicketDetail,
  getTicketConfig,
  getUserTickets,
  connectHumanSupport,
} from '../api'

const DISMISS_KEY = 'live_support_dismissed_until'
const GUEST_SESSION_KEY = 'guest_ticket_session'

const MessageContent = ({ content, isAdmin }: { content: string, isAdmin?: boolean }) => {
  const buttonRegex = /BUTTON\[(.*?)\]\((.*?)\)/g
  const match = buttonRegex.exec(content)
  let cleanContent = content
  let btnText = ''
  let btnUrl = ''
  if (match) {
    cleanContent = content.replace(buttonRegex, '')
    btnText = match[1]
    btnUrl = match[2]
  }
  return (
    <div className='flex flex-col gap-2 w-full'>
      <div className={`prose ${!isAdmin ? 'prose-invert prose-p:text-primary-foreground prose-a:text-primary-foreground' : 'prose-sm dark:prose-invert'} whitespace-pre-wrap break-words max-w-none`}>
        <Markdown content={cleanContent.trim()} />
      </div>
      {btnUrl && (
        <Button 
          className='w-full mt-2 bg-blue-600 hover:bg-blue-700 text-white' 
          onClick={() => window.location.href = btnUrl}
        >
          {btnText}
        </Button>
      )}
    </div>
  )
}

export function LiveSupportWidget() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { auth } = useAuthStore()
  const [open, setOpen] = useState(false)
  const [quickMessage, setQuickMessage] = useState('')
  const [sending, setSending] = useState(false)

  // 24h dismiss state
  const [isDismissed, setIsDismissed] = useState(() => {
    try {
      const until = localStorage.getItem(DISMISS_KEY)
      return until ? Date.now() < parseInt(until, 10) : false
    } catch {
      return false
    }
  })

  // Guest support state
  const [guestSession, setGuestSession] = useState<string>(() => {
    try {
      return localStorage.getItem(GUEST_SESSION_KEY) || ''
    } catch {
      return ''
    }
  })
  const [guestContact, setGuestContact] = useState('')
  const [guestMessage, setGuestMessage] = useState('')
  const [guestReply, setGuestReply] = useState('')

  // Listen to open-live-support event
  useEffect(() => {
    const handleOpen = () => {
      setIsDismissed(false)
      setOpen(true)
    }
    window.addEventListener('open-live-support', handleOpen)
    return () => window.removeEventListener('open-live-support', handleOpen)
  }, [])

  const { data: configResp } = useQuery({
    queryKey: ['ticket-config'],
    queryFn: getTicketConfig,
    staleTime: 60 * 1000,
  })

  const isEnabled = Boolean(
    configResp?.data?.enabled && configResp?.data?.live_support_enabled
  )

  const { data: userTicketsResp, refetch: refetchTickets } = useQuery({
    queryKey: ['user-live-support-tickets'],
    queryFn: () => getUserTickets({ page: 1, pageSize: 20, liveSupport: true }),
    enabled: Boolean(isEnabled && auth.user && open),
    refetchInterval: open && Boolean(auth.user) ? 5000 : false,
  })
  const activeLiveTicket = userTicketsResp?.data?.items?.find((ticket) => ticket.status !== 'closed')
  const { data: liveTicketDetail, isLoading: loadingLiveTicket } = useQuery({
    queryKey: ['live-support-detail', activeLiveTicket?.id],
    queryFn: () => getTicketDetail(activeLiveTicket!.id),
    enabled: Boolean(isEnabled && auth.user && open && activeLiveTicket),
    refetchInterval: open && Boolean(activeLiveTicket) ? 3000 : false,
  })

  // Guest ticket query
  const {
    data: guestTicketResp,
    refetch: refetchGuestTicket,
    isLoading: loadingGuestTicket,
  } = useQuery({
    queryKey: ['guest-ticket', guestSession],
    queryFn: () => getGuestTicket(guestSession),
    enabled: Boolean(isEnabled && !auth.user && open && guestSession),
    refetchInterval: open && Boolean(guestSession) ? 5000 : false,
  })

  const [connectingHuman, setConnectingHuman] = useState(false)

  if (!isEnabled) {
    return null
  }

  const handleDismiss = (e: React.MouseEvent) => {
    e.stopPropagation()
    const until = Date.now() + 24 * 60 * 60 * 1000
    try {
      localStorage.setItem(DISMISS_KEY, until.toString())
    } catch {}
    setIsDismissed(true)
    setOpen(false)
    toast.info(t('Live support widget dismissed for 24 hours'))
  }

  const handleConnectHuman = async (ticketId: number) => {
    try {
      setConnectingHuman(true)
      const res = await connectHumanSupport(ticketId)
      if (res.success) {
        toast.success(t('Connected to human support'))
        await Promise.all([refetchTickets(), refetchGuestTicket()])
      } else {
        toast.error(res.message || t('Failed to connect to human support'))
      }
    } catch {
      toast.error(t('Failed to connect to human support'))
    } finally {
      setConnectingHuman(false)
    }
  }

  const handleSendQuickMessage = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!quickMessage.trim()) return

    if (!auth.user) return

    try {
      setSending(true)
      const res = activeLiveTicket
        ? await addTicketMessage(activeLiveTicket.id, quickMessage.trim())
        : await createTicket({
            title: 'Live Support',
            category: 'technical',
            priority: 'normal',
            content: quickMessage.trim(),
            live_support: true,
          })

      if (res.success) {
        toast.success(t('Message sent to support! We will reply shortly.'))
        setQuickMessage('')
        await refetchTickets()
      } else {
        toast.error(res.message || t('Failed to send message'))
      }
    } catch {
      toast.error(t('Failed to send message'))
    } finally {
      setSending(false)
    }
  }

  const handleGuestSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!guestContact.trim() || !guestMessage.trim()) {
      toast.error(t('Please enter your contact information and message'))
      return
    }

    try {
      setSending(true)
      const res = await createGuestTicket({
        contact: guestContact.trim(),
        content: guestMessage.trim(),
      })
      if (res.success && res.data) {
        const sessionKey = res.data.guest_session_key || ''
        setGuestSession(sessionKey)
        try {
          localStorage.setItem(GUEST_SESSION_KEY, sessionKey)
        } catch {}
        setGuestMessage('')
        toast.success(t('Support request created successfully!'))
      } else {
        toast.error(res.message || t('Failed to create support request'))
      }
    } catch {
      toast.error(t('Failed to create support request'))
    } finally {
      setSending(false)
    }
  }

  const handleGuestReply = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!guestReply.trim() || !guestSession) return

    try {
      setSending(true)
      const res = await addGuestTicketMessage(guestSession, guestReply.trim())
      if (res.success) {
        setGuestReply('')
        void refetchGuestTicket()
      } else {
        toast.error(res.message || t('Failed to send reply'))
      }
    } catch {
      toast.error(t('Failed to send reply'))
    } finally {
      setSending(false)
    }
  }

  const handleNewGuestChat = () => {
    try {
      localStorage.removeItem(GUEST_SESSION_KEY)
    } catch {}
    setGuestSession('')
    setGuestContact('')
    setGuestMessage('')
  }

  const handleDownloadTranscript = async () => {
    if (!guestSession) return
    try {
      await downloadGuestTicketTranscript(guestSession)
    } catch {
      toast.error(t('Failed to download transcript'))
    }
  }

  if (isDismissed && !open) {
    return null
  }

  return (
    <div className='fixed bottom-5 right-5 z-50'>
      {open ? (
        <Card className='w-[350px] sm:w-[400px] shadow-2xl border-border animate-in fade-in zoom-in-95 duration-150 overflow-hidden flex flex-col'>
          {/* Widget Header */}
          <CardHeader className='bg-primary text-primary-foreground p-4 flex flex-row items-center justify-between space-y-0'>
            <div className='flex items-center gap-2.5'>
              <div className='relative flex h-8 w-8 items-center justify-center rounded-full bg-primary-foreground/20'>
                <Headphones className='h-4 w-4 text-primary-foreground' />
                <span className='absolute top-0 right-0 h-2.5 w-2.5 rounded-full bg-emerald-400 ring-2 ring-primary' />
              </div>
              <div>
                <h3 className='font-semibold text-sm leading-none'>{t('Live Support')}</h3>
                <span className='text-[11px] text-primary-foreground/80 mt-0.5 inline-block'>
                  {t('Online • How can we help?')}
                </span>
              </div>
            </div>

            <div className='flex items-center gap-1'>
              <Button
                variant='ghost'
                size='icon'
                className='h-7 w-7 text-primary-foreground hover:bg-primary-foreground/20 rounded-full'
                onClick={() => setOpen(false)}
              >
                <X className='h-4 w-4' />
              </Button>
            </div>
          </CardHeader>

          {/* Widget Body */}
          <CardContent className='p-4 space-y-4 max-h-[380px] overflow-y-auto text-sm'>
            {!auth.user ? (
              // Guest Mode View
              guestSession && guestTicketResp?.data ? (
                <div className='space-y-3'>
                  <div className='flex items-center justify-between border-b pb-2 text-xs'>
                    <div>
                      <span className='font-semibold text-foreground'>
                        #{guestTicketResp.data.ticket.id} - {guestTicketResp.data.ticket.title}
                      </span>
                      <p className='text-muted-foreground text-[10px] mt-0.5'>
                        {guestTicketResp.data.ticket.guest_contact}
                      </p>
                    </div>
                    <div className='flex items-center gap-1'>
                      <Button
                        variant='ghost'
                        size='icon'
                        className='h-7 w-7'
                        title={t('Download Transcript')}
                        onClick={handleDownloadTranscript}
                      >
                        <Download className='h-3.5 w-3.5' />
                      </Button>
                      <Button
                        variant='ghost'
                        size='icon'
                        className='h-7 w-7'
                        title={t('New Chat')}
                        onClick={handleNewGuestChat}
                      >
                        <PlusCircle className='h-3.5 w-3.5' />
                      </Button>
                    </div>
                  </div>

                  {/* Messages */}
                  <div className='space-y-2 max-h-[220px] overflow-y-auto pr-1'>
                    {guestTicketResp.data.messages.map((m) => (
                      <div
                        key={m.id}
                        className={`p-2.5 rounded-lg text-xs max-w-[85%] ${
                          m.is_admin
                            ? 'bg-muted border mr-auto'
                            : 'bg-primary text-primary-foreground ml-auto'
                        }`}
                      >
                        <div className='text-[10px] opacity-75 font-semibold mb-1'>
                          {m.is_admin ? t('Support Agent') : t('You')}
                        </div>
                        <div className='whitespace-pre-wrap break-words'><MessageContent content={m.content} isAdmin={m.is_admin} /></div>
                      </div>
                    ))}
                  </div>
                  {guestTicketResp?.data?.ticket?.is_claimed && !guestTicketResp?.data?.ticket?.human_connected && (
                    <div className='my-2 rounded-lg border border-amber-500/30 bg-amber-500/10 p-2.5 text-xs text-foreground flex flex-col gap-2'>
                      <div className='flex items-center gap-2 font-medium text-amber-700 dark:text-amber-400'>
                        <Headphones className='h-4 w-4 shrink-0' />
                        <span>{t('Support representative is ready!')}</span>
                      </div>
                      <p className='text-[11px] text-muted-foreground'>
                        {t('A representative has joined this ticket. You can switch to human support or continue with AI.')}
                      </p>
                      <Button
                        size='sm'
                        className='w-full bg-emerald-600 hover:bg-emerald-700 text-white font-medium'
                        onClick={() => handleConnectHuman(guestTicketResp.data.ticket.id)}
                        disabled={connectingHuman}
                      >
                        {connectingHuman && <Loader2 className='mr-1.5 h-3.5 w-3.5 animate-spin' />}
                        {t('Canlı Desteğe Bağlan')}
                      </Button>
                    </div>
                  )}
                </div>
              ) : loadingGuestTicket ? (
                <div className='flex items-center justify-center py-8'>
                  <Loader2 className='h-6 w-6 animate-spin text-muted-foreground' />
                </div>
              ) : (
                // Guest Creation Form
                <form onSubmit={handleGuestSubmit} className='space-y-3 py-1'>
                  <div className='rounded-lg bg-muted/50 p-2.5 text-xs text-muted-foreground border'>
                    👋 {t('Welcome! Leave your email or phone number and our team will get back to you.')}
                  </div>
                  <div className='space-y-1.5'>
                    <label className='text-xs font-medium text-foreground'>
                      {t('Email or Phone')} *
                    </label>
                    <Input
                      placeholder={t('e.g. user@example.com or +905...')}
                      value={guestContact}
                      onChange={(e) => setGuestContact(e.target.value)}
                      required
                      className='h-8 text-xs'
                    />
                  </div>
                  <div className='space-y-1.5'>
                    <label className='text-xs font-medium text-foreground'>
                      {t('How can we help?')} *
                    </label>
                    <Textarea
                      placeholder={t('Describe your question or issue...')}
                      value={guestMessage}
                      onChange={(e) => setGuestMessage(e.target.value)}
                      required
                      rows={3}
                      className='text-xs resize-none'
                    />
                  </div>
                  <Button type='submit' size='sm' className='w-full' disabled={sending}>
                    {sending ? (
                      <Loader2 className='h-4 w-4 animate-spin mr-2' />
                    ) : (
                      <Send className='h-3.5 w-3.5 mr-2' />
                    )}
                    {t('Start Conversation')}
                  </Button>
                </form>
              )
            ) : (
              // Authenticated User View
              <>
                {activeLiveTicket ? (
                  <div className='space-y-2'>
                    <div className='text-[11px] text-muted-foreground border-b pb-2'>
                      {t('Live Support')} · #{activeLiveTicket.id}
                    </div>
                    <div className='space-y-2 max-h-[230px] overflow-y-auto pr-1'>
                      {loadingLiveTicket ? (
                        <div className='flex justify-center py-4'><Loader2 className='h-5 w-5 animate-spin text-muted-foreground' /></div>
                      ) : liveTicketDetail?.data?.messages?.map((message) => (
                        <div key={message.id} className={`p-2.5 rounded-lg text-xs max-w-[88%] ${message.is_admin ? 'bg-muted border mr-auto' : 'bg-primary text-primary-foreground ml-auto'}`}>
                          <div className='text-[10px] opacity-75 font-semibold mb-1'>{message.is_admin ? t('Support Agent') : t('You')}</div>
                          <div className='whitespace-pre-wrap break-words'><MessageContent content={message.content} isAdmin={message.is_admin} /></div>
                        </div>
                      ))}
                    </div>
                    {liveTicketDetail?.data?.ticket?.is_claimed && !liveTicketDetail?.data?.ticket?.human_connected && (
                      <div className='my-2 rounded-lg border border-amber-500/30 bg-amber-500/10 p-2.5 text-xs text-foreground flex flex-col gap-2'>
                        <div className='flex items-center gap-2 font-medium text-amber-700 dark:text-amber-400'>
                          <Headphones className='h-4 w-4 shrink-0' />
                          <span>{t('Support representative is ready!')}</span>
                        </div>
                        <p className='text-[11px] text-muted-foreground'>
                          {t('A representative has joined this ticket. You can switch to human support or continue with AI.')}
                        </p>
                        <Button
                          size='sm'
                          className='w-full bg-emerald-600 hover:bg-emerald-700 text-white font-medium'
                          onClick={() => handleConnectHuman(activeLiveTicket.id)}
                          disabled={connectingHuman}
                        >
                          {connectingHuman && <Loader2 className='mr-1.5 h-3.5 w-3.5 animate-spin' />}
                          {t('Canlı Desteğe Bağlan')}
                        </Button>
                      </div>
                    )}
                  </div>
                ) : (
                  <div className='rounded-lg bg-muted/50 p-3 text-xs text-muted-foreground border'>
                    👋 {t('Welcome! Have a question or facing an issue? Send a message below and our support team will respond quickly.')}
                  </div>
                )}
              </>
            )}
          </CardContent>

          {/* Footer Input Area */}
          {!auth.user && guestSession && guestTicketResp?.data ? (
            <CardFooter className='p-3 border-t bg-card'>
              <form onSubmit={handleGuestReply} className='flex w-full items-center gap-2'>
                <Input
                  placeholder={t('Type a reply...')}
                  value={guestReply}
                  onChange={(e) => setGuestReply(e.target.value)}
                  disabled={sending}
                  className='h-9 text-xs'
                />
                <Button
                  type='submit'
                  size='icon'
                  className='h-9 w-9 shrink-0'
                  disabled={sending || !guestReply.trim()}
                >
                  {sending ? (
                    <Loader2 className='h-4 w-4 animate-spin' />
                  ) : (
                    <Send className='h-4 w-4' />
                  )}
                </Button>
              </form>
            </CardFooter>
          ) : auth.user ? (
            <CardFooter className='p-3 border-t bg-card'>
              <form onSubmit={handleSendQuickMessage} className='flex w-full items-center gap-2'>
                <Input
                  placeholder={t('Type a message to support...')}
                  value={quickMessage}
                  onChange={(e) => setQuickMessage(e.target.value)}
                  disabled={sending}
                  className='h-9 text-xs'
                />
                <Button
                  type='submit'
                  size='icon'
                  className='h-9 w-9 shrink-0'
                  disabled={sending || !quickMessage.trim()}
                >
                  {sending ? (
                    <Loader2 className='h-4 w-4 animate-spin' />
                  ) : (
                    <Send className='h-4 w-4' />
                  )}
                </Button>
              </form>
            </CardFooter>
          ) : null}
        </Card>
      ) : (
        <div className='relative inline-block'>
          {/* Small 24-hour dismiss button at top right */}
          <button
            type='button'
            aria-label='Dismiss live support'
            onClick={handleDismiss}
            title={t('Hide for 24 hours')}
            className='absolute -top-1.5 -right-1.5 z-10 flex h-5 w-5 items-center justify-center rounded-full bg-muted-foreground/80 hover:bg-destructive text-white shadow-md transition-colors'
          >
            <X className='h-3 w-3 stroke-[2.5]' />
          </button>

          <Button
            onClick={() => setOpen(true)}
            className='h-12 px-4 rounded-full shadow-lg gap-2 bg-primary text-primary-foreground hover:shadow-xl transition-all'
          >
            <MessageCircle className='h-5 w-5' />
            <span className='font-medium text-sm hidden sm:inline'>{t('Support')}</span>
          </Button>
        </div>
      )}
    </div>
  )
}
