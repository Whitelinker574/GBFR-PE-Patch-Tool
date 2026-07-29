import { RELINK_FORMULA_VERSION } from './relinkFormulaModel.js'

export const LOADOUT_SCENARIO_VERSION = '2.0.2-formula-2'
export const LOADOUT_CHARACTER_PROFILE_VERSION = '2.0.2-character-directions-1'

export const LOADOUT_ACTION_TYPES = Object.freeze({
  normal: { zh: '普攻', en: 'Normal Attack', capField: 'normalCurve' },
  ability: { zh: '能力', en: 'Ability', capField: 'artsCurve' },
  sba: { zh: '奥义', en: 'Skybound Art', capField: 'artsCurve' },
  chain: { zh: '连锁', en: 'Chain Burst', capField: 'chainBurstDamageLimit' },
})

export const DEFAULT_LOADOUT_COMBAT_SCENARIO = Object.freeze({
  formulaVersion: RELINK_FORMULA_VERSION,
  actionType: 'normal',
  odStage: 0,
  berserk: false,
  disableAttackDefenseInOD: true,
  coverage: 1,
  minimumHp: 0,
  minimumDefense: 0,
})

export const LOADOUT_DIRECTIONS = Object.freeze({
  normal: { zh: '普攻', en: 'Normal Attacks', traitIds: ['SKILL_020_00', 'SKILL_000_00', 'SKILL_146_00', 'SKILL_151_00'], actionType: 'normal', actionRate: 1, coverage: 1, evidence: '2.0.2-common-formula' },
  ability: { zh: '能力', en: 'Abilities', traitIds: ['SKILL_020_00', 'SKILL_003_00', 'SKILL_151_00', 'SKILL_069_00'], actionType: 'ability', actionRate: 1, coverage: 0.9, evidence: '2.0.2-common-formula' },
  sba: { zh: '奥义', en: 'Skybound Arts', traitIds: ['SKILL_020_00', 'SKILL_151_00', 'SKILL_146_00'], actionType: 'sba', actionRate: 1, coverage: 1, evidence: '2.0.2-common-formula' },
  chain: { zh: '连锁', en: 'Chain Bursts', traitIds: ['SKILL_020_00', 'SKILL_003_00', 'SKILL_146_00'], actionType: 'chain', actionRate: 1, coverage: 1, evidence: '2.0.2-common-formula' },
  stun: { zh: '眩晕', en: 'Stun', traitIds: ['SKILL_004_00', 'SKILL_009_00', 'SKILL_020_00'], actionType: 'normal', actionRate: 1, coverage: 1, evidence: '2.0.2-common-formula-plus-trait-score' },
  cooldown: { zh: '冷却', en: 'Cooldown', traitIds: ['SKILL_069_00', 'SKILL_070_00', 'SKILL_020_00'], actionType: 'ability', actionRate: 1, coverage: 0.85, evidence: '2.0.2-common-formula-plus-trait-score' },
  support: { zh: '团队辅助', en: 'Team Support', traitIds: ['SKILL_001_00', 'SKILL_085_00', 'SKILL_069_00'], actionType: 'normal', actionRate: 1, coverage: 0.8, evidence: '2.0.2-common-formula-plus-survival-score' },
  survival: { zh: '生存', en: 'Survival', traitIds: ['SKILL_001_00', 'SKILL_085_00', 'SKILL_036_00', 'SKILL_068_00'], actionType: 'normal', actionRate: 1, coverage: 1, evidence: '2.0.2-defense-formula' },
})

const profile = (defaultDirection, directions, provenance = 'heuristic', completeness = 'role-level') => Object.freeze({
  defaultDirection,
  directions: Object.freeze(directions),
  provenance,
  completeness,
})

export const LOADOUT_CHARACTER_PROFILES = Object.freeze({
  '2A26B1B2': profile('normal', ['normal', 'ability', 'support']),
  A4ACBA76: profile('normal', ['normal', 'ability', 'support']),
  '18E2F9F9': profile('ability', ['ability', 'support', 'survival']),
  '079DF0CC': profile('normal', ['normal', 'ability', 'stun']),
  '4D0A60C3': profile('ability', ['ability', 'cooldown', 'stun']),
  DD7A151E: profile('normal', ['normal', 'stun', 'ability']),
  C8616284: profile('support', ['support', 'cooldown', 'ability']),
  C3FFD418: profile('normal', ['normal', 'stun', 'support']),
  '22E437E5': profile('ability', ['ability', 'cooldown', 'normal']),
  '2EBE91D5': profile('ability', ['ability', 'support', 'survival']),
  BDEF7181: profile('ability', ['ability', 'normal', 'stun']),
  '627BCB0D': profile('normal', ['normal', 'support', 'stun']),
  FD3BE362: profile('normal', ['normal', 'ability', 'cooldown']),
  FC6CDF7B: profile('normal', ['normal', 'ability', 'cooldown']),
  E7053919: profile('normal', ['normal', 'ability', 'sba']),
  '978E4B18': profile('stun', ['stun', 'normal', 'survival']),
  '0D21B430': profile('normal', ['normal', 'ability', 'stun']),
  F0EB77EF: profile('normal', ['normal', 'survival', 'ability']),
  AA66178A: profile('support', ['support', 'cooldown', 'ability']),
  A3A3CB2F: profile('normal', ['normal', 'ability', 'sba']),
  '718E1A14': profile('normal', ['normal', 'ability', 'sba']),
  '296471BE': profile('normal', ['normal', 'ability', 'sba']),
  BAD16E3B: profile('normal', ['normal', 'stun', 'ability']),
  '1BB37EF0': profile('normal', ['normal', 'survival', 'stun']),
  '25D46F4B': profile('ability', ['ability', 'support', 'cooldown']),
  '9A8AF295': profile('normal', ['normal', 'survival', 'ability']),
  '9B15CFB1': profile('normal', ['normal', 'ability', 'cooldown']),
  '646C3168': profile('ability', ['ability', 'cooldown', 'support']),
  '74DD4C79': profile('ability', ['ability', 'support', 'survival']),
})

export function characterLoadoutProfile(charaHash) {
  return LOADOUT_CHARACTER_PROFILES[String(charaHash || '').replace(/^0x/i, '').toUpperCase()] || profile('normal', Object.keys(LOADOUT_DIRECTIONS))
}

export function scenarioLabel(key, locale = 'zh') {
  const item = LOADOUT_DIRECTIONS[key]
  return item ? (locale === 'en' ? item.en : item.zh) : key
}
