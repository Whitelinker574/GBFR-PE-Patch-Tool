import test from 'node:test'
import assert from 'node:assert/strict'
import { groupMasteryNodes, resolveMasteryHashes } from './loadoutMastery.js'

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
