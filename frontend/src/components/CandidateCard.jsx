import React, { useState } from 'react'
import { TokenDiff } from './TokenDiff'
import { CheckCircle2, XCircle, Sparkles, Search, Calendar, FileText, Activity } from 'lucide-react'
import { useMatcherStore } from '../store/useMatcherStore'

export function CandidateCard({ matchItem }) {
  const { updateMatchAction, evaluateLLM, setManualSearchOpen, loading } = useMatcherStore()
  const [reviewerId, setReviewerId] = useState('reviewer_john')
  const [commentText, setCommentText] = useState('')

  if (!matchItem) {
    return (
      <div className="flex flex-col items-center justify-center h-full text-slate-500 p-8">
        <FileText className="w-12 h-12 mb-3 text-slate-600 stroke-[1.5]" />
        <p className="text-base font-medium text-slate-400">No match selected</p>
        <p className="text-xs text-slate-500">Select a source record from the left panel to inspect details.</p>
      </div>
    )
  }

  const { source, destination, confidence_score, name_score, date_score, jw_score, lev_score, token_score, trigram_score, match_status, match_reasons } = matchItem

  const getStatusBadge = (status) => {
    switch (status) {
      case 'AUTO_MATCHED':
        return <span className="px-3 py-1 bg-emerald-500/10 text-emerald-400 border border-emerald-500/30 rounded-full text-xs font-semibold">AUTO MATCHED (≥90%)</span>
      case 'CONFIRMED':
        return <span className="px-3 py-1 bg-blue-500/10 text-blue-400 border border-blue-500/30 rounded-full text-xs font-semibold">MANUALLY CONFIRMED</span>
      case 'REVIEW_NEEDED':
        return <span className="px-3 py-1 bg-amber-500/10 text-amber-400 border border-amber-500/30 rounded-full text-xs font-semibold">REVIEW NEEDED (70-89%)</span>
      case 'REJECTED':
        return <span className="px-3 py-1 bg-rose-500/10 text-rose-400 border border-rose-500/30 rounded-full text-xs font-semibold">REJECTED</span>
      default:
        return <span className="px-3 py-1 bg-slate-800 text-slate-400 border border-slate-700 rounded-full text-xs font-semibold">{status}</span>
    }
  }

  return (
    <div className="flex flex-col h-full bg-slate-900/60 rounded-xl border border-slate-800 overflow-y-auto p-6 space-y-6">
      {/* Header Info */}
      <div className="flex items-center justify-between border-b border-slate-800 pb-4">
        <div>
          <span className="text-xs font-mono text-slate-500 uppercase tracking-widest">Match ID: {matchItem.id}</span>
          <h2 className="text-xl font-bold text-slate-100 mt-0.5">Match Pair Analysis</h2>
        </div>
        <div>{getStatusBadge(match_status)}</div>
      </div>

      {/* Source vs Destination Comparison Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {/* Source Box */}
        <div className="bg-slate-950/80 p-4 rounded-xl border border-slate-800/80 space-y-2">
          <div className="flex items-center justify-between text-xs font-semibold text-sky-400 uppercase tracking-wider">
            <span>Source Record</span>
            <span className="font-mono bg-sky-950 text-sky-300 px-2 py-0.5 rounded border border-sky-800/40">{source?.reference_id}</span>
          </div>
          <div className="text-lg font-semibold text-slate-100 leading-snug">{source?.customer_name_raw}</div>
          <div className="flex items-center gap-4 text-xs text-slate-400 pt-2 border-t border-slate-900">
            <span className="flex items-center gap-1.5"><Calendar className="w-3.5 h-3.5 text-slate-500" /> {source?.transaction_date ? source.transaction_date.slice(0, 10) : '-'}</span>
            {source?.transaction_type && <span className="bg-slate-900 px-2 py-0.5 rounded text-slate-400 font-mono">{source.transaction_type}</span>}
          </div>
        </div>

        {/* Destination Box */}
        <div className="bg-slate-950/80 p-4 rounded-xl border border-slate-800/80 space-y-2">
          <div className="flex items-center justify-between text-xs font-semibold text-purple-400 uppercase tracking-wider">
            <span>Destination Candidate</span>
            <span className="font-mono bg-purple-950 text-purple-300 px-2 py-0.5 rounded border border-purple-800/40">{destination?.customer_id}</span>
          </div>
          <div className="text-lg font-semibold text-slate-100 leading-snug">{destination?.customer_name_raw}</div>
          <div className="flex items-center gap-4 text-xs text-slate-400 pt-2 border-t border-slate-900">
            <span className="flex items-center gap-1.5"><Calendar className="w-3.5 h-3.5 text-slate-500" /> {destination?.transaction_date ? destination.transaction_date.slice(0, 10) : '-'}</span>
          </div>
        </div>
      </div>

      {/* Visual Token Diff */}
      <div className="bg-slate-950/50 p-4 rounded-xl border border-slate-800/80 space-y-3">
        <h3 className="text-xs font-semibold text-slate-300 uppercase tracking-wider">Bilingual Token-Level Visual Diff</h3>
        <TokenDiff sourceName={source?.customer_name_raw} candidateName={destination?.customer_name_raw} />
      </div>

      {/* Score Metrics Breakdown */}
      <div className="bg-slate-950/80 p-5 rounded-xl border border-slate-800 space-y-4">
        <div className="flex items-center justify-between border-b border-slate-800/80 pb-3">
          <span className="text-sm font-semibold text-slate-200 flex items-center gap-2">
            <Activity className="w-4 h-4 text-sky-400" /> Scoring Metrics Breakdown
          </span>
          <div className="flex items-baseline gap-1.5">
            <span className="text-xs text-slate-400">Composite Score:</span>
            <span className={`text-xl font-bold font-mono ${confidence_score >= 0.9 ? 'text-emerald-400' : confidence_score >= 0.7 ? 'text-amber-400' : 'text-rose-400'}`}>
              {(confidence_score * 100).toFixed(1)}%
            </span>
          </div>
        </div>

        {/* Progress Bars for metrics */}
        <div className="grid grid-cols-2 gap-x-6 gap-y-3 text-xs">
          <div>
            <div className="flex justify-between text-slate-400 mb-1">
              <span>Name Similarity Score (85%)</span>
              <span className="font-mono text-slate-200 font-medium">{(name_score * 100).toFixed(1)}%</span>
            </div>
            <div className="w-full bg-slate-800 h-2 rounded-full overflow-hidden">
              <div className="bg-sky-500 h-full rounded-full" style={{ width: `${name_score * 100}%` }}></div>
            </div>
          </div>

          <div>
            <div className="flex justify-between text-slate-400 mb-1">
              <span>Date Match Score (15%)</span>
              <span className="font-mono text-slate-200 font-medium">{(date_score * 100).toFixed(1)}%</span>
            </div>
            <div className="w-full bg-slate-800 h-2 rounded-full overflow-hidden">
              <div className="bg-emerald-500 h-full rounded-full" style={{ width: `${date_score * 100}%` }}></div>
            </div>
          </div>

          <div>
            <div className="flex justify-between text-slate-400 mb-1">
              <span>Jaro-Winkler Metric</span>
              <span className="font-mono text-slate-300">{(jw_score * 100).toFixed(1)}%</span>
            </div>
            <div className="w-full bg-slate-800/80 h-1.5 rounded-full overflow-hidden">
              <div className="bg-purple-500 h-full" style={{ width: `${jw_score * 100}%` }}></div>
            </div>
          </div>

          <div>
            <div className="flex justify-between text-slate-400 mb-1">
              <span>Token-Sorted Score</span>
              <span className="font-mono text-slate-300">{(token_score * 100).toFixed(1)}%</span>
            </div>
            <div className="w-full bg-slate-800/80 h-1.5 rounded-full overflow-hidden">
              <div className="bg-cyan-500 h-full" style={{ width: `${token_score * 100}%` }}></div>
            </div>
          </div>

          <div>
            <div className="flex justify-between text-slate-400 mb-1">
              <span>Levenshtein Similarity</span>
              <span className="font-mono text-slate-300">{(lev_score * 100).toFixed(1)}%</span>
            </div>
            <div className="w-full bg-slate-800/80 h-1.5 rounded-full overflow-hidden">
              <div className="bg-blue-500 h-full" style={{ width: `${lev_score * 100}%` }}></div>
            </div>
          </div>

          <div>
            <div className="flex justify-between text-slate-400 mb-1">
              <span>Trigram Overlap</span>
              <span className="font-mono text-slate-300">{(trigram_score * 100).toFixed(1)}%</span>
            </div>
            <div className="w-full bg-slate-800/80 h-1.5 rounded-full overflow-hidden">
              <div className="bg-indigo-500 h-full" style={{ width: `${trigram_score * 100}%` }}></div>
            </div>
          </div>
        </div>

        {/* Match Reasons List */}
        {match_reasons && match_reasons.length > 0 && (
          <div className="pt-3 border-t border-slate-800/60">
            <span className="text-xs font-semibold text-slate-400 uppercase tracking-wider block mb-2">Automated Match Reasons</span>
            <div className="flex flex-wrap gap-1.5">
              {match_reasons.map((reason, idx) => (
                <span key={idx} className="px-2.5 py-1 bg-slate-900 text-slate-300 rounded-md border border-slate-800 text-xs font-medium flex items-center gap-1.5">
                  <span className="w-1.5 h-1.5 rounded-full bg-sky-400"></span> {reason}
                </span>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Reviewer Compliance Action Bar */}
      <div className="space-y-3 pt-3 border-t border-slate-800">
        <div className="flex flex-wrap items-center gap-2 text-xs">
          <input
            type="text"
            placeholder="Reviewer User ID (e.g. op_john)"
            value={reviewerId}
            onChange={(e) => setReviewerId(e.target.value)}
            className="w-48 bg-slate-950 border border-slate-800 rounded p-2 text-slate-200 font-mono"
          />
          <input
            type="text"
            placeholder="Compliance Rationale / Comments (e.g. Verified Tax ID match with bank registry)"
            value={commentText}
            onChange={(e) => setCommentText(e.target.value)}
            className="flex-1 bg-slate-950 border border-slate-800 rounded p-2 text-slate-200"
          />
        </div>

        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2">
            <button
              onClick={() => {
                updateMatchAction(matchItem.id, 'CONFIRM', reviewerId || 'reviewer_op', commentText)
                setCommentText('')
              }}
              className="px-4 py-2 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg font-medium text-xs flex items-center gap-1.5 transition shadow-sm"
            >
              <CheckCircle2 className="w-4 h-4" /> Approve / Confirm
            </button>
            <button
              onClick={() => {
                updateMatchAction(matchItem.id, 'REJECT', reviewerId || 'reviewer_op', commentText)
                setCommentText('')
              }}
              className="px-4 py-2 bg-rose-600/20 hover:bg-rose-600/30 text-rose-300 border border-rose-600/40 rounded-lg font-medium text-xs flex items-center gap-1.5 transition"
            >
              <XCircle className="w-4 h-4" /> Reject Match
            </button>
          </div>

          <div className="flex items-center gap-2">
            <button
              onClick={() => evaluateLLM(matchItem)}
              disabled={loading}
              className="px-3.5 py-2 bg-purple-900/40 hover:bg-purple-900/60 text-purple-300 border border-purple-700/50 rounded-lg font-medium text-xs flex items-center gap-1.5 transition"
            >
              <Sparkles className="w-4 h-4 text-purple-400" /> LLM Edge-Case Resolver
            </button>
            <button
              onClick={() => setManualSearchOpen(true)}
              className="px-3.5 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 rounded-lg font-medium text-xs flex items-center gap-1.5 transition"
            >
              <Search className="w-4 h-4 text-slate-400" /> Manual Candidate Search
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
