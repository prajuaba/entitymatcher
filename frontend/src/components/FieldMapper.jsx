import React, { useState, useEffect } from 'react'
import { useMatcherStore } from '../store/useMatcherStore'
import { Sliders, Plus, Trash2, CheckSquare, Square, Layers, Link2, Info } from 'lucide-react'

export function FieldMapper({ availableSourceCols = [], availableDestCols = [], onMappingSaved, onMappingChange }) {
  const { config, updateConfig } = useMatcherStore()

  // Default detected columns if none provided
  const srcCols = availableSourceCols.length > 0 ? availableSourceCols : ['customer_name', "first_name", "last_name", 'reference_id', 'transaction_date', 'transaction_type', 'tax_id', 'phone']
  const destCols = availableDestCols.length > 0 ? availableDestCols : ['customer_name', "vendor_first", "vendor_last", 'customer_id', 'transaction_date', 'registration_num', 'contact_phone']

  const [mapping, setMapping] = useState(config.column_mapping || {
    name_fields_src: ['customer_name'],
    name_fields_dest: ['customer_name'],
    ref_id_src: 'reference_id',
    ref_id_dest: 'customer_id',
    date_field_src: 'transaction_date',
    date_field_dest: 'transaction_date',
    secondary_fields: [],
  })

  useEffect(() => {
    if (config.column_mapping) {
      setMapping(config.column_mapping)
    }
  }, [config.column_mapping])

  // Publish every edit upward so ConfigPanel's "Save Configuration" writes the
  // mapping the user is actually looking at, not the stale one it fetched.
  // Intentionally keyed on `mapping` alone: onMappingChange is an inline arrow
  // in the parent and would re-fire this effect on every render.
  useEffect(() => {
    if (onMappingChange) onMappingChange(mapping)
  }, [mapping])

  const toggleSrcNameField = (col) => {
    setMapping((prev) => {
      const current = prev.name_fields_src || []
      const exists = current.includes(col)
      let updated = []
      if (exists) {
        if (current.length === 1) return prev // keep at least 1
        updated = current.filter((c) => c !== col)
      } else {
        updated = [...current, col]
      }
      return { ...prev, name_fields_src: updated }
    })
  }

  const toggleDestNameField = (col) => {
    setMapping((prev) => {
      const current = prev.name_fields_dest || []
      const exists = current.includes(col)
      let updated = []
      if (exists) {
        if (current.length === 1) return prev // keep at least 1
        updated = current.filter((c) => c !== col)
      } else {
        updated = [...current, col]
      }
      return { ...prev, name_fields_dest: updated }
    })
  }

  const addSecondaryField = () => {
    setMapping((prev) => ({
      ...prev,
      secondary_fields: [
        ...(prev.secondary_fields || []),
        {
          name: `Secondary Field ${(prev.secondary_fields || []).length + 1}`,
          field_src: srcCols[0] || '',
          field_dest: destCols[0] || '',
          match_type: 'EXACT',
          weight: 0.15,
          is_mandatory: false,
        },
      ],
    }))
  }

  const removeSecondaryField = (index) => {
    setMapping((prev) => ({
      ...prev,
      secondary_fields: prev.secondary_fields.filter((_, i) => i !== index),
    }))
  }

  const updateSecondaryField = (index, key, val) => {
    setMapping((prev) => {
      const list = [...(prev.secondary_fields || [])]
      list[index] = { ...list[index], [key]: val }
      return { ...prev, secondary_fields: list }
    })
  }

  const handleSave = async () => {
    // Send only column_mapping: the backend merges per-field, so scoping the
    // write keeps this panel from overwriting settings it does not own.
    await updateConfig({ column_mapping: mapping })
    if (onMappingSaved) onMappingSaved(mapping)
  }

  return (
    <div className="space-y-6 bg-slate-950/80 p-6 rounded-xl border border-slate-800">
      <div className="flex items-center justify-between border-b border-slate-900 pb-3">
        <div>
          <h3 className="text-sm font-bold text-slate-100 flex items-center gap-2">
            <Layers className="w-4 h-4 text-sky-400" /> Dynamic Schema & Multi-Field Pairing Configuration
          </h3>
          <p className="text-xs text-slate-400 mt-0.5">Select single or multiple name columns to concatenate and configure secondary pairing attributes.</p>
        </div>
        <button
          onClick={handleSave}
          className="px-4 py-2 bg-sky-600 hover:bg-sky-500 text-white rounded-lg text-xs font-semibold flex items-center gap-1.5 transition shadow-sm"
        >
          Apply Schema Mapping
        </button>
      </div>

      {/* Primary Name Fields Selection (Single or Multiple) */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {/* Source Name Fields */}
        <div className="space-y-3 bg-slate-900/60 p-4 rounded-xl border border-slate-800">
          <label className="text-xs font-semibold text-sky-400 uppercase tracking-wider block">
            Source Primary Name Column(s)
          </label>
          <p className="text-[11px] text-slate-400">Select single or multiple fields to combine into entity name (e.g. First Name + Last Name):</p>
          <div className="flex flex-wrap gap-2 pt-1">
            {srcCols.map((col) => {
              const selected = (mapping.name_fields_src || []).includes(col)
              return (
                <button
                  key={col}
                  type="button"
                  onClick={() => toggleSrcNameField(col)}
                  className={`px-3 py-1.5 rounded-lg text-xs font-medium border flex items-center gap-1.5 transition ${
                    selected
                      ? 'bg-sky-950 text-sky-300 border-sky-600 font-semibold'
                      : 'bg-slate-950 text-slate-400 border-slate-800 hover:bg-slate-800'
                  }`}
                >
                  {selected ? <CheckSquare className="w-3.5 h-3.5 text-sky-400" /> : <Square className="w-3.5 h-3.5 text-slate-600" />}
                  <span>{col}</span>
                </button>
              )
            })}
          </div>
        </div>

        {/* Destination Name Fields */}
        <div className="space-y-3 bg-slate-900/60 p-4 rounded-xl border border-slate-800">
          <label className="text-xs font-semibold text-purple-400 uppercase tracking-wider block">
            Destination Primary Name Column(s)
          </label>
          <p className="text-[11px] text-slate-400">Select single or multiple fields to combine into candidate name:</p>
          <div className="flex flex-wrap gap-2 pt-1">
            {destCols.map((col) => {
              const selected = (mapping.name_fields_dest || []).includes(col)
              return (
                <button
                  key={col}
                  type="button"
                  onClick={() => toggleDestNameField(col)}
                  className={`px-3 py-1.5 rounded-lg text-xs font-medium border flex items-center gap-1.5 transition ${
                    selected
                      ? 'bg-purple-950 text-purple-300 border-purple-600 font-semibold'
                      : 'bg-slate-950 text-slate-400 border-slate-800 hover:bg-slate-800'
                  }`}
                >
                  {selected ? <CheckSquare className="w-3.5 h-3.5 text-purple-400" /> : <Square className="w-3.5 h-3.5 text-slate-600" />}
                  <span>{col}</span>
                </button>
              )
            })}
          </div>
        </div>
      </div>

      {/* ID & Date Mapping */}
      <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 gap-4 bg-slate-900/60 p-4 rounded-xl border border-slate-800">
        <div>
          <label className="text-xs font-medium text-slate-300 block mb-1">Source Reference ID Column</label>
          <select
            value={mapping.ref_id_src || ''}
            onChange={(e) => setMapping({ ...mapping, ref_id_src: e.target.value })}
            className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2 text-xs text-slate-200"
          >
            {srcCols.map((col) => (
              <option key={col} value={col}>{col}</option>
            ))}
          </select>
        </div>

        <div>
          <label className="text-xs font-medium text-slate-300 block mb-1">Destination ID Column</label>
          <select
            value={mapping.ref_id_dest || ''}
            onChange={(e) => setMapping({ ...mapping, ref_id_dest: e.target.value })}
            className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2 text-xs text-slate-200"
          >
            {destCols.map((col) => (
              <option key={col} value={col}>{col}</option>
            ))}
          </select>
        </div>

        <div>
          <label className="text-xs font-medium text-slate-300 block mb-1">Source Date Column</label>
          <select
            value={mapping.date_field_src || ''}
            onChange={(e) => setMapping({ ...mapping, date_field_src: e.target.value })}
            className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2 text-xs text-slate-200"
          >
            {srcCols.map((col) => (
              <option key={col} value={col}>{col}</option>
            ))}
          </select>
        </div>

        <div>
          <label className="text-xs font-medium text-slate-300 block mb-1">Destination Date Column</label>
          <select
            value={mapping.date_field_dest || ''}
            onChange={(e) => setMapping({ ...mapping, date_field_dest: e.target.value })}
            className="w-full bg-slate-950 border border-slate-800 rounded-lg p-2 text-xs text-slate-200"
          >
            {destCols.map((col) => (
              <option key={col} value={col}>{col}</option>
            ))}
          </select>
        </div>
      </div>

      {/* Dynamic Secondary Pairing Fields */}
      <div className="space-y-3 bg-slate-900/60 p-4 rounded-xl border border-slate-800">
        <div className="flex items-center justify-between">
          <div>
            <h4 className="text-xs font-bold text-slate-200 uppercase tracking-wider flex items-center gap-1.5">
              <Link2 className="w-3.5 h-3.5 text-sky-400" /> Secondary Attribute Pairing Columns
            </h4>
            <p className="text-[11px] text-slate-400">Pair secondary attributes (e.g. Tax ID, Category, Phone) with exact or fuzzy rules for extra confidence boosting.</p>
          </div>
          <button
            type="button"
            onClick={addSecondaryField}
            className="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-sky-400 rounded-lg text-xs font-semibold flex items-center gap-1 transition border border-slate-700"
          >
            <Plus className="w-3.5 h-3.5" /> Add Pairing Column
          </button>
        </div>

        {(!mapping.secondary_fields || mapping.secondary_fields.length === 0) ? (
          <div className="p-4 text-center text-slate-500 text-xs italic bg-slate-950 rounded-lg border border-slate-900">
            No secondary pairing attributes configured. Name & Date metrics will be evaluated.
          </div>
        ) : (
          <div className="space-y-2">
            {mapping.secondary_fields.map((sec, idx) => (
              <div key={idx} className="grid grid-cols-1 sm:grid-cols-12 gap-2 p-3 bg-slate-950 rounded-lg border border-slate-800 items-center text-xs">
                <input
                  type="text"
                  value={sec.name}
                  onChange={(e) => updateSecondaryField(idx, 'name', e.target.value)}
                  placeholder="Label e.g. Tax ID"
                  className="sm:col-span-3 bg-slate-900 border border-slate-800 rounded p-1.5 text-slate-200"
                />

                <select
                  value={sec.field_src}
                  onChange={(e) => updateSecondaryField(idx, 'field_src', e.target.value)}
                  className="sm:col-span-3 bg-slate-900 border border-slate-800 rounded p-1.5 text-slate-200"
                >
                  {srcCols.map((col) => (
                    <option key={col} value={col}>Src: {col}</option>
                  ))}
                </select>

                <select
                  value={sec.field_dest}
                  onChange={(e) => updateSecondaryField(idx, 'field_dest', e.target.value)}
                  className="sm:col-span-3 bg-slate-900 border border-slate-800 rounded p-1.5 text-slate-200"
                >
                  {destCols.map((col) => (
                    <option key={col} value={col}>Dest: {col}</option>
                  ))}
                </select>

                <select
                  value={sec.match_type}
                  onChange={(e) => updateSecondaryField(idx, 'match_type', e.target.value)}
                  className="sm:col-span-2 bg-slate-900 border border-slate-800 rounded p-1.5 text-slate-200 font-mono"
                >
                  <option value="EXACT">EXACT MATCH</option>
                  <option value="FUZZY">FUZZY MATCH</option>
                  <option value="NUMERIC_DELTA">NUMERIC</option>
                </select>

                <button
                  type="button"
                  onClick={() => removeSecondaryField(idx)}
                  className="sm:col-span-1 p-1.5 text-rose-400 hover:bg-rose-950 rounded transition text-center flex justify-center"
                >
                  <Trash2 className="w-4 h-4" />
                </button>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
