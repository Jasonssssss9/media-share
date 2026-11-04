import { useState } from 'react'
import { Alert, Box, Button, Dialog, DialogActions, DialogContent, DialogTitle, Skeleton, Stack, Typography } from '@mui/material'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { ApiError, del } from '../../lib/api'
import { currentUserKey, useCurrentUser } from '../auth/useCurrentUser'
import { PostCard } from './PostCard'
import { CommentsSection } from './CommentsSection'
import { commentsKey } from './useComments'
import { postKey, postListKey, usePost } from './usePosts'

export function PostDetailPage() {
  const { id = '' } = useParams()
  const query = usePost(id)
  const current = useCurrentUser()
  const client = useQueryClient()
  const navigate = useNavigate()
  const [confirming, setConfirming] = useState(false)
  const deletion = useMutation({
    mutationFn: () => del('/posts/' + id),
    onSuccess: () => {
      client.removeQueries({ queryKey: postKey(id) })
      client.removeQueries({ queryKey: commentsKey(id) })
      void client.invalidateQueries({ queryKey: postListKey })
      navigate('/', { replace: true })
    },
    onError: (error) => {
      if (error instanceof ApiError && error.status === 401) client.setQueryData(currentUserKey, null)
    },
  })

  if (query.isPending) return <Skeleton variant="rounded" height={260} />
  if (query.isError) {
    return (
      <Stack spacing={2}>
        <Alert severity={query.error instanceof ApiError && query.error.status === 404 ? 'info' : 'warning'}
          action={<Button onClick={() => void query.refetch()}>Retry</Button>}>
          {query.error.message}
        </Alert>
        <Button component={Link} to="/" sx={{ alignSelf: 'flex-start' }}>Back to feed</Button>
      </Stack>
    )
  }
  const post = query.data
  const ownPost = current.data?.id === post.author.id

  return (
    <Stack spacing={3}>
      <Box sx={{ display: 'flex', justifyContent: 'space-between', gap: 1 }}>
        <Button component={Link} to="/">Back to feed</Button>
        {ownPost && (
          <Box>
            <Button component={Link} to={'/posts/' + id + '/edit'}>Edit</Button>
            <Button color="error" onClick={() => setConfirming(true)}>Delete</Button>
          </Box>
        )}
      </Box>
      {deletion.isError && <Alert severity="error">{deletion.error.message}</Alert>}
      <Typography variant="h1" component="h1" sx={{ fontSize: '1.5rem', overflowWrap: 'anywhere' }}>Post by {post.author.display_name}</Typography>
      <PostCard post={post} detail />
      <CommentsSection key={post.id} post={post} />
      <Dialog open={confirming} onClose={() => { if (!deletion.isPending) setConfirming(false) }}>
        <DialogTitle>Delete this post?</DialogTitle>
        <DialogContent><Typography>The post will be removed from the feed and your profile.</Typography></DialogContent>
        <DialogActions>
          <Button disabled={deletion.isPending} onClick={() => setConfirming(false)}>Cancel</Button>
          <Button color="error" disabled={deletion.isPending} onClick={() => { setConfirming(false); deletion.mutate() }}>Delete post</Button>
        </DialogActions>
      </Dialog>
    </Stack>
  )
}
