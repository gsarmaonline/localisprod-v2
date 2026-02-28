import { useState } from 'react'
import Modal from './Modal'
import {
  composeImport,
  ComposePreview,
  ComposeParsedService,
  ComposeParsedDatabase,
  ComposeParsedCache,
  ComposeParsedKafka,
  Node,
  databases,
  caches,
  kafkas,
  services,
  CreateServiceInput,
} from '../api/client'

interface Props {
  nodes: Node[]
  onClose: () => void
  onDone: () => void
}

type Step = 'paste' | 'review' | 'creating' | 'done'

// Per-resource overrides the user fills in during review
interface DbOverride {
  node_id: string
  password: string
  name: string
  port: string
}
interface CacheOverride {
  node_id: string
  password: string
  name: string
  port: string
}
interface KafkaOverride {
  node_id: string
  name: string
  port: string
}
interface AppOverride {
  name: string
  docker_image: string
}

export default function ComposeImportWizard({ nodes, onClose, onDone }: Props) {
  const [step, setStep] = useState<Step>('paste')
  const [content, setContent] = useState('')
  const [preview, setPreview] = useState<ComposePreview | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [parsing, setParsing] = useState(false)

  const [dbOverrides, setDbOverrides] = useState<DbOverride[]>([])
  const [cacheOverrides, setCacheOverrides] = useState<CacheOverride[]>([])
  const [kafkaOverrides, setKafkaOverrides] = useState<KafkaOverride[]>([])
  const [appOverrides, setAppOverrides] = useState<AppOverride[]>([])

  const [createLog, setCreateLog] = useState<string[]>([])
  const [createError, setCreateError] = useState<string | null>(null)

  const handleParse = async () => {
    setError(null)
    setParsing(true)
    try {
      const p = await composeImport.preview(content)
      setPreview(p)
      setDbOverrides((p.databases ?? []).map(db => ({
        node_id: nodes[0]?.id ?? '',
        password: db.env_vars?.['POSTGRES_PASSWORD'] ?? db.env_vars?.['MYSQL_ROOT_PASSWORD'] ?? '',
        name: db.name,
        port: String(db.port),
      })))
      setCacheOverrides((p.caches ?? []).map(c => ({
        node_id: nodes[0]?.id ?? '',
        password: '',
        name: c.name,
        port: String(c.port),
      })))
      setKafkaOverrides((p.kafkas ?? []).map(k => ({
        node_id: nodes[0]?.id ?? '',
        name: k.name,
        port: String(k.port),
      })))
      setAppOverrides((p.services ?? []).map(a => ({
        name: a.name,
        docker_image: a.docker_image ?? '',
      })))
      setStep('review')
    } catch (e: unknown) {
      setError((e as Error).message)
    } finally {
      setParsing(false)
    }
  }

  const handleCreate = async () => {
    if (!preview) return
    setStep('creating')
    setCreateLog([])
    setCreateError(null)

    const log = (msg: string) => setCreateLog(prev => [...prev, msg])

    try {
      // 1. Create databases
      const createdDbIds: string[] = []
      for (let i = 0; i < (preview.databases ?? []).length; i++) {
        const db = preview.databases[i]
        const ov = dbOverrides[i]
        log(`Creating database "${ov.name}" (${db.type})…`)
        const result = await databases.create({
          name: ov.name,
          type: db.type,
          version: db.version || undefined,
          node_id: ov.node_id,
          dbname: db.dbname || undefined,
          db_user: db.db_user || undefined,
          password: ov.password,
          port: ov.port ? parseInt(ov.port) : undefined,
        })
        createdDbIds.push(result.id)
        log(`  ✓ Database "${ov.name}" created`)
      }

      // 2. Create caches
      const createdCacheIds: string[] = []
      for (let i = 0; i < (preview.caches ?? []).length; i++) {
        const c = preview.caches[i]
        const ov = cacheOverrides[i]
        log(`Creating cache "${ov.name}"…`)
        const result = await caches.create({
          name: ov.name,
          version: c.version || undefined,
          node_id: ov.node_id,
          password: ov.password,
          port: ov.port ? parseInt(ov.port) : undefined,
          volumes: c.volumes?.length > 0 ? c.volumes : undefined,
        })
        createdCacheIds.push(result.id)
        log(`  ✓ Cache "${ov.name}" created`)
      }

      // 3. Create kafkas
      const createdKafkaIds: string[] = []
      for (let i = 0; i < (preview.kafkas ?? []).length; i++) {
        const k = preview.kafkas[i]
        const ov = kafkaOverrides[i]
        log(`Creating Kafka "${ov.name}"…`)
        const result = await kafkas.create({
          name: ov.name,
          version: k.version || undefined,
          node_id: ov.node_id,
          port: ov.port ? parseInt(ov.port) : undefined,
        })
        createdKafkaIds.push(result.id)
        log(`  ✓ Kafka "${ov.name}" created`)
      }

      // 4. Create services — link all resources to apps that depend on them
      for (let i = 0; i < (preview.services ?? []).length; i++) {
        const app = preview.services[i]
        const ov = appOverrides[i]
        log(`Creating service "${ov.name}"…`)

        // Determine linked resource IDs based on depends_on service names
        const deps = new Set(app.depends_on ?? [])
        const linkedDbs = createdDbIds.filter((_, j) => {
          const dbName = (preview.databases ?? [])[j]?.name
          return deps.size === 0 || deps.has(dbName)
        })
        const linkedCaches = createdCacheIds.filter((_, j) => {
          const cacheName = (preview.caches ?? [])[j]?.name
          return deps.size === 0 || deps.has(cacheName)
        })
        const linkedKafkas = createdKafkaIds.filter((_, j) => {
          const kafkaName = (preview.kafkas ?? [])[j]?.name
          return deps.size === 0 || deps.has(kafkaName)
        })

        const envVars: Record<string, string> = { ...(app.env_vars ?? {}) }

        const data: CreateServiceInput = {
          name: ov.name,
          docker_image: ov.docker_image,
          dockerfile_path: app.build_path || undefined,
          env_vars: envVars,
          ports: (app.ports ?? []).filter(Boolean),
          volumes: (app.volumes ?? []).filter(Boolean).length > 0 ? app.volumes.filter(Boolean) : undefined,
          command: app.command || '',
          databases: linkedDbs.length > 0 ? linkedDbs : undefined,
          caches: linkedCaches.length > 0 ? linkedCaches : undefined,
          kafkas: linkedKafkas.length > 0 ? linkedKafkas : undefined,
        }
        await services.create(data)
        log(`  ✓ Service "${ov.name}" created`)
      }

      log('All objects created successfully.')
      setStep('done')
    } catch (e: unknown) {
      setCreateError((e as Error).message)
    }
  }

  const totalItems =
    (preview?.services?.length ?? 0) +
    (preview?.databases?.length ?? 0) +
    (preview?.caches?.length ?? 0) +
    (preview?.kafkas?.length ?? 0) +
    (preview?.object_storages?.length ?? 0)

  return (
    <Modal title="Import docker-compose.yml" onClose={onClose}>
      {step === 'paste' && (
        <div className="space-y-4">
          <p className="text-sm text-gray-500">
            Paste your <code className="font-mono bg-gray-100 px-1 rounded">docker-compose.yml</code> below.
            Services will be automatically classified into Services, Databases, Caches, and Kafkas.
          </p>
          <textarea
            className="w-full border rounded-lg px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-purple-500 resize-y"
            rows={14}
            placeholder={"version: '3.8'\nservices:\n  api:\n    build: ./api\n    ports:\n      - '8080:80'\n  db:\n    image: postgres:16\n    ..."}
            value={content}
            onChange={e => setContent(e.target.value)}
          />
          {error && <p className="text-sm text-red-600">{error}</p>}
          <div className="flex justify-end gap-2">
            <button onClick={onClose} className="px-4 py-2 text-sm text-gray-600 hover:text-gray-900">Cancel</button>
            <button
              onClick={handleParse}
              disabled={parsing || !content.trim()}
              className="px-4 py-2 bg-purple-600 text-white text-sm rounded-lg hover:bg-purple-700 disabled:opacity-50"
            >
              {parsing ? 'Parsing…' : 'Parse'}
            </button>
          </div>
        </div>
      )}

      {step === 'review' && preview && (
        <div className="space-y-5 max-h-[70vh] overflow-y-auto pr-1">
          <p className="text-sm text-gray-500">
            Found <strong>{totalItems}</strong> service{totalItems !== 1 ? 's' : ''}.
            Review the details below, assign nodes and passwords, then click <strong>Create All</strong>.
          </p>

          {/* Services */}
          {(preview.services ?? []).length > 0 && (
            <section>
              <h3 className="text-sm font-semibold text-gray-700 mb-2">
                Services ({preview.services.length})
              </h3>
              <div className="space-y-2">
                {preview.services.map((app, i) => (
                  <AppReviewCard
                    key={i}
                    app={app}
                    override={appOverrides[i]}
                    onChange={ov => setAppOverrides(prev => prev.map((o, j) => j === i ? ov : o))}
                  />
                ))}
              </div>
            </section>
          )}

          {/* Databases */}
          {(preview.databases ?? []).length > 0 && (
            <section>
              <h3 className="text-sm font-semibold text-gray-700 mb-2">
                Databases ({preview.databases.length})
              </h3>
              <div className="space-y-2">
                {preview.databases.map((db, i) => (
                  <DbReviewCard
                    key={i}
                    db={db}
                    override={dbOverrides[i]}
                    nodes={nodes}
                    onChange={ov => setDbOverrides(prev => prev.map((o, j) => j === i ? ov : o))}
                  />
                ))}
              </div>
            </section>
          )}

          {/* Caches */}
          {(preview.caches ?? []).length > 0 && (
            <section>
              <h3 className="text-sm font-semibold text-gray-700 mb-2">
                Caches ({preview.caches.length})
              </h3>
              <div className="space-y-2">
                {preview.caches.map((c, i) => (
                  <CacheReviewCard
                    key={i}
                    cache={c}
                    override={cacheOverrides[i]}
                    nodes={nodes}
                    onChange={ov => setCacheOverrides(prev => prev.map((o, j) => j === i ? ov : o))}
                  />
                ))}
              </div>
            </section>
          )}

          {/* Kafkas */}
          {(preview.kafkas ?? []).length > 0 && (
            <section>
              <h3 className="text-sm font-semibold text-gray-700 mb-2">
                Kafka ({preview.kafkas.length})
              </h3>
              <div className="space-y-2">
                {preview.kafkas.map((k, i) => (
                  <KafkaReviewCard
                    key={i}
                    kafka={k}
                    override={kafkaOverrides[i]}
                    nodes={nodes}
                    onChange={ov => setKafkaOverrides(prev => prev.map((o, j) => j === i ? ov : o))}
                  />
                ))}
              </div>
            </section>
          )}

          {/* Object Storages (info only — not yet created) */}
          {(preview.object_storages ?? []).length > 0 && (
            <section>
              <h3 className="text-sm font-semibold text-gray-700 mb-2">
                Object Storage ({preview.object_storages.length}) — <span className="font-normal text-gray-400">create manually in the Object Storage page</span>
              </h3>
              {preview.object_storages.map((o, i) => (
                <div key={i} className="text-sm text-gray-500 font-mono px-3 py-1 bg-gray-50 rounded border mb-1">
                  {o.name} — {o.version} — port {o.port}
                </div>
              ))}
            </section>
          )}

          <div className="flex justify-between pt-2 border-t">
            <button onClick={() => setStep('paste')} className="px-4 py-2 text-sm text-gray-600 hover:text-gray-900">← Back</button>
            <button
              onClick={handleCreate}
              className="px-4 py-2 bg-purple-600 text-white text-sm rounded-lg hover:bg-purple-700"
            >
              Create All
            </button>
          </div>
        </div>
      )}

      {step === 'creating' && (
        <div className="space-y-3">
          <p className="text-sm font-medium text-gray-700">Creating resources…</p>
          <div className="bg-gray-50 rounded-lg p-3 font-mono text-xs space-y-1 max-h-64 overflow-y-auto">
            {createLog.map((line, i) => (
              <div key={i} className="text-gray-700">{line}</div>
            ))}
          </div>
          {createError && (
            <p className="text-sm text-red-600">{createError}</p>
          )}
        </div>
      )}

      {step === 'done' && (
        <div className="space-y-4">
          <div className="bg-gray-50 rounded-lg p-3 font-mono text-xs space-y-1 max-h-48 overflow-y-auto">
            {createLog.map((line, i) => (
              <div key={i} className="text-gray-700">{line}</div>
            ))}
          </div>
          <div className="flex justify-end">
            <button
              onClick={onDone}
              className="px-4 py-2 bg-purple-600 text-white text-sm rounded-lg hover:bg-purple-700"
            >
              Done
            </button>
          </div>
        </div>
      )}
    </Modal>
  )
}

// ---- sub-cards ----

function AppReviewCard({ app, override, onChange }: {
  app: ComposeParsedService
  override: AppOverride
  onChange: (ov: AppOverride) => void
}) {
  return (
    <div className="border rounded-lg p-3 bg-white space-y-2">
      <div className="flex items-center gap-2">
        <span className="px-2 py-0.5 text-xs rounded bg-purple-100 text-purple-700 font-medium">app</span>
        <span className="text-sm font-medium text-gray-800">{app.name}</span>
        {app.build_path && <span className="text-xs text-gray-400">build: {app.build_path}</span>}
      </div>
      <div className="grid grid-cols-2 gap-2">
        <div>
          <label className="block text-xs text-gray-500 mb-0.5">Name</label>
          <input
            className="w-full border rounded px-2 py-1 text-xs font-mono focus:outline-none focus:ring-1 focus:ring-purple-400"
            value={override.name}
            onChange={e => onChange({ ...override, name: e.target.value })}
          />
        </div>
        <div>
          <label className="block text-xs text-gray-500 mb-0.5">Docker Image</label>
          <input
            className="w-full border rounded px-2 py-1 text-xs font-mono focus:outline-none focus:ring-1 focus:ring-purple-400"
            value={override.docker_image}
            placeholder="image:tag"
            onChange={e => onChange({ ...override, docker_image: e.target.value })}
          />
        </div>
      </div>
      {app.ports?.length > 0 && (
        <p className="text-xs text-gray-400">Ports: {app.ports.join(', ')}</p>
      )}
      {app.depends_on && app.depends_on.length > 0 && (
        <p className="text-xs text-gray-400">Depends on: {app.depends_on.join(', ')}</p>
      )}
    </div>
  )
}

function DbReviewCard({ db, override, nodes, onChange }: {
  db: ComposeParsedDatabase
  override: DbOverride
  nodes: Node[]
  onChange: (ov: DbOverride) => void
}) {
  return (
    <div className="border rounded-lg p-3 bg-white space-y-2">
      <div className="flex items-center gap-2">
        <span className="px-2 py-0.5 text-xs rounded bg-blue-100 text-blue-700 font-medium">{db.type}</span>
        <span className="text-sm font-medium text-gray-800">{db.name}</span>
        <span className="text-xs text-gray-400">{db.version}</span>
      </div>
      <div className="grid grid-cols-2 gap-2">
        <div>
          <label className="block text-xs text-gray-500 mb-0.5">Name</label>
          <input
            className="w-full border rounded px-2 py-1 text-xs font-mono focus:outline-none focus:ring-1 focus:ring-blue-400"
            value={override.name}
            onChange={e => onChange({ ...override, name: e.target.value })}
          />
        </div>
        <div>
          <label className="block text-xs text-gray-500 mb-0.5">Port</label>
          <input
            type="number"
            className="w-full border rounded px-2 py-1 text-xs font-mono focus:outline-none focus:ring-1 focus:ring-blue-400"
            value={override.port}
            onChange={e => onChange({ ...override, port: e.target.value })}
          />
        </div>
        <div>
          <label className="block text-xs text-gray-500 mb-0.5">Node <span className="text-red-400">*</span></label>
          <select
            className="w-full border rounded px-2 py-1 text-xs focus:outline-none focus:ring-1 focus:ring-blue-400"
            value={override.node_id}
            onChange={e => onChange({ ...override, node_id: e.target.value })}
          >
            <option value="">Select node…</option>
            {nodes.map(n => <option key={n.id} value={n.id}>{n.name}</option>)}
          </select>
        </div>
        <div>
          <label className="block text-xs text-gray-500 mb-0.5">Password <span className="text-red-400">*</span></label>
          <input
            type="password"
            className="w-full border rounded px-2 py-1 text-xs font-mono focus:outline-none focus:ring-1 focus:ring-blue-400"
            value={override.password}
            onChange={e => onChange({ ...override, password: e.target.value })}
          />
        </div>
      </div>
    </div>
  )
}

function CacheReviewCard({ cache, override, nodes, onChange }: {
  cache: ComposeParsedCache
  override: CacheOverride
  nodes: Node[]
  onChange: (ov: CacheOverride) => void
}) {
  return (
    <div className="border rounded-lg p-3 bg-white space-y-2">
      <div className="flex items-center gap-2">
        <span className="px-2 py-0.5 text-xs rounded bg-red-100 text-red-700 font-medium">redis</span>
        <span className="text-sm font-medium text-gray-800">{cache.name}</span>
        <span className="text-xs text-gray-400">{cache.version}</span>
      </div>
      <div className="grid grid-cols-2 gap-2">
        <div>
          <label className="block text-xs text-gray-500 mb-0.5">Name</label>
          <input
            className="w-full border rounded px-2 py-1 text-xs font-mono focus:outline-none focus:ring-1 focus:ring-red-400"
            value={override.name}
            onChange={e => onChange({ ...override, name: e.target.value })}
          />
        </div>
        <div>
          <label className="block text-xs text-gray-500 mb-0.5">Port</label>
          <input
            type="number"
            className="w-full border rounded px-2 py-1 text-xs font-mono focus:outline-none focus:ring-1 focus:ring-red-400"
            value={override.port}
            onChange={e => onChange({ ...override, port: e.target.value })}
          />
        </div>
        <div>
          <label className="block text-xs text-gray-500 mb-0.5">Node <span className="text-red-400">*</span></label>
          <select
            className="w-full border rounded px-2 py-1 text-xs focus:outline-none focus:ring-1 focus:ring-red-400"
            value={override.node_id}
            onChange={e => onChange({ ...override, node_id: e.target.value })}
          >
            <option value="">Select node…</option>
            {nodes.map(n => <option key={n.id} value={n.id}>{n.name}</option>)}
          </select>
        </div>
        <div>
          <label className="block text-xs text-gray-500 mb-0.5">Password <span className="text-red-400">*</span></label>
          <input
            type="password"
            className="w-full border rounded px-2 py-1 text-xs font-mono focus:outline-none focus:ring-1 focus:ring-red-400"
            value={override.password}
            onChange={e => onChange({ ...override, password: e.target.value })}
          />
        </div>
      </div>
    </div>
  )
}

function KafkaReviewCard({ kafka, override, nodes, onChange }: {
  kafka: ComposeParsedKafka
  override: KafkaOverride
  nodes: Node[]
  onChange: (ov: KafkaOverride) => void
}) {
  return (
    <div className="border rounded-lg p-3 bg-white space-y-2">
      <div className="flex items-center gap-2">
        <span className="px-2 py-0.5 text-xs rounded bg-yellow-100 text-yellow-700 font-medium">kafka</span>
        <span className="text-sm font-medium text-gray-800">{kafka.name}</span>
        <span className="text-xs text-gray-400">{kafka.version}</span>
      </div>
      <div className="grid grid-cols-3 gap-2">
        <div>
          <label className="block text-xs text-gray-500 mb-0.5">Name</label>
          <input
            className="w-full border rounded px-2 py-1 text-xs font-mono focus:outline-none focus:ring-1 focus:ring-yellow-400"
            value={override.name}
            onChange={e => onChange({ ...override, name: e.target.value })}
          />
        </div>
        <div>
          <label className="block text-xs text-gray-500 mb-0.5">Port</label>
          <input
            type="number"
            className="w-full border rounded px-2 py-1 text-xs font-mono focus:outline-none focus:ring-1 focus:ring-yellow-400"
            value={override.port}
            onChange={e => onChange({ ...override, port: e.target.value })}
          />
        </div>
        <div>
          <label className="block text-xs text-gray-500 mb-0.5">Node <span className="text-red-400">*</span></label>
          <select
            className="w-full border rounded px-2 py-1 text-xs focus:outline-none focus:ring-1 focus:ring-yellow-400"
            value={override.node_id}
            onChange={e => onChange({ ...override, node_id: e.target.value })}
          >
            <option value="">Select node…</option>
            {nodes.map(n => <option key={n.id} value={n.id}>{n.name}</option>)}
          </select>
        </div>
      </div>
    </div>
  )
}
