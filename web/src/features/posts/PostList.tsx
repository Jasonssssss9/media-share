import { Alert, Button, Pagination, Paper, Skeleton, Stack, Typography } from '@mui/material'
import { useSearchParams } from 'react-router-dom'
import { PostCard } from './PostCard'
import { usePostList } from './usePosts'

export function PostList({ author = '' }: { author?: string }) {
  const [params, setParams] = useSearchParams()
  const value = Number(params.get('page') ?? 1)
  const page = Number.isSafeInteger(value) && value >= 1 ? value : 1
  const query = usePostList(page, author)

  function changePage(next: number) {
    setParams((previous) => {
      const updated = new URLSearchParams(previous)
      if (next === 1) updated.delete('page')
      else updated.set('page', String(next))
      return updated
    })
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }

  if (query.isPending) {
    return <Stack spacing={2}>{[1, 2, 3].map((key) => <Skeleton key={key} variant="rounded" height={170} />)}</Stack>
  }
  if (query.isError) {
    return <Alert severity="warning" action={<Button onClick={() => void query.refetch()}>Retry</Button>}>{query.error.message}</Alert>
  }

  return (
    <Stack spacing={3}>
      {query.data.items.length === 0 ? (
        <Paper variant="outlined" sx={{ p: 5, textAlign: 'center' }}>
          <Typography variant="h2">{page > 1 ? 'No posts on this page' : 'No posts yet'}</Typography>
          <Typography color="text.secondary" sx={{ mt: 1 }}>
            {page > 1 ? 'Try an earlier page.' : author ? 'Posts from this user will appear here.' : 'Be the first to share something.'}
          </Typography>
          {page > 1 && <Button sx={{ mt: 2 }} onClick={() => changePage(1)}>Back to first page</Button>}
        </Paper>
      ) : query.data.items.map((post) => <PostCard key={post.id} post={post} />)}
      {(query.data.total > query.data.page_size || page > 1) && (
        <Pagination
          aria-label="Posts pagination"
          count={Math.max(1, Math.ceil(query.data.total / query.data.page_size))}
          page={page}
          onChange={(_, next) => changePage(next)}
          color="primary"
          sx={{ alignSelf: 'center' }}
        />
      )}
    </Stack>
  )
}
