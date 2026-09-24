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
  Download,
  LifeBuoy,
  Loader2,
  Plus,
  Search,
} from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

import { downloadTicketTranscript, getUserTickets } from './api'
import { CreateTicketDialog } from './components/create-ticket-dialog'
import {
  TicketCategoryBadge,
  TicketPriorityBadge,
  TicketStatusBadge,
} from './components/ticket-badges'
import { TicketDetailView } from './components/ticket-detail-view'
import type { Ticket } from './types'

export function UserTicketsPage() {
  const { t } = useTranslation()
  const [selectedTicketId, setSelectedTicketId] = useState<number | null>(null)
  const [tickets, setTickets] = useState<Ticket[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [search, setSearch] = useState('')
  const [createDialogOpen, setCreateDialogOpen] = useState(false)

  const fetchTickets = async () => {
    try {
      setLoading(true)
      const res = await getUserTickets({
        status: statusFilter === 'all' ? undefined : statusFilter,
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
  }, [statusFilter])

  const handleSearchSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    void fetchTickets()
  }

  if (selectedTicketId !== null) {
    return (
      <div className='h-full flex-1 overflow-y-auto p-4 md:p-6'>
        <TicketDetailView
          ticketId={selectedTicketId}
          onBack={() => {
            setSelectedTicketId(null)
            void fetchTickets()
          }}
        />
      </div>
    )
  }

  return (
    <div className='h-full flex-1 overflow-y-auto space-y-6 p-4 md:p-6'>
      {/* Page Header */}
      <div className='flex flex-wrap items-center justify-between gap-4'>
        <div>
          <h1 className='text-2xl font-bold tracking-tight flex items-center gap-2'>
            <LifeBuoy className='h-6 w-6 text-primary' />
            {t('Support Tickets')}
          </h1>
          <p className='text-sm text-muted-foreground'>
            {t('Create and track your support inquiries, issues, and questions.')}
          </p>
        </div>

        <Button onClick={() => setCreateDialogOpen(true)}>
          <Plus className='mr-2 h-4 w-4' />
          {t('New Ticket')}
        </Button>
      </div>

      {/* Filters and Search Bar */}
      <Card>
        <CardHeader className='py-4 px-6 border-b'>
          <div className='flex flex-wrap items-center justify-between gap-4'>
            <div className='flex items-center gap-1 overflow-x-auto'>
              {[
                { id: 'all', label: t('All') },
                { id: 'open', label: t('Open') },
                { id: 'answered', label: t('Answered') },
                { id: 'closed', label: t('Closed') },
              ].map((tab) => (
                <Button
                  key={tab.id}
                  variant={statusFilter === tab.id ? 'secondary' : 'ghost'}
                  size='sm'
                  onClick={() => setStatusFilter(tab.id)}
                  className='text-xs'
                >
                  {tab.label}
                </Button>
              ))}
            </div>

            <form onSubmit={handleSearchSubmit} className='flex items-center gap-2'>
              <div className='relative'>
                <Search className='absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground' />
                <Input
                  type='search'
                  placeholder={t('Search tickets...')}
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  className='w-[200px] md:w-[260px] pl-8 h-9 text-xs'
                />
              </div>
              <Button type='submit' variant='outline' size='sm' className='h-9'>
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
              <LifeBuoy className='h-12 w-12 stroke-[1.5] text-muted-foreground/50 mb-3' />
              <p className='text-base font-medium'>{t('No tickets found')}</p>
              <p className='text-xs mt-1'>{t('You do not have any support tickets yet.')}</p>
              <Button
                variant='outline'
                size='sm'
                className='mt-4'
                onClick={() => setCreateDialogOpen(true)}
              >
                <Plus className='mr-1 h-3.5 w-3.5' />
                {t('Create Your First Ticket')}
              </Button>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className='w-[80px]'>ID</TableHead>
                  <TableHead>{t('Subject')}</TableHead>
                  <TableHead className='w-[140px]'>{t('Category')}</TableHead>
                  <TableHead className='w-[100px]'>{t('Priority')}</TableHead>
                  <TableHead className='w-[120px]'>{t('Status')}</TableHead>
                  <TableHead className='w-[160px]'>{t('Last Updated')}</TableHead>
                  <TableHead className='w-[120px] text-right'>{t('Action')}</TableHead>
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
                    <TableCell className='font-medium'>
                      {tk.title}
                    </TableCell>
                    <TableCell>
                      <TicketCategoryBadge category={tk.category} />
                    </TableCell>
                    <TableCell>
                      <TicketPriorityBadge priority={tk.priority} />
                    </TableCell>
                    <TableCell>
                      <TicketStatusBadge status={tk.status} />
                    </TableCell>
                    <TableCell className='text-xs text-muted-foreground'>
                      {dayjs(tk.last_reply_at * 1000).format('YYYY-MM-DD HH:mm')}
                    </TableCell>
                    <TableCell className='text-right'>
                      <div className='flex items-center justify-end gap-1' onClick={(e) => e.stopPropagation()}>
                        {tk.status === 'closed' && (
                          <Button
                            variant='ghost'
                            size='icon'
                            className='h-8 w-8'
                            title={t('Download Transcript')}
                            onClick={async () => {
                              try {
                                await downloadTicketTranscript(tk.id)
                              } catch {
                                toast.error(t('Failed to download transcript'))
                              }
                            }}
                          >
                            <Download className='h-4 w-4' />
                          </Button>
                        )}
                        <Button
                          variant='ghost'
                          size='sm'
                          onClick={() => setSelectedTicketId(tk.id)}
                        >
                          {t('View')}
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

      {/* Create Ticket Dialog */}
      <CreateTicketDialog
        open={createDialogOpen}
        onOpenChange={setCreateDialogOpen}
        onSuccess={() => void fetchTickets()}
      />
    </div>
  )
}
