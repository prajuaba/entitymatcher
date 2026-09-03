import { create } from 'zustand'
import { apiFetch, getAccessToken, readErrorMessage } from '../lib/api.js'

const BATCH_ID_STORAGE_KEY = 'entity_matcher_batch_id'
let fetchSeq = 0
let searchDebounceHandle = null

function rememberBatchID(id) {
  try {
    if (id) localStorage.setItem(BATCH_ID_STORAGE_KEY, id)
  } catch {
    // ignore
  }
  return id
}

export const useMatcherStore = create((set, get) => ({
  activeTab: 'results',
  // Seeded from localStorage so a reload keeps the batch the user was reviewing.
  // Falls back to '' rather than the demo batch: showing seeded example data as
  // if it were the user's results is worse than showing nothing.
  batchID: (() => {
    try {
      return localStorage.getItem(BATCH_ID_STORAGE_KEY) || ''
    } catch {
      return ''
    }
  })(),
  jobs: [],
  loading: false,
  error: null,

  // Authentication
  token: null,
  user: null,
  authChecked: false,

  // Engine Configuration
  config: {
    auto_match_threshold: 0.90,
    review_threshold: 0.70,
    date_tolerance_days: 30,
    margin_threshold: 0.05,
    assignment_strategy: 'GREEDY_1_1',
    emit_unmatched: true,
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
      use_thai_phonetic: true,
      use_corpus_idf: true,
      use_romanized_match: true,
    },
  },

  // Job Progress
  progress: {
    batch_id: '',
    total_sources: 0,
    processed_sources: 0,
    total_candidate_pairs: 0,
    no_match_count: 0,
    total_decisions: 0,
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
  sortBy: 'created_at',
  sortDir: 'asc',
  totalPages: 1,
  statusCounts: {},
  resultsLoading: false,

  // Modals
  isManualSearchOpen: false,
  isLLMModalOpen: false,
  llmAnalysisResult: null,

  setActiveTab: (tab) => set({ activeTab: tab }),
  setStatusFilter: (filter) => {
    set({ statusFilter: filter, page: 1 })
    get().fetchResults(undefined, { includeCounts: true, resetSelection: true })
  },
  setSearchQuery: (query) => {
    set({ searchQuery: query, page: 1 })
    if (searchDebounceHandle) clearTimeout(searchDebounceHandle)
    searchDebounceHandle = setTimeout(() => {
      searchDebounceHandle = null
      get().fetchResults(undefined, { includeCounts: true, resetSelection: true })
    }, 300)
  },
  flushSearch: () => {
    if (searchDebounceHandle) {
      clearTimeout(searchDebounceHandle)
      searchDebounceHandle = null
    }
    return get().fetchResults(undefined, { includeCounts: true, resetSelection: true })
  },
  setPage: (page) => {
    const { page: currentPage, totalPages } = get()
    const maxPage = Math.max(1, totalPages)
    const clamped = Math.min(Math.max(1, page), maxPage)
    if (clamped === currentPage) return
    set({ page: clamped })
    get().fetchResults(undefined, { resetSelection: true })
  },
  setSort: (field) => {
    const { sortBy, sortDir } = get()
    if (field === sortBy) {
      set({ sortDir: sortDir === 'asc' ? 'desc' : 'asc', page: 1 })
    } else {
      const descDefault = field === 'confidence_score' || field === 'name_score' || field === 'date_score'
      set({ sortBy: field, sortDir: descDefault ? 'desc' : 'asc', page: 1 })
    }
    get().fetchResults(undefined, { resetSelection: true })
  },
  setLimit: (n) => {
    const parsed = parseInt(n, 10)
    const nextLimit = Number.isFinite(parsed) && parsed > 0 ? parsed : get().limit
    set({ limit: nextLimit, page: 1 })
    get().fetchResults(undefined, { resetSelection: true })
  },
  setSelectedMatch: (match) => set({ selectedMatch: match }),
  setManualSearchOpen: (open) => set({ isManualSearchOpen: open }),
  setLLMModalOpen: (open) => set({ isLLMModalOpen: open }),
  setBatchID: (id) => {
    try {
      if (id) localStorage.setItem(BATCH_ID_STORAGE_KEY, id)
      else localStorage.removeItem(BATCH_ID_STORAGE_KEY)
    } catch {
      // A browser that refuses storage still works for this session.
    }
    set({ batchID: id, page: 1, selectedMatch: null, statusCounts: {} })
    return get().fetchResults(id, { includeCounts: true, resetSelection: true })
  },

  // Authentication methods
  initAuth: async () => {
    const token = localStorage.getItem('entity_matcher_token')
    if (token) {
      set({ token })
      try {
        await get().fetchMe()
      } catch (e) {
        console.error('Failed to fetch user on init:', e)
        localStorage.removeItem('entity_matcher_token')
        set({ token: null, user: null })
      }
    }
    set({ authChecked: true })
  },

  login: async (username, password) => {
    set({ loading: true, error: null })
    try {
      const res = await apiFetch('/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password }),
      })
      if (res.ok) {
        const data = await res.json()
        set({ token: data.token, user: data.user, loading: false })
        localStorage.setItem('entity_matcher_token', data.token)
        return { success: true, user: data.user }
      } else {
        const errorText = await res.text()
        set({ error: errorText, loading: false })
        return { success: false, error: errorText }
      }
    } catch (e) {
      set({ error: e.message, loading: false })
      return { success: false, error: e.message }
    }
  },

  logout: () => {
    set({ token: null, user: null })
    localStorage.removeItem('entity_matcher_token')
  },

  fetchMe: async () => {
    try {
      const res = await apiFetch('/api/auth/me')
      if (res.ok) {
        const user = await res.json()
        set({ user })
      }
    } catch (e) {
      console.error('Failed to fetch user:', e)
      throw e
    }
  },

  fetchConfig: async () => {
    try {
      const res = await apiFetch('/api/config')
      if (!res.ok) {
        throw new Error(await readErrorMessage(res, 'Failed to load configuration'))
      }
      set({ config: await res.json(), error: null })
    } catch (e) {
      set({ error: `Could not load saved configuration: ${e.message}` })
    }
  },

  updateConfig: async (newCfg) => {
    set({ loading: true, error: null })
    try {
      const res = await apiFetch('/api/config', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(newCfg),
      })
      // A rejected save used to fall through this branch silently, leaving
      // loading stuck true while the caller reported success.
      if (!res.ok) {
        throw new Error(await readErrorMessage(res, 'Failed to save configuration'))
      }
      const updated = await res.json()
      set({ config: updated, loading: false })
      return updated
    } catch (e) {
      set({ error: e.message, loading: false })
      throw e
    }
  },

  fetchJobs: async () => {
    try {
      const res = await apiFetch('/api/jobs')
      if (!res.ok) {
        throw new Error(await readErrorMessage(res, 'Failed to load job history'))
      }
      const data = await res.json()
      const jobs = data.jobs || []
      set({ jobs })
      // Nothing selected yet (first visit, or storage cleared): fall back to the
      // most recent real run instead of the seeded demo batch.
      if (!get().batchID && jobs.length > 0) {
        get().setBatchID(jobs[0].batch_id)
      }
      return jobs
    } catch (e) {
      set({ error: e.message })
      return []
    }
  },

  fetchConnectorSettings: async () => {
    const res = await apiFetch('/api/connector/settings')
    if (!res.ok) {
      throw new Error(await readErrorMessage(res, 'Failed to load connector settings'))
    }
    return res.json()
  },

  saveConnectorSettings: async (settings) => {
    const res = await apiFetch('/api/connector/settings', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(settings),
    })
    if (!res.ok) {
      throw new Error(await readErrorMessage(res, 'Failed to save connector settings'))
    }
    return res.json()
  },

  loadSeedDataset: async () => {
    set({ loading: true, error: null })
    try {
      const res = await apiFetch('/api/seed', { method: 'POST' })
      const data = await res.json()
      if (res.ok) {
        set({ batchID: rememberBatchID(data.batch_id) })
        await get().runMatching(data.batch_id)
      }
    } catch (e) {
      set({ error: e.message, loading: false })
    }
  },

  loadBigSeedDataset: async () => {
    set({ loading: true, error: null })
    try {
      const res = await apiFetch('/api/seed/big', { method: 'POST' })
      const data = await res.json()
      if (res.ok) {
        set({ batchID: rememberBatchID(data.batch_id) })
        await get().runMatching(data.batch_id)
      }
    } catch (e) {
      set({ error: e.message, loading: false })
    }
  },

  uploadFiles: async (payload) => {
    set({ loading: true, error: null })
    try {
      const res = await apiFetch('/api/upload', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      })
      const data = await res.json()
      if (res.ok) {
        set({ batchID: rememberBatchID(data.batch_id), loading: false })
        return data.batch_id
      } else {
        throw new Error(data.message || 'Upload failed')
      }
    } catch (e) {
      set({ error: e.message, loading: false })
      throw e
    }
  },

  uploadDataFiles: async (formData) => {
    set({ loading: true, error: null })
    try {
      const res = await apiFetch('/api/upload/file', {
        method: 'POST',
        body: formData,
      })
      const raw = await res.text()
      if (!res.ok) {
        let message = raw.trim()
        try {
          const parsed = JSON.parse(raw)
          message = parsed.message || parsed.error || message
        } catch (e) {
          // raw is plain text, use as-is
        }
        if (!message) {
          message = `File upload failed (${res.status})`
        }
        throw new Error(message)
      } else {
        const data = JSON.parse(raw)
        set({ batchID: rememberBatchID(data.batch_id), loading: false })
        return data
      }
    } catch (e) {
      set({ error: e.message, loading: false })
      throw e
    }
  },

  ingestFromConnectors: async ({ source, destination, columnMapping }) => {
    set({ loading: true, error: null })
    try {
      const res = await apiFetch('/api/connector/ingest', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          source,
          destination,
          ...(columnMapping ? { column_mapping: columnMapping } : {})
        }),
      })
      const raw = await res.text()
      if (!res.ok) {
        let message = raw.trim()
        try {
          const parsed = JSON.parse(raw)
          message = parsed.message || parsed.error || message
        } catch (e) {
          // raw is plain text, use as-is
        }
        if (!message) {
          message = `Connector ingestion failed (${res.status})`
        }
        throw new Error(message)
      } else {
        const data = JSON.parse(raw)
        set({ batchID: rememberBatchID(data.batch_id), loading: false })
        return data
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
      const res = await apiFetch(`/api/match/run?batch_id=${bId}`, { method: 'POST' })
      if (!res.ok) throw new Error('Failed to start matching job')

      // Listen to SSE progress updates
      const token = getAccessToken()
      const eventSource = new EventSource(`/api/match/progress?batch_id=${bId}&access_token=${token || ''}`)
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

  fetchResults: async (batchIdOverride, opts = {}) => {
    const bId = batchIdOverride || get().batchID
    if (!bId) {
      set({ results: [], totalCount: 0, totalPages: 1, statusCounts: {}, selectedMatch: null, resultsLoading: false })
      return
    }

    const seq = ++fetchSeq
    set({ resultsLoading: true })

    const { statusFilter, searchQuery, page, limit, sortBy, sortDir } = get()
    const queryParams = new URLSearchParams({
      batch_id: bId,
      status: statusFilter,
      search: searchQuery,
      page: page.toString(),
      limit: limit.toString(),
      sort_by: sortBy,
      sort_dir: sortDir,
    })
    if (opts.includeCounts) {
      queryParams.append('include_counts', '1')
    }

    try {
      const res = await apiFetch(`/api/match/results?${queryParams}`)
      if (seq !== fetchSeq) return
      if (!res.ok) {
        set({ resultsLoading: false })
        return
      }
      const data = await res.json()
      const newResults = data.results || []
      const newTotalCount = data.total_count || 0
      const newTotalPages = data.total_pages && data.total_pages >= 1 ? data.total_pages : 1

      if (newTotalPages >= 1 && page > newTotalPages && newTotalPages !== page) {
        set({ page: newTotalPages, resultsLoading: false })
        return get().fetchResults(bId, opts)
      }

      let newSelectedMatch = null
      if (opts.resetSelection) {
        newSelectedMatch = newResults.length > 0 ? newResults[0] : null
      } else {
        const currentSelected = get().selectedMatch
        if (currentSelected) {
          const found = newResults.find(r => r.id === currentSelected.id)
          newSelectedMatch = found || newResults[0] || null
        } else {
          newSelectedMatch = newResults[0] || null
        }
      }

      const newStatusCounts = opts.includeCounts ? (data.status_counts || {}) : get().statusCounts

      set({
        results: newResults,
        totalCount: newTotalCount,
        totalPages: newTotalPages,
        statusCounts: newStatusCounts,
        selectedMatch: newSelectedMatch,
        resultsLoading: false
      })
    } catch (e) {
      console.error('Failed to fetch results', e)
      set({ resultsLoading: false })
    }
  },

  updateMatchAction: async (matchID, action, userID = 'reviewer_op', reviewComments = '') => {
    const { batchID } = get()
    try {
      const res = await apiFetch('/api/match/action', {
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
        await get().fetchResults(undefined, { includeCounts: true })
      }
    } catch (e) {
      console.error('Failed to update action', e)
    }
  },

  manualLink: async (sourceID, destinationID) => {
    const { batchID } = get()
    try {
      const res = await apiFetch('/api/match/manual-link', {
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

      const res = await apiFetch('/api/llm/evaluate', {
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
