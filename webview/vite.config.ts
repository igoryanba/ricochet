import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { fileURLToPath } from 'url';
import { dirname, resolve } from 'path';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

export default defineConfig({
    plugins: [react()],
    build: {
        outDir: '../extension-vscode/webview-dist',
        emptyOutDir: true,
        rollupOptions: {
            output: {
                entryFileNames: 'main.js',
                assetFileNames: 'main.[ext]'
            }
        }
    },
    resolve: {
        alias: {
            '@': resolve(__dirname, './src'),
            '@components': resolve(__dirname, './src/components'),
            '@hooks': resolve(__dirname, './src/hooks')
        }
    }
});
