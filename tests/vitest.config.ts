import { defineConfig } from 'vitest/config';
import { config } from 'dotenv';

// Load .env.test before any test modules are evaluated so BASE_URL etc. are
// available at module-init time in client.ts.
// override:true is required because vite injects BASE_URL='/' into process.env
// before this runs; without it dotenv would silently keep the vite value.
config({ path: '.env.test', override: true });

export default defineConfig({
  test: {
    testTimeout: 90_000,   // Docker pull + run can take a while
    hookTimeout: 60_000,   // afterAll cleanup with SSH ops
    reporters: ['verbose'],
    sequence: {
      shuffle: false,      // Tests must run in defined order
    },
    setupFiles: ['./setup.ts'],
  },
});
