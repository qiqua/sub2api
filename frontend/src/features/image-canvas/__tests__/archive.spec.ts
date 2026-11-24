// @vitest-environment node
import { describe, expect, it } from 'vitest'
import { createBoard } from '../document'
import { exportBoard, importBoard } from '../storage'

const prompt = { id: 'prompt-1', kind: 'prompt' as const, text: '初雪', x: -32.5, y: 71 }
const archive = { format: 'sub2api-image-canvas', version: 1, name: 'Sketch', viewport: { x: -5, y: 6, zoom: 0.5 }, nodes: [prompt] }

describe('canvas JSON archives', () => {
  it('preserves editable content, allocates a fresh board ID and omits local-only fields', async () => {
    const board = { ...createBoard(archive.name), viewport: archive.viewport, nodes: [{ ...prompt, error: 'local diagnostic', apiKey: 'private-value' }] }
    const file = await exportBoard(board)
    expect(JSON.parse(await file.text())).toEqual(archive)
    const restored = await importBoard(file)
    expect(restored.id).not.toBe(board.id)
    expect(restored).toEqual({ name: archive.name, viewport: archive.viewport, nodes: [prompt], id: restored.id })
  })

  it.each([
    ['job ownership', [{ ...prompt, job: { id: 'job-1', keyId: 7 } }], 'invalid-document'],
    ['missing parent', [{ ...prompt, parentId: 'absent' }], 'invalid-document'],
    ['duplicate ID', [prompt, prompt], 'invalid-document'],
    ['invalid coordinate', [{ ...prompt, x: null }], 'invalid-document'],
    ['text limit', [{ ...prompt, text: 'x'.repeat(32_001) }], 'limit-exceeded'],
    ['SVG declared as PNG', [{ ...prompt, kind: 'image', image: { type: 'image/png', data: 'PHN2Zy8+' } }], 'invalid-image'],
    ['malformed base64', [{ ...prompt, kind: 'image', image: { type: 'image/jpeg', data: 'ab=c' } }], 'invalid-image'],
    ['external image URL', [{ ...prompt, kind: 'image', image: { url: 'https://example.invalid/image.png' } }], 'invalid-document'],
  ])('rejects %s', async (_case, nodes, code) => {
    await expect(importBoard(new Blob([JSON.stringify({ ...archive, nodes })]))).rejects.toMatchObject({ code })
  })

  it.each([{}, { job: { id: 'job-1', keyId: 7 } }, { error: 'Generation failed' }])('identifies an unfinished image node on export', async state => {
    const board = createBoard('Draft')
    board.nodes.push({ ...prompt, kind: 'image', ...state })
    await expect(exportBoard(board)).rejects.toMatchObject({ code: 'unfinished-node', nodeId: prompt.id })
  })
})
