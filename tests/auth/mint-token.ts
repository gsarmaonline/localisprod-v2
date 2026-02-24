/**
 * SSH-based token minter — no browser required.
 *
 * Connects to the production server via SSH to fetch the user ID from SQLite,
 * then signs a JWT locally using jsonwebtoken (same algorithm as the Go server).
 * Saves the token to .session.
 *
 * Prerequisites:
 *   - At least one user must exist in the production DB
 *     (log in to localisprod.com at least once first)
 *   - SSH access to the server (TEST_NODE_HOST / TEST_NODE_USER)
 *   - JWT_SECRET set in .env.test
 *
 * Usage:
 *   npm run auth:ssh
 */
import { execSync } from 'child_process';
import fs from 'fs';
import path from 'path';
import os from 'os';
import jwt from 'jsonwebtoken';

// Read .env.test directly instead of relying on dotenv (avoids issues with
// tsx + dotenv not loading in time when run via npm scripts).
function readEnvFile(filePath: string): Record<string, string> {
  if (!fs.existsSync(filePath)) return {};
  const out: Record<string, string> = {};
  for (const line of fs.readFileSync(filePath, 'utf-8').split('\n')) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith('#')) continue;
    const idx = trimmed.indexOf('=');
    if (idx < 0) continue;
    out[trimmed.slice(0, idx).trim()] = trimmed.slice(idx + 1).trim();
  }
  return out;
}

const ENV = readEnvFile('.env.test');

const SERVER_HOST  = ENV.TEST_NODE_HOST  || '167.71.230.5';
const SERVER_USER  = ENV.TEST_NODE_USER  || 'root';
const SERVER_PORT  = ENV.TEST_NODE_PORT  || '22';
const JWT_SECRET   = ENV.JWT_SECRET      || '';
const SESSION_FILE = path.join(process.cwd(), '.session');

const rawKeyPath = ENV.TEST_NODE_KEY_PATH || '~/.ssh/id_rsa';
const KEY_PATH   = rawKeyPath.replace(/^~/, os.homedir());

if (!JWT_SECRET) {
  console.error('ERROR: JWT_SECRET not set in .env.test');
  process.exit(1);
}

// Python script that runs on the server — only reads the DB and prints user info.
// Signing happens locally in Node.js using jsonwebtoken.
const PYTHON = `
import sqlite3, json, subprocess, sys

try:
    names = subprocess.check_output(
        ['docker', 'volume', 'ls', '--format', '{{.Name}}'], text=True
    ).split()
    vol = next(n for n in names if 'localisprod' in n and 'data' in n)
    mount = subprocess.check_output(
        ['docker', 'volume', 'inspect', vol, '--format', '{{.Mountpoint}}'],
        text=True
    ).strip()
except StopIteration:
    print(json.dumps({"error": "No localisprod-data volume found"}))
    sys.exit(1)

db_path = f'{mount}/cluster.db'
conn = sqlite3.connect(db_path)
row = conn.execute('SELECT id, email, name, avatar_url FROM users LIMIT 1').fetchone()
conn.close()

if not row:
    print(json.dumps({"error": "No users in DB. Log in to localisprod.com first."}))
    sys.exit(1)

uid, email, name, avatar = row
print(json.dumps({
    "user_id": uid, "email": email,
    "name": name, "avatar_url": avatar or ""
}))
`;

console.log('\n=================================================');
console.log(' Localisprod Token Minter (SSH)');
console.log('=================================================');
console.log(`Server: ${SERVER_USER}@${SERVER_HOST}:${SERVER_PORT}`);
console.log('');

try {
  console.log('Fetching user from production DB...');

  const sshArgs = [
    'ssh',
    '-p', SERVER_PORT,
    '-o', 'StrictHostKeyChecking=accept-new',
    '-o', 'BatchMode=yes',
    ...(fs.existsSync(KEY_PATH) ? ['-i', KEY_PATH] : []),
    `${SERVER_USER}@${SERVER_HOST}`,
    'python3 -',
  ].join(' ');

  const rawOutput = execSync(sshArgs, {
    input: PYTHON,
    encoding: 'utf-8',
    stdio: ['pipe', 'pipe', 'pipe'],
  }).trim();

  const userInfo = JSON.parse(rawOutput) as {
    user_id: string;
    email: string;
    name: string;
    avatar_url: string;
    error?: string;
  };

  if (userInfo.error) {
    console.error('ERROR from server:', userInfo.error);
    process.exit(1);
  }

  // Sign JWT locally using jsonwebtoken — same algorithm (HS256) and claims
  // as the Go server's auth.JWTService.Issue() method.
  const token = jwt.sign(
    {
      user_id:   userInfo.user_id,
      email:     userInfo.email,
      name:      userInfo.name,
      avatar_url: userInfo.avatar_url,
    },
    JWT_SECRET,
    { algorithm: 'HS256', expiresIn: '30d' }
  );

  fs.writeFileSync(SESSION_FILE, token, { encoding: 'utf-8' });

  const decoded = jwt.decode(token) as Record<string, unknown>;
  const expiresDate = new Date((decoded.exp as number) * 1000).toLocaleDateString();

  console.log(`\n✓ Token saved to: ${SESSION_FILE}`);
  console.log(`  User:    ${userInfo.email}`);
  console.log(`  Expires: ${expiresDate}`);
  console.log('\nRun: npm test\n');
} catch (err: unknown) {
  const e = err as NodeJS.ErrnoException & { stderr?: Buffer };
  console.error('\nERROR:', e.message);
  if (e.stderr) console.error('Remote:', e.stderr.toString().trim());
  console.error('\nTips:');
  console.error('  - Check TEST_NODE_HOST / TEST_NODE_KEY_PATH in .env.test');
  console.error('  - Ensure JWT_SECRET in .env.test matches /opt/localisprod/app/.env');
  console.error('  - Ensure you have logged in to localisprod.com at least once');
  process.exit(1);
}
