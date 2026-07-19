import test from 'node:test'
import assert from 'node:assert/strict'
import {
  applyMasteryDirection,
  groupMasteryNodes,
  inferMasteryDirection,
  isMasteryNodeSelectable,
  resolveMasteryHashes,
} from './loadoutMastery.js'

test('EX阶节点仍按真谛、觉醒、秘义三种专精类型分组', () => {
  const groups = groupMasteryNodes([
    { hash: 'A', cat: 'SB_LIMIT', desc: '界限效果' },
    { hash: 'B', cat: 'SB_ATK', desc: '攻击效果' },
    { hash: 'C', cat: 'SB_DEF', desc: '防御效果' },
  ])
  assert.deepEqual(groups.map(group => group.cat), ['SB_ATK', 'SB_DEF', 'SB_LIMIT'])
  assert.deepEqual(groups.map(group => group.label), ['真谛', '觉醒', '秘义'])
})

test('自由配置与复制现有都能生成右栏汇总所需的真实节点 hash', () => {
  assert.deepEqual(resolveMasteryHashes({
    mode: 'free',
    picks: { R1: ['01'], R2: ['02'], R3: [], EX: ['04'] },
    sourceId: 0,
    sources: [],
  }), ['01', '02', '04'])

  assert.deepEqual(resolveMasteryHashes({
    mode: 'copy',
    picks: {},
    sourceId: 3007,
    sources: [{ unitId: 3007, nodeHashes: ['AA', 'BB'] }],
  }), ['AA', 'BB'])
})

test('stage one allows all named specializations while stage two and three lock unpacked specialization roots to the primary direction', () => {
  const attackNamed = { hash: 'A', cat: 'SB_ATK', name: '攻击专精', specialization: true }
  const defenceNamed = { hash: 'D', cat: 'SB_DEF', name: '防御专精', specialization: true }
  const defenceSubstat = { hash: 'DS', cat: 'SB_DEF', name: '', specialization: false }

  assert.equal(isMasteryNodeSelectable('R1', attackNamed, ''), true)
  assert.equal(isMasteryNodeSelectable('R1', defenceNamed, 'SB_ATK'), true)
  assert.equal(isMasteryNodeSelectable('R2', defenceSubstat, 'SB_ATK'), true)
  assert.equal(isMasteryNodeSelectable('R3', defenceSubstat, 'SB_ATK'), true)
  assert.equal(isMasteryNodeSelectable('R2', attackNamed, ''), false)
  assert.equal(isMasteryNodeSelectable('R2', attackNamed, 'SB_ATK'), true)
  assert.equal(isMasteryNodeSelectable('R3', defenceNamed, 'SB_ATK'), false)
})

test('changing primary direction removes only off-direction named stage two and three skills', () => {
  const nodes = new Map([
    ['R1-A', { hash: 'R1-A', rank: 'R1', cat: 'SB_ATK', name: '一阶攻击专精', specialization: true }],
    ['R2-A', { hash: 'R2-A', rank: 'R2', cat: 'SB_ATK', name: '', specialization: true }],
    ['R2-D', { hash: 'R2-D', rank: 'R2', cat: 'SB_DEF', name: '', specialization: true }],
    ['R2-DS', { hash: 'R2-DS', rank: 'R2', cat: 'SB_DEF', name: '', specialization: false }],
    ['R3-A', { hash: 'R3-A', rank: 'R3', cat: 'SB_ATK', name: '', specialization: true }],
    ['R3-LS', { hash: 'R3-LS', rank: 'R3', cat: 'SB_LIMIT', name: '', specialization: false }],
    ['EX-A', { hash: 'EX-A', rank: 'EX', cat: 'SB_ATK', name: 'EX技能' }],
  ])
  const picks = {
    R1: ['R1-A'],
    R2: ['R2-A', 'R2-D', 'R2-DS'],
    R3: ['R3-A', 'R3-LS'],
    EX: ['EX-A'],
  }

  assert.deepEqual(applyMasteryDirection(picks, 'SB_DEF', nodes), {
    R1: ['R1-A'],
    R2: ['R2-D', 'R2-DS'],
    R3: ['R3-LS'],
    EX: ['EX-A'],
  })
})

test('primary mastery direction is inferred from the six-node stage-two threshold before named-skill fallback', () => {
  const nodes = new Map()
  const r2 = []
  for (let index = 0; index < 6; index += 1) {
    const hash = `D${index}`
    r2.push(hash)
    nodes.set(hash, { hash, rank: 'R2', cat: 'SB_DEF', name: '' })
  }
  nodes.set('A-NAMED', { hash: 'A-NAMED', rank: 'R2', cat: 'SB_ATK', name: '', specialization: true })
  assert.equal(inferMasteryDirection({ R1: [], R2: [...r2, 'A-NAMED'], R3: [], EX: [] }, nodes), 'SB_DEF')
})
