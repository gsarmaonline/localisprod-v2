/**
 * Thin fetch wrapper that attaches the saved session cookie to every request.
 *
 * The session cookie is read once at import time from tests/.session.
 * Run `npm run auth` or `npm run auth:ssh` first to populate that file.
 */
import fs from 'fs';
import path from 'path';

// BASE_URL is read lazily inside request() to avoid ESM import-hoisting issues
// (vitest may evaluate this module before dotenv/env setup has run).
const DEFAULT_BASE_URL = 'https://localisprod.com';

export function getBaseURL(): string {
  const url = process.env.BASE_URL;
  return url?.startsWith('http') ? url : DEFAULT_BASE_URL;
}

let _session: string | null = null;

function getSession(): string {
  if (_session) return _session;
  const file = path.join(process.cwd(), '.session');
  if (!fs.existsSync(file)) {
    throw new Error(
      `No session file found at ${file}.\n` +
      `Run "npm run auth" (browser) or "npm run auth:ssh" (headless) first.`
    );
  }
  const token = fs.readFileSync(file, 'utf-8').trim();
  if (!token) throw new Error('Session file is empty. Run the auth helper again.');
  _session = token;
  return _session;
}

async function request(endpoint: string, init: RequestInit = {}): Promise<Response> {
  return fetch(`${getBaseURL()}${endpoint}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      Cookie: `session=${getSession()}`,
      ...init.headers,
    },
  });
}

export async function get<T>(endpoint: string): Promise<T> {
  const res = await request(endpoint);
  if (!res.ok) {
    const body = await res.text();
    throw new Error(`GET ${endpoint} → ${res.status} ${res.statusText}: ${body}`);
  }
  return res.json() as Promise<T>;
}

export async function post<T>(endpoint: string, body: unknown): Promise<T> {
  const res = await request(endpoint, {
    method: 'POST',
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`POST ${endpoint} → ${res.status} ${res.statusText}: ${text}`);
  }
  return res.json() as Promise<T>;
}

export async function put<T>(endpoint: string, body: unknown): Promise<T> {
  const res = await request(endpoint, {
    method: 'PUT',
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(`PUT ${endpoint} → ${res.status} ${res.statusText}: ${text}`);
  }
  return res.json() as Promise<T>;
}

export async function del(endpoint: string): Promise<void> {
  const res = await request(endpoint, { method: 'DELETE' });
  if (!res.ok && res.status !== 404) {
    const text = await res.text();
    throw new Error(`DELETE ${endpoint} → ${res.status} ${res.statusText}: ${text}`);
  }
}
