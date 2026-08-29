import React from 'react'
import { useMatcherStore } from '../store/useMatcherStore'
import { Sparkles, X, CheckCircle2, ShieldCheck, Brain } from 'lucide-react'

export function LLMAnalysisModal() {
  const { isLLMModalOpen, setLLMModalOpen, llmAnalysisResult, selectedMatch } = useMatcherStore()

  if (!isLLMModalOpen || !llmAnalysisResult) return null

  const matchDetail = llmAnalysisResult.matches?.[0]

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm p-4">
      <div className="bg-slate-900 border border-purple-950/80 rounded-2xl w-full max-w-2xl overflow-hidden shadow-2xl space-y-4 p-6">
        {/* Modal Header */}
        <div className="flex items-center justify-between border-b border-slate-800 pb-4">
          <div className="flex items-center gap-2.5">
            <div className="p-2 bg-purple-950 text-purple-400 rounded-xl border border-purple-800/40">
              <Sparkles className="w-5 h-5" />
            </div>
            <div>
              <h3 className="text-base font-bold text-slate-100">Bilingual LLM Edge-Case Analysis</h3>
              <p className="text-xs text-slate-400">Structured resolution via Section 2 prompt specification.</p>
            </div>
          </div>
          <button onClick={() => setLLMModalOpen(false)} className="text-slate-400 hover:text-slate-200">
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Source vs Candidate Summary */}
        <div className="grid grid-cols-2 gap-3 text-xs bg-slate-950 p-4 rounded-xl border border-slate-800">
          <div>
            <span className="text-slate-500 uppercase tracking-wider block font-mono text-[10px]">Source Reference ID</span>
            <span className="font-semibold text-sky-400">{llmAnalysisResult.source_reference_id}</span>
            <div className="text-slate-200 mt-1">{selectedMatch?.source?.customer_name_raw}</div>
          </div>
          <div>
            <span className="text-slate-500 uppercase tracking-wider block font-mono text-[10px]">Destination Candidate ID</span>
            <span className="font-semibold text-purple-400">{matchDetail?.destination_customer_id}</span>
            <div className="text-slate-200 mt-1">{selectedMatch?.destination?.customer_name_raw}</div>
          </div>
        </div>

        {/* LLM Structured Metrics */}
        {matchDetail && (
          <div className="space-y-4">
            <div className="grid grid-cols-3 gap-3">
              <div className="bg-slate-950 p-3 rounded-xl border border-slate-800 text-center">
                <span className="text-[11px] text-slate-400 uppercase tracking-wider block">LLM Confidence</span>
                <span className="text-xl font-bold font-mono text-emerald-400">
                  {(matchDetail.confidence_score * 100).toFixed(1)}%
                </span>
              </div>

              <div className="bg-slate-950 p-3 rounded-xl border border-slate-800 text-center">
                <span className="text-[11px] text-slate-400 uppercase tracking-wider block">Date Proximity</span>
                <span className={`text-sm font-bold font-mono ${matchDetail.date_match_status === 'EXACT' ? 'text-emerald-400' : 'text-amber-400'}`}>
                  {matchDetail.date_match_status}
                </span>
              </div>

              <div className="bg-slate-950 p-3 rounded-xl border border-slate-800 text-center">
                <span className="text-[11px] text-slate-400 uppercase tracking-wider block">Name Entity Type</span>
                <span className="text-sm font-bold font-mono text-purple-400">
                  {matchDetail.matched_name_type}
                </span>
              </div>
            </div>

            {/* LLM Match Reasons */}
            <div className="bg-slate-950 p-4 rounded-xl border border-slate-800 space-y-2">
              <span className="text-xs font-semibold text-slate-300 uppercase tracking-wider flex items-center gap-1.5">
                <Brain className="w-4 h-4 text-purple-400" /> LLM Reasoning & Normalization
              </span>
              <ul className="space-y-1.5 text-xs text-slate-300">
                {matchDetail.match_reasons?.map((reason, idx) => (
                  <li key={idx} className="flex items-start gap-2">
                    <CheckCircle2 className="w-3.5 h-3.5 text-purple-400 shrink-0 mt-0.5" />
                    <span>{reason}</span>
                  </li>
                ))}
              </ul>
            </div>
          </div>
        )}

        <div className="flex justify-end pt-2">
          <button
            onClick={() => setLLMModalOpen(false)}
            className="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 rounded-lg text-xs font-semibold transition"
          >
            Close Analysis
          </button>
        </div>
      </div>
    </div>
  )
}
