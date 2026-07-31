import { characterIdentityByHash } from './characterRoster.js'

function uniqueStrings(values, normalize = value => value) {
  const seen = new Set()
  const result = []
  for (const raw of values || []) {
    const value = normalize(String(raw || '').trim())
    if (!value || seen.has(value)) continue
    seen.add(value)
    result.push(value)
  }
  return result
}

function normalizedLevel(value) {
  const level = Number(value)
  return Number.isFinite(level) && level > 0 ? Math.round(level) : 0
}

export function createAtlasOptimizerIntent(payload = {}, requestId = Date.now()) {
  const requestedTargets = Array.isArray(payload.targets) ? payload.targets : []
  const requestedIds = [
    ...(Array.isArray(payload.traitIds) ? payload.traitIds : []),
    ...requestedTargets.map(item => item?.traitId),
  ]
  const traitIds = uniqueStrings(requestedIds)
  const namesById = new Map()
  ;(payload.traitIds || []).forEach((id, index) => {
    const normalizedID = String(id || '').trim()
    const name = String(payload.traitNames?.[index] || '').trim()
    if (normalizedID && name && !namesById.has(normalizedID)) namesById.set(normalizedID, name)
  })
  for (const target of requestedTargets) {
    const id = String(target?.traitId || '').trim()
    const name = String(target?.traitName || target?.name || '').trim()
    if (id && name) namesById.set(id, name)
  }

  const levelsById = {}
  for (const id of traitIds) {
    const requested = requestedTargets.find(item => String(item?.traitId || '').trim() === id)
    const level = normalizedLevel(payload.targetLevels?.[id]) || normalizedLevel(requested?.targetLevel)
    if (level) levelsById[id] = level
  }

  return {
    ...payload,
    allowedOwnerCodes: uniqueStrings(payload.allowedOwnerCodes, value => value.toUpperCase()),
    traitIds,
    traitNames: traitIds.map(id => namesById.get(id) || ''),
    targets: traitIds.map(id => ({
      traitId: id,
      traitName: namesById.get(id) || '',
      ...(levelsById[id] ? { targetLevel: levelsById[id] } : {}),
    })),
    targetLevels: levelsById,
    requestId: Number(requestId || payload.requestId || Date.now()),
  }
}

function groupOwnerCode(group) {
  return characterIdentityByHash(group?.charaHash)?.plId || ''
}

function targetGroupFor(groups, currentGroup, ownerCodes) {
  if (!ownerCodes.length) return currentGroup || null
  if (currentGroup && ownerCodes.includes(groupOwnerCode(currentGroup))) return currentGroup
  for (const ownerCode of ownerCodes) {
    const target = groups.find(group => groupOwnerCode(group) === ownerCode)
    if (target) return target
  }
  return null
}

export function planAtlasOptimizerRoute({
  savePath = '',
  groups = [],
  currentGroup = null,
  payload = {},
  requestId = Date.now(),
} = {}) {
  const intent = createAtlasOptimizerIntent(payload, requestId)
  if (!String(savePath || '').trim()) return { kind: 'needs-save', intent }

  const availableGroups = Array.isArray(groups) ? groups : []
  const targetGroup = targetGroupFor(availableGroups, currentGroup, intent.allowedOwnerCodes)
  if (!targetGroup) {
    return {
      kind: intent.allowedOwnerCodes.length ? 'missing-owner' : 'needs-character',
      intent,
      missingOwnerCodes: intent.allowedOwnerCodes,
    }
  }

  const loadout = (targetGroup.loadouts || []).find(item => !item?.isParty) || null
  if (!loadout) return { kind: 'no-editable-slot', intent, targetGroup }
  return { kind: 'ready', intent, targetGroup, loadout }
}
