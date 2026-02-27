import { useEffect, useState } from 'react'
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome'
import { faGithub } from '@fortawesome/free-brands-svg-icons'
import { applications, databases, caches, kafkas, monitorings, nodes as nodesApi, github, settings, Application, Database, Cache, Kafka, Monitoring, Node, CreateApplicationInput, GithubRepo } from '../api/client'
import Modal from '../components/Modal'
import ComposeImportWizard from '../components/ComposeImportWizard'

export default function Applications() {
  const [appList, setAppList] = useState<Application[]>([])
  const [dbList, setDbList] = useState<Database[]>([])
  const [cacheList, setCacheList] = useState<Cache[]>([])
  const [kafkaList, setKafkaList] = useState<Kafka[]>([])
  const [monitoringList, setMonitoringList] = useState<Monitoring[]>([])
  const [nodeList, setNodeList] = useState<Node[]>([])
  const [showCreate, setShowCreate] = useState(false)
  const [showComposeWizard, setShowComposeWizard] = useState(false)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [showRepoPicker, setShowRepoPicker] = useState(false)
  const [repos, setRepos] = useState<GithubRepo[]>([])
  const [reposLoading, setReposLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const [form, setForm] = useState<{
    name: string
    docker_image: string
    dockerfile_path: string
    command: string
    github_repo: string
    domain: string
    envPairs: { key: string; value: string }[]
    ports: string[]
    volumes: string[]
    databases: string[]
    caches: string[]
    kafkas: string[]
    monitorings: string[]
  }>({
    name: '', docker_image: '', dockerfile_path: '', command: '', github_repo: '', domain: '',
    envPairs: [{ key: '', value: '' }],
    ports: [''],
    volumes: [],
    databases: [],
    caches: [],
    kafkas: [],
    monitorings: [],
  })

  const [showEnvPaste, setShowEnvPaste] = useState(false)
  const [envPasteText, setEnvPasteText] = useState('')

  const resetForm = () => {
    setForm({ name: '', docker_image: '', dockerfile_path: '', command: '', github_repo: '', domain: '', envPairs: [{ key: '', value: '' }], ports: [''], volumes: [], databases: [], caches: [], kafkas: [], monitorings: [] })
    setShowEnvPaste(false)
    setEnvPasteText('')
  }

  const applyEnvPaste = () => {
    const pairs = envPasteText
      .split('\n')
      .filter(line => line.trim() && !line.trim().startsWith('#'))
      .map(line => {
        const idx = line.indexOf('=')
        if (idx === -1) return null
        return { key: line.slice(0, idx).trim(), value: line.slice(idx + 1).trim() }
      })
      .filter((p): p is { key: string; value: string } => p !== null && p.key !== '')
    if (pairs.length > 0) {
      setForm(prev => ({ ...prev, envPairs: pairs }))
      setShowEnvPaste(false)
      setEnvPasteText('')
    }
  }

  const load = () =>
    applications.list().then(setAppList).catch(e => setError(e.message))

  useEffect(() => {
    load()
    databases.list().then(setDbList).catch(() => {})
    caches.list().then(setCacheList).catch(() => {})
    kafkas.list().then(setKafkaList).catch(() => {})
    monitorings.list().then(setMonitoringList).catch(() => {})
    nodesApi.list().then(setNodeList).catch(() => {})
  }, [])

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
        dockerfile_path: form.dockerfile_path || undefined,
        command: form.command,
        env_vars: envVars,
        ports: form.ports.filter(Boolean),
        volumes: form.volumes.filter(Boolean).length > 0 ? form.volumes.filter(Boolean) : undefined,
        github_repo: form.github_repo || undefined,
        domain: form.domain || undefined,
        databases: form.databases.length > 0 ? form.databases : undefined,
        caches: form.caches.length > 0 ? form.caches : undefined,
        kafkas: form.kafkas.length > 0 ? form.kafkas : undefined,
        monitorings: form.monitorings.length > 0 ? form.monitorings : undefined,
      }
      if (editingId) {
        await applications.update(editingId, data)
        setEditingId(null)
      } else {
        await applications.create(data)
      }
      setShowCreate(false)
      resetForm()
      await load()
    } catch (e: unknown) {
      setError((e as Error).message)
    } finally {
      setLoading(false)
    }
  }

  const handleEdit = (a: Application) => {
    let envPairs: { key: string; value: string }[] = [{ key: '', value: '' }]
    try {
      const parsed = JSON.parse(a.env_vars) as Record<string, string>
      const pairs = Object.entries(parsed).map(([key, value]) => ({ key, value }))
      if (pairs.length > 0) envPairs = pairs
    } catch { /* keep default */ }
    let ports: string[] = ['']
    try {
      const parsed = JSON.parse(a.ports) as string[]
      if (parsed.length > 0) ports = parsed
    } catch { /* keep default */ }
    let vols: string[] = []
    try {
      vols = JSON.parse(a.volumes) as string[]
    } catch { /* keep default */ }
    let dbs: string[] = []
    try {
      dbs = JSON.parse(a.databases) as string[]
    } catch { /* keep default */ }
    let cs: string[] = []
    try {
      cs = JSON.parse(a.caches) as string[]
    } catch { /* keep default */ }
    let ks: string[] = []
    try {
      ks = JSON.parse(a.kafkas) as string[]
    } catch { /* keep default */ }
    let ms: string[] = []
    try {
      ms = JSON.parse(a.monitorings) as string[]
    } catch { /* keep default */ }
    setForm({
      name: a.name,
      docker_image: a.docker_image,
      dockerfile_path: a.dockerfile_path || '',
      command: a.command || '',
      github_repo: a.github_repo || '',
      domain: a.domain || '',
      envPairs,
      ports,
      volumes: vols,
      databases: dbs,
      caches: cs,
      kafkas: ks,
      monitorings: ms,
    })
    setEditingId(a.id)
    setShowCreate(true)
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

  const handleFromGithub = async () => {
    setError(null)
    try {
      const s = await settings.get()
      if (s.github_token !== 'configured') {
        setError('GitHub token not configured. Go to Settings to add your PAT.')
        return
      }
    } catch {
      setError('Could not check settings.')
      return
    }
    setShowRepoPicker(true)
    setReposLoading(true)
    try {
      const list = await github.listRepos()
      setRepos(list)
    } catch (e: unknown) {
      setError((e as Error).message)
      setShowRepoPicker(false)
    } finally {
      setReposLoading(false)
    }
  }

  const handleSelectRepo = (repo: GithubRepo) => {
    setShowRepoPicker(false)
    setForm({
      name: repo.name,
      docker_image: `ghcr.io/${repo.full_name}:latest`,
      dockerfile_path: '',
      command: '',
      github_repo: repo.full_name,
      domain: '',
      envPairs: [{ key: '', value: '' }],
      ports: [''],
      volumes: [],
      databases: [],
      caches: [],
      kafkas: [],
      monitorings: [],
    })
    setShowCreate(true)
  }

  const parsePorts = (s: string) => {
    try { return JSON.parse(s) as string[] } catch { return [] }
  }

  return (
    <div>
      <div className="flex flex-wrap items-center justify-between gap-3 mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Applications</h1>
        <div className="flex gap-2">
          <button
            onClick={() => setShowComposeWizard(true)}
            className="px-4 py-2 bg-indigo-600 text-white rounded-lg hover:bg-indigo-700 text-sm font-medium"
          >
            Import docker-compose.yml
          </button>
          <button
            onClick={handleFromGithub}
            className="px-4 py-2 bg-gray-800 text-white rounded-lg hover:bg-gray-900 text-sm font-medium flex items-center gap-2"
          >
            <FontAwesomeIcon icon={faGithub} className="w-4 h-4" />
            From GitHub
          </button>
          <button
            onClick={() => { resetForm(); setShowCreate(true) }}
            className="px-4 py-2 bg-purple-600 text-white rounded-lg hover:bg-purple-700 text-sm font-medium"
          >
            + Create App
          </button>
        </div>
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
              <th className="text-left px-4 py-3 font-medium text-gray-600">Image</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Ports</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Domain</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Command</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Last Deploy</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y">
            {appList.length === 0 && (
              <tr>
                <td colSpan={7} className="px-4 py-8 text-center text-gray-400">No applications yet</td>
              </tr>
            )}
            {appList.map(a => (
              <tr key={a.id} className="hover:bg-gray-50">
                <td className="px-4 py-3 font-medium">
                  <span>{a.name}</span>
                  {a.github_repo && (
                    <a
                      href={`https://github.com/${a.github_repo}`}
                      target="_blank"
                      rel="noopener noreferrer"
                      title={a.github_repo}
                      className="ml-2 text-gray-400 hover:text-gray-600 inline-block align-middle"
                    >
                      <FontAwesomeIcon icon={faGithub} className="w-3.5 h-3.5" />
                    </a>
                  )}
                </td>
                <td className="px-4 py-3 text-gray-600 font-mono text-xs">{a.docker_image}</td>
                <td className="px-4 py-3 text-gray-600">
                  {parsePorts(a.ports).join(', ') || '—'}
                </td>
                <td className="px-4 py-3 text-gray-600 font-mono text-xs">{a.domain || '—'}</td>
                <td className="px-4 py-3 text-gray-600 font-mono text-xs">{a.command || '—'}</td>
                <td className="px-4 py-3 text-gray-500 text-xs">{a.last_deployed_at ? new Date(a.last_deployed_at).toLocaleString() : '—'}</td>
                <td className="px-4 py-3 flex gap-2">
                  <button
                    onClick={() => handleEdit(a)}
                    className="px-2 py-1 text-xs bg-blue-50 text-blue-700 rounded hover:bg-blue-100"
                  >
                    Edit
                  </button>
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

      {/* Repo Picker Modal */}
      {showRepoPicker && (
        <Modal title="Select GitHub Repository" onClose={() => setShowRepoPicker(false)}>
          {reposLoading ? (
            <div className="py-8 text-center text-gray-400 text-sm">Loading repositories...</div>
          ) : (
            <div className="overflow-auto max-h-96">
              <table className="w-full text-sm">
                <thead className="bg-gray-50 border-b sticky top-0">
                  <tr>
                    <th className="text-left px-3 py-2 font-medium text-gray-600">Repository</th>
                    <th className="text-left px-3 py-2 font-medium text-gray-600">Description</th>
                    <th className="px-3 py-2"></th>
                  </tr>
                </thead>
                <tbody className="divide-y">
                  {repos.length === 0 && (
                    <tr>
                      <td colSpan={3} className="px-3 py-6 text-center text-gray-400">No repositories found</td>
                    </tr>
                  )}
                  {repos.map(repo => (
                    <tr key={repo.full_name} className="hover:bg-gray-50">
                      <td className="px-3 py-2.5 font-medium">
                        {repo.name}
                        {repo.private && (
                          <span className="ml-2 px-1.5 py-0.5 text-xs bg-gray-100 text-gray-500 rounded">private</span>
                        )}
                      </td>
                      <td className="px-3 py-2.5 text-gray-500 text-xs">{repo.description || '—'}</td>
                      <td className="px-3 py-2.5">
                        <button
                          onClick={() => handleSelectRepo(repo)}
                          className="px-3 py-1 text-xs bg-purple-600 text-white rounded hover:bg-purple-700"
                        >
                          Select
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Modal>
      )}

      {showCreate && (
        <Modal title={editingId ? 'Edit Application' : 'Create Application'} onClose={() => { setShowCreate(false); setEditingId(null); resetForm() }}>
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
              <label className="block text-sm font-medium text-gray-700 mb-1">Dockerfile Path (optional)</label>
              <input
                className="w-full border rounded-lg px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-purple-500"
                value={form.dockerfile_path}
                onChange={e => setForm(prev => ({ ...prev, dockerfile_path: e.target.value }))}
                placeholder="./Dockerfile"
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
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Domain (optional)</label>
              <input
                className="w-full border rounded-lg px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-purple-500"
                value={form.domain}
                onChange={e => setForm(prev => ({ ...prev, domain: e.target.value }))}
                placeholder="app.example.com"
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

            {/* Volumes */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Volumes</label>
              {form.volumes.map((v, i) => (
                <div key={i} className="flex gap-2 mb-1">
                  <input
                    className="flex-1 border rounded-lg px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-purple-500"
                    value={v}
                    placeholder="vol-name:/container/path"
                    onChange={e => {
                      const volumes = [...form.volumes]
                      volumes[i] = e.target.value
                      setForm(prev => ({ ...prev, volumes }))
                    }}
                  />
                  <button
                    onClick={() => setForm(prev => ({ ...prev, volumes: prev.volumes.filter((_, j) => j !== i) }))}
                    className="text-red-400 hover:text-red-600 px-2"
                  >×</button>
                </div>
              ))}
              <button
                onClick={() => setForm(prev => ({ ...prev, volumes: [...prev.volumes, ''] }))}
                className="text-xs text-purple-600 hover:underline"
              >+ Add volume</button>
            </div>

            {/* Env Vars */}
            <div>
              <div className="flex items-center justify-between mb-1">
                <label className="block text-sm font-medium text-gray-700">Environment Variables</label>
                <button
                  type="button"
                  onClick={() => { setShowEnvPaste(v => !v); setEnvPasteText('') }}
                  className="text-xs text-purple-600 hover:underline"
                >
                  {showEnvPaste ? 'Cancel paste' : 'Paste .env'}
                </button>
              </div>
              {showEnvPaste && (
                <div className="mb-2">
                  <textarea
                    className="w-full border rounded-lg px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-purple-500 resize-y"
                    rows={6}
                    placeholder={"KEY=value\nANOTHER_KEY=another_value"}
                    value={envPasteText}
                    onChange={e => setEnvPasteText(e.target.value)}
                  />
                  <button
                    type="button"
                    onClick={applyEnvPaste}
                    className="mt-1 px-3 py-1.5 bg-purple-600 text-white text-xs rounded-lg hover:bg-purple-700"
                  >
                    Apply
                  </button>
                </div>
              )}
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

            {/* Databases */}
            {dbList.length > 0 && (
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Linked Databases</label>
                <div className="space-y-1 border rounded-lg p-2">
                  {dbList.map(db => {
                    const checked = form.databases.includes(db.id)
                    return (
                      <label key={db.id} className="flex items-center gap-2 px-2 py-1.5 rounded hover:bg-gray-50 cursor-pointer text-sm">
                        <input
                          type="checkbox"
                          checked={checked}
                          onChange={() =>
                            setForm(prev => ({
                              ...prev,
                              databases: checked
                                ? prev.databases.filter(id => id !== db.id)
                                : [...prev.databases, db.id],
                            }))
                          }
                          className="accent-emerald-600"
                        />
                        <span className="font-medium">{db.name}</span>
                        <span className="text-gray-400 text-xs">{db.type}:{db.version}</span>
                        {checked && (
                          <span className="ml-auto flex items-center gap-1.5">
                            <span className="font-mono text-xs text-emerald-600">
                              {db.name.toUpperCase().replace(/[-\s.]/g, '_')}_URL
                            </span>
                            {form.databases.length === 1 && (
                              <span className="font-mono text-xs text-emerald-600">· DATABASE_URL</span>
                            )}
                          </span>
                        )}
                      </label>
                    )
                  })}
                </div>
                <p className="mt-1 text-xs text-gray-400">
                  {form.databases.length === 1
                    ? 'DATABASE_URL is also set automatically. Add DATABASE_URL to env vars above to override it.'
                    : 'Connection URLs are injected automatically as env vars on deploy.'}
                </p>
              </div>
            )}

            {/* Caches */}
            {cacheList.length > 0 && (
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Linked Caches</label>
                <div className="space-y-1 border rounded-lg p-2">
                  {cacheList.map(c => {
                    const checked = form.caches.includes(c.id)
                    return (
                      <label key={c.id} className="flex items-center gap-2 px-2 py-1.5 rounded hover:bg-gray-50 cursor-pointer text-sm">
                        <input
                          type="checkbox"
                          checked={checked}
                          onChange={() =>
                            setForm(prev => ({
                              ...prev,
                              caches: checked
                                ? prev.caches.filter(id => id !== c.id)
                                : [...prev.caches, c.id],
                            }))
                          }
                          className="accent-blue-600"
                        />
                        <span className="font-medium">{c.name}</span>
                        <span className="text-gray-400 text-xs">redis:{c.version}</span>
                        {checked && (
                          <span className="ml-auto flex items-center gap-1.5">
                            <span className="font-mono text-xs text-blue-600">
                              {c.name.toUpperCase().replace(/[-\s.]/g, '_')}_URL
                            </span>
                            {form.caches.length === 1 && (
                              <span className="font-mono text-xs text-blue-600">· CACHE_URL</span>
                            )}
                          </span>
                        )}
                      </label>
                    )
                  })}
                </div>
                <p className="mt-1 text-xs text-gray-400">
                  {form.caches.length === 1
                    ? 'CACHE_URL is also set automatically. Add CACHE_URL to env vars above to override it.'
                    : 'Connection URLs are injected automatically as env vars on deploy.'}
                </p>
              </div>
            )}

            {/* Kafkas */}
            {kafkaList.length > 0 && (
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Linked Kafka Clusters</label>
                <div className="space-y-1 border rounded-lg p-2">
                  {kafkaList.map(k => {
                    const checked = form.kafkas.includes(k.id)
                    return (
                      <label key={k.id} className="flex items-center gap-2 px-2 py-1.5 rounded hover:bg-gray-50 cursor-pointer text-sm">
                        <input
                          type="checkbox"
                          checked={checked}
                          onChange={() =>
                            setForm(prev => ({
                              ...prev,
                              kafkas: checked
                                ? prev.kafkas.filter(id => id !== k.id)
                                : [...prev.kafkas, k.id],
                            }))
                          }
                          className="accent-orange-600"
                        />
                        <span className="font-medium">{k.name}</span>
                        <span className="text-gray-400 text-xs">kafka:{k.version}</span>
                        {checked && (
                          <span className="ml-auto flex items-center gap-1.5">
                            <span className="font-mono text-xs text-orange-600">
                              {k.name.toUpperCase().replace(/[-\s.]/g, '_')}_URL
                            </span>
                            {form.kafkas.length === 1 && (
                              <span className="font-mono text-xs text-orange-600">· KAFKA_BROKERS</span>
                            )}
                          </span>
                        )}
                      </label>
                    )
                  })}
                </div>
                <p className="mt-1 text-xs text-gray-400">
                  {form.kafkas.length === 1
                    ? 'KAFKA_BROKERS is also set automatically. Add KAFKA_BROKERS to env vars above to override it.'
                    : 'Bootstrap server addresses are injected automatically as env vars on deploy.'}
                </p>
              </div>
            )}

            {/* Monitorings */}
            {monitoringList.length > 0 && (
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Linked Monitoring</label>
                <div className="space-y-1 border rounded-lg p-2">
                  {monitoringList.map(m => {
                    const checked = form.monitorings.includes(m.id)
                    return (
                      <label key={m.id} className="flex items-center gap-2 px-2 py-1.5 rounded hover:bg-gray-50 cursor-pointer text-sm">
                        <input
                          type="checkbox"
                          checked={checked}
                          onChange={() =>
                            setForm(prev => ({
                              ...prev,
                              monitorings: checked
                                ? prev.monitorings.filter(id => id !== m.id)
                                : [...prev.monitorings, m.id],
                            }))
                          }
                          className="accent-teal-600"
                        />
                        <span className="font-medium">{m.name}</span>
                        <span className="text-gray-400 text-xs">:{m.prometheus_port}/{m.grafana_port}</span>
                        {checked && (
                          <span className="ml-auto flex items-center gap-1.5 flex-wrap">
                            <span className="font-mono text-xs text-teal-600">
                              {m.name.toUpperCase().replace(/[-\s.]/g, '_')}_PROMETHEUS_URL
                            </span>
                            {form.monitorings.length === 1 && (
                              <span className="font-mono text-xs text-teal-600">· PROMETHEUS_URL · GRAFANA_URL</span>
                            )}
                          </span>
                        )}
                      </label>
                    )
                  })}
                </div>
                <p className="mt-1 text-xs text-gray-400">
                  {form.monitorings.length === 1
                    ? 'PROMETHEUS_URL and GRAFANA_URL are also set automatically.'
                    : 'Prometheus and Grafana URLs are injected automatically as env vars on deploy.'}
                </p>
              </div>
            )}

            <div className="flex gap-2 justify-end pt-2">
              <button onClick={() => { setShowCreate(false); setEditingId(null); resetForm() }} className="px-4 py-2 text-sm text-gray-600 hover:text-gray-900">
                Cancel
              </button>
              <button
                onClick={handleCreate}
                disabled={loading}
                className="px-4 py-2 bg-purple-600 text-white text-sm rounded-lg hover:bg-purple-700 disabled:opacity-50"
              >
                {loading ? 'Saving...' : editingId ? 'Save Changes' : 'Create App'}
              </button>
            </div>
          </div>
        </Modal>
      )}

      {showComposeWizard && (
        <ComposeImportWizard
          nodes={nodeList}
          onClose={() => setShowComposeWizard(false)}
          onDone={() => {
            setShowComposeWizard(false)
            load()
            databases.list().then(setDbList).catch(() => {})
            caches.list().then(setCacheList).catch(() => {})
            kafkas.list().then(setKafkaList).catch(() => {})
          }}
        />
      )}
    </div>
  )
}
