import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

// In dev the panel path is fixed so the Go server and Vite agree on URLs.
const devPath = process.env.SWIFT_PANEL_PATH ?? 'dev';
const goPort = process.env.SWIFT_PANEL_PORT ?? '24680';

export default defineConfig(({ command }) => ({
  base: command === 'build' ? './' : `/${devPath}/`,
  plugins: [svelte()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    assetsInlineLimit: 0,
    target: 'es2022',
  },
  server: {
    proxy: {
      [`/${devPath}/api`]: { target: `http://127.0.0.1:${goPort}`, changeOrigin: false },
    },
  },
  test: {
    environment: 'jsdom',
    // tokens.test.ts reads stylesheets with ?raw
    css: true,
    include: ['src/**/*.test.ts'],
  },
  resolve: process.env.VITEST ? { conditions: ['browser'] } : undefined,
}));
