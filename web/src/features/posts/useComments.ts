import { useQuery } from '@tanstack/react-query'
import { get } from '../../lib/api'
import type { Author } from './usePosts'

export type Comment = {
  id: string
  post_id: string
  author: Author
  content: string
  created_at: string
  updated_at: string
}

export type CommentPage = {
  items: Comment[]
  page: number
  page_size: number
  total: number
}

export const commentsKey = (postID: string) => ['comments', postID] as const

export function useComments(postID: string, page: number) {
  return useQuery({
    queryKey: [...commentsKey(postID), page],
    queryFn: ({ signal }) => get<CommentPage>('/posts/' + postID + '/comments?page=' + page, signal),
  })
}
