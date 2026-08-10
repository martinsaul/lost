import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Builds the SPA into web/dist, which the Go binary embeds. During dev, API
// calls are proxied to the running backend (LOST_ADDR / :8080).
export default defineConfig({
  plugins: [react()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
      '/healthz': 'http://localhost:8080',
    },
  },
})
