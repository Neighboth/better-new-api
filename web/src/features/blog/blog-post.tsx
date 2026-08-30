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
import { ArrowLeft } from 'lucide-react'
import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'

import { PublicLayout } from '@/components/layout'
import { RichContent } from '@/components/rich-content'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { useStatus } from '@/hooks/use-status'

import { fetchBlogPost } from './api'
import { BlogAds } from './components/blog-ads-block'
import { BlogComments } from './components/comments'
import { ReactionButtons } from './components/reaction-buttons'
import { useSeoMeta } from '@/hooks/use-seo-meta'

type BlogPostPageProps = {
  postId: string
}

export function BlogPostPage(props: BlogPostPageProps) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const blogEnabled = Boolean(status?.blog_enabled)

  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['blog-post', props.postId],
    queryFn: () => fetchBlogPost(props.postId),
    enabled: blogEnabled,
  })

  const post = data?.success ? data.data.post : null
  const reactions = data?.success ? data.data.reactions : {}

  useEffect(() => {
    if (!post?.title) return
    const previousTitle = document.title
    document.title = post.title
    return () => {
      document.title = previousTitle
    }
  }, [post?.title])

  // Language-aware SEO： Google indexes this page in the language of the
  // requesting user (the backend already localizes the content by header）。
  useSeoMeta({
    title: post?.title ?? '',
    description: post?.seo_description,
    localizedTitles: post?.localizations?.titles,
    localizedDescriptions: post?.localizations?.seo_descriptions,
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

  if (isLoading) {
    return (
      <PublicLayout>
        <div className='mx-auto max-w-3xl space-y-4'>
          <Skeleton className='h-8 w-2/3' />
          <Skeleton className='h-64 w-full' />
          <Skeleton className='h-40 w-full' />
        </div>
      </PublicLayout>
    )
  }

  if (error || !post) {
    return (
      <PublicLayout>
        <div className='mx-auto max-w-3xl space-y-2'>
          <h1 className='text-2xl font-semibold'>{t('Not Found')}</h1>
          <p className='text-muted-foreground'>
            {t('This post does not exist or is not published.')}
          </p>
          <Link
            to='/blog'
            className='hover:text-primary inline-flex items-center gap-1 text-sm underline underline-offset-4'
          >
            <ArrowLeft className='h-4 w-4' />
            {t('Back to blog')}
          </Link>
        </div>
      </PublicLayout>
    )
  }

  return (
    <PublicLayout>
      <article className='mx-auto w-full max-w-3xl space-y-6'>
        <BlogAds position='top' />
        <Link
          to='/blog'
          className='text-muted-foreground hover:text-primary inline-flex items-center gap-1 text-sm'
        >
          <ArrowLeft className='h-4 w-4' />
          {t('Back to blog')}
        </Link>

        <header className='space-y-3'>
          <h1 className='text-3xl font-bold'>{post.title}</h1>
          <div className='text-muted-foreground flex flex-wrap items-center gap-3 text-sm'>
            <span>{dayjs(post.created_at).format('YYYY-MM-DD')}</span>
            {(post.tags ?? []).map((tag) => (
              <Badge key={tag} variant='secondary'>
                {tag}
              </Badge>
            ))}
          </div>
        </header>
        {post.cover_image ? (
          <img
            src={post.cover_image}
            alt={post.title}
            className='w-full rounded-lg border object-cover'
          />
        ) : null}

        <RichContent
          content={post.content}
          mode='markdown'
          className='max-w-none'
        />

        <div className='flex items-center gap-2 border-y py-3'>
          <ReactionButtons
            targetType='post'
            postId={post.id}
            targetId={post.id}
            likeCount={post.like_count}
            dislikeCount={post.dislike_count}
            myReaction={reactions[`post:${post.id}`] ?? 0}
            onChanged={() => refetch()}
          />
        </div>

        {data?.success && (
          <BlogComments
            postId={post.id}
            comments={data.data.comments}
            reactions={reactions}
            onChanged={() => refetch()}
          />
        )}
        <BlogAds position='bottom' />
      </article>
    </PublicLayout>
  )
}
