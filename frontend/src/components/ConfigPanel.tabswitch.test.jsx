import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { ConfigPanel } from './ConfigPanel.jsx'
import { useMatcherStore } from '../store/useMatcherStore.js'

let serverConfig

const makeServer = () => {
  serverConfig = {
    auto_match_threshold: 0.90,
    review_threshold: 0.70,
    date_tolerance_days: 30,
    margin_threshold: 0.05,
    assignment_strategy: 'GREEDY_1_1',
    emit_unmatched: true,
    weights: { name_weight: 0.85, date_weight: 0.15 },
    algorithms: {
      use_jaro_winkler: true, use_levenshtein: true, use_token_sort: true,
      use_phonetic: true, use_trigram: true, use_thai_phonetic: true,
      use_corpus_idf: true, use_romanized_match: true,
    },
    column_mapping: {
      name_fields_src: ['customer_name'], name_fields_dest: ['customer_name'],
      ref_id_src: 'reference_id', ref_id_dest: 'customer_id',
      date_field_src: 'transaction_date', date_field_dest: 'transaction_date',
      secondary_fields: [],
    },
  }
  return vi.fn(async (url, options = {}) => {
    if (url === '/api/config' && (!options.method || options.method === 'GET')) {
      return { ok: true, status: 200, json: async () => JSON.parse(JSON.stringify(serverConfig)) }
    }
    if (url === '/api/config' && options.method === 'PUT') {
      serverConfig = { ...serverConfig, ...JSON.parse(options.body) }
      return { ok: true, status: 200, json: async () => JSON.parse(JSON.stringify(serverConfig)) }
    }
    return { ok: true, status: 200, json: async () => ({}), text: async () => '{}' }
  })
}

const defaultConfig = () => ({
  auto_match_threshold: 0.90,
  review_threshold: 0.70,
  date_tolerance_days: 30,
  margin_threshold: 0.05,
  assignment_strategy: 'GREEDY_1_1',
  emit_unmatched: true,
  weights: { name_weight: 0.85, date_weight: 0.15 },
  algorithms: {
    use_jaro_winkler: true, use_levenshtein: true, use_token_sort: true,
    use_phonetic: true, use_trigram: true, use_thai_phonetic: true,
    use_corpus_idf: true, use_romanized_match: true,
  },
  column_mapping: {
    name_fields_src: ['customer_name'], name_fields_dest: ['customer_name'],
    ref_id_src: 'reference_id', ref_id_dest: 'customer_id',
    date_field_src: 'transaction_date', date_field_dest: 'transaction_date',
    secondary_fields: [],
  },
})

beforeEach(() => {
  global.fetch = makeServer()
  useMatcherStore.setState({
    config: defaultConfig(),
    error: null,
    loading: false,
    user: null,
  })
})

describe('ConfigPanel tab-switch persistence', () => {
  it('a saved threshold survives a tab switch', async () => {
    const { unmount } = render(<ConfigPanel />)

    // Wait for the initial GET /api/config to settle and sliders to render
    await screen.findAllByRole('slider')

    // The first slider is the Auto-Match Threshold range input
    const sliders = screen.getAllByRole('slider')
    fireEvent.change(sliders[0], { target: { value: '0.95' } })

    // Click Save Configuration
    const saveBtn = screen.getByRole('button', { name: /Save Configuration/i })
    fireEvent.click(saveBtn)

    // Verify the stateful server received and stored the updated value
    await waitFor(() => {
      expect(serverConfig.auto_match_threshold).toBe(0.95)
    })

    // Simulate navigating to a different tab (full unmount of ConfigPanel)
    unmount()

    // Simulate navigating back (fresh mount)
    render(<ConfigPanel />)

    // Wait for the re-mounted panel to fetch config and render the threshold readout
    const labelEl = await screen.findByText('Auto-Match Threshold')
    const row = labelEl.parentElement

    // Scope assertion to the Auto-Match Threshold row specifically
    await waitFor(() => {
      expect(row.textContent).toContain('95%')
    })
    expect(row.textContent).not.toContain('90%')
  })

  it('the PUT actually carries the edited value', async () => {
    render(<ConfigPanel />)

    await screen.findAllByRole('slider')

    const sliders = screen.getAllByRole('slider')
    fireEvent.change(sliders[0], { target: { value: '0.95' } })

    const saveBtn = screen.getByRole('button', { name: /Save Configuration/i })
    fireEvent.click(saveBtn)

    await waitFor(() => {
      expect(serverConfig.auto_match_threshold).toBe(0.95)
    })

    // Inspect the actual PUT request body sent over the wire
    const putCall = global.fetch.mock.calls.find(
      (call) => call[0] === '/api/config' && call[1] && call[1].method === 'PUT'
    )
    expect(putCall).toBeDefined()
    const body = JSON.parse(putCall[1].body)
    expect(body.auto_match_threshold).toBe(0.95)
  })

  it('a rejected save does not report success', async () => {
    // Override the fetch mock: GET works normally, PUT returns 400 with a specific error
    global.fetch = vi.fn(async (url, options = {}) => {
      if (url === '/api/config' && (!options.method || options.method === 'GET')) {
        return { ok: true, status: 200, json: async () => JSON.parse(JSON.stringify(serverConfig)) }
      }
      if (url === '/api/config' && options.method === 'PUT') {
        return {
          ok: false,
          status: 400,
          text: async () => 'review_threshold must be <= auto_match_threshold',
          json: async () => ({ error: 'review_threshold must be <= auto_match_threshold' }),
        }
      }
      return { ok: true, status: 200, json: async () => ({}), text: async () => '{}' }
    })

    render(<ConfigPanel />)

    await screen.findAllByRole('slider')

    const sliders = screen.getAllByRole('slider')
    fireEvent.change(sliders[0], { target: { value: '0.95' } })

    const saveBtn = screen.getByRole('button', { name: /Save Configuration/i })
    fireEvent.click(saveBtn)

    // Wait for the error message to appear (confirms the save attempt has resolved)
    await screen.findByText('review_threshold must be <= auto_match_threshold')

    // The success banner must NOT be present
    expect(screen.queryByText('Configuration updated successfully!')).not.toBeInTheDocument()
  })
})
