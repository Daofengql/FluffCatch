import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  build: {
    cssMinify: true,
    rollupOptions: {
      output: {
        assetFileNames: (assetInfo) => {
          const name = assetInfo.name || '';

          if (name.endsWith('.css')) {
            return 'assets/css/[name]-[hash][extname]';
          }

          if (/\.(woff2?|ttf|eot|otf)$/i.test(name)) {
            return 'assets/fonts/[name]-[hash][extname]';
          }

          if (/\.(png|jpe?g|gif|svg|webp|ico|bmp)$/i.test(name)) {
            return 'assets/img/[name]-[hash][extname]';
          }

          return 'assets/[name]-[hash][extname]';
        },
        chunkFileNames: 'assets/js/[name]-[hash].js',
        entryFileNames: 'assets/js/[name]-[hash].js',
        manualChunks(id) {
          if (
            id.includes('node_modules/react/') ||
            id.includes('node_modules/react-dom/') ||
            id.includes('node_modules/react-router-dom/') ||
            id.includes('node_modules/@emotion/') ||
            id.includes('node_modules/scheduler/')
          ) {
            return 'vendor-react';
          }

          if (id.includes('node_modules/@mui/')) {
            return 'vendor-mui';
          }

          if (id.includes('node_modules/@fontsource/')) {
            return 'vendor-fonts';
          }

          if (
            id.includes('node_modules/@uiw/') ||
            id.includes('node_modules/react-markdown/') ||
            id.includes('node_modules/remark') ||
            id.includes('node_modules/rehype') ||
            id.includes('node_modules/unified/') ||
            id.includes('node_modules/micromark') ||
            id.includes('node_modules/mdast') ||
            id.includes('node_modules/hast') ||
            id.includes('node_modules/unist') ||
            id.includes('node_modules/vfile') ||
            id.includes('node_modules/parse-entities/') ||
            id.includes('node_modules/property-information/') ||
            id.includes('node_modules/comma-separated-tokens/') ||
            id.includes('node_modules/space-separated-tokens/')
          ) {
            return 'vendor-markdown';
          }

          if (id.includes('node_modules/')) {
            return 'vendor';
          }
        }
      }
    }
  },
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:8080',
      '/media': 'http://localhost:8080'
    }
  }
});
