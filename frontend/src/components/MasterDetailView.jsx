import React, { useEffect } from 'react'
import { useMatcherStore } from '../store/useMatcherStore'
import { CandidateCard } from './CandidateCard'
import { apiFetch, downloadBlob, getAccessToken } from '../lib/api.js'
import { Search, Filter, ChevronLeft, ChevronRight, CheckCircle, AlertCircle, XCircle, Download, RefreshCcw } from 'lucide-react'

export function MasterDetailView() {
  const {
    results,
    totalCount,
    page,
    limit,
    selectedMatch,
    statusFilter,
    searchQuery,
    setStatusFilter,
    setSearchQuery,
    setPage,
    setSelectedMatch,
    fetchResults,
    batchID,
  } = useMatcherStore()

  useEffect(() => {
    fetchResults()
  }, [])

  const handleExportCSV = async () => {
    try {
      const res = await apiFetch(`/api/export/csv?batch_id=${batchID}`)
      const blob = await res.blob()
      downloadBlob(blob, `matches-export-${new Date().toISOString()}.csv`)
    } catch (e) {
      console.error('Failed to export CSV:', e)
    }
  }

  const filterTabs = [
    { key: 'ALL', label: 'All Pairs' },
    { key: 'AUTO_MATCHED', label: 'Auto Matched (≥90%)' },
    { key: 'REVIEW_NEEDED', label: 'Review Queue (70-89%)' },
    { key: 'CONFIRMED', label: 'Confirmed' },
    { key: 'REJECTED', label: 'Rejected' },
    { key: 'NO_MATCH', label: 'Unmatched', icon: 'unmatched' },
  ]

  const totalPages = Math.ceil(totalCount / limit) || 1

  return (
    <div className="flex flex-col h-[calc(100vh-140px)] gap-4">
      {/* Top Filter and Search Bar */}
      <div className="flex flex-wrap items-center justify-between gap-4 bg-slate-900/80 p-3 px-4 rounded-xl border border-slate-800 shrink-0">
        {/* Status Filter Tabs */}
        <div className="flex items-center gap-1.5 overflow-x-auto">
          {filterTabs.map((tab) => (
            <button
              key={tab.key}
              onClick={() => setStatusFilter(tab.key)}
              className={`px-3 py-1.5 rounded-lg text-xs font-semibold whitespace-nowrap transition ${
                statusFilter === tab.key
                  ? 'bg-sky-600 text-white shadow-sm'
                  : 'bg-slate-950/60 text-slate-400 hover:text-slate-200 border border-slate-800'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>

        {/* Search Input & CSV Export */}
        <div className="flex items-center gap-3">
          <div className="relative">
            <Search className="w-4 h-4 text-slate-500 absolute left-3 top-2.5" />
            <input
              type="text"
              placeholder="Search reference ID or name..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="bg-slate-950 border border-slate-800 rounded-lg pl-9 pr-3 py-1.5 text-xs text-slate-200 focus:outline-none focus:border-sky-500 w-64"
            />
          </div>

          <button
            onClick={handleExportCSV}
            className="px-3.5 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 rounded-lg text-xs font-medium flex items-center gap-1.5 transition"
          >
            <Download className="w-3.5 h-3.5 text-slate-400" /> Export CSV
          </button>
        </div>
      </div>

      {/* Split Master-Detail Layout */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-4 flex-1 min-h-0">
        {/* Left Panel: Virtualized Master Records List */}
        <div className="lg:col-span-5 flex flex-col bg-slate-900/60 rounded-xl border border-slate-800 min-h-0 overflow-hidden">
          <div className="p-3 bg-slate-950/60 border-b border-slate-800 flex items-center justify-between text-xs text-slate-400 font-medium">
            <span>Source Matches ({totalCount} items)</span>
            <span>Confidence</span>
          </div>

          {/* List Items */}
          <div className="flex-1 overflow-y-auto divide-y divide-slate-800/60 p-2 space-y-1">
            {results.length === 0 ? (
              <div className="text-center p-8 text-slate-500 text-xs">
                No matching records found for filter criteria.
              </div>
            ) : (
              results.map((item) => {
                const isSelected = selectedMatch?.id === item.id
                return (
                  <button
                    key={item.id}
                    onClick={() => setSelectedMatch(item)}
                    className={`w-full text-left p-3 rounded-lg transition border ${
                      isSelected
                        ? 'bg-sky-950/40 border-sky-600/60 text-slate-100 shadow-md'
                        : 'bg-slate-950/40 border-slate-900/80 hover:bg-slate-900/80 text-slate-300'
                    }`}
                  >
                    <div className="flex items-center justify-between mb-1">
                      <span className="text-[11px] font-mono text-sky-400 font-semibold">{item.source?.reference_id}</span>
                      <span
                        className={`text-xs font-mono font-bold px-2 py-0.5 rounded ${
                          item.confidence_score >= 0.9
                            ? 'bg-emerald-950 text-emerald-300 border border-emerald-800/60'
                            : item.confidence_score >= 0.7
                            ? 'bg-amber-950 text-amber-300 border border-amber-800/60'
                            : 'bg-rose-950 text-rose-300 border border-rose-800/60'
                        }`}
                      >
                        {(item.confidence_score * 100).toFixed(0)}%
                      </span>
                    </div>

                    <div className="text-xs font-semibold truncate leading-snug">{item.source?.customer_name_raw}</div>

                    <div className="flex items-center justify-between text-[11px] text-slate-500 mt-2">
                      <span className="truncate max-w-[200px]">Candidate: {item.destination?.customer_name_raw}</span>
                      <span className="font-mono">{item.match_status}</span>
                    </div>
                  </button>
                )
              })
            )}
          </div>

          {/* Pagination Controls */}
          <div className="p-3 bg-slate-950/80 border-t border-slate-800 flex items-center justify-between text-xs text-slate-400">
            <span>Page {page} of {totalPages}</span>
            <div className="flex items-center gap-2">
              <button
                disabled={page <= 1}
                onClick={() => setPage(page - 1)}
                className="p-1.5 bg-slate-800 hover:bg-slate-700 disabled:opacity-40 rounded border border-slate-700 text-slate-200"
              >
                <ChevronLeft className="w-4 h-4" />
              </button>
              <button
                disabled={page >= totalPages}
                onClick={() => setPage(page + 1)}
                className="p-1.5 bg-slate-800 hover:bg-slate-700 disabled:opacity-40 rounded border border-slate-700 text-slate-200"
              >
                <ChevronRight className="w-4 h-4" />
              </button>
            </div>
          </div>
        </div>

        {/* Right Panel: Active Candidate Card */}
        <div className="lg:col-span-7 h-full min-h-0">
          <CandidateCard matchItem={selectedMatch} />
        </div>
      </div>
    </div>
  )
}
