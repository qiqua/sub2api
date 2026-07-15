const activeLocks = new Set<symbol>()

let previousBodyOverflow: string | null = null
let previousDocumentOverflow: string | null = null

function canUseDOM(): boolean {
  return typeof document !== 'undefined'
}

function rememberOverflow() {
  if (!canUseDOM()) return

  if (previousBodyOverflow === null) {
    previousBodyOverflow = document.body.style.overflow
  }
  if (previousDocumentOverflow === null) {
    previousDocumentOverflow = document.documentElement.style.overflow
  }
}

function applyLock() {
  if (!canUseDOM()) return

  document.body.style.overflow = 'hidden'
  document.documentElement.style.overflow = 'hidden'
}

function restoreOverflow() {
  if (!canUseDOM()) return

  document.body.style.overflow = previousBodyOverflow ?? ''
  document.documentElement.style.overflow = previousDocumentOverflow ?? ''
  previousBodyOverflow = null
  previousDocumentOverflow = null
}

export function acquireBodyScrollLock(lockToken: symbol) {
  if (!canUseDOM() || activeLocks.has(lockToken)) return

  if (activeLocks.size === 0) {
    rememberOverflow()
  }

  activeLocks.add(lockToken)
  applyLock()
}

export function releaseBodyScrollLock(lockToken: symbol) {
  if (!canUseDOM() || !activeLocks.has(lockToken)) return

  activeLocks.delete(lockToken)

  if (activeLocks.size === 0) {
    restoreOverflow()
  }
}

export function resetBodyScrollLocks() {
  activeLocks.clear()
  restoreOverflow()
}

export function getBodyScrollLockCount(): number {
  return activeLocks.size
}
