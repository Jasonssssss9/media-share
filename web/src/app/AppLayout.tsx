import { Alert, Avatar, Box, Button, Container, Typography } from '@mui/material'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Link, Outlet, useNavigate } from 'react-router-dom'
import { post } from '../lib/api'
import { currentUserKey, useCurrentUser } from '../features/auth/useCurrentUser'

export function AppLayout() {
  const current = useCurrentUser()
  const client = useQueryClient()
  const navigate = useNavigate()
  const logout = useMutation({
    mutationFn: () => post<void>('/auth/logout'),
    onSuccess: async () => {
      await client.cancelQueries({ queryKey: currentUserKey })
      client.setQueryData(currentUserKey, null)
      navigate('/login', { replace: true })
    },
  })

  return (
    <div className="app">
      <header className="site-header">
        <Container maxWidth="md" className="header-content">
          <Link className="brand" to="/" aria-label="MediaShare home">
            <img src="/favicon.svg" alt="" width="32" height="32" />
            <span>MediaShare</span>
          </Link>
          <nav className="header-actions" aria-label="Main navigation">
            <Box component={Link} className="nav-link" to="/" sx={{ display: { xs: 'none', sm: 'block' } }}>Feed</Box>
            {current.data ? (
              <>
                <Link className="account-link" to="/account" aria-label="Your account">
                  <Avatar src={current.data.avatar_url ?? undefined} alt="Your avatar" sx={{ width: 30, height: 30, bgcolor: 'primary.main', fontSize: 14 }}>
                    {Array.from(current.data.display_name)[0]?.toUpperCase()}
                  </Avatar>
                  <Box component="span" sx={{ display: { xs: 'none', sm: 'inline' } }}>{current.data.username}</Box>
                </Link>
                <Button size="small" disabled={logout.isPending} onClick={() => logout.mutate()}>Sign out</Button>
              </>
            ) : !current.isPending && (
              <>
                <Button component={Link} to="/login" size="small">Sign in</Button>
                <Button component={Link} to="/register" size="small" variant="outlined" sx={{ display: { xs: 'none', sm: 'inline-flex' } }}>Register</Button>
              </>
            )}
          </nav>
        </Container>
      </header>
      <Container component="main" maxWidth="md" className="main-content">
        {logout.isError && <Alert severity="error" sx={{ mb: 3 }}>{logout.error.message}</Alert>}
        <Outlet />
      </Container>
      <footer className="site-footer">
        <Typography variant="body2" color="text.secondary">
          A place for photos and everyday stories.
        </Typography>
      </footer>
    </div>
  )
}
