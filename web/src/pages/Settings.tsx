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
  const [form, setForm] = useState({ github_username: '', github_token: '', webhook_secret: '' })
  const [tokenConfigured, setTokenConfigured] = useState(false)
  const [webhookSecretConfigured, setWebhookSecretConfigured] = useState(false)
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [yamlOpen, setYamlOpen] = useState(false)
  const [webhookOpen, setWebhookOpen] = useState(false)
  const [copied, setCopied] = useState(false)
  const [urlCopied, setUrlCopied] = useState(false)

  const [webhookUrl, setWebhookUrl] = useState('')

  // Cloud provider credentials
  const [doForm, setDoForm] = useState({ do_api_token: '' })
  const [doConfigured, setDoConfigured] = useState(false)
  const [doSaving, setDoSaving] = useState(false)
  const [doSaved, setDoSaved] = useState(false)

  const [awsForm, setAwsForm] = useState({ aws_access_key_id: '', aws_secret_access_key: '' })
  const [awsSecretConfigured, setAwsSecretConfigured] = useState(false)
  const [awsSaving, setAwsSaving] = useState(false)
  const [awsSaved, setAwsSaved] = useState(false)

  useEffect(() => {
    settings.get().then(s => {
      setForm(prev => ({ ...prev, github_username: s.github_username }))
      setTokenConfigured(s.github_token === 'configured')
      setWebhookSecretConfigured(s.webhook_secret === 'configured')
      if (s.webhook_url) setWebhookUrl(s.webhook_url)
      setDoConfigured(s.do_api_token === 'configured')
      if (s.aws_access_key_id) setAwsForm(prev => ({ ...prev, aws_access_key_id: s.aws_access_key_id }))
      setAwsSecretConfigured(s.aws_secret_access_key === 'configured')
    }).catch(e => setError(e.message))
  }, [])

  const handleSave = async () => {
    try {
      setSaving(true)
      setError(null)
      await settings.update(form)
      setSaved(true)
      setTokenConfigured(form.github_token !== '' || tokenConfigured)
      if (form.webhook_secret !== '') setWebhookSecretConfigured(true)
      setForm(prev => ({ ...prev, github_token: '', webhook_secret: '' }))
      setTimeout(() => setSaved(false), 3000)
    } catch (e: unknown) {
      setError((e as Error).message)
    } finally {
      setSaving(false)
    }
  }

  const handleSaveDO = async () => {
    try {
      setDoSaving(true)
      setError(null)
      await settings.update(doForm)
      setDoSaved(true)
      if (doForm.do_api_token !== '') setDoConfigured(true)
      setDoForm({ do_api_token: '' })
      setTimeout(() => setDoSaved(false), 3000)
    } catch (e: unknown) {
      setError((e as Error).message)
    } finally {
      setDoSaving(false)
    }
  }

  const handleSaveAWS = async () => {
    try {
      setAwsSaving(true)
      setError(null)
      await settings.update(awsForm)
      setAwsSaved(true)
      if (awsForm.aws_secret_access_key !== '') setAwsSecretConfigured(true)
      setAwsForm(prev => ({ ...prev, aws_secret_access_key: '' }))
      setTimeout(() => setAwsSaved(false), 3000)
    } catch (e: unknown) {
      setError((e as Error).message)
    } finally {
      setAwsSaving(false)
    }
  }

  const handleCopy = () => {
    navigator.clipboard.writeText(WORKFLOW_YAML).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    })
  }

  const handleUrlCopy = () => {
    navigator.clipboard.writeText(webhookUrl).then(() => {
      setUrlCopied(true)
      setTimeout(() => setUrlCopied(false), 2000)
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

      <div className="bg-white rounded-xl shadow-sm border p-6 max-w-lg w-full">
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

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Webhook Secret
              {webhookSecretConfigured && (
                <span className="ml-2 text-xs text-gray-400 font-normal">(leave blank to keep existing)</span>
              )}
            </label>
            <input
              type="password"
              className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-purple-500"
              value={form.webhook_secret}
              onChange={e => setForm(prev => ({ ...prev, webhook_secret: e.target.value }))}
              placeholder={webhookSecretConfigured ? '••••••••••••••••' : 'Enter a secret to secure the webhook'}
            />
            <p className="mt-1 text-xs text-gray-400">
              Used to verify incoming GitHub webhook requests. Choose any strong random string.
            </p>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Webhook URL</label>
            <div className="flex gap-2">
              <input
                type="text"
                readOnly
                className="flex-1 border rounded-lg px-3 py-2 text-sm bg-gray-50 text-gray-600 font-mono select-all"
                value={webhookUrl}
              />
              <button
                onClick={handleUrlCopy}
                className="px-3 py-2 text-sm bg-gray-100 text-gray-700 rounded-lg hover:bg-gray-200 whitespace-nowrap"
              >
                {urlCopied ? 'Copied!' : 'Copy'}
              </button>
            </div>
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

      {/* Cloud Providers */}
      <div className="mt-8 space-y-6">
        <h2 className="text-lg font-semibold text-gray-800">Cloud Providers</h2>

        {/* DigitalOcean */}
        <div className="bg-white rounded-xl shadow-sm border p-6 max-w-lg w-full">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-base font-semibold text-gray-800">DigitalOcean</h3>
            {doConfigured ? (
              <span className="px-2 py-1 text-xs font-medium bg-green-100 text-green-700 rounded-full">Configured</span>
            ) : (
              <span className="px-2 py-1 text-xs font-medium bg-gray-100 text-gray-500 rounded-full">Not configured</span>
            )}
          </div>
          <div className="space-y-3">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                API Token
                {doConfigured && (
                  <span className="ml-2 text-xs text-gray-400 font-normal">(leave blank to keep existing)</span>
                )}
              </label>
              <input
                type="password"
                className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                value={doForm.do_api_token}
                onChange={e => setDoForm({ do_api_token: e.target.value })}
                placeholder={doConfigured ? '••••••••••••••••' : 'dop_v1_...'}
              />
              <p className="mt-1 text-xs text-gray-400">
                Generate a personal access token from the DigitalOcean control panel.
              </p>
            </div>
            <div className="flex gap-2 items-center">
              <button
                onClick={handleSaveDO}
                disabled={doSaving}
                className="px-4 py-2 bg-blue-600 text-white text-sm rounded-lg hover:bg-blue-700 disabled:opacity-50"
              >
                {doSaving ? 'Saving...' : 'Save'}
              </button>
              {doSaved && <span className="text-sm text-green-600">Saved!</span>}
            </div>
          </div>
        </div>

        {/* AWS */}
        <div className="bg-white rounded-xl shadow-sm border p-6 max-w-lg w-full">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-base font-semibold text-gray-800">AWS</h3>
            {awsSecretConfigured ? (
              <span className="px-2 py-1 text-xs font-medium bg-green-100 text-green-700 rounded-full">Configured</span>
            ) : (
              <span className="px-2 py-1 text-xs font-medium bg-gray-100 text-gray-500 rounded-full">Not configured</span>
            )}
          </div>
          <div className="space-y-3">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Access Key ID</label>
              <input
                type="text"
                className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-orange-500"
                value={awsForm.aws_access_key_id}
                onChange={e => setAwsForm(prev => ({ ...prev, aws_access_key_id: e.target.value }))}
                placeholder="AKIAIOSFODNN7EXAMPLE"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                Secret Access Key
                {awsSecretConfigured && (
                  <span className="ml-2 text-xs text-gray-400 font-normal">(leave blank to keep existing)</span>
                )}
              </label>
              <input
                type="password"
                className="w-full border rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-orange-500"
                value={awsForm.aws_secret_access_key}
                onChange={e => setAwsForm(prev => ({ ...prev, aws_secret_access_key: e.target.value }))}
                placeholder={awsSecretConfigured ? '••••••••••••••••' : 'wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY'}
              />
              <p className="mt-1 text-xs text-gray-400">
                Create an IAM user with EC2 permissions (ec2:RunInstances, ec2:DescribeInstances, ssm:GetParameter, etc.).
              </p>
            </div>
            <div className="flex gap-2 items-center">
              <button
                onClick={handleSaveAWS}
                disabled={awsSaving}
                className="px-4 py-2 bg-orange-600 text-white text-sm rounded-lg hover:bg-orange-700 disabled:opacity-50"
              >
                {awsSaving ? 'Saving...' : 'Save'}
              </button>
              {awsSaved && <span className="text-sm text-green-600">Saved!</span>}
            </div>
          </div>
        </div>
      </div>

      {/* GitHub Actions workflow template */}
      <div className="mt-6 max-w-2xl w-full">
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

      {/* Webhook setup instructions */}
      <div className="mt-4 max-w-2xl w-full">
        <button
          onClick={() => setWebhookOpen(v => !v)}
          className="flex items-center gap-2 text-sm font-medium text-gray-700 hover:text-gray-900"
        >
          <span>{webhookOpen ? '▾' : '▸'}</span>
          How to register the GitHub webhook for auto-redeploy
        </button>
        {webhookOpen && (
          <div className="mt-3 bg-gray-50 border rounded-xl p-4 text-sm text-gray-700 space-y-2">
            <ol className="list-decimal list-inside space-y-2">
              <li>Go to your GitHub repository → <strong>Settings</strong> → <strong>Webhooks</strong> → <strong>Add webhook</strong></li>
              <li>Set <strong>Payload URL</strong> to the Webhook URL shown above</li>
              <li>Set <strong>Content type</strong> to <code className="bg-gray-200 px-1 rounded">application/json</code></li>
              <li>Set <strong>Secret</strong> to the same value as the Webhook Secret you configured above</li>
              <li>Under <em>Which events</em>, choose <strong>Let me select individual events</strong> and tick <strong>Registry packages</strong></li>
              <li>Click <strong>Add webhook</strong>. GitHub will send a ping event to verify the URL.</li>
            </ol>
            <p className="text-xs text-gray-500 pt-1">
              Once configured, every time a new image is published to GHCR for this repository, all running deployments for the matching application will be automatically updated.
            </p>
          </div>
        )}
      </div>
    </div>
  )
}
