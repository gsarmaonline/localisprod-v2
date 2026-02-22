const BASE = '/api'

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(body.error || res.statusText)
  }
  if (res.status === 204) return undefined as T
  return res.json()
}

// Nodes
export interface Node {
  id: string
  name: string
  host: string
  port: number
  username: string
  status: string
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
}

// Applications
export interface Application {
  id: string
  name: string
  docker_image: string
  env_vars: string  // JSON string
  ports: string     // JSON string
  command: string
  created_at: string
}

export interface CreateApplicationInput {
  name: string
  docker_image: string
  env_vars: Record<string, string>
  ports: string[]
  command: string
}

export const applications = {
  list: () => request<Application[]>('/applications'),
  get: (id: string) => request<Application>(`/applications/${id}`),
  create: (data: CreateApplicationInput) =>
    request<Application>('/applications', { method: 'POST', body: JSON.stringify(data) }),
  delete: (id: string) =>
    request<void>(`/applications/${id}`, { method: 'DELETE' }),
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
