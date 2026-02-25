import { useEffect, useState } from 'react'
import { kafkas, nodes, Kafka, CreateKafkaInput, Node } from '../api/client'
import Modal from '../components/Modal'
import StatusBadge from '../components/StatusBadge'

export default function Kafkas() {
  const [kafkaList, setKafkaList] = useState<Kafka[]>([])
  const [nodeList, setNodeList] = useState<Node[]>([])
  const [showCreate, setShowCreate] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [form, setForm] = useState<{
    name: string
    version: string
    node_id: string
    port: string
  }>({
    name: '', version: '', node_id: '', port: '',
  })

  const resetForm = () =>
    setForm({ name: '', version: '', node_id: '', port: '' })

  const load = () =>
    kafkas.list().then(setKafkaList).catch(e => setError(e.message))

  useEffect(() => {
    load()
    nodes.list().then(setNodeList).catch(e => setError(e.message))
  }, [])

  const handleCreate = async () => {
    try {
      setLoading(true)
      const data: CreateKafkaInput = {
        name: form.name,
        node_id: form.node_id,
        version: form.version || undefined,
        port: form.port ? parseInt(form.port) : undefined,
      }
      await kafkas.create(data)
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
    if (!confirm('Stop and delete this Kafka cluster? The Docker volume will be preserved.')) return
    try {
      await kafkas.delete(id)
      await load()
    } catch (e: unknown) {
      setError((e as Error).message)
    }
  }

  return (
    <div>
      <div className="flex flex-wrap items-center justify-between gap-3 mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Kafka</h1>
        <button
          onClick={() => { resetForm(); setShowCreate(true) }}
          className="px-4 py-2 bg-orange-600 text-white rounded-lg hover:bg-orange-700 text-sm font-medium"
        >
          + Provision Kafka
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
              <th className="text-left px-4 py-3 font-medium text-gray-600">Bootstrap Server</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Env Var</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Status</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y">
            {kafkaList.length === 0 && (
              <tr>
                <td colSpan={7} className="px-4 py-8 text-center text-gray-400">No Kafka clusters yet</td>
              </tr>
            )}
            {kafkaList.map(k => (
              <tr key={k.id} className="hover:bg-gray-50">
                <td className="px-4 py-3 font-medium">{k.name}</td>
                <td className="px-4 py-3">
                  <span className="px-2 py-0.5 rounded text-xs font-medium bg-orange-100 text-orange-700">
                    kafka:{k.version}
                  </span>
                </td>
                <td className="px-4 py-3 text-gray-600">{k.node_name}</td>
                <td className="px-4 py-3 text-gray-500 font-mono text-xs">
                  {k.node_host}:{k.port}
                </td>
                <td className="px-4 py-3 font-mono text-xs text-orange-700">
                  {envVarName(k.name)}
                </td>
                <td className="px-4 py-3">
                  <StatusBadge status={k.status} />
                </td>
                <td className="px-4 py-3">
                  <button
                    onClick={() => handleDelete(k.id)}
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
        <Modal title="Provision Kafka Cluster" onClose={() => setShowCreate(false)}>
          <div className="space-y-3">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Name</label>
              <input
                className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-orange-500"
                value={form.name}
                onChange={e => setForm(prev => ({ ...prev, name: e.target.value }))}
                placeholder="my-kafka"
              />
              {form.name && (
                <p className="mt-1 text-xs text-gray-400">
                  Env var injected: <span className="font-mono text-orange-600">{envVarName(form.name)}</span>
                  {' '}· <span className="font-mono text-orange-600">KAFKA_BROKERS</span>
                </p>
              )}
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Version <span className="text-gray-400 font-normal">(default: latest)</span>
                </label>
                <input
                  className="w-full border rounded-lg px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-orange-500"
                  value={form.version}
                  onChange={e => setForm(prev => ({ ...prev, version: e.target.value }))}
                  placeholder="latest"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Host Port <span className="text-gray-400 font-normal">(default: 9092)</span>
                </label>
                <input
                  type="number"
                  className="w-full border rounded-lg px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-orange-500"
                  value={form.port}
                  onChange={e => setForm(prev => ({ ...prev, port: e.target.value }))}
                  placeholder="9092"
                />
              </div>
            </div>

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Node</label>
              <select
                className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-orange-500"
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
              Provisions a single-node Kafka cluster using <span className="font-mono">bitnami/kafka</span> in KRaft mode (no ZooKeeper).
              The Docker volume is preserved on delete.
            </p>

            <div className="flex gap-2 justify-end pt-2">
              <button onClick={() => setShowCreate(false)} className="px-4 py-2 text-sm text-gray-600 hover:text-gray-900">
                Cancel
              </button>
              <button
                onClick={handleCreate}
                disabled={loading || !form.name || !form.node_id}
                className="px-4 py-2 bg-orange-600 text-white text-sm rounded-lg hover:bg-orange-700 disabled:opacity-50"
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
