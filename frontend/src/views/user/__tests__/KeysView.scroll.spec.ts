import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const viewPath = resolve(dirname(fileURLToPath(import.meta.url)), '../KeysView.vue')
const viewSource = readFileSync(viewPath, 'utf8')

describe('KeysView table scrolling', () => {
  it('keeps the shared TablePageLayout vertical scroll chain intact', () => {
    // KeysView used to override the shared flex scroll container with
    // flex:none/overflow-y:visible, while AppLayout intentionally clips its
    // main region. That made rows below the viewport unreachable.
    expect(viewSource).not.toMatch(/\.keys-page-layout\s+:deep\(\.layout-section-scrollable\)/)
    expect(viewSource).not.toMatch(/\.keys-page-layout\s+:deep\(\.table-scroll-container\)/)
    expect(viewSource).not.toMatch(/\.keys-page-layout\s+:deep\(\.table-wrapper\)/)
  })
})
