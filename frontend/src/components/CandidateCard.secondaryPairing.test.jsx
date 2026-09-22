import { render, screen } from '@testing-library/react'
import { CandidateCard } from './CandidateCard.jsx'
import { useMatcherStore } from '../store/useMatcherStore.js'

const baseMatchItem = {
  id: 'batch-1-src-1-dest-1',
  match_status: 'REVIEW_NEEDED',
  rank: 1,
  score_margin: 0.1,
  decision_note: '',
  confidence_score: 0.9,
  name_score: 0.95,
  date_score: 0.8,
  jw_score: 0.9,
  lev_score: 0.85,
  token_score: 0.9,
  trigram_score: 0.8,
  match_reasons: [],
  source: {
    reference_id: 'REF-1',
    customer_name_raw: 'Acme Trading Co',
    transaction_date: '2026-01-01T00:00:00Z',
    transaction_type: 'WIRE',
  },
  destination: {
    customer_id: 'CUST-1',
    customer_name_raw: 'Acme Trading Company',
    transaction_date: '2026-01-01T00:00:00Z',
  },
}

beforeEach(() => {
  global.fetch = vi.fn()
  useMatcherStore.setState({
    updateMatchAction: vi.fn(),
    evaluateLLM: vi.fn(),
    setManualSearchOpen: vi.fn(),
    loading: false,
    user: null,
    config: { auto_match_threshold: 0.9 },
  })
})

describe('CandidateCard secondary attribute pairing display', () => {
  it('TEST: shows no secondary bar and default 85/15 weights when no secondary fields are configured', () => {
    render(<CandidateCard matchItem={{ ...baseMatchItem, secondary_score: 0, secondary_weight: 0 }} />)

    expect(screen.queryByText(/Secondary Attribute Match/)).not.toBeInTheDocument()
    expect(screen.getByText('Name Similarity Score (85%)')).toBeInTheDocument()
    expect(screen.getByText('Date Match Score (15%)')).toBeInTheDocument()
  })

  it('TEST: omitted secondary_weight also hides the secondary bar', () => {
    const { secondary_score, secondary_weight, ...withoutSecondary } = { ...baseMatchItem }
    render(<CandidateCard matchItem={withoutSecondary} />)

    expect(screen.queryByText(/Secondary Attribute Match/)).not.toBeInTheDocument()
    expect(screen.getByText('Name Similarity Score (85%)')).toBeInTheDocument()
  })

  it('TEST: renders the secondary pairing score and rescales name/date weight labels when configured', () => {
    render(
      <CandidateCard
        matchItem={{ ...baseMatchItem, secondary_score: 0.75, secondary_weight: 0.2 }}
      />
    )

    // 0.85 * (1 - 0.2) = 68%, 0.15 * (1 - 0.2) = 12%, secondary weight = 20%
    expect(screen.getByText('Name Similarity Score (68%)')).toBeInTheDocument()
    expect(screen.getByText('Date Match Score (12%)')).toBeInTheDocument()
    expect(screen.getByText('Secondary Attribute Match (20%)')).toBeInTheDocument()
    expect(screen.getByText('75.0%')).toBeInTheDocument()
  })
})
