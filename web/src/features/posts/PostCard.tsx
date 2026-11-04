import { Avatar, Box, Link, Paper, Stack, Typography } from '@mui/material'
import { Link as RouterLink } from 'react-router-dom'
import type { Post } from './usePosts'

export function PostCard({ post, detail = false }: { post: Post; detail?: boolean }) {
  return (
    <Paper component="article" variant="outlined" sx={{ p: { xs: 2.5, sm: 3 }, overflowWrap: 'anywhere' }}>
      <Stack spacing={2.5}>
        <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center', minWidth: 0 }}>
          <Avatar src={post.author.avatar_url ?? undefined} sx={{ bgcolor: 'primary.main', width: 38, height: 38 }}>
            {post.author.display_name.charAt(0).toUpperCase()}
          </Avatar>
          <Box sx={{ minWidth: 0 }}>
            <Link component={RouterLink} to={'/users/' + post.author.username} underline="hover" sx={{ fontWeight: 600 }}>
              {post.author.display_name}
            </Link>
            <Typography variant="body2" color="text.secondary">
              @{post.author.username} · <time dateTime={post.created_at}>{new Date(post.created_at).toLocaleString()}</time>
              {post.updated_at !== post.created_at && ' · Edited'}
            </Typography>
          </Box>
        </Stack>
        <Typography sx={{
          whiteSpace: 'pre-wrap',
          ...(detail ? {} : { display: '-webkit-box', WebkitLineClamp: 5, WebkitBoxOrient: 'vertical', overflow: 'hidden' }),
        }}>
          {post.content}
        </Typography>
        {!detail && <Link component={RouterLink} to={'/posts/' + post.id} underline="hover">View post</Link>}
      </Stack>
    </Paper>
  )
}
