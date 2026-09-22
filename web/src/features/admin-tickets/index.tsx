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
  AlertCircle,
  CheckCircle2,
  Clock,
  Headphones,
  Loader2,
  MessageSquare,
  Search,
  Trash2,
} from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  adminDeleteTicket,
  adminGetAllTickets,
  adminUpdateTicket,
} from '@/features/tickets/api'
import {
  TicketCategoryBadge,
  TicketPriorityBadge,
} from '@/features/tickets/components/ticket-badges'
import { TicketDetailView } from '@/features/tickets/components/ticket-detail-view'
import type { Ticket } from '@/features/tickets/types'

export function AdminTicketsPage() {
  const { t } = useTranslation()
  const [selectedTicketId, setSelectedTicketId] = useState<number | null>(null)
  const [tickets, setTickets] = useState<Ticket[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [categoryFilter, setCategoryFilter] = useState<string>('all')
  const [priorityFilter, setPriorityFilter] = useState<string>('all')
  const [search, setSearch] = useState('')

  const fetchTickets = async () => {
    try {
      setLoading(true)
      const res = await adminGetAllTickets({
        status: statusFilter === 'all' ? undefined : statusFilter,
        category: categoryFilter === 'all' ? undefined : categoryFilter,
        priority: priorityFilter === 'all' ? undefined : priorityFilter,
        search: search.trim() || undefined,
        page: 1,
        pageSize: 50,
      })
      if (res.success && res.data) {
        setTickets(res.data.items || [])
        setTotal(res.data.total || 0)
      } else {
        toast.error(res.message || t('Failed to load tickets'))
      }
    } catch {
      toast.error(t('Failed to load tickets'))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    void fetchTickets()
  }, [statusFilter, categoryFilter, priorityFilter])

  const handleSearchSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    void fetchTickets()
  }

  const handleStatusChange = async (ticketId: number, newStatus: string) => {
    try {
      const res = await adminUpdateTicket(ticketId, { status: newStatus })
      if (res.success) {
        toast.success(t('Ticket status updated'))
        setTickets((prev) =>
          prev.map((tk) =>
            tk.id === ticketId ? { ...tk, status: newStatus as any } : tk
          )
        )
      } else {
        toast.error(res.message || t('Failed to update ticket status'))
      }
    } catch {
      toast.error(t('Failed to update ticket status'))
    }
  }

  const handleDelete = async (ticketId: number) => {
    if (!window.confirm(t('Are you sure you want to delete this ticket?'))) {
      return
    }

    try {
      const res = await adminDeleteTicket(ticketId)
      if (res.success) {
        toast.success(t('Ticket deleted successfully'))
        setTickets((prev) => prev.filter((tk) => tk.id !== ticketId))
        setTotal((prev) => Math.max(0, prev - 1))
      } else {
        toast.error(res.message || t('Failed to delete ticket'))
      }
    } catch {
      toast.error(t('Failed to delete ticket'))
    }
  }

  if (selectedTicketId !== null) {
    return (
      <div className='container mx-auto p-4 md:p-6'>
        <TicketDetailView
          ticketId={selectedTicketId}
          isAdminView={true}
          onBack={() => {
            setSelectedTicketId(null)
            void fetchTickets()
          }}
        />
      </div>
    )
  }

  const openCount = tickets.filter((tk) => tk.status === 'open').length
  const answeredCount = tickets.filter((tk) => tk.status === 'answered').length
  const closedCount = tickets.filter((tk) => tk.status === 'closed').length

  return (
    <div className='container mx-auto space-y-6 p-4 md:p-6'>
      {/* Page Header */}
      <div>
        <h1 className='text-2xl font-bold tracking-tight flex items-center gap-2'>
          <Headphones className='h-6 w-6 text-primary' />
          {t('Ticket Management')}
        </h1>
        <p className='text-sm text-muted-foreground'>
          {t('Manage, review, and reply to user support requests.')}
        </p>
      </div>

      {/* Summary Cards */}
      <div className='grid gap-4 md:grid-cols-4'>
        <Card>
          <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
            <CardTitle className='text-sm font-medium'>{t('Total Tickets')}</CardTitle>
            <MessageSquare className='h-4 w-4 text-muted-foreground' />
          </CardHeader>
          <CardContent>
            <div className='text-2xl font-bold'>{total}</div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
            <CardTitle className='text-sm font-medium'>{t('Open / Unanswered')}</CardTitle>
            <Clock className='h-4 w-4 text-amber-500' />
          </CardHeader>
          <CardContent>
            <div className='text-2xl font-bold text-amber-500'>{openCount}</div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
            <CardTitle className='text-sm font-medium'>{t('Answered')}</CardTitle>
            <Headphones className='h-4 w-4 text-emerald-500' />
          </CardHeader>
          <CardContent>
            <div className='text-2xl font-bold text-emerald-500'>{answeredCount}</div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
            <CardTitle className='text-sm font-medium'>{t('Closed')}</CardTitle>
            <CheckCircle2 className='h-4 w-4 text-muted-foreground' />
          </CardHeader>
          <CardContent>
            <div className='text-2xl font-bold text-muted-foreground'>{closedCount}</div>
          </CardContent>
        </Card>
      </div>

      {/* Filter and Search Bar */}
      <Card>
        <CardHeader className='py-4 px-6 border-b'>
          <div className='flex flex-wrap items-center justify-between gap-4'>
            <div className='flex flex-wrap items-center gap-2'>
              {/* Status Select */}
              <Select
                value={statusFilter}
                onValueChange={(val) => setStatusFilter(val ?? 'all')}
              >
                <SelectTrigger className='w-[130px] h-8 text-xs'>
                  <SelectValue placeholder={t('Status')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='all'>{t('All Statuses')}</SelectItem>
                  <SelectItem value='open'>{t('Open')}</SelectItem>
                  <SelectItem value='answered'>{t('Answered')}</SelectItem>
                  <SelectItem value='waiting_user'>{t('Waiting for User')}</SelectItem>
                  <SelectItem value='closed'>{t('Closed')}</SelectItem>
                </SelectContent>
              </Select>

              {/* Category Select */}
              <Select
                value={categoryFilter}
                onValueChange={(val) => setCategoryFilter(val ?? 'all')}
              >
                <SelectTrigger className='w-[130px] h-8 text-xs'>
                  <SelectValue placeholder={t('Category')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='all'>{t('All Categories')}</SelectItem>
                  <SelectItem value='billing'>{t('Billing')}</SelectItem>
                  <SelectItem value='technical'>{t('Technical')}</SelectItem>
                  <SelectItem value='account'>{t('Account')}</SelectItem>
                  <SelectItem value='feature_request'>{t('Feature Request')}</SelectItem>
                  <SelectItem value='other'>{t('Other')}</SelectItem>
                </SelectContent>
              </Select>

              {/* Priority Select */}
              <Select
                value={priorityFilter}
                onValueChange={(val) => setPriorityFilter(val ?? 'all')}
              >
                <SelectTrigger className='w-[120px] h-8 text-xs'>
                  <SelectValue placeholder={t('Priority')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='all'>{t('All Priorities')}</SelectItem>
                  <SelectItem value='urgent'>{t('Urgent')}</SelectItem>
                  <SelectItem value='high'>{t('High')}</SelectItem>
                  <SelectItem value='normal'>{t('Normal')}</SelectItem>
                  <SelectItem value='low'>{t('Low')}</SelectItem>
                </SelectContent>
              </Select>
            </div>

            <form onSubmit={handleSearchSubmit} className='flex items-center gap-2'>
              <div className='relative'>
                <Search className='absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground' />
                <Input
                  type='search'
                  placeholder={t('Search by user or subject...')}
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  className='w-[200px] md:w-[260px] pl-8 h-8 text-xs'
                />
              </div>
              <Button type='submit' variant='outline' size='sm' className='h-8'>
                {t('Search')}
              </Button>
            </form>
          </div>
        </CardHeader>

        <CardContent className='p-0'>
          {loading ? (
            <div className='flex h-48 items-center justify-center'>
              <Loader2 className='h-6 w-6 animate-spin text-muted-foreground' />
            </div>
          ) : tickets.length === 0 ? (
            <div className='flex flex-col items-center justify-center py-12 text-center text-muted-foreground'>
              <AlertCircle className='h-10 w-10 stroke-[1.5] text-muted-foreground/50 mb-2' />
              <p className='text-sm font-medium'>{t('No tickets match the selected filters')}</p>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className='w-[70px]'>ID</TableHead>
                  <TableHead className='w-[160px]'>{t('User')}</TableHead>
                  <TableHead>{t('Subject')}</TableHead>
                  <TableHead className='w-[130px]'>{t('Category')}</TableHead>
                  <TableHead className='w-[100px]'>{t('Priority')}</TableHead>
                  <TableHead className='w-[150px]'>{t('Status')}</TableHead>
                  <TableHead className='w-[150px]'>{t('Last Reply')}</TableHead>
                  <TableHead className='w-[120px] text-right'>{t('Actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {tickets.map((tk) => (
                  <TableRow
                    key={tk.id}
                    className='cursor-pointer hover:bg-muted/50'
                    onClick={() => setSelectedTicketId(tk.id)}
                  >
                    <TableCell className='font-mono font-medium text-xs'>
                      #{tk.id}
                    </TableCell>
                    <TableCell>
                      <div className='flex flex-col'>
                        <span className='font-medium text-xs'>{tk.user_name || `User #${tk.user_id}`}</span>
                        {tk.user_email && (
                          <span className='text-[11px] text-muted-foreground truncate max-w-[140px]'>
                            {tk.user_email}
                          </span>
                        )}
                      </div>
                    </TableCell>
                    <TableCell className='font-medium text-sm'>
                      {tk.title}
                    </TableCell>
                    <TableCell>
                      <TicketCategoryBadge category={tk.category} />
                    </TableCell>
                    <TableCell>
                      <TicketPriorityBadge priority={tk.priority} />
                    </TableCell>
                    <TableCell onClick={(e) => e.stopPropagation()}>
                      <Select
                        value={tk.status}
                        onValueChange={(val) => {
                          if (val) void handleStatusChange(tk.id, val)
                        }}
                      >
                        <SelectTrigger className='h-7 text-xs w-[130px]'>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value='open'>{t('Open')}</SelectItem>
                          <SelectItem value='answered'>{t('Answered')}</SelectItem>
                          <SelectItem value='waiting_user'>{t('Waiting for User')}</SelectItem>
                          <SelectItem value='closed'>{t('Closed')}</SelectItem>
                        </SelectContent>
                      </Select>
                    </TableCell>
                    <TableCell className='text-xs text-muted-foreground'>
                      {dayjs(tk.last_reply_at * 1000).format('YYYY-MM-DD HH:mm')}
                    </TableCell>
                    <TableCell className='text-right' onClick={(e) => e.stopPropagation()}>
                      <div className='flex items-center justify-end gap-1'>
                        <Button
                          variant='ghost'
                          size='sm'
                          onClick={() => setSelectedTicketId(tk.id)}
                        >
                          {t('Reply')}
                        </Button>
                        <Button
                          variant='ghost'
                          size='icon'
                          className='h-8 w-8 text-destructive hover:text-destructive'
                          onClick={() => void handleDelete(tk.id)}
                        >
                          <Trash2 className='h-4 w-4' />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
