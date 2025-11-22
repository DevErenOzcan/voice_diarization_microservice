import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000, // Frontend servisi genelde 3000 portunda çalışır
    proxy: {
      // Geliştirme ortamında Gateway servisine proxy
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/meeting': {
        target: 'ws://127.0.0.1:8080',
        ws: true
      }
    }
  }
})