import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { LoginScreen } from './LoginScreen.jsx'
import { useMatcherStore } from '../store/useMatcherStore.js'

beforeEach(() => {
  useMatcherStore.setState({ loading: false })
})

describe('LoginScreen', () => {
  it('disables Sign In when either field is empty', async () => {
    render(<LoginScreen />)

    await userEvent.clear(screen.getByPlaceholderText('admin'))
    await userEvent.clear(screen.getByPlaceholderText('••••••••'))

    expect(screen.getByRole('button', { name: 'Sign In' })).toBeDisabled()

    // Type username only, password still empty
    await userEvent.type(screen.getByPlaceholderText('admin'), 'someuser')
    expect(screen.getByRole('button', { name: 'Sign In' })).toBeDisabled()
  })

  it('enables Sign In once both username and password are filled', async () => {
    render(<LoginScreen />)

    await userEvent.clear(screen.getByPlaceholderText('admin'))
    await userEvent.clear(screen.getByPlaceholderText('••••••••'))

    await userEvent.type(screen.getByPlaceholderText('admin'), 'someuser')
    await userEvent.type(screen.getByPlaceholderText('••••••••'), 'somepassword')

    expect(screen.getByRole('button', { name: 'Sign In' })).toBeEnabled()
  })
})

afterEach(() => {
  vi.restoreAllMocks()
})
