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

// Keep the editable draft intact, but expose only the nodes covered by the
// character's current save-backed mastery rank caps to calculators/summaries.
export function limitMasteryHashesByRankCaps(hashes = [], nodeByHash, rankCaps = {}) {
  const used = { R1: 0, R2: 0, R3: 0, EX: 0 }
  const effective = []
  for (const hash of hashes) {
    const rank = masteryNode(nodeByHash, hash)?.rank
    if (!(rank in used)) continue
    const cap = Math.max(0, Number(rankCaps?.[rank] || 0))
    if (used[rank] >= cap) continue
    used[rank] += 1
    effective.push(hash)
  }
  return effective
}

export function isMasteryNodeSelectable(rank, node, direction) {
  void rank
  void direction
  return Boolean(node)
}

export function applyMasteryDirection(picks = {}, direction, nodeByHash) {
  void direction
  void nodeByHash
  return Object.fromEntries(['R1', 'R2', 'R3', 'EX'].map(rank => [rank, [...(picks[rank] || [])]]))
}

export function inferMasteryDirection(picks = {}, nodeByHash) {
  const counts = Object.fromEntries(CATEGORY_ORDER.map(cat => [cat, 0]))
  for (const hash of picks.R2 || []) {
    const node = masteryNode(nodeByHash, hash)
    if (node?.cat in counts) counts[node.cat] += 1
  }
  const thresholdDirections = CATEGORY_ORDER
    .map(cat => ({ cat, count: counts[cat] }))
    .filter(item => item.count >= 6)
    .sort((a, b) => b.count - a.count)
  if (thresholdDirections.length === 1) return thresholdDirections[0].cat
  return ''
}
