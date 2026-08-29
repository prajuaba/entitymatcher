import { create } from 'zustand'

export const useMatcherStore = create((set, get) => ({
  activeTab: 'results',
  batchID: 'benchmark-batch-001',
  loading: false,
  error: null,

  // Engine Configuration
  config: {
    auto_match_threshold: 0.90,
    review_threshold: 0.70,
    date_tolerance_days: 30,
    weights: {
      name_weight: 0.85,
      date_weight: 0.15,
    },
    algorithms: {
      use_jaro_winkler: true,
      use_levenshtein: true,
      use_token_sort: true,
      use_phonetic: true,
      use_trigram: true,
    },
  },

  // Job Progress
  progress: {
    batch_id: '',
    total_sources: 0,
    processed_sources: 0,
    total_matches: 0,
    auto_matched: 0,
    review_needed: 0,
    status: 'IDLE',
    elapsed_ms: 0,
  },

  // Results & Selection
  results: [],
  selectedMatch: null,
  statusFilter: 'ALL',
  searchQuery: '',
  page: 1,
  limit: 20,
  totalCount: 0,

  // Modals
  isManualSearchOpen: false,
  isLLMModalOpen: false,
  llmAnalysisResult: null,

  setActiveTab: (tab) => set({ activeTab: tab }),
  setStatusFilter: (filter) => {
    set({ statusFilter: filter, page: 1 })
    get().fetchResults()
  },
  setSearchQuery: (query) => {
    set({ searchQuery: query, page: 1 })
    get().fetchResults()
  },
  setPage: (page) => {
    set({ page })
    get().fetchResults()
  },
  setSelectedMatch: (match) => set({ selectedMatch: match }),
  setManualSearchOpen: (open) => set({ isManualSearchOpen: open }),
  setLLMModalOpen: (open) => set({ isLLMModalOpen: open }),

  fetchConfig: async () => {
    try {
      const res = await fetch('/api/config')
      if (res.ok) {
        const cfg = await res.json()
        set({ config: cfg })
      }
    } catch (e) {
      console.error('Failed to fetch config', e)
    }
  },

  updateConfig: async (newCfg) => {
    set({ loading: true })
    try {
      const res = await fetch('/api/config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(newCfg),
      })
      if (res.ok) {
        const updated = await res.json()
        set({ config: updated, loading: false })
      }
    } catch (e) {
      set({ error: e.message, loading: false })
    }
  },

  loadSeedDataset: async () => {
    set({ loading: true, error: null })
    try {
      const res = await fetch('/api/seed', { method: 'POST' })
      const data = await res.json()
      if (res.ok) {
        set({ batchID: data.batch_id })
        await get().runMatching(data.batch_id)
      }
    } catch (e) {
      set({ error: e.message, loading: false })
    }
  },

  loadBigSeedDataset: async () => {
    set({ loading: true, error: null })
    try {
      const res = await fetch('/api/seed/big', { method: 'POST' })
      const data = await res.json()
      if (res.ok) {
        set({ batchID: data.batch_id })
        await get().runMatching(data.batch_id)
      }
    } catch (e) {
      set({ error: e.message, loading: false })
    }
  },

  uploadFiles: async (payload) => {
    set({ loading: true, error: null })
    try {
      const res = await fetch('/api/upload', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })
      const data = await res.json()
      if (res.ok) {
        set({ batchID: data.batch_id, loading: false })
        return data.batch_id
      } else {
        throw new Error(data.message || 'Upload failed')
      }
    } catch (e) {
      set({ error: e.message, loading: false })
      throw e
    }
  },

  runMatching: async (batchIdToRun) => {
    const bId = batchIdToRun || get().batchID
    set({ loading: true, activeTab: 'progress' })

    try {
      const res = await fetch(`/api/match/run?batch_id=${bId}`, { method: 'POST' })
      if (!res.ok) throw new Error('Failed to start matching job')

      // Listen to SSE progress updates
      const eventSource = new EventSource(`/api/match/progress?batch_id=${bId}`)
      eventSource.onmessage = (event) => {
        const p = JSON.parse(event.data)
        set({ progress: p })
        if (p.status === 'COMPLETED' || p.status === 'FAILED') {
          eventSource.close()
          set({ loading: false })
          get().fetchResults(bId)
        }
      }
      eventSource.onerror = () => {
        eventSource.close()
        set({ loading: false })
        get().fetchResults(bId)
      }
    } catch (e) {
      set({ error: e.message, loading: false })
    }
  },

  fetchResults: async (batchIdOverride) => {
    const bId = batchIdOverride || get().batchID
    const { statusFilter, searchQuery, page, limit } = get()

    try {
      const queryParams = new URLSearchParams({
        batch_id: bId,
        status: statusFilter,
        search: searchQuery,
        page: page.toString(),
        limit: limit.toString(),
      })

      const res = await fetch(`/api/match/results?${queryParams}`)
      if (res.ok) {
        const data = await res.json()
        set({
          results: data.results || [],
          totalCount: data.total_count || 0,
          selectedMatch: data.results && data.results.length > 0 ? data.results[0] : null,
        })
      }
    } catch (e) {
      console.error('Failed to fetch results', e)
    }
  },

  updateMatchAction: async (matchID, action, userID = 'reviewer_op', reviewComments = '') => {
    const { batchID } = get()
    try {
      const res = await fetch('/api/match/action', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          batch_id: batchID,
          match_id: matchID,
          action,
          user_id: userID,
          review_comments: reviewComments,
        }),
      })
      if (res.ok) {
        // Refresh local list
        await get().fetchResults()
      }
    } catch (e) {
      console.error('Failed to update action', e)
    }
  },

  manualLink: async (sourceID, destinationID) => {
    const { batchID } = get()
    try {
      const res = await fetch('/api/match/manual-link', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ batch_id: batchID, source_id: sourceID, destination_id: destinationID }),
      })
      if (res.ok) {
        const newItem = await res.json()
        set({ isManualSearchOpen: false })
        await get().fetchResults()
        set({ selectedMatch: newItem })
      }
    } catch (e) {
      console.error('Failed to manually link records', e)
    }
  },

  evaluateLLM: async (matchItem) => {
    set({ loading: true })
    try {
      const payload = {
        source_reference_id: matchItem.source?.reference_id || matchItem.source_id,
        source_customer_name: matchItem.source?.customer_name_raw || '',
        source_transaction_date: matchItem.source?.transaction_date ? matchItem.source.transaction_date.slice(0, 10) : '',
        source_transaction_type: matchItem.source?.transaction_type || 'PAYMENT',
        candidates: [
          {
            destination_customer_id: matchItem.destination?.customer_id || matchItem.destination_id,
            customer_name: matchItem.destination?.customer_name_raw || '',
            transaction_date: matchItem.destination?.transaction_date ? matchItem.destination.transaction_date.slice(0, 10) : '',
          },
        ],
      }

      const res = await fetch('/api/llm/evaluate', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })

      if (res.ok) {
        const data = await res.json()
        set({ llmAnalysisResult: data, isLLMModalOpen: true, loading: false })
      } else {
        throw new Error('LLM Evaluation failed')
      }
    } catch (e) {
      set({ error: e.message, loading: false })
    }
  },
}))
