import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';

const backend = 'http://127.0.0.1:8000';

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: 'build',
    assetsDir: 'static',
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/cmd': backend,
      '/con': backend,
      '/monitor': backend,
      '/proc': backend,
      '/settings': backend,
      '/status': backend,
      '/uptime': backend,
    },
  },
  test: {
    environment: 'happy-dom',
    globals: true,
    include: ['tests/**/*.{test,spec}.{js,jsx}'],
    setupFiles: './tests/setupTests.js',
    clearMocks: true,
    restoreMocks: true,
    coverage: {
      provider: 'v8',
      reporter: ['text', 'lcov'],
      include: ['src/**/*.{js,jsx}'],
      exclude: ['src/index.jsx'],
      thresholds: {
        statements: 55,
        branches: 45,
        functions: 45,
        lines: 55,
      },
    },
  },
});
