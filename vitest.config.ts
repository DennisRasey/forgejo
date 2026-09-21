import {defineConfig} from 'vitest/config';
import vuePlugin from '@vitejs/plugin-vue';
import {stringPlugin} from 'vite-string-plugin';

export default defineConfig({
  test: {
    include: ['web_src/**/*.test.{ts,js}'],
    setupFiles: ['web_src/js/vitest.setup.js'],
    environment: 'happy-dom',
    testTimeout: 20000,
    open: false,
    allowOnly: true,
    passWithNoTests: true,
    globals: true,
    watch: false,
    mockReset: true,
  },
  plugins: [
    stringPlugin(),
    vuePlugin(),
  ],
});
