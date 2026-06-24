import {defineConfig} from 'vite'
import react from '@vitejs/plugin-react'
import {fileURLToPath, URL} from 'node:url'

export default defineConfig({
    plugins: [react()],
    resolve: {
        alias: {
            '@': fileURLToPath(new URL('./src', import.meta.url)),
        },
    },
    build: {
        sourcemap: false,
        outDir: 'dist',
        emptyOutDir: true,
        rollupOptions: {
            maxParallelFileOps: 16,
            output: {
                manualChunks(id) {
                    if (id.includes('node_modules')) return 'vendor'
                }
            }
        }
    },
    optimizeDeps: {
        force: true
    },
    server: {
        port: 5173,
        proxy: {
            '/api': {
                target: 'http://localhost:8080',
                changeOrigin: true,
            },
        },
    },
})
