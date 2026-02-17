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
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) return
          if (id.includes('/node_modules/react/') || id.includes('/node_modules/react-dom/') || id.includes('/node_modules/scheduler/')) return 'vendor-react'
          if (id.includes('reactflow')) return 'vendor-flow'
          if (id.includes('recharts')) return 'vendor-charts'
          if (id.includes('react-router-dom')) return 'vendor-router'
          if (id.includes('@connectrpc') || id.includes('@bufbuild/protobuf')) return 'vendor-api'
          if (id.includes('react-markdown') || id.includes('remark-gfm')) return 'vendor-markdown'
          if (id.includes('cmdk')) return 'vendor-cmdk'
          if (id.includes('zustand')) return 'vendor-state'
          return 'vendor-misc'
        },
      },
    },
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
