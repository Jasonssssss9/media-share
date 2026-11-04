import { useState } from 'react'
import { Alert, Button, Stack, TextField, Typography } from '@mui/material'
import { useQuery } from '@tanstack/react-query'
import { get } from '../../lib/api'

type Props = {
  disabled: boolean
  generating: boolean
  onGenerate: (prompt: string) => Promise<void>
}

export function ImageGenerator({ disabled, generating, onGenerate }: Props) {
  const [prompt, setPrompt] = useState('')
  const count = Array.from(prompt.trim()).length
  const status = useQuery({
    queryKey: ['media', 'generation'],
    queryFn: ({ signal }) => get<{ enabled: boolean }>('/media/generation', signal),
  })

  return (
    <Stack spacing={1.5}>
      <Typography variant="h6" component="h2">Generate an image</Typography>
      {status.isPending ? (
        <Typography color="text.secondary">Checking image generation availability...</Typography>
      ) : status.isError ? (
        <Alert severity="warning" action={<Button onClick={() => void status.refetch()}>Retry</Button>}>
          Unable to check image generation availability.
        </Alert>
      ) : !status.data.enabled ? (
        <Alert severity="info">Image generation is not enabled on this server.</Alert>
      ) : (
        <>
          <TextField
            label="Image prompt" value={prompt}
            onChange={(event) => setPrompt(event.target.value)}
            multiline minRows={2} fullWidth disabled={disabled}
            error={count > 1000}
            helperText={count + ' / 1,000 characters. Describe the image you want to create.'}
          />
          <Button variant="outlined" sx={{ alignSelf: 'flex-start' }}
            disabled={disabled || count === 0 || count > 1000}
            onClick={() => void onGenerate(prompt.trim())}>
            {generating ? 'Generating...' : 'Generate image'}
          </Button>
          <Typography variant="body2" color="text.secondary" role="status">
            {generating
              ? 'Generating your image. This may take a couple of minutes.'
              : 'One image per request, with 30 seconds between attempts. Generated images count toward the four-image limit.'}
          </Typography>
        </>
      )}
    </Stack>
  )
}
