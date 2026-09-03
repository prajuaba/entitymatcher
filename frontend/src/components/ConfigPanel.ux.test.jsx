import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
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

describe('ConfigPanel UX', () => {
  it('default tab is Matching Rules; tab switching removes previous content and arrow keys navigate', async () => {
    render(<ConfigPanel />)
    await screen.findAllByRole('slider')

    // Default tab: Matching Rules is active, "Confidence Thresholds" visible
    expect(screen.getByText('Confidence Thresholds')).toBeInTheDocument()
    const matchingRulesTab = screen.getByRole('tab', { name: /Matching Rules/i })
    expect(matchingRulesTab).toHaveAttribute('aria-selected', 'true')

    // Click Algorithms tab
    const algorithmsTab = screen.getByRole('tab', { name: /Algorithms/i })
    fireEvent.click(algorithmsTab)
    await waitFor(() => {
      expect(screen.getByText('Enabled Similarity Metric Algorithms')).toBeInTheDocument()
    })
    expect(screen.queryByText('Confidence Thresholds')).not.toBeInTheDocument()
    expect(algorithmsTab).toHaveAttribute('aria-selected', 'true')
    expect(matchingRulesTab).toHaveAttribute('aria-selected', 'false')

    // Click Data Sources tab
    const dataSourcesTab = screen.getByRole('tab', { name: /Data Sources/i })
    fireEvent.click(dataSourcesTab)
    await waitFor(() => {
      expect(screen.queryByText('Confidence Thresholds')).not.toBeInTheDocument()
    })
    expect(screen.queryByText('Enabled Similarity Metric Algorithms')).not.toBeInTheDocument()
    expect(dataSourcesTab).toHaveAttribute('aria-selected', 'true')

    // ArrowRight from Data Sources (index 0) → Matching Rules (index 1)
    const tablist = screen.getByRole('tablist')
    fireEvent.keyDown(tablist, { key: 'ArrowRight' })
    await waitFor(() => {
      expect(screen.getByText('Confidence Thresholds')).toBeInTheDocument()
    })
    expect(matchingRulesTab).toHaveAttribute('aria-selected', 'true')

    // ArrowRight from Matching Rules (index 1) → Algorithms (index 2)
    fireEvent.keyDown(tablist, { key: 'ArrowRight' })
    await waitFor(() => {
      expect(screen.getByText('Enabled Similarity Metric Algorithms')).toBeInTheDocument()
    })
    expect(algorithmsTab).toHaveAttribute('aria-selected', 'true')

    // ArrowLeft from Algorithms (index 2) → Matching Rules (index 1)
    fireEvent.keyDown(tablist, { key: 'ArrowLeft' })
    await waitFor(() => {
      expect(screen.getByText('Confidence Thresholds')).toBeInTheDocument()
    })
    expect(matchingRulesTab).toHaveAttribute('aria-selected', 'true')
  })

  it('Operations tab is visible only for ADMIN role', async () => {
    // ADMIN: Operations tab exists
    useMatcherStore.setState({ user: { role: 'ADMIN' } })
    let { unmount } = render(<ConfigPanel />)
    await screen.findAllByRole('slider')
    expect(screen.getByRole('tab', { name: /Operations/i })).toBeInTheDocument()
    unmount()

    // ENGINEER: Operations tab absent
    useMatcherStore.setState({ user: { role: 'ENGINEER' } })
    let { unmount: unmount2 } = render(<ConfigPanel />)
    await screen.findAllByRole('slider')
    expect(screen.queryByRole('tab', { name: /Operations/i })).not.toBeInTheDocument()
    unmount2()

    // null user: Operations tab absent
    useMatcherStore.setState({ user: null })
    render(<ConfigPanel />)
    await screen.findAllByRole('slider')
    expect(screen.queryByRole('tab', { name: /Operations/i })).not.toBeInTheDocument()
  })

  it('Algorithms tab renders all 8 toggle labels with correct aria-pressed and toggling works', async () => {
    render(<ConfigPanel />)
    await screen.findAllByRole('slider')

    // Navigate to Algorithms tab
    fireEvent.click(screen.getByRole('tab', { name: /Algorithms/i }))
    await waitFor(() => {
      expect(screen.getByText('Enabled Similarity Metric Algorithms')).toBeInTheDocument()
    })

    const algoLabels = [
      'Jaro-Winkler Distance',
      'Token Sort Ratio',
      'Levenshtein Edit Distance',
      'Character Trigram Overlap',
      'Phonetic Consonant Match',
      'Thai Phonetic Key',
      'Corpus IDF Weighting',
      'Romanized Matching',
    ]

    // All should be aria-pressed="true" by default (config has all true)
    for (const label of algoLabels) {
      const btn = screen.getByRole('button', { name: new RegExp(label, 'i') })
      expect(btn).toHaveAttribute('aria-pressed', 'true')
    }

    // Toggle one off: Jaro-Winkler
    const jwBtn = screen.getByRole('button', { name: /Jaro-Winkler Distance/i })
    fireEvent.click(jwBtn)
    await waitFor(() => {
      expect(jwBtn).toHaveAttribute('aria-pressed', 'false')
    })

    // Others remain true
    const tsBtn = screen.getByRole('button', { name: /Token Sort Ratio/i })
    expect(tsBtn).toHaveAttribute('aria-pressed', 'true')

    // Toggle it back on
    fireEvent.click(jwBtn)
    await waitFor(() => {
      expect(jwBtn).toHaveAttribute('aria-pressed', 'true')
    })
  })

  it('Select all / Clear all set every toggle and the warning appears/disappears accordingly', async () => {
    render(<ConfigPanel />)
    await screen.findAllByRole('slider')

    fireEvent.click(screen.getByRole('tab', { name: /Algorithms/i }))
    await waitFor(() => {
      expect(screen.getByText('Enabled Similarity Metric Algorithms')).toBeInTheDocument()
    })

    const algoLabels = [
      'Jaro-Winkler Distance',
      'Token Sort Ratio',
      'Levenshtein Edit Distance',
      'Character Trigram Overlap',
      'Phonetic Consonant Match',
      'Thai Phonetic Key',
      'Corpus IDF Weighting',
      'Romanized Matching',
    ]

    // Clear all → all false, warning visible
    fireEvent.click(screen.getByRole('button', { name: /Clear all/i }))
    await waitFor(() => {
      for (const label of algoLabels) {
        const btn = screen.getByRole('button', { name: new RegExp(label, 'i') })
        expect(btn).toHaveAttribute('aria-pressed', 'false')
      }
    })
    expect(screen.getByText('Name similarity cannot be computed with every algorithm disabled.')).toBeInTheDocument()

    // Select all → all true, warning gone
    fireEvent.click(screen.getByRole('button', { name: /Select all/i }))
    await waitFor(() => {
      for (const label of algoLabels) {
        const btn = screen.getByRole('button', { name: new RegExp(label, 'i') })
        expect(btn).toHaveAttribute('aria-pressed', 'true')
      }
    })
    expect(screen.queryByText('Name similarity cannot be computed with every algorithm disabled.')).not.toBeInTheDocument()
  })

  it('Score Margin readout shows 0% when margin_threshold is 0, and 5% when absent', async () => {
    // Case 1: margin_threshold explicitly 0
    const cfgZero = defaultConfig()
    cfgZero.margin_threshold = 0
    serverConfig = { ...serverConfig, margin_threshold: 0 }
    useMatcherStore.setState({ config: cfgZero })

    let { unmount } = render(<ConfigPanel />)
    await screen.findAllByRole('slider')

    // On Matching Rules tab (default), find the Score Margin row
    const marginSlider = screen.getByLabelText('Score Margin Threshold')
    expect(marginSlider).toHaveAttribute('value', '0')
    const marginLabel = screen.getByText('Score Margin Threshold')
    const marginRow = marginLabel.parentElement
    expect(within(marginRow).getByText('0%')).toBeInTheDocument()

    unmount()

    // Case 2: margin_threshold absent (undefined)
    const cfgAbsent = defaultConfig()
    delete cfgAbsent.margin_threshold
    serverConfig = { ...serverConfig }
    delete serverConfig.margin_threshold
    useMatcherStore.setState({ config: cfgAbsent })

    render(<ConfigPanel />)
    await screen.findAllByRole('slider')

    const marginSlider2 = screen.getByLabelText('Score Margin Threshold')
    const marginLabel2 = screen.getByText('Score Margin Threshold')
    const marginRow2 = marginLabel2.parentElement
    expect(within(marginRow2).getByText('5%')).toBeInTheDocument()
    expect(within(marginRow2).queryByText('10%')).not.toBeInTheDocument()
  })

  it('threshold band aria-label reflects values; auto clamp and review clamp both work', async () => {
    render(<ConfigPanel />)
    await screen.findAllByRole('slider')

    // Default: review=70%, auto=90%
    const img = screen.getByRole('img')
    expect(img).toHaveAttribute(
      'aria-label',
      'Score ranges: 0 to 70% is no match, 70% to 90% is manual review, 90% to 100% is auto-match.'
    )

    // Drag Auto-Match below Review: set auto to 0.60, review was 0.70 → review clamps to 0.60
    const autoSlider = screen.getByLabelText('Auto-Match Threshold')
    fireEvent.change(autoSlider, { target: { value: '0.60' } })
    await waitFor(() => {
      const img2 = screen.getByRole('img')
      expect(img2).toHaveAttribute(
        'aria-label',
        'Score ranges: 0 to 60% is no match, 60% to 60% is manual review, 60% to 100% is auto-match.'
      )
    })
    // Manual Review Threshold readout should now show 60%
    const reviewLabel = screen.getByText('Manual Review Threshold')
    const reviewRow = reviewLabel.parentElement
    expect(within(reviewRow).getByText('60%')).toBeInTheDocument()

    // Reset to defaults for second scenario
    const reviewSlider = screen.getByLabelText('Manual Review Threshold')

    // Now set auto to 0.80 (below the Review slider's own max of 0.90, so a
    // subsequent Review drag to 0.85 is not clamped by the native `max`
    // attribute before onChange even fires -- it must be the JS clamp logic
    // that pulls it back down to Auto).
    fireEvent.change(autoSlider, { target: { value: '0.80' } })
    await waitFor(() => {
      const img3 = screen.getByRole('img')
      expect(img3).toHaveAttribute(
        'aria-label',
        'Score ranges: 0 to 60% is no match, 60% to 80% is manual review, 80% to 100% is auto-match.'
      )
    })

    // Drag Review above Auto: set review to 0.85 while auto is 0.80 → clamped to 0.80
    fireEvent.change(reviewSlider, { target: { value: '0.85' } })
    await waitFor(() => {
      const reviewRow2 = screen.getByText('Manual Review Threshold').parentElement
      expect(within(reviewRow2).getByText('80%')).toBeInTheDocument()
    })
    expect(screen.getByText(/Review threshold was lowered to stay at or below Auto-Match/i)).toBeInTheDocument()

    const img4 = screen.getByRole('img')
    expect(img4).toHaveAttribute(
      'aria-label',
      'Score ranges: 0 to 80% is no match, 80% to 80% is manual review, 80% to 100% is auto-match.'
    )
  })

  it('cap >= auto shows validation warning and disables Save; lowering cap re-enables Save', async () => {
    render(<ConfigPanel />)
    await screen.findAllByRole('slider')

    // Navigate to Algorithms tab to access the cap slider
    fireEvent.click(screen.getByRole('tab', { name: /Algorithms/i }))
    await waitFor(() => {
      expect(screen.getByText('Advanced Scorer Tuning')).toBeInTheDocument()
    })

    const capSlider = screen.getByLabelText('No-Distinctive-Overlap Score Cap')

    // Set cap to 0.95 while auto is 0.90 → cap >= auto → violation
    fireEvent.change(capSlider, { target: { value: '0.95' } })
    await waitFor(() => {
      expect(screen.getByText('Cap must be less than Auto-Match Threshold, otherwise it can never demote a match.')).toBeInTheDocument()
    })
    const saveBtn = screen.getByRole('button', { name: /Save Configuration/i })
    expect(saveBtn).toBeDisabled()

    // Lower cap to 0.50 → no violation
    fireEvent.change(capSlider, { target: { value: '0.50' } })
    await waitFor(() => {
      expect(screen.queryByText('Cap must be less than Auto-Match Threshold, otherwise it can never demote a match.')).not.toBeInTheDocument()
    })
    expect(saveBtn).not.toBeDisabled()
  })

  it('Save is enabled and Reset is disabled on fresh mount; after one change Reset becomes enabled', async () => {
    render(<ConfigPanel />)
    await screen.findAllByRole('slider')

    const saveBtn = screen.getByRole('button', { name: /Save Configuration/i })
    const resetBtn = screen.getByRole('button', { name: /Reset/i })

    // Fresh mount: Save enabled, Reset disabled
    expect(saveBtn).not.toBeDisabled()
    expect(resetBtn).toBeDisabled()

    // Make one change
    const autoSlider = screen.getByLabelText('Auto-Match Threshold')
    fireEvent.change(autoSlider, { target: { value: '0.92' } })
    await waitFor(() => {
      expect(resetBtn).not.toBeDisabled()
    })
  })

  it('dirty badge shows singular for 1 change, plural for 2, and disappears after Reset', async () => {
    render(<ConfigPanel />)
    await screen.findAllByRole('slider')

    // No dirty badge on fresh mount
    expect(screen.queryByText(/unsaved/i)).not.toBeInTheDocument()

    // Change auto_match_threshold → 1 dirty
    const autoSlider = screen.getByLabelText('Auto-Match Threshold')
    fireEvent.change(autoSlider, { target: { value: '0.93' } })
    await waitFor(() => {
      expect(screen.getByText('1 unsaved change')).toBeInTheDocument()
    })

    // Change margin_threshold → 2 dirty
    const marginSlider = screen.getByLabelText('Score Margin Threshold')
    fireEvent.change(marginSlider, { target: { value: '0.10' } })
    await waitFor(() => {
      expect(screen.getByText('2 unsaved changes')).toBeInTheDocument()
    })

    // Reset → badge disappears
    const resetBtn = screen.getByRole('button', { name: /Reset/i })
    fireEvent.click(resetBtn)
    await waitFor(() => {
      expect(screen.queryByText(/unsaved/i)).not.toBeInTheDocument()
    })
  })

  it('Advanced Scorer Tuning sliders exist with correct aria-labels and readouts update', async () => {
    render(<ConfigPanel />)
    await screen.findAllByRole('slider')

    fireEvent.click(screen.getByRole('tab', { name: /Algorithms/i }))
    await waitFor(() => {
      expect(screen.getByText('Advanced Scorer Tuning')).toBeInTheDocument()
    })

    // Verify all three sliders exist
    const crossScriptSlider = screen.getByLabelText('Cross-Script Auto-Match Threshold')
    const capSlider = screen.getByLabelText('No-Distinctive-Overlap Score Cap')
    const idfSlider = screen.getByLabelText('Distinctive Token IDF Floor')
    expect(crossScriptSlider).toBeInTheDocument()
    expect(capSlider).toBeInTheDocument()
    expect(idfSlider).toBeInTheDocument()

    // Default readouts: 84%, 85%, 30%
    const crossLabel = screen.getByText('Cross-Script Auto-Match Threshold')
    const crossRow = crossLabel.parentElement
    expect(within(crossRow).getByText('84%')).toBeInTheDocument()

    const idfLabel = screen.getByText('Distinctive Token IDF Floor')
    const idfRow = idfLabel.parentElement
    expect(within(idfRow).getByText('30%')).toBeInTheDocument()

    // Interact: change Cross-Script to 0.80
    fireEvent.change(crossScriptSlider, { target: { value: '0.80' } })
    await waitFor(() => {
      expect(within(crossRow).getByText('80%')).toBeInTheDocument()
    })

    // Interact: change IDF Floor to 0.45
    fireEvent.change(idfSlider, { target: { value: '0.45' } })
    await waitFor(() => {
      expect(within(idfRow).getByText('45%')).toBeInTheDocument()
    })
  })

  it('date tolerance preset buttons reflect aria-pressed; non-preset value shows in number input with no preset pressed', async () => {
    // Non-preset value: 14
    const cfg14 = defaultConfig()
    cfg14.date_tolerance_days = 14
    serverConfig = { ...serverConfig, date_tolerance_days: 14 }
    useMatcherStore.setState({ config: cfg14 })

    render(<ConfigPanel />)
    await screen.findAllByRole('slider')

    // Number input shows 14
    const numInput = screen.getByLabelText('Date Tolerance Days')
    expect(numInput).toHaveValue(14)

    // No preset button is pressed (0,1,3,7,30 none equal 14)
    const presetTexts = ['±0 days', '±1 day', '±3 days', '±7 days', '±30 days']
    for (const text of presetTexts) {
      const btn = screen.getByRole('button', { name: text })
      expect(btn).toHaveAttribute('aria-pressed', 'false')
    }
  })
})

