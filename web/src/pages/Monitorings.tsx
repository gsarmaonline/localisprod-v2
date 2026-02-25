import { useEffect, useState } from 'react'
import { monitorings, nodes, Monitoring, CreateMonitoringInput, Node } from '../api/client'
import Modal from '../components/Modal'
import StatusBadge from '../components/StatusBadge'

export default function Monitorings() {
  const [monitoringList, setMonitoringList] = useState<Monitoring[]>([])
  const [nodeList, setNodeList] = useState<Node[]>([])
  const [showCreate, setShowCreate] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [form, setForm] = useState<{
    name: string
    node_id: string
    prometheus_port: string
    grafana_port: string
    grafana_password: string
  }>({
    name: '', node_id: '', prometheus_port: '', grafana_port: '', grafana_password: '',
  })

  const resetForm = () =>
    setForm({ name: '', node_id: '', prometheus_port: '', grafana_port: '', grafana_password: '' })

  const load = () =>
    monitorings.list().then(setMonitoringList).catch(e => setError(e.message))

  useEffect(() => {
    load()
    nodes.list().then(setNodeList).catch(e => setError(e.message))
  }, [])

  const handleCreate = async () => {
    try {
      setLoading(true)
      const data: CreateMonitoringInput = {
        name: form.name,
        node_id: form.node_id,
        prometheus_port: form.prometheus_port ? parseInt(form.prometheus_port) : undefined,
        grafana_port: form.grafana_port ? parseInt(form.grafana_port) : undefined,
        grafana_password: form.grafana_password || 'admin',
      }
      await monitorings.create(data)
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
    if (!confirm('Stop and delete this monitoring stack? Docker volumes will be preserved.')) return
    try {
      await monitorings.delete(id)
      await load()
    } catch (e: unknown) {
      setError((e as Error).message)
    }
  }

  return (
    <div>
      <div className="flex flex-wrap items-center justify-between gap-3 mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Monitoring</h1>
        <button
          onClick={() => { resetForm(); setShowCreate(true) }}
          className="px-4 py-2 bg-teal-600 text-white rounded-lg hover:bg-teal-700 text-sm font-medium"
        >
          + Provision Monitoring
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
              <th className="text-left px-4 py-3 font-medium text-gray-600">Node</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Prometheus</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Grafana</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Status</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Started At</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y">
            {monitoringList.length === 0 && (
              <tr>
                <td colSpan={7} className="px-4 py-8 text-center text-gray-400">No monitoring stacks yet</td>
              </tr>
            )}
            {monitoringList.map(m => (
              <tr key={m.id} className="hover:bg-gray-50">
                <td className="px-4 py-3 font-medium">{m.name}</td>
                <td className="px-4 py-3 text-gray-600">{m.node_name}</td>
                <td className="px-4 py-3">
                  {isSafeHost(m.node_host) ? (
                    <a
                      href={`http://${m.node_host}:${m.prometheus_port}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="font-mono text-xs text-teal-700 hover:underline"
                    >
                      {m.node_host}:{m.prometheus_port}
                    </a>
                  ) : (
                    <span className="font-mono text-xs text-gray-400">{m.node_host}:{m.prometheus_port}</span>
                  )}
                </td>
                <td className="px-4 py-3">
                  {isSafeHost(m.node_host) ? (
                    <a
                      href={`http://${m.node_host}:${m.grafana_port}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="font-mono text-xs text-teal-700 hover:underline"
                    >
                      {m.node_host}:{m.grafana_port}
                    </a>
                  ) : (
                    <span className="font-mono text-xs text-gray-400">{m.node_host}:{m.grafana_port}</span>
                  )}
                </td>
                <td className="px-4 py-3">
                  <StatusBadge status={m.status} />
                </td>
                <td className="px-4 py-3 text-gray-500 text-xs">{m.last_deployed_at ? new Date(m.last_deployed_at).toLocaleString() : '—'}</td>
                <td className="px-4 py-3">
                  <button
                    onClick={() => handleDelete(m.id)}
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
        <Modal title="Provision Monitoring Stack" onClose={() => setShowCreate(false)}>
          <div className="space-y-3">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Name</label>
              <input
                className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-teal-500"
                value={form.name}
                onChange={e => setForm(prev => ({ ...prev, name: e.target.value }))}
                placeholder="my-monitoring"
              />
              {form.name && (
                <p className="mt-1 text-xs text-gray-400">
                  Env vars injected:{' '}
                  <span className="font-mono text-teal-600">{envVarName(form.name, 'PROMETHEUS')}</span>
                  {' · '}
                  <span className="font-mono text-teal-600">{envVarName(form.name, 'GRAFANA')}</span>
                </p>
              )}
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Prometheus Port <span className="text-gray-400 font-normal">(default: 9090)</span>
                </label>
                <input
                  type="number"
                  className="w-full border rounded-lg px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-teal-500"
                  value={form.prometheus_port}
                  onChange={e => setForm(prev => ({ ...prev, prometheus_port: e.target.value }))}
                  placeholder="9090"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Grafana Port <span className="text-gray-400 font-normal">(default: 3000)</span>
                </label>
                <input
                  type="number"
                  className="w-full border rounded-lg px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-teal-500"
                  value={form.grafana_port}
                  onChange={e => setForm(prev => ({ ...prev, grafana_port: e.target.value }))}
                  placeholder="3000"
                />
              </div>
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Grafana Admin Password <span className="text-gray-400 font-normal">(default: admin)</span>
              </label>
              <input
                type="password"
                className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-teal-500"
                value={form.grafana_password}
                onChange={e => setForm(prev => ({ ...prev, grafana_password: e.target.value }))}
                placeholder="admin"
              />
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Node</label>
              <select
                className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-teal-500"
                value={form.node_id}
                onChange={e => setForm(prev => ({ ...prev, node_id: e.target.value }))}
              >
                <option value="">Select a node…</option>
                {nodeList.map(n => (
                  <option key={n.id} value={n.id}>{n.name} ({n.host})</option>
                ))}
              </select>
            </div>

            <p className="text-xs text-gray-400">
              Provisions a <span className="font-mono">prom/prometheus</span> and{' '}
              <span className="font-mono">grafana/grafana</span> stack. Grafana is pre-configured
              with Prometheus as the default datasource. Docker volumes are preserved on delete.
            </p>

            <div className="flex gap-2 justify-end pt-2">
              <button onClick={() => setShowCreate(false)} className="px-4 py-2 text-sm text-gray-600 hover:text-gray-900">
                Cancel
              </button>
              <button
                onClick={handleCreate}
                disabled={loading || !form.name || !form.node_id}
                className="px-4 py-2 bg-teal-600 text-white text-sm rounded-lg hover:bg-teal-700 disabled:opacity-50"
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

function envVarName(name: string, suffix: string): string {
  return name.toUpperCase().replace(/[-\s.]/g, '_') + '_' + suffix + '_URL'
}

function isSafeHost(host: string): boolean {
  return /^[a-zA-Z0-9.\-]+$/.test(host)
}
