import { useQuery } from '@tanstack/react-query'
import { ApiError, get } from '../../lib/api'

export type User = {
  id: string
  email: string
  username: string
  display_name: string
  bio: string
  avatar_url: string | null
  created_at: string
}

export const currentUserKey = ['current-user'] as const

export function useCurrentUser() {
  return useQuery({
    queryKey: currentUserKey,
    queryFn: async ({ signal }) => {
      try {
        return await get<User>('/auth/me', signal)
      } catch (error) {
        if (error instanceof ApiError && error.status === 401) return null
        throw error
      }
    },
    refetchOnWindowFocus: true,
  })
}
