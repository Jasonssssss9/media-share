import { useState } from 'react'
import type { FormEvent } from 'react'
import { Alert, Box, Button, Paper, Skeleton, Stack, TextField, Typography } from '@mui/material'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { ApiError, patch, post as createPost } from '../../lib/api'
import { currentUserKey, useCurrentUser } from '../auth/useCurrentUser'
import { postKey, postListKey, usePost } from './usePosts'
import type { Post } from './usePosts'

export function PostEditorPage() {
  const { id = '' } = useParams()
  const query = usePost(id)
  const current = useCurrentUser()
  if (id && query.isPending) return <Skeleton variant="rounded" height={320} />
  if (id && query.isError) {
    return <Alert severity="warning" action={<Button onClick={() => void query.refetch()}>Retry</Button>}>{query.error.message}</Alert>
  }
  if (id && query.data?.author.id !== current.data?.id) {
    return <Alert severity="warning">You can only edit your own posts.</Alert>
  }
  return <PostForm key={id || 'new'} existing={id ? query.data : undefined} />
}

function PostForm({ existing }: { existing?: Post }) {
  const [content, setContent] = useState(existing?.content ?? '')
  const [originalUpdatedAt] = useState(existing?.updated_at)
  const count = Array.from(content).length
  const valid = content.trim() !== '' && count <= 2000
  const navigate = useNavigate()
  const client = useQueryClient()
  const save = useMutation({
    mutationFn: () => existing
      ? patch<Post>('/posts/' + existing.id, { content, updated_at: originalUpdatedAt })
      : createPost<Post>('/posts', { content }),
    onSuccess: (saved) => {
      client.setQueryData(postKey(saved.id), saved)
      void client.invalidateQueries({ queryKey: postListKey })
      navigate('/posts/' + saved.id, { replace: true })
    },
    onError: (error) => {
      if (error instanceof ApiError && error.status === 401) client.setQueryData(currentUserKey, null)
    },
  })

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (valid && !save.isPending) save.mutate()
  }

  return (
    <Paper variant="outlined" sx={{ p: { xs: 2.5, sm: 4 } }}>
      <Stack spacing={3}>
        <Box>
          <Typography variant="h1" component="h1">{existing ? 'Edit post' : 'New post'}</Typography>
          <Typography color="text.secondary" sx={{ mt: 1 }}>Share a thought, an update, or a short story.</Typography>
        </Box>
        {save.isError && (
          <Alert severity="error" action={save.error instanceof ApiError && save.error.status === 409
            ? <Button onClick={() => window.location.reload()}>Reload post</Button> : undefined}>
            {save.error.message}
          </Alert>
        )}
        <Box component="form" onSubmit={submit}>
          <Stack spacing={3}>
            <TextField
              label="Post content"
              value={content}
              onChange={(event) => setContent(event.target.value)}
              multiline
              minRows={7}
              fullWidth
              required
              disabled={save.isPending}
              error={count > 2000}
              helperText={count + ' / 2,000 characters'}
            />
            <Stack direction="row" spacing={1}>
              <Button type="submit" variant="contained" disabled={!valid || save.isPending}>
                {save.isPending ? 'Saving...' : existing ? 'Save changes' : 'Publish post'}
              </Button>
              <Button component={Link} to={existing ? '/posts/' + existing.id : '/'} disabled={save.isPending}>Cancel</Button>
            </Stack>
          </Stack>
        </Box>
      </Stack>
    </Paper>
  )
}
