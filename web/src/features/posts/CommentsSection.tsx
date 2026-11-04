import { useState } from 'react'
import type { FormEvent } from 'react'
import { Alert, Box, Button, Link, Pagination, Skeleton, Stack, TextField, Typography } from '@mui/material'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Link as RouterLink } from 'react-router-dom'
import { ApiError, post as createResource } from '../../lib/api'
import { currentUserKey, useCurrentUser } from '../auth/useCurrentUser'
import type { User } from '../auth/useCurrentUser'
import { CommentItem } from './CommentItem'
import { commentsKey, useComments } from './useComments'
import type { Comment } from './useComments'
import type { Post } from './usePosts'

export function CommentsSection({ post }: { post: Post }) {
  const [page, setPage] = useState(1)
  const [content, setContent] = useState('')
  const current = useCurrentUser()
  const client = useQueryClient()
  const query = useComments(post.id, page)
  const count = Array.from(content.trim()).length
  const userID = current.data?.id

  function refresh() {
    return Promise.all([
      client.invalidateQueries({ queryKey: commentsKey(post.id) }),
      client.invalidateQueries({ queryKey: ['posts'] }),
    ])
  }
  const create = useMutation({
    mutationFn: () => createResource<Comment>('/posts/' + post.id + '/comments', { content }),
    onSuccess: async () => { setContent(''); setPage(1); await refresh() },
    onError: (error) => {
      if (error instanceof ApiError && error.status === 401) {
        client.setQueryData<User | null>(currentUserKey, (user) => user?.id === userID ? null : user)
      }
    },
  })

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (count >= 1 && count <= 500 && !create.isPending) create.mutate()
  }

  return (
    <Stack component="section" id="comments" spacing={2.5}>
      <Box>
        <Typography variant="h2" component="h2">Comments ({post.comment_count})</Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>Newest comments first.</Typography>
      </Box>
      {current.data ? (
        <Box component="form" onSubmit={submit}>
          <Stack spacing={1.5}>
            {create.isError && <Alert severity="error">{create.error.message}</Alert>}
            <TextField label="Write a comment" value={content} multiline minRows={2} fullWidth disabled={create.isPending}
              onChange={(event) => setContent(event.target.value)} error={count > 500}
              helperText={count + ' / 500 characters.'} />
            <Button type="submit" variant="contained" disabled={create.isPending || count < 1 || count > 500} sx={{ alignSelf: 'flex-start' }}>
              {create.isPending ? 'Posting...' : 'Post comment'}
            </Button>
          </Stack>
        </Box>
      ) : (
        <Typography>
          <Link component={RouterLink} to="/login" state={{ from: '/posts/' + post.id }}>Sign in</Link> to leave a comment.
        </Typography>
      )}
      {query.isPending ? <Skeleton variant="rounded" height={140} /> : query.isError ? (
        <Alert severity="warning" action={<Button onClick={() => void query.refetch()}>Retry</Button>}>{query.error.message}</Alert>
      ) : (
        <>
          {query.data.items.length === 0 ? (
            <Stack spacing={1} sx={{ py: 2 }}>
              <Typography color="text.secondary">{page === 1 ? 'No comments yet.' : 'No comments on this page.'}</Typography>
              {page > 1 && <Button sx={{ alignSelf: 'flex-start' }} onClick={() => setPage(1)}>Back to first page</Button>}
            </Stack>
          ) : query.data.items.map((comment) => (
            <CommentItem key={comment.id} comment={comment} userID={userID} postOwnerID={post.author.id}
              onSaved={refresh} onDeleted={() => {
                if (query.data.items.length === 1 && page > 1) setPage(page - 1)
                return refresh()
              }} />
          ))}
          {(query.data.total > query.data.page_size || page > 1) && (
            <Pagination aria-label="Comments pagination" page={page} color="primary"
              count={Math.max(1, Math.ceil(query.data.total / query.data.page_size))}
              onChange={(_, next) => setPage(next)} sx={{ alignSelf: 'center' }} />
          )}
        </>
      )}
    </Stack>
  )
}
