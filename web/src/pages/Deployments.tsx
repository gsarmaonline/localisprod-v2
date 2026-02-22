import { useEffect, useState } from 'react'
import { deployments, applications, nodes, Deployment, Application, Node } from '../api/client'
import Modal from '../components/Modal'
import StatusBadge from '../components/StatusBadge'

export default function Deployments() {
  const [depList, setDepList] = useState<Deployment[]>([])
  const [appList, setAppList] = useState<Application[]>([])
  const [nodeList, setNodeList] = useState<Node[]>([])
  const [showDeploy, setShowDeploy] = useState(false)
  const [logsModal, setLogsModal] = useState<{ id: string; logs: string } | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [selectedApp, setSelectedApp] = useState('')
  const [selectedNode, setSelectedNode] = useState('')

  const load = async () => {
    try {
      const [deps, apps, ns] = await Promise.all([
        deployments.list(),
        applications.list(),
        nodes.list(),
      ])
      setDepList(deps)
      setAppList(apps)
      setNodeList(ns)
    } catch (e: unknown) {
      setError((e as Error).message)
    }
  }

  useEffect(() => { load() }, [])

  const handleDeploy = async () => {
    if (!selectedApp || !selectedNode) return
    try {
      setLoading(true)
      await deployments.create({ application_id: selectedApp, node_id: selectedNode })
      setShowDeploy(false)
      setSelectedApp('')
      setSelectedNode('')
      await load()
    } catch (e: unknown) {
      setError((e as Error).message)
    } finally {
      setLoading(false)
    }
  }

  const handleStop = async (id: string) => {
    if (!confirm('Stop and remove this deployment?')) return
    try {
      await deployments.delete(id)
      await load()
    } catch (e: unknown) {
      setError((e as Error).message)
    }
  }

  const handleRestart = async (id: string) => {
    try {
      await deployments.restart(id)
      await load()
    } catch (e: unknown) {
      setError((e as Error).message)
    }
  }

  const handleLogs = async (id: string) => {
    try {
      const result = await deployments.logs(id)
      setLogsModal({ id, logs: result.logs || result.error || '(empty)' })
    } catch (e: unknown) {
      setError((e as Error).message)
    }
  }

  return (
    <div>
      <div className="flex flex-wrap items-center justify-between gap-3 mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Deployments</h1>
        <button
          onClick={() => setShowDeploy(true)}
          className="px-4 py-2 bg-green-600 text-white rounded-lg hover:bg-green-700 text-sm font-medium"
        >
          + Deploy
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
              <th className="text-left px-4 py-3 font-medium text-gray-600">App</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Node</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Container</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Status</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Created</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y">
            {depList.length === 0 && (
              <tr>
                <td colSpan={6} className="px-4 py-8 text-center text-gray-400">No deployments yet</td>
              </tr>
            )}
            {depList.map(d => (
              <tr key={d.id} className="hover:bg-gray-50">
                <td className="px-4 py-3 font-medium">{d.app_name}</td>
                <td className="px-4 py-3 text-gray-600">{d.node_name}</td>
                <td className="px-4 py-3 text-gray-600 font-mono text-xs">{d.container_name}</td>
                <td className="px-4 py-3"><StatusBadge status={d.status} /></td>
                <td className="px-4 py-3 text-gray-500">{new Date(d.created_at).toLocaleString()}</td>
                <td className="px-4 py-3 flex gap-1">
                  <button
                    onClick={() => handleLogs(d.id)}
                    className="px-2 py-1 text-xs bg-gray-100 text-gray-700 rounded hover:bg-gray-200"
                  >
                    Logs
                  </button>
                  <button
                    onClick={() => handleRestart(d.id)}
                    className="px-2 py-1 text-xs bg-blue-50 text-blue-700 rounded hover:bg-blue-100"
                  >
                    Restart
                  </button>
                  <button
                    onClick={() => handleStop(d.id)}
                    className="px-2 py-1 text-xs bg-red-50 text-red-700 rounded hover:bg-red-100"
                  >
                    Stop
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {showDeploy && (
        <Modal title="Deploy Application" onClose={() => setShowDeploy(false)}>
          <div className="space-y-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Application</label>
              <select
                className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-green-500"
                value={selectedApp}
                onChange={e => setSelectedApp(e.target.value)}
              >
                <option value="">Select application...</option>
                {appList.map(a => (
                  <option key={a.id} value={a.id}>{a.name} ({a.docker_image})</option>
                ))}
              </select>
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Node</label>
              <select
                className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-green-500"
                value={selectedNode}
                onChange={e => setSelectedNode(e.target.value)}
              >
                <option value="">Select node...</option>
                {nodeList.map(n => (
                  <option key={n.id} value={n.id}>{n.name} ({n.host})</option>
                ))}
              </select>
            </div>
            <div className="flex gap-2 justify-end pt-2">
              <button onClick={() => setShowDeploy(false)} className="px-4 py-2 text-sm text-gray-600">
                Cancel
              </button>
              <button
                onClick={handleDeploy}
                disabled={loading || !selectedApp || !selectedNode}
                className="px-4 py-2 bg-green-600 text-white text-sm rounded-lg hover:bg-green-700 disabled:opacity-50"
              >
                {loading ? 'Deploying...' : 'Deploy'}
              </button>
            </div>
          </div>
        </Modal>
      )}

      {logsModal && (
        <Modal title="Container Logs" onClose={() => setLogsModal(null)}>
          <pre className="bg-gray-900 text-green-400 text-xs rounded-lg p-4 overflow-auto max-h-96 whitespace-pre-wrap font-mono">
            {logsModal.logs || '(no output)'}
          </pre>
        </Modal>
      )}
    </div>
  )
}
