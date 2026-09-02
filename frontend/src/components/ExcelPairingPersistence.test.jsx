import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import { ConfigPanel } from './ConfigPanel.jsx'
import { useMatcherStore } from '../store/useMatcherStore.js'

let serverConfig
let serverSettings

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

// Empty-string type means "nothing was ever saved" -- ConnectionManager's
// mount-time apply() skips a side whose type is falsy, so the panel keeps its
// hardcoded demo defaults until something real is introspected/saved.
const defaultConnectorSettings = () => ({
  source: { type: '', host: '', port: 0, database: '', username: '', table_or_query: '', file_path: '', columns: [] },
  destination: { type: '', host: '', port: 0, database: '', username: '', table_or_query: '', file_path: '', columns: [] },
})

const SRC_COLUMNS = [
  { name: 'CustID', data_type: 'STRING' },
  { name: 'CustomerName', data_type: 'STRING' },
  { name: 'TaxRegistrationNo', data_type: 'STRING' },
]
const DEST_COLUMNS = [
  { name: 'client_ref', data_type: 'STRING' },
  { name: 'client_name', data_type: 'STRING' },
  { name: 'reg_no', data_type: 'STRING' },
]

// Mirrors the Go struct: password is never part of the persisted connector
// settings record, so strip it defensively even though the current client
// (toEndpoint in ConnectionManager.jsx) never includes it in the first place.
const stripPassword = (side) => {
  if (!side || typeof side !== 'object') return side
  const { password, ...rest } = side
  return rest
}

const makeServer = () => {
  serverConfig = defaultConfig()
  serverSettings = defaultConnectorSettings()

  return vi.fn(async (url, options = {}) => {
    const method = options.method || 'GET'

    if (url === '/api/config' && method === 'GET') {
      return { ok: true, status: 200, json: async () => JSON.parse(JSON.stringify(serverConfig)) }
    }
    if (url === '/api/config' && method === 'PUT') {
      const body = JSON.parse(options.body)
      serverConfig = { ...serverConfig, ...body }
      return { ok: true, status: 200, json: async () => JSON.parse(JSON.stringify(serverConfig)) }
    }

    if (url === '/api/connector/settings' && method === 'GET') {
      return { ok: true, status: 200, json: async () => JSON.parse(JSON.stringify(serverSettings)) }
    }
    if (url === '/api/connector/settings' && method === 'PUT') {
      const body = JSON.parse(options.body)
      serverSettings = {
        source: stripPassword(body.source),
        destination: stripPassword(body.destination),
      }
      return { ok: true, status: 200, json: async () => JSON.parse(JSON.stringify(serverSettings)) }
    }

    if (url === '/api/connector/introspect/upload' && method === 'POST') {
      const file = options.body && typeof options.body.get === 'function' ? options.body.get('file') : null
      const name = file ? file.name : ''
      const isSource = name.includes('src')
      return {
        ok: true,
        status: 200,
        json: async () => ({
          status: 'success',
          type: 'EXCEL',
          filename: name,
          columns: isSource ? SRC_COLUMNS : DEST_COLUMNS,
        }),
      }
    }

    // Permissive catch-all for anything else (dictionary, scheduler, calibration, auth/me, etc.)
    return { ok: true, status: 200, json: async () => ({}), text: async () => '{}' }
  })
}

beforeEach(() => {
  global.fetch = makeServer()
  useMatcherStore.setState({
    config: defaultConfig(),
    error: null,
    loading: false,
    user: null,
  })
  localStorage.setItem('entity_matcher_token', 'test-token')
})

// --- DOM helpers ------------------------------------------------------

const srcPairingContainer = () => screen.getByText('Source Primary Name Column(s)').closest('div')
const destPairingContainer = () => screen.getByText('Destination Primary Name Column(s)').closest('div')

const setBothTypesToExcel = () => {
  const combos = screen.getAllByRole('combobox')
  // ConnectionManager always renders the source type <select> first, then the
  // destination type <select>, regardless of the currently selected type --
  // both are unconditional, so these indices are stable.
  fireEvent.change(combos[0], { target: { value: 'EXCEL' } })
  fireEvent.change(combos[1], { target: { value: 'EXCEL' } })
}

const getFileInputs = () => document.querySelectorAll('input[type="file"]')

const introspectBothExcelSides = async () => {
  setBothTypesToExcel()

  const srcFile = new File(['x'], 'src.xlsx', { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' })
  const destFile = new File(['x'], 'dest.xlsx', { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' })

  const fileInputsBefore = getFileInputs()
  expect(fileInputsBefore.length).toBe(2)
  fireEvent.change(fileInputsBefore[0], { target: { files: [srcFile] } })
  fireEvent.click(screen.getByRole('button', { name: /Introspect Source Columns/i }))
  await screen.findByText(/Introspected 3 columns from src\.xlsx/i)

  const fileInputsAfter = getFileInputs()
  fireEvent.change(fileInputsAfter[1], { target: { files: [destFile] } })
  fireEvent.click(screen.getByRole('button', { name: /Introspect Destination Columns/i }))
  await screen.findByText(/Introspected 3 columns from dest\.xlsx/i)
}

describe('Excel introspection -> Schema Pairing persistence across remount', () => {
  it('TEST: introspected Excel columns reach the Schema Pairing section', async () => {
    render(<ConfigPanel />)
    await screen.findAllByRole('combobox')

    await introspectBothExcelSides()

    expect(within(srcPairingContainer()).getByText('CustID')).toBeInTheDocument()
    expect(within(destPairingContainer()).getByText('client_ref')).toBeInTheDocument()

    // The hardcoded FieldMapper demo fallback column must NOT be present --
    // its presence would mean introspected columns never reached FieldMapper
    // and it fell back to its built-in demo list.
    expect(screen.queryByText('first_name')).not.toBeInTheDocument()
  })

  it('TEST: the pairing selection survives a menu switch', async () => {
    const { unmount } = render(<ConfigPanel />)
    await screen.findAllByRole('combobox')

    await introspectBothExcelSides()

    // Tick CustID (source) and client_ref (destination) as pairing columns.
    const custIDButtonBeforeSave = within(srcPairingContainer()).getByText('CustID').closest('button')
    fireEvent.click(custIDButtonBeforeSave)
    const clientRefButtonBeforeSave = within(destPairingContainer()).getByText('client_ref').closest('button')
    fireEvent.click(clientRefButtonBeforeSave)

    // Sanity: selected immediately renders the CheckSquare icon (DOM signal
    // used throughout this test: presence of an svg.lucide-square-check-big
    // inside the toggle button means selected; svg.lucide-square means not).
    expect(within(srcPairingContainer()).getByText('CustID').closest('button').querySelector('svg.lucide-square-check-big')).toBeTruthy()

    // Save Configuration -- writes both /api/config and /api/connector/settings.
    fireEvent.click(screen.getByRole('button', { name: /Save Configuration/i }))
    await screen.findByText('Configuration updated successfully!')

    // Navigate away: App.jsx conditionally renders {activeTab === 'config' && <ConfigPanel />},
    // so switching tabs fully unmounts ConfigPanel.
    unmount()

    // Navigate back: fresh mount.
    render(<ConfigPanel />)
    await screen.findAllByRole('combobox')

    // (a) CustID must still be listed in the Schema Pairing section.
    await waitFor(() => {
      expect(within(srcPairingContainer()).getByText('CustID')).toBeInTheDocument()
    })

    // (b) CustID must still be marked SELECTED -- asserted via the
    // CheckSquare vs Square lucide icon class on the toggle button.
    await waitFor(() => {
      const btn = within(srcPairingContainer()).getByText('CustID').closest('button')
      expect(btn.querySelector('svg.lucide-square-check-big')).toBeTruthy()
      expect(btn.querySelector('svg.lucide-square')).toBeFalsy()
    })
  })

  it('TEST: Save Configuration persists the connector columns', async () => {
    render(<ConfigPanel />)
    await screen.findAllByRole('combobox')

    await introspectBothExcelSides()

    const custIDButton = within(srcPairingContainer()).getByText('CustID').closest('button')
    fireEvent.click(custIDButton)

    fireEvent.click(screen.getByRole('button', { name: /Save Configuration/i }))
    await screen.findByText('Configuration updated successfully!')

    const putSettingsCall = global.fetch.mock.calls.find(
      (call) => call[0] === '/api/connector/settings' && call[1] && call[1].method === 'PUT'
    )
    expect(putSettingsCall).toBeDefined()
    const settingsBody = JSON.parse(putSettingsCall[1].body)
    expect(settingsBody.source.columns).toContain('CustID')

    const putConfigCall = global.fetch.mock.calls.find(
      (call) => call[0] === '/api/config' && call[1] && call[1].method === 'PUT'
    )
    expect(putConfigCall).toBeDefined()
    const configBody = JSON.parse(putConfigCall[1].body)
    expect(configBody.column_mapping.name_fields_src).toContain('CustID')
  })

  it('TEST: no password is ever sent to connector settings', async () => {
    render(<ConfigPanel />)
    await screen.findAllByRole('combobox')

    // Default source/destination types are database connectors (SQLSERVER /
    // MONGODB), so the password field is visible without switching type.
    const secret = 'SuperSecretDbPass!42'
    const pwInputs = screen.getAllByPlaceholderText('Password')
    fireEvent.change(pwInputs[0], { target: { value: secret } })

    fireEvent.click(screen.getByRole('button', { name: /Save Configuration/i }))
    await screen.findByText('Configuration updated successfully!')

    const settingsCalls = global.fetch.mock.calls.filter(
      (call) => call[0] === '/api/connector/settings' && call[1] && call[1].method === 'PUT'
    )
    expect(settingsCalls.length).toBeGreaterThan(0)
    for (const call of settingsCalls) {
      expect(call[1].body).not.toContain(secret)
    }
  })
})
