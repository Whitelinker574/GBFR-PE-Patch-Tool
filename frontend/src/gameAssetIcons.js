import iconCatalog from './gameAssetIcons.json'

export function normalizeGameAssetKey(value) {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return (value >>> 0).toString(16).toUpperCase().padStart(8, '0')
  }
  return String(value ?? '').trim().replace(/^0x/i, '').toUpperCase()
}

export function gameAssetPath(folder, file) {
  if (!file) return ''
  const encoded = String(file).split('/').map(part => encodeURIComponent(part).replace(/'/g, '%27')).join('/')
  return `/loadout-icons/${folder}/${encoded}`
}

function mappedFile(section, kind, value) {
  const key = normalizeGameAssetKey(value)
  return key ? iconCatalog?.[section]?.[kind]?.[key] || '' : ''
}

export function traitAssetIcon({ internalId = '', hash = '', name = '' } = {}) {
  const file = mappedFile('traits', 'byId', internalId)
    || mappedFile('traits', 'byHash', hash)
    || iconCatalog?.traits?.byName?.[name]
    || ''
  return gameAssetPath('traits', file)
}

export function weaponAssetIcon(weapon = {}) {
  const file = mappedFile('weapons', 'byId', weapon.internalId)
    || mappedFile('weapons', 'byHash', weapon.baseHash)
    || mappedFile('weapons', 'byHash', weapon.hash)
    || mappedFile('weapons', 'byHash', weapon.storedHash)
    || ''
  return gameAssetPath('weapons', file)
}

export function summonAssetIcon(summon = {}) {
  return gameAssetPath('summons', mappedFile('summons', 'byHash', summon.typeHash ?? summon.hash))
}

export function itemAssetIcon(item = {}) {
  return gameAssetPath('items', mappedFile('items', 'byHash', item.hash))
}

export function characterAssetIcon(hash) {
  return gameAssetPath('characters', mappedFile('characters', 'byHash', hash))
}

export { iconCatalog as gameAssetIconCatalog }
