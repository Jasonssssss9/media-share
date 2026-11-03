import { Alert, Box, Button, Chip, Paper, Skeleton, Stack, Typography } from '@mui/material'
import { useQuery } from '@tanstack/react-query'
import { get } from '../../lib/api'

type ServiceStatus = { name: string; status: 'ready' }

export function FeedPage() {
  const status = useQuery({
    queryKey: ['service-status'],
    queryFn: ({ signal }) => get<ServiceStatus>('/status', signal),
  })

  return (
    <Stack spacing={4}>
      <Stack direction="row" sx={{ alignItems: 'center', justifyContent: 'space-between', gap: 2 }}>
        <Box>
          <Typography variant="h1" component="h1">Your feed</Typography>
          <Typography color="text.secondary" sx={{ mt: 1 }}>
            Photos, thoughts, and everyday moments.
          </Typography>
        </Box>
        {status.isPending && <Skeleton width={90} height={32} />}
        {status.isSuccess && <Chip label="Connected" color="success" size="small" variant="outlined" />}
      </Stack>

      {status.isError ? (
        <Alert
          severity="warning"
          action={<Button color="inherit" size="small" onClick={() => void status.refetch()}>Retry</Button>}
        >
          MediaShare is unavailable right now. Please try again.
        </Alert>
      ) : (
        <Paper className="empty-feed" variant="outlined" aria-busy={status.isPending}>
          <Box className="empty-icon" aria-hidden="true">
            <svg width="42" height="42" viewBox="0 0 32 32" fill="none">
              <rect x="4" y="5" width="24" height="22" rx="3" stroke="currentColor" strokeWidth="1.8" />
              <circle cx="11" cy="12" r="2" fill="currentColor" />
              <path d="m6 24 7-7 4 4 4-6 5 9" stroke="currentColor" strokeWidth="1.8" strokeLinejoin="round" />
            </svg>
          </Box>
          <Typography variant="h2" component="h2">No posts yet</Typography>
          <Typography color="text.secondary" sx={{ mt: 1, maxWidth: 320 }}>
            Shared photos and stories will appear here.
          </Typography>
        </Paper>
      )}
    </Stack>
  )
}
