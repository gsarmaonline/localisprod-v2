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
  provider?: string
  provider_region?: string
  provider_instance_id?: string
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
  dockerfile_path: string
  env_vars: string  // JSON string
  ports: string     // JSON string
  volumes: string   // JSON string
  command: string
  github_repo: string
  domain: string
  databases: string    // JSON string — array of database IDs
  caches: string       // JSON string — array of cache IDs
  kafkas: string       // JSON string — array of kafka cluster IDs
  monitorings: string  // JSON string — array of monitoring stack IDs
  created_at: string
  last_deployed_at?: string
}

export interface CreateApplicationInput {
  name: string
  docker_image: string
  dockerfile_path?: string
  env_vars: Record<string, string>
  ports: string[]
  volumes?: string[]
  command: string
  github_repo?: string
  domain?: string
  databases?: string[]
  caches?: string[]
  kafkas?: string[]
  monitorings?: string[]
}

// Databases
export interface Database {
  id: string
  name: string
  type: string     // postgres
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
  last_deployed_at?: string
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

// Caches
export interface Cache {
  id: string
  name: string
  version: string
  node_id: string
  node_name: string
  node_host: string
  port: number
  volumes: string  // JSON string
  container_name: string
  status: string
  created_at: string
  last_deployed_at?: string
}

export interface CreateCacheInput {
  name: string
  version?: string
  node_id: string
  password: string
  port?: number
  volumes?: string[]
}

export const caches = {
  list: () => request<Cache[]>('/caches'),
  get: (id: string) => request<Cache>(`/caches/${id}`),
  create: (data: CreateCacheInput) =>
    request<Cache>('/caches', { method: 'POST', body: JSON.stringify(data) }),
  delete: (id: string) =>
    request<void>(`/caches/${id}`, { method: 'DELETE' }),
}

// Kafkas
export interface Kafka {
  id: string
  name: string
  version: string
  node_id: string
  node_name: string
  node_host: string
  port: number
  container_name: string
  status: string
  created_at: string
  last_deployed_at?: string
}

export interface CreateKafkaInput {
  name: string
  version?: string
  node_id: string
  port?: number
}

export const kafkas = {
  list: () => request<Kafka[]>('/kafkas'),
  get: (id: string) => request<Kafka>(`/kafkas/${id}`),
  create: (data: CreateKafkaInput) =>
    request<Kafka>('/kafkas', { method: 'POST', body: JSON.stringify(data) }),
  delete: (id: string) =>
    request<void>(`/kafkas/${id}`, { method: 'DELETE' }),
}

// Monitorings
export interface Monitoring {
  id: string
  name: string
  node_id: string
  node_name: string
  node_host: string
  prometheus_port: number
  grafana_port: number
  prometheus_container_name: string
  grafana_container_name: string
  status: string
  created_at: string
  last_deployed_at?: string
}

export interface CreateMonitoringInput {
  name: string
  node_id: string
  prometheus_port?: number
  grafana_port?: number
  grafana_password: string
}

export const monitorings = {
  list: () => request<Monitoring[]>('/monitorings'),
  get: (id: string) => request<Monitoring>(`/monitorings/${id}`),
  create: (data: CreateMonitoringInput) =>
    request<Monitoring>('/monitorings', { method: 'POST', body: JSON.stringify(data) }),
  delete: (id: string) =>
    request<void>(`/monitorings/${id}`, { method: 'DELETE' }),
}

// Object Storages
export interface ObjectStorage {
  id: string
  name: string
  version: string
  node_id: string
  node_name: string
  node_host: string
  s3_port: number
  access_key_id: string
  secret_access_key?: string
  container_name: string
  status: string
  created_at: string
  last_deployed_at?: string
}

export interface CreateObjectStorageInput {
  name: string
  node_id: string
  s3_port?: number
  version?: string
}

export const objectStorages = {
  list: () => request<ObjectStorage[]>('/object-storages'),
  get: (id: string) => request<ObjectStorage>(`/object-storages/${id}`),
  create: (data: CreateObjectStorageInput) =>
    request<ObjectStorage>('/object-storages', { method: 'POST', body: JSON.stringify(data) }),
  delete: (id: string) =>
    request<void>(`/object-storages/${id}`, { method: 'DELETE' }),
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
  do_api_token: string     // "configured" | ""
  aws_access_key_id: string
  aws_secret_access_key: string  // "configured" | ""
}

export const settings = {
  get: () => request<Settings>('/settings'),
  update: (data: Record<string, string>) =>
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
  last_deployed_at?: string
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

// Cloud Providers
export interface DORegion { slug: string; name: string }
export interface DOSize { slug: string; description: string; vcpus: number; memory_mb: number; disk_gb: number; price_monthly: number }
export interface DOImage { slug: string; name: string }
export interface DOMetadata { regions: DORegion[]; sizes: DOSize[]; images: DOImage[] }
export interface DOProvisionInput { name: string; region: string; size: string; image: string }

export interface AWSRegion { id: string; name: string }
export interface AWSInstanceType { id: string; vcpus: number; memory_gib: number; description: string }
export interface AWSOSOption { id: string; name: string }
export interface AWSMetadata { regions: AWSRegion[]; instance_types: AWSInstanceType[]; os_options: AWSOSOption[] }
export interface AWSProvisionInput { name: string; region: string; instance_type: string; os: string }

export const providers = {
  doMetadata: () => request<DOMetadata>('/providers/do/metadata'),
  doProvision: (data: DOProvisionInput) =>
    request<Node>('/providers/do/provision', { method: 'POST', body: JSON.stringify(data) }),
  awsMetadata: () => request<AWSMetadata>('/providers/aws/metadata'),
  awsProvision: (data: AWSProvisionInput) =>
    request<Node>('/providers/aws/provision', { method: 'POST', body: JSON.stringify(data) }),
}

// Docker Compose Import
export interface ComposeParsedApplication {
  name: string
  docker_image: string
  build_path?: string
  ports: string[]
  volumes: string[]
  env_vars: Record<string, string>
  command?: string
  depends_on?: string[]
}

export interface ComposeParsedDatabase {
  name: string
  type: string
  version: string
  port: number
  dbname?: string
  db_user?: string
  env_vars: Record<string, string>
  volumes: string[]
}

export interface ComposeParsedCache {
  name: string
  version: string
  port: number
  volumes: string[]
}

export interface ComposeParsedKafka {
  name: string
  version: string
  port: number
}

export interface ComposeParsedObjectStorage {
  name: string
  version: string
  port: number
}

export interface ComposePreview {
  applications: ComposeParsedApplication[]
  databases: ComposeParsedDatabase[]
  caches: ComposeParsedCache[]
  kafkas: ComposeParsedKafka[]
  object_storages: ComposeParsedObjectStorage[]
}

export const composeImport = {
  preview: (content: string) =>
    request<ComposePreview>('/import/docker-compose', {
      method: 'POST',
      body: JSON.stringify({ content }),
    }),
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
