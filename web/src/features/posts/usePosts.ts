import { useQuery } from '@tanstack/react-query'
import { get } from '../../lib/api'

export type Author = {
  id: string
  username: string
  display_name: string
  bio: string
  avatar_url: string | null
}

export type Post = {
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
  return useQuery({
    queryKey: postKey(id),
    queryFn: ({ signal }) => get<Post>('/posts/' + id, signal),
    enabled: id !== '',
  })
}

export function usePostList(page: number, author: string) {
  const params = new URLSearchParams({ page: String(page) })
  if (author) params.set('author', author)
  return useQuery({
    queryKey: [...postListKey, author, page],
    queryFn: ({ signal }) => get<PostPage>('/posts?' + params, signal),
  })
}
