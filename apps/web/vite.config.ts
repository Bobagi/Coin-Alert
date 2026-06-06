import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

// In dev, proxy API + auth calls to the Go backend so the SPA is same-origin.
// In production the static build is served by nginx, which proxies /api and /auth to the API.
export default defineConfig({
  plugins: [svelte()],
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:5020',
      '/auth': 'http://localhost:5020'
    }
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true
  }
})
