import { render, screen, cleanup } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { CalibrationPanel } from './CalibrationPanel.jsx'

describe('CalibrationPanel', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('shows progress toward the minimum', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        calibration_enabled: false,
        has_active_model: false,
        active_model: null,
        observation_count: 14,
        positive_count: 9,
        negative_count: 5,
        by_previous_status: { REVIEW_NEEDED: 12, AUTO_MATCHED: 2 },
        caveat: 'Training data is drawn almost entirely from the human review queue; this is a known selection bias.'
      }),
      text: async () => JSON.stringify({
        calibration_enabled: false,
        has_active_model: false,
        active_model: null,
        observation_count: 14,
        positive_count: 9,
        negative_count: 5,
        by_previous_status: { REVIEW_NEEDED: 12, AUTO_MATCHED: 2 },
        caveat: 'Training data is drawn almost entirely from the human review queue; this is a known selection bias.'
      })
    })

    render(<CalibrationPanel />)

    await screen.findByText('14 / 20')
    await screen.findByText(/6 more reviewer decisions/i)
  })

  it('disables fitting below the minimum', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        calibration_enabled: false,
        has_active_model: false,
        active_model: null,
        observation_count: 14,
        positive_count: 9,
        negative_count: 5,
        by_previous_status: { REVIEW_NEEDED: 12, AUTO_MATCHED: 2 },
        caveat: 'Training data is drawn almost entirely from the human review queue; this is a known selection bias.'
      }),
      text: async () => JSON.stringify({
        calibration_enabled: false,
        has_active_model: false,
        active_model: null,
        observation_count: 14,
        positive_count: 9,
        negative_count: 5,
        by_previous_status: { REVIEW_NEEDED: 12, AUTO_MATCHED: 2 },
        caveat: 'Training data is drawn almost entirely from the human review queue; this is a known selection bias.'
      })
    })

    render(<CalibrationPanel />)

    const button = await screen.findByRole('button', { name: /Fit Calibrator/i })
    expect(button).toBeDisabled()
  })

  it('enables fitting at the minimum', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        calibration_enabled: false,
        has_active_model: false,
        active_model: null,
        observation_count: 20,
        positive_count: 9,
        negative_count: 5,
        by_previous_status: { REVIEW_NEEDED: 12, AUTO_MATCHED: 2 },
        caveat: 'Training data is drawn almost entirely from the human review queue; this is a known selection bias.'
      }),
      text: async () => JSON.stringify({
        calibration_enabled: false,
        has_active_model: false,
        active_model: null,
        observation_count: 20,
        positive_count: 9,
        negative_count: 5,
        by_previous_status: { REVIEW_NEEDED: 12, AUTO_MATCHED: 2 },
        caveat: 'Training data is drawn almost entirely from the human review queue; this is a known selection bias.'
      })
    })

    render(<CalibrationPanel />)

    const button = await screen.findByRole('button', { name: /Fit Calibrator/i })
    expect(button).not.toBeDisabled()
  })

  it('warns when calibration is disabled', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        calibration_enabled: false,
        has_active_model: false,
        active_model: null,
        observation_count: 25,
        positive_count: 9,
        negative_count: 5,
        by_previous_status: { REVIEW_NEEDED: 12, AUTO_MATCHED: 2 },
        caveat: 'Training data is drawn almost entirely from the human review queue; this is a known selection bias.'
      }),
      text: async () => JSON.stringify({
        calibration_enabled: false,
        has_active_model: false,
        active_model: null,
        observation_count: 25,
        positive_count: 9,
        negative_count: 5,
        by_previous_status: { REVIEW_NEEDED: 12, AUTO_MATCHED: 2 },
        caveat: 'Training data is drawn almost entirely from the human review queue; this is a known selection bias.'
      })
    })

    render(<CalibrationPanel />)

    await screen.findByText(/Fitting stores a model but it will not affect scoring until calibration is enabled in the engine configuration/i)

    cleanup()

    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        calibration_enabled: true,
        has_active_model: false,
        active_model: null,
        observation_count: 25,
        positive_count: 9,
        negative_count: 5,
        by_previous_status: { REVIEW_NEEDED: 12, AUTO_MATCHED: 2 },
        caveat: 'Training data is drawn almost entirely from the human review queue; this is a known selection bias.'
      }),
      text: async () => JSON.stringify({
        calibration_enabled: true,
        has_active_model: false,
        active_model: null,
        observation_count: 25,
        positive_count: 9,
        negative_count: 5,
        by_previous_status: { REVIEW_NEEDED: 12, AUTO_MATCHED: 2 },
        caveat: 'Training data is drawn almost entirely from the human review queue; this is a known selection bias.'
      })
    })

    render(<CalibrationPanel />)

    expect(screen.queryByText(/Fitting stores a model but it will not affect scoring until calibration is enabled in the engine configuration/i)).not.toBeInTheDocument()
  })

  it('renders the server caveat verbatim', async () => {
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        calibration_enabled: false,
        has_active_model: false,
        active_model: null,
        observation_count: 25,
        positive_count: 9,
        negative_count: 5,
        by_previous_status: { REVIEW_NEEDED: 12, AUTO_MATCHED: 2 },
        caveat: 'Training data is drawn almost entirely from the human review queue; this is a known selection bias.'
      }),
      text: async () => JSON.stringify({
        calibration_enabled: false,
        has_active_model: false,
        active_model: null,
        observation_count: 25,
        positive_count: 9,
        negative_count: 5,
        by_previous_status: { REVIEW_NEEDED: 12, AUTO_MATCHED: 2 },
        caveat: 'Training data is drawn almost entirely from the human review queue; this is a known selection bias.'
      })
    })

    render(<CalibrationPanel />)

    await screen.findByText('Training data is drawn almost entirely from the human review queue; this is a known selection bias.')
  })

  it('shows Brier and ECE movement after a successful fit', async () => {
    global.fetch = vi.fn().mockImplementation((url) => {
      if (url === '/api/calibration/status') {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: async () => ({
            calibration_enabled: false,
            has_active_model: false,
            active_model: null,
            observation_count: 20,
            positive_count: 9,
            negative_count: 5,
            by_previous_status: { REVIEW_NEEDED: 12, AUTO_MATCHED: 2 },
            caveat: 'Training data is drawn almost entirely from the human review queue; this is a known selection bias.'
          }),
          text: async () => JSON.stringify({
            calibration_enabled: false,
            has_active_model: false,
            active_model: null,
            observation_count: 20,
            positive_count: 9,
            negative_count: 5,
            by_previous_status: { REVIEW_NEEDED: 12, AUTO_MATCHED: 2 },
            caveat: 'Training data is drawn almost entirely from the human review queue; this is a known selection bias.'
          })
        })
      } else if (url === '/api/calibration/fit') {
        return Promise.resolve({
          ok: true,
          status: 200,
          text: async () => JSON.stringify({
            status: "fitted",
            model_id: "cal-123",
            batch_id: "",
            observation_count: 40,
            positive_count: 25,
            negative_count: 15,
            train_count: 32,
            holdout_count: 8,
            brier_score_before: 0.2412,
            brier_score_after: 0.1188,
            ece_score_before: 0.1955,
            ece_score_after: 0.0642,
            by_previous_status: {},
            caveat: "..."
          })
        })
      }
      return Promise.reject(new Error('Unexpected fetch'))
    })

    render(<CalibrationPanel />)

    const button = await screen.findByRole('button', { name: /Fit Calibrator/i })
    await userEvent.click(button)

    await screen.findByText('0.2412')
    await screen.findByText('0.1188')
    await screen.findByText('0.1955')
    await screen.findByText('0.0642')
  })

  it('surfaces a plain-text fit failure', async () => {
    global.fetch = vi.fn().mockImplementation((url) => {
      if (url === '/api/calibration/status') {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: async () => ({
            calibration_enabled: false,
            has_active_model: false,
            active_model: null,
            observation_count: 20,
            positive_count: 9,
            negative_count: 5,
            by_previous_status: { REVIEW_NEEDED: 12, AUTO_MATCHED: 2 },
            caveat: 'Training data is drawn almost entirely from the human review queue; this is a known selection bias.'
          }),
          text: async () => JSON.stringify({
            calibration_enabled: false,
            has_active_model: false,
            active_model: null,
            observation_count: 20,
            positive_count: 9,
            negative_count: 5,
            by_previous_status: { REVIEW_NEEDED: 12, AUTO_MATCHED: 2 },
            caveat: 'Training data is drawn almost entirely from the human review queue; this is a known selection bias.'
          })
        })
      } else if (url === '/api/calibration/fit') {
        return Promise.resolve({
          ok: false,
          status: 400,
          text: async () => 'insufficient calibration observations: have 14, need at least 20; reviewer decisions are the only source of labels, so review more pairs before fitting'
        })
      }
      return Promise.reject(new Error('Unexpected fetch'))
    })

    render(<CalibrationPanel />)

    const button = await screen.findByRole('button', { name: /Fit Calibrator/i })
    await userEvent.click(button)

    await screen.findByText('insufficient calibration observations: have 14, need at least 20; reviewer decisions are the only source of labels, so review more pairs before fitting')
  })
})
