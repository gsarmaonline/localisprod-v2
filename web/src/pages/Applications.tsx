import { useEffect, useState } from 'react'
import { applications, github, settings, Application, CreateApplicationInput, GithubRepo } from '../api/client'
import Modal from '../components/Modal'

export default function Applications() {
  const [appList, setAppList] = useState<Application[]>([])
  const [showCreate, setShowCreate] = useState(false)
  const [showRepoPicker, setShowRepoPicker] = useState(false)
  const [repos, setRepos] = useState<GithubRepo[]>([])
  const [reposLoading, setReposLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const [form, setForm] = useState<{
    name: string
    docker_image: string
    command: string
    github_repo: string
    domain: string
    envPairs: { key: string; value: string }[]
    ports: string[]
  }>({
    name: '', docker_image: '', command: '', github_repo: '', domain: '',
    envPairs: [{ key: '', value: '' }],
    ports: [''],
  })

  const resetForm = () =>
    setForm({ name: '', docker_image: '', command: '', github_repo: '', domain: '', envPairs: [{ key: '', value: '' }], ports: [''] })

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
        github_repo: form.github_repo || undefined,
        domain: form.domain || undefined,
      }
      await applications.create(data)
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
      command: '',
      github_repo: repo.full_name,
      domain: '',
      envPairs: [{ key: '', value: '' }],
      ports: [''],
    })
    setShowCreate(true)
  }

  const parsePorts = (s: string) => {
    try { return JSON.parse(s) as string[] } catch { return [] }
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Applications</h1>
        <div className="flex gap-2">
          <button
            onClick={handleFromGithub}
            className="px-4 py-2 bg-gray-800 text-white rounded-lg hover:bg-gray-900 text-sm font-medium flex items-center gap-2"
          >
            <svg className="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0 0 24 12c0-6.63-5.37-12-12-12z" />
            </svg>
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

      <div className="bg-white rounded-xl shadow-sm border overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-gray-50 border-b">
            <tr>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Name</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Image</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Ports</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Domain</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Command</th>
              <th className="text-left px-4 py-3 font-medium text-gray-600">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y">
            {appList.length === 0 && (
              <tr>
                <td colSpan={6} className="px-4 py-8 text-center text-gray-400">No applications yet</td>
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
                      <svg className="w-3.5 h-3.5 inline" viewBox="0 0 24 24" fill="currentColor">
                        <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0 0 24 12c0-6.63-5.37-12-12-12z" />
                      </svg>
                    </a>
                  )}
                </td>
                <td className="px-4 py-3 text-gray-600 font-mono text-xs">{a.docker_image}</td>
                <td className="px-4 py-3 text-gray-600">
                  {parsePorts(a.ports).join(', ') || '—'}
                </td>
                <td className="px-4 py-3 text-gray-600 font-mono text-xs">{a.domain || '—'}</td>
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
