import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ConnectionManager } from './ConnectionManager.jsx'
import { useMatcherStore } from '../store/useMatcherStore.js'

beforeEach(() => {
  useMatcherStore.setState({ batchID: 'benchmark-batch-001', error: null, loading: false })
})

describe('ConnectionManager', () => {
  it('refuses file types without calling the API', async () => {
    // Failing fast here is better than round-tripping to a backend that will reject the file type anyway.
    global.fetch = vi.fn()

    render(<ConnectionManager />)

    const selects = screen.getAllByRole('combobox')
    await userEvent.selectOptions(selects[0], 'CSV')

    const loadBtn = screen.getByRole('button', { name: /Load Data & Start Batch/i })
    await userEvent.click(loadBtn)

    const statusEl = await screen.findByTestId('ingest-status')
    expect(global.fetch).not.toHaveBeenCalled()
    expect(statusEl.getAttribute('data-outcome')).toBe('error')
    expect(statusEl.textContent).toMatch(/Data Ingestion/i)
  })

  it('reports the row counts it actually loaded', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      text: async () => JSON.stringify({ batch_id: 'batch-7', source_count: 250, destination_count: 120, truncated: false }),
    })

    render(<ConnectionManager />)

    const loadBtn = screen.getByRole('button', { name: /Load Data & Start Batch/i })
    await userEvent.click(loadBtn)

    const statusEl = await screen.findByTestId('ingest-status')
    expect(statusEl.getAttribute('data-outcome')).toBe('success')
    expect(statusEl.textContent).toContain('250')
    expect(statusEl.textContent).toContain('120')
    expect(statusEl.textContent).toContain('batch-7')
  })

  it('a truncated ingest is not reported as success', async () => {
    // A partial/truncated batch that renders as a clean success banner is one an operator will trust
    // and match against, so this must render as an error outcome even on HTTP 200.
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      text: async () => JSON.stringify({
        batch_id: 'batch-8',
        source_count: 50000,
        destination_count: 10,
        truncated: true,
        source_truncated: true,
        warning: 'ingestion stopped at the 50000 row cap and more rows remain; the batch is incomplete',
      }),
    })

    render(<ConnectionManager />)

    const loadBtn = screen.getByRole('button', { name: /Load Data & Start Batch/i })
    await userEvent.click(loadBtn)

    const statusEl = await screen.findByTestId('ingest-status')
    // Explicitly NOT 'success' — the HTTP call succeeded with 200, but the data is incomplete.
    expect(statusEl.getAttribute('data-outcome')).toBe('error')
    expect(statusEl.textContent).toContain('more rows remain')
  })

  it('sends the port as a number, not a string', async () => {
    // The Go backend decodes `port` into an int, so a quoted string fails JSON decoding
    // with a 400 that looks like a connection error.
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      text: async () => JSON.stringify({ batch_id: 'batch-9', source_count: 10, destination_count: 5, truncated: false }),
    })

    render(<ConnectionManager />)

    const selects = screen.getAllByRole('combobox')
    await userEvent.selectOptions(selects[0], 'POSTGRES')

    // Index 0 is the source port input because it is rendered first in the DOM (source card precedes destination card).
    const portInputs = screen.getAllByPlaceholderText('Port')
    await userEvent.clear(portInputs[0])
    await userEvent.type(portInputs[0], '6543')

    const loadBtn = screen.getByRole('button', { name: /Load Data & Start Batch/i })
    await userEvent.click(loadBtn)

    await screen.findByTestId('ingest-status')

    const [, options] = global.fetch.mock.calls[0]
    const body = JSON.parse(options.body)
    expect(typeof body.source.port).toBe('number')
    expect(body.source.port).toBe(6543)
  })

  it('the port follows the selected type', async () => {
    // Stub for safety in case it is accidentally triggered
    global.fetch = vi.fn()

    render(<ConnectionManager />)

    const selects = screen.getAllByRole('combobox')

    await userEvent.selectOptions(selects[0], 'POSTGRES')
    // Input value is a string in the DOM even though the underlying state is a number
    expect(screen.getAllByPlaceholderText('Port')[0].value).toBe('5432')

    await userEvent.selectOptions(selects[0], 'MONGODB')
    expect(screen.getAllByPlaceholderText('Port')[0].value).toBe('27017')
  })
})
