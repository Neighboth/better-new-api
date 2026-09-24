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
import { api } from '@/lib/api'

import type {
  CreateTicketInput,
  Ticket,
  TicketConfig,
  TicketDetailResponse,
  TicketListResponse,
  TicketMessage,
} from './types'

export interface ApiResponse<T = unknown> {
  success: boolean
  message?: string
  data: T
}

export async function getTicketConfig(): Promise<ApiResponse<TicketConfig>> {
  const res = await api.get<ApiResponse<TicketConfig>>('/api/ticket/config')
  return res.data
}

export async function getUserTickets(params: {
  page?: number
  pageSize?: number
  status?: string
  search?: string
}): Promise<ApiResponse<TicketListResponse>> {
  const res = await api.get<ApiResponse<TicketListResponse>>('/api/ticket', {
    params: {
      page: params.page ?? 1,
      page_size: params.pageSize ?? 20,
      status: params.status,
      search: params.search,
    },
    skipErrorHandler: true,
  })
  return res.data
}

export async function createTicket(
  data: CreateTicketInput
): Promise<ApiResponse<Ticket>> {
  const res = await api.post<ApiResponse<Ticket>>('/api/ticket', data)
  return res.data
}

export async function getTicketDetail(
  id: number
): Promise<ApiResponse<TicketDetailResponse>> {
  const res = await api.get<ApiResponse<TicketDetailResponse>>(`/api/ticket/${id}`)
  return res.data
}

export async function addTicketMessage(
  ticketId: number,
  content: string,
  attachments?: string
): Promise<ApiResponse<TicketMessage>> {
  const res = await api.post<ApiResponse<TicketMessage>>(
    `/api/ticket/${ticketId}/message`,
    {
      content,
      attachments,
    }
  )
  return res.data
}

export async function closeTicket(id: number): Promise<ApiResponse<void>> {
  const res = await api.put<ApiResponse<void>>(`/api/ticket/${id}/close`)
  return res.data
}

// Admin APIs

export async function adminGetAllTickets(params: {
  page?: number
  pageSize?: number
  status?: string
  category?: string
  priority?: string
  search?: string
}): Promise<ApiResponse<TicketListResponse>> {
  const res = await api.get<ApiResponse<TicketListResponse>>('/api/admin/ticket', {
    params: {
      page: params.page ?? 1,
      page_size: params.pageSize ?? 20,
      status: params.status,
      category: params.category,
      priority: params.priority,
      search: params.search,
    },
  })
  return res.data
}

export async function adminUpdateTicket(
  id: number,
  data: { status?: string; priority?: string }
): Promise<ApiResponse<void>> {
  const res = await api.put<ApiResponse<void>>(`/api/admin/ticket/${id}`, data)
  return res.data
}

export async function adminDeleteTicket(id: number): Promise<ApiResponse<void>> {
  const res = await api.delete<ApiResponse<void>>(`/api/admin/ticket/${id}`)
  return res.data
}

export async function adminPermanentDeleteTicket(id: number): Promise<ApiResponse<void>> {
  const res = await api.delete<ApiResponse<void>>(`/api/admin/ticket/${id}/permanent`)
  return res.data
}

export async function downloadTicketTranscript(id: number): Promise<void> {
  const res = await api.get(`/api/ticket/${id}/transcript`, { responseType: 'blob' })
  const blob = new Blob([res.data], { type: 'text/markdown;charset=utf-8' })
  const url = window.URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `ticket-${id}-transcript.md`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  window.URL.revokeObjectURL(url)
}

// Guest Ticket APIs
export interface GuestTicketResponse {
  ticket: Ticket
  messages: TicketMessage[]
}

export async function createGuestTicket(data: {
  contact: string
  content: string
}): Promise<ApiResponse<Ticket>> {
  const res = await api.post<ApiResponse<Ticket>>('/api/ticket/guest', data)
  return res.data
}

export async function getGuestTicket(
  sessionKey: string
): Promise<ApiResponse<GuestTicketResponse>> {
  const res = await api.get<ApiResponse<GuestTicketResponse>>(`/api/ticket/guest/${sessionKey}`)
  return res.data
}

export async function addGuestTicketMessage(
  sessionKey: string,
  content: string
): Promise<ApiResponse<TicketMessage>> {
  const res = await api.post<ApiResponse<TicketMessage>>(
    `/api/ticket/guest/${sessionKey}/message`,
    { content }
  )
  return res.data
}

export async function downloadGuestTicketTranscript(sessionKey: string): Promise<void> {
  const res = await api.get(`/api/ticket/guest/${sessionKey}/transcript`, { responseType: 'blob' })
  const blob = new Blob([res.data], { type: 'text/markdown;charset=utf-8' })
  const url = window.URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `guest-ticket-transcript.md`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  window.URL.revokeObjectURL(url)
}

