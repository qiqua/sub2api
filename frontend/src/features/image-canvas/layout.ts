import type { CanvasNode, Point } from './document'

const columnGap = 286
const rowGap = 208

export function vacantPoint(nodes: CanvasNode[], point: Point): Point {
  let y = point.y
  while (nodes.some(node => Math.abs(node.x - point.x) < 232 && Math.abs(node.y - y) < 174)) y += rowGap
  return { x: point.x, y }
}

export function branchPoint(nodes: CanvasNode[], parent: CanvasNode) {
  return vacantPoint(nodes, { x: parent.x + columnGap, y: parent.y })
}

// A reference can be an ancestor or a different branch, but never the prompt's own result.
export function referenceChoices(nodes: CanvasNode[], promptId: string) {
  const byId = new Map(nodes.map(node => [node.id, node]))
  return nodes.filter(node => {
    if (!node.image) return false
    let parentId = node.parentId
    while (parentId) {
      if (parentId === promptId) return false
      parentId = byId.get(parentId)?.parentId
    }
    return true
  })
}

export function arrangeNodes(nodes: CanvasNode[]): CanvasNode[] {
  const byId = new Map(nodes.map(node => [node.id, node]))
  const rows = new Map<number, number>()
  return nodes.map(node => {
    let depth = 0
    let parentId = node.parentId
    while (parentId && byId.has(parentId)) {
      depth++
      parentId = byId.get(parentId)?.parentId
    }
    const row = rows.get(depth) ?? 0
    rows.set(depth, row + 1)
    return { ...node, x: depth * columnGap, y: row * rowGap }
  })
}
