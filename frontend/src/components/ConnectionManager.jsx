import React, { useState, useEffect } from 'react'
import { Database, Server, FileSpreadsheet, FileCode, CheckCircle2, AlertCircle, RefreshCcw, Layers, Edit3, Plus, Trash2, Plug, Save } from 'lucide-react'
import { apiFetch, readErrorMessage } from '../lib/api.js'
import { useMatcherStore } from '../store/useMatcherStore'

export function ConnectionManager({ onSchemaIntrospected, onSettingsChange }) {
  const DEFAULT_PORTS = { POSTGRES: 5432, SQLSERVER: 1433, MONGODB: 27017 }

  const [srcType, setSrcType] = useState('SQLSERVER')
  const [srcConfig, setSrcConfig] = useState({
    host: '10.0.0.12',
    port: 1433,
    database: 'EnterpriseDB',
    username: 'sa',
    password: '',
    table_or_query: 'dbo.Customers',
  })

  const [destType, setDestType] = useState('MONGODB')
  const [destConfig, setDestConfig] = useState({
    host: 'mongodb.internal',
    port: 27017,
    database: 'CRM_DB',
    username: 'admin',
    password: '',
    table_or_query: 'clients_collection',
  })

  const [srcCols, setSrcCols] = useState(['CustID', 'CustomerName', 'TaxRegistrationNo', 'TxDate', 'BranchNo'])
  const [destCols, setDestCols] = useState(['_id', 'client_name', 'registration_id', 'created_at', 'contact_person'])
  const [srcFile, setSrcFile] = useState(null)
  const [destFile, setDestFile] = useState(null)

  const [srcStatus, setSrcStatus] = useState(null)
  const [destStatus, setDestStatus] = useState(null)
  const [loadingSrc, setLoadingSrc] = useState(false)
  const [loadingDest, setLoadingDest] = useState(false)

  // Manual Column & Row Editor State
  const [manualSrcCols, setManualSrcCols] = useState(['id', 'customer_name', 'tx_date', 'tax_id'])
  const [newColName, setNewColName] = useState('')

  const ingestFromConnectors = useMatcherStore((s) => s.ingestFromConnectors)
  const fetchConnectorSettings = useMatcherStore((s) => s.fetchConnectorSettings)
  const saveConnectorSettings = useMatcherStore((s) => s.saveConnectorSettings)
  const [ingesting, setIngesting] = useState(false)
  const [ingestStatus, setIngestStatus] = useState(null)
  const [savingSettings, setSavingSettings] = useState(false)
  const [settingsStatus, setSettingsStatus] = useState(null)

  const FILE_TYPES = ['CSV', 'EXCEL']

  // password is deliberately omitted: the server has no field for it.
  const toEndpoint = (type, cfg, cols) => ({
    type,
    host: cfg.host || '',
    port: Number(cfg.port) || 0,
    database: cfg.database || '',
    username: cfg.username || '',
    table_or_query: cfg.table_or_query || '',
    file_path: cfg.file_path || '',
    columns: cols,
  })

  // Connector settings are stored server-side so the panel survives a reload.
  // A side with no stored type means nothing was ever saved: leave the
  // built-in demo defaults alone rather than blanking the form.
  useEffect(() => {
    let cancelled = false
    fetchConnectorSettings()
      .then((settings) => {
        if (cancelled || !settings) return
        const apply = (side, setType, setConfig, setCols) => {
          if (!side || !side.type) return
          setType(side.type)
          setConfig({
            host: side.host || '',
            port: side.port || 0,
            database: side.database || '',
            username: side.username || '',
            password: '', // never persisted; re-entered each session
            table_or_query: side.table_or_query || '',
            file_path: side.file_path || '',
          })
          if (Array.isArray(side.columns) && side.columns.length > 0) setCols(side.columns)
        }
        apply(settings.source, setSrcType, setSrcConfig, setSrcCols)
        apply(settings.destination, setDestType, setDestConfig, setDestCols)
      })
      .catch(() => {
        // A failed load is not worth blocking the panel: the demo defaults stand.
      })
    return () => {
      cancelled = true
    }
  }, [])

  // Publish the column lists upward on EVERY change, including the mount-time
  // hydration from saved settings. Previously this fired only on an Introspect
  // click, so after a remount FieldMapper saw empty lists and fell back to its
  // hardcoded demo columns, making a saved pairing look unselected.
  useEffect(() => {
    if (onSchemaIntrospected) onSchemaIntrospected(srcCols, destCols)
  }, [srcCols, destCols])

  // Publish the full connector payload so ConfigPanel's "Save Configuration"
  // can persist it in the same click.
  useEffect(() => {
    if (onSettingsChange) {
      onSettingsChange({
        source: toEndpoint(srcType, srcConfig, srcCols),
        destination: toEndpoint(destType, destConfig, destCols),
      })
    }
  }, [srcType, srcConfig, srcCols, destType, destConfig, destCols])

  const handleTestOnly = async (isSource) => {
    const type = isSource ? srcType : destType
    const cfg = isSource ? srcConfig : destConfig

    if (isSource) setLoadingSrc(true)
    else setLoadingDest(true)

    try {
      const res = await apiFetch('/api/connector/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ type, ...cfg }),
      })
      if (!res.ok) {
        throw new Error(await readErrorMessage(res, 'Connection test failed'))
      }
      const data = await res.json()
      const msg = data.message || `Successfully connected to ${type} database!`
      if (isSource) setSrcStatus({ success: true, message: msg })
      else setDestStatus({ success: true, message: msg })
    } catch (e) {
      if (isSource) setSrcStatus({ success: false, message: e.message })
      else setDestStatus({ success: false, message: e.message })
    } finally {
      if (isSource) setLoadingSrc(false)
      else setLoadingDest(false)
    }
  }

  const handleLoadData = async () => {
    if (FILE_TYPES.includes(srcType) || FILE_TYPES.includes(destType)) {
      setIngestStatus({ success: false, message: 'File sources are loaded from the Data Ingestion tab, which uploads the file itself. This panel loads from database connectors only.' })
      return
    }

    setIngesting(true)
    setIngestStatus(null)

    try {
      const data = await ingestFromConnectors({
        source: { type: srcType, ...srcConfig },
        destination: { type: destType, ...destConfig },
      })

      let message = `Loaded ${data.source_count} source and ${data.destination_count} destination records into batch ${data.batch_id}.`
      if (data.truncated) {
        message += ' ' + (data.warning || 'Some records were truncated.')
      }
      setIngestStatus({ success: !data.truncated, message })
    } catch (e) {
      setIngestStatus({ success: false, message: e.message })
    } finally {
      setIngesting(false)
    }
  }

  const handleTestAndIntrospect = async (isSource) => {
    const type = isSource ? srcType : destType
    const cfg = isSource ? srcConfig : destConfig
    const file = isSource ? srcFile : destFile

    if (isSource) setLoadingSrc(true)
    else setLoadingDest(true)

    try {
      let res
      if (FILE_TYPES.includes(type) && file) {
        // Send the browsed file itself: the browser only ever knows the file's
        // name, never a path the server could open.
        const form = new FormData()
        form.append('file', file)
        if (type === 'EXCEL' && cfg.sheet) form.append('sheet', cfg.sheet)
        res = await apiFetch('/api/connector/introspect/upload', { method: 'POST', body: form })
      } else {
        res = await apiFetch('/api/connector/introspect', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ type, ...cfg }),
        })
      }

      if (!res.ok) {
        throw new Error(await readErrorMessage(res, 'Introspection failed'))
      }

      const data = await res.json()
      const extractedCols = (data.columns || []).map((c) => c.name)
      const origin = file ? file.name : type
      if (isSource) {
        setSrcCols(extractedCols)
        setSrcStatus({ success: true, message: `Introspected ${extractedCols.length} columns from ${origin}.` })
      } else {
        setDestCols(extractedCols)
        setDestStatus({ success: true, message: `Introspected ${extractedCols.length} columns from ${origin}.` })
      }
    } catch (e) {
      if (isSource) setSrcStatus({ success: false, message: e.message })
      else setDestStatus({ success: false, message: e.message })
    } finally {
      if (isSource) setLoadingSrc(false)
      else setLoadingDest(false)
    }
  }

  const addManualColumn = (isSource) => {
    if (!newColName.trim()) return
    if (isSource) {
      setSrcCols((prev) => [...prev, newColName.trim()])
    } else {
      setDestCols((prev) => [...prev, newColName.trim()])
    }
    setNewColName('')
  }

  const removeColumn = (isSource, colName) => {
    if (isSource) {
      setSrcCols((prev) => prev.filter((c) => c !== colName))
    } else {
      setDestCols((prev) => prev.filter((c) => c !== colName))
    }
  }

  const handleSaveSettings = async () => {
    setSavingSettings(true)
    setSettingsStatus(null)
    try {
      await saveConnectorSettings({
        source: toEndpoint(srcType, srcConfig, srcCols),
        destination: toEndpoint(destType, destConfig, destCols),
      })
      setSettingsStatus({ success: true, message: 'Connection settings saved. Passwords are not stored and must be re-entered each session.' })
    } catch (e) {
      setSettingsStatus({ success: false, message: e.message })
    } finally {
      setSavingSettings(false)
    }
  }

  return (
    <div className="space-y-6 bg-slate-900/60 p-6 rounded-2xl border border-slate-800">
      <div className="border-b border-slate-800 pb-3 flex flex-wrap items-center justify-between gap-4">
        <div>
          <h2 className="text-base font-bold text-slate-100 flex items-center gap-2">
            <Server className="w-5 h-5 text-sky-400" /> Heterogeneous Data Source Connectors
          </h2>
          <p className="text-xs text-slate-400 mt-0.5">Configure SQL Server, Postgres, MongoDB, Excel, CSV, or Manual entry for Source and Destination.</p>
        </div>

        {/* Visual Pair Connection Badge */}
        <div className="flex items-center gap-2 text-xs font-mono bg-slate-950 px-3 py-1.5 rounded-lg border border-slate-800">
          <span className="text-sky-400 font-bold">{srcType} ({srcConfig.table_or_query || 'Source'})</span>
          <span className="text-slate-500">➔</span>
          <span className="text-purple-400 font-bold">{destType} ({destConfig.table_or_query || 'Dest'})</span>
        </div>
      </div>

      {/* Grid: Source vs Destination Connector Form */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {/* Source Connector Card */}
        <div className="bg-slate-950/80 p-5 rounded-xl border border-slate-800 space-y-4">
          <div className="flex justify-between items-center border-b border-slate-900 pb-2">
            <span className="text-xs font-bold text-sky-400 uppercase tracking-wider flex items-center gap-1.5">
              <Database className="w-4 h-4" /> Source Data Source
            </span>
            <select
              value={srcType}
              onChange={(e) => {
                const nextType = e.target.value
                setSrcType(nextType)
                if (DEFAULT_PORTS[nextType]) {
                  setSrcConfig((prev) => ({ ...prev, port: DEFAULT_PORTS[nextType] }))
                }
              }}
              className="bg-slate-900 border border-slate-800 text-slate-200 text-xs rounded font-semibold p-1.5"
            >
              <option value="SQLSERVER">SQL Server (MSSQL)</option>
              <option value="POSTGRES">PostgreSQL</option>
              <option value="MONGODB">MongoDB</option>
              <option value="EXCEL">Excel (.xlsx)</option>
              <option value="CSV">CSV File</option>
              <option value="MANUAL">Manual Entry</option>
            </select>
          </div>

          {/* Connection inputs for DB */}
          {srcType !== 'MANUAL' && srcType !== 'CSV' && srcType !== 'EXCEL' && (
            <div className="grid grid-cols-2 gap-2 text-xs">
              <input
                type="text"
                placeholder="Host (e.g. 10.0.0.12)"
                value={srcConfig.host}
                onChange={(e) => setSrcConfig({ ...srcConfig, host: e.target.value })}
                className="bg-slate-900 border border-slate-800 rounded p-2 text-slate-200"
              />
              <input
                type="number"
                placeholder="Port"
                value={srcConfig.port}
                onChange={(e) => setSrcConfig({ ...srcConfig, port: parseInt(e.target.value, 10) || 0 })}
                className="bg-slate-900 border border-slate-800 rounded p-2 text-slate-200"
              />
              <input
                type="text"
                placeholder="Database Name"
                value={srcConfig.database}
                onChange={(e) => setSrcConfig({ ...srcConfig, database: e.target.value })}
                className="bg-slate-900 border border-slate-800 rounded p-2 text-slate-200"
              />
              <input
                type="text"
                placeholder="Username"
                value={srcConfig.username}
                onChange={(e) => setSrcConfig({ ...srcConfig, username: e.target.value })}
                className="bg-slate-900 border border-slate-800 rounded p-2 text-slate-200"
              />
              <input
                type="password"
                placeholder="Password"
                value={srcConfig.password}
                onChange={(e) => setSrcConfig({ ...srcConfig, password: e.target.value })}
                className="bg-slate-900 border border-slate-800 rounded p-2 text-slate-200"
              />
              <input
                type="text"
                placeholder="Table / View / Query"
                value={srcConfig.table_or_query}
                onChange={(e) => setSrcConfig({ ...srcConfig, table_or_query: e.target.value })}
                className="col-span-2 bg-slate-900 border border-slate-800 rounded p-2 text-slate-200 font-mono"
              />
            </div>
          )}

          {/* File Upload for CSV / Excel */}
          {(srcType === 'CSV' || srcType === 'EXCEL') && (
            <div className="space-y-2 text-xs">
              <label className="block text-slate-400 font-medium">Select or Drop CSV / Excel File:</label>
              <input
                type="file"
                accept={srcType === 'CSV' ? '.csv' : '.xlsx,.xls'}
                onChange={(e) => {
                  const file = e.target.files[0] || null
                  setSrcFile(file)
                  setSrcStatus(
                    file
                      ? { success: true, message: `Selected ${file.name}. Click Introspect Source Columns to read its headers.` }
                      : null
                  )
                }}
                className="w-full bg-slate-900 border border-slate-800 rounded p-2 text-slate-300 font-mono text-xs file:mr-3 file:py-1 file:px-3 file:rounded file:border-0 file:text-xs file:bg-sky-950 file:text-sky-300"
              />
              <input
                type="text"
                placeholder="Or Server File Path (e.g. /var/data/source.csv) (needs CONNECTOR_FILE_ROOT set on the server)"
                value={srcConfig.file_path || ''}
                onChange={(e) => {
                  setSrcConfig({ ...srcConfig, file_path: e.target.value })
                  setSrcFile(null)
                }}
                className="w-full bg-slate-900 border border-slate-800 rounded p-2 text-slate-200 font-mono"
              />
            </div>
          )}

          <div className="flex items-center gap-2 pt-1">
            <button
              onClick={() => handleTestOnly(true)}
              disabled={loadingSrc}
              className="px-3.5 py-2 bg-slate-800 hover:bg-slate-700 text-sky-400 border border-slate-700 rounded-lg text-xs font-semibold flex items-center gap-1.5 transition"
            >
              <Plug className="w-3.5 h-3.5" /> Test Connection
            </button>
            <button
              onClick={() => handleTestAndIntrospect(true)}
              disabled={loadingSrc}
              className="px-3.5 py-2 bg-sky-600 hover:bg-sky-500 text-white rounded-lg text-xs font-semibold flex items-center gap-1.5 transition"
            >
              <RefreshCcw className={`w-3.5 h-3.5 ${loadingSrc ? 'animate-spin' : ''}`} /> Introspect Source Columns
            </button>
          </div>

          {srcStatus && (
            <div className={`p-2.5 rounded text-xs flex items-center gap-2 ${srcStatus.success ? 'bg-emerald-950/60 text-emerald-300 border border-emerald-800' : 'bg-rose-950/60 text-rose-300 border border-rose-800'}`}>
              {srcStatus.success ? <CheckCircle2 className="w-4 h-4 shrink-0" /> : <AlertCircle className="w-4 h-4 shrink-0" />}
              <span>{srcStatus.message}</span>
            </div>
          )}

          {/* Introspected / Manual Column Chips */}
          <div className="space-y-2 pt-2 border-t border-slate-900">
            <span className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider block">Introspected Columns ({srcCols.length})</span>
            <div className="flex flex-wrap gap-1.5">
              {srcCols.map((col) => (
                <span key={col} className="px-2.5 py-1 bg-slate-900 text-sky-300 rounded border border-slate-800 text-xs font-mono flex items-center gap-1">
                  {col}
                  <button onClick={() => removeColumn(true, col)} className="hover:text-rose-400 text-slate-500">×</button>
                </span>
              ))}
            </div>
          </div>
        </div>

        {/* Destination Connector Card */}
        <div className="bg-slate-950/80 p-5 rounded-xl border border-slate-800 space-y-4">
          <div className="flex justify-between items-center border-b border-slate-900 pb-2">
            <span className="text-xs font-bold text-purple-400 uppercase tracking-wider flex items-center gap-1.5">
              <Database className="w-4 h-4" /> Destination Data Source
            </span>
            <select
              value={destType}
              onChange={(e) => {
                const nextType = e.target.value
                setDestType(nextType)
                if (DEFAULT_PORTS[nextType]) {
                  setDestConfig((prev) => ({ ...prev, port: DEFAULT_PORTS[nextType] }))
                }
              }}
              className="bg-slate-900 border border-slate-800 text-slate-200 text-xs rounded font-semibold p-1.5"
            >
              <option value="MONGODB">MongoDB</option>
              <option value="POSTGRES">PostgreSQL</option>
              <option value="SQLSERVER">SQL Server (MSSQL)</option>
              <option value="EXCEL">Excel (.xlsx)</option>
              <option value="CSV">CSV File</option>
              <option value="MANUAL">Manual Entry</option>
            </select>
          </div>

          {/* Connection inputs for DB */}
          {destType !== 'MANUAL' && destType !== 'CSV' && destType !== 'EXCEL' && (
            <div className="grid grid-cols-2 gap-2 text-xs">
              <input
                type="text"
                placeholder="Host (e.g. mongodb.internal)"
                value={destConfig.host}
                onChange={(e) => setDestConfig({ ...destConfig, host: e.target.value })}
                className="bg-slate-900 border border-slate-800 rounded p-2 text-slate-200"
              />
              <input
                type="number"
                placeholder="Port"
                value={destConfig.port}
                onChange={(e) => setDestConfig({ ...destConfig, port: parseInt(e.target.value, 10) || 0 })}
                className="bg-slate-900 border border-slate-800 rounded p-2 text-slate-200"
              />
              <input
                type="text"
                placeholder="Database Name"
                value={destConfig.database}
                onChange={(e) => setDestConfig({ ...destConfig, database: e.target.value })}
                className="bg-slate-900 border border-slate-800 rounded p-2 text-slate-200"
              />
              <input
                type="text"
                placeholder="Username"
                value={destConfig.username}
                onChange={(e) => setDestConfig({ ...destConfig, username: e.target.value })}
                className="bg-slate-900 border border-slate-800 rounded p-2 text-slate-200"
              />
              <input
                type="password"
                placeholder="Password"
                value={destConfig.password}
                onChange={(e) => setDestConfig({ ...destConfig, password: e.target.value })}
                className="bg-slate-900 border border-slate-800 rounded p-2 text-slate-200"
              />
              <input
                type="text"
                placeholder="Collection / Table Name"
                value={destConfig.table_or_query}
                onChange={(e) => setDestConfig({ ...destConfig, table_or_query: e.target.value })}
                className="col-span-2 bg-slate-900 border border-slate-800 rounded p-2 text-slate-200 font-mono"
              />
            </div>
          )}

          {/* File Upload for CSV / Excel */}
          {(destType === 'CSV' || destType === 'EXCEL') && (
            <div className="space-y-2 text-xs">
              <label className="block text-slate-400 font-medium">Select or Drop CSV / Excel File:</label>
              <input
                type="file"
                accept={destType === 'CSV' ? '.csv' : '.xlsx,.xls'}
                onChange={(e) => {
                  const file = e.target.files[0] || null
                  setDestFile(file)
                  setDestStatus(
                    file
                      ? { success: true, message: `Selected ${file.name}. Click Introspect Destination Columns to read its headers.` }
                      : null
                  )
                }}
                className="w-full bg-slate-900 border border-slate-800 rounded p-2 text-slate-300 font-mono text-xs file:mr-3 file:py-1 file:px-3 file:rounded file:border-0 file:text-xs file:bg-purple-950 file:text-purple-300"
              />
              <input
                type="text"
                placeholder="Or Server File Path (e.g. /var/data/dest.csv) (needs CONNECTOR_FILE_ROOT set on the server)"
                value={destConfig.file_path || ''}
                onChange={(e) => {
                  setDestConfig({ ...destConfig, file_path: e.target.value })
                  setDestFile(null)
                }}
                className="w-full bg-slate-900 border border-slate-800 rounded p-2 text-slate-200 font-mono"
              />
            </div>
          )}

          <div className="flex items-center gap-2 pt-1">
            <button
              onClick={() => handleTestOnly(false)}
              disabled={loadingDest}
              className="px-3.5 py-2 bg-slate-800 hover:bg-slate-700 text-purple-400 border border-slate-700 rounded-lg text-xs font-semibold flex items-center gap-1.5 transition"
            >
              <Plug className="w-3.5 h-3.5" /> Test Connection
            </button>
            <button
              onClick={() => handleTestAndIntrospect(false)}
              disabled={loadingDest}
              className="px-3.5 py-2 bg-purple-600 hover:bg-purple-500 text-white rounded-lg text-xs font-semibold flex items-center gap-1.5 transition"
            >
              <RefreshCcw className={`w-3.5 h-3.5 ${loadingDest ? 'animate-spin' : ''}`} /> Introspect Destination Columns
            </button>
          </div>

          {destStatus && (
            <div className={`p-2.5 rounded text-xs flex items-center gap-2 ${destStatus.success ? 'bg-emerald-950/60 text-emerald-300 border border-emerald-800' : 'bg-rose-950/60 text-rose-300 border border-rose-800'}`}>
              {destStatus.success ? <CheckCircle2 className="w-4 h-4 shrink-0" /> : <AlertCircle className="w-4 h-4 shrink-0" />}
              <span>{destStatus.message}</span>
            </div>
          )}

          {/* Introspected / Manual Column Chips */}
          <div className="space-y-2 pt-2 border-t border-slate-900">
            <span className="text-[11px] font-semibold text-slate-400 uppercase tracking-wider block">Introspected Columns ({destCols.length})</span>
            <div className="flex flex-wrap gap-1.5">
              {destCols.map((col) => (
                <span key={col} className="px-2.5 py-1 bg-slate-900 text-purple-300 rounded border border-slate-800 text-xs font-mono flex items-center gap-1">
                  {col}
                  <button onClick={() => removeColumn(false, col)} className="hover:text-rose-400 text-slate-500">×</button>
                </span>
              ))}
            </div>
          </div>
        </div>
      </div>

      {/* Load Data Button Section */}
      <div className="p-4 bg-slate-950/60 rounded-xl border border-slate-800 space-y-3">
        <button
          onClick={handleLoadData}
          disabled={ingesting}
          className="px-4 py-2.5 bg-sky-600 hover:bg-sky-500 text-white rounded-lg text-xs font-semibold flex items-center gap-1.5 transition disabled:opacity-60"
        >
          <Layers className="w-3.5 h-3.5" /> {ingesting ? 'Loading…' : 'Load Data & Start Batch'}
        </button>
        <p className="text-xs text-slate-500">Reads both connectors to completion and writes a batch the matcher can run against.</p>
        {ingestStatus && (
          <div
            data-testid="ingest-status"
            data-outcome={ingestStatus.success ? 'success' : 'error'}
            className={`p-2.5 rounded text-xs flex items-center gap-2 ${ingestStatus.success ? 'bg-emerald-950/60 text-emerald-300 border border-emerald-800' : 'bg-rose-950/60 text-rose-300 border border-rose-800'}`}>
            {ingestStatus.success ? <CheckCircle2 className="w-4 h-4 shrink-0" /> : <AlertCircle className="w-4 h-4 shrink-0" />}
            <span>{ingestStatus.message}</span>
          </div>
        )}
      </div>

      {/* Persist Connector Settings */}
      <div className="flex items-center gap-3 pt-2 border-t border-slate-900">
        <button
          onClick={handleSaveSettings}
          disabled={savingSettings}
          className="px-4 py-2 bg-emerald-600 hover:bg-emerald-500 disabled:opacity-60 text-white rounded-lg text-xs font-semibold flex items-center gap-1.5 transition"
        >
          <Save className="w-3.5 h-3.5" /> {savingSettings ? 'Saving…' : 'Save Connection Settings'}
        </button>
        <span className="text-[11px] text-slate-500">Stored on the server. Passwords are never saved.</span>
      </div>

      {settingsStatus && (
        <div className={`p-2.5 rounded text-xs flex items-center gap-2 ${settingsStatus.success ? 'bg-emerald-950/60 text-emerald-300 border border-emerald-800' : 'bg-rose-950/60 text-rose-300 border border-rose-800'}`}>
          {settingsStatus.success ? <CheckCircle2 className="w-4 h-4 shrink-0" /> : <AlertCircle className="w-4 h-4 shrink-0" />}
          <span>{settingsStatus.message}</span>
        </div>
      )}

      {/* Add Custom Manual Column Editor */}
      <div className="p-4 bg-slate-950/60 rounded-xl border border-slate-800 flex items-center justify-between gap-4 text-xs">
        <span className="text-slate-300 font-medium">Add Manual Custom Column to Introspected Schema:</span>
        <div className="flex items-center gap-2">
          <input
            type="text"
            placeholder="New Column Name"
            value={newColName}
            onChange={(e) => setNewColName(e.target.value)}
            className="bg-slate-900 border border-slate-800 rounded p-1.5 text-xs text-slate-200 font-mono"
          />
          <button
            onClick={() => addManualColumn(true)}
            className="px-3 py-1.5 bg-sky-950 hover:bg-sky-900 text-sky-300 border border-sky-800/60 rounded font-semibold"
          >
            + Source
          </button>
          <button
            onClick={() => addManualColumn(false)}
            className="px-3 py-1.5 bg-purple-950 hover:bg-purple-900 text-purple-300 border border-purple-800/60 rounded font-semibold"
          >
            + Destination
          </button>
        </div>
      </div>
    </div>
  )
}
