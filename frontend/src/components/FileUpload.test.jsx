import { render, screen, fireEvent } from '@testing-library/react'
import { FileUpload } from './FileUpload.jsx'
import { useMatcherStore } from '../store/useMatcherStore.js'

beforeEach(() => {
  useMatcherStore.setState({ loading: false, error: null })
})

describe('FileUpload', () => {
  it('shows an inline error and never calls window.alert when the pasted JSON is invalid', async () => {
    const alertSpy = vi.spyOn(window, 'alert')
    
    render(<FileUpload />)
    
    const textareas = document.querySelectorAll('textarea')
    fireEvent.change(textareas[0], { target: { value: '' } })
    fireEvent.change(textareas[1], { target: { value: '' } })
    
    const button = screen.getByRole('button', { name: /Stream Ingest & Execute Matching/i })
    fireEvent.click(button)
    
    const matches = await screen.findAllByText('Please provide valid JSON arrays for both Source and Destination records.')
    expect(matches.length).toBeGreaterThan(0)
    expect(window.alert).not.toHaveBeenCalled()
  })

  it('shows an inline error and never calls window.alert when the pasted JSON is malformed', async () => {
    const alertSpy = vi.spyOn(window, 'alert')
    
    render(<FileUpload />)
    
    const textareas = document.querySelectorAll('textarea')
    fireEvent.change(textareas[0], { target: { value: '{not valid json' } })
    fireEvent.change(textareas[1], { target: { value: '' } })
    
    const button = screen.getByRole('button', { name: /Stream Ingest & Execute Matching/i })
    fireEvent.click(button)
    
    const matches = await screen.findAllByText(/Failed to upload records:/i)
    expect(matches.length).toBeGreaterThan(0)
    expect(window.alert).not.toHaveBeenCalled()
  })
})

afterEach(() => {
  vi.restoreAllMocks()
})
