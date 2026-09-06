import { useMatcherStore } from './useMatcherStore.js'

const fakeResponse = (status, body) => ({
  ok: status >= 200 && status < 300,
  status,
  text: async () => body,
})

describe('ingestFromConnectors', () => {
  let mockFetch

  beforeEach(() => {
    mockFetch = vi.fn()
    global.fetch = mockFetch
    useMatcherStore.setState({ batchID: 'benchmark-batch-001', error: null, loading: false })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('sets batchID and returns the payload on success', async () => {
    mockFetch.mockResolvedValue(
      fakeResponse(200, JSON.stringify({ status: 'success', batch_id: 'batch-9', source_count: 250, destination_count: 250, truncated: false }))
    )

    const result = await useMatcherStore.getState().ingestFromConnectors({
      source: { type: 'csv', name: 'src' },
      destination: { type: 'csv', name: 'dst' },
    })

    expect(result.batch_id).toBe('batch-9')
    expect(useMatcherStore.getState().batchID).toBe('batch-9')
    expect(useMatcherStore.getState().loading).toBe(false)
  })

  it('posts to the ingest endpoint with source and destination', async () => {
    mockFetch.mockResolvedValue(
      fakeResponse(200, JSON.stringify({ status: 'success', batch_id: 'batch-10' }))
    )

    const source = { type: 'csv', name: 'source-file' }
    const destination = { type: 'postgres', name: 'dest-db' }

    await useMatcherStore.getState().ingestFromConnectors({ source, destination })

    expect(mockFetch).toHaveBeenCalledTimes(1)
    const [url, options] = mockFetch.mock.calls[0]
    expect(url).toBe('/api/connector/ingest')
    expect(options.method).toBe('POST')
    expect(options.headers['Content-Type']).toBe('application/json')

    const body = JSON.parse(options.body)
    expect(body.source).toEqual(source)
    expect(body.destination).toEqual(destination)
  })

  it('omits column_mapping when none is given', async () => {
    mockFetch.mockResolvedValue(
      fakeResponse(200, JSON.stringify({ status: 'success', batch_id: 'batch-11' }))
    )

    await useMatcherStore.getState().ingestFromConnectors({
      source: { type: 'csv', name: 's' },
      destination: { type: 'csv', name: 'd' },
    })

    const [ , options] = mockFetch.mock.calls[0]
    const body = JSON.parse(options.body)
    // The backend treats an absent mapping as "keep the configured one", so sending an explicit null/undefined would be a different instruction.
    expect(body).not.toHaveProperty('column_mapping')
  })

  it('includes column_mapping when given', async () => {
    mockFetch.mockResolvedValue(
      fakeResponse(200, JSON.stringify({ status: 'success', batch_id: 'batch-12' }))
    )

    await useMatcherStore.getState().ingestFromConnectors({
      source: { type: 'csv', name: 's' },
      destination: { type: 'csv', name: 'd' },
      columnMapping: { name_fields_src: ['a'] },
    })

    const [ , options] = mockFetch.mock.calls[0]
    const body = JSON.parse(options.body)
    expect(body.column_mapping).toEqual({ name_fields_src: ['a'] })
  })

  it('surfaces a plain-text error body', async () => {
    const errMsg = 'connector type CSV cannot be ingested here; upload the file to /api/upload/file instead'
    mockFetch.mockResolvedValue(fakeResponse(400, errMsg))

    await expect(
      useMatcherStore.getState().ingestFromConnectors({
        source: { type: 'csv', name: 's' },
        destination: { type: 'csv', name: 'd' },
      })
    ).rejects.toThrow(errMsg)

    expect(useMatcherStore.getState().error).toBe(errMsg)
    expect(useMatcherStore.getState().batchID).toBe('benchmark-batch-001')
  })

  it('surfaces a JSON error message', async () => {
    mockFetch.mockResolvedValue(fakeResponse(400, JSON.stringify({ message: 'boom' })))

    await expect(
      useMatcherStore.getState().ingestFromConnectors({
        source: { type: 'csv', name: 's' },
        destination: { type: 'csv', name: 'd' },
      })
    ).rejects.toThrow('boom')
  })

  it('falls back to a status message on an empty error body', async () => {
    mockFetch.mockResolvedValue(fakeResponse(500, ''))

    await expect(
      useMatcherStore.getState().ingestFromConnectors({
        source: { type: 'csv', name: 's' },
        destination: { type: 'csv', name: 'd' },
      })
    ).rejects.toThrow(/500/)
  })
})

describe('loadProgress', () => {
  let mockFetch

  beforeEach(() => {
    mockFetch = vi.fn()
    global.fetch = mockFetch
    useMatcherStore.setState({ batchID: 'benchmark-batch-001', error: null, loading: false })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('populates progress from the endpoint\'s payload', async () => {
    mockFetch.mockImplementation(async (url) => {
      if (url.startsWith('/api/match/status')) {
        return {
          ok: true,
          status: 200,
          json: async () => ({
            batch_id: 'batch-1',
            total_sources: 100,
            processed_sources: 100,
            total_candidate_pairs: 50,
            auto_matched: 40,
            review_needed: 5,
            no_match_count: 5,
            total_decisions: 45,
            status: 'COMPLETED',
            started_at: '2026-01-01T00:00:00Z',
            completed_at: '2026-01-01T00:01:00Z',
            elapsed_ms: 60000
          })
        }
      }
      if (url.startsWith('/api/match/results')) {
        return {
          ok: true,
          status: 200,
          json: async () => ({ results: [], total_count: 0 })
        }
      }
      throw new Error(`Unexpected fetch call to ${url}`)
    })

    await useMatcherStore.getState().loadProgress('batch-1')

    expect(useMatcherStore.getState().progress.status).toBe('COMPLETED')
    expect(useMatcherStore.getState().progress.auto_matched).toBe(40)
  })

  it('resets progress to IDLE/zeros on a 404', async () => {
    useMatcherStore.setState({
      progress: {
        batch_id: 'old-batch',
        total_sources: 500,
        processed_sources: 500,
        total_candidate_pairs: 300,
        auto_matched: 250,
        review_needed: 30,
        no_match_count: 20,
        total_decisions: 280,
        status: 'COMPLETED',
        elapsed_ms: 12345
      }
    })

    mockFetch.mockImplementation(async (url) => {
      if (url.startsWith('/api/match/status')) {
        return {
          ok: false,
          status: 404,
          text: async () => 'not found',
          json: async () => { throw new Error('should not be called') }
        }
      }
      if (url.startsWith('/api/match/results')) {
        return {
          ok: true,
          status: 200,
          json: async () => ({ results: [], total_count: 0 })
        }
      }
      throw new Error(`Unexpected fetch call to ${url}`)
    })

    await useMatcherStore.getState().loadProgress('new-batch')

    const progress = useMatcherStore.getState().progress
    expect(progress.status).toBe('IDLE')
    expect(progress.auto_matched).toBe(0)
    expect(progress.review_needed).toBe(0)
    expect(progress.total_sources).toBe(0)
    expect(progress.batch_id).toBe('new-batch')
  })
})

describe('setBatchID triggers status fetch', () => {
  let mockFetch

  beforeEach(() => {
    mockFetch = vi.fn()
    global.fetch = mockFetch
    useMatcherStore.setState({ batchID: 'benchmark-batch-001', error: null, loading: false })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('triggers the status fetch with the right batch_id', async () => {
    mockFetch.mockImplementation(async (url) => {
      if (url.startsWith('/api/match/results')) {
        return {
          ok: true,
          status: 200,
          json: async () => ({ results: [], total_count: 0 })
        }
      }
      if (url.startsWith('/api/match/status')) {
        return {
          ok: true,
          status: 200,
          json: async () => ({
            batch_id: 'batch-42',
            status: 'RUNNING',
            total_sources: 10,
            processed_sources: 5,
            total_candidate_pairs: 0,
            auto_matched: 0,
            review_needed: 0,
            no_match_count: 0,
            total_decisions: 0,
            elapsed_ms: 100
          })
        }
      }
      throw new Error(`Unexpected fetch call to ${url}`)
    })

    await useMatcherStore.getState().setBatchID('batch-42')

    const statusCalls = mockFetch.mock.calls.filter(([url]) => url.startsWith('/api/match/status'))
    expect(statusCalls.length).toBe(1)
    const [statusUrl] = statusCalls[0]
    expect(statusUrl).toContain('batch_id=batch-42')
    expect(useMatcherStore.getState().progress.status).toBe('RUNNING')
  })
})

describe('manualLink', () => {
  let mockFetch

  beforeEach(() => {
    mockFetch = vi.fn()
    global.fetch = mockFetch
    useMatcherStore.setState({ batchID: 'batch-ml', isManualSearchOpen: true })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('requests results with include_counts set', async () => {
    mockFetch.mockImplementation(async (url) => {
      if (url === '/api/match/manual-link') {
        return {
          ok: true,
          status: 200,
          json: async () => ({ id: 'match-99', match_status: 'CONFIRMED' })
        }
      }
      if (url.startsWith('/api/match/results')) {
        return {
          ok: true,
          status: 200,
          json: async () => ({ results: [], total_count: 0, status_counts: { ALL: 1, CONFIRMED: 1 } })
        }
      }
      throw new Error('Unexpected fetch call to ' + url)
    })

    await useMatcherStore.getState().manualLink('src-1', 'dst-1')

    const resultsCalls = mockFetch.mock.calls.filter(([url]) => url.startsWith('/api/match/results'))
    expect(resultsCalls.length).toBe(1)
    const [resultsUrl] = resultsCalls[0]
    expect(resultsUrl).toContain('include_counts=1')
    expect(useMatcherStore.getState().selectedMatch).toEqual({ id: 'match-99', match_status: 'CONFIRMED' })
  })
})

describe('updateMatchAction', () => {
  let mockFetch

  beforeEach(() => {
    mockFetch = vi.fn()
    global.fetch = mockFetch
    useMatcherStore.setState({ batchID: 'batch-ua' })
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('does NOT include user_id in its POST body', async () => {
    mockFetch.mockImplementation(async (url) => {
      if (url === '/api/match/action') {
        return {
          ok: true,
          status: 200,
          json: async () => ({})
        }
      }
      if (url.startsWith('/api/match/results')) {
        return {
          ok: true,
          status: 200,
          json: async () => ({ results: [], total_count: 0 })
        }
      }
      throw new Error('Unexpected fetch call to ' + url)
    })

    await useMatcherStore.getState().updateMatchAction('match-1', 'CONFIRM', 'looks good')

    const actionCalls = mockFetch.mock.calls.filter(([url]) => url === '/api/match/action')
    expect(actionCalls.length).toBe(1)
    const [ , options] = actionCalls[0]
    const body = JSON.parse(options.body)
    expect(body).not.toHaveProperty('user_id')
    expect(body.review_comments).toBe('looks good')
    expect(body.action).toBe('CONFIRM')
  })
})

describe('rank1Only filter (backlog S2)', () => {
  afterEach(() => { vi.restoreAllMocks() })

  it('omits rank1_only by default and sends it once enabled', async () => {
    const urls = []
    global.fetch = vi.fn(async (url) => {
      urls.push(url)
      return {
        ok: true, status: 200,
        json: async () => ({ results: [], total_count: 0, total_pages: 1, status_counts: {} }),
      }
    })

    useMatcherStore.setState({ batchID: 'b1', rank1Only: false })
    await useMatcherStore.getState().fetchResults()
    expect(urls.at(-1)).not.toContain('rank1_only')

    // Enabling must resend with the flag, so the server -- which owns paging and
    // the status counts -- applies the same filter the UI is showing.
    await useMatcherStore.getState().setRank1Only(true)
    expect(urls.at(-1)).toContain('rank1_only=1')
    expect(useMatcherStore.getState().rank1Only).toBe(true)
  })
})
