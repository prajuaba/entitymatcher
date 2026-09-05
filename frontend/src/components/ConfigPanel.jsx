import React, { useState, useEffect } from 'react'
import { useMatcherStore } from '../store/useMatcherStore'
import { ConnectionManager } from './ConnectionManager'
import { FieldMapper } from './FieldMapper'
import { DictionaryManager } from './DictionaryManager'
import { SchedulerPanel } from './SchedulerPanel'
import { CalibrationPanel } from './CalibrationPanel'
import { can } from '../lib/rbac'
import { Sliders, CheckSquare, Square, Save, RotateCcw, AlertTriangle, ChevronDown } from 'lucide-react'

// These nine fallbacks intentionally mirror DefaultConfig() in backend/matcher/pipeline.go (emit_unmatched: true)
// and DefaultAlgorithms in backend/matcher/scorer.go (all eight algorithm toggles default true),
// so that a config arriving without a key proposes the engine's real default rather than silently proposing to disable the feature.
const withDefaults = (cfg) => ({
  auto_match_threshold: cfg.auto_match_threshold ?? 0.90,
  review_threshold: cfg.review_threshold ?? 0.70,
  date_tolerance_days: cfg.date_tolerance_days ?? 30,
  margin_threshold: cfg.margin_threshold ?? 0.05,
  assignment_strategy: cfg.assignment_strategy ?? 'GREEDY_1_1',
  emit_unmatched: cfg.emit_unmatched ?? true,
  cross_script_auto_threshold: cfg.cross_script_auto_threshold ?? 0.84,
  no_distinctive_overlap_cap: cfg.no_distinctive_overlap_cap ?? 0.85,
  distinctive_overlap_min_weight: cfg.distinctive_overlap_min_weight ?? 0.30,
  weights: {
    name_weight: cfg.weights?.name_weight ?? 0.85,
    date_weight: cfg.weights?.date_weight ?? 0.15,
  },
  algorithms: {
    use_jaro_winkler: cfg.algorithms?.use_jaro_winkler ?? true,
    use_levenshtein: cfg.algorithms?.use_levenshtein ?? true,
    use_token_sort: cfg.algorithms?.use_token_sort ?? true,
    use_trigram: cfg.algorithms?.use_trigram ?? true,
    use_phonetic: cfg.algorithms?.use_phonetic ?? true,
    use_thai_phonetic: cfg.algorithms?.use_thai_phonetic ?? true,
    use_corpus_idf: cfg.algorithms?.use_corpus_idf ?? true,
    use_romanized_match: cfg.algorithms?.use_romanized_match ?? true,
  },
  column_mapping: {
    // Spread first so fields this whitelist does not know about survive a save.
    // Rebuilding column_mapping key-by-key silently dropped date_calendar_src/dest
    // (and previously blanked date_field_src/dest), wiping config on every save.
    ...(cfg.column_mapping ?? {}),
    date_calendar_src: cfg.column_mapping?.date_calendar_src || 'AUTO',
    date_calendar_dest: cfg.column_mapping?.date_calendar_dest || 'AUTO',
    name_fields_src: cfg.column_mapping?.name_fields_src ?? [],
    name_fields_dest: cfg.column_mapping?.name_fields_dest ?? [],
    ref_id_src: cfg.column_mapping?.ref_id_src ?? '',
    ref_id_dest: cfg.column_mapping?.ref_id_dest ?? '',
    date_field_src: cfg.column_mapping?.date_field_src ?? '',
    date_field_dest: cfg.column_mapping?.date_field_dest ?? '',
    secondary_fields: cfg.column_mapping?.secondary_fields ?? [],
  },
})

function countDirtyFields(a, b) {
  const aDef = withDefaults(a)
  const bDef = withDefaults(b)
  let count = 0

  if (aDef.auto_match_threshold !== bDef.auto_match_threshold) count++
  if (aDef.review_threshold !== bDef.review_threshold) count++
  if (aDef.date_tolerance_days !== bDef.date_tolerance_days) count++
  if (aDef.margin_threshold !== bDef.margin_threshold) count++
  if (aDef.assignment_strategy !== bDef.assignment_strategy) count++
  if (aDef.emit_unmatched !== bDef.emit_unmatched) count++
  if (aDef.cross_script_auto_threshold !== bDef.cross_script_auto_threshold) count++
  if (aDef.no_distinctive_overlap_cap !== bDef.no_distinctive_overlap_cap) count++
  if (aDef.distinctive_overlap_min_weight !== bDef.distinctive_overlap_min_weight) count++

  if (aDef.weights.name_weight !== bDef.weights.name_weight) count++
  if (aDef.weights.date_weight !== bDef.weights.date_weight) count++

  if (aDef.algorithms.use_jaro_winkler !== bDef.algorithms.use_jaro_winkler) count++
  if (aDef.algorithms.use_levenshtein !== bDef.algorithms.use_levenshtein) count++
  if (aDef.algorithms.use_token_sort !== bDef.algorithms.use_token_sort) count++
  if (aDef.algorithms.use_trigram !== bDef.algorithms.use_trigram) count++
  if (aDef.algorithms.use_phonetic !== bDef.algorithms.use_phonetic) count++
  if (aDef.algorithms.use_thai_phonetic !== bDef.algorithms.use_thai_phonetic) count++
  if (aDef.algorithms.use_corpus_idf !== bDef.algorithms.use_corpus_idf) count++
  if (aDef.algorithms.use_romanized_match !== bDef.algorithms.use_romanized_match) count++

  if (JSON.stringify(aDef.column_mapping.name_fields_src) !== JSON.stringify(bDef.column_mapping.name_fields_src)) count++
  if (JSON.stringify(aDef.column_mapping.name_fields_dest) !== JSON.stringify(bDef.column_mapping.name_fields_dest)) count++
  if (aDef.column_mapping.ref_id_src !== bDef.column_mapping.ref_id_src) count++
  if (aDef.column_mapping.ref_id_dest !== bDef.column_mapping.ref_id_dest) count++
  if (aDef.column_mapping.date_field_src !== bDef.column_mapping.date_field_src) count++
  if (aDef.column_mapping.date_field_dest !== bDef.column_mapping.date_field_dest) count++
  if (JSON.stringify(aDef.column_mapping.secondary_fields) !== JSON.stringify(bDef.column_mapping.secondary_fields)) count++

  return count
}

const ALGO_LIST = [
  { key: 'use_jaro_winkler', label: 'Jaro-Winkler Distance', desc: 'Rune-safe string prefix similarity' },
  { key: 'use_token_sort', label: 'Token Sort Ratio', desc: 'Alphabetical word ordering invariance' },
  { key: 'use_levenshtein', label: 'Levenshtein Edit Distance', desc: 'Character insertion/deletion penalty' },
  { key: 'use_trigram', label: 'Character Trigram Overlap', desc: '3-gram sub-string jaccard similarity' },
  { key: 'use_phonetic', label: 'Phonetic Consonant Match', desc: 'Thai/English phonetic key indexing' },
  { key: 'use_thai_phonetic', label: 'Thai Phonetic Key', desc: 'Thai-specific consonant/vowel phonetic keying' },
  { key: 'use_corpus_idf', label: 'Corpus IDF Weighting', desc: 'Rare tokens count more than common ones' },
  { key: 'use_romanized_match', label: 'Romanized Matching', desc: 'Matches Thai script against RTGS/English spellings' },
]

export function ConfigPanel() {
  const { config, updateConfig, fetchConfig, loading, user } = useMatcherStore()
  const saveConnectorSettings = useMatcherStore((s) => s.saveConnectorSettings)
  const [localCfg, setLocalCfg] = useState(withDefaults(config))
  const [savedMessage, setSavedMessage] = useState(false)
  const [saveError, setSaveError] = useState(null)
  const [introspectedSrcCols, setIntrospectedSrcCols] = useState([])
  const [introspectedDestCols, setIntrospectedDestCols] = useState([])
  const [connectorPayload, setConnectorPayload] = useState(null)
  const [activeTab, setActiveTab] = useState('Matching Rules')
  const [reviewClampNote, setReviewClampNote] = useState(false)
  const [dirtyCount, setDirtyCount] = useState(0)

  // Keep localCfg synced with store config on mount and whenever config changes
  useEffect(() => {
    setLocalCfg(withDefaults(config))
    setDirtyCount(countDirtyFields(withDefaults(config), withDefaults(config)))
  }, [config])

  useEffect(() => {
    setDirtyCount(countDirtyFields(localCfg, config))
  }, [localCfg, config])

  // Beforeunload listener when dirty
  useEffect(() => {
    if (dirtyCount > 0) {
      const handleBeforeUnload = (e) => {
        e.preventDefault()
        e.returnValue = ''
      }
      window.addEventListener('beforeunload', handleBeforeUnload)
      return () => window.removeEventListener('beforeunload', handleBeforeUnload)
    }
  }, [dirtyCount])

  const handleSave = async () => {
    setSaveError(null)
    try {
      await updateConfig(localCfg)
      if (connectorPayload) await saveConnectorSettings(connectorPayload)
      setSavedMessage(true)
      setTimeout(() => setSavedMessage(false), 3000)
    } catch (e) {
      setSaveError(e.message)
    }
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

  const selectAllAlgorithms = () => {
    setLocalCfg((prev) => ({
      ...prev,
      algorithms: {
        use_jaro_winkler: true,
        use_levenshtein: true,
        use_token_sort: true,
        use_trigram: true,
        use_phonetic: true,
        use_thai_phonetic: true,
        use_corpus_idf: true,
        use_romanized_match: true,
      },
    }))
  }

  const clearAllAlgorithms = () => {
    setLocalCfg((prev) => ({
      ...prev,
      algorithms: {
        use_jaro_winkler: false,
        use_levenshtein: false,
        use_token_sort: false,
        use_trigram: false,
        use_phonetic: false,
        use_thai_phonetic: false,
        use_corpus_idf: false,
        use_romanized_match: false,
      },
    }))
  }

  const allAlgosOff = ALGO_LIST.every(a => !localCfg.algorithms?.[a.key])

  // These client-side checks mirror server-side validation in backend/api/handlers.go
  // so the user sees the problem before submitting, not after a 400.
  const review = localCfg.review_threshold ?? 0.70
  const auto = localCfg.auto_match_threshold ?? 0.90
  const cap = localCfg.no_distinctive_overlap_cap ?? 0.85

  const capViolatesRule = cap > 0 && cap >= auto
  const reviewViolatesRule = review > auto
  const hasBlockingValidationError = capViolatesRule || reviewViolatesRule

  // Tab list & panels
  const tabs = ["Data Sources", "Matching Rules", "Algorithms"]
  const canOperations = can(user, 'SCHEDULER_CONFIG') || can(user, 'CALIBRATION')
  if (canOperations) tabs.push("Operations")

  return (
    <div className="max-w-4xl mx-auto space-y-6 bg-slate-900/60 p-8 rounded-2xl border border-slate-800 flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center justify-between border-b border-slate-800 pb-4">
        <div>
          <h2 className="text-xl font-bold text-slate-100 flex items-center gap-2">
            <Sliders className="w-5 h-5 text-sky-400" /> Engine Configuration
          </h2>
          <p className="text-xs text-slate-400 mt-1">Confidence thresholds, scoring weights, similarity algorithms, and data source connections for the matching engine.</p>
          {dirtyCount > 0 && (
            <span className="ml-0 mt-1 inline-flex items-center px-2 py-0.5 rounded-full text-[11px] font-semibold bg-amber-950/60 text-amber-300 border border-amber-700/50">
              {dirtyCount === 1 ? '1 unsaved change' : `${dirtyCount} unsaved changes`}
            </span>
          )}
        </div>
      </div>

      {/* Tab List */}
      <div
        role="tablist"
        className="flex flex-wrap items-center gap-1.5 mb-2"
        onKeyDown={(e) => {
          if (['ArrowLeft', 'ArrowRight'].includes(e.key)) {
            e.preventDefault()
            const idx = tabs.indexOf(activeTab)
            if (e.key === 'ArrowLeft') {
              setActiveTab(tabs[(idx - 1 + tabs.length) % tabs.length])
            } else {
              setActiveTab(tabs[(idx + 1) % tabs.length])
            }
          }
        }}
      >
        {tabs.map((tab) => {
          const isActive = activeTab === tab
          return (
            <button
              key={tab}
              role="tab"
              aria-selected={isActive}
              aria-controls={`configpanel-panel-${tab}`}
              id={`configpanel-tab-${tab}`}
              onClick={() => setActiveTab(tab)}
              className={`px-3 py-1.5 rounded-lg text-xs font-semibold whitespace-nowrap transition ${
                isActive
                  ? 'bg-sky-600 text-white shadow-sm'
                  : 'bg-slate-950/60 text-slate-400 border border-slate-800 hover:text-slate-200'
              }`}
            >
              {tab}
            </button>
          )
        })}
      </div>

      {/* Tab Content */}
      <div className="flex-1 min-h-0 overflow-y-auto">
        {/* Data Sources Tab */}
        {activeTab === 'Data Sources' && (
          <div
            id="configpanel-panel-Data Sources"
            role="tabpanel"
            aria-labelledby="configpanel-tab-Data Sources"
            className="space-y-6"
          >
            <ConnectionManager
              onSchemaIntrospected={(srcCols, destCols) => {
                setIntrospectedSrcCols(srcCols)
                setIntrospectedDestCols(destCols)
              }}
              onSettingsChange={setConnectorPayload}
            />
            <FieldMapper
              availableSourceCols={introspectedSrcCols}
              availableDestCols={introspectedDestCols}
              onMappingChange={(mapping) =>
                setLocalCfg((prev) => ({ ...prev, column_mapping: mapping }))
              }
            />
            <DictionaryManager />
          </div>
        )}

        {/* Matching Rules Tab */}
        {activeTab === 'Matching Rules' && (
          <div
            id="configpanel-panel-Matching Rules"
            role="tabpanel"
            aria-labelledby="configpanel-tab-Matching Rules"
            className="space-y-6"
          >
            {/* Confidence Thresholds */}
            <div className="bg-slate-950/80 p-6 rounded-xl border border-slate-800 space-y-5">
              <h3 className="text-sm font-semibold text-slate-200 border-b border-slate-900 pb-2">Confidence Thresholds</h3>

              {/* Threshold band visualization */}
              <div className="relative">
                <div role="img" aria-label={`Score ranges: 0 to ${Math.round(review*100)}% is no match, ${Math.round(review*100)}% to ${Math.round(auto*100)}% is manual review, ${Math.round(auto*100)}% to 100% is auto-match.`} className="h-7 overflow-hidden flex rounded-lg">
                <div style={{ width: `${Math.min(review*100, 100)}%` }} className="bg-rose-600/70 flex items-center justify-center">
                  <span className="text-[10px] font-semibold truncate px-1 text-rose-50">No match · 0-{Math.round(review*100)}%</span>
                </div>
                <div style={{ width: `${Math.max(0, (auto - review) * 100)}%` }} className="bg-amber-600/70 flex items-center justify-center">
                  <span className="text-[10px] font-semibold truncate px-1 text-amber-50">Manual review · {Math.round(review*100)}-{Math.round(auto*100)}%</span>
                </div>
                <div style={{ width: `${Math.max(0, (1 - auto) * 100)}%` }} className="bg-emerald-600/70 flex items-center justify-center">
                  <span className="text-[10px] font-semibold truncate px-1 text-emerald-50">Auto-match · {Math.round(auto*100)}-100%</span>
                </div>
              </div>

              {/* Tick labels */}
              <div className="flex justify-between mt-2">
                <span className="text-[10px] text-slate-500">0%</span>
                <span className="text-[10px] text-slate-500">{Math.round(review*100)}%</span>
                <span className="text-[10px] text-slate-500">{Math.round(auto*100)}%</span>
                <span className="text-[10px] text-slate-500">100%</span>
              </div>

              {/* Threshold sliders */}
              <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mt-6">
                <div>
                  <div className="flex justify-between text-xs font-medium text-slate-300 mb-2">
                    <span>Auto-Match Threshold</span>
                    <span className="font-mono text-emerald-400 font-bold">{Math.round((localCfg.auto_match_threshold ?? 0.90) * 100)}%</span>
                  </div>
                  <input
                    type="range"
                    min="0.50"
                    max="1.00"
                    step="0.01"
                    value={localCfg.auto_match_threshold ?? 0.90}
                    aria-label="Auto-Match Threshold"
                    onChange={(e) => {
                      const auto = parseFloat(e.target.value)
                      setLocalCfg((prev) => ({
                        ...prev,
                        auto_match_threshold: auto,
                        review_threshold: Math.min(prev.review_threshold ?? 0.7, auto),
                      }))
                    }}
                    className="w-full h-2 bg-slate-800 rounded-lg appearance-none cursor-pointer accent-emerald-500"
                  />
                  <p className="text-[11px] text-slate-500 mt-1">Pairs scoring above this threshold are auto-approved.</p>
                </div>

                <div>
                  <div className="flex justify-between text-xs font-medium text-slate-300 mb-2">
                    <span>Manual Review Threshold</span>
                    <span className="font-mono text-amber-400 font-bold">{Math.round((localCfg.review_threshold ?? 0.70) * 100)}%</span>
                  </div>
                  <input
                    type="range"
                    min="0.30"
                    max="0.90"
                    step="0.01"
                    value={localCfg.review_threshold ?? 0.70}
                    aria-label="Manual Review Threshold"
                    onChange={(e) => {
                      const review = parseFloat(e.target.value)
                      const autoVal = localCfg.auto_match_threshold ?? 0.90
                      const clampedReview = Math.min(review, autoVal)
                      setReviewClampNote(clampedReview < review)
                      setLocalCfg((prev) => ({
                        ...prev,
                        review_threshold: clampedReview,
                      }))
                    }}
                    className="w-full h-2 bg-slate-800 rounded-lg appearance-none cursor-pointer accent-amber-500"
                  />
                  {reviewViolatesRule && (
                    <p className="text-[11px] text-rose-400 mt-1">Review threshold cannot exceed Auto-Match Threshold.</p>
                  )}
                  {reviewClampNote && (
                    <p className="text-[11px] text-amber-400 mt-1">Review threshold was lowered to stay at or below Auto-Match ({Math.round(auto*100)}%).</p>
                  )}
                  {!reviewViolatesRule && !reviewClampNote && (
                    <p className="text-[11px] text-slate-500 mt-1">Pairs scoring between Review & Auto-Match enter manual queue.</p>
                  )}
                </div>
              </div>
            </div>
          </div>

          {/* Scoring Weights & Date Proximity */}
          <div className="bg-slate-950/80 p-6 rounded-xl border border-slate-800 space-y-5">
            <h3 className="text-sm font-semibold text-slate-200 border-b border-slate-900 pb-2">Scoring Weights & Date Proximity</h3>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <div>
                <div className="flex justify-between text-xs font-medium text-slate-300 mb-2">
                  <span>Name Weight vs Date Weight</span>
                  <span className="font-mono text-sky-400 font-bold">
                    Name: {Math.round((localCfg.weights?.name_weight ?? 0.85) * 100)}% | Date: {Math.round((localCfg.weights?.date_weight ?? 0.15) * 100)}%
                  </span>
                </div>
                <input
                  type="range"
                  min="0.50"
                  max="0.95"
                  step="0.05"
                  value={localCfg.weights?.name_weight ?? 0.85}
                  aria-label="Name Weight vs Date Weight"
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
                <div className="flex gap-2 items-center">
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
                      aria-pressed={localCfg.date_tolerance_days === days}
                    >
                      ±{days} {days === 1 ? 'day' : 'days'}
                    </button>
                  ))}
                  <input
                    type="number"
                    min="0"
                    step="1"
                    aria-label="Date Tolerance Days"
                    value={localCfg.date_tolerance_days ?? 30}
                    onChange={(e) => setLocalCfg({ ...localCfg, date_tolerance_days: parseInt(e.target.value, 10) || 0 })}
                    className="w-16 bg-slate-900 border border-slate-800 rounded-lg px-2 py-1.5 text-xs text-slate-200 text-center focus:outline-none focus:border-sky-500"
                  />
                </div>
              </div>
            </div>
          </div>

          {/* Advanced Matching Strategy */}
          <div className="bg-slate-950/80 p-6 rounded-xl border border-slate-800 space-y-5">
            <h3 className="text-sm font-semibold text-slate-200 border-b border-slate-900 pb-2">Advanced Matching Strategy</h3>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <div>
                <label className="text-xs font-medium text-slate-300 block mb-2">Assignment Strategy</label>
                <select
                  value={localCfg.assignment_strategy ?? 'GREEDY_1_1'}
                  onChange={(e) => setLocalCfg({ ...localCfg, assignment_strategy: e.target.value })}
                  className="w-full bg-slate-900 border border-slate-800 rounded-lg px-3 py-2 text-xs text-slate-200 focus:outline-none focus:border-sky-500 focus:ring-1 focus:ring-sky-500"
                >
                  <option value="GREEDY_1_1">Greedy 1:1 Matching</option>
                  <option value="TOP_1">Top-1 Priority</option>
                  <option value="ALL_CANDIDATES">All Candidates</option>
                </select>
                <p className="text-[11px] text-slate-500 mt-1">How to handle multiple candidates per source record</p>
              </div>

              <div>
                <div className="flex justify-between text-xs font-medium text-slate-300 mb-2">
                  <span>Score Margin Threshold</span>
                  <span className="font-mono text-sky-400 font-bold">{Math.round((localCfg.margin_threshold ?? 0.05) * 100)}%</span>
                </div>
                <input
                  type="range"
                  min="0"
                  max="1"
                  step="0.01"
                  value={localCfg.margin_threshold ?? 0.05}
                  aria-label="Score Margin Threshold"
                  onChange={(e) => setLocalCfg({ ...localCfg, margin_threshold: parseFloat(e.target.value) })}
                  className="w-full h-2 bg-slate-800 rounded-lg appearance-none cursor-pointer accent-sky-500"
                />
                <p className="text-[11px] text-slate-500 mt-1">Minimum gap between best and second-best candidate required for auto-match</p>
              </div>
            </div>

            <label className="flex items-center gap-3 cursor-pointer">
              <input
                type="checkbox"
                checked={localCfg.emit_unmatched}
                onChange={(e) => setLocalCfg({ ...localCfg, emit_unmatched: e.target.checked })}
                className="w-4 h-4 rounded border-slate-700 accent-emerald-600 cursor-pointer"
              />
              <span className="text-xs text-slate-300">Include unmatched source records in results</span>
            </label>
          </div>
        </div>
        )}

        {/* Algorithms Tab */}
        {activeTab === 'Algorithms' && (
          <div
            id="configpanel-panel-Algorithms"
            role="tabpanel"
            aria-labelledby="configpanel-tab-Algorithms"
            className="space-y-6"
          >
            <div className="bg-slate-950/80 p-6 rounded-xl border border-slate-800 space-y-4">
              <h3 className="text-sm font-semibold text-slate-200 border-b border-slate-900 pb-2">Enabled Similarity Metric Algorithms</h3>

              <div className="flex items-center justify-between mb-2">
                <span className="text-[11px] text-slate-500">Select algorithms to use when scoring name similarity.</span>
                <div className="flex items-center gap-2">
                  <button
                    type="button"
                    onClick={selectAllAlgorithms}
                    className="text-[11px] text-sky-400 hover:text-sky-300 font-medium"
                  >
                    Select all
                  </button>
                  <button
                    type="button"
                    onClick={clearAllAlgorithms}
                    className="text-[11px] text-sky-400 hover:text-sky-300 font-medium"
                  >
                    Clear all
                  </button>
                </div>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3">
                {ALGO_LIST.map((algo) => {
                  const active = localCfg.algorithms?.[algo.key]
                  return (
                    <button
                      key={algo.key}
                      type="button"
                      onClick={() => toggleAlgo(algo.key)}
                      className={`p-3 text-left rounded-xl border transition flex items-start gap-3 ${
                        active ? 'bg-sky-950/40 border-sky-800/60 text-slate-100' : 'bg-slate-900/60 border-slate-800/60 text-slate-500'
                      }`}
                      aria-pressed={active}
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

              {allAlgosOff && (
                <div className="p-3 bg-rose-950/40 border border-rose-800/50 rounded-lg text-[11px] text-rose-300 flex items-center gap-2">
                <AlertTriangle className="w-4 h-4 shrink-0" />
                Name similarity cannot be computed with every algorithm disabled.
              </div>
            )}
          </div>

          <div className="bg-slate-900/60 p-5 rounded-xl border border-slate-800/80 space-y-5">
            <div>
              <h3 className="text-sm font-semibold text-slate-200 border-b border-slate-900 pb-2">Advanced Scorer Tuning</h3>
              <p className="text-[11px] text-slate-500">These tune the scorer directly and are best left at defaults unless you are actively calibrating.</p>
            </div>

            <div className="space-y-6">
              <div>
                <div className="flex justify-between text-xs font-medium text-slate-300 mb-2">
                  <span>Cross-Script Auto-Match Threshold</span>
                  <span className="font-mono text-sky-400 font-bold">{Math.round((localCfg.cross_script_auto_threshold ?? 0.84) * 100)}%</span>
                </div>
                <input
                  type="range"
                  min="0"
                  max="1"
                  step="0.01"
                  value={localCfg.cross_script_auto_threshold ?? 0.84}
                  aria-label="Cross-Script Auto-Match Threshold"
                  onChange={(e) => setLocalCfg({ ...localCfg, cross_script_auto_threshold: parseFloat(e.target.value) })}
                  className="w-full h-2 bg-slate-800 rounded-lg appearance-none cursor-pointer accent-sky-500"
                />
                <p className="text-[11px] text-slate-500 mt-1">The lower auto-match bar applied to Thai-vs-romanized pairs, which score systematically lower than same-script pairs. 0 means unset — use the main Auto-Match Threshold.</p>
              </div>

              <div>
                <div className="flex justify-between text-xs font-medium text-slate-300 mb-2">
                  <span>No-Distinctive-Overlap Score Cap</span>
                  <span className="font-mono text-sky-400 font-bold">{Math.round((localCfg.no_distinctive_overlap_cap ?? 0.85) * 100)}%</span>
                </div>
                <input
                  type="range"
                  min="0"
                  max="1"
                  step="0.01"
                  value={localCfg.no_distinctive_overlap_cap ?? 0.85}
                  aria-label="No-Distinctive-Overlap Score Cap"
                  onChange={(e) => setLocalCfg({ ...localCfg, no_distinctive_overlap_cap: parseFloat(e.target.value) })}
                  className="w-full h-2 bg-slate-800 rounded-lg appearance-none cursor-pointer accent-sky-500"
                />
                {capViolatesRule && (
                  <p className="text-[11px] text-rose-400 mt-1">Cap must be less than Auto-Match Threshold, otherwise it can never demote a match.</p>
                )}
                <p className="text-[11px] text-slate-500 mt-1">Ceiling applied to a pair that shares no distinctive token.</p>
              </div>

              <div>
                <div className="flex justify-between text-xs font-medium text-slate-300 mb-2">
                  <span>Distinctive Token IDF Floor</span>
                  <span className="font-mono text-sky-400 font-bold">{Math.round((localCfg.distinctive_overlap_min_weight ?? 0.30) * 100)}%</span>
                </div>
                <input
                  type="range"
                  min="0"
                  max="1"
                  step="0.01"
                  value={localCfg.distinctive_overlap_min_weight ?? 0.30}
                  aria-label="Distinctive Token IDF Floor"
                  onChange={(e) => setLocalCfg({ ...localCfg, distinctive_overlap_min_weight: parseFloat(e.target.value) })}
                  className="w-full h-2 bg-slate-800 rounded-lg appearance-none cursor-pointer accent-sky-500"
                />
                <p className="text-[11px] text-slate-500 mt-1">The corpus-IDF weight above which a shared token counts as identity evidence.</p>
              </div>
            </div>
          </div>
        </div>
        )}

        {/* Operations Tab */}
        {activeTab === 'Operations' && (
          <div
            id="configpanel-panel-Operations"
            role="tabpanel"
            aria-labelledby="configpanel-tab-Operations"
            className="space-y-6"
          >
            {can(user, 'SCHEDULER_CONFIG') && <SchedulerPanel />}
            {can(user, 'CALIBRATION') && <CalibrationPanel />}
          </div>
        )}
      </div>

      {/* Sticky Action Bar */}
      <div className="sticky bottom-0 z-10 -mx-8 -mb-8 px-8 py-4 bg-slate-950/95 border-t border-slate-800 backdrop-blur flex flex-col md:flex-row items-center justify-between gap-4">
        {/* Error/Success Banner Area */}
        {savedMessage && (
          <div className="w-full md:w-auto px-4 py-3 bg-emerald-950/80 border border-emerald-700/50 text-emerald-300 rounded-lg text-xs font-medium text-center md:text-left">
            Configuration updated successfully!
          </div>
        )}
        {saveError && (
          <div className="w-full md:w-auto px-4 py-3 bg-rose-950/80 border border-rose-700/50 text-rose-300 rounded-lg text-xs font-medium text-center md:text-left">
            {saveError}
          </div>
        )}

        {/* Buttons */}
        <div className="flex items-center gap-3">
          <button
            onClick={() => setLocalCfg(withDefaults(config))}
            disabled={loading || dirtyCount === 0}
            className="px-3.5 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg text-xs font-medium flex items-center gap-1.5 transition disabled:opacity-40"
          >
            <RotateCcw className="w-3.5 h-3.5" /> Reset
          </button>
          <button
            onClick={handleSave}
            disabled={loading || hasBlockingValidationError}
            className="px-4 py-2 bg-sky-600 hover:bg-sky-500 text-white rounded-lg text-xs font-semibold flex items-center gap-1.5 transition shadow-sm disabled:opacity-40"
          >
            {/* Save is deliberately not dirty-gated: ConnectionManager's onSettingsChange
                fires again on mount-time hydration of previously-saved connector settings
                (not just on user edits), and toEndpoint() deliberately omits the password
                field from that payload, so ConfigPanel can never reliably tell whether the
                connector side has a real unsaved edit. Gating Save here would either block
                legitimate saves (password-only edits) or wrongly enable itself on every
                load that has saved connector settings. The dirty badge above stays accurate
                because it only reasons about localCfg vs config. */}
            {loading ? (
              <>
                <svg className="w-4 h-4 animate-spin" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                <span>Save Configuration</span>
              </>
            ) : (
              <>
                <Save className="w-4 h-4" /> Save Configuration
              </>
            )}
          </button>
        </div>
      </div>
    </div>
  )
}
