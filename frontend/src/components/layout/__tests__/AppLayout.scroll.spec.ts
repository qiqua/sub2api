import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const testDir = dirname(fileURLToPath(import.meta.url))
const appLayoutSource = readFileSync(resolve(testDir, '../AppLayout.vue'), 'utf8')
const tablePageLayoutSource = readFileSync(resolve(testDir, '../TablePageLayout.vue'), 'utf8')

describe('AppLayout scroll containment', () => {
  it('pins the authenticated shell to the viewport without making the outer main area scroll', () => {
    expect(appLayoutSource).toContain('h-[100dvh]')
    expect(appLayoutSource).toContain('overflow-hidden bg-gray-50')
    expect(appLayoutSource).toContain('flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden')
    expect(appLayoutSource).toContain('min-h-0 flex-1 overflow-hidden')
    expect(appLayoutSource).not.toContain('overflow-y-auto p-4')
  })
})

describe('TablePageLayout scroll containment', () => {
  it('fills the AppLayout main area instead of creating a page-level vertical scroll', () => {
    expect(tablePageLayoutSource).toContain('@apply flex h-full min-h-0 flex-col gap-6 overflow-hidden;')
    expect(tablePageLayoutSource).toContain('@apply flex-1 min-h-0 flex flex-col overflow-hidden;')
    expect(tablePageLayoutSource).toContain('@apply h-full overflow-y-auto overflow-x-hidden pb-4;')
    expect(tablePageLayoutSource).not.toContain('calc(100vh')
  })
})
