// Velero Backup/Restore phase normalization.
// Velero canonical phases: New, FailedValidation, InProgress, WaitingForPluginOperations,
// WaitingForPluginOperationsPartiallyFailed, Finalizing, FinalizingPartiallyFailed,
// Completed, PartiallyFailed, Failed, Deleting.
// Anything else (null/undefined/empty/unrecognized) is rendered as 'Unknown'.

const KNOWN_PHASES = new Set([
  'New',
  'FailedValidation',
  'InProgress',
  'WaitingForPluginOperations',
  'WaitingForPluginOperationsPartiallyFailed',
  'Finalizing',
  'FinalizingPartiallyFailed',
  'Completed',
  'PartiallyFailed',
  'Failed',
  'Deleting'
])

export const normalizePhase = (phase) => {
  if (phase === null || phase === undefined) return 'Unknown'
  const trimmed = String(phase).trim()
  if (trimmed === '') return 'Unknown'
  if (KNOWN_PHASES.has(trimmed)) return trimmed
  return 'Unknown'
}

export const phaseTagType = (phase) => {
  const normalized = normalizePhase(phase)
  const map = {
    Completed: 'success',
    InProgress: 'warning',
    WaitingForPluginOperations: 'warning',
    Finalizing: 'warning',
    New: 'info',
    Failed: 'danger',
    FailedValidation: 'danger',
    PartiallyFailed: 'warning',
    WaitingForPluginOperationsPartiallyFailed: 'warning',
    FinalizingPartiallyFailed: 'warning',
    Deleting: 'info',
    Unknown: 'info'
  }
  return map[normalized] || 'info'
}
