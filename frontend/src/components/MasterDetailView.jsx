import React, { useEffect, useState } from 'react'
import { useMatcherStore } from '../store/useMatcherStore'
import { CandidateCard } from './CandidateCard'
import { apiFetch, downloadBlob, getAccessToken } from '../lib/api.js'
import { Search, Filter, ChevronLeft, ChevronRight, CheckCircle, AlertCircle, XCircle, Download, RefreshCcw, X, ChevronsLeft, ChevronsRight, ArrowUp, ArrowDown } from 'lucide-react'

export function MasterDetailView() {
  const {
    results,
    totalCount,
    totalPages,
    statusCounts,
    resultsLoading,
    page,
    limit,
    selectedMatch,
    statusFilter,
    searchQuery,
    sortBy,
    sortDir,
    setStatusFilter,
    setSearchQuery,
    flushSearch,
    setPage,
    setSort,
    setLimit,
    setSelectedMatch,
    fetchResults,
    batchID,
    jobs,
    fetchJobs,
    setBatchID
  } = useMatcherStore()

  useEffect(() => {
    fetchJobs().then(() => fetchResults(undefined, { includeCounts: true, resetSelection: true }))
  }, [])

  const [pageDraft, setPageDraft] = useState(String(page))

  useEffect(() => {
    setPageDraft(String(page))
  }, [page])

  const handleExportCSV = async () => {
    try {
      const res = await apiFetch(`/api/export/csv?batch_id=${batchID}`)
      const blob = await res.blob()
      downloadBlob(blob, `matches-export-${new Date().toISOString()}.csv`)
    } catch (e) {
      console.error('Failed to export CSV:', e)
    }
  }

  const commitPageDraft = () => {
    const num = parseInt(pageDraft, 10)
    if (Number.isFinite(num)) {
      const clamped = Math.min(Math.max(1, num), totalPages)
      setPage(clamped)
      setPageDraft(String(clamped))
    } else {
      setPageDraft(String(page))
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

  const rangeText = totalCount === 0
    ? 'Showing 0 of 0'
    : `Showing ${(page - 1) * limit + 1}–${Math.min(page * limit, totalCount)} of ${totalCount.toLocaleString()}`

  return (
    <div className="flex flex-col h-[calc(100vh-140px)] gap-4">
      {/* Top Filter and Search Bar */}
      <div className="flex flex-wrap items-center justify-between gap-4 bg-slate-900/80 p-3 px-4 rounded-xl border border-slate-800 shrink-0">
        {/* Status Filter Tabs */}
        <div className="flex items-center gap-1.5 overflow-x-auto">
          {filterTabs.map((tab) => {
            let count = null
            if (Object.keys(statusCounts).length > 0) {
              if (tab.key === 'ALL') {
                count = Object.values(statusCounts).reduce((sum, val) => sum + (typeof val === 'number' ? val : 0), 0)
              } else {
                count = statusCounts[tab.key]
              }
            }
            const hasCount = typeof count === 'number'
            return (
              <button
                key={tab.key}
                onClick={() => setStatusFilter(tab.key)}
                className={`px-3 py-1.5 rounded-lg text-xs font-semibold whitespace-nowrap transition ${
                  statusFilter === tab.key
                    ? 'bg-sky-600 text-white shadow-sm'
                    : 'bg-slate-950/60 text-slate-400 hover:text-slate-200 border border-slate-800'
                }`}
                aria-pressed={statusFilter === tab.key}
              >
                {tab.label}
                {hasCount && (
                  <span className={`ml-1.5 px-1.5 py-0.5 rounded-full text-[10px] font-bold ${
                    statusFilter === tab.key ? 'bg-white/20 text-white' : 'bg-slate-800 text-slate-400 border border-slate-700'
                  }`}>
                    {count}
                  </span>
                )}
              </button>
            )
          })}
        </div>

        {/* Search Input & CSV Export */}
        <div className="flex items-center gap-3">
          <span className="text-xs text-slate-400">Batch</span>
          <select
            value={batchID}
            onChange={(e) => setBatchID(e.target.value)}
            className="bg-slate-950 border border-slate-800 rounded-lg p-2 text-xs text-slate-200 max-w-xs"
            title="Which match run to review"
          >
            {!batchID && <option value="">— select a batch —</option>}
            {jobs.map((j) => (
              <option key={j.batch_id} value={j.batch_id}>
                {`${j.batch_id} — ${j.auto_matched} matched, ${j.review_needed} to review`}
              </option>
            ))}
            {batchID && !jobs.some((j) => j.batch_id === batchID) && (
              <option value={batchID}>{batchID}</option>
            )}
          </select>
          
          <div className="relative">
            <Search className="w-4 h-4 text-slate-500 absolute left-3 top-2.5" />
            <input
              type="text"
              placeholder="Search by reference ID, source name, or destination name/ID..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') flushSearch()
                if (e.key === 'Escape') setSearchQuery('')
              }}
              className="bg-slate-950 border border-slate-800 rounded-lg pl-9 pr-8 py-1.5 text-xs text-slate-200 focus:outline-none focus:border-sky-500 w-64"
            />
            {searchQuery && (
              <button
                onClick={() => setSearchQuery('')}
                className="absolute right-3 top-2.5 text-slate-500 hover:text-slate-300"
                aria-label="Clear search"
              >
                <X className="w-4 h-4" />
              </button>
            )}
          </div>

          <select
            value={sortBy}
            onChange={(e) => setSort(e.target.value)}
            className="bg-slate-950 border border-slate-800 rounded-lg p-2 text-xs text-slate-200"
          >
            <option value="created_at">Newest</option>
            <option value="confidence_score">Confidence</option>
            <option value="name_score">Name score</option>
            <option value="date_score">Date score</option>
            <option value="match_status">Status</option>
            <option value="source_name">Source name</option>
            <option value="reference_id">Reference ID</option>
          </select>

          <button
            onClick={() => setSort(sortBy)}
            className="p-1.5 bg-slate-800 hover:bg-slate-700 rounded border border-slate-700 text-slate-200"
            title={sortDir === 'asc' ? 'Ascending' : 'Descending'}
            aria-label={sortDir === 'asc' ? 'Ascending' : 'Descending'}
          >
            {sortDir === 'asc' ? <ArrowUp className="w-3.5 h-3.5" /> : <ArrowDown className="w-3.5 h-3.5" />}
          </button>

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
            <span className="flex items-center gap-1.5">
              {resultsLoading && <RefreshCcw className="w-3.5 h-3.5 animate-spin text-slate-500" />}
              <span>{rangeText}</span>
            </span>
            <span>Confidence</span>
          </div>

          {/* List Items */}
          <div className={`flex-1 overflow-y-auto divide-y divide-slate-800/60 p-2 space-y-1 ${resultsLoading ? 'opacity-50 pointer-events-none' : ''}`}>
            {results.length === 0 ? (
              <div className="text-center p-8 text-slate-500 text-xs">
                {!batchID ? (
                  "Select a match run from the Batch dropdown above to see results."
                ) : (
                  <>
                    No records match the current filter or search.
                    <button
                      onClick={() => {
                        setStatusFilter('ALL')
                        setSearchQuery('')
                      }}
                      className="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 rounded-lg text-xs font-medium ml-2"
                    >
                      Clear filters
                    </button>
                  </>
                )}
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
                    aria-current={isSelected}
                  >
                    <div className="flex items-center justify-between mb-1">
                      <span className="text-[11px] font-mono text-sky-400 font-semibold">
                        {item.source?.reference_id || <span className="text-slate-600 italic">no reference ID</span>}
                      </span>
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
                      <span className="truncate max-w-[200px]">
                        {item.destination ? (
                          `Candidate: ${item.destination.customer_name_raw}`
                        ) : (
                          <span className="text-slate-600 italic">no candidate</span>
                        )}
                      </span>
                      <span className="font-mono">{item.match_status}</span>
                    </div>
                  </button>
                )
              })
            )}
          </div>

          {/* Pagination Controls */}
          <div className="p-3 bg-slate-950/80 border-t border-slate-800 flex items-center justify-between text-xs text-slate-400">
            <div className="flex items-center gap-2">
              <span className="text-slate-400">Rows</span>
              <select
                value={limit}
                onChange={(e) => setLimit(e.target.value)}
                className="bg-slate-950 border border-slate-800 rounded-lg p-1.5 text-xs text-slate-200 w-16"
              >
                <option value="20">20</option>
                <option value="50">50</option>
                <option value="100">100</option>
              </select>
            </div>

            <span className="flex items-center gap-1">
              Page
              <input
                type="text"
                inputMode="numeric"
                value={pageDraft}
                onChange={(e) => setPageDraft(e.target.value)}
                onBlur={commitPageDraft}
                onKeyDown={(e) => { if (e.key === 'Enter') commitPageDraft() }}
                className="w-10 text-center bg-slate-950 border border-slate-800 rounded text-slate-200 text-xs"
              />
              of {totalPages}
            </span>

            <div className="flex items-center gap-2">
              <button
                disabled={page <= 1 || resultsLoading}
                onClick={() => setPage(1)}
                className="p-1.5 bg-slate-800 hover:bg-slate-700 disabled:opacity-40 rounded border border-slate-700 text-slate-200"
                aria-label="First page"
              >
                <ChevronsLeft className="w-4 h-4" />
              </button>
              <button
                disabled={page <= 1 || resultsLoading}
                onClick={() => setPage(page - 1)}
                className="p-1.5 bg-slate-800 hover:bg-slate-700 disabled:opacity-40 rounded border border-slate-700 text-slate-200"
                aria-label="Previous page"
              >
                <ChevronLeft className="w-4 h-4" />
              </button>
              <button
                disabled={page >= totalPages || resultsLoading}
                onClick={() => setPage(page + 1)}
                className="p-1.5 bg-slate-800 hover:bg-slate-700 disabled:opacity-40 rounded border border-slate-700 text-slate-200"
                aria-label="Next page"
              >
                <ChevronRight className="w-4 h-4" />
              </button>
              <button
                disabled={page >= totalPages || resultsLoading}
                onClick={() => setPage(totalPages)}
                className="p-1.5 bg-slate-800 hover:bg-slate-700 disabled:opacity-40 rounded border border-slate-700 text-slate-200"
                aria-label="Last page"
              >
                <ChevronsRight className="w-4 h-4" />
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
