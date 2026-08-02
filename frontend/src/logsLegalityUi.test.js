import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./components/LogsBattleArchive.vue', import.meta.url), 'utf8')

test('GBFR Logs findings state the exact scope without implying weapon or virtual-sigil auditing', () => {
  assert.match(source, /12 个物理因子槽/)
  assert.match(source, /武器自带技能和本工具运行时虚拟因子不在这套规则输入中/)
  assert.match(source, /低概率提示不等同于作弊证明/)
})

test('GBFR Logs findings distinguish table breaches from odds reports', () => {
  assert.match(source, /selectedPlayer\.legalityFindings/)
  assert.match(source, /finding\.hardBreach \? 'is-hard' : 'is-odds'/)
  assert.match(source, /finding\.odds != null/)
  assert.match(source, /item\.legalityFindingCount/)
})
