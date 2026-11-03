import { Container, Typography } from '@mui/material'
import { Link, Outlet } from 'react-router-dom'

export function AppLayout() {
  return (
    <div className="app">
      <header className="site-header">
        <Container maxWidth="md" className="header-content">
          <Link className="brand" to="/" aria-label="MediaShare home">
            <img src="/favicon.svg" alt="" width="32" height="32" />
            <span>MediaShare</span>
          </Link>
          <nav aria-label="Main navigation">
            <Link className="nav-link" to="/">Feed</Link>
          </nav>
        </Container>
      </header>
      <Container component="main" maxWidth="md" className="main-content">
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
