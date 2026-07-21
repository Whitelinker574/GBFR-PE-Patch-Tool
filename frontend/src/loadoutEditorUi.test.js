import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./components/LoadoutEditor.vue', import.meta.url), 'utf8')
const viewer = readFileSync(new URL('./components/LoadoutViewer.vue', import.meta.url), 'utf8')
const patchTool = readFileSync(new URL('./components/PatchTool.vue', import.meta.url), 'utf8')

test('mastery starts collapsed while the three-direction summary stays visible', () => {
  assert.match(source, /const masteryExpanded = ref\(false\)/)
  const toggle = source.indexOf('class="mastery-toggle"')
  const summary = source.indexOf('class="mastery-direction-map"')
  const details = source.indexOf('v-if="masteryExpanded" class="mastery-panel"')
  assert.ok(toggle >= 0 && summary > toggle && details > summary,
    'the direction summary must sit between the toggle and collapsible details')
})

test('mastery direction is derived from selected nodes without a manual direction picker', () => {
  assert.match(source, /inferMasteryDirection\(masteryPick\.value, masteryNodeByHash\.value\)/)
  assert.doesNotMatch(source, /class="direction-picker"/)
  assert.doesNotMatch(source, /chooseMasteryDirection/)
  assert.doesNotMatch(source, /applyMasteryDirection/)
  assert.doesNotMatch(source, /isMasteryNodeSelectable/)
})

test('specialization effects remain directly visible in the three-direction summary', () => {
  assert.match(source, /row\.effect/)
  assert.match(source, /class="direction-effect"/)
})

test('result sidebar follows the stable overview, skills, totals and mastery order', () => {
  const titles = ['角色效果总计', '技能效果', '总计加成', '专精解锁内估算']
  const positions = titles.map(title => source.indexOf(`<strong>${title}</strong>`))
  assert.ok(positions.every(position => position >= 0), `missing result heading: ${positions}`)
  assert.deepEqual([...positions].sort((a, b) => a - b), positions)
})

test('single-loadout import and export stay in the sticky save bar at every editor size', () => {
  const bar = source.match(/<div class="editor-save-bar">([\s\S]*?)<\/div>\s*<\/div>/)?.[0] || ''
  assert.match(bar, /class="single-loadout-label">单套配装<\/small>/)
  assert.match(bar, /@click="exportCurrentLoadout">导出单套<\/button>/)
  assert.match(bar, /@click="importLoadout">导入单套<\/button>/)
  assert.match(bar, /@click="apply"/)
  assert.equal((source.match(/@click="exportCurrentLoadout"/g) || []).length, 1)
  assert.equal((source.match(/@click="importLoadout"/g) || []).length, 1)
  assert.match(source, /\.editor-save-bar\s*\{[^}]*position:sticky/is)
  assert.match(source, /@container\s+loadout-editor\s*\(max-width\s*:\s*760px\)[\s\S]*?\.editor-persistent-actions\s*\{[^}]*grid-template-columns\s*:\s*repeat\(2,minmax\(0,1fr\)\)/is)
})

test('editor typography uses standard font weights only', () => {
  assert.doesNotMatch(source, /font-weight\s*:\s*(?:[58]50|560|680|750|800|900)\b/)
  assert.match(source, /Microsoft YaHei UI/)
})

test('factor cards keep all four lines without becoming oversized tiles', () => {
  const rule = source.match(/\.factor-slot-card\s*\{[^}]*min-height\s*:\s*(\d+)px/is)
  assert.ok(rule, 'factor slot card rule is missing')
  assert.ok(Number(rule[1]) >= 88, `factor card is only ${rule[1]}px tall`)
  assert.ok(Number(rule[1]) <= 96, `factor card is still an oversized ${rule[1]}px tile`)
})

test('character profile distinguishes four runtime-exact values from the draft estimate', () => {
  assert.match(source, /LoadoutStatContext/)
	assert.match(source, /LoadoutRuntimePanelStats/)
	assert.match(source, /aria-label="人物属性面板"/)
	for (const label of ['HP', '攻击力', '暴击率', '昏厥值', '防御力加成', '伤害上限']) {
		assert.match(source, new RegExp(`>${label}<`))
	}
	assert.match(source, /游戏真实回读/)
	assert.match(source, /配装草稿估算/)
	assert.match(source, /从游戏读取/)
	assert.match(source, /formatPanelStat/)
	assert.doesNotMatch(source, /最终人物属性/)
	assert.match(source, /formatFinalStat/)
	assert.doesNotMatch(source, /\.profile-stat-cap\s*\{[^}]*grid-column\s*:\s*1\s*\/\s*-1/is)
  assert.match(source, /v-for="index in 4"/)
  assert.match(source, /statContext\.summons/)
})

test('character values have a visible hierarchy and use the full profile width', () => {
	assert.match(source, /class="profile-stat-card"/)
	assert.match(source, /class="profile-stat-heading"[\s\S]*<strong>人物属性<\/strong>[\s\S]*游戏真实回读/)
	assert.match(source, /<dl class="profile-stats" aria-label="人物属性面板">/)
	assert.equal((source.match(/class="profile-stat-value"/g) || []).length, 6)
	assert.match(source, /\.profile-stat-card\s*\{[^}]*grid-column\s*:\s*1\s*\/\s*-1/is)
	assert.match(source, /\.profile-stats\s*\{[^}]*grid-template-columns\s*:\s*repeat\(2,\s*minmax\(0,\s*1fr\)\)/is)
	assert.match(source, /\.profile-stat-value\s*\{[^}]*white-space\s*:\s*nowrap/is)
})

test('defense calculation is explicit, sourced and does not pretend to be an absolute panel stat', () => {
	assert.match(source, /formatFinalStat\(finalStats\?\.defenseBonus, 'signedPct'\)/)
	assert.match(source, /formatFinalStat\(finalStats\?\.damageTakenRate, 'pct'\)/)
	assert.match(source, /预计受击倍率/)
	assert.match(source, /配装防御加成/)
	assert.match(source, /无条件防御力按百分比降低受击伤害/)
	assert.match(source, /\.defense-scope-note\s*\{[^}]*border-left\s*:\s*2px/is)
	assert.doesNotMatch(source, />最终防御力</)
})

test('offline values expose approximation and every backend warning without claiming false precision', () => {
	assert.match(source, /const calculationWarnings = computed/)
	assert.match(source, /finalStats\.value\?\.warnings/)
	assert.match(source, /selectedWeaponContext\.value\?\.warnings/)
	assert.match(source, /finalStats\.value\?\.formulaVerified/)
	assert.match(source, /selectedWeaponContext\.value\?\.formulaVerified/)
	assert.match(source, /公式未完全验证/)
	assert.match(source, /return `≈\$\{formatted\}`/)
	assert.match(source, /v-for="warning in calculationWarnings"/)
})

test('weapon panel reads the serialized attack field instead of showing a false missing value', () => {
	assert.match(source, /selectedWeaponContext\.total\?\.attack/)
	assert.doesNotMatch(source, /selectedWeaponContext\.total\?\.atk(?![A-Za-z0-9_$])/)
})

test('verified character hash selects the matching official compact icon', () => {
  assert.match(source, /import \{[^}]*characterAssetIcon[^}]*\} from '\.\.\/gameAssetIcons'/)
  assert.match(source, /const characterAvatar = computed\(\(\) => characterAssetIcon\(props\.charaHash\)\)/)
  assert.match(source, /<img v-if="characterAvatar" :src="characterAvatar"/)
})

test('complete build simulation follows weapon, factors, mastery and summon slots', () => {
  assert.match(source, /LoadoutSimulateBuild/)
	assert.match(source, /form\.value\.weaponSlotId[\s\S]*payload\.sigilSlotIds[\s\S]*selectedMasteryHashes\.value[\s\S]*summonSlotIds\.value/)
	assert.match(source, /watch\(\(\)\s*=>\s*form\.value\.weaponSlotId\s*,\s*refreshSim\)/)
	assert.match(source, /watch\(\(\)\s*=>\s*selectedMasteryHashes\.value\.slice\(\)\s*,\s*refreshSim/)
  assert.match(source, /w\.summonSlotIds\s*=\s*\[\.\.\.summonSlotIds\.value\]/)
})

test('calculation scope is explicit beside merged totals', () => {
	const scope = '人物属性以存档中的角色基础值、命运篇章与角色强化为固定基准；加成明细默认只汇总可随时更换的武器（含武器技能）、因子、专精、角色上限突破与召唤石，不含任务、队伍、临时状态及战斗内条件加成。'
	assert.equal(source.split(scope).length - 1, 1)
	const totalsTitle = source.indexOf('<strong>总计加成</strong>')
	const note = source.indexOf(scope)
	const list = source.indexOf('class="effect-total-list"')
	assert.ok(totalsTitle >= 0 && note > totalsTitle && list > note)
})

test('character detail separates save base, permanent growth and the fixed baseline', () => {
  for (const field of ['baseHp', 'baseAtk', 'permanentGrowth?.fateHp', 'permanentGrowth?.fateAtk', 'permanentGrowth?.masterHp', 'permanentGrowth?.masterAtk', 'baselineHp', 'baselineAtk', 'baselineStun', 'baselineCritRate', 'baselineDamageCap']) {
    assert.ok(source.includes(`statContext.${field}`), `${field} is absent from the character detail`)
  }
  assert.match(source, /固定基准 HP/)
  assert.match(source, /角色强化伤害上限/)
})

test('character detail exposes all four permanent mastery tabs and stun raw/display units', () => {
	for (const tab of ['attackTab', 'defenseTab', 'collectionTab', 'transcendenceTab']) {
		assert.match(source, new RegExp(`legacyMastery\\?\\.${tab}`))
	}
	for (const label of ['攻击强化', 'HP・抗性', '武器收集加成', '上限突破']) assert.ok(source.includes(label))
	assert.match(source, /原始 f32/)
	assert.match(source, /×10 面板/)
})

test('mastery separates editable structure from the character current unlock and effective calculation', () => {
	assert.match(source, /permanentGrowth\?\.masteryRankCaps/)
	assert.match(source, /permanentGrowth\.masterTotalMsp/)
	assert.match(source, /无法区分“专精系统尚未开放”和“已开放但尚未获得 MSP”/)
	assert.match(source, /const masteryStructuralRankCap =/)
	assert.match(source, /const masteryUnlockedRankCap =/)
	assert.match(source, /const effectiveMasteryHashes = computed/)
	assert.match(source, /limitMasteryHashesByRankCaps/)
	assert.match(source, /toggleNode\(activeRankPool\.rank, n\.hash, masteryStructuralRankCap\(activeRankPool\.rank\)\)/)
	assert.match(source, /草稿 \{\{ selectedMasteryHashes\.length \}\}\/\{\{ masteryCapacity \}\}/)
	assert.match(source, /解锁内估算 \{\{ effectiveMasteryHashes\.length \}\}\/\{\{ masteryUnlockedCapacity \}\}/)
	assert.match(source, /离线属性暂按各阶存档顺序截取到当前容量/)
	assert.match(source, /masteryUnlockedRankCap\(rank\.rank\)/)
	assert.match(source, /角色强化 Lv\{\{ statContext\.permanentGrowth\?\.masterLevel \|\| 1 \}\} HP/)
})

test('final stats expose the three independent damage-cap totals in a compact drill-down', () => {
	assert.match(source, /class="final-stat-detail-disclosure[^\"]*"/)
	assert.match(source, />普通伤害上限</)
	assert.match(source, /finalStats\?\.normalDamageCap/)
	assert.match(source, />能力伤害上限</)
	assert.match(source, /finalStats\?\.abilityDamageCap/)
	assert.match(source, />奥义伤害上限</)
	assert.match(source, /finalStats\?\.skyboundDamageCap/)
	assert.match(source, /\.cap-detail-grid\s*\{[^}]*grid-template-columns\s*:\s*repeat\(3,/is)
})

test('merged totals surface effective and overflow trait levels without another permanent panel', () => {
	assert.match(source, /summarizeTraitLevels/)
	assert.match(source, /const traitLevelSummary = computed/)
	assert.match(source, /class="trait-level-ledger"/)
	assert.match(source, /class="trait-level-ledger"[\s\S]*?有效[\s\S]*?\{\{\s*traitLevelSummary\.effective\s*\}\}/)
	assert.match(source, /class="trait-level-ledger"[\s\S]*?溢出[\s\S]*?\{\{\s*traitLevelSummary\.overflow\s*\}\}/)
	assert.match(source, /v-if="traitLevelSummary\.overflow > 0"/)
})

test('weapon skills are visible and traceable in the result sidebar', () => {
	assert.match(source, /const weaponSkills = ref\(\[\]\)/)
	assert.match(source, /weaponSkills\.value\s*=\s*result\?\.weaponSkills\s*\|\|\s*\[\]/)
	assert.match(source, />武器技能</)
	assert.match(source, /skill\.name/)
	assert.match(source, /formatWeaponSkillLevel\(skill\)/)
	assert.match(source, /skill\.effect/)
	assert.match(source, /skill\.sourceWeapon/)
	assert.match(source, /skill\.unlockCondition/)
	assert.match(source, /解锁阶段/)
})

test('weapon skill rows keep missing fields honest instead of rendering undefined', () => {
	assert.match(source, /import \{[^}]*formatFinalStat[^}]*formatWeaponSkillLevel[^}]*\} from '\.\.\/loadoutFinalStats'/)
	assert.match(source, /formatWeaponSkillLevel\(skill\)/)
	assert.match(source, /skill\.name \|\| '未收录武器技能'/)
	assert.match(source, /class="weapon-skill-effect"[\s\S]*skill\.effect/)
	assert.match(source, /暂无可验证效果说明/)
	assert.match(source, /来源 · \{\{ skill\.sourceWeapon \|\| '未收录武器' \}\}/)
	assert.match(source, /有效等级 · 游戏运行时 \{\{ skill\.stableReads \}\} 次稳定读取；静态表 Lv\{\{ skill\.staticLevel \}\}/)
})

test('runtime calibration exposes per-stat deltas and the effective wrightstone snapshot', () => {
	assert.match(source, /const runtimeStatComparisons = computed/)
	assert.match(source, /当前是两套不同配装，不是校准失败/)
	assert.match(source, /const factorComparison = computed/)
	assert.match(source, /当前游戏与草稿的因子不同，不是同一套配装/)
	assert.match(source, /第 \$\{index \+ 1\} 槽：草稿 \$\{draftName\}，游戏 \$\{runtimeName\}/)
	assert.match(source, /runtimePanelStats\.currentFactorStableReads/)
	assert.match(source, /草稿来源/)
	assert.match(source, /游戏当前/)
	assert.match(source, /runtimePanelStats\.currentWeaponSlotId/)
	assert.match(source, /formatComparisonDelta\(row\.delta, row\.unit\)/)
	assert.match(source, /class="wrightstone-audit"/)
	assert.match(source, /wrightstone\.runtimeObserved/)
	assert.match(source, /游戏运行时 · \{\{ selectedWeaponContext\.wrightstone\.stableReads \}\} 次稳定读取/)
	assert.match(source, /全来源合并 Lv\{\{ mergedTraitBonus\(trait\)\.rawLevel \}\}/)
	assert.match(source, /不会凭武器类型补假数据/)
})

test('pre-DLC saves keep loadout preview open when summon and mastery systems are unavailable', () => {
	assert.match(source, /!statContext\.summonSystemAvailable/)
	assert.match(source, /尚未进入或初始化召唤石系统/)
	assert.match(source, /预览按无召唤石效果继续，空槽不会报错/)
	assert.match(source, /!statContext\.permanentGrowth\?\.masterSystemAvailable/)
	assert.match(source, /缺失层按未开启处理/)
})

test('mastery details distinguish verified panel scale from unpacked raw text', () => {
	assert.match(source, /解包原始文本：\{\{ node\.rawDesc \}\}/)
	assert.match(source, /显示尺度 ×\{\{ node\.displayScale \}\}/)
})

test('fixed character growth identifies character-specific runtime evidence', () => {
	assert.match(source, /statContext\.permanentGrowth\?\.runtimeObserved/)
	assert.match(source, /角色基础、命运篇章、角色强化固定成长：2\.0\.2 运行时状态对象/)
	assert.match(source, /连续 \{\{ statContext\.permanentGrowth\?\.stableReads \}\} 次稳定读取（角色独立）/)
})

test('dedicated editing hides the global portrait background while ordinary viewing retains it', () => {
	assert.match(viewer, /defineEmits\(\['status', 'editing-change'\]\)/)
	assert.match(viewer, /emit\('editing-change', value\)/)
	assert.match(patchTool, /const isLoadoutWorkspace = computed\(\(\) => activeTab\.value === 'loadoutPresets' && loadoutEditing\.value\)/)
	assert.match(patchTool, /class="tool-stage"[^>]*:style="\{ '--function-art': `url\('\$\{currentArt\}'\)` \}"/)
	assert.match(patchTool, /\.tool-stage\.loadout-dedicated::before\s*\{[^}]*display:none/)
	assert.doesNotMatch(source, /class="art-rail"/)
})

test('simulation request sequencing prevents stale results from replacing the current build', () => {
	assert.match(source, /let simRequestId = 0/)
	assert.match(source, /requestId !== simRequestId/)
	assert.match(source, /async function loadCtx\(\)\s*\{\s*simRequestId\+\+\s*\n\s*clearTimeout\(simTimer\)\s*\n\s*clearSimulationResult\(\)/)
})

test('column shells use quiet section separators instead of stacked inner shadows', () => {
  assert.match(source, /\.result-card\s*\{[^}]*box-shadow\s*:\s*none/is)
  assert.match(source, /\.setup-column\s*>\s*\*\s*\+\s*\*/)
  assert.match(source, /\.result-card\s*\+\s*\.result-card/)
})

test('successful parent reload rehydrates the editor from the newly read save', () => {
  assert.match(source, /watch\(\(\)\s*=>\s*props\.loadouts\s*,/)
})

test('maximised editor fills its workspace while keeping bounded side-column measures', () => {
	assert.match(source, /\.loadout-editor\s*\{[^}]*width\s*:\s*100%/is)
	assert.match(source, /grid-template-columns\s*:\s*clamp\(250px,\s*20vw,\s*360px\)\s+minmax\(540px,\s*1fr\)\s+clamp\(280px,\s*22vw,\s*400px\)/)
	assert.match(source, /justify-content\s*:\s*stretch/)
	assert.doesNotMatch(source, /\.editor-layout\s*\{[^}]*justify-content\s*:\s*center/is)
  assert.match(source, /container\s*:\s*loadout-editor\s*\/\s*inline-size/)
  assert.match(source, /@container\s+loadout-editor\s*\(max-width\s*:\s*1199px\)/)
})

test('factor and bag grids add columns by card measure instead of stretching a few giant cards', () => {
  assert.match(source, /\.factor-slot-grid\s*\{[^}]*repeat\(auto-fit,\s*minmax\(190px,\s*1fr\)\)/is)
  assert.match(source, /\.pick-grid\.sigils\s*\{[^}]*repeat\(auto-fit,\s*minmax\(260px,\s*1fr\)\)/is)
})

test('summon selectors are explicit global equipment and only write after opt-in', () => {
  assert.match(source, /const writeGlobalSummons = ref\(false\)/)
  assert.match(source, /全局已装备召唤石（独立于单套配装）/)
  assert.match(source, /v-model="writeGlobalSummons"/)
  assert.match(source, /if \(writeGlobalSummons\.value\) w\.summonSlotIds = \[\.\.\.summonSlotIds\.value\]/)
  assert.match(source, /writeGlobalSummons\.value[\s\S]*全局四槽/)
})

test('merged total names and source ledgers wrap instead of being ellipsized', () => {
  assert.match(source, /\.effect-total-row\s*>\s*span\s+b\s*\{[^}]*white-space\s*:\s*normal/is)
  assert.match(source, /\.effect-total-row\s*>\s*span\s+small\s*\{[^}]*white-space\s*:\s*normal/is)
})

test('fullscreen editor keeps a persistent save action and compact preset metadata', () => {
	assert.match(source, /class="editor-save-bar"/)
	assert.match(source, /saveButtonLabel/)
	assert.match(source, /class="editor-save-button[^"]*"[^>]*@click="apply"/)
	assert.match(source, /\.editor-save-bar\s*\{[^}]*position\s*:\s*sticky/is)
	assert.match(viewer, /class="preset-count-badge"[\s\S]*\{\{\s*currentGroup\.loadouts\.filter\(l => !l\.isParty\)\.length\s*\}\}[\s\S]*套已有预设/)
	assert.doesNotMatch(viewer, /class="editor-workspace-meta">\s*<b>/)
})

test('constructor and bag controls expose real filtering, sorting and empty states', () => {
	assert.match(source, /from '\.\.\/loadoutCatalogFilters'/)
	assert.match(source, /watch\(filteredConstructCatalog/)
	assert.match(source, /构造目录无匹配结果/)
	for (const model of ['bagStateFilter', 'bagTraitFilter', 'bagSort']) {
		assert.match(source, new RegExp(`v-model="${model}"`))
	}
	assert.match(source, /未装入当前草稿/)
	assert.match(source, /主词条等级从高到低/)
})

test('constructor exposes the complete trait catalog while natural table rules remain advisory', () => {
	assert.match(source, /GetSigilList/)
	assert.match(source, /GetTraitList/)
	assert.match(source, /GetCompatibleSecondaryTraits/)
	assert.match(source, /Promise\.all\(\[GetSigilList\(\), GetTraitList\(\)\]\)/)
	assert.match(source, /import CatalogSelect from '.\/CatalogSelect\.vue'/)
	assert.match(source, /v-model="constructPrimaryId" :options="constructTraits"/)
	assert.match(source, /v-model="constructSecondaryId"/)
	assert.match(source, /search-placeholder="搜索副词条"/)
	assert.match(source, /constructSecondaryOptions = computed\(\(\) => constructTraits\.value\)/)
	assert.match(source, /天然因子组合与等级只作提醒/)
	assert.doesNotMatch(source, /forceWrite/)
	assert.doesNotMatch(source, /templateSlotId:\s*Number\(sigil\.templateSlotId/)
})

test('narrow weapon skill editor uses a single shrinkable column', () => {
	assert.match(source, /\.weapon-skill-edit-row\s*\{[^}]*grid-template-columns\s*:\s*minmax\(0,\s*1fr\)/is)
	assert.match(source, /\.weapon-skill-edit-row\s+\.ui-select\s*\{[^}]*width\s*:\s*100%/is)
	assert.match(source, /\.sim-name\s*\{[^}]*min-width\s*:\s*0[^}]*white-space\s*:\s*normal[^}]*overflow-wrap\s*:\s*anywhere/is)
	assert.match(source, /\.sim-lv\s*\{[^}]*white-space\s*:\s*nowrap/is)
})
