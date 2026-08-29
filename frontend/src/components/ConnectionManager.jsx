import React, { useState } from 'react'
import { Database, Server, FileSpreadsheet, FileCode, CheckCircle2, AlertCircle, RefreshCcw, Layers, Edit3, Plus, Trash2, Plug } from 'lucide-react'

export function ConnectionManager({ onSchemaIntrospected }) {
  const [srcType, setSrcType] = useState('SQLSERVER')
  const [srcConfig, setSrcConfig] = useState({
    host: '10.0.0.12',
    port: 1433,
    database: 'EnterpriseDB',
    username: 'sa',
    password: '••••••••',
    table_or_query: 'dbo.Customers',
  })

  const [destType, setDestType] = useState('MONGODB')
  const [destConfig, setDestConfig] = useState({
    host: 'mongodb.internal',
    port: 27017,
    database: 'CRM_DB',
    username: 'admin',
    password: '••••••••',
    table_or_query: 'clients_collection',
  })

  const [srcCols, setSrcCols] = useState(['CustID', 'CustomerName', 'TaxRegistrationNo', 'TxDate', 'BranchNo'])
  const [destCols, setDestCols] = useState(['_id', 'client_name', 'registration_id', 'created_at', 'contact_person'])

  const [srcStatus, setSrcStatus] = useState(null)
  const [destStatus, setDestStatus] = useState(null)
  const [loadingSrc, setLoadingSrc] = useState(false)
  const [loadingDest, setLoadingDest] = useState(false)

  // Manual Column & Row Editor State
  const [manualSrcCols, setManualSrcCols] = useState(['id', 'customer_name', 'tx_date', 'tax_id'])
  const [newColName, setNewColName] = useState('')

  const handleTestOnly = async (isSource) => {
    const type = isSource ? srcType : destType
    const cfg = isSource ? srcConfig : destConfig

    if (isSource) setLoadingSrc(true)
    else setLoadingDest(true)

    try {
      const res = await fetch('/api/connector/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ type, ...cfg }),
      })
      const data = await res.json()
      if (res.ok) {
        const msg = data.message || `Successfully connected to ${type} database!`
        if (isSource) setSrcStatus({ success: true, message: msg })
        else setDestStatus({ success: true, message: msg })
      } else {
        throw new Error(data.message || 'Connection test failed')
      }
    } catch (e) {
      if (isSource) setSrcStatus({ success: false, message: e.message })
      else setDestStatus({ success: false, message: e.message })
    } finally {
      if (isSource) setLoadingSrc(false)
      else setLoadingDest(false)
    }
  }

  const handleTestAndIntrospect = async (isSource) => {
    const type = isSource ? srcType : destType
    const cfg = isSource ? srcConfig : destConfig

    if (isSource) setLoadingSrc(true)
    else setLoadingDest(true)

    try {
      const res = await fetch('/api/connector/introspect', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ type, ...cfg }),
      })

      const data = await res.json()
      if (res.ok) {
        const extractedCols = (data.columns || []).map((c) => c.name)
        if (isSource) {
          setSrcCols(extractedCols)
          setSrcStatus({ success: true, message: `Connected to ${type}! Introspected ${extractedCols.length} columns.` })
        } else {
          setDestCols(extractedCols)
          setDestStatus({ success: true, message: `Connected to ${type}! Introspected ${extractedCols.length} columns.` })
        }

        if (onSchemaIntrospected) {
          onSchemaIntrospected(
            isSource ? extractedCols : srcCols,
            isSource ? destCols : extractedCols
          )
        }
      } else {
        throw new Error(data.message || 'Introspection failed')
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
              onChange={(e) => setSrcType(e.target.value)}
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
                type="text"
                placeholder="Database Name"
                value={srcConfig.database}
                onChange={(e) => setSrcConfig({ ...srcConfig, database: e.target.value })}
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
                  const file = e.target.files[0]
                  if (file) {
                    const reader = new FileReader()
                    reader.onload = (evt) => {
                      const text = evt.target.result
                      const firstLine = text.split('\n')[0]
                      const headers = firstLine.split(',').map((h) => h.trim().replace(/^"|"$/g, ''))
                      setSrcCols(headers)
                      setSrcStatus({ success: true, message: `Parsed CSV file ${file.name}! Detected ${headers.length} headers.` })
                    }
                    reader.readAsText(file)
                  }
                }}
                className="w-full bg-slate-900 border border-slate-800 rounded p-2 text-slate-300 font-mono text-xs file:mr-3 file:py-1 file:px-3 file:rounded file:border-0 file:text-xs file:bg-sky-950 file:text-sky-300"
              />
              <input
                type="text"
                placeholder="Or Server File Path (e.g. /var/data/source.csv)"
                value={srcConfig.file_path || ''}
                onChange={(e) => setSrcConfig({ ...srcConfig, file_path: e.target.value })}
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
              onChange={(e) => setDestType(e.target.value)}
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
                type="text"
                placeholder="Database Name"
                value={destConfig.database}
                onChange={(e) => setDestConfig({ ...destConfig, database: e.target.value })}
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
                  const file = e.target.files[0]
                  if (file) {
                    const reader = new FileReader()
                    reader.onload = (evt) => {
                      const text = evt.target.result
                      const firstLine = text.split('\n')[0]
                      const headers = firstLine.split(',').map((h) => h.trim().replace(/^"|"$/g, ''))
                      setDestCols(headers)
                      setDestStatus({ success: true, message: `Parsed CSV file ${file.name}! Detected ${headers.length} headers.` })
                    }
                    reader.readAsText(file)
                  }
                }}
                className="w-full bg-slate-900 border border-slate-800 rounded p-2 text-slate-300 font-mono text-xs file:mr-3 file:py-1 file:px-3 file:rounded file:border-0 file:text-xs file:bg-purple-950 file:text-purple-300"
              />
              <input
                type="text"
                placeholder="Or Server File Path (e.g. /var/data/dest.csv)"
                value={destConfig.file_path || ''}
                onChange={(e) => setDestConfig({ ...destConfig, file_path: e.target.value })}
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
