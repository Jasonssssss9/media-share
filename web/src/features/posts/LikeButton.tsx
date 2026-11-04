import { Alert, Button, Stack } from '@mui/material'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { ApiError, del, put } from '../../lib/api'
import { currentUserKey, useCurrentUser } from '../auth/useCurrentUser'
import type { User } from '../auth/useCurrentUser'
import type { Post } from './usePosts'

type LikeState = { like_count: number; liked_by_me: boolean }

export function LikeButton({ post }: { post: Post }) {
  const current = useCurrentUser()
  const client = useQueryClient()
  const userID = current.data?.id
  const toggle = useMutation({
    mutationFn: () => post.liked_by_me
      ? del<LikeState>('/posts/' + post.id + '/like')
      : put<LikeState>('/posts/' + post.id + '/like'),
    onSuccess: () => client.invalidateQueries({ queryKey: ['posts'] }),
    onError: (error) => {
      if (error instanceof ApiError && error.status === 401) {
        client.setQueryData<User | null>(currentUserKey, (user) => user?.id === userID ? null : user)
      }
    },
  })

  return (
    <Stack spacing={1}>
      {current.data ? (
        <Button variant={post.liked_by_me ? 'contained' : 'outlined'} size="small"
          aria-pressed={post.liked_by_me} disabled={toggle.isPending}
          onClick={() => toggle.mutate()}>
          {post.liked_by_me ? 'Unlike' : 'Like'} ({post.like_count})
        </Button>
      ) : (
        <Button component={Link} to="/login" state={{ from: '/posts/' + post.id }}
          variant="outlined" size="small" disabled={current.isPending}>
          Like ({post.like_count})
        </Button>
      )}
      {toggle.isError && <Alert severity="error">{toggle.error.message}</Alert>}
    </Stack>
  )
}
