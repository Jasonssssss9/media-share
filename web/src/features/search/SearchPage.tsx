import type { FormEvent } from 'react'
import { Alert, Box, Button, Pagination, Paper, Skeleton, Stack, TextField, Typography } from '@mui/material'
import { useSearchParams } from 'react-router-dom'
import { PostCard } from '../posts/PostCard'
import { useSearchPosts } from './useSearchPosts'

export function SearchPage() {
  const [params, setParams] = useSearchParams()
  const keywords = (params.get('q') ?? '').trim()
  const author = (params.get('author') ?? '').trim()
  const value = Number(params.get('page') ?? 1)
  const page = Number.isSafeInteger(value) && value >= 1 ? value : 1
  const hasQuery = keywords !== '' || author !== ''
  const query = useSearchPosts(keywords, author, page)

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    const body = new FormData(event.currentTarget)
    const next = new URLSearchParams()
    const text = String(body.get('q') ?? '').trim()
    const username = String(body.get('author') ?? '').trim()
    if (text) next.set('q', text)
    if (username) next.set('author', username)
    setParams(next)
  }

  function changePage(next: number) {
    setParams((previous) => {
      const updated = new URLSearchParams(previous)
      if (next === 1) updated.delete('page')
      else updated.set('page', String(next))
      return updated
    })
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }

  return (
    <Stack spacing={3}>
      <Box>
        <Typography variant="h1" component="h1">Search posts</Typography>
        <Typography color="text.secondary" sx={{ mt: 1 }}>
          Find posts by their text or author. Uploaded and generated images appear with their posts.
        </Typography>
      </Box>
      <Paper variant="outlined" sx={{ p: { xs: 2.5, sm: 3 } }}>
        <Box component="form" key={JSON.stringify([keywords, author])} onSubmit={submit}>
          <Stack spacing={2}>
            <TextField name="q" label="Keywords" defaultValue={keywords} fullWidth
              helperText="Up to 200 characters. Leave empty to search by author only." />
            <TextField name="author" label="Author username" defaultValue={author} fullWidth
              helperText="Exact username, including letter case. For example, Alice and alice are different users."
              slotProps={{ htmlInput: { autoCapitalize: 'none', spellCheck: false } }} />
            <Button type="submit" variant="contained" sx={{ alignSelf: 'flex-start' }}>Search</Button>
          </Stack>
        </Box>
      </Paper>
      {!hasQuery ? (
        <Typography color="text.secondary">Enter keywords, an author username, or both.</Typography>
      ) : query.isPending ? (
        <Stack spacing={2}>{[1, 2, 3].map((key) => <Skeleton key={key} variant="rounded" height={170} />)}</Stack>
      ) : query.isError ? (
        <Alert severity="warning" action={<Button onClick={() => void query.refetch()}>Retry</Button>}>{query.error.message}</Alert>
      ) : (
        <Stack spacing={3}>
          {query.data.items.length === 0 ? (
            <Paper variant="outlined" sx={{ p: 4, textAlign: 'center' }}>
              <Typography variant="h2">{page === 1 ? 'No matching posts' : 'No posts on this page'}</Typography>
              <Typography color="text.secondary" sx={{ mt: 1 }}>Try different keywords or check the author's exact username.</Typography>
              {page > 1 && <Button sx={{ mt: 2 }} onClick={() => changePage(1)}>Back to first page</Button>}
            </Paper>
          ) : query.data.items.map((post) => <PostCard key={post.id} post={post} />)}
          {(query.data.total > query.data.page_size || page > 1) && (
            <Pagination aria-label="Search pagination" page={page} color="primary"
              count={Math.max(1, Math.ceil(Math.min(query.data.total, 10000) / query.data.page_size))}
              onChange={(_, next) => changePage(next)} sx={{ alignSelf: 'center' }} />
          )}
          {query.data.total > 10000 && <Typography color="text.secondary">Narrow your search to explore more results.</Typography>}
        </Stack>
      )}
    </Stack>
  )
}
