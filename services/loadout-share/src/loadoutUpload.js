import { encode } from '@msgpack/msgpack'
import { brotliCompressSync, constants } from 'node:zlib'
import overLimitCurves from '../../../internal/backend/data/overlimit_curves.json' with { type: 'json' }

const MAX_JSON_BYTES = 1024 * 1024
const MAX_FRAME_BYTES = 8 * 1024
const HASH_PATTERN = /^(?:0x)?[0-9a-f]{8}$/i
const SOURCE_KINDS = new Set(['save', 'runtime', 'logs-db'])
const CAPTURED_FIELDS = new Set(['stats', 'sigils', 'summons', 'skills', 'weapon', 'weaponSkills', 'wrightstone', 'mastery', 'character', 'overLimit'])
const OVER_LIMIT = Object.fromEntries(overLimitCurves.entries.map(entry => [entry.hash, [entry.name, entry.nameEn, entry.unit, entry.rawValues.map(value => value * entry.displayScale)]]))
const OVER_LIMIT_ALIASES = Object.fromEntries(overLimitCurves.entries.flatMap(entry => (entry.aliases || []).map(alias => [alias, entry.hash])))

function integer(value, field, min = 0, max = 0x7fffffff) {
  const number = Number(value)
  if (!Number.isInteger(number) || number < min || number > max) throw new Error(`${field}无效`)
  return number
}

function text(value, field, max = 120, optional = false) {
  const result = String(value || '').replace(/[\u0000-\u001f\u007f]/g, ' ').trim()
  if (!result && !optional) throw new Error(`${field}不能为空`)
  if (result.length > max) throw new Error(`${field}过长`)
  return result
}

function hash(value, field, optional = false) {
  const source = String(value || '').trim()
  if (!source && optional) return 0
  if (!HASH_PATTERN.test(source)) throw new Error(`${field}无效`)
  return Number.parseInt(source.replace(/^0x/i, ''), 16) >>> 0
}

function list(value, field, max, exact = -1) {
  if (!Array.isArray(value)) throw new Error(`${field}必须是数组`)
  if (exact >= 0 && value.length !== exact) throw new Error(`${field}需要恰好 ${exact} 项`)
  if (value.length > max) throw new Error(`${field}超过 ${max} 项上限`)
  return value
}

function wrightstone(value, field) {
  if (value == null) return null
  const traits = list(value.traits || [], `${field}词条`, 3)
  return [
    hash(value.hash, `${field}哈希`),
    traits.map((trait, index) => [
      integer(trait.index ?? index, `${field}词条索引`, 0, 2),
      hash(trait.hash, `${field}词条哈希`),
      integer(trait.level, `${field}词条等级`, 0, 999),
    ]),
  ]
}

function compactCharacter(value) {
  if (!value || typeof value !== 'object') throw new Error('配装缺少角色强化快照')
  const nodeValues = value.enhancementNodeValues
  const nodes = Array.isArray(nodeValues)
    ? list(nodeValues, '角色强化节点', 1000).map((nodeValue, index) => ({ index, value: nodeValue }))
    : list(value.enhancementNodes, '角色强化节点', 1000)
  if (!nodes.length) throw new Error('角色强化节点不能为空')
  return [
    integer(value.characterLevel, '角色等级', 0, 1000),
    integer(value.baseHp ?? value.baseHP, '角色基础HP', -0x80000000, 0x7fffffff),
    integer(value.baseAtk ?? value.baseATK, '角色基础攻击力', -0x80000000, 0x7fffffff),
    integer(value.baseStunBits, '角色基础昏厥位模式', 0, 0xffffffff),
    integer(value.baseCritRate, '角色基础暴击率', -0x80000000, 0x7fffffff),
    Boolean(value.characterBaseCaptured),
    integer(value.masterTotalMsp ?? value.masterTotalMSP, '专精MSP', -0x80000000, 0x7fffffff),
    integer(value.legacyProgress, '角色强化进度', -0x80000000, 0x7fffffff),
    list(value.enhancementPanel, '角色强化面板', 2, 2).map((item, index) => integer(item, `角色强化面板${index + 1}`, -0x80000000, 0x7fffffff)),
    nodes.map(node => [integer(node.index, '角色强化节点索引', 0, 9999), integer(node.value, '角色强化节点值', -0x80000000, 0x7fffffff)]),
    list(value.weapons || [], '角色武器收藏', 128).map(weapon => [
      hash(weapon.hash, '角色武器哈希'),
      hash(weapon.baseHash, '角色武器基础哈希', true),
      text(weapon.internalId, '角色武器内部ID', 48, true),
      integer(weapon.level, '角色武器等级', 0, 1000),
      integer(weapon.uncap, '角色武器突破', 0, 99),
      integer(weapon.mirage, '角色武器幻晶', 0, 999),
      integer(weapon.awakening, '角色武器觉醒', 0, 99),
      integer(weapon.transcendence, '角色武器超凡', 0, 99),
      text(weapon.transcendenceSkill, '角色武器超凡技能', 16, true),
      wrightstone(weapon.wrightstone, '角色武器祝福'),
    ]),
    Boolean(value.weaponWrightstonesCaptured),
  ]
}

function compactWeapon(value, weaponSkillsCaptured) {
  if (value == null) return null
  const skillHashes = list(value.skillHashes || [], '当前武器技能', 5, weaponSkillsCaptured ? 5 : 0)
  return [
    hash(value.storedHash, '当前武器存储哈希'),
    integer(value.xp, '当前武器经验', 0, 0xffffffff),
    integer(value.uncap, '当前武器突破', 0, 99),
    integer(value.mirage, '当前武器幻晶', 0, 999),
    integer(value.awakening, '当前武器觉醒', 0, 99),
    integer(value.transcendence, '当前武器超凡', 0, 99),
    Boolean(value.exactState),
    integer(value.flags ?? 0, '当前武器标志', 0, 0xffffffff),
    hash(value.wrightstoneReference, '当前武器祝福引用', true),
    integer(value.state ?? 0, '当前武器状态', -0x80000000, 0x7fffffff),
    skillHashes.map(item => hash(item, '当前武器技能哈希')),
    wrightstone(value.wrightstone, '当前武器祝福'),
  ]
}

function compactLoadout(source) {
  if (!source || typeof source !== 'object' || Array.isArray(source)) throw new Error('文件内容不是配装对象')
  if (source.format !== 'gbfr-loadout') throw new Error('不是 GBFR 单套配装文件')
  if (![10, 11].includes(source.version)) throw new Error(`网页上传需要 v10 或 v11 配装文件，当前为 v${source.version || 0}；请用最新版应用重新导出`)
  const sourceKind = source.version >= 11 ? text(source.sourceKind || 'save', '配装来源', 24) : 'save'
  const capturedFields = source.version >= 11
    ? list(source.capturedFields || [], '捕获字段', 24).map(item => text(item, '捕获字段', 32))
    : []
  const partial = source.version >= 11 && sourceKind !== 'save'
  if (!SOURCE_KINDS.has(sourceKind)) throw new Error(`配装来源 ${sourceKind} 无效`)
  if (new Set(capturedFields).size !== capturedFields.length || capturedFields.some(field => !CAPTURED_FIELDS.has(field))) throw new Error('捕获字段包含未知或重复项目')
  const progressionPolicy = text(source.progressionPolicy || 'exact', '养成策略', 24)
  if ((sourceKind === 'save' && progressionPolicy !== 'exact') || (partial && progressionPolicy !== 'endgame-max')) throw new Error('配装来源与养成策略不匹配')
  if (source.version >= 11 && capturedFields.length === 0) throw new Error('v11 配装缺少捕获字段声明')
  const captured = field => !partial || capturedFields.includes(field)
  const sigils = captured('sigils') ? list(source.sigils || [], '因子配置', 12) : []
  const summons = captured('summons') ? list(source.summons || [], '召唤石配置', 4, 4) : []
  const skills = captured('skills') ? list(source.skills || [], '角色技能', 4) : []
  const weaponSkills = captured('weaponSkills') ? list(source.weaponSkillHashes || [], '武器技能快照', 5, 5) : []
  const mastery = captured('mastery') ? list(source.masteryHashes || [], '专精快照', 50, 50) : []
  const overLimit = captured('overLimit') ? list(source.overLimit || [], '上限突破配置', 4, 4) : []
  if (partial && captured('sigils') !== (sigils.length > 0)) throw new Error('部分配装的因子内容与捕获字段声明不一致')
  const rejectedPayload = [
    !captured('summons') && (source.summons || []).length,
    !captured('skills') && (source.skills || []).length,
    !captured('weaponSkills') && (source.weaponSkillHashes || []).length,
    !captured('mastery') && (source.masteryHashes || []).length,
    !captured('character') && source.character,
    !captured('overLimit') && (source.overLimit || []).length,
    !captured('weapon') && (source.weapon || source.weaponHash || source.weaponName),
    !captured('wrightstone') && (source.weapon?.wrightstone || source.weapon?.wrightstoneReference),
    !captured('weaponSkills') && (source.weapon?.skillHashes || []).length,
  ]
  if (partial && rejectedPayload.some(Boolean)) throw new Error('部分配装包含未声明为已捕获的字段')
  if (partial && captured('weapon') && !source.weapon) throw new Error('部分配装声明已捕获武器但缺少武器状态')
  if (partial && (captured('weaponSkills') || captured('wrightstone')) && !captured('weapon')) throw new Error('部分配装的武器技能或祝福范围缺少武器主体')
  if (partial && captured('wrightstone') && !source.weapon?.wrightstone) throw new Error('部分配装声明已捕获武器祝福但缺少祝福状态')
  const deployable = sigils.length || summons.length || skills.length || weaponSkills.length || mastery.length || overLimit.length || (captured('weapon') && source.weapon)
  if (partial && !deployable) throw new Error('部分配装没有可部署的配装范围')
  const compact = [
    source.version,
    hash(source.charaHash, '角色哈希'),
    text(source.charaName, '角色名称', 48),
    text(source.ownerCode, '角色所有者代码', 24),
    text(source.name, '配装名称', 80),
    hash(source.weaponHash, '武器哈希', true),
    text(source.weaponName, '武器名称', 80, true),
    sigils.map((sigil, index) => [
      integer(sigil.index ?? index, '因子槽位', 0, 11),
      hash(sigil.hash, '因子哈希'),
      text(sigil.name, '因子名称', 80),
      integer(sigil.level, '因子等级', 0, 999),
      hash(sigil.primaryTraitHash, '因子主词条哈希'),
      integer(sigil.primaryTraitLevel, '因子主词条等级', 0, 999),
      hash(sigil.secondaryTraitHash, '因子副词条哈希', true),
      integer(sigil.secondaryTraitLevel || 0, '因子副词条等级', 0, 999),
    ]),
    summons.map(summon => [
      hash(summon.typeHash, '召唤石类型哈希'),
      text(summon.name, '召唤石名称', 80),
      hash(summon.mainTraitHash, '召唤石主加护哈希'),
      integer(summon.mainTraitLevel, '召唤石主加护等级', 0, 999),
      hash(summon.subParamHash, '召唤石副词条哈希'),
      integer(summon.subParamLevel, '召唤石副词条等级', 0, 999),
      integer(summon.rank, '召唤石阶级', 0, 99),
    ]),
    skills.map(skill => [hash(skill.hash, '角色技能哈希'), text(skill.name, '角色技能名称', 80), text(skill.key, '角色技能键', 48)]),
    weaponSkills.map(item => hash(item, '武器技能哈希')),
    mastery.map(item => hash(item, '专精节点哈希')),
    captured('character') ? compactCharacter(source.character) : null,
    captured('weapon') ? compactWeapon(source.weapon, captured('weaponSkills')) : null,
    overLimit.map(slot => [
      integer(slot.index, '上限突破槽位', 0, 3),
      hash(slot.attributeHash, '上限突破属性哈希', true),
      integer(slot.level, '上限突破等级', 0, 999),
    ]),
  ]
  if (source.version >= 11) {
    compact.push(
      sourceKind,
      capturedFields,
      progressionPolicy,
    )
  }
  return compact
}

function crc32(bytes) {
  let crc = 0xffffffff
  for (const byte of bytes) {
    crc ^= byte
    for (let bit = 0; bit < 8; bit += 1) crc = (crc >>> 1) ^ (0xedb88320 & -(crc & 1))
  }
  return (crc ^ 0xffffffff) >>> 0
}

export function previewFromLoadout(source) {
  return {
    title: text(source.name, '配装名称', 80),
    characterHash: text(source.charaHash, '角色哈希', 16),
    characterName: text(source.charaName, '角色名称', 48),
    weaponHash: text(source.weaponHash, '武器哈希', 16, true),
    weaponName: text(source.weaponName, '武器名称', 80, true),
    sigils: (source.sigils || []).map(sigil => ({
      hash: sigil.hash, name: sigil.name, level: sigil.level,
      primaryHash: sigil.primaryTraitHash, primaryLevel: sigil.primaryTraitLevel,
      secondaryHash: sigil.secondaryTraitHash, secondaryLevel: sigil.secondaryTraitLevel,
    })),
    abilities: (source.skills || []).map(skill => ({ hash: skill.hash, key: skill.key, name: skill.name })),
    weaponSkills: (source.weaponSkillHashes || []).map(item => ({ hash: item })),
    summons: (source.summons || []).map(summon => ({
      typeHash: summon.typeHash, name: summon.name, rank: summon.rank,
      mainTraitHash: summon.mainTraitHash, mainTraitLevel: summon.mainTraitLevel,
      subParamHash: summon.subParamHash, subParamLevel: summon.subParamLevel,
    })),
    masteryCount: (source.masteryHashes || []).filter(item => !/^0+$/.test(String(item || ''))).length,
    masterySkills: (source.masteryHashes || []).filter(item => !/^0+$/.test(String(item || ''))).map(item => ({ hash: item })),
    overLimit: (source.overLimit || []).map(slot => {
      const attributeHash = String(slot.attributeHash || '').toUpperCase()
      const definition = OVER_LIMIT[OVER_LIMIT_ALIASES[attributeHash] || attributeHash]
      const level = Number(slot.level) || 0
      return {
        index: Number(slot.index) || 0, attributeHash, level,
        name: definition?.[0] || '', nameEn: definition?.[1] || '', unit: definition?.[2] || '',
        value: definition && level >= 1 && level <= 10 ? definition[3][level - 1] : 0,
      }
    }),
  }
}

async function compressBrotli(bytes) {
  return new Uint8Array(brotliCompressSync(bytes, {
    params: { [constants.BROTLI_PARAM_QUALITY]: 6 },
  }))
}

export async function loadoutJSONToFrame(input, compressor = compressBrotli) {
  const bytes = input instanceof Uint8Array ? input : new TextEncoder().encode(String(input || ''))
  if (!bytes.length || bytes.length > MAX_JSON_BYTES) throw new Error('配装文件大小必须在 1 B 到 1 MiB 之间')
  let source
  try { source = JSON.parse(new TextDecoder('utf-8', { fatal: true }).decode(bytes)) } catch { throw new Error('配装 JSON 无法解析或不是 UTF-8') }
  const packed = encode(compactLoadout(source), { ignoreUndefined: true })
  const compressed = await compressor(packed)
  const frame = new Uint8Array(18 + compressed.length)
  frame.set(new TextEncoder().encode('GBLC'), 0)
  frame[4] = 2
  frame[5] = 1
  const view = new DataView(frame.buffer)
  view.setUint32(6, packed.length, true)
  view.setUint32(10, crc32(packed), true)
  view.setUint32(14, compressed.length, true)
  frame.set(compressed, 18)
  if (frame.length > MAX_FRAME_BYTES) throw new Error('压缩后的配装超过 8 KiB，无法在线分享')
  return { frame, source, preview: previewFromLoadout(source) }
}
