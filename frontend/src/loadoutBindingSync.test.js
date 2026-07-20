import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const appJS = readFileSync(new URL('../wailsjs/go/main/App.js', import.meta.url), 'utf8')
const appTypes = readFileSync(new URL('../wailsjs/go/main/App.d.ts', import.meta.url), 'utf8')
const models = readFileSync(new URL('../wailsjs/go/models.ts', import.meta.url), 'utf8')

const loadoutMethods = [
  'LoadoutApply',
  'LoadoutApplyWithResources',
  'LoadoutCheckCompliance',
  'LoadoutConstructSigil',
  'LoadoutEditContext',
  'LoadoutExport',
  'LoadoutImport',
  'LoadoutList',
  'LoadoutRuntimePanelStats',
  'LoadoutSimulate',
  'LoadoutSimulateBuild',
  'LoadoutSimulateDraft',
  'LoadoutStatContext',
  'MasteryNodePool',
  'MasterySummarize',
  'LoadBadgeState',
  'SetBadgeState',
  'SetAllBadgeStates',
]

test('generated Wails bindings expose every loadout and mastery backend entry point', () => {
  for (const method of loadoutMethods) {
    assert.match(appJS, new RegExp(`export function ${method}\\(`), `${method} is absent from App.js`)
    assert.match(appTypes, new RegExp(`export function ${method}\\(`), `${method} is absent from App.d.ts`)
  }
})

test('generated models keep the complete build, share-write and real-stat fields', () => {
  for (const field of [
    'primaryTraitName',
    'primaryTraitLevel',
    'secondaryTraitName',
    'secondaryTraitLevel',
    'constructedSigils',
    'summonSlotIds',
    'equippedSummonSlotIds',
    'equippedSummons',
    'overLimit',
    'finalStats',
    'weaponSkills',
    'normalDamageCap',
    'abilityDamageCap',
    'skyboundDamageCap',
    'permanentGrowth',
    'baselineHp',
    'baselineAtk',
    'baselineStun',
    'baselineCritRate',
    'baselineDamageCap',
    'fateEpisodeMask',
    'fateEpisodeCount',
    'fateHp',
    'fateAtk',
    'masterTotalMsp',
    'masterLevel',
    'masterHp',
    'masterAtk',
    'masterDamageCap',
    'masteryRankCaps',
    'runtimeVerified',
    'verification',
    'expectSkillHashes',
    'skillHashes',
    'skillSlots',
    'groupHash',
    'currentHash',
    'editable',
    'options',
    'weaponEdits',
    'summonEdits',
  ]) {
    assert.match(models, new RegExp(`\\b${field}\\??:`), `${field} is absent from generated models.ts`)
  }
})
