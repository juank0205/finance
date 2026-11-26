import react from '@vitejs/plugin-react'
import { defineConfig, loadEnv } from 'vite'

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  // With VITE_API_URL=/api/ the dev and preview servers forward /api to VITE_API_PROXY: a local
  // cloud-api (the default) or https://finance.chimborrio.shop for the API in AWS. The browser then
  // talks to one origin, as it does in production. No CORS on either side.
  const target = loadEnv(mode, process.cwd(), '').VITE_API_PROXY || 'http://localhost:8080'
  const proxy = { '/api': { target, changeOrigin: true } }
  return {
    plugins: [react()],
    server: { proxy },
    preview: { proxy },
  }
})
