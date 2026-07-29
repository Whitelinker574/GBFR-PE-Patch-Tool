import { GetSigilAtlasIndex } from '../wailsjs/go/backend/SigilGen.js'

export function hydrateSigilAtlasIndex(index) {
  const traits = Array.isArray(index?.traits) ? index.traits : []
  const sigils = (Array.isArray(index?.sigils) ? index.sigils : []).map(entry => {
    const indexes = Array.isArray(entry.secondaryTraitIndexes) ? entry.secondaryTraitIndexes : []
    const levels = Array.isArray(entry.secondaryTraitMaxLevels) ? entry.secondaryTraitMaxLevels : []
    if (indexes.length !== levels.length) {
      throw new Error(`Invalid sigil atlas secondary trait arrays for ${entry.internalId || 'unknown sigil'}`)
    }
    const secondaryTraits = indexes.map((traitIndex, index) => {
      if (!Number.isInteger(traitIndex) || traitIndex < 0 || traitIndex >= traits.length) {
        throw new Error(`Invalid sigil atlas trait index: ${traitIndex}`)
      }
      const trait = traits[traitIndex]
      const maxLevel = Number(levels[index])
      if (!Number.isInteger(maxLevel) || maxLevel < 1) {
        throw new Error(`Invalid sigil atlas trait level: ${levels[index]}`)
      }
      return { ...trait, maxLevel }
    })
    return {
      ...entry,
      secondaryTraits,
      searchText: [entry.displayName, entry.primaryTraitName, entry.internalId, entry.hash, ...secondaryTraits.map(trait => trait.displayName)].filter(Boolean).join(' '),
    }
  })
  return { dataVersion: index?.dataVersion || '', traits, sigils }
}

export function createSigilAtlasStore(loader = GetSigilAtlasIndex) {
  const requests = new Map()
  return {
    load(locale = 'zh') {
      const key = String(locale || 'zh')
      if (requests.has(key)) return requests.get(key)
      const request = Promise.resolve().then(loader).then(hydrateSigilAtlasIndex)
      requests.set(key, request)
      request.catch(() => requests.delete(key))
      return request
    },
    clear() { requests.clear() },
  }
}

export const sigilAtlasStore = createSigilAtlasStore()
