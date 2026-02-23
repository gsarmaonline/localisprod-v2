const BASE = '/api'

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    ...options,
  })
  if (!res.ok) {
    if (res.status === 401 && path !== '/auth/me') {
      window.location.href = '/login'
      return undefined as T
    }
    const body = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(body.error || res.statusText)
  }
  if (res.status === 204) return undefined as T
  return res.json()
}

// Auth
export interface CurrentUser {
  id: string
  email: string
  name: string
  avatar_url: string
}

export const auth = {
  me: () => request<CurrentUser>('/auth/me'),
  logout: () => request('/auth/logout', { method: 'POST' }),
}

// Nodes
export interface Node {
  id: string
  name: string
  host: string
  port: number
  username: string
  status: string
  traefik_enabled: boolean
  created_at: string
}

export interface CreateNodeInput {
  name: string
  host: string
  port: number
  username: string
  private_key: string
}

export const nodes = {
  list: () => request<Node[]>('/nodes'),
  get: (id: string) => request<Node>(`/nodes/${id}`),
  create: (data: CreateNodeInput) =>
    request<Node>('/nodes', { method: 'POST', body: JSON.stringify(data) }),
  delete: (id: string) =>
    request<void>(`/nodes/${id}`, { method: 'DELETE' }),
  ping: (id: string) =>
    request<{ status: string; message: string }>(`/nodes/${id}/ping`, { method: 'POST' }),
  setupTraefik: (id: string) =>
    request<{ status: string; output: string }>(`/nodes/${id}/setup-traefik`, { method: 'POST' }),
}

// Applications
export interface Application {
  id: string
  name: string
  docker_image: string
  env_vars: string  // JSON string
  ports: string     // JSON string
  command: string
  github_repo: string
  domain: string
  databases: string // JSON string — array of database IDs
  created_at: string
}

export interface CreateApplicationInput {
  name: string
  docker_image: string
  env_vars: Record<string, string>
  ports: string[]
  command: string
  github_repo?: string
  domain?: string
  databases?: string[]
}

// Databases
export interface Database {
  id: string
  name: string
  type: string     // postgres | mysql | redis | mongodb
  version: string
  node_id: string
  node_name: string
  node_host: string
  dbname: string
  db_user: string
  port: number
  container_name: string
  status: string
  created_at: string
}

export interface CreateDatabaseInput {
  name: string
  type: string
  version?: string
  node_id: string
  dbname?: string
  db_user?: string
  password: string
  port?: number
}

export const databases = {
  list: () => request<Database[]>('/databases'),
  get: (id: string) => request<Database>(`/databases/${id}`),
  create: (data: CreateDatabaseInput) =>
    request<Database>('/databases', { method: 'POST', body: JSON.stringify(data) }),
  delete: (id: string) =>
    request<void>(`/databases/${id}`, { method: 'DELETE' }),
}

export const applications = {
  list: () => request<Application[]>('/applications'),
  get: (id: string) => request<Application>(`/applications/${id}`),
  create: (data: CreateApplicationInput) =>
    request<Application>('/applications', { method: 'POST', body: JSON.stringify(data) }),
  update: (id: string, data: CreateApplicationInput) =>
    request<Application>(`/applications/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  delete: (id: string) =>
    request<void>(`/applications/${id}`, { method: 'DELETE' }),
}

// GitHub
export interface GithubRepo {
  name: string
  full_name: string
  description: string
  private: boolean
  html_url: string
}

export const github = {
  listRepos: () => request<GithubRepo[]>('/github/repos'),
}

// Settings
export interface Settings {
  github_username: string
  github_token: string     // "configured" | ""
  webhook_secret: string   // "configured" | ""
  webhook_url: string
}

export const settings = {
  get: () => request<Settings>('/settings'),
  update: (data: { github_username: string; github_token: string; webhook_secret?: string }) =>
    request<{ status: string }>('/settings', { method: 'PUT', body: JSON.stringify(data) }),
}

// Deployments
export interface Deployment {
  id: string
  application_id: string
  node_id: string
  container_name: string
  container_id: string
  status: string
  created_at: string
  app_name?: string
  node_name?: string
  docker_image?: string
}

export interface CreateDeploymentInput {
  application_id: string
  node_id: string
}

export const deployments = {
  list: () => request<Deployment[]>('/deployments'),
  get: (id: string) => request<Deployment>(`/deployments/${id}`),
  create: (data: CreateDeploymentInput) =>
    request<Deployment>('/deployments', { method: 'POST', body: JSON.stringify(data) }),
  delete: (id: string) =>
    request<void>(`/deployments/${id}`, { method: 'DELETE' }),
  restart: (id: string) =>
    request<{ status: string; message: string }>(`/deployments/${id}/restart`, { method: 'POST' }),
  logs: (id: string) =>
    request<{ logs: string; error?: string }>(`/deployments/${id}/logs`),
}

// Dashboard
export interface Stats {
  nodes: number
  applications: number
  deployments: Record<string, number>
}

export const dashboard = {
  stats: () => request<Stats>('/stats'),
}
