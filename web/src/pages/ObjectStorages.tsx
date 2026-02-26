import { Fragment, useEffect, useState } from 'react'
import { objectStorages, nodes, ObjectStorage, CreateObjectStorageInput, Node } from '../api/client'
import Modal from '../components/Modal'
import StatusBadge from '../components/StatusBadge'

export default function ObjectStorages() {
  const [list, setList] = useState<ObjectStorage[]>([])
  const [nodeList, setNodeList] = useState<Node[]>([])
  const [showCreate, setShowCreate] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [expandedID, setExpandedID] = useState<string | null>(null)
  const [maskedSecrets, setMaskedSecrets] = useState<Record<string, boolean>>({})
  const [copied, setCopied] = useState<string | null>(null)

  const [form, setForm] = useState<{
    name: string
    node_id: string
    s3_port: string
    version: string
  }>({
    name: '', node_id: '', s3_port: '', version: '',
  })

  const resetForm = () => setForm({ name: '', node_id: '', s3_port: '', version: '' })

  const load = () =>
    objectStorages.list().then(setList).catch(e => setError(e.message))

  useEffect(() => {
    load()
    nodes.list().then(setNodeList).catch(e => setError(e.message))
  }, [])

  const handleCreate = async () => {
    try {
      setLoading(true)
      const data: CreateObjectStorageInput = {
        name: form.name,
        node_id: form.node_id,
        s3_port: form.s3_port ? parseInt(form.s3_port) : undefined,
        version: form.version || undefined,
      }
      await objectStorages.create(data)
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
    if (!confirm('Stop and delete this object storage? Data volumes will be preserved.')) return
    try {
      await objectStorages.delete(id)
      if (expandedID === id) setExpandedID(null)
      await load()
    } catch (e: unknown) {
      setError((e as Error).message)
    }
  }

  const copyToClipboard = async (text: string, key: string) => {
    await navigator.clipboard.writeText(text)
    setCopied(key)
    setTimeout(() => setCopied(null), 2000)
  }

  return (
    <div>
      <div className="flex flex-wrap items-center justify-between gap-3 mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Object Storage</h1>
        <button
          onClick={() => { resetForm(); setShowCreate(true) }}
          className="px-4 py-2 bg-emerald-600 text-white rounded-lg hover:bg-emerald-700 text-sm font-medium"
        >
          + Deploy Object Storage
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
              <th className="text-left px-4 py-3 font-medium text-gray-600">S3 Endpoint</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Status</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Deployed At</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y">
            {list.length === 0 && (
              <tr>
                <td colSpan={6} className="px-4 py-8 text-center text-gray-400">No object storages yet</td>
              </tr>
            )}
            {list.map(o => (
              <Fragment key={o.id}>
                <tr
                  className="hover:bg-gray-50 cursor-pointer"
                  onClick={() => setExpandedID(expandedID === o.id ? null : o.id)}
                >
                  <td className="px-4 py-3 font-medium">{o.name}</td>
                  <td className="px-4 py-3 text-gray-600">{o.node_name}</td>
                  <td className="px-4 py-3 font-mono text-xs text-gray-500">
                    {o.node_host ? `http://${o.node_host}:${o.s3_port}` : `port ${o.s3_port}`}
                  </td>
                  <td className="px-4 py-3">
                    <StatusBadge status={o.status} />
                  </td>
                  <td className="px-4 py-3 text-gray-500 text-xs">
                    {o.last_deployed_at ? new Date(o.last_deployed_at).toLocaleString() : '—'}
                  </td>
                  <td className="px-4 py-3" onClick={e => e.stopPropagation()}>
                    <button
                      onClick={() => handleDelete(o.id)}
                      className="px-2 py-1 text-xs bg-red-50 text-red-700 rounded hover:bg-red-100"
                    >
                      Delete
                    </button>
                  </td>
                </tr>
                {expandedID === o.id && (
                  <tr>
                    <td colSpan={6} className="bg-slate-50 px-6 py-4 border-b">
                      <div className="text-sm font-semibold text-gray-700 mb-3">Connection Info</div>
                      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                        <CredField
                          label="S3 Endpoint"
                          value={o.node_host ? `http://${o.node_host}:${o.s3_port}` : ''}
                          copyKey={`${o.id}-endpoint`}
                          copied={copied}
                          onCopy={copyToClipboard}
                        />
                        <CredField
                          label="Region"
                          value="garage"
                          copyKey={`${o.id}-region`}
                          copied={copied}
                          onCopy={copyToClipboard}
                        />
                        <CredField
                          label="Access Key ID"
                          value={o.access_key_id || '—'}
                          copyKey={`${o.id}-akid`}
                          copied={copied}
                          onCopy={copyToClipboard}
                        />
                        <div>
                          <div className="text-xs text-gray-500 mb-1">Secret Access Key</div>
                          <div className="flex items-center gap-2">
                            <code className="flex-1 font-mono text-xs bg-white border rounded px-2 py-1 text-gray-800 truncate">
                              {o.secret_access_key
                                ? maskedSecrets[o.id]
                                  ? o.secret_access_key
                                  : '••••••••••••••••'
                                : '—'}
                            </code>
                            {o.secret_access_key && (
                              <>
                                <button
                                  onClick={() => setMaskedSecrets(prev => ({ ...prev, [o.id]: !prev[o.id] }))}
                                  className="text-xs text-gray-500 hover:text-gray-800 underline whitespace-nowrap"
                                >
                                  {maskedSecrets[o.id] ? 'hide' : 'show'}
                                </button>
                                <button
                                  onClick={() => copyToClipboard(o.secret_access_key!, `${o.id}-sak`)}
                                  className="text-xs text-indigo-600 hover:text-indigo-800 underline whitespace-nowrap"
                                >
                                  {copied === `${o.id}-sak` ? 'copied!' : 'copy'}
                                </button>
                              </>
                            )}
                          </div>
                        </div>
                      </div>
                      <div className="mt-3 text-xs text-gray-400">
                        Version: <span className="font-mono">{o.version}</span> &nbsp;·&nbsp;
                        Container: <span className="font-mono">{o.container_name}</span>
                      </div>
                    </td>
                  </tr>
                )}
              </Fragment>
            ))}
          </tbody>
        </table>
      </div>

      {showCreate && (
        <Modal title="Deploy Object Storage" onClose={() => setShowCreate(false)}>
          <div className="space-y-3">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Name</label>
              <input
                className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-emerald-500"
                value={form.name}
                onChange={e => setForm(prev => ({ ...prev, name: e.target.value }))}
                placeholder="my-storage"
              />
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
                  S3 Port <span className="text-gray-400 font-normal">(default: 3900)</span>
                </label>
                <input
                  type="number"
                  className="w-full border rounded-lg px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-emerald-500"
                  value={form.s3_port}
                  onChange={e => setForm(prev => ({ ...prev, s3_port: e.target.value }))}
                  placeholder="3900"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">
                  Version <span className="text-gray-400 font-normal">(default: v1.0.1)</span>
                </label>
                <input
                  className="w-full border rounded-lg px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-emerald-500"
                  value={form.version}
                  onChange={e => setForm(prev => ({ ...prev, version: e.target.value }))}
                  placeholder="v1.0.1"
                />
              </div>
            </div>

            <div className="flex gap-2 justify-end pt-2">
              <button onClick={() => setShowCreate(false)} className="px-4 py-2 text-sm text-gray-600 hover:text-gray-900">
                Cancel
              </button>
              <button
                onClick={handleCreate}
                disabled={loading || !form.name || !form.node_id}
                className="px-4 py-2 bg-emerald-600 text-white text-sm rounded-lg hover:bg-emerald-700 disabled:opacity-50"
              >
                {loading ? 'Deploying…' : 'Deploy'}
              </button>
            </div>
          </div>
        </Modal>
      )}
    </div>
  )
}

function CredField({
  label, value, copyKey, copied, onCopy,
}: {
  label: string
  value: string
  copyKey: string
  copied: string | null
  onCopy: (text: string, key: string) => void
}) {
  return (
    <div>
      <div className="text-xs text-gray-500 mb-1">{label}</div>
      <div className="flex items-center gap-2">
        <code className="flex-1 font-mono text-xs bg-white border rounded px-2 py-1 text-gray-800 truncate">{value}</code>
        {value && value !== '—' && (
          <button
            onClick={() => onCopy(value, copyKey)}
            className="text-xs text-indigo-600 hover:text-indigo-800 underline whitespace-nowrap"
          >
            {copied === copyKey ? 'copied!' : 'copy'}
          </button>
        )}
      </div>
    </div>
  )
}
