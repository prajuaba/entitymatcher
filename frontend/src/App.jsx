import React, { useEffect } from 'react'
import { useMatcherStore } from './store/useMatcherStore'
import { MasterDetailView } from './components/MasterDetailView'
import { ProgressDashboard } from './components/ProgressDashboard'
import { ConfigPanel } from './components/ConfigPanel'
import { FileUpload } from './components/FileUpload'
import { AuditDashboard } from './components/AuditDashboard'
import { ManualSearchModal } from './components/ManualSearchModal'
import { LLMAnalysisModal } from './components/LLMAnalysisModal'
import { Cpu, Sliders, Activity, Database, FileCheck, ShieldCheck } from 'lucide-react'

export function App() {
  const { activeTab, setActiveTab, loadSeedDataset, progress, totalCount } = useMatcherStore()

  useEffect(() => {
    // Load benchmark dataset on boot for out-of-the-box demonstration
    loadSeedDataset()
  }, [])

  const navItems = [
    { id: 'results', label: 'Review Queue', icon: FileCheck },
    { id: 'progress', label: 'Execution Dashboard', icon: Activity, badge: progress.status === 'RUNNING' },
    { id: 'audit', label: 'Audit Trail & Governance', icon: ShieldCheck },
    { id: 'config', label: 'Engine Configuration', icon: Sliders },
    { id: 'ingestion', label: 'Ingestion & Benchmarks', icon: Database },
  ]

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

        {/* Tab Navigation Controls */}
        <nav className="flex items-center gap-1.5 bg-slate-950 p-1 rounded-xl border border-slate-800">
          {navItems.map((item) => {
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
