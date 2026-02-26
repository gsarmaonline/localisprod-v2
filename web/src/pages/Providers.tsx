import { useEffect, useState } from 'react'
import { settings } from '../api/client'

export default function Providers() {
  const [error, setError] = useState<string | null>(null)

  // DigitalOcean
  const [doForm, setDoForm] = useState({ do_api_token: '' })
  const [doConfigured, setDoConfigured] = useState(false)
  const [doSaving, setDoSaving] = useState(false)
  const [doSaved, setDoSaved] = useState(false)

  // AWS
  const [awsForm, setAwsForm] = useState({ aws_access_key_id: '', aws_secret_access_key: '' })
  const [awsSecretConfigured, setAwsSecretConfigured] = useState(false)
  const [awsSaving, setAwsSaving] = useState(false)
  const [awsSaved, setAwsSaved] = useState(false)

  useEffect(() => {
    settings.get().then(s => {
      setDoConfigured(s.do_api_token === 'configured')
      if (s.aws_access_key_id) setAwsForm(prev => ({ ...prev, aws_access_key_id: s.aws_access_key_id }))
      setAwsSecretConfigured(s.aws_secret_access_key === 'configured')
    }).catch(e => setError(e.message))
  }, [])

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

  return (
    <div>
      <h1 className="text-2xl font-bold text-gray-900 mb-6">Providers</h1>

      {error && (
        <div className="mb-4 p-3 bg-red-50 text-red-700 rounded-lg text-sm">
          {error}
          <button onClick={() => setError(null)} className="ml-2 underline">dismiss</button>
        </div>
      )}

      <div className="space-y-6">
        {/* DigitalOcean */}
        <div className="bg-white rounded-xl shadow-sm border p-6 max-w-lg w-full">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-base font-semibold text-gray-800">DigitalOcean</h2>
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
            <h2 className="text-base font-semibold text-gray-800">AWS</h2>
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
    </div>
  )
}
