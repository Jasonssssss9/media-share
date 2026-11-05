import { useQuery } from '@tanstack/react-query'
import { get } from '../../lib/api'
import { useCurrentUser } from '../auth/useCurrentUser'
import type { PostPage } from '../posts/usePosts'

export function useSearchPosts(keywords: string, author: string, page: number) {
  const current = useCurrentUser()
  const viewer = current.data?.id ?? (current.isError ? 'unknown' : 'guest')
  const params = new URLSearchParams({ q: keywords, author, page: String(page) })
  return useQuery({
    queryKey: ['posts', 'search', keywords, author, page, viewer],
    queryFn: ({ signal }) => get<PostPage>('/search?' + params, signal),
    enabled: (keywords !== '' || author !== '') && !current.isPending,
  })
}
