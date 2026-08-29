import React, { useState, useEffect } from 'react'
import { useMatcherStore } from '../store/useMatcherStore'
import { ConnectionManager } from './ConnectionManager'
import { FieldMapper } from './FieldMapper'
import { DictionaryManager } from './DictionaryManager'
import { Sliders, CheckSquare, Square, Save, RotateCcw } from 'lucide-react'

export function ConfigPanel() {
  const { config, updateConfig, fetchConfig, loading } = useMatcherStore()
  const [localCfg, setLocalCfg] = useState(config)
  const [savedMessage, setSavedMessage] = useState(false)
  const [introspectedSrcCols, setIntrospectedSrcCols] = useState([])
  const [introspectedDestCols, setIntrospectedDestCols] = useState([])

  useEffect(() => {
    fetchConfig()
  }, [])

  useEffect(() => {
    setLocalCfg(config)
  }, [config])

  const handleSave = async () => {
    await updateConfig(localCfg)
    setSavedMessage(true)
    setTimeout(() => setSavedMessage(false), 3000)
  }

  const toggleAlgo = (key) => {
    setLocalCfg((prev) => ({
      ...prev,
      algorithms: {
        ...prev.algorithms,
        [key]: !prev.algorithms?.[key],
      },
    }))
  }

  return (
    <div className="max-w-4xl mx-auto space-y-6 bg-slate-900/60 p-8 rounded-2xl border border-slate-800">
      <div className="flex items-center justify-between border-b border-slate-800 pb-4">
        <div>
          <h2 className="text-xl font-bold text-slate-100 flex items-center gap-2">
            <Sliders className="w-5 h-5 text-sky-400" /> Heterogeneous Data Connectors & Schema Pairing
          </h2>
          <p className="text-xs text-slate-400 mt-1">Connect SQL Server, Postgres, MongoDB, Excel, CSV, or Manual entry, introspect schema columns, and map fields.</p>
        </div>
        <div className="flex items-center gap-3">
          <button
            onClick={() => setLocalCfg(config)}
            className="px-3.5 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg text-xs font-medium flex items-center gap-1.5 transition"
          >
            <RotateCcw className="w-3.5 h-3.5" /> Reset
          </button>
          <button
            onClick={handleSave}
            disabled={loading}
            className="px-4 py-2 bg-sky-600 hover:bg-sky-500 text-white rounded-lg text-xs font-semibold flex items-center gap-1.5 transition shadow-sm"
          >
            <Save className="w-4 h-4" /> Save Configuration
          </button>
        </div>
      </div>

      {/* Database & File Data Source Connection Manager */}
      <ConnectionManager
        onSchemaIntrospected={(srcCols, destCols) => {
          setIntrospectedSrcCols(srcCols)
          setIntrospectedDestCols(destCols)
        }}
      />

      {/* Dynamic Schema & Multi-Field Pairing Mapper */}
      <FieldMapper
        availableSourceCols={introspectedSrcCols}
        availableDestCols={introspectedDestCols}
      />

      {/* Enterprise Brand Synonym & Alias Manager */}
      <DictionaryManager />

      {savedMessage && (
        <div className="p-3 bg-emerald-950/80 border border-emerald-700/50 text-emerald-300 rounded-lg text-xs font-medium text-center">
          Configuration updated successfully!
        </div>
      )}

      {/* Thresholds Section */}
      <div className="bg-slate-950/80 p-6 rounded-xl border border-slate-800 space-y-5">
        <h3 className="text-sm font-semibold text-slate-200 border-b border-slate-900 pb-2">Confidence Thresholds</h3>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          <div>
            <div className="flex justify-between text-xs font-medium text-slate-300 mb-2">
              <span>Auto-Match Threshold</span>
              <span className="font-mono text-emerald-400 font-bold">{Math.round((localCfg.auto_match_threshold || 0.90) * 100)}%</span>
            </div>
            <input
              type="range"
              min="0.50"
              max="1.00"
              step="0.01"
              value={localCfg.auto_match_threshold || 0.90}
              onChange={(e) => setLocalCfg({ ...localCfg, auto_match_threshold: parseFloat(e.target.value) })}
              className="w-full h-2 bg-slate-800 rounded-lg appearance-none cursor-pointer accent-emerald-500"
            />
            <p className="text-[11px] text-slate-500 mt-1">Pairs scoring above this threshold are auto-approved.</p>
          </div>

          <div>
            <div className="flex justify-between text-xs font-medium text-slate-300 mb-2">
              <span>Manual Review Threshold</span>
              <span className="font-mono text-amber-400 font-bold">{Math.round((localCfg.review_threshold || 0.70) * 100)}%</span>
            </div>
            <input
              type="range"
              min="0.30"
              max="0.90"
              step="0.01"
              value={localCfg.review_threshold || 0.70}
              onChange={(e) => setLocalCfg({ ...localCfg, review_threshold: parseFloat(e.target.value) })}
              className="w-full h-2 bg-slate-800 rounded-lg appearance-none cursor-pointer accent-amber-500"
            />
            <p className="text-[11px] text-slate-500 mt-1">Pairs scoring between Review & Auto-Match enter manual queue.</p>
          </div>
        </div>
      </div>

      {/* Weights & Date Tolerance */}
      <div className="bg-slate-950/80 p-6 rounded-xl border border-slate-800 space-y-5">
        <h3 className="text-sm font-semibold text-slate-200 border-b border-slate-900 pb-2">Scoring Weights & Date Proximity</h3>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          <div>
            <div className="flex justify-between text-xs font-medium text-slate-300 mb-2">
              <span>Name Weight vs Date Weight</span>
              <span className="font-mono text-sky-400 font-bold">
                Name: {Math.round((localCfg.weights?.name_weight || 0.85) * 100)}% | Date: {Math.round((localCfg.weights?.date_weight || 0.15) * 100)}%
              </span>
            </div>
            <input
              type="range"
              min="0.50"
              max="1.00"
              step="0.05"
              value={localCfg.weights?.name_weight || 0.85}
              onChange={(e) => {
                const nw = parseFloat(e.target.value)
                setLocalCfg({
                  ...localCfg,
                  weights: { name_weight: nw, date_weight: Math.round((1 - nw) * 100) / 100 },
                })
              }}
              className="w-full h-2 bg-slate-800 rounded-lg appearance-none cursor-pointer accent-sky-500"
            />
          </div>

          <div>
            <label className="text-xs font-medium text-slate-300 block mb-2">Transaction Date Tolerance Window</label>
            <div className="flex gap-2">
              {[0, 1, 3, 7, 30].map((days) => (
                <button
                  key={days}
                  type="button"
                  onClick={() => setLocalCfg({ ...localCfg, date_tolerance_days: days })}
                  className={`flex-1 py-1.5 rounded-lg text-xs font-semibold border transition ${
                    localCfg.date_tolerance_days === days
                      ? 'bg-sky-600 text-white border-sky-500'
                      : 'bg-slate-900 text-slate-400 border-slate-800 hover:bg-slate-800'
                  }`}
                >
                  ±{days} {days === 1 ? 'day' : 'days'}
                </button>
              ))}
            </div>
          </div>
        </div>
      </div>

      {/* Algorithm Toggles */}
      <div className="bg-slate-950/80 p-6 rounded-xl border border-slate-800 space-y-4">
        <h3 className="text-sm font-semibold text-slate-200 border-b border-slate-900 pb-2">Enabled Similarity Metric Algorithms</h3>

        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3">
          {[
            { key: 'use_jaro_winkler', label: 'Jaro-Winkler Distance', desc: 'Rune-safe string prefix similarity' },
            { key: 'use_token_sort', label: 'Token Sort Ratio', desc: 'Alphabetical word ordering invariance' },
            { key: 'use_levenshtein', label: 'Levenshtein Edit Distance', desc: 'Character insertion/deletion penalty' },
            { key: 'use_trigram', label: 'Character Trigram Overlap', desc: '3-gram sub-string jaccard similarity' },
            { key: 'use_phonetic', label: 'Phonetic Consonant Match', desc: 'Thai/English phonetic key indexing' },
          ].map((algo) => {
            const active = localCfg.algorithms?.[algo.key]
            return (
              <button
                key={algo.key}
                type="button"
                onClick={() => toggleAlgo(algo.key)}
                className={`p-3 text-left rounded-xl border transition flex items-start gap-3 ${
                  active ? 'bg-sky-950/40 border-sky-800/60 text-slate-100' : 'bg-slate-900/60 border-slate-800/60 text-slate-500'
                }`}
              >
                {active ? <CheckSquare className="w-5 h-5 text-sky-400 shrink-0 mt-0.5" /> : <Square className="w-5 h-5 text-slate-600 shrink-0 mt-0.5" />}
                <div>
                  <div className="text-xs font-semibold">{algo.label}</div>
                  <div className="text-[11px] text-slate-400 mt-0.5">{algo.desc}</div>
                </div>
              </button>
            )
          })}
        </div>
      </div>
    </div>
  )
}
