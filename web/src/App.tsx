import { useState } from 'react'
import { BrowserRouter, NavLink, Route, Routes } from 'react-router-dom'
import { AuthProvider, useAuth } from './contexts/AuthContext'
import ProtectedRoute from './components/ProtectedRoute'
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome'
import {
  faGaugeHigh, faServer, faBoxOpen, faBolt, faGear,
  faBars, faXmark, faDatabase, faMemory, faStream, faChartLine, faCloud,
} from '@fortawesome/free-solid-svg-icons'
import Dashboard from './pages/Dashboard'
import Nodes from './pages/Nodes'
import Applications from './pages/Applications'
import Databases from './pages/Databases'
import Caches from './pages/Caches'
import Kafkas from './pages/Kafkas'
import Monitorings from './pages/Monitorings'
import Deployments from './pages/Deployments'
import Settings from './pages/Settings'
import Providers from './pages/Providers'
import Login from './pages/Login'

const navItems = [
  { to: '/', label: 'Dashboard', icon: faGaugeHigh },
  { to: '/nodes', label: 'Nodes', icon: faServer },
  { to: '/applications', label: 'Applications', icon: faBoxOpen },
  { to: '/databases', label: 'Databases', icon: faDatabase },
  { to: '/caches', label: 'Cache', icon: faMemory },
  { to: '/kafkas', label: 'Kafka', icon: faStream },
  { to: '/monitorings', label: 'Monitoring', icon: faChartLine },
  { to: '/deployments', label: 'Deployments', icon: faBolt },
  { to: '/providers', label: 'Providers', icon: faCloud },
  { to: '/settings', label: 'Settings', icon: faGear },
]

function AppLayout() {
  const { user, logout } = useAuth()
  const [sidebarOpen, setSidebarOpen] = useState(false)

  return (
    <div className="min-h-screen bg-gradient-to-br from-slate-50 to-gray-100 flex">
      {/* Mobile backdrop */}
      {sidebarOpen && (
        <div
          className="fixed inset-0 z-30 bg-black/50 md:hidden"
          onClick={() => setSidebarOpen(false)}
        />
      )}

      {/* Sidebar */}
      <aside className={`fixed inset-y-0 left-0 z-40 w-56 bg-gradient-to-b from-slate-900 to-slate-800 text-white flex flex-col transition-transform duration-200 md:relative md:translate-x-0 md:z-auto ${sidebarOpen ? 'translate-x-0' : '-translate-x-full'}`}>
        <div className="px-4 py-5 border-b border-white/10 flex items-center justify-between">
          <div>
            <h1 className="font-semibold text-base tracking-tight bg-gradient-to-r from-white to-slate-300 bg-clip-text text-transparent">Localisprod</h1>
            <p className="text-xs text-slate-400 mt-0.5">Cluster Manager</p>
          </div>
          <button
            className="md:hidden text-gray-400 hover:text-white p-1"
            onClick={() => setSidebarOpen(false)}
            aria-label="Close menu"
          >
            <FontAwesomeIcon icon={faXmark} className="w-4 h-4" />
          </button>
        </div>
        <nav className="flex-1 px-2 py-4 space-y-1">
          {navItems.map(item => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.to === '/'}
              onClick={() => setSidebarOpen(false)}
              className={({ isActive }) =>
                `flex items-center gap-3 px-3 py-2 rounded-lg text-sm transition-all duration-150 ${
                  isActive
                    ? 'bg-gradient-to-r from-indigo-500/25 to-indigo-500/10 text-white font-medium'
                    : 'text-slate-400 hover:text-white hover:bg-white/5'
                }`
              }
            >
              <FontAwesomeIcon icon={item.icon} className="w-4 h-4" />
              {item.label}
            </NavLink>
          ))}
        </nav>
        {user && (
          <div className="px-4 py-4 border-t border-white/10">
            <p className="text-xs text-slate-400 truncate mb-2">{user.email}</p>
            <button
              onClick={logout}
              className="text-xs text-slate-400 hover:text-white transition-colors"
            >
              Sign out
            </button>
          </div>
        )}
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-auto min-w-0">
        {/* Mobile top bar */}
        <div className="md:hidden flex items-center px-4 py-3 bg-gradient-to-r from-slate-900 to-slate-800 text-white">
          <button
            onClick={() => setSidebarOpen(true)}
            className="text-gray-400 hover:text-white mr-3"
            aria-label="Open menu"
          >
            <FontAwesomeIcon icon={faBars} className="w-5 h-5" />
          </button>
          <span className="font-bold text-sm tracking-tight">Localisprod</span>
        </div>
        <div className="p-4 sm:p-8">
          <Routes>
            <Route path="/" element={<Dashboard />} />
            <Route path="/nodes" element={<Nodes />} />
            <Route path="/applications" element={<Applications />} />
            <Route path="/databases" element={<Databases />} />
            <Route path="/caches" element={<Caches />} />
            <Route path="/kafkas" element={<Kafkas />} />
            <Route path="/monitorings" element={<Monitorings />} />
            <Route path="/deployments" element={<Deployments />} />
            <Route path="/providers" element={<Providers />} />
            <Route path="/settings" element={<Settings />} />
          </Routes>
        </div>
      </main>
    </div>
  )
}

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route path="/login" element={<Login />} />
          <Route
            path="/*"
            element={
              <ProtectedRoute>
                <AppLayout />
              </ProtectedRoute>
            }
          />
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  )
}
