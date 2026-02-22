import { BrowserRouter, NavLink, Route, Routes } from 'react-router-dom'
import Dashboard from './pages/Dashboard'
import Nodes from './pages/Nodes'
import Applications from './pages/Applications'
import Deployments from './pages/Deployments'
import Settings from './pages/Settings'

const navItems = [
  { to: '/', label: 'Dashboard', icon: '▦' },
  { to: '/nodes', label: 'Nodes', icon: '⬡' },
  { to: '/applications', label: 'Applications', icon: '⬜' },
  { to: '/deployments', label: 'Deployments', icon: '⚡' },
  { to: '/settings', label: 'Settings', icon: '⚙' },
]

export default function App() {
  return (
    <BrowserRouter>
      <div className="min-h-screen bg-gray-100 flex">
        {/* Sidebar */}
        <aside className="w-56 bg-gray-900 text-white flex flex-col">
          <div className="px-4 py-5 border-b border-gray-700">
            <h1 className="font-bold text-base tracking-tight">Localisprod</h1>
            <p className="text-xs text-gray-400 mt-0.5">Cluster Manager</p>
          </div>
          <nav className="flex-1 px-2 py-4 space-y-1">
            {navItems.map(item => (
              <NavLink
                key={item.to}
                to={item.to}
                end={item.to === '/'}
                className={({ isActive }) =>
                  `flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-colors ${
                    isActive
                      ? 'bg-gray-700 text-white'
                      : 'text-gray-400 hover:text-white hover:bg-gray-800'
                  }`
                }
              >
                <span>{item.icon}</span>
                {item.label}
              </NavLink>
            ))}
          </nav>
        </aside>

        {/* Main content */}
        <main className="flex-1 p-8 overflow-auto">
          <Routes>
            <Route path="/" element={<Dashboard />} />
            <Route path="/nodes" element={<Nodes />} />
            <Route path="/applications" element={<Applications />} />
            <Route path="/deployments" element={<Deployments />} />
            <Route path="/settings" element={<Settings />} />
          </Routes>
        </main>
      </div>
    </BrowserRouter>
  )
}
