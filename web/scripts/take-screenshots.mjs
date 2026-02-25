/**
 * Screenshot capture script for Localisprod v2
 *
 * Starts the Vite dev server, mocks all API responses to bypass
 * Google OAuth, then captures desktop + mobile screenshots of every route.
 *
 * Usage:
 *   npm run screenshots          (from web/)
 *   node scripts/take-screenshots.mjs
 */

import puppeteer from 'puppeteer'
import { spawn } from 'child_process'
import { mkdirSync } from 'fs'
import { fileURLToPath } from 'url'
import { dirname, join } from 'path'

const __dirname = dirname(fileURLToPath(import.meta.url))
const ROOT = join(__dirname, '..')
const SCREENSHOTS_DIR = join(ROOT, 'screenshots')
const PORT = 5173
const BASE_URL = `http://localhost:${PORT}`

// ─── Mock API responses ───────────────────────────────────────────────────────

const MOCK_USER = {
  id: '1',
  email: 'demo@localisprod.dev',
  name: 'Demo User',
  avatar_url: '',
}

const MOCK_STATS = {
  nodes: 5,
  applications: 3,
  deployments: { running: 8, stopped: 2, failed: 1, pending: 3 },
}

const MOCK_NODES = [
  { id: '1', name: 'prod-server-1', host: '10.0.0.10', port: 22, username: 'ubuntu', status: 'online', traefik_enabled: true, provider: 'digitalocean', provider_region: 'nyc3', created_at: '2025-11-01T10:00:00Z' },
  { id: '2', name: 'staging-node', host: '10.0.0.11', port: 22, username: 'ubuntu', status: 'online', traefik_enabled: false, created_at: '2025-11-15T14:20:00Z' },
  { id: '3', name: 'worker-eu', host: '10.0.0.20', port: 22, username: 'root', status: 'offline', traefik_enabled: false, provider: 'aws', provider_region: 'eu-west-1', created_at: '2025-12-01T08:30:00Z' },
]

const MOCK_APPS = [
  { id: '1', name: 'web-frontend', docker_image: 'nginx:1.25-alpine', dockerfile_path: '', env_vars: '{}', ports: '["80:80"]', command: '', github_repo: 'org/web-frontend', domain: 'app.example.com', databases: '[]', caches: '["1"]', kafkas: '[]', monitorings: '[]', created_at: '2025-11-05T09:00:00Z' },
  { id: '2', name: 'api-service', docker_image: 'node:20-alpine', dockerfile_path: 'Dockerfile', env_vars: '{"NODE_ENV":"production"}', ports: '["3000:3000"]', command: 'node dist/index.js', github_repo: 'org/api-service', domain: 'api.example.com', databases: '["1"]', caches: '[]', kafkas: '["1"]', monitorings: '["1"]', created_at: '2025-11-10T12:00:00Z' },
  { id: '3', name: 'worker', docker_image: 'python:3.12-slim', dockerfile_path: 'Dockerfile', env_vars: '{}', ports: '[]', command: 'python worker.py', github_repo: '', domain: '', databases: '["1"]', caches: '[]', kafkas: '["1"]', monitorings: '[]', created_at: '2025-12-01T10:00:00Z' },
]

const MOCK_DATABASES = [
  { id: '1', name: 'main-postgres', type: 'postgres', version: '16', node_id: '1', node_name: 'prod-server-1', node_host: '10.0.0.10', dbname: 'appdb', db_user: 'appuser', port: 5432, container_name: 'localisprod-main-postgres-abc12345', status: 'running', created_at: '2025-11-02T10:00:00Z' },
  { id: '2', name: 'analytics-db', type: 'postgres', version: '15', node_id: '2', node_name: 'staging-node', node_host: '10.0.0.11', dbname: 'analytics', db_user: 'analyst', port: 5433, container_name: 'localisprod-analytics-db-def45678', status: 'running', created_at: '2025-12-05T08:00:00Z' },
]

const MOCK_CACHES = [
  { id: '1', name: 'session-cache', version: '7.2', node_id: '1', node_name: 'prod-server-1', node_host: '10.0.0.10', port: 6379, container_name: 'localisprod-session-cache-ghi90123', status: 'running', created_at: '2025-11-03T10:00:00Z' },
]

const MOCK_KAFKAS = [
  { id: '1', name: 'event-bus', version: '3.6', node_id: '1', node_name: 'prod-server-1', node_host: '10.0.0.10', port: 9092, container_name: 'localisprod-event-bus-jkl45678', status: 'running', created_at: '2025-11-04T10:00:00Z' },
]

const MOCK_MONITORINGS = [
  { id: '1', name: 'prod-monitoring', node_id: '1', node_name: 'prod-server-1', node_host: '10.0.0.10', prometheus_port: 9090, grafana_port: 3001, prometheus_container_name: 'localisprod-prometheus-mno12345', grafana_container_name: 'localisprod-grafana-pqr67890', status: 'running', created_at: '2025-11-05T10:00:00Z' },
]

const MOCK_DEPLOYMENTS = [
  { id: '1', application_id: '1', node_id: '1', container_name: 'localisprod-web-frontend-stu12345', container_id: 'abc123def456', status: 'running', created_at: '2026-01-10T09:30:00Z', app_name: 'web-frontend', node_name: 'prod-server-1', docker_image: 'nginx:1.25-alpine' },
  { id: '2', application_id: '2', node_id: '1', container_name: 'localisprod-api-service-vwx67890', container_id: 'def456ghi789', status: 'running', created_at: '2026-01-12T14:00:00Z', app_name: 'api-service', node_name: 'prod-server-1', docker_image: 'node:20-alpine' },
  { id: '3', application_id: '3', node_id: '2', container_name: 'localisprod-worker-yza34567', container_id: 'ghi789jkl012', status: 'stopped', created_at: '2026-01-08T11:00:00Z', app_name: 'worker', node_name: 'staging-node', docker_image: 'python:3.12-slim' },
  { id: '4', application_id: '1', node_id: '2', container_name: 'localisprod-web-frontend-bcd90123', container_id: 'jkl012mno345', status: 'failed', created_at: '2026-01-07T08:00:00Z', app_name: 'web-frontend', node_name: 'staging-node', docker_image: 'nginx:1.25-alpine' },
]

const MOCK_SETTINGS = {
  github_username: 'myorg',
  github_token: 'configured',
  webhook_secret: '',
  webhook_url: 'https://myapp.example.com/api/webhooks/github',
  do_api_token: 'configured',
  aws_access_key_id: 'AKIA••••••••EXAMPLE',
  aws_secret_access_key: 'configured',
}

// ─── Route -> mock response map ───────────────────────────────────────────────

/** Only match top-level /api/* paths (not src/api/client.ts module requests) */
function getApiPath(url) {
  try {
    return new URL(url).pathname
  } catch {
    return ''
  }
}

function getMockResponse(url) {
  const p = getApiPath(url)
  if (!p.startsWith('/api/'))             return null  // not an API call
  if (p.startsWith('/api/auth/me'))       return MOCK_USER
  if (p.startsWith('/api/stats'))         return MOCK_STATS
  if (p.startsWith('/api/nodes'))         return MOCK_NODES
  if (p.startsWith('/api/applications'))  return MOCK_APPS
  if (p.startsWith('/api/databases'))     return MOCK_DATABASES
  if (p.startsWith('/api/caches'))        return MOCK_CACHES
  if (p.startsWith('/api/kafkas'))        return MOCK_KAFKAS
  if (p.startsWith('/api/monitorings'))   return MOCK_MONITORINGS
  if (p.startsWith('/api/deployments'))   return MOCK_DEPLOYMENTS
  if (p.startsWith('/api/settings'))      return MOCK_SETTINGS
  return []  // unknown API endpoint — return empty array
}

// ─── Routes ───────────────────────────────────────────────────────────────────

const ROUTES = [
  { path: '/login',       name: 'login',        requiresAuth: false },
  { path: '/',            name: 'dashboard',    requiresAuth: true  },
  { path: '/nodes',       name: 'nodes',        requiresAuth: true  },
  { path: '/applications',name: 'applications', requiresAuth: true  },
  { path: '/databases',   name: 'databases',    requiresAuth: true  },
  { path: '/caches',      name: 'caches',       requiresAuth: true  },
  { path: '/kafkas',      name: 'kafka',        requiresAuth: true  },
  { path: '/monitorings', name: 'monitoring',   requiresAuth: true  },
  { path: '/deployments', name: 'deployments',  requiresAuth: true  },
  { path: '/settings',    name: 'settings',     requiresAuth: true  },
]

const VIEWPORTS = [
  { name: 'desktop', width: 1440, height: 900  },
  { name: 'mobile',  width: 390,  height: 844  },
]

// ─── Helpers ──────────────────────────────────────────────────────────────────

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms))
}

async function waitForServer(url, retries = 30, interval = 1000) {
  for (let i = 0; i < retries; i++) {
    try {
      const res = await fetch(url)
      if (res.ok || res.status < 500) return true
    } catch { /* still starting */ }
    process.stdout.write(i === 0 ? '  Waiting for dev server' : '.')
    await sleep(interval)
  }
  console.log()
  throw new Error(`Dev server at ${url} did not respond after ${retries}s`)
}

// ─── Main ─────────────────────────────────────────────────────────────────────

async function main() {
  mkdirSync(SCREENSHOTS_DIR, { recursive: true })

  // Start dev server
  console.log('\n🚀 Starting Vite dev server...')
  const server = spawn('npm', ['run', 'dev'], {
    cwd: ROOT,
    stdio: 'pipe',
    shell: true,
  })
  server.stderr.on('data', () => {})  // suppress vite stderr noise

  try {
    await waitForServer(`${BASE_URL}/`)
    console.log('\n  Server ready!\n')
  } catch (err) {
    server.kill()
    console.error(err.message)
    process.exit(1)
  }

  // Launch browser
  const browser = await puppeteer.launch({
    headless: true,
    args: ['--no-sandbox', '--disable-setuid-sandbox'],
  })

  let captured = 0
  let failed = 0

  try {
    for (const viewport of VIEWPORTS) {
      console.log(`📐 Viewport: ${viewport.name} (${viewport.width}×${viewport.height})`)
      const vpDir = join(SCREENSHOTS_DIR, viewport.name)
      mkdirSync(vpDir, { recursive: true })

      for (const route of ROUTES) {
        const page = await browser.newPage()
        await page.setViewport({ width: viewport.width, height: viewport.height })

        // Intercept API calls and return mock data
        await page.setRequestInterception(true)
        page.on('request', req => {
          const mock = getMockResponse(req.url())
          if (mock === null) {
            // Not an API call — pass through to Vite dev server
            req.continue()
          } else {
            req.respond({
              status: 200,
              contentType: 'application/json',
              body: JSON.stringify(mock),
            })
          }
        })

        const filename = `${route.name}.png`
        const filepath = join(vpDir, filename)

        try {
          await page.goto(`${BASE_URL}${route.path}`, { waitUntil: 'domcontentloaded', timeout: 15000 })
          // Wait for React to mount into #root
          await page.waitForFunction(
            () => document.querySelector('#root')?.children?.length > 0,
            { timeout: 10000 }
          )
          // Extra wait for fonts, CSS injection, and animations to settle
          await sleep(1500)
          await page.screenshot({ path: filepath, fullPage: true })
          console.log(`  ✅ ${viewport.name}/${filename}`)
          captured++
        } catch (err) {
          console.log(`  ❌ ${viewport.name}/${filename} — ${err.message}`)
          failed++
        } finally {
          await page.close()
        }
      }
      console.log()
    }
  } finally {
    await browser.close()
    server.kill()
  }

  console.log(`\n📸 Done! ${captured} screenshots saved to screenshots/`)
  if (failed > 0) console.log(`   ⚠️  ${failed} failed`)
  console.log(`   📁 screenshots/desktop/   — 1440×900`)
  console.log(`   📁 screenshots/mobile/    — 390×844\n`)
}

main().catch(err => {
  console.error(err)
  process.exit(1)
})
