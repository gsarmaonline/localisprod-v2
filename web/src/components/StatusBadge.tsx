interface StatusBadgeProps {
  status: string
}

const colors: Record<string, string> = {
  online: 'bg-green-100 text-green-800',
  offline: 'bg-red-100 text-red-800',
  unknown: 'bg-gray-100 text-gray-600',
  running: 'bg-green-100 text-green-800',
  stopped: 'bg-yellow-100 text-yellow-800',
  failed: 'bg-red-100 text-red-800',
  pending: 'bg-blue-100 text-blue-800',
}

export default function StatusBadge({ status }: StatusBadgeProps) {
  const cls = colors[status] ?? 'bg-gray-100 text-gray-600'
  return (
    <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${cls}`}>
      {status}
    </span>
  )
}
