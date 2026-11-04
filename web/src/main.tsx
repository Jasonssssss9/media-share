import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { CssBaseline, ThemeProvider } from '@mui/material'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createBrowserRouter, RouterProvider } from 'react-router-dom'
import { AppLayout } from './app/AppLayout'
import { theme } from './app/theme'
import { FeedPage } from './features/feed/FeedPage'
import { NotFoundPage } from './app/NotFoundPage'
import { AuthPage } from './features/auth/AuthPage'
import { RequireAuth } from './features/auth/RequireAuth'
import { AccountPage } from './features/auth/AccountPage'
import { PostDetailPage } from './features/posts/PostDetailPage'
import { PostEditorPage } from './features/posts/PostEditorPage'
import { AuthorPage } from './features/posts/AuthorPage'
import './styles.css'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: { retry: 1, staleTime: 30_000, refetchOnWindowFocus: false },
  },
})

const router = createBrowserRouter([
  {
    element: <AppLayout />,
    children: [
      { path: '/', element: <FeedPage /> },
      { path: '/login', element: <AuthPage key="login" mode="login" /> },
      { path: '/register', element: <AuthPage key="register" mode="register" /> },
      { path: '/posts/:id', element: <PostDetailPage /> },
      { path: '/users/:username', element: <AuthorPage /> },
      { element: <RequireAuth />, children: [
        { path: '/account', element: <AccountPage /> },
        { path: '/posts/new', element: <PostEditorPage /> },
        { path: '/posts/:id/edit', element: <PostEditorPage /> },
      ] },
      { path: '*', element: <NotFoundPage /> },
    ],
  },
])

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </ThemeProvider>
  </StrictMode>,
)
