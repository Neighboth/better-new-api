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
  closeTicket,
  downloadTicketTranscript,
  getTicketDetail,
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
    const token = auth.session?.sid || ''
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
      const res = await addTicketMessage(ticketId, replyText.trim())
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
        <div className='flex items-center gap-4 rounded-lg border bg-muted/30 px-4 py-2 text-sm'>
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
      <Card className='flex flex-col'>
        <CardHeader className='border-b py-3 px-4'>
          <div className='flex items-center justify-between text-xs text-muted-foreground'>
            <span>{t('Conversation History')}</span>
            <span>
              {t('Last updated')}:{' '}
              {dayjs(ticket.last_reply_at * 1000).format('YYYY-MM-DD HH:mm')}
            </span>
          </div>
        </CardHeader>

        <CardContent className='flex max-h-[600px] min-h-[300px] flex-col overflow-y-auto p-4 space-y-4'>
          {messages.map((msg) => {
            const isStaff = msg.is_admin
            return (
              <div
                key={msg.id}
                className={`flex gap-3 ${
                  isStaff
                    ? 'flex-row-reverse self-end max-w-[85%]'
                    : 'self-start max-w-[85%]'
                }`}
              >
                <Avatar className='h-8 w-8 mt-1 shrink-0'>
                  <AvatarFallback
                    className={
                      isStaff
                        ? 'bg-primary text-primary-foreground'
                        : 'bg-muted text-muted-foreground'
                    }
                  >
                    {isStaff ? (
                      <Headphones className='h-4 w-4' />
                    ) : (
                      <User className='h-4 w-4' />
                    )}
                  </AvatarFallback>
                </Avatar>

                <div className='flex flex-col gap-1'>
                  <div
                    className={`flex items-center gap-2 text-xs text-muted-foreground ${
                      isStaff ? 'justify-end' : ''
                    }`}
                  >
                    <span className='font-semibold text-foreground'>
                      {isStaff
                        ? t('Support Team')
                        : msg.user_name || t('User')}
                    </span>
                    {isStaff && (
                      <span className='inline-flex items-center gap-0.5 rounded bg-primary/10 px-1 py-0.2 text-[10px] font-medium text-primary'>
                        <ShieldCheck className='h-3 w-3' />
                        {t('Staff')}
                      </span>
                    )}
                    <span>•</span>
                    <span>
                      {dayjs(msg.created_at * 1000).format('MMM D, HH:mm')}
                    </span>
                  </div>

                  <div
                    className={`rounded-2xl px-4 py-2.5 text-sm whitespace-pre-wrap leading-relaxed shadow-xs ${
                      isStaff
                        ? 'bg-primary text-primary-foreground rounded-tr-none'
                        : 'bg-muted/80 text-foreground border rounded-tl-none'
                    }`}
                  >
                    {msg.content}
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
