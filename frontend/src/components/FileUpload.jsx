import React, { useState, useEffect } from 'react'
import { useMatcherStore } from '../store/useMatcherStore'
import { FieldMapper } from './FieldMapper'
import { Upload, Database, FileText, CheckCircle2, Play, Settings } from 'lucide-react'

export function FileUpload() {
  const { uploadFiles, loadSeedDataset, loadBigSeedDataset, runMatching, loading, error, config } = useMatcherStore()
  const [sourcesText, setSourcesText] = useState('')
  const [destsText, setDestsText] = useState('')
  const [detectedSrcCols, setDetectedSrcCols] = useState([])
  const [detectedDestCols, setDetectedDestCols] = useState([])
  const [showMapper, setShowMapper] = useState(false)
  const [activeMapping, setActiveMapping] = useState(config.column_mapping)

  // Auto-detect JSON keys whenever text changes
  useEffect(() => {
    try {
      if (sourcesText.trim()) {
        const parsed = JSON.parse(sourcesText)
        if (Array.isArray(parsed) && parsed.length > 0) {
          setDetectedSrcCols(Object.keys(parsed[0]))
        }
      }
    } catch (e) {}
  }, [sourcesText])

  useEffect(() => {
    try {
      if (destsText.trim()) {
        const parsed = JSON.parse(destsText)
        if (Array.isArray(parsed) && parsed.length > 0) {
          setDetectedDestCols(Object.keys(parsed[0]))
        }
      }
    } catch (e) {}
  }, [destsText])

  const handleCustomUpload = async () => {
    try {
      let sources = []
      let dests = []

      if (sourcesText.trim()) {
        sources = JSON.parse(sourcesText)
      }
      if (destsText.trim()) {
        dests = JSON.parse(destsText)
      }

      if (sources.length === 0 || dests.length === 0) {
        alert('Please provide valid JSON arrays for both Source and Destination records.')
        return
      }

      const batchId = await uploadFiles({
        sources,
        destinations: dests,
        column_mapping: activeMapping,
      })

      await runMatching(batchId)
    } catch (e) {
      alert('Failed to upload records: ' + e.message)
    }
  }

  return (
    <div className="max-w-4xl mx-auto space-y-6 bg-slate-900/60 p-8 rounded-2xl border border-slate-800">
      <div className="border-b border-slate-800 pb-4">
        <h2 className="text-xl font-bold text-slate-100 flex items-center gap-2">
          <Upload className="w-5 h-5 text-sky-400" /> Data Ingestion & Schema Mapper
        </h2>
        <p className="text-xs text-slate-400 mt-1">Upload dynamic datasets, configure single or multiple pairing fields, and run benchmarks.</p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {/* Standard Benchmark Card */}
        <div className="bg-gradient-to-r from-sky-950/60 to-purple-950/60 p-5 rounded-xl border border-sky-800/40 flex flex-col justify-between space-y-4">
          <div>
            <h3 className="text-sm font-bold text-sky-200 flex items-center gap-2">
              <Database className="w-4 h-4 text-sky-400" /> Standard Benchmark (58 Records)
            </h3>
            <p className="text-xs text-slate-400 mt-1">Includes transposed names, honorifics, corporate suffixes, and date deltas.</p>
          </div>
          <button
            onClick={loadSeedDataset}
            disabled={loading}
            className="w-full py-2.5 bg-sky-600 hover:bg-sky-500 text-white rounded-lg text-xs font-semibold flex items-center justify-center gap-2 transition shadow-lg shadow-sky-900/30"
          >
            <Play className="w-4 h-4 fill-white" /> Run Standard Test Suite
          </button>
        </div>

        {/* High-Scale Benchmark Card */}
        <div className="bg-gradient-to-r from-purple-950/60 to-emerald-950/60 p-5 rounded-xl border border-purple-800/40 flex flex-col justify-between space-y-4">
          <div>
            <h3 className="text-sm font-bold text-purple-200 flex items-center gap-2">
              <Database className="w-4 h-4 text-purple-400" /> Big Mock Benchmark (4,000 Records)
            </h3>
            <p className="text-xs text-slate-400 mt-1">Full-loop stress test over 16 Million potential pair combinations with ground truth validation.</p>
          </div>
          <button
            onClick={loadBigSeedDataset}
            disabled={loading}
            className="w-full py-2.5 bg-purple-600 hover:bg-purple-500 text-white rounded-lg text-xs font-semibold flex items-center justify-center gap-2 transition shadow-lg shadow-purple-900/30"
          >
            <Play className="w-4 h-4 fill-white" /> Run 4,000-Record High-Scale Test
          </button>
        </div>
      </div>

      {/* Custom JSON Paste Section */}
      <div className="bg-slate-950/80 p-6 rounded-xl border border-slate-800 space-y-4">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-semibold text-slate-200">Custom Dataset Payload & Column Mapper</h3>
          <button
            type="button"
            onClick={() => setShowMapper(!showMapper)}
            className="px-3 py-1.5 bg-slate-900 hover:bg-slate-800 text-sky-400 border border-slate-800 rounded-lg text-xs font-medium flex items-center gap-1.5 transition"
          >
            <Settings className="w-3.5 h-3.5" /> {showMapper ? 'Hide Schema Mapper' : 'Configure Schema Mapping'}
          </button>
        </div>

        {/* Collapsible Field Mapper */}
        {showMapper && (
          <FieldMapper
            availableSourceCols={detectedSrcCols}
            availableDestCols={detectedDestCols}
            onMappingSaved={(m) => setActiveMapping(m)}
          />
        )}

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <div className="flex justify-between items-center mb-1.5">
              <label className="text-xs text-slate-400 block font-mono">Source Records (JSON Array)</label>
              {detectedSrcCols.length > 0 && (
                <span className="text-[10px] font-mono text-sky-400">Cols: {detectedSrcCols.join(', ')}</span>
              )}
            </div>
            <textarea
              rows={8}
              value={sourcesText}
              onChange={(e) => setSourcesText(e.target.value)}
              placeholder={`[\n  {\n    "first_name": "สมชาย",\n    "last_name": "เข็มกลัด",\n    "reference_id": "SRC-001",\n    "tax_id": "123456789"\n  }\n]`}
              className="w-full bg-slate-900 border border-slate-800 rounded-lg p-3 text-xs font-mono text-slate-200 focus:outline-none focus:border-sky-500"
            />
          </div>

          <div>
            <div className="flex justify-between items-center mb-1.5">
              <label className="text-xs text-slate-400 block font-mono">Destination Records (JSON Array)</label>
              {detectedDestCols.length > 0 && (
                <span className="text-[10px] font-mono text-purple-400">Cols: {detectedDestCols.join(', ')}</span>
              )}
            </div>
            <textarea
              rows={8}
              value={destsText}
              onChange={(e) => setDestsText(e.target.value)}
              placeholder={`[\n  {\n    "vendor_first": "เข็มกลัด",\n    "vendor_last": "สมชาย",\n    "customer_id": "DEST-001",\n    "registration_num": "123456789"\n  }\n]`}
              className="w-full bg-slate-900 border border-slate-800 rounded-lg p-3 text-xs font-mono text-slate-200 focus:outline-none focus:border-sky-500"
            />
          </div>
        </div>

        <div className="flex justify-end pt-2">
          <button
            onClick={handleCustomUpload}
            disabled={loading}
            className="px-5 py-2.5 bg-slate-800 hover:bg-slate-700 text-slate-100 border border-slate-700 rounded-lg text-xs font-semibold flex items-center gap-2 transition"
          >
            <Upload className="w-4 h-4" /> Stream Ingest & Execute Matching
          </button>
        </div>
      </div>
    </div>
  )
}
