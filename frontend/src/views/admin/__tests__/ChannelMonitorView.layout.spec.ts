import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const testDir = dirname(fileURLToPath(import.meta.url))
const source = readFileSync(resolve(testDir, '../ChannelMonitorView.vue'), 'utf8')

describe('ChannelMonitorView layout', () => {
  it('gives each monitor tab a scroll owner below the fixed header', () => {
    expect(source).toContain('flex h-full min-h-0 w-full min-w-0 flex-col gap-6')
    expect(source).toContain('page-header mb-0 shrink-0')
    expect(source).toContain('min-h-0 flex-1 overflow-y-auto overscroll-contain pb-8 pr-1')
    expect(source).toContain('<TablePageLayout v-else class="min-h-0 flex-1">')
  })
})
