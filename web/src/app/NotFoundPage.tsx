import { Button, Stack, Typography } from '@mui/material'
import { Link } from 'react-router-dom'

export function NotFoundPage() {
  return (
    <Stack spacing={2} sx={{ alignItems: 'center', py: 10, textAlign: 'center' }}>
      <Typography variant="h1">Page not found</Typography>
      <Typography color="text.secondary">This page does not exist.</Typography>
      <Button component={Link} to="/" variant="contained">Back to feed</Button>
    </Stack>
  )
}
