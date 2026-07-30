import { RELINK_FORMULA_VERSION } from './relinkFormulaModel.js'

export const LOADOUT_SCENARIO_VERSION = '2.0.2-formula-2'
export const LOADOUT_CHARACTER_PROFILE_VERSION = '2.0.2-character-evidence-2'

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

const profile = (ownerCode, defaultDirection, directions, mechanics = 'directional-heuristic') => Object.freeze({
  ownerCode,
  defaultDirection,
  directions: Object.freeze(directions),
  provenance: '2.0.2-roster-plus-runtime-cap-curves',
  completeness: 'character-cap-and-direction',
  research: Object.freeze({
    identity: 'verified-roster',
    skills: 'save-and-unpacked-skill-catalog',
    capCurves: '2.0.2-runtime-reference',
    defense: 'audited-common-formula',
    mechanics,
    actionCoefficients: 'unverified-per-action',
    rotation: 'unverified',
  }),
})

export const LOADOUT_CHARACTER_PROFILES = Object.freeze({
  '2A26B1B2': profile('PL0000', 'normal', ['normal', 'ability', 'support']),
  A4ACBA76: profile('PL0100', 'normal', ['normal', 'ability', 'support']),
  '18E2F9F9': profile('PL0200', 'ability', ['ability', 'support', 'survival']),
  '079DF0CC': profile('PL0300', 'normal', ['normal', 'ability', 'stun']),
  '4D0A60C3': profile('PL0400', 'ability', ['ability', 'cooldown', 'stun'], 'io-stage-1-ability-direction'),
  DD7A151E: profile('PL0500', 'normal', ['normal', 'stun', 'ability']),
  C8616284: profile('PL0600', 'support', ['support', 'cooldown', 'ability']),
  C3FFD418: profile('PL0700', 'normal', ['normal', 'stun', 'support']),
  '22E437E5': profile('PL0800', 'ability', ['ability', 'cooldown', 'normal']),
  '2EBE91D5': profile('PL0900', 'ability', ['ability', 'support', 'survival']),
  BDEF7181: profile('PL1000', 'ability', ['ability', 'normal', 'stun']),
  '627BCB0D': profile('PL1100', 'normal', ['normal', 'support', 'stun']),
  FD3BE362: profile('PL1200', 'normal', ['normal', 'ability', 'cooldown']),
  FC6CDF7B: profile('PL1300', 'normal', ['normal', 'ability', 'cooldown']),
  E7053919: profile('PL1400', 'normal', ['normal', 'ability', 'sba']),
  '978E4B18': profile('PL1500', 'stun', ['stun', 'normal', 'survival']),
  '0D21B430': profile('PL1600', 'normal', ['normal', 'ability', 'stun']),
  F0EB77EF: profile('PL1700', 'normal', ['normal', 'survival', 'ability']),
  AA66178A: profile('PL1800', 'support', ['support', 'cooldown', 'ability']),
  A3A3CB2F: profile('PL1900', 'normal', ['normal', 'ability', 'sba']),
  '718E1A14': profile('PL2100', 'normal', ['normal', 'ability', 'sba']),
  '296471BE': profile('PL2200', 'normal', ['normal', 'ability', 'sba']),
  BAD16E3B: profile('PL2300', 'normal', ['normal', 'stun', 'ability']),
  '1BB37EF0': profile('PL2400', 'normal', ['normal', 'survival', 'stun']),
  '25D46F4B': profile('PL2500', 'ability', ['ability', 'support', 'cooldown']),
  '9A8AF295': profile('PL2600', 'normal', ['normal', 'survival', 'ability']),
  '9B15CFB1': profile('PL2700', 'normal', ['normal', 'ability', 'cooldown']),
  '646C3168': profile('PL2800', 'ability', ['ability', 'cooldown', 'support']),
  '74DD4C79': profile('PL2900', 'ability', ['ability', 'support', 'survival']),
})

export function characterLoadoutProfile(charaHash) {
  return LOADOUT_CHARACTER_PROFILES[String(charaHash || '').replace(/^0x/i, '').toUpperCase()]
    || profile('', 'normal', Object.keys(LOADOUT_DIRECTIONS), 'unidentified-character')
}

export function scenarioLabel(key, locale = 'zh') {
  const item = LOADOUT_DIRECTIONS[key]
  return item ? (locale === 'en' ? item.en : item.zh) : key
}
