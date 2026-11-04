import { Box, Button, Stack, Typography } from '@mui/material'
import { Link } from 'react-router-dom'
import { useCurrentUser } from '../auth/useCurrentUser'
import { PostList } from '../posts/PostList'

export function FeedPage() {
  const current = useCurrentUser()
  return (
    <Stack spacing={4}>
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 2 }}>
        <Box>
          <Typography variant="h1" component="h1">Latest posts</Typography>
          <Typography color="text.secondary" sx={{ mt: 1 }}>Thoughts and everyday moments.</Typography>
        </Box>
        <Button component={Link} to="/posts/new" variant="contained" disabled={current.isPending} sx={{ flexShrink: 0 }}>
          {current.data ? 'New post' : 'Sign in to post'}
        </Button>
      </Box>
      <PostList />
    </Stack>
  )
}
