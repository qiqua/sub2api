import { effectScope } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createBoard, type CanvasNode } from '../document'
import { useCanvasWorkspace } from '../useCanvasWorkspace'

vi.mock('../storage', () => ({
  listBoards: vi.fn(async () => [{ id: 'board', name: 'Draft' }]),
  loadBoard: vi.fn(async () => ({ ...createBoard('Draft'), id: 'board' })),
  saveBoard: vi.fn(async () => {}),
}))

afterEach(() => { localStorage.clear() })

describe('canvas editing around a paid request', () => {
  it('keeps the finished image when unrelated edits are undone and redone', async () => {
    const scope = effectScope()
    try {
      const editor = scope.run(() => useCanvasWorkspace(12, () => 'Draft'))!
      await editor.initialize()
      const prompt: CanvasNode = { id: 'prompt', text: 'A bridge', kind: 'prompt', x: 0, y: 0 }
      const output: CanvasNode = { id: 'result', text: prompt.text, kind: 'image', parentId: prompt.id, x: 286, y: 0 }
      editor.setNodes([prompt])
      editor.startResult(output)
      editor.patchResult(output.id, { job: { id: 'job', keyId: 19 } })
      editor.setNodes([{ ...prompt, text: 'A new idea' }, editor.board.value!.nodes[1]])
      const image = new Blob(['result'], { type: 'image/png' })
      editor.patchResult(output.id, { image, job: undefined })
      editor.undo()
      expect(editor.board.value?.nodes).toEqual([prompt, { ...output, image, job: undefined }])
      expect(editor.canUndo.value).toBe(false)
      editor.redo()
      expect(editor.board.value?.nodes[0].text).toBe('A new idea')
      expect(editor.board.value?.nodes[1].image).toBe(image)
      expect(editor.board.value?.nodes[1].job).toBeUndefined()
    } finally { scope.stop() }
  })
})
