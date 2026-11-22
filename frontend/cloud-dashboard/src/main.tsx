import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { ready } from './api/auth.ts'

const root = createRoot(document.getElementById('root')!)

// Render only once signed in, so the app never sees the sign-in redirect's ?code= in the URL.
ready.then(
  () =>
    root.render(
      <StrictMode>
        <App />
      </StrictMode>,
    ),
  (err: unknown) => root.render(<p role="alert">{String(err)}</p>),
)
