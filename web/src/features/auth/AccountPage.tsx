import { Avatar, Button, Divider, Paper, Stack, Typography } from '@mui/material'
import { Link } from 'react-router-dom'
import { useCurrentUser } from './useCurrentUser'
import { ProfileForm } from './ProfileForm'

export function AccountPage() {
  const { data: user } = useCurrentUser()
  if (!user) return null

  return (
    <Paper variant="outlined" sx={{ maxWidth: 600, mx: 'auto', p: { xs: 3, sm: 4 }, overflowWrap: 'anywhere' }}>
      <Stack spacing={3}>
        <Stack direction="row" spacing={2} sx={{ alignItems: 'center' }}>
          <Avatar src={user.avatar_url ?? undefined} alt="Your avatar" sx={{ width: 52, height: 52, bgcolor: 'primary.main' }}>
            {Array.from(user.display_name)[0]?.toUpperCase()}
          </Avatar>
          <div>
            <Typography variant="h1" component="h1">Your account</Typography>
            <Typography color="text.secondary">Signed in as {user.display_name}</Typography>
          </div>
        </Stack>
        <Button component={Link} to={'/users/' + user.username} sx={{ alignSelf: 'flex-start' }}>View your profile</Button>
        <Divider />
        <ProfileForm key={user.id} user={user} />
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
