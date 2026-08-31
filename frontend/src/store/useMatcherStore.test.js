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
