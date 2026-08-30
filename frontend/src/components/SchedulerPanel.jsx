import React, { useState, useEffect } from 'react'
import { apiFetch } from '../lib/api.js'
import { Clock, Save, RotateCcw, AlertCircle, CheckCircle } from 'lucide-react'

const presetCrons = [
  { label: 'Every 15 min', value: '*/15 * * * *' },
  { label: 'Hourly', value: '0 * * * *' },
  { label: 'Nightly 2 AM', value: '0 2 * * *' },
]

export function SchedulerPanel() {
  const [config, setConfig] = useState(null)
  const [localConfig, setLocalConfig] = useState(null)
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState(false)
  const [cronError, setCronError] = useState('')

  useEffect(() => {
    fetchSchedulerConfig()
  }, [])

  const fetchSchedulerConfig = async () => {
    setLoading(true)
    try {
      const res = await apiFetch('/api/scheduler/config')
      if (res.ok) {
        const data = await res.json()
        setConfig(data)
        setLocalConfig(data)
        setError('')
      } else {
        setError('Failed to load scheduler config')
      }
    } catch (e) {
      setError(e.message || 'Failed to load scheduler config')
    } finally {
      setLoading(false)
    }
  }

  const validateCronExpression = (expr) => {
    if (!expr) return true
    const parts = expr.trim().split(/\s+/)
    return parts.length === 5 || parts.length === 6
  }

  const handleCronChange = (value) => {
    setLocalConfig({ ...localConfig, cron_expression: value })
    if (validateCronExpression(value)) {
      setCronError('')
    } else {
      setCronError('Cron expression must have 5 or 6 space-separated fields')
    }
  }

  const handleSave = async () => {
    if (!validateCronExpression(localConfig.cron_expression)) {
      setCronError('Invalid cron expression format')
      return
    }

    setSaving(true)
    try {
      const res = await apiFetch('/api/scheduler/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(localConfig),
      })

      if (res.ok) {
        const updated = await res.json()
        setConfig(updated)
        setLocalConfig(updated)
        setSuccess(true)
        setError('')
        setCronError('')
        setTimeout(() => setSuccess(false), 3000)
      } else {
        const errorData = await res.text()
        setError(errorData || 'Failed to save scheduler config')
        setCronError('')
      }
    } catch (e) {
      setError(e.message || 'Failed to save scheduler config')
      setCronError('')
    } finally {
      setSaving(false)
    }
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

  if (loading) {
    return (
      <div className="bg-slate-950/80 p-6 rounded-xl border border-slate-800 space-y-4">
        <h3 className="text-sm font-semibold text-slate-200">Scheduler Configuration</h3>
        <p className="text-xs text-slate-400">Loading...</p>
      </div>
    )
  }

  if (!localConfig) {
    return (
      <div className="bg-slate-950/80 p-6 rounded-xl border border-slate-800 space-y-4">
        <h3 className="text-sm font-semibold text-slate-200">Scheduler Configuration</h3>
        <p className="text-xs text-slate-400">Failed to load scheduler config</p>
      </div>
    )
  }

  return (
    <div className="bg-slate-950/80 p-6 rounded-xl border border-slate-800 space-y-5">
      <div className="flex items-center justify-between border-b border-slate-900 pb-3">
        <div className="flex items-center gap-2">
          <Clock className="w-4 h-4 text-sky-400" />
          <h3 className="text-sm font-semibold text-slate-200">Scheduler Configuration</h3>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => setLocalConfig(config)}
            disabled={saving}
            className="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg text-xs font-medium transition disabled:opacity-50"
          >
            <RotateCcw className="w-3.5 h-3.5" />
          </button>
          <button
            onClick={handleSave}
            disabled={saving || !localConfig}
            className="px-4 py-1.5 bg-sky-600 hover:bg-sky-500 text-white rounded-lg text-xs font-semibold transition disabled:opacity-50 flex items-center gap-1.5"
          >
            <Save className="w-3.5 h-3.5" />
            {saving ? 'Saving...' : 'Save'}
          </button>
        </div>
      </div>

      {error && (
        <div className="p-3 bg-rose-950/80 border border-rose-700/50 text-rose-300 rounded-lg text-xs font-medium flex items-start gap-2">
          <AlertCircle className="w-4 h-4 shrink-0 mt-0.5" />
          {error}
        </div>
      )}

      {success && (
        <div className="p-3 bg-emerald-950/80 border border-emerald-700/50 text-emerald-300 rounded-lg text-xs font-medium flex items-start gap-2">
          <CheckCircle className="w-4 h-4 shrink-0 mt-0.5" />
          Configuration saved successfully!
        </div>
      )}

      {/* Enable Toggle */}
      <div className="space-y-2">
        <label className="text-xs font-medium text-slate-300">Enable Scheduler</label>
        <label className="flex items-center gap-3 cursor-pointer">
          <input
            type="checkbox"
            checked={localConfig.enabled || false}
            onChange={(e) => setLocalConfig({ ...localConfig, enabled: e.target.checked })}
            disabled={saving}
            className="w-4 h-4 rounded border-slate-700 accent-sky-600 cursor-pointer"
          />
          <span className="text-xs text-slate-400">Automatic matching runs enabled</span>
        </label>
      </div>

      {/* Cron Expression */}
      <div className="space-y-2">
        <label className="text-xs font-medium text-slate-300">Cron Expression</label>
        <input
          type="text"
          value={localConfig.cron_expression || ''}
          onChange={(e) => handleCronChange(e.target.value)}
          placeholder="0 2 * * *"
          disabled={saving || !localConfig.enabled}
          className={`w-full bg-slate-900 border rounded-lg px-3 py-2 text-xs font-mono text-slate-200 placeholder-slate-500 focus:outline-none focus:ring-1 disabled:opacity-50 ${
            cronError ? 'border-rose-700/50 focus:ring-rose-500' : 'border-slate-800 focus:border-sky-500 focus:ring-sky-500'
          }`}
        />
        <p className="text-[11px] text-slate-500">Standard cron format with 5 or 6 fields (minute hour day month weekday [second])</p>
        {cronError && <p className="text-[11px] text-rose-400">{cronError}</p>}
      </div>

      {/* Preset Crons */}
      <div className="space-y-2">
        <label className="text-xs font-medium text-slate-300">Preset Schedules</label>
        <div className="flex flex-wrap gap-2">
          {presetCrons.map((preset) => (
            <button
              key={preset.value}
              type="button"
              onClick={() => handleCronChange(preset.value)}
              disabled={saving || !localConfig.enabled}
              className="px-3 py-1.5 bg-slate-900 hover:bg-slate-800 text-slate-300 border border-slate-700 rounded-lg text-xs font-medium transition disabled:opacity-50"
            >
              {preset.label}
            </button>
          ))}
        </div>
      </div>

      {/* Webhook URL */}
      <div className="space-y-2">
        <label className="text-xs font-medium text-slate-300">Webhook URL (optional)</label>
        <input
          type="text"
          value={localConfig.webhook_url || ''}
          onChange={(e) => setLocalConfig({ ...localConfig, webhook_url: e.target.value })}
          placeholder="https://your-webhook-endpoint.com/notify"
          disabled={saving || !localConfig.enabled}
          className="w-full bg-slate-900 border border-slate-800 rounded-lg px-3 py-2 text-xs text-slate-200 placeholder-slate-500 focus:outline-none focus:border-sky-500 focus:ring-1 focus:ring-sky-500 disabled:opacity-50"
        />
        <p className="text-[11px] text-slate-500">Optional endpoint to receive webhook notifications</p>
      </div>

      {/* Notifications */}
      <div className="space-y-3 bg-slate-900/50 p-4 rounded-lg border border-slate-800">
        <h4 className="text-xs font-semibold text-slate-300">Notifications</h4>
        <label className="flex items-center gap-3 cursor-pointer">
          <input
            type="checkbox"
            checked={localConfig.notify_on_success || false}
            onChange={(e) => setLocalConfig({ ...localConfig, notify_on_success: e.target.checked })}
            disabled={saving || !localConfig.enabled}
            className="w-4 h-4 rounded border-slate-700 accent-emerald-600 cursor-pointer"
          />
          <span className="text-xs text-slate-300">Notify on successful completion</span>
        </label>
        <label className="flex items-center gap-3 cursor-pointer">
          <input
            type="checkbox"
            checked={localConfig.notify_on_anomaly || false}
            onChange={(e) => setLocalConfig({ ...localConfig, notify_on_anomaly: e.target.checked })}
            disabled={saving || !localConfig.enabled}
            className="w-4 h-4 rounded border-slate-700 accent-amber-600 cursor-pointer"
          />
          <span className="text-xs text-slate-300">Notify on anomalies detected</span>
        </label>
      </div>

      {/* Last Run Info */}
      {config && (
        <div className="bg-slate-900/50 p-4 rounded-lg border border-slate-800 space-y-2">
          <h4 className="text-xs font-semibold text-slate-300">Last Run</h4>
          <div className="text-xs text-slate-400 space-y-1">
            <p>
              <span className="text-slate-500">Timestamp:</span> {formatTimestamp(config.last_run_timestamp)}
            </p>
            <p>
              <span className="text-slate-500">Status:</span>{' '}
              <span className={config.last_run_status === 'SUCCESS' ? 'text-emerald-400' : 'text-rose-400'}>
                {config.last_run_status || 'No runs yet'}
              </span>
            </p>
          </div>
        </div>
      )}
    </div>
  )
}
