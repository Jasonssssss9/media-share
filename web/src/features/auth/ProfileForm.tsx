import { useEffect, useRef, useState } from 'react'
import type { ChangeEvent, FormEvent } from 'react'
import { Alert, Avatar, Box, Button, Stack, TextField, Typography } from '@mui/material'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { ApiError, del, patch, post } from '../../lib/api'
import { currentUserKey } from './useCurrentUser'
import type { User } from './useCurrentUser'

type AvatarUpload = { id: string; url: string }

export function ProfileForm({ user }: { user: User }) {
  const [name, setName] = useState(user.display_name)
  const [bio, setBio] = useState(user.bio)
  const [avatar, setAvatar] = useState<AvatarUpload | null | undefined>(undefined)
  const [working, setWorking] = useState(false)
  const [error, setError] = useState('')
  const [saved, setSaved] = useState(false)
  const pending = useRef<string | null>(null)
  const active = useRef(true)
  const client = useQueryClient()
  const nameCount = Array.from(name.trim()).length
  const bioCount = Array.from(bio.trim()).length
  const dirty = name !== user.display_name || bio !== user.bio || avatar !== undefined
  const valid = nameCount >= 1 && nameCount <= 60 && bioCount <= 280
  const avatarURL = avatar === undefined ? user.avatar_url : avatar?.url ?? null

  useEffect(() => {
    active.current = true
    return () => {
      active.current = false
      if (pending.current) discardAvatar(pending.current)
    }
  }, [])

  function showError(cause: unknown, fallback: string) {
    if (!active.current) return
    if (cause instanceof ApiError && cause.status === 401) {
      client.setQueryData<User | null>(currentUserKey, (current) => current?.id === user.id ? null : current)
    }
    setError(cause instanceof Error ? cause.message : fallback)
  }

  const save = useMutation({
    mutationFn: async () => {
      const draft = pending.current
      pending.current = null
      try {
        return await patch<User>('/users/me', {
          display_name: name,
          bio,
          ...(avatar !== undefined ? { avatar_id: avatar?.id ?? null } : {}),
        })
      } catch (cause) {
        if (draft) {
          if (active.current) pending.current = draft
          else discardAvatar(draft)
        }
        throw cause
      }
    },
    onMutate: () => { setError(''); setSaved(false) },
    onSuccess: async (updated) => {
      await client.cancelQueries({ queryKey: currentUserKey })
      client.setQueryData<User | null>(currentUserKey, (current) => current?.id === updated.id ? updated : current)
      void client.invalidateQueries({ queryKey: ['authors', updated.username] })
      void client.invalidateQueries({ queryKey: ['posts'] })
      void client.invalidateQueries({ queryKey: ['comments'] })
      if (active.current) {
        pending.current = null
        setName(updated.display_name)
        setBio(updated.bio)
        setAvatar(undefined)
        setSaved(true)
      }
    },
    onError: (cause) => showError(cause, 'Unable to save your profile.'),
  })
  const busy = working || save.isPending

  async function uploadAvatar(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) return
    setError('')
    setSaved(false)
    if (file.size > 5 * 1024 * 1024) { setError('Each image must be 5 MiB or smaller.'); return }
    setWorking(true)
    try {
      const body = new FormData()
      body.append('file', file)
      const uploaded = await post<AvatarUpload>('/media', body)
      if (!active.current) {
        discardAvatar(uploaded.id)
        return
      }
      const previous = pending.current
      pending.current = uploaded.id
      setAvatar(uploaded)
      if (previous) discardAvatar(previous)
    } catch (cause) {
      showError(cause, 'Unable to upload your avatar.')
    } finally {
      if (active.current) setWorking(false)
    }
  }

  async function removeAvatar() {
    setError('')
    setSaved(false)
    setWorking(true)
    try {
      if (pending.current) await del('/media/' + pending.current)
      pending.current = null
      if (active.current) setAvatar(null)
    } catch (cause) {
      showError(cause, 'Unable to remove this avatar.')
    } finally {
      if (active.current) setWorking(false)
    }
  }

  function reset() {
    if (pending.current) discardAvatar(pending.current)
    pending.current = null
    setName(user.display_name)
    setBio(user.bio)
    setAvatar(undefined)
    setError('')
    setSaved(false)
  }

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (dirty && valid && !busy) save.mutate()
  }

  return (
    <Box component="form" onSubmit={submit}>
      <Stack spacing={2.5}>
        <Typography variant="h2" component="h2">Edit profile</Typography>
        {error && <Alert severity="error">{error}</Alert>}
        {saved && !dirty && <Alert severity="success">Profile saved.</Alert>}
        <Stack direction="row" spacing={2} sx={{ alignItems: 'center', flexWrap: 'wrap', rowGap: 1 }}>
          <Avatar src={avatarURL ?? undefined} alt="Avatar preview" sx={{ width: 72, height: 72, bgcolor: 'primary.main' }}>
            {Array.from(name.trim() || user.display_name)[0]?.toUpperCase()}
          </Avatar>
          <Button component="label" variant="outlined" disabled={busy}>
            {working ? 'Working...' : 'Upload avatar'}
            <input type="file" hidden accept="image/jpeg,image/png,image/webp"
              disabled={busy} onChange={(event) => void uploadAvatar(event)} />
          </Button>
          <Button color="error" disabled={busy || !avatarURL} onClick={() => void removeAvatar()}>Remove avatar</Button>
        </Stack>
        <Typography variant="body2" color="text.secondary">
          JPEG, PNG, or WebP, up to 5 MiB and 25 megapixels. Your avatar becomes public when you save.
        </Typography>
        <TextField label="Display name" value={name} required fullWidth disabled={busy}
          onChange={(event) => { setName(event.target.value); setSaved(false) }}
          error={nameCount > 60}
          helperText={nameCount + ' / 60 characters. This is the name shown on your profile and posts.'}
        />
        <TextField label="Bio" value={bio} multiline minRows={3} fullWidth disabled={busy}
          onChange={(event) => { setBio(event.target.value); setSaved(false) }}
          error={bioCount > 280} helperText={bioCount + ' / 280 characters.'}
        />
        <Stack direction="row" spacing={1}>
          <Button type="submit" variant="contained" disabled={busy || !dirty || !valid}>
            {save.isPending ? 'Saving...' : 'Save profile'}
          </Button>
          <Button disabled={busy || !dirty} onClick={reset}>Reset changes</Button>
        </Stack>
      </Stack>
    </Box>
  )
}

function discardAvatar(id: string) {
  void del('/media/' + id).catch((error) => console.warn('Unable to discard an unused avatar.', error))
}
