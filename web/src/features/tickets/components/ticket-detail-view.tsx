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
import dayjs from 'dayjs'
import {
  ArrowLeft,
  CheckCircle2,
  Download,
  Headphones,
  Loader2,
  Send,
  ShieldCheck,
  User,
} from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Textarea } from '@/components/ui/textarea'
import { useAuthStore } from '@/stores/auth-store'

import {
  addTicketMessage,
  adminAddTicketMessage,
  closeTicket,
  downloadTicketTranscript,
  getTicketDetail,
  claimTicket,
  connectHumanSupport,
} from '../api'
import type { Ticket, TicketMessage } from '../types'
import {
  TicketCategoryBadge,
  TicketPriorityBadge,
  TicketStatusBadge,
} from './ticket-badges'

interface TicketDetailViewProps {
  ticketId: number
  onBack: () => void
  isAdminView?: boolean
}

export function TicketDetailView({
  ticketId,
  onBack,
  isAdminView = false,
}: TicketDetailViewProps) {
  const { t } = useTranslation()
  const { auth } = useAuthStore()
  const [ticket, setTicket] = useState<Ticket | null>(null)
  const [messages, setMessages] = useState<TicketMessage[]>([])
  const [loading, setLoading] = useState(true)
  const [replyText, setReplyText] = useState('')
  const [sending, setSending] = useState(false)
  const [closing, setClosing] = useState(false)
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const wsRef = useRef<WebSocket | null>(null)

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }

  const fetchDetail = async () => {
    try {
      setLoading(true)
      const res = await getTicketDetail(ticketId)
      if (res.success && res.data) {
        setTicket(res.data.ticket)
        setMessages(res.data.messages || [])
      } else {
        toast.error(res.message || t('Failed to load ticket details'))
      }
    } catch {
      toast.error(t('Failed to load ticket details'))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void fetchDetail()
  }, [ticketId])

  useEffect(() => {
    scrollToBottom()
  }, [messages])

  // WebSocket for real-time live support chat
  useEffect(() => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const host = window.location.host
    const token = auth.accessToken || auth.session?.sid || ''
    const wsUrl = `${protocol}//${host}/api/ticket/ws?ticket_id=${ticketId}&token=${encodeURIComponent(token)}`

    const ws = new WebSocket(wsUrl)
    wsRef.current = ws

    ws.onopen = () => {
      ws.send(JSON.stringify({ type: 'subscribe', ticket_id: ticketId }))
    }

    ws.onmessage = (event) => {
      try {
        const payload = JSON.parse(event.data)
        if (payload.type === 'new_message' && payload.data) {
          const newMsg = payload.data as TicketMessage
          setMessages((prev) => {
            if (prev.some((m) => m.id === newMsg.id)) return prev
            return [...prev, newMsg]
          })
          if (newMsg.is_admin) {
            setTicket((prev) => (prev ? { ...prev, status: 'answered' } : prev))
          } else {
            setTicket((prev) => (prev ? { ...prev, status: 'open' } : prev))
          }
        } else if (payload.type === 'status_changed' && payload.data?.status) {
          setTicket((prev) => (prev ? { ...prev, status: payload.data.status } : prev))
        }
      } catch {
        /* parse error */
      }
    }

    return () => {
      ws.close()
    }
  }, [ticketId, auth.session?.sid])

  const handleSendReply = async () => {
    if (!replyText.trim()) return

    try {
      setSending(true)
      const res = isAdminView
        ? await adminAddTicketMessage(ticketId, replyText.trim())
        : await addTicketMessage(ticketId, replyText.trim())
      if (res.success && res.data) {
        setReplyText('')
        setMessages((prev) => {
          if (prev.some((m) => m.id === res.data.id)) return prev
          return [...prev, res.data]
        })
        if (isAdminView) {
          setTicket((prev) => (prev ? { ...prev, status: 'answered' } : prev))
        } else {
          setTicket((prev) => (prev ? { ...prev, status: 'open' } : prev))
        }
      } else {
        toast.error(res.message || t('Failed to send reply'))
      }
    } catch {
      toast.error(t('Failed to send reply'))
    } finally {
      setSending(false)
    }
  }

  const handleCloseTicket = async () => {
    try {
      setClosing(true)
      const res = await closeTicket(ticketId)
      if (res.success) {
        toast.success(t('Ticket marked as closed'))
        setTicket((prev) => (prev ? { ...prev, status: 'closed' } : prev))
      } else {
        toast.error(res.message || t('Failed to close ticket'))
      }
    } catch {
      toast.error(t('Failed to close ticket'))
    } finally {
      setClosing(false)
    }
  }

  if (loading) {
    return (
      <div className='flex h-64 items-center justify-center'>
        <Loader2 className='h-8 w-8 animate-spin text-muted-foreground' />
      </div>
    )
  }

  if (!ticket) {
    return (
      <div className='flex flex-col items-center justify-center gap-4 py-12'>
        <p className='text-muted-foreground'>{t('Ticket not found')}</p>
        <Button variant='outline' onClick={onBack}>
          <ArrowLeft className='mr-2 h-4 w-4' />
          {t('Go Back')}
        </Button>
      </div>
    )
  }

  const isClosed = ticket.status === 'closed'

  return (
    <div className='space-y-4'>
      {/* Header Bar */}
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <div className='flex items-center gap-3'>
          <Button variant='ghost' size='sm' onClick={onBack}>
            <ArrowLeft className='mr-1 h-4 w-4' />
            {t('Back to list')}
          </Button>
          <h2 className='text-xl font-bold tracking-tight'>
            #{ticket.id} - {ticket.title}
          </h2>
        </div>

        <div className='flex items-center gap-2'>
          <TicketCategoryBadge category={ticket.category} />
          <TicketPriorityBadge priority={ticket.priority} />
          <TicketStatusBadge status={ticket.status} />

          <Button
            variant='outline'
            size='sm'
            title={t('Download Transcript')}
            onClick={async () => {
              try {
                await downloadTicketTranscript(ticket.id)
              } catch {
                toast.error(t('Failed to download transcript'))
              }
            }}
          >
            <Download className='mr-1.5 h-3.5 w-3.5' />
            {t('Download')}
          </Button>

          {isAdminView && ticket.is_live_support && !ticket.is_claimed && (
            <Button
              variant='outline'
              size='sm'
              className='bg-blue-600/10 text-blue-600 hover:bg-blue-600/20 border-blue-500/30'
              onClick={async () => {
                try {
                  const res = await claimTicket(ticket.id)
                  if (res.success) {
                    toast.success(t('Talebi başarıyla üstlendiniz'))
                    setTicket((prev) => (prev ? { ...prev, is_claimed: true } : prev))
                  }
                } catch {
                  toast.error(t('Talebi üstlenme başarısız oldu'))
                }
              }}
            >
              <Headphones className='mr-1.5 h-3.5 w-3.5' />
              {t('Talebi Üstlen')}
            </Button>
          )}

          {!isClosed && (
            <Button
              variant='outline'
              size='sm'
              onClick={handleCloseTicket}
              disabled={closing}
            >
              {closing ? (
                <Loader2 className='mr-1.5 h-3.5 w-3.5 animate-spin' />
              ) : (
                <CheckCircle2 className='mr-1.5 h-3.5 w-3.5 text-emerald-500' />
              )}
              {t('Close Ticket')}
            </Button>
          )}
        </div>
      </div>

      {/* Info card for admin or user */}
      {isAdminView && ticket.user_name && (
        <div className='flex items-center gap-4 rounded-none border border-border bg-muted/30 px-4 py-2 text-sm'>
          <span className='font-medium text-muted-foreground'>{t('User')}:</span>
          <span>{ticket.user_name}</span>
          {ticket.user_email && (
            <span className='text-muted-foreground'>({ticket.user_email})</span>
          )}
          <span className='ml-auto text-xs text-muted-foreground'>
            {t('Created at')}: {dayjs(ticket.created_at * 1000).format('YYYY-MM-DD HH:mm')}
          </span>
        </div>
      )}

      {/* Conversation Thread */}
      <Card className='flex flex-col rounded-none border border-border shadow-sm'>
        <CardHeader className='border-b border-border py-2.5 px-4 bg-muted/40 rounded-none'>
          <div className='flex items-center justify-between text-xs text-muted-foreground'>
            <span>{t('Conversation History')}</span>
            <span>
              {t('Last updated')}:{' '}
              {dayjs(ticket.last_reply_at * 1000).format('YYYY-MM-DD HH:mm')}
            </span>
          </div>
        </CardHeader>

        <CardContent className='flex min-h-[300px] flex-col overflow-y-auto p-4 space-y-4 bg-muted/10'>
          {!isAdminView && ticket.is_claimed && !ticket.human_connected && (
            <div className='rounded-none border border-amber-500/40 bg-amber-500/10 p-3 text-xs text-foreground flex items-center justify-between gap-3'>
              <div>
                <span className='font-bold text-amber-800 dark:text-amber-300'>{t('Destek yetkilisi talebinizi üstlendi!')}</span>
                <p className='text-muted-foreground text-[11px]'>{t('İsterseniz canlı desteğe bağlanabilir veya yapay zeka ile görüşmeye devam edebilirsiniz.')}</p>
              </div>
              <Button
                size='sm'
                className='rounded-none bg-emerald-600 hover:bg-emerald-700 text-white font-medium shrink-0'
                onClick={async () => {
                  try {
                    const res = await connectHumanSupport(ticket.id)
                    if (res.success) {
                      toast.success(t('Canlı desteğe bağlandınız'))
                      setTicket((prev) => (prev ? { ...prev, human_connected: true } : prev))
                    }
                  } catch {
                    toast.error(t('Bağlantı hatası'))
                  }
                }}
              >
                {t('Canlı Desteğe Bağlan')}
              </Button>
            </div>
          )}

          {messages.map((msg, idx) => {
            const isStaff = msg.is_admin
            const postNumber = idx + 1
            return (
              <div
                key={msg.id}
                className='rounded-none border border-border bg-card shadow-sm transition-all overflow-hidden'
              >
                {/* XenForo Post Header Bar */}
                <div className='flex items-center justify-between border-b border-border px-4 py-2 bg-muted/50 text-xs text-muted-foreground rounded-none'>
                  <div className='flex items-center gap-2'>
                    <span className='font-medium text-foreground'>
                      {dayjs(msg.created_at * 1000).format('YYYY-MM-DD HH:mm:ss')}
                    </span>
                  </div>
                  <div className='flex items-center gap-2'>
                    <span className='font-mono font-semibold text-muted-foreground'>
                      #{postNumber}
                    </span>
                  </div>
                </div>

                {/* XenForo Post Body with Author Sidebar */}
                <div className='flex flex-col sm:flex-row'>
                  {/* Left: Author Block */}
                  <div className='flex sm:flex-col items-center sm:items-center justify-between sm:justify-start gap-3 p-4 border-b sm:border-b-0 sm:border-r border-border sm:w-44 bg-muted/20 shrink-0 text-center rounded-none'>
                    <Avatar className='h-12 w-12 rounded-none border border-border shadow-sm'>
                      <AvatarFallback
                        className={
                          isStaff
                            ? 'rounded-none bg-slate-700 text-white dark:bg-slate-800 dark:text-slate-100 font-bold text-base'
                            : 'rounded-none bg-muted-foreground/10 text-foreground font-bold text-base'
                        }
                      >
                        {isStaff ? (
                          <ShieldCheck className='h-6 w-6' />
                        ) : (
                          <User className='h-6 w-6' />
                        )}
                      </AvatarFallback>
                    </Avatar>
                    <div className='flex flex-col items-center gap-1 min-w-0'>
                      <span className='font-bold text-sm text-foreground truncate max-w-[150px]'>
                        {isStaff
                          ? t('Support Team')
                          : msg.user_name || t('User')}
                      </span>
                      {isStaff ? (
                        <span className='inline-flex items-center gap-1 rounded-none bg-slate-100 dark:bg-slate-800 text-slate-800 dark:text-slate-200 border border-slate-300 dark:border-slate-700 px-2 py-0.5 text-[10px] font-bold uppercase tracking-wider'>
                          {t('Staff Member')}
                        </span>
                      ) : (
                        <span className='inline-flex items-center rounded-none bg-muted text-muted-foreground border border-border px-2 py-0.5 text-[10px] font-medium'>
                          {t('Member')}
                        </span>
                      )}
                    </div>
                  </div>

                  {/* Right: Message Content */}
                  <div className='flex-1 p-5 min-w-0 flex flex-col justify-between'>
                    <div className='text-sm whitespace-pre-wrap leading-relaxed text-foreground select-text'>
                      {msg.content}
                    </div>
                  </div>
                </div>
              </div>
            )
          })}
          <div ref={messagesEndRef} />
        </CardContent>

        {/* Reply Area */}
        <div className='border-t p-4 bg-card'>
          {isClosed && !isAdminView ? (
            <div className='rounded-md border border-dashed p-4 text-center text-sm text-muted-foreground'>
              {t('This ticket is closed. To ask more questions, please create a new ticket.')}
            </div>
          ) : (
            <div className='space-y-3'>
              <Textarea
                placeholder={
                  isAdminView
                    ? t('Write your response to the user...')
                    : t('Type your reply here...')
                }
                value={replyText}
                onChange={(e) => setReplyText(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
                    e.preventDefault()
                    void handleSendReply()
                  }
                }}
                rows={3}
                disabled={sending}
              />
              <div className='flex items-center justify-between'>
                <span className='text-xs text-muted-foreground'>
                  {t('Press Ctrl + Enter to send')}
                </span>
                <Button
                  onClick={handleSendReply}
                  disabled={sending || !replyText.trim()}
                  size='sm'
                >
                  {sending ? (
                    <Loader2 className='mr-1.5 h-3.5 w-3.5 animate-spin' />
                  ) : (
                    <Send className='mr-1.5 h-3.5 w-3.5' />
                  )}
                  {t('Send Reply')}
                </Button>
              </div>
            </div>
          )}
        </div>
      </Card>
    </div>
  )
}
