import React, { useState } from 'react'
import { useMatcherStore } from '../store/useMatcherStore'
import { Cpu, Eye, EyeOff, Loader } from 'lucide-react'

const demoAccounts = [
  { username: 'admin', name: 'Admin User', role: 'ADMIN' },
  { username: 'engineer_alex', name: 'Engineer Alex', role: 'ENGINEER' },
  { username: 'reviewer_sarah', name: 'Reviewer Sarah', role: 'REVIEWER' },
  { username: 'auditor_mike', name: 'Auditor Mike', role: 'AUDITOR' },
]

export function LoginScreen() {
  const { login, loading } = useMatcherStore()
  const [username, setUsername] = useState('admin')
  const [password, setPassword] = useState('password')
  const [showPassword, setShowPassword] = useState(false)
  const [error, setError] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)

  const handleLogin = async (e) => {
    e.preventDefault()
    setError('')
    setIsSubmitting(true)

    const result = await login(username, password)
    if (!result.success) {
      setError(result.error || 'Login failed')
      setPassword('')
    }
    setIsSubmitting(false)
  }

  const handleDemoAccountClick = (account) => {
    setUsername(account.username)
    setPassword('password')
    setError('')
  }

  return (
    <div className="flex flex-col items-center justify-center h-screen bg-slate-950 text-slate-100 font-sans antialiased overflow-hidden">
      <div className="flex flex-col items-center gap-8 w-full max-w-md px-6">
        {/* Logo & Header */}
        <div className="flex flex-col items-center gap-4">
          <div className="p-3 bg-gradient-to-tr from-sky-600 to-emerald-500 text-white rounded-xl shadow-lg shadow-sky-950">
            <Cpu className="w-8 h-8" />
          </div>
          <div className="text-center">
            <h1 className="text-2xl font-bold tracking-tight">Entity Matcher</h1>
            <p className="text-xs text-slate-400 mt-1">High-Scale Bilingual Thai-English Resolution Engine</p>
          </div>
        </div>

        {/* Login Form Card */}
        <div className="w-full bg-slate-900/80 border border-slate-800 rounded-2xl p-8 space-y-6">
          <div>
            <h2 className="text-lg font-semibold text-slate-100">Sign In</h2>
            <p className="text-xs text-slate-400 mt-1">Enter your credentials to access the matching engine</p>
          </div>

          {error && (
            <div className="p-3 bg-rose-950/80 border border-rose-700/50 text-rose-300 rounded-lg text-xs font-medium">
              {error}
            </div>
          )}

          <form onSubmit={handleLogin} className="space-y-4">
            {/* Username Field */}
            <div>
              <label className="text-xs font-semibold text-slate-300 block mb-2">Username</label>
              <input
                type="text"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder="admin"
                disabled={isSubmitting || loading}
                className="w-full bg-slate-950 border border-slate-800 rounded-lg px-4 py-2.5 text-slate-100 placeholder-slate-500 focus:outline-none focus:border-sky-500 focus:ring-1 focus:ring-sky-500 disabled:opacity-50 disabled:cursor-not-allowed text-sm"
              />
            </div>

            {/* Password Field */}
            <div>
              <label className="text-xs font-semibold text-slate-300 block mb-2">Password</label>
              <div className="relative">
                <input
                  type={showPassword ? 'text' : 'password'}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="••••••••"
                  disabled={isSubmitting || loading}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg px-4 py-2.5 text-slate-100 placeholder-slate-500 focus:outline-none focus:border-sky-500 focus:ring-1 focus:ring-sky-500 disabled:opacity-50 disabled:cursor-not-allowed text-sm"
                />
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  disabled={isSubmitting || loading}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-300 disabled:opacity-50"
                >
                  {showPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                </button>
              </div>
            </div>

            {/* Submit Button */}
            <button
              type="submit"
              disabled={isSubmitting || loading}
              className="w-full py-2.5 bg-sky-600 hover:bg-sky-500 text-white rounded-lg text-sm font-semibold transition disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
            >
              {isSubmitting || loading ? (
                <>
                  <Loader className="w-4 h-4 animate-spin" />
                  Signing in...
                </>
              ) : (
                'Sign In'
              )}
            </button>
          </form>

          {/* Divider */}
          <div className="relative">
            <div className="absolute inset-0 flex items-center">
              <div className="w-full border-t border-slate-800"></div>
            </div>
            <div className="relative flex justify-center text-xs">
              <span className="px-2 bg-slate-900/80 text-slate-400">Demo accounts</span>
            </div>
          </div>

          {/* Demo Account Chips */}
          <div className="space-y-2">
            <p className="text-xs text-slate-400 text-center">Click to prefill form:</p>
            <div className="flex flex-wrap gap-2">
              {demoAccounts.map((account) => (
                <button
                  key={account.username}
                  type="button"
                  onClick={() => handleDemoAccountClick(account)}
                  disabled={isSubmitting || loading}
                  className="flex-1 min-w-max px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700 rounded-lg text-xs font-medium transition disabled:opacity-50 disabled:cursor-not-allowed flex flex-col items-center gap-0.5"
                >
                  <span className="font-semibold">{account.name}</span>
                  <span className="text-[10px] text-slate-400">{account.role}</span>
                </button>
              ))}
            </div>
          </div>
        </div>

        {/* Footer Note */}
        <p className="text-xs text-slate-500 text-center">
          Demo system with preset credentials. Use any demo account to explore the engine.
        </p>
      </div>
    </div>
  )
}
