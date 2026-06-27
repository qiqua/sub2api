import {
  getConfiguredTableDefaultPageSize,
  getConfiguredTablePageSizeOptions,
  normalizeTablePageSize
} from '@/utils/tablePreferences'

const STORAGE_KEY = 'table-page-size'
const STORAGE_CONFIG_KEY = 'table-page-size-config'

const getTablePageSizeConfigSignature = (): string => {
  return JSON.stringify({
    default: getConfiguredTableDefaultPageSize(),
    options: getConfiguredTablePageSizeOptions()
  })
}

export function getPersistedPageSize(fallback = getConfiguredTableDefaultPageSize()): number {
  if (typeof window !== 'undefined' && window.__APP_CONFIG__?.table_default_page_size !== undefined) {
    return normalizeTablePageSize(getConfiguredTableDefaultPageSize())
  }

  if (typeof window !== 'undefined') {
    try {
      const stored = window.localStorage.getItem(STORAGE_KEY)
      const storedConfig = window.localStorage.getItem(STORAGE_CONFIG_KEY)
      if (stored !== null) {
        const parsed = Number(stored)
        if (Number.isFinite(parsed) && storedConfig === getTablePageSizeConfigSignature()) {
          return normalizeTablePageSize(parsed)
        }
      }
    } catch (error) {
      console.warn('Failed to read persisted page size:', error)
    }
  }
  return normalizeTablePageSize(getConfiguredTableDefaultPageSize() || fallback)
}

export function setPersistedPageSize(size: number): void {
  if (typeof window === 'undefined') return
  try {
    window.localStorage.setItem(STORAGE_KEY, String(size))
    window.localStorage.setItem(STORAGE_CONFIG_KEY, getTablePageSizeConfigSignature())
  } catch (error) {
    console.warn('Failed to persist page size:', error)
  }
}
