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
import { Loader2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'

import { createTicket } from '../api'
import type { TicketCategory, TicketPriority } from '../types'

interface CreateTicketDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSuccess: () => void
}

export function CreateTicketDialog({
  open,
  onOpenChange,
  onSuccess,
}: CreateTicketDialogProps) {
  const { t } = useTranslation()
  const [loading, setLoading] = useState(false)
  const [title, setTitle] = useState('')
  const [category, setCategory] = useState<TicketCategory>('technical')
  const [priority, setPriority] = useState<TicketPriority>('normal')
  const [content, setContent] = useState('')

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!title.trim()) {
      toast.error(t('Please enter a ticket title'))
      return
    }
    if (!content.trim()) {
      toast.error(t('Please enter your message'))
      return
    }

    try {
      setLoading(true)
      const res = await createTicket({
        title: title.trim(),
        category,
        priority,
        content: content.trim(),
      })

      if (res.success) {
        toast.success(t('Ticket created successfully!'))
        setTitle('')
        setContent('')
        setCategory('technical')
        setPriority('normal')
        onOpenChange(false)
        onSuccess()
      } else {
        toast.error(res.message || t('Failed to create ticket'))
      }
    } catch {
      toast.error(t('Failed to create ticket'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(val) => {
        if (!loading) onOpenChange(val)
      }}
      title={t('Create Support Ticket')}
      description={t('Describe your issue or question and our support team will get back to you.')}
      contentClassName='sm:max-w-lg'
      bodyClassName='space-y-4 py-2'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => onOpenChange(false)}
            disabled={loading}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='button'
            onClick={handleSubmit}
            disabled={loading || !title.trim() || !content.trim()}
          >
            {loading && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
            {t('Submit Ticket')}
          </Button>
        </>
      }
    >
      <form onSubmit={handleSubmit} className='space-y-4'>
        <div className='space-y-2'>
          <Label htmlFor='ticket-title'>{t('Subject / Title')}</Label>
          <Input
            id='ticket-title'
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder={t('e.g. Question about API quota balance')}
            disabled={loading}
            maxLength={100}
          />
        </div>

        <div className='grid grid-cols-2 gap-3'>
          <div className='space-y-2'>
            <Label>{t('Category')}</Label>
            <Select
              value={category}
              onValueChange={(val) => setCategory(val as TicketCategory)}
              disabled={loading}
            >
              <SelectTrigger>
                <SelectValue placeholder={t('Select category')} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='billing'>{t('Billing & Payments')}</SelectItem>
                <SelectItem value='technical'>{t('Technical Issue')}</SelectItem>
                <SelectItem value='account'>{t('Account & Auth')}</SelectItem>
                <SelectItem value='feature_request'>{t('Feature Request')}</SelectItem>
                <SelectItem value='other'>{t('Other')}</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className='space-y-2'>
            <Label>{t('Priority')}</Label>
            <Select
              value={priority}
              onValueChange={(val) => setPriority(val as TicketPriority)}
              disabled={loading}
            >
              <SelectTrigger>
                <SelectValue placeholder={t('Select priority')} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='low'>{t('Low')}</SelectItem>
                <SelectItem value='normal'>{t('Normal')}</SelectItem>
                <SelectItem value='high'>{t('High')}</SelectItem>
                <SelectItem value='urgent'>{t('Urgent')}</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>

        <div className='space-y-2'>
          <Label htmlFor='ticket-content'>{t('Detailed Message')}</Label>
          <Textarea
            id='ticket-content'
            value={content}
            onChange={(e) => setContent(e.target.value)}
            placeholder={t('Please provide details, error messages, or steps to reproduce...')}
            rows={5}
            disabled={loading}
          />
        </div>
      </form>
    </Dialog>
  )
}
