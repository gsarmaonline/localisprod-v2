import { useEffect, useState } from 'react'
import { databases, nodes, Database, CreateDatabaseInput, Node } from '../api/client'
import Modal from '../components/Modal'
import StatusBadge from '../components/StatusBadge'

const DB_TYPES = ['postgres', 'redis'] as const
const DEFAULT_VERSIONS: Record<string, string> = {
  postgres: '16', redis: '7',
}

export default function Databases() {
  const [dbList, setDbList] = useState<Database[]>([])
  const [nodeList, setNodeList] = useState<Node[]>([])
  const [showCreate, setShowCreate] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [form, setForm] = useState<{
    name: string
    type: string
    version: string
    node_id: string
    dbname: string
    db_user: string
    password: string
    port: string
  }>({
    name: '', type: 'postgres', version: '', node_id: '',
    dbname: '', db_user: '', password: '', port: '',
  })

  const resetForm = () =>
    setForm({ name: '', type: 'postgres', version: '', node_id: '', dbname: '', db_user: '', password: '', port: '' })

  const load = () =>
    databases.list().then(setDbList).catch(e => setError(e.message))

  useEffect(() => {
    load()
    nodes.list().then(setNodeList).catch(e => setError(e.message))
  }, [])

  const handleCreate = async () => {
    try {
      setLoading(true)
      const data: CreateDatabaseInput = {
        name: form.name,
        type: form.type,
        node_id: form.node_id,
        password: form.password,
        version: form.version || undefined,
        dbname: form.dbname || undefined,
        db_user: form.db_user || undefined,
        port: form.port ? parseInt(form.port) : undefined,
      }
      await databases.create(data)
      setShowCreate(false)
      resetForm()
      await load()
    } catch (e: unknown) {
      setError((e as Error).message)
    } finally {
      setLoading(false)
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('Stop and delete this database? The Docker volume will be preserved.')) return
    try {
      await databases.delete(id)
      await load()
    } catch (e: unknown) {
      setError((e as Error).message)
    }
  }

  const typeColor: Record<string, string> = {
    postgres: 'bg-blue-100 text-blue-700',
    redis:    'bg-red-100 text-red-700',
  }

  return (
    <div>
      <div className="flex flex-wrap items-center justify-between gap-3 mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Databases</h1>
        <button
          onClick={() => { resetForm(); setShowCreate(true) }}
          className="px-4 py-2 bg-emerald-600 text-white rounded-lg hover:bg-emerald-700 text-sm font-medium"
        >
          + Provision Database
        </button>
      </div>

      {error && (
        <div className="mb-4 p-3 bg-red-50 text-red-700 rounded-lg text-sm">
          {error}
          <button onClick={() => setError(null)} className="ml-2 underline">dismiss</button>
        </div>
      )}

      <div className="bg-white rounded-xl shadow-sm border overflow-hidden overflow-x-auto">
        <table className="w-full text-sm">
          <thead className="bg-gray-50 border-b">
            <tr>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Name</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Type</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Node</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Connection</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Env Var</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Status</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y">
            {dbList.length === 0 && (
              <tr>
                <td colSpan={7} className="px-4 py-8 text-center text-gray-400">No databases yet</td>
              </tr>
            )}
            {dbList.map(db => (
              <tr key={db.id} className="hover:bg-gray-50">
                <td className="px-4 py-3 font-medium">{db.name}</td>
                <td className="px-4 py-3">
                  <span className={`px-2 py-0.5 rounded text-xs font-medium ${typeColor[db.type] ?? 'bg-gray-100 text-gray-700'}`}>
                    {db.type}:{db.version}
                  </span>
                </td>
                <td className="px-4 py-3 text-gray-600">{db.node_name}</td>
                <td className="px-4 py-3 text-gray-500 font-mono text-xs">
                  {db.node_host}:{db.port}/{db.dbname || db.name}
                </td>
                <td className="px-4 py-3 font-mono text-xs text-emerald-700">
                  {envVarName(db.name)}
                </td>
                <td className="px-4 py-3">
                  <StatusBadge status={db.status} />
                </td>
                <td className="px-4 py-3">
                  <button
                    onClick={() => handleDelete(db.id)}
                    className="px-2 py-1 text-xs bg-red-50 text-red-700 rounded hover:bg-red-100"
                  >
                    Delete
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {showCreate && (
        <Modal title="Provision Database" onClose={() => setShowCreate(false)}>
          <div className="space-y-3">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Name</label>
              <input
                className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-emerald-500"
                value={form.name}
                onChange={e => setForm(prev => ({ ...prev, name: e.target.value }))}
                placeholder="my-db"
              />
              {form.name && (
                <p className="mt-1 text-xs text-gray-400">
                  Env var injected: <span className="font-mono text-emerald-600">{envVarName(form.name)}</span>
                </p>
              )}
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Type</label>
                <select
                  className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-emerald-500"
                  value={form.type}
                  onChange={e => setForm(prev => ({ ...prev, type: e.target.value, version: '' }))}
                >
                  {DB_TYPES.map(t => (
                    <option key={t} value={t}>{t}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Version <span className="text-gray-400 font-normal">(default: {DEFAULT_VERSIONS[form.type]})</span>
                </label>
                <input
                  className="w-full border rounded-lg px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-emerald-500"
                  value={form.version}
                  onChange={e => setForm(prev => ({ ...prev, version: e.target.value }))}
                  placeholder={DEFAULT_VERSIONS[form.type]}
                />
              </div>
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Node</label>
              <select
                className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-emerald-500"
                value={form.node_id}
                onChange={e => setForm(prev => ({ ...prev, node_id: e.target.value }))}
              >
                <option value="">Select a node…</option>
                {nodeList.map(n => (
                  <option key={n.id} value={n.id}>{n.name} ({n.host})</option>
                ))}
              </select>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  DB Name <span className="text-gray-400 font-normal">(optional)</span>
                </label>
                <input
                  className="w-full border rounded-lg px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-emerald-500"
                  value={form.dbname}
                  onChange={e => setForm(prev => ({ ...prev, dbname: e.target.value }))}
                  placeholder={form.name || 'my-db'}
                  disabled={form.type === 'redis'}
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  DB User <span className="text-gray-400 font-normal">(optional)</span>
                </label>
                <input
                  className="w-full border rounded-lg px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-emerald-500"
                  value={form.db_user}
                  onChange={e => setForm(prev => ({ ...prev, db_user: e.target.value }))}
                  placeholder={form.name || 'my-db'}
                  disabled={form.type === 'redis'}
                />
              </div>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Password</label>
                <input
                  type="password"
                  className="w-full border rounded-lg px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-emerald-500"
                  value={form.password}
                  onChange={e => setForm(prev => ({ ...prev, password: e.target.value }))}
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Host Port <span className="text-gray-400 font-normal">(optional)</span>
                </label>
                <input
                  type="number"
                  className="w-full border rounded-lg px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-emerald-500"
                  value={form.port}
                  onChange={e => setForm(prev => ({ ...prev, port: e.target.value }))}
                  placeholder={{ postgres: '5432', redis: '6379' }[form.type]}
                />
              </div>
            </div>

            <div className="flex gap-2 justify-end pt-2">
              <button onClick={() => setShowCreate(false)} className="px-4 py-2 text-sm text-gray-600 hover:text-gray-900">
                Cancel
              </button>
              <button
                onClick={handleCreate}
                disabled={loading || !form.name || !form.node_id || !form.password}
                className="px-4 py-2 bg-emerald-600 text-white text-sm rounded-lg hover:bg-emerald-700 disabled:opacity-50"
              >
                {loading ? 'Provisioning…' : 'Provision'}
              </button>
            </div>
          </div>
        </Modal>
      )}
    </div>
  )
}

function envVarName(name: string): string {
  return name.toUpperCase().replace(/[-\s.]/g, '_') + '_URL'
}
