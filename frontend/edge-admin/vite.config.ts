import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    // In dev, forward API calls to a locally running admin-api (same path Caddy uses on the Pi).
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
})
