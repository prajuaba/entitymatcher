import React from 'react'
import { useMatcherStore } from '../store/useMatcherStore'
import { Activity, CheckCircle2, Clock, Zap, ArrowRight, RefreshCw } from 'lucide-react'

export function ProgressDashboard() {
  const { progress, batchID, runMatching, setActiveTab, loading } = useMatcherStore()

  const { total_sources, processed_sources, total_candidate_pairs, no_match_count, total_decisions, auto_matched, review_needed, status, elapsed_ms } = progress

  const percentage = total_sources > 0 ? Math.round((processed_sources / total_sources) * 100) : 0
  const recordsPerSec = elapsed_ms > 0 ? Math.round((processed_sources / (elapsed_ms / 1000))) : 0

  return (
    <div className="max-w-4xl mx-auto space-y-6 bg-slate-900/60 p-8 rounded-2xl border border-slate-800">
      <div className="flex items-center justify-between border-b border-slate-800 pb-4">
        <div>
          <span className="text-xs font-mono text-slate-400 uppercase tracking-widest">Batch execution ID: {batchID || 'benchmark-batch-001'}</span>
          <h2 className="text-xl font-bold text-slate-100 flex items-center gap-2 mt-0.5">
            <Activity className="w-5 h-5 text-sky-400" /> Real-Time Engine Execution Stream
          </h2>
        </div>
        <div className="flex items-center gap-3">
          <button
            onClick={() => runMatching(batchID)}
            disabled={loading}
            className="px-3.5 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 rounded-lg text-xs font-medium flex items-center gap-1.5 transition"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} /> Re-run Job
          </button>
          {status === 'COMPLETED' && (
            <button
              onClick={() => setActiveTab('results')}
              className="px-4 py-2 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg text-xs font-semibold flex items-center gap-1.5 transition shadow-sm"
            >
              View Pair Results <ArrowRight className="w-4 h-4" />
            </button>
          )}
        </div>
      </div>

      {/* Progress Bar Container */}
      <div className="bg-slate-950/80 p-6 rounded-xl border border-slate-800 space-y-3">
        <div className="flex justify-between items-center text-xs">
          <span className="font-semibold text-slate-300 flex items-center gap-2">
            {status === 'RUNNING' && <span className="w-2 h-2 rounded-full bg-sky-400 animate-ping"></span>}
            Status: <span className={status === 'COMPLETED' ? 'text-emerald-400' : 'text-sky-400'}>{status}</span>
          </span>
          <span className="font-mono text-slate-300 font-bold text-base">{percentage}%</span>
        </div>

        <div className="w-full bg-slate-800 h-3 rounded-full overflow-hidden p-0.5">
          <div
            className={`h-full rounded-full transition-all duration-300 ${
              status === 'COMPLETED' ? 'bg-emerald-500' : 'bg-gradient-to-r from-sky-500 to-emerald-400'
            }`}
            style={{ width: `${percentage}%` }}
          ></div>
        </div>

        <div className="flex justify-between text-[11px] text-slate-400 pt-1 font-mono">
          <span>Processed: {processed_sources} / {total_sources} source records</span>
          <span>Elapsed: {(elapsed_ms / 1000).toFixed(2)}s ({recordsPerSec} rec/sec)</span>
        </div>
      </div>

      {/* Execution Statistics Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 gap-4">
        <div className="bg-slate-950/80 p-4 rounded-xl border border-slate-800 space-y-1">
          <span className="text-xs text-slate-400 uppercase tracking-wider font-semibold">Candidate Pairs</span>
          <div className="text-2xl font-bold text-slate-100 font-mono">{total_candidate_pairs}</div>
          <p className="text-[11px] text-slate-500">Total pair candidates</p>
        </div>

        <div className="bg-slate-950/80 p-4 rounded-xl border border-slate-800 space-y-1">
          <span className="text-xs text-emerald-400 uppercase tracking-wider font-semibold">Auto-Matched</span>
          <div className="text-2xl font-bold text-emerald-400 font-mono">{auto_matched}</div>
          <p className="text-[11px] text-slate-500">Confidence ≥ 90%</p>
        </div>

        <div className="bg-slate-950/80 p-4 rounded-xl border border-slate-800 space-y-1">
          <span className="text-xs text-amber-400 uppercase tracking-wider font-semibold">Review Queue</span>
          <div className="text-2xl font-bold text-amber-400 font-mono">{review_needed}</div>
          <p className="text-[11px] text-slate-500">Confidence 70% - 89%</p>
        </div>

        <div className="bg-slate-950/80 p-4 rounded-xl border border-slate-800 space-y-1">
          <span className="text-xs text-rose-400 uppercase tracking-wider font-semibold">Unmatched</span>
          <div className="text-2xl font-bold text-rose-400 font-mono">{no_match_count || 0}</div>
          <p className="text-[11px] text-slate-500">No match found</p>
        </div>

        <div className="bg-slate-950/80 p-4 rounded-xl border border-slate-800 space-y-1">
          <span className="text-xs text-sky-400 uppercase tracking-wider font-semibold">Total Decisions</span>
          <div className="text-2xl font-bold text-sky-400 font-mono">{total_decisions || 0}</div>
          <p className="text-[11px] text-slate-500">Review & human decisions</p>
        </div>
      </div>
    </div>
  )
}
