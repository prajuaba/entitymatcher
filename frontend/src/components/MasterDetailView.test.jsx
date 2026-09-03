import { render, screen, waitFor } from '@testing-library/react'
import { MasterDetailView } from './MasterDetailView'
import { useMatcherStore } from '../store/useMatcherStore.js'

beforeEach(() => {
  useMatcherStore.setState({ 
    batchID: 'benchmark-batch-001', 
    jobs: [], 
    error: null, 
    loading: false 
  })
})

describe('MasterDetailView', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('TEST: defaults to the most recent job when nothing is stored', async () => {
    localStorage.clear()
    
    useMatcherStore.setState({ batchID: '' })

    global.fetch = vi.fn(async (url) => {
      if (url === '/api/jobs') {
        return { 
          ok: true, 
          status: 200, 
          json: async () => ({ 
            count: 2, 
            jobs: [ 
              { batch_id: 'batch-newest', status: 'COMPLETED', auto_matched: 10, review_needed: 2 }, 
              { batch_id: 'batch-older', status: 'COMPLETED', auto_matched: 5, review_needed: 1 } 
            ] 
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

    render(<MasterDetailView />)

    await waitFor(() => expect(useMatcherStore.getState().batchID).toBe('batch-newest'))
    expect(useMatcherStore.getState().batchID).not.toBe('benchmark-batch-001')
  })

  it('TEST: a stored batch id survives a remount', async () => {
    localStorage.setItem('entity_matcher_batch_id', 'batch-stored')
    useMatcherStore.setState({ batchID: 'batch-stored', jobs: [] })

    global.fetch = vi.fn(async (url) => {
      if (url === '/api/jobs') {
        return { 
          ok: true, 
          status: 200, 
          json: async () => ({ 
            count: 1, 
            jobs: [ 
              { batch_id: 'batch-other', status: 'COMPLETED', auto_matched: 1, review_needed: 1 } 
            ] 
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

    const { unmount } = render(<MasterDetailView />)
    
    await waitFor(() => expect(useMatcherStore.getState().batchID).toBe('batch-stored'))
    
    unmount()
    
    render(<MasterDetailView />)
    
    await waitFor(() => expect(useMatcherStore.getState().batchID).toBe('batch-stored'))
  })

  it('TEST: selecting a batch persists it', async () => {
    localStorage.clear()
    useMatcherStore.setState({ batchID: '', jobs: [] })

    global.fetch = vi.fn(async (url) => {
      if (url.startsWith('/api/match/results')) {
        return { 
          ok: true, 
          status: 200, 
          json: async () => ({ results: [], total_count: 0 }) 
        }
      }
      if (url === '/api/jobs') {
        return { 
          ok: true, 
          status: 200, 
          json: async () => ({ count: 0, jobs: [] }) 
        }
      }
      throw new Error(`Unexpected fetch call to ${url}`)
    })

    await useMatcherStore.getState().setBatchID('batch-xyz')
    
    expect(localStorage.getItem('entity_matcher_batch_id')).toBe('batch-xyz')
    expect(useMatcherStore.getState().batchID).toBe('batch-xyz')
  })

  it('TEST: the selector keeps an unlisted current batch visible', async () => {
    localStorage.clear()
    useMatcherStore.setState({ batchID: 'batch-unlisted', jobs: [] })

    global.fetch = vi.fn(async (url) => {
      if (url === '/api/jobs') {
        return { 
          ok: true, 
          status: 200, 
          json: async () => ({ 
            count: 1, 
            jobs: [ 
              { batch_id: 'batch-listed', status: 'COMPLETED', auto_matched: 3, review_needed: 4 } 
            ] 
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

    render(<MasterDetailView />)
    
    await waitFor(() => screen.getByTitle('Which match run to review'))

    // The toolbar now also has sort-by and rows-per-page selects (per the
    // paging/sorting spec), so a plain combobox role query is ambiguous;
    // the batch picker keeps its distinguishing title attribute.
    const select = screen.getByTitle('Which match run to review')
    expect(select.querySelector('option[value="batch-unlisted"]')).toBeInTheDocument()
    expect(useMatcherStore.getState().batchID).toBe('batch-unlisted')
  })
})
