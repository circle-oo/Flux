import path from 'path'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
      '@proto': path.resolve(__dirname, './src/gen'),
    },
    preserveSymlinks: true,
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/ws': {
        target: 'ws://localhost:8080',
        ws: true,
      },
      '/internal': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/flux.v1.FluxService': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})
