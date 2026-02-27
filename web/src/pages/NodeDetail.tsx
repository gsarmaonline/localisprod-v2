import { useCallback, useEffect, useRef, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { nodes, Node, volumeMigration, NodeVolumeMigration } from '../api/client'
import StatusBadge from '../components/StatusBadge'

type Tab = 'overview' | 'disk'

// Ordered migration steps for the progress stepper
const MIGRATION_STEPS = [
  { key: 'provisioning', label: 'Create volume' },
  { key: 'provisioned',  label: 'Attach volume' },
  { key: 'mounted',      label: 'Format & mount' },
  { key: 'synced',       label: 'Sync data' },
  { key: 'stopping',     label: 'Stop containers' },
  { key: 'renamed',      label: 'Rename dir' },
  { key: 'symlinked',    label: 'Create symlink' },
  { key: 'restarting',   label: 'Restart containers' },
  { key: 'verified',     label: 'Verify health' },
  { key: 'completed',    label: 'Done' },
]

const TERMINAL = new Set(['completed', 'rolled_back', 'failed'])
const ACTIVE   = new Set([
  'pending', 'provisioning', 'provisioned', 'mounted', 'synced',
  'stopping', 'renamed', 'symlinked', 'restarting', 'verified', 'rolling_back',
])

function stepIndex(status: string): number {
  return MIGRATION_STEPS.findIndex(s => s.key === status)
}

function MigrationStepper({ status }: { status: string }) {
  const current = stepIndex(status)
  const failed  = status === 'failed'
  const rolled  = status === 'rolled_back'

  return (
    <div className="mt-4">
      <div className="flex items-center gap-0 flex-wrap">
        {MIGRATION_STEPS.map((step, i) => {
          const done    = current > i || status === 'completed'
          const active  = current === i && !TERMINAL.has(status)
          const isFailed = failed && current === i

          return (
            <div key={step.key} className="flex items-center">
              <div className="flex flex-col items-center">
                <div className={`w-7 h-7 rounded-full flex items-center justify-center text-xs font-bold border-2 transition-all
                  ${done      ? 'bg-green-500 border-green-500 text-white'
                  : active    ? 'bg-blue-500 border-blue-500 text-white animate-pulse'
                  : isFailed  ? 'bg-red-500 border-red-500 text-white'
                  : rolled    ? 'bg-gray-200 border-gray-300 text-gray-400'
                  :             'bg-white border-gray-300 text-gray-400'}`}
                >
                  {done ? '✓' : isFailed ? '✗' : i + 1}
                </div>
                <span className={`text-xs mt-1 text-center w-16 leading-tight
                  ${done ? 'text-green-600' : active ? 'text-blue-600 font-medium' : 'text-gray-400'}`}
                >
                  {step.label}
                </span>
              </div>
              {i < MIGRATION_STEPS.length - 1 && (
                <div className={`h-0.5 w-4 mb-4 mx-0.5 flex-shrink-0 ${done ? 'bg-green-400' : 'bg-gray-200'}`} />
              )}
            </div>
          )
        })}
      </div>

      {rolled && (
        <div className="mt-3 p-2 bg-yellow-50 border border-yellow-200 rounded text-sm text-yellow-800">
          Migration was rolled back. You can try again.
        </div>
      )}
    </div>
  )
}

function DiskTab({ node }: { node: Node }) {
  const [migration, setMigration]   = useState<NodeVolumeMigration | null>(null)
  const [loading, setLoading]       = useState(true)
  const [actionLoading, setAction]  = useState<string | null>(null)
  const [error, setError]           = useState<string | null>(null)
  const pollerRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const supportsBlockStorage = !!(node.provider && node.provider_instance_id &&
    (node.provider === 'aws' || node.provider === 'digitalocean'))

  const fetchMigration = useCallback(async () => {
    try {
      const m = await volumeMigration.get(node.id)
      setMigration(m)
      return m
    } catch {
      setMigration(null)
      return null
    }
  }, [node.id])

  // Initial load
  useEffect(() => {
    setLoading(true)
    fetchMigration().finally(() => setLoading(false))
  }, [fetchMigration])

  // Poll while migration is active
  useEffect(() => {
    if (pollerRef.current) clearInterval(pollerRef.current)
    if (migration && ACTIVE.has(migration.status)) {
      pollerRef.current = setInterval(() => fetchMigration(), 3000)
    }
    return () => { if (pollerRef.current) clearInterval(pollerRef.current) }
  }, [migration?.status, fetchMigration])

  const handleStart = async () => {
    setError(null)
    setAction('start')
    try {
      const m = await volumeMigration.start(node.id)
      setMigration(m)
    } catch (e: unknown) {
      setError((e as Error).message)
    } finally {
      setAction(null)
    }
  }

  const handleRollback = async () => {
    if (!confirm('Roll back the volume migration? This will restore the original Docker volumes directory.')) return
    setError(null)
    setAction('rollback')
    try {
      await volumeMigration.rollback(node.id)
      await fetchMigration()
    } catch (e: unknown) {
      setError((e as Error).message)
    } finally {
      setAction(null)
    }
  }

  const handleDeleteBak = async () => {
    if (!confirm('Permanently delete /var/lib/docker/volumes.bak on this node? This cannot be undone.')) return
    setError(null)
    setAction('deletebak')
    try {
      await volumeMigration.deleteBak(node.id)
      await fetchMigration()
    } catch (e: unknown) {
      setError((e as Error).message)
    } finally {
      setAction(null)
    }
  }

  if (!supportsBlockStorage) {
    return (
      <div className="p-6 text-center text-gray-500">
        <div className="text-4xl mb-3">💽</div>
        <p className="font-medium text-gray-700">Block storage not available</p>
        <p className="text-sm mt-1">This node was not provisioned through a cloud provider.<br />Block storage migration requires an AWS or DigitalOcean node.</p>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Block Volume section */}
      <div className="bg-white rounded-xl border shadow-sm p-6">
        <h3 className="text-base font-semibold text-gray-900 mb-1">Block Volume Migration</h3>
        <p className="text-sm text-gray-500 mb-4">
          Migrate Docker volumes to a provider-backed block volume ({node.provider === 'aws' ? 'AWS EBS' : 'DigitalOcean Volume'}, 20 GB).
          Data is synced before cutover — no downtime on healthy containers.
        </p>

        {error && (
          <div className="mb-4 p-3 bg-red-50 text-red-700 rounded-lg text-sm">
            {error}
            <button onClick={() => setError(null)} className="ml-2 underline">dismiss</button>
          </div>
        )}

        {loading ? (
          <p className="text-sm text-gray-400">Loading...</p>
        ) : !migration || TERMINAL.has(migration.status) && migration.status === 'rolled_back' ? (
          // No migration or rolled back → show start button
          <div>
            {(!migration || migration.status === 'rolled_back') && (
              <button
                onClick={handleStart}
                disabled={actionLoading === 'start'}
                className="px-4 py-2 bg-indigo-600 text-white text-sm rounded-lg hover:bg-indigo-700 disabled:opacity-50 font-medium"
              >
                {actionLoading === 'start' ? 'Starting…' : 'Migrate Volumes'}
              </button>
            )}
          </div>
        ) : null}

        {migration && (
          <div>
            {/* Status row */}
            <div className="flex items-center gap-3 mb-2">
              <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium
                ${migration.status === 'completed'   ? 'bg-green-100 text-green-800'
                : migration.status === 'failed'       ? 'bg-red-100 text-red-800'
                : migration.status === 'rolled_back'  ? 'bg-yellow-100 text-yellow-800'
                : migration.status === 'rolling_back' ? 'bg-orange-100 text-orange-800'
                :                                       'bg-blue-100 text-blue-800'}`}
              >
                {migration.status}
              </span>
              {migration.provider_volume_id && (
                <span className="text-xs text-gray-500 font-mono">{migration.provider_volume_id}</span>
              )}
              {migration.device_path && (
                <span className="text-xs text-gray-500 font-mono">{migration.device_path}</span>
              )}
            </div>

            {/* Stepper (hide on rolled_back to avoid confusion) */}
            {migration.status !== 'rolled_back' && (
              <MigrationStepper status={migration.status} />
            )}

            {/* Error detail */}
            {migration.status === 'failed' && migration.error && (
              <div className="mt-3 p-3 bg-red-50 border border-red-200 rounded text-xs text-red-800 font-mono whitespace-pre-wrap">
                {migration.error}
              </div>
            )}

            {/* Actions */}
            <div className="mt-5 flex flex-wrap gap-2">
              {/* Re-migrate after failure or rollback */}
              {(migration.status === 'failed' || migration.status === 'rolled_back') && (
                <button
                  onClick={handleStart}
                  disabled={actionLoading === 'start'}
                  className="px-4 py-2 bg-indigo-600 text-white text-sm rounded-lg hover:bg-indigo-700 disabled:opacity-50 font-medium"
                >
                  {actionLoading === 'start' ? 'Starting…' : 'Retry Migration'}
                </button>
              )}

              {/* Rollback while in progress (non-terminal, non-rolling_back) */}
              {ACTIVE.has(migration.status) && migration.status !== 'rolling_back' && (
                <button
                  onClick={handleRollback}
                  disabled={actionLoading === 'rollback'}
                  className="px-4 py-2 bg-red-50 text-red-700 text-sm rounded-lg hover:bg-red-100 border border-red-200 disabled:opacity-50"
                >
                  {actionLoading === 'rollback' ? 'Rolling back…' : 'Rollback'}
                </button>
              )}

              {/* Delete .bak after completion */}
              {migration.status === 'completed' && (
                <button
                  onClick={handleDeleteBak}
                  disabled={actionLoading === 'deletebak'}
                  className="px-4 py-2 bg-gray-50 text-gray-700 text-sm rounded-lg hover:bg-gray-100 border border-gray-200 disabled:opacity-50"
                >
                  {actionLoading === 'deletebak' ? 'Deleting…' : 'Delete .bak'}
                </button>
              )}
            </div>

            {migration.status === 'completed' && (
              <p className="mt-3 text-xs text-gray-500">
                /var/lib/docker/volumes is now a symlink to {migration.mount_path}/volumes.
                The original <code>.bak</code> directory is kept for 24 h as a safety net — delete it when you are confident everything works.
              </p>
            )}
          </div>
        )}
      </div>

      {/* Snapshots placeholder */}
      <div className="bg-white rounded-xl border shadow-sm p-6">
        <h3 className="text-base font-semibold text-gray-900 mb-1">Snapshots</h3>
        <p className="text-sm text-gray-400">Coming soon — on-demand and scheduled snapshots of the attached block volume.</p>
      </div>
    </div>
  )
}

function OverviewTab({ node }: { node: Node }) {
  const [pingMsg, setPingMsg]         = useState<string | null>(null)
  const [traefikOut, setTraefikOut]   = useState<{ status: string; output: string } | null>(null)
  const [pinging, setPinging]         = useState(false)
  const [traefik, setTraefik]         = useState(false)

  const handlePing = async () => {
    setPinging(true)
    try {
      const r = await nodes.ping(node.id)
      setPingMsg(`${r.status}: ${r.message}`)
    } catch (e: unknown) {
      setPingMsg((e as Error).message)
    } finally {
      setPinging(false)
    }
  }

  const handleTraefik = async () => {
    setTraefik(true)
    try {
      const r = await nodes.setupTraefik(node.id)
      setTraefikOut(r)
    } catch (e: unknown) {
      setTraefikOut({ status: 'error', output: (e as Error).message })
    } finally {
      setTraefik(false)
    }
  }

  const rows: [string, string | number | boolean | undefined][] = [
    ['Host',        node.host],
    ['Port',        node.port],
    ['Username',    node.username],
    ['Provider',    node.provider || '—'],
    ['Region',      node.provider_region || '—'],
    ['Instance ID', node.provider_instance_id || '—'],
    ['Traefik',     node.traefik_enabled ? 'Enabled' : 'Disabled'],
    ['Created',     new Date(node.created_at).toLocaleString()],
  ]

  return (
    <div className="bg-white rounded-xl border shadow-sm divide-y">
      {rows.map(([label, value]) => (
        <div key={label} className="flex items-center px-6 py-3 gap-4">
          <span className="w-32 text-sm text-gray-500 flex-shrink-0">{label}</span>
          <span className="text-sm font-mono text-gray-800">{String(value)}</span>
        </div>
      ))}
      <div className="px-6 py-4 flex flex-wrap gap-2 items-center">
        <button
          onClick={handlePing}
          disabled={pinging}
          className="px-3 py-1.5 text-sm bg-blue-50 text-blue-700 rounded hover:bg-blue-100 disabled:opacity-50"
        >
          {pinging ? 'Pinging…' : 'Ping'}
        </button>
        <button
          onClick={handleTraefik}
          disabled={traefik}
          className="px-3 py-1.5 text-sm bg-green-50 text-green-700 rounded hover:bg-green-100 disabled:opacity-50"
        >
          {traefik ? 'Setting up…' : 'Setup Traefik'}
        </button>
        {pingMsg && <span className="text-xs text-gray-500">{pingMsg}</span>}
      </div>
      {traefikOut && (
        <div className="px-6 py-3">
          <pre className={`text-xs p-3 rounded whitespace-pre-wrap ${traefikOut.status === 'ok' ? 'bg-green-50 text-green-800' : 'bg-red-50 text-red-800'}`}>
            {traefikOut.output}
          </pre>
        </div>
      )}
    </div>
  )
}

export default function NodeDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [node, setNode]   = useState<Node | null>(null)
  const [tab, setTab]     = useState<Tab>('overview')
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!id) return
    nodes.get(id)
      .then(setNode)
      .catch(e => setError(e.message))
  }, [id])

  if (error) {
    return (
      <div className="p-8 text-center text-red-600">
        {error}
        <button onClick={() => navigate('/nodes')} className="ml-3 underline text-blue-600">Back to nodes</button>
      </div>
    )
  }

  if (!node) {
    return <div className="p-8 text-center text-gray-400">Loading…</div>
  }

  const tabs: { key: Tab; label: string }[] = [
    { key: 'overview', label: 'Overview' },
    { key: 'disk',     label: 'Disk' },
  ]

  return (
    <div>
      {/* Header */}
      <div className="mb-6">
        <button
          onClick={() => navigate('/nodes')}
          className="text-sm text-gray-500 hover:text-gray-700 mb-3 flex items-center gap-1"
        >
          ← Nodes
        </button>
        <div className="flex flex-wrap items-center gap-3">
          <h1 className="text-2xl font-bold text-gray-900">{node.name}</h1>
          <StatusBadge status={node.status} />
          {node.traefik_enabled && (
            <span className="px-2 py-0.5 text-xs bg-green-100 text-green-700 rounded font-medium">Traefik</span>
          )}
          {node.provider === 'digitalocean' && (
            <span className="px-2 py-0.5 text-xs bg-blue-100 text-blue-700 rounded font-medium">DigitalOcean</span>
          )}
          {node.provider === 'aws' && (
            <span className="px-2 py-0.5 text-xs bg-orange-100 text-orange-700 rounded font-medium">AWS</span>
          )}
        </div>
        <p className="text-sm text-gray-500 mt-1">{node.host}:{node.port}</p>
      </div>

      {/* Tabs */}
      <div className="border-b mb-6">
        <div className="flex gap-0">
          {tabs.map(t => (
            <button
              key={t.key}
              onClick={() => setTab(t.key)}
              className={`px-5 py-2.5 text-sm font-medium border-b-2 transition-colors
                ${tab === t.key
                  ? 'border-indigo-600 text-indigo-600'
                  : 'border-transparent text-gray-500 hover:text-gray-700'}`}
            >
              {t.label}
            </button>
          ))}
        </div>
      </div>

      {/* Tab content */}
      {tab === 'overview' && <OverviewTab node={node} />}
      {tab === 'disk'     && <DiskTab node={node} />}
    </div>
  )
}
