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
            id.includes('node_modules/jszip/') ||
            id.includes('node_modules/pako/') ||
            id.includes('node_modules/readable-stream/') ||
            id.includes('node_modules/immediate/') ||
            id.includes('node_modules/lie/') ||
            id.includes('node_modules/setimmediate/')
          ) {
            return 'vendor-zip';
          }

          if (id.includes('node_modules/china-division/')) {
            return 'vendor-region-data';
          }

          if (id.includes('node_modules/qrcode.react/')) {
            return 'vendor-qrcode';
          }

          if (
            id.includes('node_modules/react/') ||
            id.includes('node_modules/react-dom/') ||
            id.includes('node_modules/react-router-dom/') ||
            id.includes('node_modules/react-router/') ||
            id.includes('node_modules/@emotion/') ||
            id.includes('node_modules/hoist-non-react-statics/') ||
            id.includes('node_modules/react-is/') ||
            id.includes('node_modules/scheduler/')
          ) {
            return 'vendor-react';
          }

          if (
            id.includes('node_modules/@mui/icons-material/') ||
            id.includes('node_modules/@mui/material/SvgIcon/') ||
            id.includes('node_modules/@mui/material/utils/') ||
            id.includes('node_modules/@mui/utils/')
          ) {
            return 'vendor-mui-icons';
          }

          if (
            id.includes('node_modules/@mui/') ||
            id.includes('node_modules/@popperjs/') ||
            id.includes('node_modules/react-transition-group/') ||
            id.includes('node_modules/dom-helpers/')
          ) {
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
            id.includes('node_modules/bail/') ||
            id.includes('node_modules/ccount/') ||
            id.includes('node_modules/character-entities') ||
            id.includes('node_modules/decode-named-character-reference/') ||
            id.includes('node_modules/devlop/') ||
            id.includes('node_modules/entities/') ||
            id.includes('node_modules/estree-util-is-identifier-name/') ||
            id.includes('node_modules/github-slugger/') ||
            id.includes('node_modules/inline-style-parser/') ||
            id.includes('node_modules/longest-streak/') ||
            id.includes('node_modules/markdown-table/') ||
            id.includes('node_modules/parse-entities/') ||
            id.includes('node_modules/property-information/') ||
            id.includes('node_modules/rehype-prism-plus/') ||
            id.includes('node_modules/refractor/') ||
            id.includes('node_modules/space-separated-tokens/') ||
            id.includes('node_modules/stringify-entities/') ||
            id.includes('node_modules/style-to-js/') ||
            id.includes('node_modules/style-to-object/') ||
            id.includes('node_modules/trim-lines/') ||
            id.includes('node_modules/trough/') ||
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
      '/api': 'http://localhost:8888',
      '/media': 'http://localhost:8888'
    }
  }
});
