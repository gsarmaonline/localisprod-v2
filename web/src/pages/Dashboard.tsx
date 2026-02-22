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
    { label: 'Nodes', value: stats.nodes, color: 'bg-blue-50 text-blue-700' },
    { label: 'Applications', value: stats.applications, color: 'bg-purple-50 text-purple-700' },
    { label: 'Running', value: running, color: 'bg-green-50 text-green-700' },
    { label: 'Pending', value: pending, color: 'bg-yellow-50 text-yellow-700' },
    { label: 'Stopped', value: stopped, color: 'bg-orange-50 text-orange-700' },
    { label: 'Failed', value: failed, color: 'bg-red-50 text-red-700' },
  ]

  return (
    <div>
      <h1 className="text-2xl font-bold text-gray-900 mb-6">Dashboard</h1>
      <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
        {cards.map(c => (
          <div key={c.label} className={`rounded-xl p-6 ${c.color}`}>
            <p className="text-sm font-medium opacity-70">{c.label}</p>
            <p className="text-4xl font-bold mt-1">{c.value}</p>
          </div>
        ))}
      </div>
    </div>
  )
}
