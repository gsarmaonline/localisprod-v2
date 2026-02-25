import { useEffect, useState } from 'react'
import { dashboard, Stats } from '../api/client'

export default function Dashboard() {
  const [stats, setStats] = useState<Stats | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    dashboard.stats()
      .then(setStats)
      .catch(e => setError(e.message))
  }, [])

  if (error) return <p className="text-red-600 p-6">Error: {error}</p>
  if (!stats) return <p className="text-gray-500 p-6">Loading...</p>

  const dep = stats.deployments ?? {}
  const running = dep['running'] ?? 0
  const stopped = dep['stopped'] ?? 0
  const failed = dep['failed'] ?? 0
  const pending = dep['pending'] ?? 0

  const cards = [
    { label: 'Nodes', value: stats.nodes, color: 'bg-gradient-to-br from-blue-50 to-indigo-100 text-blue-700' },
    { label: 'Applications', value: stats.applications, color: 'bg-gradient-to-br from-violet-50 to-purple-100 text-purple-700' },
    { label: 'Running', value: running, color: 'bg-gradient-to-br from-emerald-50 to-green-100 text-green-700' },
    { label: 'Pending', value: pending, color: 'bg-gradient-to-br from-amber-50 to-yellow-100 text-yellow-700' },
    { label: 'Stopped', value: stopped, color: 'bg-gradient-to-br from-orange-50 to-orange-100 text-orange-700' },
    { label: 'Failed', value: failed, color: 'bg-gradient-to-br from-red-50 to-rose-100 text-red-700' },
  ]

  return (
    <div>
      <h1 className="text-2xl font-semibold text-gray-900 mb-6 tracking-tight">Dashboard</h1>
      <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
        {cards.map(c => (
          <div key={c.label} className={`rounded-xl p-6 shadow-sm ${c.color}`}>
            <p className="text-sm font-medium opacity-60 uppercase tracking-wide">{c.label}</p>
            <p className="text-4xl font-bold mt-1">{c.value}</p>
          </div>
        ))}
      </div>
    </div>
  )
}
