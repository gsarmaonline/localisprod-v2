import { useEffect, useState } from 'react'
import { caches, nodes, Cache, CreateCacheInput, Node } from '../api/client'
import Modal from '../components/Modal'
import StatusBadge from '../components/StatusBadge'

export default function Caches() {
  const [cacheList, setCacheList] = useState<Cache[]>([])
  const [nodeList, setNodeList] = useState<Node[]>([])
  const [showCreate, setShowCreate] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [form, setForm] = useState<{
    name: string
    version: string
    node_id: string
    password: string
    port: string
  }>({
    name: '', version: '', node_id: '', password: '', port: '',
  })

  const resetForm = () =>
    setForm({ name: '', version: '', node_id: '', password: '', port: '' })

  const load = () =>
    caches.list().then(setCacheList).catch(e => setError(e.message))

  useEffect(() => {
    load()
    nodes.list().then(setNodeList).catch(e => setError(e.message))
  }, [])

  const handleCreate = async () => {
    try {
      setLoading(true)
      const data: CreateCacheInput = {
        name: form.name,
        node_id: form.node_id,
        password: form.password,
        version: form.version || undefined,
        port: form.port ? parseInt(form.port) : undefined,
      }
      await caches.create(data)
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
    if (!confirm('Stop and delete this cache? The Docker volume will be preserved.')) return
    try {
      await caches.delete(id)
      await load()
    } catch (e: unknown) {
      setError((e as Error).message)
    }
  }

  return (
    <div>
      <div className="flex flex-wrap items-center justify-between gap-3 mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Cache</h1>
        <button
          onClick={() => { resetForm(); setShowCreate(true) }}
          className="px-4 py-2 bg-emerald-600 text-white rounded-lg hover:bg-emerald-700 text-sm font-medium"
        >
          + Provision Cache
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
              <th className="text-left px-4 py-3 font-medium text-gray-600">Version</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Node</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Connection</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Env Var</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Status</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Started At</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y">
            {cacheList.length === 0 && (
              <tr>
                <td colSpan={8} className="px-4 py-8 text-center text-gray-400">No caches yet</td>
              </tr>
            )}
            {cacheList.map(c => (
              <tr key={c.id} className="hover:bg-gray-50">
                <td className="px-4 py-3 font-medium">{c.name}</td>
                <td className="px-4 py-3">
                  <span className="px-2 py-0.5 rounded text-xs font-medium bg-red-100 text-red-700">
                    redis:{c.version}
                  </span>
                </td>
                <td className="px-4 py-3 text-gray-600">{c.node_name}</td>
                <td className="px-4 py-3 text-gray-500 font-mono text-xs">
                  {c.node_host}:{c.port}
                </td>
                <td className="px-4 py-3 font-mono text-xs text-emerald-700">
                  {envVarName(c.name)}
                </td>
                <td className="px-4 py-3">
                  <StatusBadge status={c.status} />
                </td>
                <td className="px-4 py-3 text-gray-500 text-xs">{c.last_deployed_at ? new Date(c.last_deployed_at).toLocaleString() : '—'}</td>
                <td className="px-4 py-3">
                  <button
                    onClick={() => handleDelete(c.id)}
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
        <Modal title="Provision Cache" onClose={() => setShowCreate(false)}>
          <div className="space-y-3">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Name</label>
              <input
                className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-emerald-500"
                value={form.name}
                onChange={e => setForm(prev => ({ ...prev, name: e.target.value }))}
                placeholder="my-cache"
              />
              {form.name && (
                <p className="mt-1 text-xs text-gray-400">
                  Env var injected: <span className="font-mono text-emerald-600">{envVarName(form.name)}</span>
                </p>
              )}
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Version <span className="text-gray-400 font-normal">(default: 7)</span>
                </label>
                <input
                  className="w-full border rounded-lg px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-emerald-500"
                  value={form.version}
                  onChange={e => setForm(prev => ({ ...prev, version: e.target.value }))}
                  placeholder="7"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Host Port <span className="text-gray-400 font-normal">(default: 6379)</span>
                </label>
                <input
                  type="number"
                  className="w-full border rounded-lg px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-emerald-500"
                  value={form.port}
                  onChange={e => setForm(prev => ({ ...prev, port: e.target.value }))}
                  placeholder="6379"
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

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Password</label>
              <input
                type="password"
                className="w-full border rounded-lg px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-emerald-500"
                value={form.password}
                onChange={e => setForm(prev => ({ ...prev, password: e.target.value }))}
              />
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
