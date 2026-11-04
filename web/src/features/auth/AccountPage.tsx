import { Avatar, Divider, Paper, Stack, Typography } from '@mui/material'
import { useCurrentUser } from './useCurrentUser'

export function AccountPage() {
  const { data: user } = useCurrentUser()
  if (!user) return null

  return (
    <Paper variant="outlined" sx={{ maxWidth: 600, mx: 'auto', p: { xs: 3, sm: 4 } }}>
      <Stack spacing={3}>
        <Stack direction="row" spacing={2} sx={{ alignItems: 'center' }}>
          <Avatar sx={{ width: 52, height: 52, bgcolor: 'primary.main' }}>
            {user.display_name.charAt(0).toUpperCase()}
          </Avatar>
          <div>
            <Typography variant="h1" component="h1">Your account</Typography>
            <Typography color="text.secondary">Signed in as {user.display_name}</Typography>
          </div>
        </Stack>
        <Divider />
        <div>
          <Typography variant="body2" color="text.secondary">Username</Typography>
          <Typography>{user.username}</Typography>
        </div>
        <div>
          <Typography variant="body2" color="text.secondary">Email</Typography>
          <Typography sx={{ overflowWrap: 'anywhere' }}>{user.email}</Typography>
        </div>
        <div>
          <Typography variant="body2" color="text.secondary">Member since</Typography>
          <Typography>{new Date(user.created_at).toLocaleDateString()}</Typography>
        </div>
      </Stack>
    </Paper>
  )
}
