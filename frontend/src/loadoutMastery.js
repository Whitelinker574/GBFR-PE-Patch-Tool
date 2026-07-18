const CATEGORY_ORDER = ['SB_ATK', 'SB_DEF', 'SB_LIMIT']
const CATEGORY_LABELS = { SB_ATK: '真谛', SB_DEF: '觉醒', SB_LIMIT: '秘义' }

export function groupMasteryNodes(nodes = []) {
  return CATEGORY_ORDER.map(cat => ({
    cat,
    label: CATEGORY_LABELS[cat],
    nodes: nodes.filter(node => node.cat === cat),
  }))
}

export function resolveMasteryHashes({ mode, picks = {}, sourceId = 0, sources = [] }) {
  if (mode === 'copy') {
    const source = sources.find(item => item.unitId === sourceId)
    return source ? [...(source.nodeHashes || [])] : []
  }
  return ['R1', 'R2', 'R3', 'EX'].flatMap(rank => picks[rank] || [])
}
