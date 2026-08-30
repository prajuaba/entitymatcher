// Role-based access control helper
const capabilities = {
  REVIEW_QUEUE: ['ADMIN', 'ENGINEER', 'REVIEWER', 'AUDITOR'],
  EXECUTION_DASHBOARD: ['ADMIN', 'ENGINEER', 'REVIEWER'],
  AUDIT_TRAIL: ['ADMIN', 'AUDITOR'],
  ENGINE_CONFIG: ['ADMIN', 'ENGINEER'],
  SCHEDULER_CONFIG: ['ADMIN'],
  INGESTION_BENCHMARKS: ['ADMIN', 'ENGINEER'],
  CONFIRM_MATCH: ['ADMIN', 'REVIEWER'],
  REJECT_MATCH: ['ADMIN', 'REVIEWER'],
  MANUAL_LINK: ['ADMIN', 'REVIEWER'],
}

export function can(user, capability) {
  if (!user) return false
  const allowedRoles = capabilities[capability]
  if (!allowedRoles) return false
  return allowedRoles.includes(user.role)
}

export function getRoleColor(role) {
  switch (role) {
    case 'ADMIN':
      return 'bg-rose-950/80 text-rose-300 border-rose-700/50'
    case 'ENGINEER':
      return 'bg-sky-950/80 text-sky-300 border-sky-700/50'
    case 'REVIEWER':
      return 'bg-emerald-950/80 text-emerald-300 border-emerald-700/50'
    case 'AUDITOR':
      return 'bg-amber-950/80 text-amber-300 border-amber-700/50'
    default:
      return 'bg-slate-800 text-slate-300 border-slate-700'
  }
}
