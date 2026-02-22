import { useEffect, useState } from 'react'
import { applications, Application, CreateApplicationInput } from '../api/client'
import Modal from '../components/Modal'

export default function Applications() {
  const [appList, setAppList] = useState<Application[]>([])
  const [showCreate, setShowCreate] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const [form, setForm] = useState<{
    name: string
    docker_image: string
    command: string
    envPairs: { key: string; value: string }[]
    ports: string[]
  }>({
    name: '', docker_image: '', command: '',
    envPairs: [{ key: '', value: '' }],
    ports: [''],
  })

  const load = () =>
    applications.list().then(setAppList).catch(e => setError(e.message))

  useEffect(() => { load() }, [])

  const handleCreate = async () => {
    try {
      setLoading(true)
      const envVars: Record<string, string> = {}
      for (const { key, value } of form.envPairs) {
        if (key) envVars[key] = value
      }
      const data: CreateApplicationInput = {
        name: form.name,
        docker_image: form.docker_image,
        command: form.command,
        env_vars: envVars,
        ports: form.ports.filter(Boolean),
      }
      await applications.create(data)
      setShowCreate(false)
      setForm({ name: '', docker_image: '', command: '', envPairs: [{ key: '', value: '' }], ports: [''] })
      await load()
    } catch (e: unknown) {
      setError((e as Error).message)
    } finally {
      setLoading(false)
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('Delete this application?')) return
    try {
      await applications.delete(id)
      await load()
    } catch (e: unknown) {
      setError((e as Error).message)
    }
  }

  const parsePorts = (s: string) => {
    try { return JSON.parse(s) as string[] } catch { return [] }
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Applications</h1>
        <button
          onClick={() => setShowCreate(true)}
          className="px-4 py-2 bg-purple-600 text-white rounded-lg hover:bg-purple-700 text-sm font-medium"
        >
          + Create App
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
              <th className="text-left px-4 py-3 font-medium text-gray-600">Image</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Ports</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Command</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y">
            {appList.length === 0 && (
              <tr>
                <td colSpan={5} className="px-4 py-8 text-center text-gray-400">No applications yet</td>
              </tr>
            )}
            {appList.map(a => (
              <tr key={a.id} className="hover:bg-gray-50">
                <td className="px-4 py-3 font-medium">{a.name}</td>
                <td className="px-4 py-3 text-gray-600 font-mono text-xs">{a.docker_image}</td>
                <td className="px-4 py-3 text-gray-600">
                  {parsePorts(a.ports).join(', ') || '—'}
                </td>
                <td className="px-4 py-3 text-gray-600 font-mono text-xs">{a.command || '—'}</td>
                <td className="px-4 py-3">
                  <button
                    onClick={() => handleDelete(a.id)}
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
        <Modal title="Create Application" onClose={() => setShowCreate(false)}>
          <div className="space-y-3">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Name</label>
              <input
                className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-purple-500"
                value={form.name}
                onChange={e => setForm(prev => ({ ...prev, name: e.target.value }))}
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Docker Image</label>
              <input
                className="w-full border rounded-lg px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-purple-500"
                value={form.docker_image}
                onChange={e => setForm(prev => ({ ...prev, docker_image: e.target.value }))}
                placeholder="nginx:latest"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Command (optional)</label>
              <input
                className="w-full border rounded-lg px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-purple-500"
                value={form.command}
                onChange={e => setForm(prev => ({ ...prev, command: e.target.value }))}
              />
            </div>

            {/* Ports */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Port Mappings</label>
              {form.ports.map((p, i) => (
                <div key={i} className="flex gap-2 mb-1">
                  <input
                    className="flex-1 border rounded-lg px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-purple-500"
                    value={p}
                    placeholder="8080:80"
                    onChange={e => {
                      const ports = [...form.ports]
                      ports[i] = e.target.value
                      setForm(prev => ({ ...prev, ports }))
                    }}
                  />
                  <button
                    onClick={() => setForm(prev => ({ ...prev, ports: prev.ports.filter((_, j) => j !== i) }))}
                    className="text-red-400 hover:text-red-600 px-2"
                  >×</button>
                </div>
              ))}
              <button
                onClick={() => setForm(prev => ({ ...prev, ports: [...prev.ports, ''] }))}
                className="text-xs text-purple-600 hover:underline"
              >+ Add port</button>
            </div>

            {/* Env Vars */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Environment Variables</label>
              {form.envPairs.map((pair, i) => (
                <div key={i} className="flex gap-2 mb-1">
                  <input
                    className="flex-1 border rounded-lg px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-purple-500"
                    placeholder="KEY"
                    value={pair.key}
                    onChange={e => {
                      const envPairs = [...form.envPairs]
                      envPairs[i] = { ...envPairs[i], key: e.target.value }
                      setForm(prev => ({ ...prev, envPairs }))
                    }}
                  />
                  <input
                    className="flex-1 border rounded-lg px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-purple-500"
                    placeholder="VALUE"
                    value={pair.value}
                    onChange={e => {
                      const envPairs = [...form.envPairs]
                      envPairs[i] = { ...envPairs[i], value: e.target.value }
                      setForm(prev => ({ ...prev, envPairs }))
                    }}
                  />
                  <button
                    onClick={() => setForm(prev => ({ ...prev, envPairs: prev.envPairs.filter((_, j) => j !== i) }))}
                    className="text-red-400 hover:text-red-600 px-2"
                  >×</button>
                </div>
              ))}
              <button
                onClick={() => setForm(prev => ({ ...prev, envPairs: [...prev.envPairs, { key: '', value: '' }] }))}
                className="text-xs text-purple-600 hover:underline"
              >+ Add variable</button>
            </div>

            <div className="flex gap-2 justify-end pt-2">
              <button onClick={() => setShowCreate(false)} className="px-4 py-2 text-sm text-gray-600 hover:text-gray-900">
                Cancel
              </button>
              <button
                onClick={handleCreate}
                disabled={loading}
                className="px-4 py-2 bg-purple-600 text-white text-sm rounded-lg hover:bg-purple-700 disabled:opacity-50"
              >
                {loading ? 'Creating...' : 'Create App'}
              </button>
            </div>
          </div>
        </Modal>
      )}
    </div>
  )
}
