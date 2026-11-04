import { Alert, Avatar, Box, Button, Paper, Skeleton, Stack, Typography } from '@mui/material'
import { useQuery } from '@tanstack/react-query'
import { Link, useParams } from 'react-router-dom'
import { get } from '../../lib/api'
import { useCurrentUser } from '../auth/useCurrentUser'
import { PostList } from './PostList'
import type { Author } from './usePosts'

export function AuthorPage() {
  const { username = '' } = useParams()
  const current = useCurrentUser()
  const query = useQuery({
    queryKey: ['authors', username],
    queryFn: ({ signal }) => get<Author>('/users/' + encodeURIComponent(username), signal),
  })
  if (query.isPending) return <Skeleton variant="rounded" height={200} />
  if (query.isError) {
    return <Alert severity="warning" action={<Button onClick={() => void query.refetch()}>Retry</Button>}>{query.error.message}</Alert>
  }
  const author = query.data
  return (
    <Stack spacing={4}>
      <Paper variant="outlined" sx={{ p: 3, overflowWrap: 'anywhere' }}>
        <Stack direction="row" spacing={2} sx={{ alignItems: 'center', minWidth: 0 }}>
          <Avatar src={author.avatar_url ?? undefined} sx={{ bgcolor: 'primary.main', width: 48, height: 48 }}>
            {Array.from(author.display_name)[0]?.toUpperCase()}
          </Avatar>
          <Box sx={{ minWidth: 0 }}>
            <Typography variant="h1" component="h1">{author.display_name}</Typography>
            <Typography color="text.secondary">@{author.username}</Typography>
          </Box>
        </Stack>
        {author.bio && <Typography sx={{ mt: 2, whiteSpace: 'pre-wrap' }}>{author.bio}</Typography>}
        {current.data?.id === author.id && (
          <Stack direction="row" spacing={1} sx={{ mt: 2 }}>
            <Button component={Link} to="/account">Edit profile</Button>
            <Button component={Link} to="/posts/new">New post</Button>
          </Stack>
        )}
      </Paper>
      <Typography variant="h2" component="h2">Posts</Typography>
      <PostList author={username} />
    </Stack>
  )
}
