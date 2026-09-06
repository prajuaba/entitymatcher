import { render, screen } from '@testing-library/react'
import { ProgressDashboard } from './ProgressDashboard'
import { useMatcherStore } from '../store/useMatcherStore.js'

beforeEach(() => {
  useMatcherStore.setState({
    batchID: '',
    loading: false,
    progress: {
      status: 'IDLE',
      total_sources: 0,
      processed_sources: 0,
      total_candidate_pairs: 0,
      auto_matched: 0,
      review_needed: 0,
      no_match_count: 0,
      total_decisions: 0,
      elapsed_ms: 0,
      batch_id: ''
    },
    config: {}
  })
})

afterEach(() => {
  vi.restoreAllMocks()
})

describe('ProgressDashboard', () => {
  it('TEST 1: shows View Pair Results button when status is COMPLETED and no run has been started', async () => {
    useMatcherStore.setState({
      batchID: 'batch-done',
      loading: false,
      progress: {
        batch_id: 'batch-done',
        total_sources: 100,
        processed_sources: 100,
        total_candidate_pairs: 60,
        auto_matched: 50,
        review_needed: 8,
        no_match_count: 2,
        total_decisions: 55,
        status: 'COMPLETED',
        elapsed_ms: 5000
      },
      config: { auto_match_threshold: 0.90 }
    })

    render(<ProgressDashboard />)

    expect(screen.getByText('View Pair Results')).toBeInTheDocument()
  })

  it('TEST 2: does not show "70% - 89%" string anywhere in the component', async () => {
    useMatcherStore.setState({
      batchID: 'batch-done',
      loading: false,
      progress: {
        batch_id: 'batch-done',
        total_sources: 100,
        processed_sources: 100,
        total_candidate_pairs: 60,
        auto_matched: 50,
        review_needed: 8,
        no_match_count: 2,
        total_decisions: 55,
        status: 'COMPLETED',
        elapsed_ms: 5000
      },
      config: { auto_match_threshold: 0.90 }
    })

    render(<ProgressDashboard />)

    expect(screen.queryByText(/70%\s*-\s*89%/)).not.toBeInTheDocument()
    expect(screen.getByText('Awaiting human review')).toBeInTheDocument()
    expect(document.body.textContent).not.toMatch(/70%\s*-\s*89%/)
  })

  it('TEST 3: derives Auto-Matched threshold label from config, not hardcoded 90%', async () => {
    useMatcherStore.setState({
      batchID: 'batch-cfg',
      loading: false,
      progress: {
        batch_id: 'batch-cfg',
        total_sources: 10,
        processed_sources: 10,
        total_candidate_pairs: 5,
        auto_matched: 4,
        review_needed: 1,
        no_match_count: 0,
        total_decisions: 4,
        status: 'COMPLETED',
        elapsed_ms: 100
      },
      config: { auto_match_threshold: 0.95 }
    })

    render(<ProgressDashboard />)

    expect(screen.getByText(/Confidence\s*≥\s*95%/)).toBeInTheDocument()
    expect(screen.queryByText(/\b90%/)).not.toBeInTheDocument()
  })
})
