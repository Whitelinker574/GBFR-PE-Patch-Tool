function normalizedText(value) {
  return String(value || '').trim().toLocaleLowerCase()
}

export function buildConstructCatalog(naturalItems, bagItems) {
  void bagItems
  return [...(naturalItems || [])]
}

export function filterConstructCatalog(items, query) {
  const needle = normalizedText(query)
  if (!needle) return items || []
  return (items || []).filter(item => [item.displayName, item.primaryTraitName, item.internalId]
    .some(value => normalizedText(value).includes(needle)))
}

export function resolveConstructSelection(matches, currentId, query) {
  if (!normalizedText(query)) return currentId || matches?.[0]?.internalId || ''
  if ((matches || []).some(item => item.internalId === currentId)) return currentId
  return matches?.[0]?.internalId || ''
}

export function collectBagTraitOptions(items) {
  const names = new Set()
  for (const item of items || []) {
    if (item.primaryTraitName) names.add(item.primaryTraitName)
    if (item.secondaryTraitName) names.add(item.secondaryTraitName)
  }
  return [...names].sort((left, right) => left.localeCompare(right, 'zh-Hans-CN'))
}

function bagMatchesState(item, state, usedSlotIds) {
  const used = usedSlotIds.has(Number(item.slotId))
  switch (state) {
    case 'unused': return !used
    case 'used': return used
    case 'dual': return Boolean(item.secondaryTraitName)
    case 'single': return !item.secondaryTraitName
    default: return true
  }
}

function compareNumberDesc(field, left, right) {
  return Number(right[field] || 0) - Number(left[field] || 0) || Number(left.slotId) - Number(right.slotId)
}

export function filterAndSortBagSigils(items, options = {}) {
  const needle = normalizedText(options.query)
  const trait = String(options.trait || '')
  const usedSlotIds = options.usedSlotIds instanceof Set ? options.usedSlotIds : new Set(options.usedSlotIds || [])
  const result = (items || []).filter(item => {
    if (needle && ![item.name, item.primaryTraitName, item.secondaryTraitName, item.slotId]
      .some(value => normalizedText(value).includes(needle))) return false
    if (trait && item.primaryTraitName !== trait && item.secondaryTraitName !== trait) return false
    return bagMatchesState(item, options.state || 'all', usedSlotIds)
  })

  const sort = options.sort || 'slot-asc'
  return result.slice().sort((left, right) => {
    switch (sort) {
      case 'slot-desc': return Number(right.slotId) - Number(left.slotId)
      case 'name': return String(left.name || '').localeCompare(String(right.name || ''), 'zh-Hans-CN') || Number(left.slotId) - Number(right.slotId)
      case 'primary-desc': return compareNumberDesc('primaryTraitLevel', left, right)
      case 'secondary-desc': return compareNumberDesc('secondaryTraitLevel', left, right)
      default: return Number(left.slotId) - Number(right.slotId)
    }
  })
}
