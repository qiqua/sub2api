import { readdirSync, readFileSync, statSync } from 'node:fs'
import { dirname, extname, join, relative, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const testDir = dirname(fileURLToPath(import.meta.url))
const viewsDir = resolve(testDir, '../../../views')

function collectVueFiles(dir: string): string[] {
  const files: string[] = []
  for (const entry of readdirSync(dir)) {
    const fullPath = join(dir, entry)
    if (statSync(fullPath).isDirectory()) {
      files.push(...collectVueFiles(fullPath))
    } else if (extname(fullPath) === '.vue') {
      files.push(fullPath)
    }
  }
  return files
}

describe('AppLayout page scroll ownership', () => {
  it('requires every AppLayout view to own its scroll area explicitly', () => {
    const offenders = collectVueFiles(viewsDir)
      .filter((file) => {
        const source = readFileSync(file, 'utf8')
        if (!source.includes('AppLayout')) return false
        if (source.includes('<TablePageLayout')) return false
        return !source.includes('app-page-scroll') && !source.includes('app-page-fixed')
      })
      .map((file) => relative(viewsDir, file))

    expect(offenders).toEqual([])
  })

  it('prevents nested page-level scroll containers inside a single AppLayout view', () => {
    const offenders = collectVueFiles(viewsDir)
      .filter((file) => {
        const source = readFileSync(file, 'utf8')
        if (!source.includes('AppLayout')) return false
        if (source.includes('<TablePageLayout')) return false
        const ownerCount = source.match(/app-page-(?:scroll|fixed)/g)?.length ?? 0
        return ownerCount !== 1
      })
      .map((file) => relative(viewsDir, file))

    expect(offenders).toEqual([])
  })
})
