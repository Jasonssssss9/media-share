import { useState } from 'react'
import type { FormEvent } from 'react'
import { Alert, Avatar, Box, Button, Dialog, DialogActions, DialogContent, DialogTitle, Link, Paper, Stack, TextField, Typography } from '@mui/material'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Link as RouterLink } from 'react-router-dom'
import { ApiError, del, patch } from '../../lib/api'
import { currentUserKey } from '../auth/useCurrentUser'
import type { User } from '../auth/useCurrentUser'
import type { Comment } from './useComments'

type Props = {
  comment: Comment
  userID?: string
  postOwnerID: string
  onSaved: () => Promise<unknown>
  onDeleted: () => Promise<unknown>
}

export function CommentItem({ comment, userID, postOwnerID, onSaved, onDeleted }: Props) {
  const [editing, setEditing] = useState(false)
  const [content, setContent] = useState(comment.content)
  const [confirming, setConfirming] = useState(false)
  const client = useQueryClient()
  const own = userID === comment.author.id
  const canDelete = own || userID === postOwnerID
  const count = Array.from(content.trim()).length

  function sessionError(error: Error) {
    if (error instanceof ApiError && error.status === 401) {
      client.setQueryData<User | null>(currentUserKey, (user) => user?.id === userID ? null : user)
    }
  }
  const update = useMutation({
    mutationFn: () => patch<Comment>('/comments/' + comment.id, { content }),
    onSuccess: async () => { setEditing(false); await onSaved() },
    onError: sessionError,
  })
  const deletion = useMutation({
    mutationFn: () => del('/comments/' + comment.id),
    onSuccess: onDeleted,
    onError: sessionError,
  })
  const busy = update.isPending || deletion.isPending

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (count >= 1 && count <= 500 && !busy) update.mutate()
  }

  return (
    <Paper component="article" variant="outlined" sx={{ p: 2.5, overflowWrap: 'anywhere' }}>
      <Stack spacing={2}>
        <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center' }}>
          <Avatar src={comment.author.avatar_url ?? undefined} sx={{ width: 32, height: 32, bgcolor: 'primary.main' }}>
            {Array.from(comment.author.display_name)[0]?.toUpperCase()}
          </Avatar>
          <Box>
            <Link component={RouterLink} to={'/users/' + comment.author.username} underline="hover">
              {comment.author.display_name}
            </Link>
            <Typography variant="body2" color="text.secondary">
              @{comment.author.username} · <time dateTime={comment.created_at}>{new Date(comment.created_at).toLocaleString()}</time>
              {comment.updated_at !== comment.created_at && ' · Edited'}
            </Typography>
          </Box>
        </Stack>
        {update.isError && <Alert severity="error">{update.error.message}</Alert>}
        {deletion.isError && <Alert severity="error">{deletion.error.message}</Alert>}
        {editing && own ? (
          <Box component="form" onSubmit={submit}>
            <Stack spacing={1.5}>
              <TextField label="Edit comment" value={content} multiline minRows={2} fullWidth disabled={busy}
                onChange={(event) => setContent(event.target.value)} error={count > 500}
                helperText={count + ' / 500 characters.'} />
              <Stack direction="row" spacing={1}>
                <Button type="submit" variant="contained" size="small" disabled={busy || count < 1 || count > 500}>
                  {update.isPending ? 'Saving...' : 'Save comment'}
                </Button>
                <Button size="small" disabled={busy} onClick={() => setEditing(false)}>Cancel edit</Button>
              </Stack>
            </Stack>
          </Box>
        ) : (
          <>
            <Typography sx={{ whiteSpace: 'pre-wrap' }}>{comment.content}</Typography>
            {(own || canDelete) && (
              <Stack direction="row" spacing={1}>
                {own && <Button size="small" disabled={busy} onClick={() => {
                  setContent(comment.content); update.reset(); setEditing(true)
                }}>Edit comment</Button>}
                {canDelete && <Button size="small" color="error" disabled={busy} onClick={() => setConfirming(true)}>Delete comment</Button>}
              </Stack>
            )}
          </>
        )}
      </Stack>
      <Dialog open={confirming} onClose={() => { if (!busy) setConfirming(false) }}>
        <DialogTitle>Delete this comment?</DialogTitle>
        <DialogContent><Typography>The comment will be removed from this post.</Typography></DialogContent>
        <DialogActions>
          <Button disabled={busy} onClick={() => setConfirming(false)}>Cancel</Button>
          <Button color="error" disabled={busy} onClick={() => { setConfirming(false); deletion.mutate() }}>Delete comment</Button>
        </DialogActions>
      </Dialog>
    </Paper>
  )
}
