/**
 * Production end-to-end API test suite for localisprod.com
 *
 * Covers the full user flow:
 *   auth → dashboard → settings → node → application → database → deployment → cleanup
 *
 * Prerequisites:
 *   1. Run `npm run auth` or `npm run auth:ssh` to populate .session
 *   2. Copy .env.test.example to .env.test and fill in TEST_NODE_* values
 *
 * WARNING: This test creates and destroys real resources on the production server.
 *          A best-effort cleanup runs in afterAll even if tests fail midway.
 */
import { describe, it, expect, afterAll } from 'vitest';
import fs from 'fs';
import os from 'os';
import * as api from './client.js';

// ── SSH key helper ───────────────────────────────────────────────────────────
function getPrivateKey(): string {
  if (process.env.TEST_NODE_KEY_B64) {
    return Buffer.from(process.env.TEST_NODE_KEY_B64, 'base64').toString('utf-8');
  }
  const raw = process.env.TEST_NODE_KEY_PATH || '~/.ssh/id_rsa';
  const keyPath = raw.replace(/^~/, os.homedir());
  if (!fs.existsSync(keyPath)) {
    throw new Error(
      `SSH key not found at ${keyPath}.\n` +
      `Set TEST_NODE_KEY_PATH or TEST_NODE_KEY_B64 in .env.test`
    );
  }
  return fs.readFileSync(keyPath, 'utf-8');
}

// ── Shared state (filled in as tests create resources) ───────────────────────
const created = {
  nodeID:       '',
  appID:        '',
  dbID:         '',
  deploymentID: '',
};

// Unique suffix so multiple runs don't collide
const RUN = `ci-${Date.now()}`;

// ── Types ────────────────────────────────────────────────────────────────────
interface WithID   { id: string }
interface NodeResp { id: string; name: string; status: string }
interface AppResp  { id: string; name: string }
interface DBResp   { id: string; name: string; status: string }
interface DepResp  { id: string; status: string; container_name: string }

// ── Global cleanup — runs after all tests, even if they fail ─────────────────
afterAll(async () => {
  const targets = [
    created.deploymentID && `/api/deployments/${created.deploymentID}`,
    created.dbID         && `/api/databases/${created.dbID}`,
    created.appID        && `/api/applications/${created.appID}`,
    created.nodeID       && `/api/nodes/${created.nodeID}`,
  ].filter(Boolean) as string[];

  if (targets.length === 0) return;

  console.log('\n[cleanup] Removing test resources...');
  for (const endpoint of targets) {
    await api.del(endpoint).catch(e =>
      console.warn(`  [cleanup] warning: ${endpoint}: ${(e as Error).message}`)
    );
    console.log(`  [cleanup] deleted ${endpoint}`);
  }
  console.log('[cleanup] Done.\n');
}, 120_000);

// ═══════════════════════════════════════════════════════════════════════════
// 1. Auth
// ═══════════════════════════════════════════════════════════════════════════
describe('auth', () => {
  it('GET /api/auth/me → returns authenticated user', async () => {
    const me = await api.get<{ id: string; email: string; name: string }>('/api/auth/me');
    expect(me.id).toBeTruthy();
    expect(me.email).toContain('@');
    console.log(`  → ${me.name} <${me.email}>`);
  });
});

// ═══════════════════════════════════════════════════════════════════════════
// 2. Dashboard
// ═══════════════════════════════════════════════════════════════════════════
describe('dashboard', () => {
  it('GET /api/stats → returns counts', async () => {
    const stats = await api.get<{
      nodes: number;
      applications: number;
      deployments: Record<string, number> | number;
    }>('/api/stats');
    expect(typeof stats.nodes).toBe('number');
    expect(typeof stats.applications).toBe('number');
    // deployments may be a flat number or a { running: N, failed: N, ... } object
    expect(stats.deployments).toBeDefined();
    const depTotal =
      typeof stats.deployments === 'number'
        ? stats.deployments
        : Object.values(stats.deployments as Record<string, number>).reduce((s, v) => s + v, 0);
    console.log(
      `  → nodes=${stats.nodes}  apps=${stats.applications}  deployments(total)=${depTotal}`
    );
  });
});

// ═══════════════════════════════════════════════════════════════════════════
// 3. Settings
// ═══════════════════════════════════════════════════════════════════════════
describe('settings', () => {
  it('GET /api/settings → returns settings object', async () => {
    const s = await api.get<Record<string, string>>('/api/settings');
    expect(s).toHaveProperty('github_username');
    expect(s).toHaveProperty('github_token');
    expect(s).toHaveProperty('webhook_secret');
  });
});

// ═══════════════════════════════════════════════════════════════════════════
// 4. Nodes
// ═══════════════════════════════════════════════════════════════════════════
describe('nodes', () => {
  it('POST /api/nodes → creates a node', async () => {
    const node = await api.post<NodeResp>('/api/nodes', {
      name:        `${RUN}-node`,
      host:        process.env.TEST_NODE_HOST || '167.71.230.5',
      port:        Number(process.env.TEST_NODE_PORT || 22),
      username:    process.env.TEST_NODE_USER || 'root',
      private_key: getPrivateKey(),
    });
    expect(node.id).toBeTruthy();
    created.nodeID = node.id;
    console.log(`  → id=${node.id}  name=${node.name}`);
  });

  it('GET /api/nodes → lists nodes, includes created one', async () => {
    const nodes = await api.get<WithID[]>('/api/nodes');
    expect(Array.isArray(nodes)).toBe(true);
    expect(nodes.some(n => n.id === created.nodeID)).toBe(true);
  });

  it('GET /api/nodes/:id → returns node by id', async () => {
    const node = await api.get<NodeResp>(`/api/nodes/${created.nodeID}`);
    expect(node.id).toBe(created.nodeID);
    expect(node.name).toContain(RUN);
  });

  it('POST /api/nodes/:id/ping → SSH connection is online', async () => {
    const res = await api.post<{ status: string; message: string }>(
      `/api/nodes/${created.nodeID}/ping`, {}
    );
    expect(res.status).toBe('online');
    console.log(`  → ${res.message}`);
  }, 20_000);
});

// ═══════════════════════════════════════════════════════════════════════════
// 5. Applications
// ═══════════════════════════════════════════════════════════════════════════
describe('applications', () => {
  it('POST /api/applications → creates an app', async () => {
    const app = await api.post<AppResp>('/api/applications', {
      name:         `${RUN}-app`,
      docker_image: 'nginx:alpine',
      ports:        ['18080:80'],
      env_vars:     { APP_ENV: 'ci-test', RUN_ID: RUN },
    });
    expect(app.id).toBeTruthy();
    created.appID = app.id;
    console.log(`  → id=${app.id}  name=${app.name}`);
  });

  it('GET /api/applications → lists apps, includes created one', async () => {
    const apps = await api.get<WithID[]>('/api/applications');
    expect(apps.some(a => a.id === created.appID)).toBe(true);
  });

  it('GET /api/applications/:id → returns app by id', async () => {
    const app = await api.get<AppResp>(`/api/applications/${created.appID}`);
    expect(app.id).toBe(created.appID);
  });

  it('PUT /api/applications/:id → updates app env vars', async () => {
    const updated = await api.put<AppResp>(`/api/applications/${created.appID}`, {
      name:         `${RUN}-app`,
      docker_image: 'nginx:alpine',
      ports:        ['18080:80'],
      env_vars:     { APP_ENV: 'ci-test', RUN_ID: RUN, UPDATED: '1' },
    });
    expect(updated.id).toBe(created.appID);
  });
});

// ═══════════════════════════════════════════════════════════════════════════
// 6. Databases
// ═══════════════════════════════════════════════════════════════════════════
describe('databases', () => {
  it('POST /api/databases → provisions a postgres container on the node', async () => {
    // Use a high non-standard port to avoid collisions with existing postgres containers.
    const DB_PORT = 25432;
    // The handler returns either the DB object directly (201) or a wrapper on docker failure (200).
    type DBCreateResp = DBResp | { database: DBResp; error: string; output?: string };
    const raw = await api.post<DBCreateResp>('/api/databases', {
      name:     `${RUN}-db`,
      type:     'postgres',
      port:     DB_PORT,
      node_id:  created.nodeID,
      password: process.env.TEST_DB_PASSWORD || 'CiTestSecure999!',
    });
    // Unwrap if the handler returned the error-wrapper shape
    const db: DBResp = 'database' in raw
      ? (raw as { database: DBResp }).database
      : (raw as DBResp);
    if ('error' in raw) {
      console.warn(`  [db] docker error: ${(raw as { error: string }).error}`);
    }
    expect(db.id).toBeTruthy();
    expect(db.status).toBe('running');
    created.dbID = db.id;
    console.log(`  → id=${db.id}  status=${db.status}`);
  }, 45_000);

  it('GET /api/databases → lists databases, includes created one', async () => {
    const dbs = await api.get<WithID[]>('/api/databases');
    expect(dbs.some(d => d.id === created.dbID)).toBe(true);
  });

  it('GET /api/databases/:id → returns database, status=running', async () => {
    const db = await api.get<DBResp>(`/api/databases/${created.dbID}`);
    expect(db.id).toBe(created.dbID);
    expect(db.status).toBe('running');
  });
});

// ═══════════════════════════════════════════════════════════════════════════
// 7. Deployments
// ═══════════════════════════════════════════════════════════════════════════
describe('deployments', () => {
  it('POST /api/deployments → pulls and runs the app container', async () => {
    const dep = await api.post<DepResp>('/api/deployments', {
      application_id: created.appID,
      node_id:        created.nodeID,
    });
    expect(dep.id).toBeTruthy();
    expect(dep.status).toBe('running');
    created.deploymentID = dep.id;
    console.log(`  → id=${dep.id}  container=${dep.container_name}  status=${dep.status}`);
  }, 90_000);

  it('GET /api/deployments → lists deployments, includes created one', async () => {
    const deps = await api.get<WithID[]>('/api/deployments');
    expect(deps.some(d => d.id === created.deploymentID)).toBe(true);
  });

  it('GET /api/deployments/:id → returns deployment, status=running', async () => {
    const dep = await api.get<DepResp>(`/api/deployments/${created.deploymentID}`);
    expect(dep.id).toBe(created.deploymentID);
    expect(dep.status).toBe('running');
  });

  it('GET /api/deployments/:id/logs → returns container stdout/stderr', async () => {
    const res = await api.get<{ logs: string }>(
      `/api/deployments/${created.deploymentID}/logs`
    );
    expect(res).toHaveProperty('logs');
    expect(typeof res.logs).toBe('string');
    const preview = res.logs.slice(0, 300).replace(/\n/g, ' ').trim();
    console.log(`  → logs preview: ${preview || '(empty)'}`);
  }, 20_000);

  it('POST /api/deployments/:id/restart → restarts the container', async () => {
    const res = await api.post<{ status: string; message: string }>(
      `/api/deployments/${created.deploymentID}/restart`, {}
    );
    expect(res.status).toBe('running');
    expect(res.message).toBe('container restarted');
    console.log(`  → ${res.message}`);
  }, 30_000);
});
