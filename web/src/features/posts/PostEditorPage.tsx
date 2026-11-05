import { useEffect, useRef, useState } from 'react'
import type { ChangeEvent, FormEvent } from 'react'
import { Alert, Box, Button, Paper, Skeleton, Stack, TextField, Typography } from '@mui/material'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { ApiError, del, patch, post as createPost } from '../../lib/api'
import { currentUserKey, useCurrentUser } from '../auth/useCurrentUser'
import { postKey, usePost } from './usePosts'
import { ImageGenerator } from './ImageGenerator'
import type { Post, PostImage } from './usePosts'

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
  const [images, setImages] = useState<PostImage[]>(existing?.images ?? [])
  const [working, setWorking] = useState<'upload' | 'generation' | 'remove' | null>(null)
  const [imageError, setImageError] = useState('')
  const pending = useRef(new Set<string>())
  const active = useRef(true)
  const count = Array.from(content).length
  const valid = (content.trim() !== '' || images.length > 0) && count <= 2000
  const navigate = useNavigate()
  const client = useQueryClient()

  useEffect(() => {
    active.current = true
    const uploads = pending.current
    return () => {
      active.current = false
      for (const id of uploads) {
        discardUpload(id)
      }
    }
  }, [])

  const save = useMutation({
    mutationFn: async () => {
      const uploads = [...pending.current]
      pending.current.clear()
      const body = { content, media_ids: images.map((image) => image.id) }
      try {
        return await (existing
          ? patch<Post>('/posts/' + existing.id, { ...body, updated_at: originalUpdatedAt })
          : createPost<Post>('/posts', body))
      } catch (error) {
        for (const id of uploads) {
          if (active.current) pending.current.add(id)
          else discardUpload(id)
        }
        throw error
      }
    },
    onSuccess: (saved) => {
      pending.current.clear()
      client.setQueryData([...postKey(saved.id), saved.author.id], saved)
      void client.invalidateQueries({ queryKey: ['posts'] })
      if (active.current) navigate('/posts/' + saved.id, { replace: true })
    },
    onError: (error) => {
      if (error instanceof ApiError && error.status === 401) client.setQueryData(currentUserKey, null)
    },
  })
  const busy = working !== null || save.isPending

  async function addImages(event: ChangeEvent<HTMLInputElement>) {
    const files = Array.from(event.target.files ?? [])
    event.target.value = ''
    if (files.length === 0) return
    if (images.length + files.length > 4) { setImageError('Choose no more than four images per post.'); return }
    if (files.some((file) => file.size > 5 * 1024 * 1024)) { setImageError('Each image must be 5 MiB or smaller.'); return }
    setWorking('upload')
    setImageError('')
    try {
      for (const file of files) {
        const form = new FormData()
        form.append('file', file)
        const image = await createPost<PostImage>('/media', form)
        if (!active.current) {
          await del('/media/' + image.id)
          break
        }
        pending.current.add(image.id)
        setImages((previous) => [...previous, image])
      }
    } catch (error) {
      if (error instanceof ApiError && error.status === 401) client.setQueryData(currentUserKey, null)
      setImageError(error instanceof Error ? error.message : 'Unable to upload this image.')
    } finally { setWorking(null) }
  }

  async function generateImage(prompt: string) {
    setWorking('generation')
    setImageError('')
    try {
      const image = await createPost<PostImage>('/media/generation', { prompt })
      if (!active.current) {
        discardUpload(image.id)
        return
      }
      pending.current.add(image.id)
      setImages((previous) => [...previous, image])
    } catch (error) {
      if (!active.current) return
      if (error instanceof ApiError && error.status === 401) client.setQueryData(currentUserKey, null)
      setImageError(error instanceof Error ? error.message : 'Unable to generate this image.')
    } finally {
      if (active.current) setWorking(null)
    }
  }

  async function removeImage(image: PostImage) {
    setImageError('')
    if (!pending.current.has(image.id)) {
      setImages((previous) => previous.filter((item) => item.id !== image.id))
      return
    }
    setWorking('remove')
    try {
      await del('/media/' + image.id)
      pending.current.delete(image.id)
      setImages((previous) => previous.filter((item) => item.id !== image.id))
    } catch (error) {
      setImageError(error instanceof Error ? error.message : 'Unable to discard this image.')
    } finally { setWorking(null) }
  }

  function moveImage(index: number, step: number) {
    const next = [...images]
    ;[next[index], next[index + step]] = [next[index + step], next[index]]
    setImages(next)
  }

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (valid && !busy) save.mutate()
  }

  return (
    <Paper variant="outlined" sx={{ p: { xs: 2.5, sm: 4 } }}>
      <Stack spacing={3}>
        <Box>
          <Typography variant="h1" component="h1">{existing ? 'Edit post' : 'New post'}</Typography>
          <Typography color="text.secondary" sx={{ mt: 1 }}>Share a thought, a photo, or an everyday moment.</Typography>
        </Box>
        {save.isError && (
          <Alert severity="error" action={existing && save.error instanceof ApiError && save.error.status === 409
            ? <Button onClick={() => window.location.reload()}>Reload post</Button> : undefined}>
            {save.error.message}
          </Alert>
        )}
        {imageError && <Alert severity="error">{imageError}</Alert>}
        <Box component="form" onSubmit={submit}>
          <Stack spacing={3}>
            <TextField
              label="Post content"
              value={content}
              onChange={(event) => setContent(event.target.value)}
              multiline minRows={5} fullWidth disabled={save.isPending}
              error={count > 2000}
              helperText={count + ' / 2,000 characters. Add text, images, or both.'}
            />
            <Box>
              <Button component="label" variant="outlined" disabled={busy || images.length === 4}>
                {working === 'upload' ? 'Uploading...' : 'Add images'}
                <input type="file" hidden multiple accept="image/jpeg,image/png,image/webp" onChange={(event) => void addImages(event)} disabled={busy || images.length === 4} />
              </Button>
              <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
                Up to four JPEG, PNG, or WebP images. 5 MiB and 25 megapixels per image.
              </Typography>
            </Box>
            <ImageGenerator disabled={busy || images.length === 4} generating={working === 'generation'} onGenerate={generateImage} />
            {images.length > 0 && (
              <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: 'repeat(2, minmax(0, 1fr))' }, gap: 2 }}>
                {images.map((image, index) => (
                  <Box key={image.id}>
                    <Box component="img" src={image.url} alt={'Selected image ' + (index + 1)}
                      sx={{ display: 'block', width: '100%', height: 180, objectFit: 'cover', borderRadius: 1 }} />
                    <Stack direction="row" spacing={0.5} sx={{ mt: 1 }}>
                      <Button size="small" disabled={busy || index === 0} aria-label={'Move image ' + (index + 1) + ' earlier'} onClick={() => moveImage(index, -1)}>Earlier</Button>
                      <Button size="small" disabled={busy || index === images.length - 1} aria-label={'Move image ' + (index + 1) + ' later'} onClick={() => moveImage(index, 1)}>Later</Button>
                      <Button size="small" color="error" disabled={busy} aria-label={'Remove image ' + (index + 1)} onClick={() => void removeImage(image)}>Remove</Button>
                    </Stack>
                  </Box>
                ))}
              </Box>
            )}
            <Stack direction="row" spacing={1}>
              <Button type="submit" variant="contained" disabled={!valid || busy}>
                {save.isPending ? 'Saving...' : existing ? 'Save changes' : 'Publish post'}
              </Button>
              <Button component={Link} to={existing ? '/posts/' + existing.id : '/'} disabled={busy}>Cancel</Button>
            </Stack>
          </Stack>
        </Box>
      </Stack>
    </Paper>
  )
}

function discardUpload(id: string) {
  void del('/media/' + id).catch((error) => console.warn('Unable to discard an unused upload.', error))
}
