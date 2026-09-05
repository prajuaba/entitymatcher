import React, { useEffect } from 'react'
import { useMatcherStore } from './store/useMatcherStore'
import { MasterDetailView } from './components/MasterDetailView'
import { ProgressDashboard } from './components/ProgressDashboard'
import { ConfigPanel } from './components/ConfigPanel'
import { FileUpload } from './components/FileUpload'
import { AuditDashboard } from './components/AuditDashboard'
import { ManualSearchModal } from './components/ManualSearchModal'
import { LLMAnalysisModal } from './components/LLMAnalysisModal'
import { LoginScreen } from './components/LoginScreen'
import { Cpu, Sliders, Activity, Database, FileCheck, ShieldCheck, LogOut } from 'lucide-react'
import { can } from './lib/rbac'

export function App() {
  const { activeTab, setActiveTab, loadSeedDataset, progress, totalCount, authChecked, user, logout, initAuth, batchID, fetchConfig } = useMatcherStore()

  useEffect(() => {
    // Initialize auth on mount
    initAuth()
  }, [])

  useEffect(() => {
    // Only seed when there is no batch to return to. The store deliberately
    // rehydrates batchID from localStorage so a reload keeps the batch the user
    // was reviewing; seeding unconditionally overwrote that remembered id with
    // the demo batch on every page load.
    // Pull the server's saved config into the store. Without this the store keeps
    // its hardcoded defaults, which carry no column_mapping at all -- so FieldMapper
    // fell back to its own blank defaults and saving from that screen overwrote the
    // real mapping (this is how date_field_src/date_field_dest got silently cleared).
    if (authChecked && user) {
      fetchConfig()
    }
    if (authChecked && user && !batchID) {
      // Load benchmark dataset on boot for out-of-the-box demonstration
      loadSeedDataset()
    }
  }, [authChecked])

  // Listen for unauthorized events
  useEffect(() => {
    const handleUnauthorized = () => {
      logout()
    }
    window.addEventListener('auth:unauthorized', handleUnauthorized)
    return () => window.removeEventListener('auth:unauthorized', handleUnauthorized)
  }, [])

  const navItems = [
    { id: 'results', label: 'Review Queue', icon: FileCheck, capability: 'REVIEW_QUEUE' },
    { id: 'progress', label: 'Execution Dashboard', icon: Activity, badge: progress.status === 'RUNNING', capability: 'EXECUTION_DASHBOARD' },
    { id: 'audit', label: 'Audit Trail & Governance', icon: ShieldCheck, capability: 'AUDIT_TRAIL' },
    { id: 'config', label: 'Engine Configuration', icon: Sliders, capability: 'ENGINE_CONFIG' },
    { id: 'ingestion', label: 'Ingestion & Benchmarks', icon: Database, capability: 'INGESTION_BENCHMARKS' },
  ]

  // Filter nav items based on user role
  const allowedNavItems = navItems.filter(item => !item.capability || can(user, item.capability))

  // Redirect to first allowed tab if current tab is not allowed
  useEffect(() => {
    if (authChecked && user && activeTab) {
      const isAllowed = allowedNavItems.some(item => item.id === activeTab)
      if (!isAllowed && allowedNavItems.length > 0) {
        setActiveTab(allowedNavItems[0].id)
      }
    }
  }, [authChecked, user])

  // Show loading state while auth is being checked
  if (!authChecked) {
    return (
      <div className="flex flex-col items-center justify-center h-screen bg-slate-950 text-slate-100">
        <div className="flex flex-col items-center gap-4">
          <div className="p-3 bg-gradient-to-tr from-sky-600 to-emerald-500 text-white rounded-xl shadow-lg shadow-sky-950 animate-pulse">
            <Cpu className="w-8 h-8" />
          </div>
          <p className="text-sm text-slate-400">Initializing...</p>
        </div>
      </div>
    )
  }

  // Show login screen if not authenticated
  if (!user) {
    return <LoginScreen />
  }

  return (
    <div className="flex flex-col h-screen bg-slate-950 text-slate-100 font-sans antialiased overflow-hidden">
      {/* Top Navigation Bar */}
      <header className="bg-slate-900 border-b border-slate-800 px-6 py-3 shrink-0 flex flex-wrap items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <div className="p-2 bg-gradient-to-tr from-sky-600 to-emerald-500 text-white rounded-xl shadow-lg shadow-sky-950">
            <Cpu className="w-5 h-5" />
          </div>
          <div>
            <h1 className="text-base font-bold text-slate-100 tracking-tight flex items-center gap-2">
              Entity Matcher <span className="px-2 py-0.5 bg-sky-950 text-sky-400 border border-sky-800/40 rounded text-[10px] font-mono">TH / EN 100K+ Engine</span>
            </h1>
            <p className="text-[11px] text-slate-400">High-Scale Bilingual Thai-English Resolution Engine</p>
          </div>
        </div>

        {/* User Profile & Logout */}
        <div className="flex items-center gap-2.5 bg-slate-950/80 p-1.5 pl-3 rounded-xl border border-slate-800 shrink-0">
          <div className="flex items-center gap-2">
            <span className="text-xs font-semibold text-slate-100">{user.name}</span>
            <span className={`text-[10px] font-mono px-1.5 py-0.5 rounded border ${
              user.role === 'ADMIN' ? 'bg-rose-950/80 text-rose-300 border-rose-700/50' :
              user.role === 'ENGINEER' ? 'bg-sky-950/80 text-sky-300 border-sky-700/50' :
              user.role === 'REVIEWER' ? 'bg-emerald-950/80 text-emerald-300 border-emerald-700/50' :
              'bg-amber-950/80 text-amber-300 border-amber-700/50'
            }`}>
              {user.role}
            </span>
          </div>
          <div className="w-px h-4 bg-slate-800" />
          <button
            onClick={logout}
            title="Logout"
            className="p-1.5 hover:bg-slate-800 text-slate-400 hover:text-rose-400 rounded-lg text-xs font-medium flex items-center gap-1 transition"
          >
            <LogOut className="w-3.5 h-3.5" />
            <span className="hidden sm:inline text-[11px]">Logout</span>
          </button>
        </div>

        {/* Tab Navigation Controls */}
        <nav className="flex items-center gap-1.5 bg-slate-950 p-1 rounded-xl border border-slate-800 ml-auto">
          {allowedNavItems.map((item) => {
            const Icon = item.icon
            const isActive = activeTab === item.id
            return (
              <button
                key={item.id}
                onClick={() => setActiveTab(item.id)}
                className={`relative px-4 py-2 rounded-lg text-xs font-semibold flex items-center gap-2 transition ${
                  isActive
                    ? 'bg-sky-600 text-white shadow-md'
                    : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900'
                }`}
              >
                <Icon className="w-4 h-4" />
                <span>{item.label}</span>
                {item.badge && <span className="w-2 h-2 rounded-full bg-emerald-400 animate-ping"></span>}
              </button>
            )
          })}
        </nav>
      </header>

      {/* Main Content Area */}
      <main className="flex-1 p-6 overflow-y-auto min-h-0">
        {activeTab === 'results' && <MasterDetailView />}
        {activeTab === 'progress' && <ProgressDashboard />}
        {activeTab === 'audit' && <AuditDashboard />}
        {activeTab === 'config' && <ConfigPanel />}
        {activeTab === 'ingestion' && <FileUpload />}
      </main>

      {/* Interactive Modals */}
      <ManualSearchModal />
      <LLMAnalysisModal />
    </div>
  )
}

export default App
