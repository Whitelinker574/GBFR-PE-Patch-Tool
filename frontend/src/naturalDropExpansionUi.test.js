import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

const source = readFileSync(new URL('./components/NaturalDropLab.vue', import.meta.url), 'utf8')

test('natural drop workspace exposes Transmarvel sigils as a first-class configurable pool', () => {
  assert.match(source, /const sigilSelections = reactive\(\{\}\)/)
  assert.match(source, /workspace\.value\?\.sigils/)
  assert.match(source, /selectedSigils/)
  assert.match(source, /sigils:\s*selectedSigils\.value/)
  assert.match(source, /Transmarvel 因子/)
  assert.match(source, /addSigilDraft/)
  assert.match(source, /加入待部署/)
})

test('natural drop cards reuse exact game icons for summons, factors, traits and items', () => {
  assert.match(source, /import \{ itemAssetIcon, summonAssetIcon, traitAssetIcon \} from '\.\.\/gameAssetIcons'/)
  assert.match(source, /function summonIcon\(item\)/)
  assert.match(source, /function sigilIcon\(item\)/)
  assert.match(source, /function traitIcon\(item\)/)
  assert.match(source, /function pickerItemIcon\(option\)/)
  assert.match(source, /class="drop-item-icon"/)
  assert.match(source, /class="drop-trait-icon"/)
})

test('natural drop uses compact searchable builders and one reviewable deployment ledger', () => {
  assert.match(source, /import CatalogSelect/)
  assert.match(source, /class="drop-builder-grid"/)
  assert.match(source, /pendingDropRows/)
  assert.match(source, /核对待部署清单/)
  assert.match(source, /removePendingDrop/)
  assert.match(source, /clearPendingDrops/)
  assert.match(source, /已清空待部署清单/)
  assert.doesNotMatch(source, /class="sigil-grid"/)
  assert.doesNotMatch(source, /class="summon-grid"/)
  assert.doesNotMatch(source, /class="wrightstone-grid"/)
})

test('sigils wrightstones summons and items share one explicit pending list without mode switches', () => {
  assert.match(source, /四类内容可以同时添加，不需要先选模式/)
  assert.match(source, /class="advanced-drop-panel"/)
  assert.match(source, /指定掉落与锻造内容/)
  assert.match(source, /sigilOnly: false/)
  assert.match(source, /wrightstoneOnly: false/)
  assert.doesNotMatch(source, /transmarvelResultMode/)
  assert.doesNotMatch(source, /混合结果|只出因子|只出祝福石/)
})

test('generic items use a verified automatic reward target', () => {
  assert.match(source, /itemPickerOptions/)
  assert.match(source, /genericDropWeight/)
  assert.match(source, /addItemDraft/)
  assert.match(source, /无尽模式锻造师奖励池/)
  assert.match(source, /itemSelections\[row\.target\]\.quantity/)
  assert.match(source, /min="1" max="999"/)
  assert.doesNotMatch(source, /genericDropRoute/)
  assert.doesNotMatch(source, /genericDropQuantity/)
  assert.doesNotMatch(source, /具体任务\/敌人\/宝箱/)
})

test('custom regular-item entries use exact per-row quantities independent of the live multiplier', () => {
  assert.match(source, /itemMultiplier: 1/)
  assert.match(source, /每次抽中直接掉落/)
  assert.match(source, /清单直接改为 1–999/)
  assert.match(source, /不与上方实时倍率相乘/)
  assert.doesNotMatch(source, /itemRewardMultiplier|itemRewardMultipliers/)
})

test('quest result multiplier is an independent owned runtime switch', () => {
  assert.match(source, /const taskRewardMultipliers = \[1, 2, 4, 8, 16\]/)
  assert.match(source, /CharaAcquire\(nextRuntimeAcquireRequestID\(\)\)/)
  assert.match(source, /TaskRewardMultiplierSetOwned\(taskRewardOwnerToken, multiplier\)/)
  assert.match(source, /queueRuntimeLeaseRelease\(taskRewardLeaseScope, taskRewardOwnerToken, CharaRelease\)/)
  assert.match(source, /全任务普通物品倍率/)
  assert.match(source, /商店购买、锻造和手工物品编辑不受影响/)
  assert.match(source, /因子、祝福石、召唤石和武器属于独立实例奖励/)
  assert.match(source, /保持连接即可持续生效/)
  assert.match(source, /常用功能 · 游戏运行时/)
  assert.match(source, /高级功能 · 需要时再展开/)
  assert.match(source, /跨平台队友是否收到同样倍率，仍需双方实机验收/)
})
