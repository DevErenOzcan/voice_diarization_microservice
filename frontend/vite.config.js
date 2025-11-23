import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react(),tailwindcss()],
  server: {
    port: 3000, // Frontend servisi genelde 3000 portunda çalışır
    proxy: {
        '/ws': {
            target: 'ws://localhost:8080',
            ws: true,
            changeOrigin: true
        }
    }
  }
})