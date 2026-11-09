import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

// The subscription page is a separate tiny bundle served at /s/<token>.
export default defineConfig({
  root: 'sub',
  base: './',
  publicDir: '../public',
  plugins: [svelte({ configFile: '../svelte.config.js' })],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    assetsInlineLimit: 0,
    target: 'es2022',
  },
});
