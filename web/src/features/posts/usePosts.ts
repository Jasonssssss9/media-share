import { useQuery } from '@tanstack/react-query'
import { get } from '../../lib/api'
import { useCurrentUser } from '../auth/useCurrentUser'

export type Author = {
  id: string
  username: string
  display_name: string
  bio: string
  avatar_url: string | null
}

export type PostImage = {
  id: string
  url: string
  width: number
  height: number
}

export type Post = {
  like_count: number
  comment_count: number
  liked_by_me: boolean
  images: PostImage[]
  id: string
  author: Author
  content: string
  created_at: string
  updated_at: string
}

export type PostPage = {
  items: Post[]
  page: number
  page_size: number
  total: number
}

export const postListKey = ['posts', 'list'] as const
export const postKey = (id: string) => ['posts', 'detail', id] as const

export function usePost(id: string) {
  const current = useCurrentUser()
  const viewer = current.data?.id ?? (current.isError ? 'unknown' : 'guest')
  return useQuery({
    queryKey: [...postKey(id), viewer],
    queryFn: ({ signal }) => get<Post>('/posts/' + id, signal),
    enabled: id !== '' && !current.isPending,
  })
}

export function usePostList(page: number, author: string) {
  const current = useCurrentUser()
  const viewer = current.data?.id ?? (current.isError ? 'unknown' : 'guest')
  const params = new URLSearchParams({ page: String(page) })
  if (author) params.set('author', author)
  return useQuery({
    queryKey: [...postListKey, author, page, viewer],
    queryFn: ({ signal }) => get<PostPage>('/posts?' + params, signal),
    enabled: !current.isPending,
  })
}
