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

export type BlogPostLocalizations = {
  titles?: Record<string, string>
  summaries?: Record<string, string>
  contents?: Record<string, string>
  tags_list?: Record<string, string>
  seo_descriptions?: Record<string, string>
}

export type BlogPost = {
  id: number
  title: string
  summary: string
  cover: string
  tags: string[]
  created_at: string
  seo_description?: string
  localizations?: BlogPostLocalizations
}

export type BlogPostDetail = {
  id: number
  title: string
  summary: string
  content: string
  cover_image: string
  tags: string[]
  seo_description: string
  like_count: number
  dislike_count: number
  created_at: string
  updated_at: string
  localizations?: BlogPostLocalizations
}

export type BlogComment = {
  id: number
  post_id: number
  parent_id: number
  user_id: number
  username: string
  avatar: string
  content: string
  like_count: number
  dislike_count: number
  created_at: string
  replies?: BlogComment[]
}

export type BlogPostResponse = {
  post: BlogPostDetail
  comments: BlogComment[]
  /** Keys look like "post:12" or "comment:34", value is 1 | -1 */
  reactions: Record<string, number>
}

type ApiResult<T> = {
  success: boolean
  message?: string
  data: T
}

export async function fetchBlogPosts() {
  const res = await api.get<ApiResult<{ items: BlogPost[]; total: number }>>(
    '/api/blog/posts'
  )
  return res.data.data
}

export type CustomAd = {
  id: string
  image: string
  url: string
}

export type BlogAdsConfig = {
  enabled: boolean
  mode: 'adsense' | 'custom' | 'both' | string
  adsense_client_id: string
  adsense_slot_id: string
  custom_ads: CustomAd[]
}

export async function fetchBlogAds(): Promise<BlogAdsConfig> {
  const res = await api.get<ApiResult<BlogAdsConfig>>('/api/blog/ads')
  return res.data.data
}

export async function trackAdImpression(id: string): Promise<void> {
  try {
    await api.post('/api/blog/ads/impressions', { id })
  } catch {
    /* impressions are best-effort */
  }
}

export async function fetchBlogPost(id: string) {
  const res = await api.get<ApiResult<BlogPostResponse>>(
    `/api/blog/posts/${id}`
  )
  return res.data
}

export async function reactToBlogPost(postId: number, value: 1 | -1) {
  const res = await api.post<ApiResult<{ value: number }>>(
    `/api/blog/posts/${postId}/reactions`,
    { target_type: 'post', value }
  )
  return res.data
}

export async function reactToBlogComment(
  postId: number,
  commentId: number,
  value: 1 | -1
) {
  const res = await api.post<ApiResult<{ value: number }>>(
    `/api/blog/posts/${postId}/comments/${commentId}/reactions`,
    { target_type: 'comment', value }
  )
  return res.data
}

export async function createBlogComment(
  postId: number,
  content: string,
  parentId?: number
) {
  const res = await api.post<ApiResult<BlogComment>>(
    `/api/blog/posts/${postId}/comments`,
    { content, parent_id: parentId ?? 0 }
  )
  return res.data
}

export async function deleteBlogComment(postId: number, commentId: number) {
  const res = await api.delete<ApiResult<null>>(
    `/api/blog/posts/${postId}/comments/${commentId}`
  )
  return res.data
}

/** Builds a reply tree from the flat comment list returned by the API. */
export function buildCommentTree(flat: BlogComment[]): BlogComment[] {
  const byId = new Map<number, BlogComment>()
  const roots: BlogComment[] = []
  for (const comment of flat) {
    byId.set(comment.id, { ...comment, replies: [] })
  }
  for (const comment of byId.values()) {
    if (comment.parent_id && byId.has(comment.parent_id)) {
      byId.get(comment.parent_id)?.replies?.push(comment)
    } else {
      roots.push(comment)
    }
  }
  return roots
}
