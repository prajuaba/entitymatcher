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

describe('ConfigPanel scoring warnings', () => {
  it('warns when date weight is set but the source date column is blank, naming the missing side', async () => {
    const cfg = defaultConfig()
    cfg.column_mapping.date_field_src = ''
    useMatcherStore.setState({ config: cfg })

    render(<ConfigPanel />)
    await screen.findAllByRole('slider')

    const warning = screen.getByText(/Date weight is 15%/i)
    expect(warning).toBeInTheDocument()
    // Assert it's about source side, not destination or both
    expect(warning.textContent).toMatch(/no date column is mapped on the source side/i)
    expect(warning.textContent).not.toMatch(/source and destination sides/i)
  })

  it('no warning when both date columns are mapped', async () => {
    render(<ConfigPanel />)
    await screen.findAllByRole('slider')

    expect(screen.queryByText(/Date weight is/i)).not.toBeInTheDocument()
  })

  it('no warning when date_weight is 0 even though both date columns are blank', async () => {
    const cfg = defaultConfig()
    cfg.weights = { name_weight: 1.0, date_weight: 0 }
    cfg.column_mapping.date_field_src = ''
    cfg.column_mapping.date_field_dest = ''
    useMatcherStore.setState({ config: cfg })

    render(<ConfigPanel />)
    await screen.findAllByRole('slider')

    expect(screen.queryByText(/Date weight is/i)).not.toBeInTheDocument()
    expect(screen.queryByText(/date column/i)).not.toBeInTheDocument()
  })

  it('Save button stays enabled while the warning is showing, and clicking it still saves', async () => {
    const cfg = defaultConfig()
    cfg.column_mapping.date_field_src = ''
    useMatcherStore.setState({ config: cfg })

    render(<ConfigPanel />)
    await screen.findAllByRole('slider')

    // Sanity check warning is present
    expect(screen.getByText(/Date weight is 15%/i)).toBeInTheDocument()

    const saveBtn = screen.getByRole('button', { name: /Save Configuration/i })
    expect(saveBtn).not.toBeDisabled()

    fireEvent.click(saveBtn)

    await waitFor(() => {
      const putCall = global.fetch.mock.calls.find(
        (call) => call[0] === '/api/config' && call[1] && call[1].method === 'PUT'
      )
      expect(putCall).toBeDefined()
    })
  })
})
