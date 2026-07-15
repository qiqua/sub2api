import { beforeEach, describe, expect, it } from 'vitest'
import {
  acquireBodyScrollLock,
  getBodyScrollLockCount,
  releaseBodyScrollLock,
  resetBodyScrollLocks
} from '../useBodyScrollLock'

describe('useBodyScrollLock', () => {
  beforeEach(() => {
    resetBodyScrollLocks()
    document.body.removeAttribute('style')
    document.documentElement.removeAttribute('style')
  })

  it('keeps the page locked until every active lock is released', () => {
    const first = Symbol('first')
    const second = Symbol('second')

    document.body.style.overflow = 'auto'
    document.documentElement.style.overflow = 'clip'

    acquireBodyScrollLock(first)
    acquireBodyScrollLock(second)

    expect(getBodyScrollLockCount()).toBe(2)
    expect(document.body.style.overflow).toBe('hidden')
    expect(document.documentElement.style.overflow).toBe('hidden')

    releaseBodyScrollLock(first)

    expect(getBodyScrollLockCount()).toBe(1)
    expect(document.body.style.overflow).toBe('hidden')
    expect(document.documentElement.style.overflow).toBe('hidden')

    releaseBodyScrollLock(second)

    expect(getBodyScrollLockCount()).toBe(0)
    expect(document.body.style.overflow).toBe('auto')
    expect(document.documentElement.style.overflow).toBe('clip')
  })

  it('does not count the same token twice', () => {
    const token = Symbol('same-token')

    acquireBodyScrollLock(token)
    acquireBodyScrollLock(token)

    expect(getBodyScrollLockCount()).toBe(1)

    releaseBodyScrollLock(token)

    expect(getBodyScrollLockCount()).toBe(0)
    expect(document.body.style.overflow).toBe('')
    expect(document.documentElement.style.overflow).toBe('')
  })
})
