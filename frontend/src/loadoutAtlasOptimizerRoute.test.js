import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

import {
  createAtlasOptimizerIntent,
  planAtlasOptimizerRoute,
} from './loadoutAtlasOptimizerRoute.js'

const editor = readFileSync(new URL('./components/LoadoutEditor.vue', import.meta.url), 'utf8')
const optimizer = readFileSync(new URL('./components/LoadoutOptimizer.vue', import.meta.url), 'utf8')
const atlas = readFileSync(new URL('./components/SigilAtlas.vue', import.meta.url), 'utf8')
const viewer = readFileSync(new URL('./components/LoadoutViewer.vue', import.meta.url), 'utf8')

const ioGroup = {
  charaHash: '4D0A60C3',
  charaName: '伊欧',
  loadouts: [{ unitId: 42, isParty: false }],
}
const rackamGroup = {
  charaHash: '079DF0CC',
  charaName: '拉卡姆',
  loadouts: [{ unitId: 7, isParty: false }],
}

test('atlas optimizer intent preserves owner, exact trait levels, and stable order', () => {
  const intent = createAtlasOptimizerIntent({
    allowedOwnerCodes: ['pl0400', 'PL0400'],
    targets: [
      { traitId: 'SKILL_IO', traitName: '怒发冲冠', targetLevel: 15 },
      { traitId: 'SKILL_CAP', traitName: '伤害上限', targetLevel: 30 },
    ],
    traitIds: ['SKILL_IO', 'SKILL_CAP', 'SKILL_IO'],
    traitNames: ['怒发冲冠', '伤害上限', '怒发冲冠'],
  }, 123)

  assert.deepEqual(intent.allowedOwnerCodes, ['PL0400'])
  assert.deepEqual(intent.traitIds, ['SKILL_IO', 'SKILL_CAP'])
  assert.deepEqual(intent.traitNames, ['怒发冲冠', '伤害上限'])
  assert.deepEqual(intent.targetLevels, { SKILL_IO: 15, SKILL_CAP: 30 })
  assert.equal(intent.requestId, 123)
})

test('character-specific atlas target routes away from the currently selected foreign character', () => {
  const route = planAtlasOptimizerRoute({
    savePath: 'SaveData1.dat',
    groups: [rackamGroup, ioGroup],
    currentGroup: rackamGroup,
    payload: {
      allowedOwnerCodes: ['PL0400'],
      traitIds: ['SKILL_IO'],
      targetLevels: { SKILL_IO: 15 },
    },
    requestId: 456,
  })

  assert.equal(route.kind, 'ready')
  assert.equal(route.targetGroup, ioGroup)
  assert.equal(route.loadout.unitId, 42)
  assert.equal(route.intent.requestId, 456)
})

test('generic targets stay on the selected character and shared owner targets respect it', () => {
  const generic = planAtlasOptimizerRoute({
    savePath: 'SaveData1.dat',
    groups: [rackamGroup, ioGroup],
    currentGroup: rackamGroup,
    payload: { traitIds: ['SKILL_CAP'] },
    requestId: 1,
  })
  assert.equal(generic.kind, 'ready')
  assert.equal(generic.targetGroup, rackamGroup)

  const shared = planAtlasOptimizerRoute({
    savePath: 'SaveData1.dat',
    groups: [rackamGroup, ioGroup],
    currentGroup: ioGroup,
    payload: { allowedOwnerCodes: ['PL0300', 'PL0400'], traitIds: ['SHARED'] },
    requestId: 2,
  })
  assert.equal(shared.kind, 'ready')
  assert.equal(shared.targetGroup, ioGroup)
})

test('route decision exposes actionable states for no save, missing character, and no editable slot', () => {
  assert.equal(planAtlasOptimizerRoute({
    savePath: '',
    groups: [],
    currentGroup: null,
    payload: { traitIds: ['A'] },
  }).kind, 'needs-save')

  assert.equal(planAtlasOptimizerRoute({
    savePath: 'SaveData1.dat',
    groups: [rackamGroup],
    currentGroup: rackamGroup,
    payload: { allowedOwnerCodes: ['PL0400'], traitIds: ['SKILL_IO'] },
  }).kind, 'missing-owner')

  assert.equal(planAtlasOptimizerRoute({
    savePath: 'SaveData1.dat',
    groups: [{ ...ioGroup, loadouts: [{ unitId: 9, isParty: true }] }],
    currentGroup: null,
    payload: { allowedOwnerCodes: ['PL0400'], traitIds: ['SKILL_IO'] },
  }).kind, 'no-editable-slot')
})

test('editor opens the embedded optimizer and optimizer consumes exact pending levels', () => {
  assert.match(editor, /watch\(\(\) => props\.pendingOptimizerTarget\?\.requestId,[\s\S]*?factorWorkspaceMode\.value = 'smart'/)
  assert.match(optimizer, /target\?\.targetLevels/)
  assert.match(optimizer, /profile\.value = 'custom'/)
})

test('atlas-to-editor wiring carries the owner and levels through the route decision', () => {
  assert.match(atlas, /allowedOwnerCodes: \[\.\.\.\(entry\.allowedOwnerCodes \|\| \[\]\)\]/)
  assert.match(atlas, /targets: traits\.map/)
  assert.match(atlas, /targetLevels: Object\.fromEntries/)
  assert.match(viewer, /@optimize="openAtlasOptimizer"/)
  assert.match(viewer, /planAtlasOptimizerRoute\(\{/)
  assert.match(viewer, /selectedChara\.value = route\.targetGroup\.charaName[\s\S]*?await nextTick\(\)[\s\S]*?pendingOptimizerTarget\.value = route\.intent/)
  assert.match(viewer, /kind === 'needs-save'[\s\S]*?请先在角色配装页选择目标存档和角色/)
  assert.match(viewer, /kind === 'missing-owner'[\s\S]*?当前存档没有对应角色配装/)
})

test('atlas handoff actions stay light and flat without a dark primary button', () => {
  assert.match(atlas, /class="ui-btn atlas-flat-action" @click="sendToOptimizer/)
  assert.match(atlas, /class="ui-btn atlas-constructor-action atlas-flat-action" :disabled="!entry\.constructible"/)
  assert.doesNotMatch(atlas, /class="ui-btn is-primary"[^>]*sendToConstructor/)
  assert.match(atlas, /\.atlas-flat-action \{[\s\S]*?box-shadow:none;/)
  assert.match(atlas, /\.atlas-constructor-action \{[\s\S]*?background:var\(--accent-soft\);/)
})
