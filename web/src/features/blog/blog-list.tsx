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
import { Link } from '@tanstack/react-router'
import dayjs from 'dayjs'
import { useTranslation } from 'react-i18next'

import { PublicLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { useStatus } from '@/hooks/use-status'
import { useSeoMeta } from '@/hooks/use-seo-meta'

import { fetchBlogPosts, type BlogPost } from './api'

const LOADING_SKELETON_KEYS = ['s1', 's2', 's3', 's4', 's5', 's6']

function BlogPostCard(props: { post: BlogPost }) {
  const post = props.post

  return (
    <Link to='/blog/$postId' params={{ postId: String(post.id) }}>
      <Card className='hover:border-primary/60 h-full gap-2 overflow-hidden py-4 transition-colors'>
        {post.cover ? (
          <img
            src={post.cover}
            alt={post.title}
            className='-mt-4 aspect-video w-full object-cover'
            loading='lazy'
          />
        ) : null}
        <CardHeader className='space-y-1'>
          <CardTitle className='line-clamp-2 text-base'>{post.title}</CardTitle>
        </CardHeader>
        <CardContent className='space-y-3'>
          {post.summary ? (
            <p className='text-muted-foreground line-clamp-3 text-sm'>
              {post.summary}
            </p>
          ) : null}
          <div className='flex flex-wrap items-center gap-2'>
            {(post.tags ?? []).map((tag) => (
              <Badge key={tag} variant='secondary'>
                {tag}
              </Badge>
            ))}
          </div>
          <div className='text-muted-foreground text-xs'>
            {dayjs(post.created_at).format('YYYY-MM-DD')}
          </div>
        </CardContent>
      </Card>
    </Link>
  )
}

export function BlogListPage() {
  const { t } = useTranslation()
  const { status } = useStatus()

  const blogEnabled = Boolean(status?.blog_enabled)

  const { data, isLoading } = useQuery({
    queryKey: ['blog-posts'],
    queryFn: () => fetchBlogPosts(),
    enabled: blogEnabled,
  })

  // The API returns posts oldest-first (ascending id); newest goes top-left.

  const posts = [...(data?.items ?? [])].reverse()

  // Language-aware SEO for the blog index: the render already chooses the
  // requesting user's language,and the per-language hreflang alternatives are
  // derived from the latest post's localized fields so Google can serve the
  // right language per user.

  useSeoMeta({
    title: t('Blog'),
    description: t('News, updates and articles.'),
    localizedTitles: {
      ...(posts[0]?.localizations?.titles ?? {}),
      en: t('Blog'),
    },
    localizedDescriptions: {
      ...(posts[0]?.localizations?.seo_descriptions ?? {}),
      en: t('News, updates and articles.'),
    },
  })

  if (!blogEnabled) {
    return (
      <PublicLayout>
        <div className='mx-auto max-w-3xl space-y-2'>
          <h1 className='text-2xl font-semibold'>{t('Not Found')}</h1>
          <p className='text-muted-foreground'>
            {t('The blog is not enabled on this site.')}
          </p>
        </div>
      </PublicLayout>
    )
  }
  return (
    <PublicLayout>
      <div className='mx-auto w-full max-w-6xl space-y-6'>
        <div className='space-y-1'>
          <h1 className='text-3xl font-bold'>{t('Blog')}</h1>
          <p className='text-muted-foreground'>
            {t('News, updates and articles.')}
          </p>
        </div>

        {isLoading && (
          <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-3'>
            {LOADING_SKELETON_KEYS.map((key) => (
              <Skeleton key={key} className='h-64 w-full' />
            ))}
          </div>
        )}
        {!isLoading && posts.length === 0 && (
          <p className='text-muted-foreground'>{t('No posts yet.')}</p>
        )}
        {!isLoading && posts.length > 0 && (
          <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-3'>
            {posts.map((post) => (
              <BlogPostCard key={post.id} post={post} />
            ))}
          </div>
        )}
      </div>
    </PublicLayout>
  )
}
