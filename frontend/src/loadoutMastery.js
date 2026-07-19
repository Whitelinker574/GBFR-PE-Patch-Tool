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

function masteryNode(nodeByHash, hash) {
  if (nodeByHash instanceof Map) return nodeByHash.get(hash)
  return nodeByHash?.[hash]
}

export function isMasteryNodeSelectable(rank, node, direction) {
  if (!node) return false
  if (!['R2', 'R3'].includes(rank) || !node.specialization) return true
  return Boolean(direction && node.cat === direction)
}

export function applyMasteryDirection(picks = {}, direction, nodeByHash) {
  const next = {}
  for (const rank of ['R1', 'R2', 'R3', 'EX']) {
    const selected = [...(picks[rank] || [])]
    next[rank] = !['R2', 'R3'].includes(rank)
      ? selected
      : selected.filter(hash => {
          const node = masteryNode(nodeByHash, hash)
          return !node?.specialization || node.cat === direction
        })
  }
  return next
}

export function inferMasteryDirection(picks = {}, nodeByHash) {
  const counts = Object.fromEntries(CATEGORY_ORDER.map(cat => [cat, 0]))
  for (const hash of picks.R2 || []) {
    const node = masteryNode(nodeByHash, hash)
    if (node?.cat in counts) counts[node.cat] += 1
  }
  const thresholdDirection = CATEGORY_ORDER
    .map(cat => ({ cat, count: counts[cat] }))
    .filter(item => item.count >= 6)
    .sort((a, b) => b.count - a.count)[0]?.cat
  if (thresholdDirection) return thresholdDirection

  for (const rank of ['R2', 'R3']) {
    for (const hash of picks[rank] || []) {
      const node = masteryNode(nodeByHash, hash)
      if (node?.specialization && CATEGORY_ORDER.includes(node.cat)) return node.cat
    }
  }
  return ''
}
