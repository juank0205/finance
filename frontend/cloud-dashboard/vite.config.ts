import react from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    // With VITE_API_URL=/ the dev server forwards the API to a local cloud-api, so the browser talks
    // to one origin, as it will in production. No CORS on either side.
    proxy: { '/v1': 'http://localhost:8080' },
  },
})
