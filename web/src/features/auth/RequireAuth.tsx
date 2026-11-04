import { Alert, Box, Button, CircularProgress } from '@mui/material'
import { Navigate, Outlet, useLocation } from 'react-router-dom'
import { useCurrentUser } from './useCurrentUser'

export function RequireAuth() {
  const current = useCurrentUser()
  const location = useLocation()
  if (current.isPending) {
    return <Box sx={{ py: 8, textAlign: 'center' }}><CircularProgress aria-label="Checking your session" /></Box>
  }
  if (current.isError) {
    return (
      <Alert severity="warning" action={<Button onClick={() => void current.refetch()}>Retry</Button>}>
        Unable to check your session. Please try again.
      </Alert>
    )
  }
  if (!current.data) {
    return <Navigate to="/login" state={{ from: location.pathname }} replace />
  }
  return <Outlet />
}
