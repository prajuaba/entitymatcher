import React, { useState, useEffect } from 'react'
import { useMatcherStore } from '../store/useMatcherStore'
import { ShieldCheck, Download, Filter, UserCheck, CheckCircle2, XCircle, Clock, FileSpreadsheet } from 'lucide-react'

export function AuditDashboard() {
  const { currentBatchId } = useMatcherStore()
  const [logs, setLogs] = useState([])
  const [loading, setLoading] = useState(false)
  const [userFilter, setUserFilter] = useState('')
  const [actionFilter, setActionFilter] = useState('')

  const fetchAuditLogs = async () => {
    setLoading(true)
    try {
      const batchParam = currentBatchId ? `batch_id=${currentBatchId}` : ''
      const userParam = userFilter ? `&user_id=${encodeURIComponent(userFilter)}` : ''
      const actionParam = actionFilter ? `&action=${encodeURIComponent(actionFilter)}` : ''
      const res = await fetch(`/api/audit/logs?${batchParam}${userParam}${actionParam}`)
      const data = await res.json()
      setLogs(data.logs || [])
    } catch (e) {
      console.error('Failed to fetch audit logs:', e)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchAuditLogs()
  }, [currentBatchId, userFilter, actionFilter])

  const exportAuditCSV = () => {
    const batchParam = currentBatchId ? `?batch_id=${currentBatchId}` : ''
    window.open(`/api/audit/export${batchParam}`, '_blank')
  }

  return (
    <div className="max-w-6xl mx-auto space-y-6 bg-slate-900/60 p-8 rounded-2xl border border-slate-800">
      <div className="flex flex-wrap items-center justify-between gap-4 border-b border-slate-800 pb-4">
        <div>
          <h2 className="text-xl font-bold text-slate-100 flex items-center gap-2">
            <ShieldCheck className="w-5 h-5 text-emerald-400" /> Compliance Audit Trail & Governance Log
          </h2>
          <p className="text-xs text-slate-400 mt-1">Immutable log of human review decisions, reviewer IDs, timestamps, and compliance rationale.</p>
        </div>

        <div className="flex items-center gap-3">
          <button
            onClick={exportAuditCSV}
            className="px-4 py-2 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg text-xs font-semibold flex items-center gap-2 transition shadow-sm"
          >
            <Download className="w-4 h-4" /> Export Compliance Audit CSV
          </button>
        </div>
      </div>

      {/* Filters Bar */}
      <div className="flex flex-wrap items-center gap-4 bg-slate-950/80 p-4 rounded-xl border border-slate-800 text-xs">
        <div className="flex items-center gap-2">
          <Filter className="w-4 h-4 text-sky-400" />
          <span className="font-semibold text-slate-300">Filter By:</span>
        </div>

        <input
          type="text"
          placeholder="Filter by Reviewer User ID"
          value={userFilter}
          onChange={(e) => setUserFilter(e.target.value)}
          className="bg-slate-900 border border-slate-800 rounded p-2 text-slate-200"
        />

        <select
          value={actionFilter}
          onChange={(e) => setActionFilter(e.target.value)}
          className="bg-slate-900 border border-slate-800 rounded p-2 text-slate-200 font-semibold"
        >
          <option value="">All Review Actions</option>
          <option value="CONFIRM">CONFIRMED MATCHES</option>
          <option value="REJECT">REJECTED MATCHES</option>
          <option value="OVERRIDE">MANUAL OVERRIDES</option>
        </select>

        <span className="text-slate-500 font-mono ml-auto">Total Log Entries: {logs.length}</span>
      </div>

      {/* Audit Logs Table */}
      <div className="bg-slate-950/80 rounded-xl border border-slate-800 overflow-hidden">
        {logs.length === 0 ? (
          <div className="p-8 text-center text-slate-500 text-xs italic">
            No audit log entries recorded yet. Perform reviewer actions (Approve / Reject) in the Review Queue tab to populate logs.
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs">
              <thead className="bg-slate-900 border-b border-slate-800 text-slate-400 font-semibold uppercase tracking-wider">
                <tr>
                  <th className="py-3 px-4">Timestamp</th>
                  <th className="py-3 px-4">Reviewer ID</th>
                  <th className="py-3 px-4">Action</th>
                  <th className="py-3 px-4">Confidence</th>
                  <th className="py-3 px-4">Previous ➔ New Status</th>
                  <th className="py-3 px-4">Compliance Rationale / Comments</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-900 text-slate-300">
                {logs.map((log) => (
                  <tr key={log.id} className="hover:bg-slate-900/40 transition">
                    <td className="py-3 px-4 font-mono text-slate-400">
                      {new Date(log.timestamp).toLocaleString()}
                    </td>
                    <td className="py-3 px-4 font-mono text-sky-400 font-medium">
                      <span className="flex items-center gap-1.5">
                        <UserCheck className="w-3.5 h-3.5" /> {log.user_id}
                      </span>
                    </td>
                    <td className="py-3 px-4">
                      {log.action === 'CONFIRM' ? (
                        <span className="px-2 py-0.5 bg-emerald-950 text-emerald-300 border border-emerald-800 rounded font-bold text-[10px] flex items-center gap-1 w-fit">
                          <CheckCircle2 className="w-3 h-3" /> CONFIRMED
                        </span>
                      ) : (
                        <span className="px-2 py-0.5 bg-rose-950 text-rose-300 border border-rose-800 rounded font-bold text-[10px] flex items-center gap-1 w-fit">
                          <XCircle className="w-3 h-3" /> REJECTED
                        </span>
                      )}
                    </td>
                    <td className="py-3 px-4 font-mono font-bold text-slate-200">
                      {(log.confidence_score * 100).toFixed(2)}%
                    </td>
                    <td className="py-3 px-4 font-mono text-[11px]">
                      <span className="text-slate-500">{log.previous_status}</span> ➔ <span className="text-slate-200 font-bold">{log.new_status}</span>
                    </td>
                    <td className="py-3 px-4 text-slate-300 italic">
                      {log.review_comments || 'No reviewer comment provided.'}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}
