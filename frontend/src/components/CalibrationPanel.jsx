import React, { useState, useEffect } from 'react'
import { apiFetch } from '../lib/api.js'
import { Gauge, AlertTriangle, ArrowDown, ArrowUp } from 'lucide-react'

const MIN_OBSERVATIONS = 20

export function CalibrationPanel() {
  const [status, setStatus] = useState(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [fitting, setFitting] = useState(false)
  const [fitResult, setFitResult] = useState(null)

  useEffect(() => {
    fetchCalibrationStatus()
  }, [])

  const fetchCalibrationStatus = async () => {
    setLoading(true)
    try {
      const res = await apiFetch('/api/calibration/status')
      if (res.ok) {
        const data = await res.json()
        setStatus(data)
        setError('')
      } else {
        setError('Failed to load calibration status')
      }
    } catch (e) {
      setError(e.message || 'Failed to load calibration status')
    } finally {
      setLoading(false)
    }
  }

  const handleFitCalibrator = async () => {
    if (status.observation_count < MIN_OBSERVATIONS) return

    setFitting(true)
    try {
      const res = await apiFetch('/api/calibration/fit', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ batch_id: '' }),
      })

      const raw = await res.text()
      if (!res.ok) {
        setError(raw.trim())
        return
      }

      const result = JSON.parse(raw)
      setFitResult(result)
      setError('')
      await fetchCalibrationStatus()
    } catch (e) {
      setError(e.message || 'Failed to fit calibrator')
    } finally {
      setFitting(false)
    }
  }

  if (loading) {
    return (
      <div className="bg-slate-950/80 p-6 rounded-xl border border-slate-800 space-y-4">
        <h3 className="text-sm font-semibold text-slate-200">Score Calibration</h3>
        <p className="text-xs text-slate-400">Loading...</p>
      </div>
    )
  }

  if (!status) {
    return (
      <div className="bg-slate-950/80 p-6 rounded-xl border border-slate-800 space-y-4">
        <h3 className="text-sm font-semibold text-slate-200">Score Calibration</h3>
        <p className="text-xs text-slate-400">Failed to load calibration status</p>
      </div>
    )
  }

  const formatTimestamp = (ts) => {
    if (!ts || ts === '0001-01-01T00:00:00Z') {
      return 'Never'
    }
    try {
      const date = new Date(ts)
      return date.toLocaleString()
    } catch {
      return 'Never'
    }
  }

  return (
    <div className="bg-slate-950/80 p-6 rounded-xl border border-slate-800 space-y-5">
      <div className="flex items-center justify-between border-b border-slate-900 pb-3">
        <div className="flex items-center gap-2">
          <Gauge className="w-4 h-4 text-sky-400" />
          <h3 className="text-sm font-semibold text-slate-200">Score Calibration</h3>
        </div>
      </div>

      <p className="text-xs text-slate-400">Reviewer decisions are the only source of labels for calibration.</p>

      <div className="space-y-2">
        <div className="flex justify-between items-center">
          <span className="text-xs font-medium text-slate-300">{status.observation_count} / {MIN_OBSERVATIONS}</span>
        </div>
        <div className="h-2 bg-slate-800 rounded-full overflow-hidden">
          <div 
            className="h-full bg-sky-500 rounded-full"
            style={{ width: `${Math.min(100, (status.observation_count / MIN_OBSERVATIONS) * 100)}%` }}
          />
        </div>
        <p className="text-xs text-slate-400">
          {status.observation_count < MIN_OBSERVATIONS 
            ? `${MIN_OBSERVATIONS - status.observation_count} more reviewer decisions needed before a calibrator can be fitted.`
            : 'Enough observations to fit a calibrator.'}
        </p>
      </div>

      <div className="space-y-2">
        <div className="flex gap-4 text-xs">
          <span className="text-slate-300">{status.positive_count} positive</span>
          <span className="text-slate-300">{status.negative_count} negative</span>
        </div>
        <div className="space-y-1">
          {Object.entries(status.by_previous_status || {}).map(([key, count]) => (
            <div key={key} className="flex justify-between text-xs text-slate-400">
              <span>{key}</span>
              <span>{count}</span>
            </div>
          ))}
        </div>
      </div>

      {status.calibration_enabled === false && (
        <div className="p-3 bg-amber-950/80 border border-amber-700/50 text-amber-300 rounded-lg text-xs font-medium flex items-start gap-2">
          <AlertTriangle className="w-4 h-4 shrink-0 mt-0.5" />
          Fitting stores a model but it will not affect scoring until calibration is enabled in the engine configuration.
        </div>
      )}

      {status.has_active_model && status.active_model && (
        <div className="space-y-2">
          <h4 className="text-xs font-semibold text-slate-300">Active Model</h4>
          <div className="text-xs space-y-1">
            <p><span className="text-slate-500">ID:</span> {status.active_model.id}</p>
            <p><span className="text-slate-500">Fitted at:</span> {formatTimestamp(status.active_model.created_at)}</p>
            {status.active_model.observation_count !== undefined && (
              <p><span className="text-slate-500">Observations:</span> {status.active_model.observation_count}</p>
            )}
          </div>
        </div>
      )}

      {status.caveat && (
        <p className="italic text-slate-500 text-xs">{status.caveat}</p>
      )}

      <button
        onClick={handleFitCalibrator}
        disabled={status.observation_count < MIN_OBSERVATIONS || fitting}
        className="px-4 py-2 bg-sky-600 hover:bg-sky-500 text-white rounded-lg text-xs font-semibold transition disabled:opacity-50"
      >
        {fitting ? 'Fitting…' : 'Fit Calibrator'}
      </button>

      {fitResult && (
        <div className="space-y-3 bg-slate-900/50 p-4 rounded-lg border border-slate-800">
          <h4 className="text-xs font-semibold text-slate-300">Fit Result</h4>
          <div className="grid grid-cols-2 gap-3 text-xs">
            <div className="space-y-1">
              <p className="text-slate-500">Brier Score</p>
              <div className="flex items-center gap-1">
                <span>{fitResult.brier_score_before.toFixed(4)}</span>
                <span>→</span>
                <span className={fitResult.brier_score_after < fitResult.brier_score_before ? 'text-emerald-400' : 'text-rose-400'}>
                  {fitResult.brier_score_after.toFixed(4)}
                </span>
                {fitResult.brier_score_after < fitResult.brier_score_before && (
                  <ArrowDown className="w-3 h-3 text-emerald-400" />
                )}
                {fitResult.brier_score_after >= fitResult.brier_score_before && (
                  <ArrowUp className="w-3 h-3 text-rose-400" />
                )}
              </div>
            </div>
            <div className="space-y-1">
              <p className="text-slate-500">ECE Score</p>
              <div className="flex items-center gap-1">
                <span>{fitResult.ece_score_before.toFixed(4)}</span>
                <span>→</span>
                <span className={fitResult.ece_score_after < fitResult.ece_score_before ? 'text-emerald-400' : 'text-rose-400'}>
                  {fitResult.ece_score_after.toFixed(4)}
                </span>
                {fitResult.ece_score_after < fitResult.ece_score_before && (
                  <ArrowDown className="w-3 h-3 text-emerald-400" />
                )}
                {fitResult.ece_score_after >= fitResult.ece_score_before && (
                  <ArrowUp className="w-3 h-3 text-rose-400" />
                )}
              </div>
            </div>
            <div>
              <p className="text-slate-500">Train Count</p>
              <p>{fitResult.train_count}</p>
            </div>
            <div>
              <p className="text-slate-500">Holdout Count</p>
              <p>{fitResult.holdout_count}</p>
            </div>
          </div>
          <p className="text-[11px] text-slate-500 italic">Lower is better for both metrics</p>
        </div>
      )}

      {error && (
        <div className="p-3 bg-rose-950/80 border border-rose-700/50 text-rose-300 rounded-lg text-xs font-medium flex items-start gap-2">
          <AlertTriangle className="w-4 h-4 shrink-0 mt-0.5" />
          {error}
        </div>
      )}
    </div>
  )
}
