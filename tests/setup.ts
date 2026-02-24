// Runs in each vitest worker before test files are imported.
// Loads .env.test so process.env vars are available to all test code.
import { config } from 'dotenv';

config({ path: '.env.test', override: true });
