import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { DictionaryManager } from './DictionaryManager.jsx'

describe('DictionaryManager', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('issues a DELETE to /api/dictionary with the alias correctly URL-encoded, including non-ASCII Thai aliases', async () => {
    global.fetch = vi.fn(async (url, options) => {
      if (url === '/api/dictionary') {
        return {
          ok: true,
          status: 200,
          json: async () => ({
            entries: [
              {
                alias: 'ไทยพาณิชย์',
                canonical: 'Siam Commercial Bank'
              }
            ]
          })
        }
      }
      if (url.startsWith('/api/dictionary?alias=')) {
        return {
          ok: true,
          status: 200,
          json: async () => ({ status: 'success', entries: [] })
        }
      }
      throw new Error(`Unexpected fetch call to ${url}`)
    })

    render(<DictionaryManager />)

    await screen.findByText('ไทยพาณิชย์')

    const deleteButton = screen.getByRole('button', { name: 'Remove alias ไทยพาณิชย์' })
    await userEvent.click(deleteButton)

    const deleteCall = global.fetch.mock.calls.find(call => call[0].startsWith('/api/dictionary?alias='))
    expect(deleteCall).toBeDefined()
    expect(deleteCall[0]).toEqual(`/api/dictionary?alias=${encodeURIComponent('ไทยพาณิชย์')}`)
    expect(deleteCall[1]).toEqual(expect.objectContaining({ method: 'DELETE' }))
  })

  it('re-renders the chip list from the server\'s returned entries, not from local filtering', async () => {
    global.fetch = vi.fn(async (url, options) => {
      if (url === '/api/dictionary') {
        return {
          ok: true,
          status: 200,
          json: async () => ({
            entries: [
              { alias: 'KBank', canonical: 'Kasikornbank' },
              { alias: 'SCB', canonical: 'Siam Commercial Bank' }
            ]
          })
        }
      }
      if (url.startsWith('/api/dictionary?alias=')) {
        return {
          ok: true,
          status: 200,
          json: async () => ({
            status: 'success',
            entries: [{ alias: 'UnrelatedServerAlias', canonical: 'Something Else' }]
          })
        }
      }
      throw new Error(`Unexpected fetch call to ${url}`)
    })

    render(<DictionaryManager />)

    await screen.findByText('KBank')
    await screen.findByText('SCB')

    const deleteButton = screen.getByRole('button', { name: 'Remove alias SCB' })
    await userEvent.click(deleteButton)

    await screen.findByText('UnrelatedServerAlias')
    expect(screen.queryByText('KBank')).not.toBeInTheDocument()
    expect(screen.queryByText('SCB')).not.toBeInTheDocument()
  })

  it('renders an inline error on a failed delete and leaves the alias visible', async () => {
    global.fetch = vi.fn(async (url, options) => {
      if (url === '/api/dictionary') {
        return {
          ok: true,
          status: 200,
          json: async () => ({
            entries: [
              { alias: 'KBank', canonical: 'Kasikornbank' }
            ]
          })
        }
      }
      if (url.startsWith('/api/dictionary?alias=')) {
        // The backend reports this failure with http.Error, which writes PLAIN
        // TEXT, not JSON. Mocking a JSON body here would test a response shape
        // the server never sends, and would pass even if the component parsed
        // the body in a way that throws on the real thing.
        return {
          ok: false,
          status: 500,
          text: async () => 'Failed to delete dictionary entry: write failed\n'
        }
      }
      throw new Error(`Unexpected fetch call to ${url}`)
    })

    render(<DictionaryManager />)

    await screen.findByText('KBank')

    const deleteButton = screen.getByRole('button', { name: 'Remove alias KBank' })
    await userEvent.click(deleteButton)

    expect(await screen.findByText('Failed to delete dictionary entry: write failed')).toBeInTheDocument()
    expect(screen.getByText('KBank')).toBeInTheDocument()
  })
})
