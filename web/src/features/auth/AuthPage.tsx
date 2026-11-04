import { useState } from 'react'
import type { FormEvent } from 'react'
import { Alert, Box, Button, CircularProgress, Link, Paper, Stack, TextField, Typography } from '@mui/material'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Link as RouterLink, Navigate, useLocation, useNavigate } from 'react-router-dom'
import { post } from '../../lib/api'
import { currentUserKey, useCurrentUser } from './useCurrentUser'
import type { User } from './useCurrentUser'

export function AuthPage({ mode }: { mode: 'login' | 'register' }) {
  const registering = mode === 'register'
  const [username, setUsername] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const current = useCurrentUser()
  const client = useQueryClient()
  const navigate = useNavigate()
  const location = useLocation()
  const destination = location.state?.from ?? '/account'

  const submit = useMutation({
    mutationFn: () => post<User>(`/auth/${mode}`, {
      email,
      password,
      ...(registering ? { username } : {}),
    }),
    onSuccess: async (user) => {
      await client.cancelQueries({ queryKey: currentUserKey })
      client.setQueryData(currentUserKey, user)
      navigate(destination, { replace: true })
    },
  })

  if (current.isPending) {
    return <Box sx={{ py: 8, textAlign: 'center' }}><CircularProgress aria-label="Checking your session" /></Box>
  }
  if (current.data) return <Navigate to={destination} replace />

  function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    submit.mutate()
  }

  return (
    <Paper variant="outlined" sx={{ maxWidth: 440, mx: 'auto', p: { xs: 3, sm: 4 } }}>
      <Typography variant="h1" component="h1">
        {registering ? 'Create an account' : 'Welcome back'}
      </Typography>
      <Typography color="text.secondary" sx={{ mt: 1, mb: 3 }}>
        {registering ? 'Join MediaShare to share your everyday moments.' : 'Sign in to your MediaShare account.'}
      </Typography>
      <Box component="form" onSubmit={handleSubmit}>
        <Stack spacing={2.5}>
          {submit.isError && <Alert severity="error" role="alert">{submit.error.message}</Alert>}
          {registering && (
            <TextField
              label="Username"
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              autoComplete="username"
              required
              fullWidth
              helperText="3-24 letters (A-Z, a-z), digits, or underscores. Case-sensitive."
              slotProps={{ htmlInput: { minLength: 3, maxLength: 24, pattern: '[A-Za-z0-9_]{3,24}', autoCapitalize: 'none', spellCheck: false } }}
            />
          )}
          <TextField
            label="Email"
            type="email"
            value={email}
            onChange={(event) => setEmail(event.target.value)}
            autoComplete={registering ? 'email' : 'username'}
            required
            fullWidth
            slotProps={{ htmlInput: { maxLength: 254 } }}
          />
          <TextField
            label="Password"
            type="password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            autoComplete={registering ? 'new-password' : 'current-password'}
            required
            fullWidth
            helperText={registering ? 'Use 8-128 characters.' : undefined}
          />
          <Button type="submit" variant="contained" size="large" disabled={submit.isPending}>
            {submit.isPending ? 'Please wait...' : registering ? 'Create account' : 'Sign in'}
          </Button>
          <Typography variant="body2" color="text.secondary" sx={{ textAlign: 'center' }}>
            {registering ? 'Already have an account? ' : 'New to MediaShare? '}
            <Link component={RouterLink} to={registering ? '/login' : '/register'} state={location.state}>
              {registering ? 'Sign in' : 'Create an account'}
            </Link>
          </Typography>
        </Stack>
      </Box>
    </Paper>
  )
}
