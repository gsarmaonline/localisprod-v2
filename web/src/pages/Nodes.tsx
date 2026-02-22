import { useEffect, useState } from 'react'
import { nodes, Node, CreateNodeInput } from '../api/client'
import Modal from '../components/Modal'
import StatusBadge from '../components/StatusBadge'

export default function Nodes() {
  const [nodeList, setNodeList] = useState<Node[]>([])
  const [showAdd, setShowAdd] = useState(false)
  const [pingResults, setPingResults] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const [form, setForm] = useState<CreateNodeInput>({
    name: '', host: '', port: 22, username: '', private_key: '',
  })

  const load = () =>
    nodes.list().then(setNodeList).catch(e => setError(e.message))

  useEffect(() => { load() }, [])

  const handleAdd = async () => {
    try {
      setLoading(true)
      await nodes.create(form)
      setShowAdd(false)
      setForm({ name: '', host: '', port: 22, username: '', private_key: '' })
      await load()
    } catch (e: unknown) {
      setError((e as Error).message)
    } finally {
      setLoading(false)
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('Delete this node?')) return
    try {
      await nodes.delete(id)
      await load()
    } catch (e: unknown) {
      setError((e as Error).message)
    }
  }

  const handlePing = async (id: string) => {
    try {
      const result = await nodes.ping(id)
      setPingResults(prev => ({ ...prev, [id]: result.message }))
      await load()
    } catch (e: unknown) {
      setPingResults(prev => ({ ...prev, [id]: (e as Error).message }))
    }
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Nodes</h1>
        <button
          onClick={() => setShowAdd(true)}
          className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 text-sm font-medium"
        >
          + Add Node
        </button>
      </div>

      {error && (
        <div className="mb-4 p-3 bg-red-50 text-red-700 rounded-lg text-sm">
          {error}
          <button onClick={() => setError(null)} className="ml-2 underline">dismiss</button>
        </div>
      )}

      <div className="bg-white rounded-xl shadow-sm border overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-gray-50 border-b">
            <tr>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Name</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Host</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Port</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">User</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Status</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y">
            {nodeList.length === 0 && (
              <tr>
                <td colSpan={6} className="px-4 py-8 text-center text-gray-400">No nodes yet</td>
              </tr>
            )}
            {nodeList.map(n => (
              <tr key={n.id} className="hover:bg-gray-50">
                <td className="px-4 py-3 font-medium">{n.name}</td>
                <td className="px-4 py-3 text-gray-600">{n.host}</td>
                <td className="px-4 py-3 text-gray-600">{n.port}</td>
                <td className="px-4 py-3 text-gray-600">{n.username}</td>
                <td className="px-4 py-3">
                  <StatusBadge status={n.status} />
                  {pingResults[n.id] && (
                    <span className="ml-2 text-xs text-gray-500">({pingResults[n.id]})</span>
                  )}
                </td>
                <td className="px-4 py-3 flex gap-2">
                  <button
                    onClick={() => handlePing(n.id)}
                    className="px-2 py-1 text-xs bg-blue-50 text-blue-700 rounded hover:bg-blue-100"
                  >
                    Ping
                  </button>
                  <button
                    onClick={() => handleDelete(n.id)}
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

      {showAdd && (
        <Modal title="Add Node" onClose={() => setShowAdd(false)}>
          <div className="space-y-3">
            {(['name', 'host', 'username'] as const).map(field => (
              <div key={field}>
                <label className="block text-sm font-medium text-gray-700 mb-1 capitalize">{field}</label>
                <input
                  className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                  value={form[field]}
                  onChange={e => setForm(prev => ({ ...prev, [field]: e.target.value }))}
                />
              </div>
            ))}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Port</label>
              <input
                type="number"
                className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                value={form.port}
                onChange={e => setForm(prev => ({ ...prev, port: parseInt(e.target.value) || 22 }))}
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Private Key (PEM)</label>
              <textarea
                rows={6}
                className="w-full border rounded-lg px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-blue-500"
                value={form.private_key}
                onChange={e => setForm(prev => ({ ...prev, private_key: e.target.value }))}
                placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
              />
            </div>
            <div className="flex gap-2 justify-end pt-2">
              <button
                onClick={() => setShowAdd(false)}
                className="px-4 py-2 text-sm text-gray-600 hover:text-gray-900"
              >
                Cancel
              </button>
              <button
                onClick={handleAdd}
                disabled={loading}
                className="px-4 py-2 bg-blue-600 text-white text-sm rounded-lg hover:bg-blue-700 disabled:opacity-50"
              >
                {loading ? 'Adding...' : 'Add Node'}
              </button>
            </div>
          </div>
        </Modal>
      )}
    </div>
  )
}
