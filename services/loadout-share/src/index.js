import { CHARACTER_ROSTER, characterByIdentity } from './roster.js'
import iconCatalog from './gameAssetIcons.json' with { type: 'json' }
import skillIconCatalog from './loadoutSkillIcons.json' with { type: 'json' }
import traitNameIcons from './traitNameIcons.json' with { type: 'json' }
import skillNames from './skillNames.json' with { type: 'json' }
import summonCatalog from './summons.json' with { type: 'json' }
import weaponCatalog from './weapons.json' with { type: 'json' }
import weaponAliases from './weaponAliases.json' with { type: 'json' }
import masteryNames from './masteryNames.json' with { type: 'json' }
import gameNames from './gameNames.json' with { type: 'json' }
import { loadoutJSONToFrame } from './loadoutUpload.js'

const FRAME_MAGIC = 'GBLC'
const FRAME_VERSIONS = new Set([1, 2])
const FRAME_CODEC = 1
const FRAME_HEADER_SIZE = 18
const MAX_FRAME_BYTES = 8 * 1024
const MAX_RAW_BYTES = 1024 * 1024
const MAX_TITLE_LENGTH = 80
const MAX_PREVIEW_BYTES = 12 * 1024
const DEFAULT_CATALOG_LIMIT = 24
const MAX_CATALOG_LIMIT = 48
const MAX_SEARCH_LIMIT = 96
const METADATA_READ_BATCH = 10
const CODE_ALPHABET = '0123456789ABCDEFGHJKMNPQRSTVWXYZ'
const CODE_PATTERN = /^[0-9A-HJKMNP-TV-Z]{16,24}$/
const DEFAULT_TRAIT_ICON = '/assets/traits/cmn_icskill_05_00.png'
const DEFAULT_SUMMON_ICON = '/assets/summons/cmn_icitmsmn02_0000.png'
const DEFAULT_WEAPON_ICON = '/assets/weapons/cmn_imgequ_wp0000.png'
const overLimitNames = new Map(Object.entries({
  C4925BD7: ['攻击力', 'ATK'], '52A207B5': ['最大HP', 'Max HP'], '45C65767': ['暴击率', 'Critical Hit Rate'],
  '6CB38EF3': ['昏厥值', 'Stun Power'], '9A97C049': ['能力伤害', 'Skill Damage'], '4E42646B': ['奥义伤害', 'Skybound Art Damage'],
  '68B39018': ['奥义连锁伤害', 'Chain Burst Damage'], '43B7581D': ['普通攻击伤害上限', 'Normal Attack Damage Cap'],
  '9C555433': ['能力伤害上限', 'Skill Damage Cap'], '4A4C093D': ['奥义伤害上限', 'Skybound Art Damage Cap'], '54929589': ['HP回复上限', 'Healing Cap'],
  CB63BE55: ['攻击力', 'ATK'], DCBD8423: ['攻击力', 'ATK'], '59DCE1E8': ['攻击力', 'ATK'], F203BB15: ['攻击力', 'ATK'],
  '57BBC478': ['最大HP', 'Max HP'], '5A51F0CB': ['最大HP', 'Max HP'], '9C6375CF': ['最大HP', 'Max HP'], F004E9F2: ['最大HP', 'Max HP'],
  C4B86ED7: ['暴击率', 'Critical Hit Rate'], CEB0DBD2: ['暴击率', 'Critical Hit Rate'], A3545CA1: ['昏厥值', 'Stun Power'], '59FBB7D8': ['昏厥值', 'Stun Power'],
}))

const abilityKeyByHash = new Map()
const abilityKeyByName = new Map()
const abilityNameByHash = new Map()
const abilityNameByName = new Map()
for (const [hash, skill] of Object.entries(skillNames?.skills || {})) {
  if (skill?.key) abilityKeyByHash.set(assetKey(hash), skill.key)
  if (skill?.key && skill?.name) abilityKeyByName.set(String(skill.name).trim(), skill.key)
  const names = { zh: skill?.name || '', en: skill?.nameEn || '' }
  abilityNameByHash.set(assetKey(hash), names)
  if (names.zh) abilityNameByName.set(names.zh, names)
  if (names.en) abilityNameByName.set(names.en, names)
}

const weaponHashByName = new Map()
const weaponByHash = new Map()
const baseWeaponHashByAlias = new Map()
for (const weapon of weaponCatalog?.weapons || []) {
  weaponByHash.set(assetKey(weapon?.hash), weapon)
  if (weapon?.name) weaponHashByName.set(String(weapon.name).trim(), weapon.hash)
  if (weapon?.nameCn) weaponHashByName.set(String(weapon.nameCn).trim(), weapon.hash)
}
for (const [baseHash, aliases] of Object.entries(weaponAliases || {})) {
  for (const alias of aliases || []) baseWeaponHashByAlias.set(assetKey(alias), assetKey(baseHash))
}

function catalogWeapon(hash) {
  const key = assetKey(hash)
  return weaponByHash.get(key) || weaponByHash.get(baseWeaponHashByAlias.get(key))
}

function catalogWeaponHash(hash) {
  const key = assetKey(hash)
  return weaponByHash.has(key) ? key : baseWeaponHashByAlias.get(key) || key
}

const summonHashByName = new Map()
const summonByHash = new Map()
const summonByName = new Map()
for (const summon of summonCatalog?.summons || []) {
  summonByHash.set(assetKey(summon?.hash), summon)
  if (summon?.displayName && !summonHashByName.has(String(summon.displayName).trim())) {
    summonHashByName.set(String(summon.displayName).trim(), summon.hash)
  }
  if (summon?.displayNameEn && !summonHashByName.has(String(summon.displayNameEn).trim())) {
    summonHashByName.set(String(summon.displayNameEn).trim(), summon.hash)
  }
  if (summon?.displayName) summonByName.set(String(summon.displayName).trim(), summon)
  if (summon?.displayNameEn) summonByName.set(String(summon.displayNameEn).trim(), summon)
}

const traitByName = new Map()
for (const entry of Object.values(gameNames?.traits || {})) {
  if (entry?.zh) traitByName.set(String(entry.zh).trim(), entry)
  if (entry?.en) traitByName.set(String(entry.en).trim(), entry)
}
for (const entry of Object.values(gameNames?.summonSkills || {})) {
  if (entry?.zh) traitByName.set(String(entry.zh).trim(), entry)
  if (entry?.en) traitByName.set(String(entry.en).trim(), entry)
}
for (const entry of Object.values(gameNames?.summonSubParams || {})) {
  if (entry?.zh) traitByName.set(String(entry.zh).trim(), entry)
  if (entry?.en) traitByName.set(String(entry.en).trim(), entry)
}

const wrightstoneByName = new Map([
  ['畏惧之祝福', { zh: '畏惧之祝福', en: 'Dread Wrightstone' }],
  ['Dread Wrightstone', { zh: '畏惧之祝福', en: 'Dread Wrightstone' }],
  ['生机之祝福', { zh: '生机之祝福', en: 'Vitality Wrightstone' }],
  ['Vitality Wrightstone', { zh: '生机之祝福', en: 'Vitality Wrightstone' }],
  ['镇守之祝福', { zh: '镇守之祝福', en: 'Fortification Wrightstone' }],
  ['Fortification Wrightstone', { zh: '镇守之祝福', en: 'Fortification Wrightstone' }],
  ['隔绝之祝福', { zh: '隔绝之祝福', en: 'Sequestration Wrightstone' }],
  ['Sequestration Wrightstone', { zh: '隔绝之祝福', en: 'Sequestration Wrightstone' }],
])

function pageLanguage(url) {
  return String(url?.searchParams?.get('lang') || '').toLowerCase() === 'en' ? 'en' : 'zh'
}

function withLanguage(path, lang) {
  return `${path}${path.includes('?') ? '&' : '?'}lang=${lang}`
}

function localizedName(entry, lang, fallback = '') {
  if (!entry) return fallback
  return cleanText(lang === 'en' ? entry.en || entry.nameEn || entry.name : entry.zh || entry.nameCn || entry.name, 120) || fallback
}

function languageSwitch(path, lang) {
  return `<nav class="language-switch" aria-label="${lang === 'en' ? 'Language' : '语言'}"><a href="${withLanguage(path, 'zh')}"${lang === 'zh' ? ' class="active" aria-current="true"' : ''}>中文</a><a href="${withLanguage(path, 'en')}"${lang === 'en' ? ' class="active" aria-current="true"' : ''}>EN</a></nav>`
}

const baseHeaders = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Headers': 'Content-Type',
  'Access-Control-Allow-Methods': 'GET, HEAD, POST, OPTIONS',
  'X-Content-Type-Options': 'nosniff',
}

function jsonResponse(value, status = 200, extraHeaders = {}) {
  return new Response(JSON.stringify(value), {
    status,
    headers: {
      ...baseHeaders,
      ...extraHeaders,
      'Content-Type': 'application/json; charset=utf-8',
    },
  })
}

function errorResponse(message, status) {
  return jsonResponse({ error: message }, status, { 'Cache-Control': 'no-store' })
}

function readUint32LE(bytes, offset) {
  return new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength).getUint32(offset, true)
}

export function validateFrame(bytes) {
  if (!(bytes instanceof Uint8Array)) return '请求体必须是二进制配装帧'
  if (bytes.byteLength < FRAME_HEADER_SIZE) return '配装帧过短'
  if (bytes.byteLength > MAX_FRAME_BYTES) return '配装帧超过 8 KB'
  const magic = String.fromCharCode(...bytes.subarray(0, 4))
  if (magic !== FRAME_MAGIC) return '配装帧标识无效'
  if (!FRAME_VERSIONS.has(bytes[4])) return '不支持的配装帧版本'
  if (bytes[5] !== FRAME_CODEC) return '不支持的配装压缩格式'
  const rawSize = readUint32LE(bytes, 6)
  if (rawSize === 0 || rawSize > MAX_RAW_BYTES) return '配装原始数据大小无效'
  const compressedSize = readUint32LE(bytes, 14)
  if (compressedSize === 0 || compressedSize !== bytes.byteLength - FRAME_HEADER_SIZE) {
    return '配装帧长度不一致'
  }
  return ''
}

function encodeBase32(bytes) {
  let output = ''
  let buffer = 0
  let bits = 0
  for (const byte of bytes) {
    buffer = (buffer << 8) | byte
    bits += 8
    while (bits >= 5) {
      bits -= 5
      output += CODE_ALPHABET[(buffer >>> bits) & 31]
    }
  }
  if (bits > 0) output += CODE_ALPHABET[(buffer << (5 - bits)) & 31]
  return output
}

function toHex(bytes) {
  return Array.from(bytes, byte => byte.toString(16).padStart(2, '0')).join('')
}

export async function frameIdentity(bytes) {
  const digest = new Uint8Array(await crypto.subtle.digest('SHA-256', bytes))
  return {
    code: encodeBase32(digest.subarray(0, 10)),
    hash: toHex(digest),
  }
}

export function normalizeCode(value) {
  const code = String(value || '').toUpperCase().replace(/[-\s]/g, '')
  return CODE_PATTERN.test(code) ? code : ''
}

export function displayCode(code) {
  return normalizeCode(code).match(/.{1,4}/g)?.join('-') || ''
}

function objectKey(code) {
  return `v1/${code}`
}

function metadataKey(code) {
  return `meta/v1/${code}.json`
}

function cleanText(value, max = MAX_TITLE_LENGTH) {
  return String(value || '').replace(/[\u0000-\u001f\u007f]/g, ' ').trim().slice(0, max)
}

function escapeHtml(value) {
  return String(value ?? '').replace(/[&<>"']/g, character => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
  })[character])
}

function displayText(value) {
  return cleanText(value)
    .replaceAll('Catastrophe', '浩劫')
    .replaceAll('DMG Cap', '伤害上限')
    .replaceAll('Dread Wrightstone', '畏惧之祝福')
    .replaceAll('Vitality Wrightstone', '生机之祝福')
    .replaceAll('Fortification Wrightstone', '镇守之祝福')
    .replaceAll('Sequestration Wrightstone', '隔绝之祝福')
    .replace(/(?:\||｜)HP$/i, '｜体力')
}

function assetKey(value) {
  if (typeof value === 'number' && Number.isFinite(value)) return (value >>> 0).toString(16).toUpperCase().padStart(8, '0')
  return String(value || '').trim().replace(/^0x/i, '').toUpperCase()
}

function assetIcon(section, value, folder) {
  const key = assetKey(value)
  if (!key) return ''
  const table = iconCatalog?.[section] || {}
  const file = table.byHash?.[key] || table.byId?.[key] || ''
  return file ? `/assets/${folder}/${encodeURIComponent(file)}` : ''
}

function namedAssetIcon(section, value, folder) {
  const direct = assetIcon(section, value, folder)
  if (direct) return direct
  const name = String(value || '').trim()
  if (!name) return ''
  const names = [name, name.replace(/[IVX]+(?:\+)?$/i, '').replace(/\+$/, '').trim()]
  const catalog = section === 'traits' ? traitNameIcons : section === 'skills' ? skillIconCatalog : null
  const file = names.map(candidate => catalog?.[candidate]).find(Boolean) || ''
  return file ? `/assets/${folder}/${encodeURIComponent(file)}` : ''
}

function abilityAssetIcon(skill) {
  const key = String(skill?.key || '').trim() || abilityKeyByHash.get(assetKey(skill?.hash)) || abilityKeyByName.get(String(skill?.name || '').trim()) || ''
  return namedAssetIcon('skills', key, 'skills') || DEFAULT_TRAIT_ICON
}

function masteryAssetIcon(node) {
  const text = `${node?.name || ''} ${node?.effect || ''}`
  for (const label of ['伤害上限', '攻击力', '暴击率', '昏厥', '追击', '快速冷却', '快速蓄力', '防御力', '最大HP']) {
    if (text.includes(label)) {
      const icon = namedAssetIcon('traits', label, 'traits')
      if (icon) return icon
    }
  }
  return DEFAULT_TRAIT_ICON
}

function legalLegacySigilName(sigil) {
  const primary = String(sigil?.primary || '').trim()
  let name = String(sigil?.name || '').trim()
  const hasSecondary = Boolean(String(sigil?.secondary || '').trim())
  if (hasSecondary && name === `${primary} + ${String(sigil.secondary).trim()}`) name = primary
  const rankedPlusShell = /(?:^|\s)(?:V|IV|III|II|I)\+?$/i.test(name)
  if (hasSecondary && (name === primary || name === `${primary} V` || name === `${primary} V+` || rankedPlusShell)) {
    if (['可怕的漆黑钳蟹因子', '相扑斗力', '漆黑之谊'].includes(primary)) return primary
    if (primary === '躲避性能') return `${primary}+`
    return `${primary} V+`
  }
  if (!name && primary) return hasSecondary ? `${primary} V+` : primary
  return name.replace(/([^\s])((?:V|IV|III|II|I)\+?)$/i, '$1 $2')
}

function chineseWeaponName(name, hash = '') {
  const byHash = catalogWeapon(hash)
  const byName = weaponByHash.get(assetKey(weaponHashByName.get(String(name || '').trim())))
  return cleanText(byHash?.nameCn || byName?.nameCn || name, 60)
}

function chinesePreviewSource(source, preview, skill, summonNames) {
  const parts = cleanText(source, 100).split(/\s*·\s*/).filter(Boolean)
  if (!parts.length) return ''
  const traitName = displayText(skill?.name || '')
  const withLevel = value => {
    const match = String(value || '').match(/\s+(Lv\s*\d+(?:\.\d+)?)\s*$/i)
    return `${traitName || displayText(String(value || '').replace(/\s+Lv\s*\d+(?:\.\d+)?\s*$/i, ''))}${match ? ` ${match[1].replace(/\s+/g, '')}` : ''}`
  }
  const factor = parts[0].match(/^(?:因子|Sigil)\s*(\d+)$/i)
  if (factor) return `因子${factor[1].padStart(2, '0')} · ${withLevel(parts[1])}`
  const constructed = parts[0].match(/^(?:构造因子|Constructed Sigil)\s*(\d+)$/i)
  if (constructed) return `构造因子${constructed[1].padStart(2, '0')} · ${withLevel(parts[1])}`
  if (/^(?:武器|Weapon)$/i.test(parts[0])) {
    const weaponName = chineseWeaponName(preview.weaponName, preview.weaponHash)
    return `武器 · ${weaponName || '未标注武器'}${parts[2] ? ` · ${withLevel(parts[2])}` : ''}`
  }
  if (/^(?:武炼结晶|Wrightstone)$/i.test(parts[0])) {
    const name = displayText(preview.wrightstone?.name || parts[1] || '未标注祝福')
    return `武器祝福 · ${name}${parts[2] ? ` · ${withLevel(parts[2])}` : ''}`
  }
  if (/^(?:召唤石|Summon)$/i.test(parts[0])) {
    return `召唤石 · ${summonNames.get(parts[1]) || parts[1] || '未标注召唤石'}`
  }
  return parts.map(displayText).join(' · ')
}

function decoratePreview(preview) {
  if (!preview || typeof preview !== 'object') return preview
  const characterHash = assetKey(preview.characterHash)
  const characterName = String(preview.characterName || '').trim().toLocaleLowerCase('zh-CN')
  const character = CHARACTER_ROSTER.find(item => (
    characterHash && item.hash === characterHash
  ) || [item.name, item.nameEn, item.slug, ...(item.aliases || [])]
    .some(value => String(value || '').trim().toLocaleLowerCase('zh-CN') === characterName))
  if (character) {
    preview.characterName = character.name
    preview.characterNameEn = character.nameEn
    preview.characterHash = character.hash
  }
  preview.abilities = (preview.abilities || []).map(skill => typeof skill === 'string' ? { name: skill } : skill)
  preview.weaponName = chineseWeaponName(preview.weaponName, preview.weaponHash)
  preview.weaponIcon = assetIcon('weapons', catalogWeaponHash(preview.weaponHash), 'weapons') || assetIcon('weapons', weaponHashByName.get(String(preview.weaponName || '').trim()), 'weapons') || DEFAULT_WEAPON_ICON
  for (const sigil of preview.sigils || []) {
    const catalogItemName = localizedName(gameNames?.traits?.[assetKey(sigil.hash)], 'zh', '')
    if (catalogItemName) sigil.name = catalogItemName
    sigil.name = legalLegacySigilName(sigil)
    sigil.primaryIcon = namedAssetIcon('traits', sigil.primaryHash || sigil.primary, 'traits') || (sigil.primary ? DEFAULT_TRAIT_ICON : '')
    sigil.secondaryIcon = namedAssetIcon('traits', sigil.secondaryHash || sigil.secondary, 'traits') || (sigil.secondary ? DEFAULT_TRAIT_ICON : '')
    sigil.icon = sigil.primaryIcon || namedAssetIcon('traits', sigil.primary || sigil.name, 'traits') || DEFAULT_TRAIT_ICON
  }
  for (const skill of preview.abilities || []) {
    if (skill && typeof skill === 'object') skill.icon = abilityAssetIcon(skill)
  }
  for (const skill of preview.weaponSkills || []) {
    if (skill && typeof skill === 'object') skill.icon = namedAssetIcon('traits', skill.hash || skill.name, 'traits') || DEFAULT_TRAIT_ICON
  }
  if (preview.wrightstone) {
    preview.wrightstone.name = displayText(preview.wrightstone.name)
    for (const trait of preview.wrightstone.traits || []) {
      trait.name = displayText(trait.name)
      trait.icon = namedAssetIcon('traits', trait.hash || trait.name, 'traits') || DEFAULT_TRAIT_ICON
    }
    preview.wrightstone.icon = preview.wrightstone.traits?.find(trait => trait.icon)?.icon || ''
  }
  const summonNames = new Map()
  for (const summon of preview.summons || []) {
    const oldName = String(summon.name || '').trim()
    const catalogSummon = summonByHash.get(assetKey(summon.typeHash))
    if (catalogSummon?.displayName) summon.name = catalogSummon.displayName
    if (oldName) summonNames.set(oldName, summon.name || oldName)
    summon.icon = assetIcon('summons', summon.typeHash, 'summons') || assetIcon('summons', summonHashByName.get(String(summon.name || '').trim()), 'summons') || DEFAULT_SUMMON_ICON
    summon.mainIcon = namedAssetIcon('traits', summon.mainTraitHash || summon.mainTrait, 'traits') || (summon.mainTrait ? DEFAULT_TRAIT_ICON : '')
    summon.subIcon = namedAssetIcon('traits', summon.subParamHash || summon.subParam, 'traits') || (summon.subParam ? DEFAULT_TRAIT_ICON : '')
  }
  for (const node of preview.masterySkills || []) {
    node.icon = namedAssetIcon('traits', node.hash || node.name, 'traits') || masteryAssetIcon(node)
  }
  for (const slot of preview.overLimit || []) {
    const names = overLimitNames.get(assetKey(slot.attributeHash))
    if (names) {
      slot.name = names[0]
      slot.nameEn = names[1]
    }
    slot.icon = namedAssetIcon('traits', slot.name, 'traits') || DEFAULT_TRAIT_ICON
  }
  for (const skill of preview.combinedSkills || []) {
    skill.name = displayText(skill.name)
    skill.effect = displayText(skill.effect)
    skill.sources = (skill.sources || []).map(source => chinesePreviewSource(source, preview, skill, summonNames)).filter(Boolean)
    skill.icon = namedAssetIcon('traits', skill.hash || skill.name, 'traits') || DEFAULT_TRAIT_ICON
  }
  return preview
}

function skillIdentityKeys(hash, name) {
  const keys = new Set()
  const addName = value => {
    const normalized = String(value || '').trim().toLocaleLowerCase().replace(/[\s·・|｜:：()（）\[\]【】]/g, '')
    if (normalized) keys.add(`name:${normalized}`)
  }
  const hashKey = assetKey(hash)
  if (hashKey) keys.add(`hash:${hashKey}`)
  addName(name)
  const catalog = gameNames?.traits?.[hashKey] || gameNames?.summonSkills?.[hashKey] || traitByName.get(String(name || '').trim())
  addName(catalog?.zh)
  addName(catalog?.en)
  return keys
}

function rebuildCombinedSkillSources(preview, lang) {
  const sourceIndex = new Map()
  const remember = (hash, name, source) => {
    if (!source || !String(name || '').trim()) return
    for (const key of skillIdentityKeys(hash, name)) {
      const sources = sourceIndex.get(key) || []
      if (!sources.includes(source)) sources.push(source)
      sourceIndex.set(key, sources)
    }
  }
  const levelText = level => Number(level) > 0 ? ` Lv${Number(level)}` : ''
  const sigilWord = lang === 'en' ? 'Sigil' : '因子'
  const weaponWord = lang === 'en' ? 'Weapon' : '武器'
  const wrightstoneWord = lang === 'en' ? 'Wrightstone' : '武器祝福'
  const summonWord = lang === 'en' ? 'Summon' : '召唤石'
  const masteryWord = lang === 'en' ? 'Mastery' : '专精'

  for (const [index, sigil] of (preview.sigils || []).entries()) {
    const slot = String(index + 1).padStart(2, '0')
    remember(sigil.primaryHash, sigil.primary, `${sigilWord}${slot} · ${sigil.primary}${levelText(sigil.primaryLevel || sigil.level)}`)
    remember(sigil.secondaryHash, sigil.secondary, `${sigilWord}${slot} · ${sigil.secondary}${levelText(sigil.secondaryLevel || sigil.level)}`)
  }
  for (const skill of preview.weaponSkills || []) {
    remember(skill.hash, skill.name, `${weaponWord} · ${preview.weaponName || ''} · ${skill.name}${levelText(skill.level)}`)
  }
  for (const trait of preview.wrightstone?.traits || []) {
    remember(trait.hash, trait.name, `${wrightstoneWord} · ${preview.wrightstone?.name || ''} · ${trait.name}${levelText(trait.level)}`)
  }
  for (const [index, summon] of (preview.summons || []).entries()) {
    const slot = String(index + 1).padStart(2, '0')
    remember(summon.mainTraitHash, summon.mainTrait, `${summonWord}${slot} · ${summon.name} · ${summon.mainTrait}${levelText(summon.mainTraitLevel)}`)
  }
  for (const node of preview.masterySkills || []) {
    const count = Number(node.count) > 1 ? ` ×${Number(node.count)}` : ''
    remember(node.hash, node.name, `${masteryWord} · ${node.rank || node.name}${count}`)
  }

  for (const skill of preview.combinedSkills || []) {
    if ((skill.sources || []).length) continue
    const sources = []
    for (const key of skillIdentityKeys(skill.hash, skill.name)) {
      for (const source of sourceIndex.get(key) || []) {
        if (!sources.includes(source)) sources.push(source)
      }
    }
    skill.sources = sources
  }
}

function localizePreview(preview, lang) {
  if (!preview || typeof preview !== 'object') return preview
  const traitName = (hash, fallback = '') => localizedName(gameNames?.traits?.[assetKey(hash)] || gameNames?.summonSkills?.[assetKey(hash)] || traitByName.get(String(fallback || '').trim()), lang, fallback)
  const summonSkillName = (hash, fallback = '') => localizedName(gameNames?.summonSkills?.[assetKey(hash)] || traitByName.get(String(fallback || '').trim()), lang, traitName(hash, fallback))
  const summonSubName = (hash, fallback = '') => localizedName(gameNames?.summonSubParams?.[assetKey(hash)] || traitByName.get(String(fallback || '').trim()), lang, fallback)
  const weapon = catalogWeapon(preview.weaponHash)
  if (weapon) preview.weaponName = lang === 'en' ? weapon.name : weapon.nameCn
  for (const sigil of preview.sigils || []) {
    const originalName = String(sigil.name || '')
    const itemName = localizedName(gameNames?.traits?.[assetKey(sigil.hash)], lang, '')
    sigil.primary = traitName(sigil.primaryHash, sigil.primary)
    sigil.secondary = traitName(sigil.secondaryHash, sigil.secondary)
    if (lang === 'zh') {
      if (itemName) sigil.name = itemName
      sigil.name = legalLegacySigilName(sigil)
    } else if (itemName) {
      sigil.name = itemName
    } else if (sigil.primary) {
      if (sigil.secondary) sigil.name = `${sigil.primary}${/\bV\+$/i.test(originalName) ? ' V+' : /\+$/i.test(originalName) ? '+' : ' V+'}`
      else {
        const suffix = originalName.match(/\b(?:V|IV|III|II|I)\+?$/i)?.[0] || ''
        sigil.name = `${sigil.primary}${suffix ? ` ${suffix}` : ''}`
      }
    }
  }
  for (const skill of preview.abilities || []) {
    skill.name = localizedName(abilityNameByHash.get(assetKey(skill.hash)) || abilityNameByName.get(String(skill.name || '').trim()), lang, skill.name)
  }
  for (const skill of preview.weaponSkills || []) skill.name = traitName(skill.hash, skill.name)
  if (preview.wrightstone) {
    preview.wrightstone.name = localizedName(wrightstoneByName.get(String(preview.wrightstone.name || '').trim()), lang, preview.wrightstone.name)
    for (const trait of preview.wrightstone.traits || []) trait.name = traitName(trait.hash, trait.name)
  }
  for (const summon of preview.summons || []) {
    const catalog = summonByHash.get(assetKey(summon.typeHash)) || summonByName.get(String(summon.name || '').trim())
    if (catalog) summon.name = lang === 'en' ? catalog.displayNameEn : catalog.displayName
    summon.mainTrait = summonSkillName(summon.mainTraitHash, summon.mainTrait)
    const legacySubHash = summon.subParam === '普通攻击伤害上限'
      ? (Number(summon.subParamValue) > 50 ? '9245DFA4' : 'A66241C9')
      : ''
    summon.subParam = summonSubName(summon.subParamHash || legacySubHash, summon.subParam)
  }
  for (const node of preview.masterySkills || []) {
    const catalog = masteryNames?.nodes?.[assetKey(node.hash)]
    if (catalog) {
      node.name = lang === 'en' ? catalog.nameEn : catalog.name
      node.effect = lang === 'en' ? catalog.descEn : catalog.desc
    }
  }
  for (const slot of preview.overLimit || []) {
    if (lang === 'en') slot.name = cleanText(slot.nameEn, 60) || slot.name
  }
  if (lang === 'en') {
    preview.masteryLabel = ({ SB_ATK: 'Attack Mastery', SB_DEF: 'Defense Mastery', SB_LIMIT: 'Limit Mastery' })[preview.masteryCat] || (preview.masteryCount ? 'Mastery' : '')
    const stripUntranslated = value => /[\u3400-\u9fff]/.test(String(value || '')) ? '' : value
    for (const skill of preview.weaponSkills || []) skill.effect = stripUntranslated(skill.effect)
    for (const trait of preview.wrightstone?.traits || []) trait.effect = stripUntranslated(trait.effect)
    for (const node of preview.masterySkills || []) {
      node.rank = ({ R1: 'Mastery I', R2: 'Mastery II', R3: 'Mastery III', EX: 'EX Mastery' })[node.rank] || stripUntranslated(node.rank) || 'Mastery'
      node.name = stripUntranslated(node.name)
      node.effect = stripUntranslated(node.effect)
    }
    for (const skill of preview.combinedSkills || []) {
      skill.effect = stripUntranslated(skill.effect)
      skill.sources = (skill.sources || []).filter(source => !/[\u3400-\u9fff]/.test(source))
    }
  }
  for (const skill of preview.combinedSkills || []) {
    skill.name = traitName(skill.hash, skill.name)
  }
  rebuildCombinedSkillSources(preview, lang)
  return preview
}

function hasPublicCatalogPreview(metadata) {
  const preview = metadata?.preview
  if (!preview || typeof preview !== 'object') return false
  const hash = assetKey(metadata.characterHash || preview.characterHash)
  const name = String(metadata.characterName || preview.characterName || '').trim().toLocaleLowerCase()
  const knownCharacter = CHARACTER_ROSTER.some(character => assetKey(character.hash) === hash || [character.name, character.nameEn, character.slug].some(value => String(value || '').trim().toLocaleLowerCase() === name))
  if (!knownCharacter) return false
  return Boolean(
    preview.weaponHash || preview.weaponName ||
    preview.sigils?.length || preview.weaponSkills?.length || preview.abilities?.length ||
    preview.summons?.length || preview.masterySkills?.length || preview.combinedSkills?.length,
  )
}

function headerText(request, name, encodedName) {
  const encoded = request.headers.get(encodedName) || ''
  if (encoded) {
    try { return cleanText(new TextDecoder().decode(Uint8Array.from(atob(encoded), char => char.charCodeAt(0)))) } catch { /* fall through */ }
  }
  return cleanText(request.headers.get(name))
}

function decodePreviewHeader(request) {
  const encoded = request.headers.get('X-Loadout-Preview') || ''
  if (!encoded || encoded.length > MAX_PREVIEW_BYTES * 2) return null
  try {
    const binary = atob(encoded.replace(/-/g, '+').replace(/_/g, '/'))
    const bytes = Uint8Array.from(binary, char => char.charCodeAt(0))
    if (bytes.byteLength > MAX_PREVIEW_BYTES) return null
    const value = JSON.parse(new TextDecoder().decode(bytes))
    if (!value || typeof value !== 'object' || Array.isArray(value)) return null
    // The preview is informational only. Never accept storage paths, owner codes,
    // slot ids, or arbitrary HTML from the desktop client.
    const cleanHash = value => cleanText(value, 16).replace(/[^0-9A-Z_]/gi, '')
    const cleanSkill = item => ({
      hash: cleanHash(item?.hash),
      key: cleanText(item?.key, 40),
      name: cleanText(item?.name, 48),
      level: Math.max(0, Math.min(999, Number(item?.level) || 0)),
      effect: cleanText(item?.effect, 240),
    })
    return {
      title: cleanText(value.title),
      characterHash: cleanText(value.characterHash, 16),
      characterName: cleanText(value.characterName, 40),
      weaponHash: cleanText(value.weaponHash, 16),
      weaponName: cleanText(value.weaponName, 60),
      sigils: Array.isArray(value.sigils) ? value.sigils.slice(0, 12).map(item => ({
        hash: cleanHash(item?.hash),
        name: cleanText(item?.name, 48),
        level: Math.max(0, Math.min(99, Number(item?.level) || 0)),
        primaryHash: cleanHash(item?.primaryHash),
        primary: cleanText(item?.primary, 48),
        primaryLevel: Math.max(0, Math.min(999, Number(item?.primaryLevel) || 0)),
        secondaryHash: cleanHash(item?.secondaryHash),
        secondary: cleanText(item?.secondary, 48),
        secondaryLevel: item?.secondary ? Math.max(0, Math.min(999,
          Number(item?.secondaryLevel) || Number(item?.primaryLevel) || Number(item?.level) || 0,
        )) : 0,
      })) : [],
      abilities: Array.isArray(value.abilities || value.skills) ? (value.abilities || value.skills).slice(0, 4).map(item => typeof item === 'object' ? cleanSkill(item) : { name: cleanText(item, 48), hash: '' }).filter(item => item.name) : [],
      weaponSkills: Array.isArray(value.weaponSkills) ? value.weaponSkills.slice(0, 8).map(cleanSkill).filter(item => item.name) : [],
      wrightstone: value.wrightstone && typeof value.wrightstone === 'object' ? {
        hash: cleanHash(value.wrightstone.hash),
        name: cleanText(value.wrightstone.name, 60),
        traits: Array.isArray(value.wrightstone.traits) ? value.wrightstone.traits.slice(0, 3).map(cleanSkill).filter(item => item.name) : [],
      } : null,
      summons: Array.isArray(value.summons) ? value.summons.slice(0, 4).map(item => ({
        typeHash: cleanHash(item?.typeHash),
        name: cleanText(item?.name, 48),
        rank: Math.max(0, Math.min(99, Number(item?.rank) || 0)),
        mainTraitHash: cleanHash(item?.mainTraitHash),
        mainTrait: cleanText(item?.mainTrait, 48),
        mainTraitLevel: Math.max(0, Math.min(999, Number(item?.mainTraitLevel) || 0)),
        subParamHash: cleanHash(item?.subParamHash),
        subParam: cleanText(item?.subParam, 48),
        subParamLevel: Math.max(0, Math.min(999, Number(item?.subParamLevel) || 0)),
        subParamValue: Math.max(-1000000, Math.min(1000000, Number(item?.subParamValue) || 0)),
        subParamUnit: ['pct', 'flat'].includes(item?.subParamUnit) ? item.subParamUnit : '',
      })).filter(item => item.name) : [],
      masteryCount: Math.max(0, Math.min(1000, Number(value.masteryCount) || 0)),
      masteryCat: cleanText(value.masteryCat, 24),
      masteryLabel: cleanText(value.masteryLabel, 60),
      masterySkills: Array.isArray(value.masterySkills) ? value.masterySkills.slice(0, 50).map(item => ({
        hash: cleanHash(item?.hash),
        rank: cleanText(item?.rank, 24),
        name: cleanText(item?.name, 60),
        effect: cleanText(item?.effect, 240),
        count: Math.max(1, Math.min(50, Number(item?.count) || 1)),
      })).filter(item => item.name || item.effect) : [],
      overLimit: Array.isArray(value.overLimit) ? value.overLimit.slice(0, 4).map(item => ({
        index: Math.max(0, Math.min(3, Number(item?.index) || 0)),
        attributeHash: cleanHash(item?.attributeHash),
        name: cleanText(item?.name, 60),
        nameEn: cleanText(item?.nameEn, 60),
        level: Math.max(0, Math.min(10, Number(item?.level) || 0)),
        value: Math.max(-1000000, Math.min(1000000, Number(item?.value) || 0)),
        unit: ['pct', 'flat'].includes(item?.unit) ? item.unit : '',
      })).filter(item => item.name || item.attributeHash) : [],
      combinedSkills: Array.isArray(value.combinedSkills) ? value.combinedSkills.slice(0, 80).map(item => ({
        hash: cleanHash(item?.hash),
        name: cleanText(item?.name, 60),
        level: Math.max(0, Math.min(999, Number(item?.level) || 0)),
        rawLevel: Math.max(0, Math.min(9999, Number(item?.rawLevel) || 0)),
        maxLevel: Math.max(0, Math.min(999, Number(item?.maxLevel) || 0)),
        effect: cleanText(item?.effect, 240),
        sources: Array.isArray(item?.sources) ? item.sources.slice(0, 8).map(source => cleanText(source, 100)).filter(Boolean) : [],
      })).filter(item => item.name) : [],
    }
  } catch {
    return null
  }
}

async function readMetadata(env, code, lang = 'zh') {
  const object = await env.LOADOUTS.get(metadataKey(code))
  if (!object) return null
  try {
    const metadata = JSON.parse(new TextDecoder().decode(await object.arrayBuffer()))
    if (metadata?.preview) localizePreview(decoratePreview(metadata.preview), lang)
    return metadata
  } catch {
    return null
  }
}

function previewSearchText(metadata) {
  const preview = metadata?.preview || {}
  const values = [metadata?.title, metadata?.characterName, preview.weaponName, preview.masteryLabel]
  const add = (items, fields) => {
    for (const item of items || []) {
      for (const field of fields) values.push(item?.[field])
    }
  }
  add(preview.sigils, ['name', 'primary', 'secondary'])
  add(preview.abilities || preview.skills, ['name'])
  add(preview.weaponSkills, ['name', 'effect'])
  values.push(preview.wrightstone?.name)
  add(preview.wrightstone?.traits, ['name', 'effect'])
  add(preview.summons, ['name', 'mainTrait', 'subParam'])
  add(preview.masterySkills, ['name', 'effect', 'rank'])
  add(preview.combinedSkills, ['name', 'effect'])
  return values.filter(Boolean).join('\n').toLowerCase()
}

async function readMetadataInBatches(env, objects, lang) {
  const results = []
  for (let offset = 0; offset < objects.length; offset += METADATA_READ_BATCH) {
    const batch = objects.slice(offset, offset + METADATA_READ_BATCH)
    results.push(...await Promise.all(batch.map(async item => {
      const code = item.key.slice('meta/v1/'.length).replace(/\.json$/, '')
      return { code, meta: await readMetadata(env, code, lang) }
    })))
  }
  return results
}

async function visitorDigest(value) {
  const bytes = new TextEncoder().encode(String(value || '').slice(0, 256))
  const digest = new Uint8Array(await crypto.subtle.digest('SHA-256', bytes))
  return toHex(digest).slice(0, 32)
}

async function ensureCommunityEntry(env, metadata) {
  if (!env.COMMUNITY_DB || !metadata?.code) return
  try {
    await env.COMMUNITY_DB.prepare(`INSERT OR IGNORE INTO loadouts (code, title, character_name, character_hash, created_at) VALUES (?, ?, ?, ?, ?)`)
      .bind(metadata.code, metadata.title || '', metadata.characterName || '', metadata.characterHash || '', metadata.createdAt || new Date().toISOString()).run()
  } catch { /* Community features are optional; binary sharing must remain available. */ }
}

async function communitySummary(env, code) {
  if (!env.COMMUNITY_DB) return { enabled: false, likes: 0, comments: [] }
  try {
    await ensureCommunityEntry(env, await readMetadata(env, code))
    const entry = await env.COMMUNITY_DB.prepare('SELECT likes_count FROM loadouts WHERE code = ?').bind(code).first()
    const rows = await env.COMMUNITY_DB.prepare('SELECT id, author, body, created_at FROM comments WHERE code = ? AND deleted = 0 ORDER BY id DESC LIMIT 50').bind(code).all()
    return { enabled: true, likes: Number(entry?.likes_count || 0), comments: rows.results || [] }
  } catch { return { enabled: false, likes: 0, comments: [] } }
}

async function communityAction(request, env, code, action) {
  if (!env.COMMUNITY_DB) return errorResponse('社区互动尚未启用', 503)
  if (!(await env.LOADOUTS.head(objectKey(code)))) return errorResponse('没有找到这套配装', 404)
  let body = {}
  try { body = await request.json() } catch { return errorResponse('请求内容必须是 JSON', 400) }
  const visitorKey = String(body.visitorKey || request.headers.get('X-Visitor-Key') || '').trim()
  if (visitorKey.length < 8 || visitorKey.length > 256) return errorResponse('缺少有效的匿名访问标识', 400)
  const visitor = await visitorDigest(visitorKey)
  try {
    if (action === 'like') {
      await env.COMMUNITY_DB.prepare('INSERT OR IGNORE INTO likes (code, visitor_key) VALUES (?, ?)').bind(code, visitor).run()
      await env.COMMUNITY_DB.prepare('UPDATE loadouts SET likes_count = (SELECT COUNT(*) FROM likes WHERE code = ?) WHERE code = ?').bind(code, code).run()
      return jsonResponse(await communitySummary(env, code), 200, { 'Cache-Control': 'no-store' })
    }
    const text = cleanText(body.body, 500)
    if (!text) return errorResponse('留言不能为空', 400)
    const author = cleanText(body.author, 24) || '匿名旅人'
    await env.COMMUNITY_DB.prepare('INSERT INTO comments (code, author, body, visitor_key, created_at) VALUES (?, ?, ?, ?, ?)').bind(code, author, text, visitor, new Date().toISOString()).run()
    return jsonResponse(await communitySummary(env, code), 201, { 'Cache-Control': 'no-store' })
  } catch { return errorResponse('社区互动暂时不可用', 503) }
}

async function readObjectBytes(object) {
  if (!object) return null
  return new Uint8Array(await object.arrayBuffer())
}

async function readBoundedRequestBody(request, maximum, message) {
  const declaredText = request.headers.get('Content-Length')
  if (declaredText) {
    const declared = Number(declaredText)
    if (!Number.isFinite(declared) || declared < 0) return { response: errorResponse('Content-Length 无效', 400) }
    if (declared > maximum) return { response: errorResponse(message, 413) }
  }
  if (!request.body) return { bytes: new Uint8Array() }
  const reader = request.body.getReader()
  const chunks = []
  let total = 0
  try {
    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      const chunk = value instanceof Uint8Array ? value : new Uint8Array(value)
      total += chunk.byteLength
      if (total > maximum) {
        await reader.cancel()
        return { response: errorResponse(message, 413) }
      }
      chunks.push(chunk)
    }
  } finally {
    reader.releaseLock()
  }
  const bytes = new Uint8Array(total)
  let offset = 0
  for (const chunk of chunks) {
    bytes.set(chunk, offset)
    offset += chunk.byteLength
  }
  return { bytes }
}

async function publish(request, env, origin) {
  const contentType = request.headers.get('Content-Type') || ''
  if (!contentType.toLowerCase().startsWith('application/octet-stream')) {
    return errorResponse('只接受 application/octet-stream', 415)
  }
  const body = await readBoundedRequestBody(request, MAX_FRAME_BYTES, '配装帧超过 8 KB')
  if (body.response) return body.response
  const bytes = body.bytes
  const frameError = validateFrame(bytes)
  if (frameError) return errorResponse(frameError, 400)

  const identity = await frameIdentity(bytes)
  let code = identity.code
  let key = objectKey(code)
  let existing = await env.LOADOUTS.head(key)
  if (existing && existing.customMetadata?.sha256 !== identity.hash) {
    const digest = new Uint8Array(await crypto.subtle.digest('SHA-256', bytes))
    code = encodeBase32(digest.subarray(0, 15))
    key = objectKey(code)
    existing = await env.LOADOUTS.head(key)
  }
  if (existing && existing.customMetadata?.sha256 !== identity.hash) {
    return errorResponse('短码冲突，请稍后重试', 409)
  }

  const reused = Boolean(existing)
  if (!existing) {
    await env.LOADOUTS.put(key, bytes, {
      httpMetadata: { contentType: 'application/vnd.gbfr.loadout' },
      customMetadata: {
        sha256: identity.hash,
        protocol: `GBLC${bytes[4]}`,
      },
    })
  }

  const preview = decodePreviewHeader(request)
  const previousMetadata = reused ? await readMetadata(env, code) : null
  if (!previousMetadata) {
    const title = headerText(request, 'X-Loadout-Title', 'X-Loadout-Title-B64') || preview?.title || preview?.characterName || '未命名配装'
    const metadata = {
      schema: 1,
      code,
      title,
      characterHash: cleanText(request.headers.get('X-Loadout-Character-Hash'), 16) || preview?.characterHash || '',
      characterName: headerText(request, 'X-Loadout-Character', 'X-Loadout-Character-B64') || preview?.characterName || '',
      preview,
      createdAt: previousMetadata?.createdAt || new Date().toISOString(),
    }
    await env.LOADOUTS.put(metadataKey(code), JSON.stringify(metadata), {
      httpMetadata: { contentType: 'application/json; charset=utf-8' },
      customMetadata: { code, schema: '1' },
    })
  }

  const shown = displayCode(code)
  const metadata = await readMetadata(env, code)
  await ensureCommunityEntry(env, metadata)
  return jsonResponse({
    code: shown,
    compactCode: code,
    url: `${origin}/s/${code}`,
    downloadUrl: `${origin}/download/${code}.gbfr-loadout`,
    bytes: bytes.byteLength,
    reused,
    title: metadata?.title || '',
    characterName: metadata?.characterName || '',
  }, reused ? 200 : 201, { 'Cache-Control': 'no-store' })
}

function previewHeader(preview) {
  const bytes = new TextEncoder().encode(JSON.stringify(preview))
  let binary = ''
  for (const byte of bytes) binary += String.fromCharCode(byte)
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
}

async function importJSON(request, env, origin) {
  const contentType = (request.headers.get('Content-Type') || '').toLowerCase()
  if (!contentType.startsWith('application/json')) return errorResponse('只接受 application/json 配装文件', 415)
  try {
    const body = await readBoundedRequestBody(request, MAX_RAW_BYTES, '配装 JSON 超过 1 MiB')
    if (body.response) return body.response
    const converted = await loadoutJSONToFrame(body.bytes)
    const headers = new Headers({
      'Content-Type': 'application/octet-stream',
      'X-Loadout-Preview': previewHeader(converted.preview),
      'X-Loadout-Character-Hash': converted.preview.characterHash,
    })
    return publish(new Request(`${origin}/api/v1/loadouts`, { method: 'POST', headers, body: converted.frame }), env, origin)
  } catch (error) {
    return errorResponse(error instanceof Error ? error.message : '配装文件转换失败', 400)
  }
}

async function loadFrame(env, code) {
  const object = await env.LOADOUTS.get(objectKey(code))
  if (!object) return null
  const bytes = await readObjectBytes(object)
  if (!bytes || validateFrame(bytes)) return null
  return { object, bytes }
}

function binaryResponse(frame, attachmentName = '') {
  const headers = new Headers(baseHeaders)
  headers.set('Content-Type', 'application/vnd.gbfr.loadout')
  headers.set('Cache-Control', 'public, max-age=86400, immutable')
  if (frame.object.httpEtag) headers.set('ETag', frame.object.httpEtag)
  if (attachmentName) {
    headers.set('Content-Disposition', `attachment; filename="${attachmentName}"`)
  }
  return new Response(frame.bytes, { status: 200, headers })
}

function rosterNav(activeSlug = '', lang = 'zh') {
  return CHARACTER_ROSTER.map(character => {
    const name = lang === 'en' ? character.nameEn : character.name
    return `<a class="roster-chip${character.slug === activeSlug ? ' active' : ''}" href="${withLanguage(`/c/${character.slug}`, lang)}" title="${name}"${character.slug === activeSlug ? ' aria-current="page"' : ''}><img src="/assets/avatars/${character.iconFile}" alt="${name}" width="46" height="46"><span>${name}</span></a>`
  }).join('')
}

function rosterBar(activeSlug = '', allActive = false, lang = 'zh') {
  return `<div class="roster-bar"><a class="roster-all${allActive ? ' active' : ''}" href="${withLanguage('/', lang)}"${allActive ? ' aria-current="page"' : ''}><strong>All</strong><span>${lang === 'en' ? 'All' : '全部'}</span></a><nav class="roster" aria-label="${lang === 'en' ? 'Choose a character' : '选择角色'}">${rosterNav(activeSlug, lang)}</nav></div>`
}

function showcaseStyles() {
  return `<style>
  @font-face{font-family:"GBFR UI Latin";src:url('/assets/fonts/gbfr-ui.woff2') format('woff2');font-style:normal;font-weight:400 800;font-display:swap}
  :root{font-family:"GBFR UI Latin","Microsoft YaHei UI","Microsoft YaHei","Noto Sans SC",sans-serif;color:#3f3932;background:#e9dfcc;color-scheme:light;--paper:#f7f0df;--paper-deep:#efe2c8;--ink:#3f3932;--ink-soft:#655b50;--brass:#896331;--line:rgba(105,76,37,.24)}
  *{box-sizing:border-box}::selection{background:rgba(167,123,57,.28)}
  html{scrollbar-width:thin;scrollbar-color:rgba(126,89,40,.42) transparent}
  ::-webkit-scrollbar{width:8px;height:8px}::-webkit-scrollbar-track{background:transparent}::-webkit-scrollbar-thumb{border:2px solid transparent;border-radius:8px;background:rgba(126,89,40,.42);background-clip:content-box}::-webkit-scrollbar-thumb:hover{background-color:rgba(110,78,40,.62)}
  body{margin:0;min-height:100vh;background:#e9dfcc url('/assets/backgrounds/parchment-archive.webp') center/cover fixed;color:var(--ink)}
  body:before{content:"";position:fixed;inset:0;pointer-events:none;background:linear-gradient(90deg,rgba(80,50,22,.1),transparent 13%,transparent 87%,rgba(80,50,22,.1));mix-blend-mode:multiply}
  a,button,input,textarea{font:inherit}a{color:inherit;text-decoration:none}button{cursor:pointer}.page{position:relative;width:min(1400px,100%);margin:auto;padding:18px clamp(14px,3vw,38px) 42px}
  .masthead{display:flex;align-items:end;justify-content:space-between;gap:20px;padding:8px 0 14px;border-bottom:1px solid var(--line)}.eyebrow{color:#7f5d31;font-size:10px;font-weight:800;letter-spacing:.15em;text-transform:uppercase}.masthead h1{margin:4px 0 0;color:var(--ink);font-size:clamp(26px,3.4vw,40px);line-height:1.08}.masthead p{max-width:500px;margin:0;color:var(--ink-soft);font-size:13px;line-height:1.6}
  .masthead-tools{display:flex;align-items:flex-end;gap:14px}.language-switch{display:inline-flex;padding:2px;border:1px solid rgba(105,76,37,.25);border-radius:5px;background:rgba(255,253,247,.72)}.language-switch a{min-width:42px;padding:5px 8px;border-radius:3px;color:#7f5d31;font-size:10px;font-weight:800;text-align:center}.language-switch a.active{background:#896331;color:#fffaf0}
  .roster-bar{--roster-item-height:73px;display:grid;grid-template-columns:70px minmax(0,1fr);gap:6px;align-items:start}.roster{display:flex;gap:6px;min-width:0;overflow-x:auto;overflow-y:hidden;padding:12px 1px 9px;scroll-behavior:smooth;scroll-snap-type:x proximity;overscroll-behavior-x:contain;scrollbar-width:thin;scrollbar-color:transparent transparent;mask-image:linear-gradient(90deg,transparent,#000 18px,#000 calc(100% - 18px),transparent)}
  .roster-all{display:grid;height:var(--roster-item-height);place-content:center;justify-items:center;gap:3px;margin:12px 0 9px;padding:4px 3px 6px;border:1px solid rgba(105,76,37,.14);border-radius:7px;background:rgba(255,253,247,.52);color:var(--ink-soft);font-size:10px;font-weight:700;transition:background-color .16s,border-color .16s,color .16s,transform .16s}.roster-all strong{font-size:18px;line-height:1}.roster-all:hover{border-color:rgba(127,93,49,.55);background:rgba(255,250,238,.82);color:#6e4e28;transform:translateY(-1px)}.roster-all.active{border-color:#6e4e28;background:#896331;color:#fffaf0}.roster-all:focus-visible{outline:2px solid #98703a;outline-offset:2px}
  .roster:hover,.roster:focus-within{scrollbar-color:rgba(59,42,26,.3) transparent}.roster::-webkit-scrollbar{height:3px}.roster::-webkit-scrollbar-track{background:transparent}.roster::-webkit-scrollbar-thumb{background:transparent;border-radius:2px}.roster:hover::-webkit-scrollbar-thumb,.roster:focus-within::-webkit-scrollbar-thumb{background:rgba(59,42,26,.3)}
  .roster-chip{flex:0 0 70px;display:grid;height:var(--roster-item-height);justify-items:center;gap:3px;padding:4px 3px 6px;border:1px solid rgba(105,76,37,.14);border-radius:7px;background:rgba(255,253,247,.52);color:var(--ink-soft);font-size:10px;font-weight:700;scroll-snap-align:center;transition:background-color .16s,border-color .16s,color .16s,transform .16s}.roster-chip img{display:block;width:46px;height:46px;object-fit:contain;border-radius:5px;filter:drop-shadow(0 2px 2px rgba(72,50,22,.16))}.roster-chip:hover{border-color:rgba(127,93,49,.55);background:rgba(255,250,238,.82);transform:translateY(-1px)}.roster-chip.active{border-color:#6e4e28;background:#896331;color:#fffaf0}.roster-chip:focus-visible{outline:2px solid #98703a;outline-offset:2px}
  .showcase{--character-accent:#896331;position:relative;margin-top:11px;overflow:hidden;border:1px solid var(--line);border-top:3px solid var(--character-accent);border-radius:8px;background:rgba(255,253,247,.86);box-shadow:0 14px 34px rgba(72,50,22,.12)}
  .panel{position:relative;width:100%;min-width:0;padding:clamp(20px,2.7vw,34px)}.panel-head{display:flex;align-items:center;gap:12px}.panel-head img{display:block;width:58px;height:58px;object-fit:contain;border:1px solid var(--line);border-radius:7px;background:#f4ead6}.panel-head small{display:block;color:#98703a;font-size:10px;font-weight:800;letter-spacing:.08em}.panel-head h2{margin:3px 0 0;color:var(--ink);font-size:clamp(24px,2.8vw,34px)}.panel-copy{max-width:760px;margin:12px 0 16px;color:var(--ink-soft);font-size:14px;line-height:1.65}.panel-copy b{color:#6e4e28}
  .code{display:flex;flex-wrap:wrap;align-items:center;justify-content:space-between;gap:12px;margin:16px 0;padding:12px 14px;border:1px solid rgba(126,89,40,.4);border-left:4px solid var(--character-accent);background:rgba(244,234,214,.88)}.code strong{color:#6e4e28;font:700 clamp(18px,2.6vw,26px) ui-monospace,Consolas,monospace;letter-spacing:.08em;overflow-wrap:anywhere}.actions{display:flex;flex-wrap:wrap;gap:8px}.button{display:inline-flex;align-items:center;justify-content:center;box-sizing:border-box;min-height:36px;padding:0 12px;border:1px solid #7f5d31;border-radius:5px;background:#fffdf7;color:#6e4e28;font-weight:800;line-height:1}.button.primary{background:#7f5d31;color:#fffaf0}.button:focus-visible,input:focus-visible,textarea:focus-visible{outline:2px solid #98703a;outline-offset:2px}
  .data-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:8px;margin-top:14px}.tile{min-width:0;padding:11px;border:1px solid rgba(105,76,37,.16);border-radius:6px;background:rgba(244,234,214,.78)}.tile small{display:block;color:#98703a;font-size:10px;font-weight:800;letter-spacing:.06em;text-transform:uppercase}.tile b{display:block;margin-top:4px;color:var(--ink);font-size:14px;line-height:1.45;overflow-wrap:anywhere}.wide{grid-column:1/-1}.sigils{display:flex;flex-wrap:wrap;gap:6px;margin-top:8px}.sigils span{padding:4px 7px;border:1px solid rgba(126,89,40,.24);border-radius:4px;background:#fffdf7;color:var(--ink-soft);font-size:11px}
  .sticker{display:flex;align-items:center;gap:9px;margin-top:14px;padding:9px 10px;border-left:3px solid var(--character-accent);background:rgba(251,246,233,.88)}.sticker img{display:block;width:54px;height:54px;object-fit:contain;border-radius:5px}.sticker b{display:block;color:#6e4e28;font-size:13px}.sticker span{display:block;margin-top:3px;color:var(--ink-soft);font-size:12px}.community{margin-top:14px;padding-top:14px;border-top:1px solid rgba(105,76,37,.16)}.community h3{margin:0 0 9px;color:#6e4e28;font-size:16px}.community-actions{display:flex;flex-wrap:wrap;align-items:center;gap:8px}.community small{color:#98703a;font-size:11px}.community textarea{width:100%;min-height:68px;margin-top:8px;padding:9px;border:1px solid rgba(126,89,40,.26);border-radius:5px;background:#fffdf7;color:var(--ink)}.comments{display:grid;gap:7px;margin-top:9px}.comment{padding:9px;border-left:3px solid #c9a25e;background:#f4ead6}.comment small{color:#98703a}.comment p{margin:3px 0 0;color:var(--ink-soft);font-size:12px}
  .character-bar{display:grid;grid-template-columns:auto minmax(150px,1fr) minmax(230px,380px);align-items:center;gap:13px;margin-top:11px;padding:13px 15px;border:1px solid var(--line);border-left:4px solid var(--character-accent,#896331);background:rgba(255,253,247,.76)}.character-bar img{width:58px;height:58px;object-fit:contain;border-radius:6px}.character-bar>div{min-width:0}.character-bar small{color:#98703a;font-size:10px;font-weight:800;letter-spacing:.08em}.character-bar h2{margin:2px 0;color:var(--ink);font-size:25px}.character-bar p{margin:0;color:var(--ink-soft);font-size:12px}.character-tools{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:7px;align-items:center}.character-tools input{width:100%;min-height:36px;padding:0 9px;border:1px solid rgba(126,89,40,.26);background:#fffdf7;color:#3f3932}.character-tools em{grid-column:1/-1;color:#7f5d31;font-size:10px;font-style:normal;font-weight:800;text-align:right}
  .catalog{display:grid;grid-template-columns:repeat(auto-fill,minmax(300px,1fr));gap:12px;margin-top:14px}.loadout-card{min-width:0;min-height:232px;display:flex;flex-direction:column;padding:15px;border:1px solid rgba(105,76,37,.2);border-top:3px solid var(--card-accent,#896331);border-radius:7px;background:rgba(255,253,247,.84);box-shadow:0 5px 14px rgba(72,50,22,.06);transition:transform .16s,border-color .16s,box-shadow .16s}.loadout-card:hover{transform:translateY(-2px);border-color:#98703a;box-shadow:0 9px 20px rgba(72,50,22,.1)}.loadout-card-head{display:grid;grid-template-columns:48px minmax(0,1fr);gap:10px;align-items:center}.loadout-card-head img{display:block;width:48px;height:48px;object-fit:contain;border-radius:6px;background:#f4ead6}.loadout-card-head small{display:block;color:#98703a;font-size:10px;font-weight:800}.loadout-card h3{display:-webkit-box;min-height:44px;margin:3px 0 0;overflow:hidden;color:var(--ink);font-size:17px;line-height:1.3;-webkit-box-orient:vertical;-webkit-line-clamp:2}.loadout-weapon{display:flex;justify-content:space-between;gap:10px;margin:12px 0 8px;padding:8px 9px;border-left:3px solid #a77b39;background:rgba(244,234,214,.76)}.loadout-weapon small{color:#98703a;font-size:10px}.loadout-weapon b{overflow:hidden;color:#4c3c2d;font-size:13px;text-align:right;text-overflow:ellipsis;white-space:nowrap}.card-tags{display:flex;flex-wrap:wrap;gap:5px}.card-tags span{max-width:100%;overflow:hidden;padding:3px 6px;border:1px solid rgba(126,89,40,.22);border-radius:4px;background:#fffdf7;color:var(--ink-soft);font-size:10px;text-overflow:ellipsis;white-space:nowrap}.card-tags span.mastery{border-color:rgba(93,122,73,.38);background:rgba(93,122,73,.09);color:#47613b}.loadout-card .meta{display:flex;justify-content:space-between;gap:8px;margin-top:auto;padding-top:12px;color:#98703a;font:10px ui-monospace,Consolas,monospace}.empty{padding:42px 12px;text-align:center;color:#98703a}.footer{margin-top:24px;color:#98703a;font-size:11px;line-height:1.6}
  .detail-head{display:flex;align-items:center;gap:13px}.detail-avatar-stack{align-self:flex-start;display:grid;justify-items:start;gap:6px}.detail-back{display:inline-flex;align-items:center;min-height:22px;padding:0 6px;border-left:2px solid rgba(137,99,49,.58);color:#7f5d31;font-size:10px;font-weight:800;line-height:1.25;transition:background-color .16s,color .16s}.detail-back:hover{background:rgba(137,99,49,.09);color:#4f371c}.detail-back:focus-visible{outline:2px solid #98703a;outline-offset:2px}.detail-avatar-stack img{width:64px;height:64px;object-fit:contain;border-radius:7px;background:#f4ead6}.detail-title{min-width:0}.detail-title small{color:#98703a;font-size:10px;font-weight:800}.detail-title h2{margin:3px 0;color:var(--ink);font-size:clamp(22px,2.6vw,32px)}.detail-actions{margin-left:auto;display:flex;flex-wrap:wrap;justify-content:flex-end;gap:7px}.detail-code{padding:6px 9px;border:1px solid rgba(126,89,40,.32);background:#f4ead6;color:#6e4e28;font:700 13px ui-monospace,Consolas,monospace;letter-spacing:.06em}.detail-layout{display:grid;grid-template-columns:minmax(250px,.86fr) minmax(280px,1fr) minmax(320px,1.18fr);gap:12px;margin-top:16px;align-items:start}.detail-column{display:flex;min-width:0;flex-direction:column;gap:10px}.detail-section{min-width:0;padding:12px;border:1px solid rgba(105,76,37,.17);border-radius:6px;background:rgba(244,234,214,.52)}.detail-section h3{display:flex;align-items:center;justify-content:space-between;gap:8px;margin:0 0 9px;color:#6e4e28;font-size:14px}.detail-section h3 small{color:#98703a;font-size:10px;font-weight:500}.detail-section .lead{display:block;color:var(--ink);font-size:16px}.detail-section .sub{display:block;margin-top:3px;color:var(--ink-soft);font-size:11px}.detail-icon{width:28px;height:28px;flex:0 0 28px;object-fit:contain;border:1px solid rgba(126,89,40,.2);border-radius:5px;background:#f4ead6}.detail-icon.is-large{width:36px;height:36px;flex-basis:36px}.row-title,.effect-row,.summon-row,.mastery-row,.factor{display:flex;align-items:flex-start;gap:8px}.row-title{min-width:0}.row-title>div,.effect-copy,.summon-copy,.mastery-copy,.factor-copy{min-width:0;flex:1}.factor-grid{display:grid;grid-template-columns:1fr;gap:7px}.factor{min-width:0;padding:8px;border-bottom:1px solid rgba(126,89,40,.18);background:rgba(255,253,247,.72)}.factor header{display:flex;justify-content:space-between;gap:8px}.factor b{overflow:hidden;color:var(--ink);font-size:12px;text-overflow:ellipsis;white-space:nowrap}.factor em{color:#98703a;font-size:10px;font-style:normal}.factor p{margin:5px 0 0;color:var(--ink-soft);font-size:10px;line-height:1.4}.detail-chips{display:flex;flex-wrap:wrap;gap:6px}.detail-chips span.detail-chip{display:inline-flex;align-items:center;gap:5px;padding:4px 7px;border:1px solid rgba(126,89,40,.22);border-radius:4px;background:#fffdf7;color:var(--ink-soft);font-size:11px}.detail-chip small{font-size:9px}.trait-pair,.summon-traits{display:flex;flex-wrap:wrap;gap:6px;margin-top:6px}.trait-pair-item{display:inline-flex;align-items:center;gap:4px;max-width:100%;color:var(--ink-soft);font-size:10px}.trait-pair-item .detail-icon{width:20px;height:20px;flex-basis:20px}.effect-rows,.summon-list,.mastery-list,.skill-ledger{display:grid;gap:6px}.effect-row,.summon-row,.mastery-row{min-width:0;padding:8px 9px;border-left:2px solid rgba(137,99,49,.58);background:rgba(255,253,247,.68)}.effect-row header,.summon-row header,.mastery-row header{display:flex;align-items:baseline;justify-content:space-between;gap:8px}.effect-row b,.summon-row b,.mastery-row b{color:var(--ink);font-size:12px}.effect-row em,.summon-row em,.mastery-row em{flex:none;color:#98703a;font-size:10px;font-style:normal}.effect-row p,.summon-row p,.mastery-row p{margin:4px 0 0;color:var(--ink-soft);font-size:10px;line-height:1.45}.skill-entry{display:grid;grid-template-columns:auto minmax(0,1fr) auto;border:0;border-left:3px solid #94703f;background:rgba(255,253,247,.75)}.skill-entry summary{display:contents;cursor:pointer;list-style:none}.skill-entry summary::-webkit-details-marker{display:none}.skill-entry summary>.detail-icon{margin:9px 0 0 10px}.skill-entry summary>span:nth-of-type(1){padding:9px 8px}.skill-entry summary b{display:block;color:var(--ink);font-size:12px}.skill-entry summary small{display:block;margin-top:2px;color:#98703a;font-size:9px}.skill-level{text-align:right;padding:9px 10px}.skill-level strong{display:block;color:#6e4e28;font-size:13px}.skill-body{grid-column:1/-1;padding:0 10px 10px;border-top:1px solid rgba(126,89,40,.12)}.skill-body p{margin:8px 0 0;color:var(--ink-soft);font-size:10px;line-height:1.55}.skill-sources{display:flex;flex-wrap:wrap;gap:4px;margin-top:7px}.skill-sources span{padding:2px 5px;background:#efe2c8;color:#756044;font-size:9px}
  .skill-entry{display:block}.skill-entry summary{display:grid;grid-template-columns:auto minmax(0,1fr) auto;gap:8px;align-items:center;padding:9px 10px}.skill-entry summary>.detail-icon{margin:0}.skill-entry summary>span:nth-of-type(1){padding:0}.skill-level{padding:0}.skill-body{padding:0 10px 10px}
  @media(max-width:1100px){.detail-layout{grid-template-columns:minmax(240px,.9fr) minmax(280px,1.1fr)}.detail-layout>.detail-column:last-child{grid-column:1/-1}}
  @media(max-width:900px){.masthead{align-items:start}.catalog{grid-template-columns:repeat(auto-fill,minmax(270px,1fr))}.detail-layout{grid-template-columns:1fr}.detail-layout>.detail-column:last-child{grid-column:auto}.detail-head{align-items:flex-start}.detail-actions{max-width:330px}.character-bar{grid-template-columns:auto minmax(0,1fr)}.character-tools{grid-column:1/-1}}
  @media(max-width:620px){.page{padding:12px 12px 34px}.masthead{display:block}.masthead p{margin-top:8px}.roster-bar{--roster-item-height:69px;grid-template-columns:60px minmax(0,1fr)}.roster{padding-top:10px}.roster-all{margin-top:10px}.roster-chip{flex-basis:64px}.roster-chip img{width:42px;height:42px}.showcase{margin-top:7px}.panel{padding:16px}.data-grid,.factor-grid{grid-template-columns:1fr}.wide{grid-column:auto}.panel-head img{width:52px;height:52px}.character-bar{align-items:flex-start}.character-tools{grid-template-columns:minmax(0,1fr) auto}.catalog{grid-template-columns:1fr}.detail-head{display:grid;grid-template-columns:minmax(88px,auto) minmax(0,1fr)}.detail-avatar-stack img{width:54px;height:54px}.detail-actions{grid-column:1/-1;margin:2px 0 0;justify-content:flex-start;max-width:none}.sticker{padding-right:8px}}
  @media(hover:none){.roster{scrollbar-color:rgba(59,42,26,.2) transparent}.roster::-webkit-scrollbar-thumb{background:rgba(59,42,26,.2)}}
  </style>`
}

function catalogScript(origin, fixedCharacter = '', lang = 'zh') {
  return `<script>
  const fixedCharacter=${JSON.stringify(fixedCharacter)};
  const lang=${JSON.stringify(lang)};
  const t=${JSON.stringify(lang === 'en' ? {
    unnamed:'Untitled Loadout', weapon:'Weapon', unknownWeapon:'Not recorded', emptySummary:'Full preview unavailable',
    mastery:'Mastery', nodes:'nodes', publicCount:'public loadouts', empty:'No public loadouts yet'
  } : {
    unnamed:'未命名配装', weapon:'装备武器', unknownWeapon:'未标注', emptySummary:'等待完整配装摘要',
    mastery:'专精', nodes:'节点', publicCount:'套公开配装', empty:'暂时还没有公开配装'
  })};
  const esc=s=>String(s??'').replace(/[&<>\"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','\"':'&quot;',"'":'&#39;'}[c]));
  const label=s=>String(s??'').replaceAll('Catastrophe','浩劫').replaceAll('DMG Cap','伤害上限').replace(/(?:\||｜)HP$/i,'｜体力');
  function renderCard(x){
    const p=x.preview||{};
    const tags=[];
    if(p.masteryLabel)tags.push('<span class="mastery">'+esc(p.masteryLabel)+'</span>');
    else if(p.masteryCount)tags.push('<span class="mastery">'+t.mastery+' '+esc(p.masteryCount)+' '+t.nodes+'</span>');
    (p.weaponSkills||[]).slice(0,2).forEach(s=>tags.push('<span>'+esc(s.name)+(s.level?' Lv'+esc(s.level):'')+'</span>'));
    (p.sigils||[]).slice(0,3).forEach(s=>tags.push('<span>'+esc(s.name)+(s.level?' Lv'+esc(s.level):'')+'</span>'));
    const code=x.code.replace(/(.{4})/g,'$1-').replace(/-$/,'');
    return '<a class="loadout-card" style="--card-accent:'+esc(x.accent||'#896331')+'" href="/s/'+esc(x.code)+'?lang='+lang+'"><header class="loadout-card-head"><img src="'+esc(x.avatarPath)+'" alt="'+esc(x.characterName)+'"><div><small>'+esc(x.characterName)+'</small><h3>'+esc(label(x.title||t.unnamed))+'</h3></div></header><div class="loadout-weapon"><small>'+t.weapon+'</small><b>'+esc(x.weaponName||t.unknownWeapon)+'</b></div><div class="card-tags">'+(tags.join('')||'<span>'+t.emptySummary+'</span>')+'</div><div class="meta"><span>'+code+'</span><span>'+new Date(x.createdAt).toLocaleDateString(lang==='en'?'en-US':'zh-CN')+'</span></div></a>';
  }
  async function load(){
    const query=document.querySelector('#q')?.value.trim()||'';
    const params=new URLSearchParams();
    if(fixedCharacter)params.set('character',fixedCharacter);
    if(query)params.set('q',query);
    params.set('lang',lang);
    const suffix=params.toString()?'?'+params.toString():'';
    const r=await fetch('${origin}/api/v1/loadouts'+suffix);
    const d=await r.json();const g=document.querySelector('#grid');
    const count=document.querySelector('#character-count');if(count)count.textContent=(d.items?.length||0)+' '+t.publicCount;
    if(!d.items?.length){g.innerHTML='<div class="empty">'+t.empty+'</div>';return}
    g.innerHTML=d.items.map(renderCard).join('');
  }
  document.querySelector('#q')?.addEventListener('keydown',event=>{if(event.key==='Enter')load()});
  requestAnimationFrame(()=>document.querySelector('.roster .active')?.scrollIntoView({block:'nearest',inline:'center'}));
  load();
  </script>`
}

function communityScript(origin, code, lang = 'zh') {
  const author = lang === 'en' ? 'Anonymous Traveler' : '匿名旅人'
  const disabled = lang === 'en' ? 'Community features are not enabled yet. Short codes and previews still work.' : '互动功能尚未启用，短码与预览不受影响。'
  return `<script>(async()=>{const key='gbfr-share-visitor';let visitor=localStorage.getItem(key);if(!visitor){visitor=crypto.randomUUID();localStorage.setItem(key,visitor)}const esc=s=>String(s).replace(/[&<>\"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','\"':'&quot;',"'":'&#39;'}[c]));async function refresh(){try{const r=await fetch('${origin}/api/v1/loadouts/${code}/community');const d=await r.json();document.querySelector('#like span').textContent=d.likes||0;document.querySelector('#comments').innerHTML=(d.comments||[]).map(c=>'<article class="comment"><small>'+esc(c.author||'${author}')+'</small><p>'+esc(c.body)+'</p></article>').join('');if(!d.enabled)document.querySelector('#community-note').textContent='${disabled}'}catch{}}document.querySelector('#like').onclick=async()=>{await fetch('${origin}/api/v1/loadouts/${code}/like',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({visitorKey:visitor})});refresh()};document.querySelector('#send').onclick=async()=>{const box=document.querySelector('#comment');if(!box.value.trim())return;await fetch('${origin}/api/v1/loadouts/${code}/comments',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({visitorKey:visitor,body:box.value,author:'${author}'})});box.value='';refresh()};refresh()})()</script>`
}

function detailScript(origin, code, lang = 'zh') {
  return `<script>
  (async()=>{
    const lang=${JSON.stringify(lang)};
    const en=lang==='en';
    const t=${JSON.stringify(lang === 'en' ? {
      missing:'Not recorded', unnamedSkill:'Unnamed skill', skill:'Trait', levelMissing:'Level not recorded', sigil:'Sigil', sigilMissing:'Sigil level not recorded',
      primary:'Primary', secondary:'Secondary', subMissing:'Sub trait not recorded', rank:'Rank', summonMissing:'Summon data not recorded', mastery:'Mastery', masteryNode:'Mastery node',
      sources:'recorded sources', sourceUnavailable:'Source details unavailable', invested:'Invested', effectMissing:'No value description at this level', mergedMissing:'Merged skill levels unavailable in this legacy preview',
      weapon:'Weapon', weaponSkills:'Weapon Traits', wrightstone:'Wrightstone', abilities:'Character Skills', summons:'Summons', summonMeta:'Main Aura / Sub Trait / Rank',
      sigils:'Sigils', slots:'12 slots', masterySkills:'Mastery Skills', direction:'Direction', nodes:'nodes', overLimit:'Over Mastery', overLimitMeta:'4 slots', merged:'Combined Skill Levels', mergedMeta:'Sigils / Weapon / Wrightstone / Summons',
      unknownWeapon:'Not recorded', oldWrightstone:'Not equipped or unavailable in this legacy preview', noMastery:'No selected mastery nodes', oldMastery:'Mastery nodes unavailable in this legacy preview', noDirection:'No primary direction', failed:'Failed to load loadout preview'
    } : {
      missing:'未记录', unnamedSkill:'未命名技能', skill:'技能', levelMissing:'等级未记录', sigil:'因子', sigilMissing:'因子等级未记录',
      primary:'主词条', secondary:'副词条', subMissing:'副词条未记录', rank:'阶级', summonMissing:'未记录召唤石配置', mastery:'专精', masteryNode:'专精节点',
      sources:'个记录来源', sourceUnavailable:'来源明细未记录', invested:'投入', effectMissing:'当前等级暂无数值说明', mergedMissing:'这份旧摘要未记录合并技能等级',
      weapon:'装备武器', weaponSkills:'武器技能', wrightstone:'武器祝福', abilities:'角色技能', summons:'召唤石', summonMeta:'主加护 / 副词条 / 阶级',
      sigils:'因子配置', slots:'12 槽', masterySkills:'专精技能', direction:'方向', nodes:'节点', overLimit:'上限突破', overLimitMeta:'4 槽', merged:'合并技能等级', mergedMeta:'因子 / 武器 / 祝福 / 召唤石',
      unknownWeapon:'未标注', oldWrightstone:'未佩戴或旧摘要未记录', noMastery:'这套配装没有已选专精节点', oldMastery:'这份旧摘要未记录专精节点', noDirection:'未形成主方向', failed:'配装摘要读取失败'
    })};
    const esc=s=>String(s??'').replace(/[&<>\"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','\"':'&quot;',"'":'&#39;'}[c]));
    const label=s=>en?String(s??''):String(s??'')
      .replaceAll('Dread Wrightstone','畏惧之祝福')
      .replaceAll('Vitality Wrightstone','生机之祝福')
      .replaceAll('Fortification Wrightstone','镇守之祝福')
      .replaceAll('Sequestration Wrightstone','隔绝之祝福')
      .replaceAll('Catastrophe','浩劫')
      .replaceAll('DMG Cap','伤害上限')
      .replaceAll('Quick Cooldown','快速冷却')
      .replaceAll('Quick Charge','快速蓄力')
      .replaceAll('Berserker Echo','狂战士')
      .replaceAll('Spartan Echo','斯巴达')
      .replaceAll('Supplementary DMG','追击')
      .replaceAll('Stun Power','昏厥')
      .replaceAll('Low Profile','匿踪')
      .replaceAll('Sigil Booster','因子强化');
    const empty=text=>'<span class="sub">'+esc(text||t.missing)+'</span>';
    const icon=(src,alt='')=>src?'<img class="detail-icon" loading="lazy" src="'+esc(src)+'" alt="'+esc(alt)+'">':'';
    const chips=items=>items.length?'<div class="detail-chips">'+items.map(x=>{const v=typeof x==='string'?{name:x}:x;return '<span class="detail-chip">'+icon(v.icon,v.name)+ '<span>'+esc(label(v.name||t.unnamedSkill))+(v.level?' <small>Lv'+esc(v.level)+'</small>':'')+'</span></span>'}).join('')+'</div>':empty();
    const effectRows=items=>items.length?'<div class="effect-rows">'+items.map(x=>'<article class="effect-row">'+icon(x.icon,x.name)+'<div class="effect-copy"><header><b>'+esc(label(x.name||t.unnamedSkill))+'</b><em>'+(x.level?'Lv'+esc(x.level):'')+'</em></header>'+(x.effect?'<p>'+esc(label(x.effect))+'</p>':'')+'</div></article>').join('')+'</div>':empty();
    const traitPair=(labelText,name,src,level)=>name?'<span class="trait-pair-item">'+icon(src,name)+'<span>'+esc(labelText+' · '+label(name))+' <small>'+(level?t.skill+' Lv'+esc(level):t.levelMissing)+'</small></span></span>':'';
    const factors=items=>items.length?'<div class="factor-grid">'+items.map((s,i)=>{const primaryLevel=Number(s.primaryLevel)||Number(s.level)||0;const secondaryLevel=s.secondary?(Number(s.secondaryLevel)||primaryLevel):0;return '<article class="factor">'+icon(s.icon||s.primaryIcon,s.name)+'<div class="factor-copy"><header><b>'+esc((i+1)+'. '+label(s.name||t.sigil))+'</b><em>'+(s.level?t.sigil+' Lv'+esc(s.level):t.sigilMissing)+'</em></header><div class="trait-pair">'+traitPair(t.primary,s.primary,s.primaryIcon,primaryLevel)+' '+traitPair(t.secondary,s.secondary,s.secondaryIcon,secondaryLevel)+'</div></div></article>'}).join('')+'</div>':empty();
    const summons=items=>items.length?'<div class="summon-list">'+items.map(s=>{const sub=s.subParam?esc(s.subParam)+(s.subParamValue?' '+(s.subParamValue>0?'+':'')+esc(s.subParamValue)+(s.subParamUnit==='pct'?'%':''):'')+(s.subParamLevel?' · Lv'+esc(s.subParamLevel):''):t.subMissing;return '<article class="summon-row">'+icon(s.icon,s.name)+'<div class="summon-copy"><header><b>'+esc(s.name||t.summons)+'</b><em>'+t.rank+' '+esc(s.rank||0)+'</em></header><div class="summon-traits">'+traitPair(en?'Main Aura':'主加护',s.mainTrait,s.mainIcon,s.mainTraitLevel)+traitPair(t.secondary,s.subParam,s.subIcon,s.subParamLevel)+'</div>'+(s.subParam?'<p>'+sub+'</p>':'')+'</div></article>'}).join('')+'</div>':empty(t.summonMissing);
    const mastery=(items,emptyText)=>items.length?'<div class="mastery-list">'+items.map(x=>'<article class="mastery-row">'+icon(x.icon,x.name||x.effect)+'<div class="mastery-copy"><header><b>'+esc(x.name||x.effect||t.masteryNode)+'</b><em>'+esc(x.rank||t.mastery)+(x.count>1?' ×'+esc(x.count):'')+'</em></header>'+(x.name&&x.effect?'<p>'+esc(label(x.effect))+'</p>':'')+'</div></article>').join('')+'</div>':empty(emptyText);
    const overLimit=items=>items.length?'<div class="effect-rows">'+items.map((x,i)=>'<article class="effect-row">'+icon(x.icon,x.name)+'<div class="effect-copy"><header><b>'+(i+1)+'. '+esc(label(x.name)||x.attributeHash)+'</b><em>Lv'+esc(x.level||0)+'</em></header><p>+'+esc(x.value||0)+(x.unit==='pct'?'%':'')+'</p></div></article>').join('')+'</div>':empty();
    const ledger=items=>items.length?'<div class="skill-ledger">'+items.map(x=>{const sources=x.sources||[];return '<details class="skill-entry"><summary>'+icon(x.icon,x.name)+'<span><b>'+esc(label(x.name))+'</b><small>'+(sources.length?esc(sources.length)+' '+t.sources:t.sourceUnavailable)+'</small></span><span class="skill-level"><strong>Lv'+esc(x.level||0)+'</strong>'+(x.rawLevel&&x.rawLevel!==x.level?'<small>'+t.invested+' '+esc(x.rawLevel)+'</small>':'')+'</span></summary><div class="skill-body">'+(x.effect?'<p>'+esc(label(x.effect))+'</p>':empty(t.effectMissing))+(sources.length?'<div class="skill-sources">'+sources.map(source=>'<span>'+esc(label(source))+'</span>').join('')+'</div>':'')+'</div></details>'}).join('')+'</div>':empty(t.mergedMissing);
    try{
      const r=await fetch('${origin}/api/v1/loadouts/${code}/meta?lang='+lang);
      const m=await r.json();
      const p=m.preview||{};
      const wrightstone=p.wrightstone&&p.wrightstone.name?'<div class="row-title">'+icon(p.wrightstone.icon,p.wrightstone.name)+'<b class="lead">'+esc(label(p.wrightstone.name))+'</b></div>'+effectRows(p.wrightstone.traits||[]):empty(t.oldWrightstone);
      const masteryEmpty=(p.masteryCount||0)===0?t.noMastery:t.oldMastery;
      const direction=p.masteryLabel||t.noDirection;
      document.querySelector('#preview').innerHTML='<div class="detail-column"><section class="detail-section"><h3>'+t.weapon+'</h3><div class="row-title">'+icon(p.weaponIcon,p.weaponName)+'<div><b class="lead">'+esc(p.weaponName||t.unknownWeapon)+'</b><span class="sub">'+esc(p.weaponHash||'')+'</span></div></div></section><section class="detail-section"><h3>'+t.weaponSkills+'</h3>'+effectRows(p.weaponSkills||[])+'</section><section class="detail-section"><h3>'+t.wrightstone+'</h3>'+wrightstone+'</section><section class="detail-section"><h3>'+t.abilities+'</h3>'+chips(p.abilities||p.skills||[])+'</section><section class="detail-section"><h3>'+t.summons+' <small>'+t.summonMeta+'</small></h3>'+summons(p.summons||[])+'</section></div><div class="detail-column"><section class="detail-section"><h3>'+t.sigils+' <small>'+t.slots+'</small></h3>'+factors(p.sigils||[])+'</section><section class="detail-section"><h3>'+t.masterySkills+' <small>'+t.direction+': '+esc(direction)+' · '+esc(p.masteryCount||0)+' '+t.nodes+'</small></h3>'+mastery(p.masterySkills||[],masteryEmpty)+'</section></div><div class="detail-column"><section class="detail-section"><h3>'+t.overLimit+' <small>'+t.overLimitMeta+'</small></h3>'+overLimit(p.overLimit||[])+'</section><section class="detail-section"><h3>'+t.merged+' <small>'+t.mergedMeta+'</small></h3>'+ledger(p.combinedSkills||[])+'</section></div>';
    }catch{document.querySelector('#preview').innerHTML='<div class="empty">'+t.failed+'</div>'}
  })()
  </script>`
}

function landingPage(origin, code, metadata = null, lang = 'zh') {
  const shown = displayCode(code)
  const character = characterByIdentity(metadata?.characterName, metadata?.characterHash)
  const characterName = lang === 'en' ? character.nameEn : character.name
  const title = (lang === 'en' ? cleanText(metadata?.title) : displayText(metadata?.title)) || (lang === 'en' ? `${characterName} Loadout` : `${characterName} 配装`)
  const safeTitle = escapeHtml(title)
  const download = `${origin}/download/${code}.gbfr-loadout`
  const en = lang === 'en'
  return `<!doctype html><html lang="${en ? 'en' : 'zh-CN'}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="color-scheme" content="light"><meta name="theme-color" content="#e9dfcc"><meta name="description" content="GBFR ${en ? 'loadout share' : '配装分享'} ${shown}"><title>${safeTitle}</title>${showcaseStyles()}</head><body><main class="page"><header class="masthead"><div><div class="eyebrow">GBFR · ${en ? 'LOADOUT DETAILS' : '配装详情'}</div><h1>${en ? 'Loadout Details' : '配装详情'}</h1></div><div class="masthead-tools"><p>${en ? 'Review equipment, sigils, skills, and mastery before importing.' : '按桌面应用的结构核对装备、因子、技能和专精，再决定是否导入。'}</p>${languageSwitch(`/s/${code}`, lang)}</div></header>${rosterBar(character.slug, false, lang)}<section class="showcase" style="--character-accent:${character.accent}"><div class="panel"><header class="detail-head"><div class="detail-avatar-stack"><a class="detail-back" href="${withLanguage(`/c/${character.slug}`, lang)}">← ${en ? `Back to ${characterName}` : `返回${characterName}配装`}</a><img src="/assets/avatars/${character.iconFile}" alt="${characterName}"></div><div class="detail-title"><small>${characterName} · ${en ? 'CHARACTER LOADOUT' : '角色配装'}</small><h2>${safeTitle}</h2></div><div class="detail-actions"><span class="detail-code">${shown}</span><a class="button primary" href="${download}">${en ? 'Download' : '下载配装'}</a><button class="button" type="button" onclick="navigator.clipboard.writeText('${shown}').then(()=>this.textContent='${en ? 'Copied' : '已复制'}')">${en ? 'Copy Code' : '复制短码'}</button></div></header><p class="panel-copy">${en ? 'This is a sanitized preview. The website never modifies save data.' : '这是一份脱敏预览。网页不会修改存档，导入范围仍由桌面工具确认。'}</p><div class="detail-layout" id="preview"><div class="empty">${en ? 'Loading loadout details…' : '正在读取配装详情…'}</div></div><section class="community"><h3>${en ? 'Guestbook' : '旅人留言板'}</h3><div class="community-actions"><button class="button" id="like" type="button">♡ ${en ? 'Like' : '喜欢'} <span>0</span></button><small id="community-note">${en ? 'No account required.' : '无需登录，浏览器会保存匿名标识。'}</small></div><textarea id="comment" maxlength="500" placeholder="${en ? 'Leave a plain-text comment' : '给这套配装留一句话（纯文本）'}"></textarea><button class="button" id="send" type="button" style="margin-top:8px">${en ? 'Post' : '发布留言'}</button><div id="comments" class="comments"></div></section></div></section><p class="footer">${en ? 'Only the compressed loadout frame and sanitized preview are stored online.' : '线上只保存压缩配装帧与脱敏预览，不包含存档、路径或本机身份信息。'}</p>${detailScript(origin, code, lang)}${communityScript(origin, code, lang)}</main></body></html>`
}

function catalogPage(origin, lang = 'zh') {
  const en = lang === 'en'
  const uploadScript = `<script>(()=>{const input=document.querySelector('#loadout-file'),zone=document.querySelector('#upload-zone'),status=document.querySelector('#upload-status');zone.querySelector('small').textContent='${en ? 'Drop or choose a v10/v11 .gbfr-loadout.json file. Save data is never uploaded.' : '拖入或选择 v10/v11 .gbfr-loadout.json；只上传脱敏配装，不上传存档。'}';async function upload(file){if(!file)return;status.textContent='${en ? 'Uploading and validating…' : '正在校验并上传…'}';try{const r=await fetch('/api/v1/loadouts/import',{method:'POST',headers:{'Content-Type':'application/json'},body:await file.arrayBuffer()});const d=await r.json();if(!r.ok)throw new Error(d.error||'${en ? 'Upload failed' : '上传失败'}');status.innerHTML='<a href="/s/'+d.compactCode+'?lang=${lang}">${en ? 'Uploaded. Open loadout' : '上传成功，打开配装'} · '+d.code+'</a>';await load()}catch(e){status.textContent=e.message}}input.onchange=()=>upload(input.files[0]);zone.onclick=()=>input.click();zone.ondragover=e=>{e.preventDefault();zone.classList.add('drag')};zone.ondragleave=()=>zone.classList.remove('drag');zone.ondrop=e=>{e.preventDefault();zone.classList.remove('drag');upload(e.dataTransfer.files[0])}})()</script>`
  return `<!doctype html><html lang="${en ? 'en' : 'zh-CN'}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="color-scheme" content="light"><meta name="theme-color" content="#e9dfcc"><title>GBFR ${en ? 'Loadout Archive' : '配装图鉴'}</title>${showcaseStyles()}<style>.upload-zone{margin-top:14px;padding:14px 18px;border:1px dashed rgba(126,89,40,.42);border-radius:7px;background:rgba(255,253,247,.7);color:var(--ink-soft);cursor:pointer;transition:.16s}.upload-zone:hover,.upload-zone.drag{border-color:#896331;background:#fffdf7;box-shadow:0 5px 14px rgba(72,50,22,.08)}.upload-zone strong{display:block;color:var(--ink);font-size:14px}.upload-zone small{display:block;margin-top:3px}.upload-status{margin-top:7px;min-height:18px;color:#6e4e28;font-size:12px}.upload-status a{color:#6e4e28;font-weight:800}</style></head><body><main class="page"><header class="masthead"><div><div class="eyebrow">GBFR · ${en ? 'COMMUNITY LOADOUT ARCHIVE' : '社区配装图鉴'}</div><h1>${en ? 'Loadout Archive' : '配装图鉴'}</h1></div><div class="masthead-tools"><p>${en ? 'Choose a character, then compare weapons, skills, sigils, and mastery.' : '先选角色，再用卡片快速比较武器、技能、因子和专精方向。'}</p>${languageSwitch('/', lang)}</div></header>${rosterBar('', true, lang)}<section class="upload-zone" id="upload-zone" tabindex="0"><strong>${en ? 'Publish an exported loadout' : '发布应用导出的单套配装'}</strong><small>${en ? 'Drop or choose a v10 .gbfr-loadout.json file. Save data is never uploaded.' : '拖入或选择 v10 .gbfr-loadout.json；只上传脱敏配装，不上传存档。'}</small><input id="loadout-file" type="file" accept=".json,.gbfr-loadout.json,application/json" hidden><div class="upload-status" id="upload-status"></div></section><div class="code"><strong>${en ? 'All Public Loadouts' : '全部公开配装'}</strong><div class="actions"><input id="q" aria-label="${en ? 'Search loadouts' : '搜索配装'}" placeholder="${en ? 'Search characters, weapons, sigils, traits, or mastery' : '搜索角色、武器、因子、祝福、技能、专精'}" style="min-height:38px;padding:0 10px;border:1px solid rgba(126,89,40,.26);background:#fffdf7;color:#3f3932"><button class="button primary" onclick="load()">${en ? 'Search' : '搜索'}</button></div></div><section id="grid" class="catalog"><div class="empty">${en ? 'Loading archive…' : '正在读取配装目录…'}</div></section><p class="footer">${en ? 'Open a card for the complete preview. Import choices remain in the desktop app.' : '点开卡片查看完整预览；导入仍由桌面工具逐项确认。'}</p>${catalogScript(origin, '', lang)}${uploadScript}</main></body></html>`
}

function characterPage(origin, slug, lang = 'zh') {
  const character = CHARACTER_ROSTER.find(item => item.slug === slug) || CHARACTER_ROSTER[0]
  const en = lang === 'en'
  const name = en ? character.nameEn : character.name
  return `<!doctype html><html lang="${en ? 'en' : 'zh-CN'}"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="color-scheme" content="light"><meta name="theme-color" content="#e9dfcc"><title>${name} · GBFR ${en ? 'Loadouts' : '配装'}</title>${showcaseStyles()}</head><body><main class="page"><header class="masthead"><div><div class="eyebrow">GBFR · ${en ? 'CHARACTER LOADOUTS' : '角色配装'}</div><h1>${en ? `${name} Loadouts` : `${name} 的配装`}</h1></div><div class="masthead-tools"><p>${en ? 'Compare weapons, skills, sigils, and mastery for this character.' : '比较这个角色的武器、技能、因子和专精方向，点开卡片查看完整配置。'}</p>${languageSwitch(`/c/${character.slug}`, lang)}</div></header>${rosterBar(character.slug, false, lang)}<section class="character-bar" style="--character-accent:${character.accent}"><img src="/assets/avatars/${character.iconFile}" alt="${name}"><div><small>${en ? 'CHARACTER LOADOUT ARCHIVE' : '角色配装目录'} · ${character.plId.toUpperCase()}</small><h2>${name}</h2><p>${en ? 'Public loadouts' : '公开配装目录'}</p></div><div class="character-tools"><input id="q" aria-label="${en ? `Search ${name} loadouts` : '搜索当前角色配装'}" placeholder="${en ? 'Search weapons, sigils, traits, skills, or mastery' : '搜索此角色的武器、因子、祝福、技能或专精'}"><button class="button primary" onclick="load()">${en ? 'Search' : '搜索'}</button><em id="character-count">${en ? 'Loading…' : '正在读取…'}</em></div></section><section id="grid" class="catalog"><div class="empty">${en ? `Loading ${name} loadouts…` : `正在读取 ${name} 的配装…`}</div></section><p class="footer">${en ? `Use the fixed ` : '固定栏的 '}<a href="${withLanguage('/', lang)}" style="color:#6e4e28;font-weight:800">All</a>${en ? ' button to return to the full archive.' : ' 可返回全部配装。'}</p>${catalogScript(origin, en ? character.nameEn : character.name, lang)}</main></body></html>`
}

export default {
  async fetch(request, env) {
    if (request.method === 'OPTIONS') return new Response(null, { status: 204, headers: baseHeaders })
    const url = new URL(request.url)
    const origin = url.origin
    const lang = pageLanguage(url)

    if (url.pathname.startsWith('/assets/') && env.ASSETS) {
      return env.ASSETS.fetch(request)
    }
    if (request.method === 'GET' && url.pathname === '/favicon.ico') {
      return new Response(null, { status: 204, headers: { 'Cache-Control': 'public, max-age=86400' } })
    }
    if (request.method === 'GET' && url.pathname === '/health') {
      return jsonResponse({ ok: true, protocol: 'GBLC', frameVersions: [...FRAME_VERSIONS] }, 200, { 'Cache-Control': 'no-store' })
    }
    if (request.method === 'GET' && url.pathname === '/') {
      return new Response(catalogPage(origin, lang), { status: 200, headers: { ...baseHeaders, 'Content-Type': 'text/html; charset=utf-8', 'Cache-Control': 'public, max-age=60', 'Content-Security-Policy': "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; img-src 'self'; font-src 'self'; base-uri 'none'; frame-ancestors 'none'" } })
    }
    const characterMatch = url.pathname.match(/^\/(?:character|c)\/([a-z0-9-]+)$/i)
    if (request.method === 'GET' && characterMatch) {
      const slug = characterMatch[1].toLowerCase()
      if (!CHARACTER_ROSTER.some(character => character.slug === slug)) return errorResponse('角色页面不存在', 404)
      return new Response(characterPage(origin, slug, lang), { status: 200, headers: { ...baseHeaders, 'Content-Type': 'text/html; charset=utf-8', 'Cache-Control': 'public, max-age=60', 'Content-Security-Policy': "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; img-src 'self'; font-src 'self'; base-uri 'none'; frame-ancestors 'none'" } })
    }
    if (request.method === 'POST' && url.pathname === '/api/v1/loadouts') {
      return publish(request, env, origin)
    }
    if (request.method === 'POST' && url.pathname === '/api/v1/loadouts/import') {
      return importJSON(request, env, origin)
    }

    if (request.method === 'GET' && url.pathname === '/api/v1/loadouts') {
      if (!env.LOADOUTS?.list) return errorResponse('当前分享服务未启用目录索引', 503)
      const character = cleanText(url.searchParams.get('character'), 40).toLowerCase()
      const query = cleanText(url.searchParams.get('q') || url.searchParams.get('search'), 80).toLowerCase()
      const maximumLimit = query ? MAX_SEARCH_LIMIT : MAX_CATALOG_LIMIT
      const fallbackLimit = query ? MAX_CATALOG_LIMIT : DEFAULT_CATALOG_LIMIT
      const requestedLimit = Math.max(1, Math.min(maximumLimit, Number(url.searchParams.get('limit') || fallbackLimit)))
      const cursor = cleanText(url.searchParams.get('cursor'), 256)
      const items = []
      const filtered = Boolean(character || query)
      let nextCursor = cursor
      let truncated = false
      let scanned = 0
      do {
        const pageLimit = filtered ? Math.max(1, Math.min(requestedLimit - items.length, 128)) : requestedLimit
        const listed = await env.LOADOUTS.list({ prefix: 'meta/v1/', limit: pageLimit, ...(nextCursor ? { cursor: nextCursor } : {}) })
        scanned += (listed.objects || []).length
        truncated = Boolean(listed.truncated)
        nextCursor = truncated ? String(listed.cursor || '') : ''
        for (const { code, meta } of await readMetadataInBatches(env, listed.objects || [], lang)) {
          if (!hasPublicCatalogPreview(meta)) continue
          const identity = characterByIdentity(meta?.characterName, meta?.characterHash)
          const characterTerms = [meta?.characterName, identity.name, identity.nameEn, identity.slug].filter(Boolean).map(value => String(value).toLowerCase())
          if (!meta || (character && !characterTerms.some(value => value === character))) continue
          if (query && !previewSearchText(meta).includes(query)) continue
          items.push({
            code, title: meta.title || '', characterName: lang === 'en' ? identity.nameEn : identity.name,
            characterHash: meta.characterHash || identity.hash, characterSlug: identity.slug,
            avatarPath: `/assets/avatars/${identity.iconFile}`, accent: identity.accent, weaponName: meta.preview?.weaponName || '',
            preview: meta.preview || {}, createdAt: meta.createdAt || '',
          })
        }
        if (!filtered || !truncated || items.length >= requestedLimit || scanned >= 1000) break
      } while (nextCursor)
      return jsonResponse({ items, truncated, cursor: truncated ? nextCursor : '' }, 200, { 'Cache-Control': 'public, max-age=30' })
    }

    const apiMatch = url.pathname.match(/^\/api\/v1\/loadouts\/([^/]+)$/)
    if ((request.method === 'GET' || request.method === 'HEAD') && apiMatch) {
      const code = normalizeCode(apiMatch[1])
      if (!code) return errorResponse('短码格式无效', 400)
      const frame = await loadFrame(env, code)
      if (!frame) return errorResponse('没有找到这套配装', 404)
      if (request.method === 'HEAD') {
        return new Response(null, {
          status: 200,
          headers: { ...baseHeaders, 'Content-Type': 'application/vnd.gbfr.loadout' },
        })
      }
      return binaryResponse(frame)
    }

    const metaMatch = url.pathname.match(/^\/api\/v1\/loadouts\/([^/]+)\/meta$/)
    if (request.method === 'GET' && metaMatch) {
      const code = normalizeCode(metaMatch[1])
      if (!code) return errorResponse('短码格式无效', 400)
      const metadata = await readMetadata(env, code, lang)
      if (!metadata) return errorResponse('没有找到这套配装预览', 404)
      return jsonResponse(metadata, 200, { 'Cache-Control': 'no-store' })
    }

    const communityMatch = url.pathname.match(/^\/api\/v1\/loadouts\/([^/]+)\/community$/)
    if (communityMatch && (request.method === 'GET' || request.method === 'HEAD')) {
      const code = normalizeCode(communityMatch[1])
      if (!code) return errorResponse('短码格式无效', 400)
      return jsonResponse(await communitySummary(env, code), 200, { 'Cache-Control': 'no-store' })
    }
    const likeMatch = url.pathname.match(/^\/api\/v1\/loadouts\/([^/]+)\/like$/)
    if (likeMatch && request.method === 'POST') {
      const code = normalizeCode(likeMatch[1])
      if (!code) return errorResponse('短码格式无效', 400)
      return communityAction(request, env, code, 'like')
    }
    const commentMatch = url.pathname.match(/^\/api\/v1\/loadouts\/([^/]+)\/comments$/)
    if (commentMatch && request.method === 'POST') {
      const code = normalizeCode(commentMatch[1])
      if (!code) return errorResponse('短码格式无效', 400)
      return communityAction(request, env, code, 'comment')
    }

    const downloadMatch = url.pathname.match(/^\/download\/([^/]+)\.gbfr-loadout$/)
    if (request.method === 'GET' && downloadMatch) {
      const code = normalizeCode(downloadMatch[1])
      if (!code) return errorResponse('短码格式无效', 400)
      const frame = await loadFrame(env, code)
      if (!frame) return errorResponse('没有找到这套配装', 404)
      return binaryResponse(frame, `GBFR-${displayCode(code)}.gbfr-loadout`)
    }

    const shareMatch = url.pathname.match(/^\/s\/([^/]+)$/)
    if (request.method === 'GET' && shareMatch) {
      const code = normalizeCode(shareMatch[1])
      if (!code) return errorResponse('短码格式无效', 400)
      const object = await env.LOADOUTS.head(objectKey(code))
      if (!object) return errorResponse('没有找到这套配装', 404)
      return new Response(landingPage(origin, code, await readMetadata(env, code, lang), lang), {
        status: 200,
        headers: {
          ...baseHeaders,
          'Content-Type': 'text/html; charset=utf-8',
          'Cache-Control': 'no-store',
          'Content-Security-Policy': "default-src 'none'; style-src 'unsafe-inline'; script-src 'unsafe-inline'; connect-src 'self'; img-src 'self'; font-src 'self'; base-uri 'none'; frame-ancestors 'none'",
        },
      })
    }

    return errorResponse('Not found', 404)
  },
}
