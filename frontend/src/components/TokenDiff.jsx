import React from 'react'

export function TokenDiff({ sourceName, candidateName }) {
  if (!sourceName || !candidateName) {
    return <div className="text-slate-400 text-sm">{sourceName || candidateName || '-'}</div>
  }

  // Tokenize words
  const srcTokens = sourceName.toLowerCase().split(/\s+/).filter(Boolean)
  const candTokens = candidateName.toLowerCase().split(/\s+/).filter(Boolean)

  const srcOriginal = sourceName.split(/\s+/)
  const candOriginal = candidateName.split(/\s+/)

  const isSrcTokenMatched = (token) => candTokens.some(ct => ct.includes(token.toLowerCase()) || token.toLowerCase().includes(ct))
  const isCandTokenMatched = (token) => srcTokens.some(st => st.includes(token.toLowerCase()) || token.toLowerCase().includes(st))

  return (
    <div className="space-y-2 text-sm">
      <div>
        <span className="text-xs uppercase tracking-wider text-slate-400 font-semibold block mb-1">Source Record Name</span>
        <div className="flex flex-wrap gap-1 p-2 bg-slate-900 rounded-lg border border-slate-800">
          {srcOriginal.map((tok, i) => {
            const matched = isSrcTokenMatched(tok)
            return (
              <span
                key={i}
                className={`px-2 py-0.5 rounded text-xs font-mono transition-colors ${
                  matched
                    ? 'bg-emerald-950/80 text-emerald-300 border border-emerald-700/50 font-semibold'
                    : 'bg-amber-950/60 text-amber-300 border border-amber-800/40'
                }`}
              >
                {tok}
              </span>
            )
          })}
        </div>
      </div>

      <div>
        <span className="text-xs uppercase tracking-wider text-slate-400 font-semibold block mb-1">Destination Candidate Name</span>
        <div className="flex flex-wrap gap-1 p-2 bg-slate-900 rounded-lg border border-slate-800">
          {candOriginal.map((tok, i) => {
            const matched = isCandTokenMatched(tok)
            return (
              <span
                key={i}
                className={`px-2 py-0.5 rounded text-xs font-mono transition-colors ${
                  matched
                    ? 'bg-emerald-950/80 text-emerald-300 border border-emerald-700/50 font-semibold'
                    : 'bg-slate-800 text-slate-400 border border-slate-700'
                }`}
              >
                {tok}
              </span>
            )
          })}
        </div>
      </div>
    </div>
  )
}
