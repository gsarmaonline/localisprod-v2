import { useEffect, useState } from 'react'
import { settings } from '../api/client'

const WORKFLOW_YAML = `name: Build and Push to GHCR

on:
  push:
    branches: [main]

jobs:
  build:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write
    steps:
      - uses: actions/checkout@v4

      - name: Log in to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: \${{ github.actor }}
          password: \${{ secrets.GITHUB_TOKEN }}

      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          push: true
          tags: ghcr.io/\${{ github.repository }}:latest
`

export default function Settings() {
  const [form, setForm] = useState({ github_username: '', github_token: '' })
  const [tokenConfigured, setTokenConfigured] = useState(false)
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [yamlOpen, setYamlOpen] = useState(false)
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    settings.get().then(s => {
      setForm(prev => ({ ...prev, github_username: s.github_username }))
      setTokenConfigured(s.github_token === 'configured')
    }).catch(e => setError(e.message))
  }, [])

  const handleSave = async () => {
    try {
      setSaving(true)
      setError(null)
      await settings.update(form)
      setSaved(true)
      setTokenConfigured(form.github_token !== '')
      setForm(prev => ({ ...prev, github_token: '' }))
      setTimeout(() => setSaved(false), 3000)
    } catch (e: unknown) {
      setError((e as Error).message)
    } finally {
      setSaving(false)
    }
  }

  const handleCopy = () => {
    navigator.clipboard.writeText(WORKFLOW_YAML).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

  return (
    <div>
      <h1 className="text-2xl font-bold text-gray-900 mb-6">Settings</h1>

      {error && (
        <div className="mb-4 p-3 bg-red-50 text-red-700 rounded-lg text-sm">
          {error}
          <button onClick={() => setError(null)} className="ml-2 underline">dismiss</button>
        </div>
      )}

      <div className="bg-white rounded-xl shadow-sm border p-6 max-w-lg">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold text-gray-800">GitHub Integration</h2>
          {tokenConfigured ? (
            <span className="px-2 py-1 text-xs font-medium bg-green-100 text-green-700 rounded-full">Connected</span>
          ) : (
            <span className="px-2 py-1 text-xs font-medium bg-gray-100 text-gray-500 rounded-full">Not configured</span>
          )}
        </div>

        <div className="space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">GitHub Username</label>
            <input
              type="text"
              className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-purple-500"
              value={form.github_username}
              onChange={e => setForm(prev => ({ ...prev, github_username: e.target.value }))}
              placeholder="your-github-username"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Personal Access Token
              {tokenConfigured && (
                <span className="ml-2 text-xs text-gray-400 font-normal">(leave blank to keep existing)</span>
              )}
            </label>
            <input
              type="password"
              className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-purple-500"
              value={form.github_token}
              onChange={e => setForm(prev => ({ ...prev, github_token: e.target.value }))}
              placeholder={tokenConfigured ? '••••••••••••••••' : 'ghp_...'}
            />
            <p className="mt-1 text-xs text-gray-400">
              Needs <code>read:packages</code> scope. Used to authenticate <code>docker pull</code> from ghcr.io.
            </p>
          </div>

          <div className="flex gap-2 items-center pt-1">
            <button
              onClick={handleSave}
              disabled={saving}
              className="px-4 py-2 bg-purple-600 text-white text-sm rounded-lg hover:bg-purple-700 disabled:opacity-50"
            >
              {saving ? 'Saving...' : 'Save'}
            </button>
            {saved && <span className="text-sm text-green-600">Saved!</span>}
          </div>
        </div>
      </div>

      {/* GitHub Actions workflow template */}
      <div className="mt-6 max-w-2xl">
        <button
          onClick={() => setYamlOpen(v => !v)}
          className="flex items-center gap-2 text-sm font-medium text-gray-700 hover:text-gray-900"
        >
          <span>{yamlOpen ? '▾' : '▸'}</span>
          GitHub Actions workflow to build & push to ghcr.io
        </button>
        {yamlOpen && (
          <div className="mt-3 relative">
            <pre className="bg-gray-900 text-gray-100 rounded-xl p-4 text-xs overflow-auto leading-relaxed">
              {WORKFLOW_YAML}
            </pre>
            <button
              onClick={handleCopy}
              className="absolute top-3 right-3 px-2 py-1 text-xs bg-gray-700 text-gray-200 rounded hover:bg-gray-600"
            >
              {copied ? 'Copied!' : 'Copy'}
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
