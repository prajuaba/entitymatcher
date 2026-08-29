import React, { useState, useEffect } from 'react'
import { useMatcherStore } from '../store/useMatcherStore'
import { Search, X, Link, Calendar } from 'lucide-react'

export function ManualSearchModal() {
  const { isManualSearchOpen, setManualSearchOpen, selectedMatch, manualLink, batchID } = useMatcherStore()
  const [query, setQuery] = useState('')
  const [candidates, setCandidates] = useState([])
  const [searching, setSearching] = useState(false)

  useEffect(() => {
    if (isManualSearchOpen) {
      handleSearch()
    }
  }, [isManualSearchOpen, query])

  const handleSearch = async () => {
    setSearching(true)
    try {
      const res = await fetch(`/api/destinations/search?batch_id=${batchID}&query=${encodeURIComponent(query)}`)
      if (res.ok) {
        const data = await res.json()
        setCandidates(data || [])
      }
    } catch (e) {
      console.error(e)
    } finally {
      setSearching(false)
    }
  }

  if (!isManualSearchOpen) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm p-4">
      <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-2xl overflow-hidden shadow-2xl flex flex-col max-h-[85vh]">
        {/* Modal Header */}
        <div className="p-4 bg-slate-950 border-b border-slate-800 flex items-center justify-between">
          <div>
            <h3 className="text-base font-bold text-slate-100">Manual Candidate Search & Pairing</h3>
            <p className="text-xs text-slate-400">Pair source record [{selectedMatch?.source?.reference_id}] with custom destination candidate.</p>
          </div>
          <button onClick={() => setManualSearchOpen(false)} className="text-slate-400 hover:text-slate-200">
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Search Input */}
        <div className="p-4 border-b border-slate-800 bg-slate-900">
          <div className="relative">
            <Search className="w-4 h-4 text-slate-500 absolute left-3 top-3" />
            <input
              type="text"
              placeholder="Search candidate name or ID..."
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              className="w-full bg-slate-950 border border-slate-800 rounded-xl pl-9 pr-4 py-2 text-xs text-slate-200 focus:outline-none focus:border-sky-500"
            />
          </div>
        </div>

        {/* Results List */}
        <div className="flex-1 overflow-y-auto p-4 space-y-2">
          {candidates.length === 0 ? (
            <div className="text-center py-8 text-slate-500 text-xs">No candidate records found.</div>
          ) : (
            candidates.map((cand) => (
              <div
                key={cand.id}
                className="p-3 bg-slate-950/80 border border-slate-800 rounded-xl flex items-center justify-between hover:border-slate-700 transition"
              >
                <div>
                  <div className="flex items-center gap-2 mb-1">
                    <span className="text-xs font-mono font-semibold text-purple-400">{cand.customer_id}</span>
                    <span className="text-[11px] text-slate-500 flex items-center gap-1 font-mono">
                      <Calendar className="w-3 h-3" /> {cand.transaction_date ? cand.transaction_date.slice(0, 10) : '-'}
                    </span>
                  </div>
                  <div className="text-xs font-semibold text-slate-200">{cand.customer_name_raw}</div>
                </div>

                <button
                  onClick={() => manualLink(selectedMatch.source_id, cand.id)}
                  className="px-3 py-1.5 bg-sky-600 hover:bg-sky-500 text-white rounded-lg text-xs font-medium flex items-center gap-1.5 transition shrink-0"
                >
                  <Link className="w-3.5 h-3.5" /> Pair Record
                </button>
              </div>
            ))
          )}
        </div>
      </div>
    </div>
  )
}
