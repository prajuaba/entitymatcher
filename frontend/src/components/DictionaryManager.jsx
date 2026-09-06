import React, { useState, useEffect } from 'react'
import { BookOpen, Plus, Trash2, CheckCircle2 } from 'lucide-react'
import { apiFetch, readErrorMessage } from '../lib/api.js'

export function DictionaryManager() {
  const [entries, setEntries] = useState([])
  const [newAlias, setNewAlias] = useState('')
  const [newCanonical, setNewCanonical] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)

  const fetchDictionary = async () => {
    setLoading(true)
    try {
      const res = await apiFetch('/api/dictionary')
      const data = await res.json()
      setEntries(data.entries || [])
    } catch (e) {
      console.error('Failed to fetch dictionary:', e)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    fetchDictionary()
  }, [])

  const handleAddSynonym = async (e) => {
    e.preventDefault()
    setError(null)
    if (!newAlias.trim() || !newCanonical.trim()) return

    try {
      const res = await apiFetch('/api/dictionary', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ alias: newAlias.trim(), canonical: newCanonical.trim() }),
      })
      // Same plain-text failure body as the delete path above.
      if (!res.ok) {
        setError(await readErrorMessage(res, 'Failed to add alias'))
        return
      }
      const data = await res.json()
      setEntries(data.entries || [])
      setNewAlias('')
      setNewCanonical('')
    } catch (e) {
      console.error('Failed to add synonym:', e)
      setError(e.message || 'Failed to add alias.')
    }
  }

  const handleDeleteSynonym = async (alias) => {
    setError(null)
    try {
      const res = await apiFetch(`/api/dictionary?alias=${encodeURIComponent(alias)}`, {
        method: 'DELETE',
      })
      // The backend reports failures with http.Error, which writes PLAIN TEXT.
      // Parsing the body as JSON before checking res.ok would throw on exactly
      // the responses whose message the operator needs to read, and surface a
      // parser error in place of the reason -- the same trap the calibration
      // panel hit. Read the body as text on the failure path.
      if (!res.ok) {
        setError(await readErrorMessage(res, 'Failed to delete alias'))
        return
      }
      // The server's list is authoritative: reflect exactly what it returns
      // rather than filtering the local array ourselves.
      const data = await res.json()
      setEntries(data.entries || [])
    } catch (e) {
      console.error('Failed to delete synonym:', e)
      setError(e.message || 'Failed to delete alias.')
    }
  }

  return (
    <div className="space-y-4 bg-slate-950/80 p-6 rounded-xl border border-slate-800">
      <div className="border-b border-slate-900 pb-3">
        <h3 className="text-sm font-bold text-slate-100 flex items-center gap-2">
          <BookOpen className="w-4 h-4 text-sky-400" /> Enterprise Brand Synonym & Alias Manager
        </h3>
        <p className="text-xs text-slate-400 mt-0.5">Map custom brand aliases (e.g. KBank ➔ Kasikornbank, SCB ➔ Siam Commercial Bank) for pre-normalizer replacement.</p>
      </div>

      {/* Form: Add New Synonym */}
      <form onSubmit={handleAddSynonym} className="flex flex-wrap items-center gap-2 text-xs bg-slate-900/60 p-3 rounded-lg border border-slate-800">
        <input
          type="text"
          placeholder="Alias (e.g. KBank or กสิกร)"
          value={newAlias}
          onChange={(e) => setNewAlias(e.target.value)}
          className="bg-slate-950 border border-slate-800 rounded p-2 text-slate-200 font-mono"
        />
        <span className="text-slate-500 font-bold">➔</span>
        <input
          type="text"
          placeholder="Canonical Name (e.g. Kasikornbank)"
          value={newCanonical}
          onChange={(e) => setNewCanonical(e.target.value)}
          className="flex-1 bg-slate-950 border border-slate-800 rounded p-2 text-slate-200"
        />
        <button
          type="submit"
          className="px-3.5 py-2 bg-sky-600 hover:bg-sky-500 text-white rounded font-semibold flex items-center gap-1 transition"
        >
          <Plus className="w-3.5 h-3.5" /> Add Alias
        </button>
      </form>

      {error && (
        <div className="p-2.5 rounded text-xs flex items-center gap-2 bg-rose-950/60 text-rose-300 border border-rose-800">
          <span>⚠</span> {error}
        </div>
      )}

      {/* Synonym List Chips */}
      <div className="flex flex-wrap gap-2 pt-2">
        {entries.map((item, idx) => (
          <div key={idx} className="px-3 py-1.5 bg-slate-900 border border-slate-800 rounded-lg text-xs flex items-center gap-2">
            <span className="font-mono text-sky-300 font-bold">{item.alias}</span>
            <span className="text-slate-500">➔</span>
            <span className="text-slate-200">{item.canonical}</span>
            <button
              type="button"
              onClick={() => handleDeleteSynonym(item.alias)}
              aria-label={`Remove alias ${item.alias}`}
              className="text-slate-500 hover:text-rose-400 transition"
            >
              <Trash2 className="w-3 h-3" />
            </button>
          </div>
        ))}
      </div>
    </div>
  )
}
