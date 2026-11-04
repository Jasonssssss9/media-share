import { Avatar, Box, Link, Paper, Stack, Typography } from '@mui/material'
import { Link as RouterLink } from 'react-router-dom'
import type { Post } from './usePosts'
import { LikeButton } from './LikeButton'

export function PostCard({ post, detail = false }: { post: Post; detail?: boolean }) {
  return (
    <Paper component="article" variant="outlined" sx={{ p: { xs: 2.5, sm: 3 }, overflowWrap: 'anywhere' }}>
      <Stack spacing={2.5}>
        <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center', minWidth: 0 }}>
          <Avatar src={post.author.avatar_url ?? undefined} sx={{ bgcolor: 'primary.main', width: 38, height: 38 }}>
            {Array.from(post.author.display_name)[0]?.toUpperCase()}
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
        {post.content && <Typography sx={{
          whiteSpace: 'pre-wrap',
          ...(detail ? {} : { display: '-webkit-box', WebkitLineClamp: 5, WebkitBoxOrient: 'vertical', overflow: 'hidden' }),
        }}>
          {post.content}
        </Typography>}
        {post.images.length > 0 && (
          <Box sx={{ display: 'grid', gridTemplateColumns: post.images.length === 1 ? '1fr' : 'repeat(2, minmax(0, 1fr))', gap: 1 }}>
            {post.images.map((image, index) => (
              <Box component="a" key={image.id} href={image.url} target="_blank" rel="noreferrer">
                <Box component="img" src={image.url} alt={'Image ' + (index + 1) + ' from ' + post.author.display_name}
                  width={image.width} height={image.height} loading="lazy"
                  sx={{ display: 'block', width: '100%', height: post.images.length === 1 ? 'auto' : { xs: 140, sm: 240 },
                    maxHeight: detail ? 600 : 360, objectFit: post.images.length === 1 ? 'contain' : 'cover', borderRadius: 1 }} />
              </Box>
            ))}
          </Box>
        )}
        <Stack direction="row" spacing={2} sx={{ alignItems: 'center', flexWrap: 'wrap', rowGap: 1 }}>
          <LikeButton post={post} />
          <Link component={RouterLink} to={'/posts/' + post.id} underline="hover">
            {post.comment_count} {post.comment_count === 1 ? 'comment' : 'comments'}
          </Link>
          {!detail && <Link component={RouterLink} to={'/posts/' + post.id} underline="hover">View post</Link>}
        </Stack>
      </Stack>
    </Paper>
  )
}
