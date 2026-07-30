import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./components/LoadoutOptimizer.vue', import.meta.url), 'utf8')

test('smart loadout keeps one compact result region directly below both solve flows', () => {
  const autoSolve = source.indexOf('生成这条路线')
  const targetSolve = source.indexOf('按目标生成方案')
  const output = source.indexOf('data-testid="optimizer-output"')
  assert.ok(autoSolve >= 0, 'automatic recommendation action is missing')
  assert.ok(targetSolve >= 0, 'target-skill solve action is missing')
  assert.ok(output > autoSolve && output > targetSolve, 'both solve flows must lead into the same visible result region')
  assert.match(source, /点击计算后，方案会显示在这里，并可直接预览 12 个因子槽。/)
  assert.doesNotMatch(source, /\.optimizer-empty\s*\{[^}]*min-height\s*:\s*(?:[1-9]\d{2}px|[1-9]\d?rem)/s)
})

test('the first solution previews twelve slots immediately while alternatives expand on demand', () => {
  assert.match(source, /class="[^"]*\bsolution-slot-grid\b[^"]*"/)
  assert.match(source, /v-if="isResultExpanded\(result, index\)"/)
  assert.match(source, /展开 12 槽/)
  assert.match(source, /12 个因子槽预览/)
  assert.match(source, /目标技能达成/)
  assert.match(source, /最终技能等级/)
  assert.match(source, /function localizedResultTotals\(result\)/)
  assert.match(source, /\^\(\?:MEMORY\|SKILL\|TRAIT\|SIGIL\|ABILITY\|WEAPON\|SUMMON\)_/)
})

test('advanced battle conditions use aligned field rows and explicit unit suffixes', () => {
  assert.match(source, /class="battle-condition-grid"/)
  assert.match(source, /class="condition-field"/)
  assert.match(source, /class="condition-suffix"[^>]*>%<\/span>/)
  assert.match(source, /class="condition-check"/)
  assert.match(source, /基准单次伤害/)
  assert.match(source, /最低减伤约束/)
  assert.match(source, /incomingDamage:\s*Number\(incomingDamage\.value/)
  assert.match(source, /minimumDefense:\s*Number\(minimumDefense\.value/)
})

test('worker messages are converted to cloneable plain data at the postMessage boundary', () => {
  assert.match(source, /createOptimizerWorkerMessage/)
  assert.match(source, /worker\.postMessage\(createOptimizerWorkerMessage\(/)
})

test('frame-verified routes use the linear twelve-slot assignment instead of the full backpack combat search', () => {
  assert.match(source, /solveFixedRoute:\s*true/)
  assert.match(source, /targets:\s*selectedRouteRequiredTargets\.value/)
  assert.match(source, /catalogCandidates:\s*domain\.value === 'inventory' \? \[\] : catalogCandidates/)
  assert.match(source, /12 个主因子槽按固定路线排满/)
})

test('graduation builds expose flat, explainable direction branches in the character loadout page', () => {
  assert.match(source, /graduationRouteBranches/)
  assert.match(source, /先选毕业母路线，再选实战方向/)
  assert.match(source, /路线方向/)
  assert.match(source, /都从上面的毕业配装发散，不是另一套通用模板/)
  assert.match(source, /class="character-route-tabs route-branch-tabs ui-seg"/)
  assert.match(source, /本方向改了什么/)
  assert.match(source, /这是依据内置 2\.0\.2 词条效果从已核对毕业路线推导出的方向分支/)
  assert.match(source, /characterRouteBranchVersion/)
  assert.match(source, /\.route-branch-tabs \.ui-seg-btn \{[^}]*font-size:13px/s)
  assert.doesNotMatch(source, /\.route-branch-tabs \.ui-seg-btn \{[^}]*box-shadow:(?!none)/s)
})

test('automatic recommendations expose character, skill, cap, and defense evidence', () => {
  assert.match(source, /当前角色独立档案/)
  assert.match(source, /selectedSkillNames/)
  assert.match(source, /角色伤害上限表/)
  assert.match(source, /防御端核对/)
  assert.match(source, /offenseGap/)
  assert.match(source, /尚未逐动作校准的倍率、角色机制与完整轮转不会冒充精确结论/)
  assert.match(source, /suppressedManufacturedCount/)
  assert.match(source, /不会为了“能制造”而推荐更差配装/)
})
