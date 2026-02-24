/**
 * Interactive auth helper — opens a browser for Google OAuth login,
 * extracts the session cookie, and saves it to .session.
 *
 * Usage:
 *   npm run auth
 *
 * The saved session is valid for 30 days. Re-run when it expires.
 */
import { chromium } from 'playwright';
import fs from 'fs';
import path from 'path';
import { config } from 'dotenv';

config({ path: '.env.test' });

const BASE_URL = process.env.BASE_URL || 'https://localisprod.com';
const SESSION_FILE = path.join(process.cwd(), '.session');
const baseOrigin = new URL(BASE_URL).origin;

console.log('\n=================================================');
console.log(' Localisprod Auth Helper (Browser)');
console.log('=================================================');
console.log(`Target: ${BASE_URL}`);
console.log('');

const browser = await chromium.launch({ headless: false });
const context = await browser.newContext();
const page = await context.newPage();

console.log('Opening browser...');
await page.goto(BASE_URL);

console.log('Please complete Google OAuth login in the browser window.');
console.log('Waiting up to 2 minutes...\n');

// Wait until the browser returns to the app domain and past the login page
await page.waitForURL(
  url => {
    try {
      const u = new URL(url);
      return u.origin === baseOrigin && u.pathname !== '/login';
    } catch {
      return false;
    }
  },
  { timeout: 120_000 }
);

// Small delay to ensure cookie is fully committed
await page.waitForTimeout(500);

const cookies = await context.cookies();
const session = cookies.find(
  c => c.name === 'session' && c.domain.includes(new URL(BASE_URL).hostname)
);

if (!session) {
  console.error('\nERROR: Session cookie not found after login.');
  console.error('The login may have failed. Try again or check the browser.');
  process.exit(1);
}

fs.writeFileSync(SESSION_FILE, session.value, { encoding: 'utf-8' });

const expiresDate = new Date(session.expires * 1000).toLocaleDateString();
console.log(`\n✓ Session saved to: ${SESSION_FILE}`);
console.log(`  Expires: ${expiresDate}`);
console.log('\nRun: npm test\n');

await browser.close();
process.exit(0);
