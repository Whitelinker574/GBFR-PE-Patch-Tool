import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const source = readFileSync(new URL('./components/LoadoutEditor.vue', import.meta.url), 'utf8')
const viewer = readFileSync(new URL('./components/LoadoutViewer.vue', import.meta.url), 'utf8')
const importDialog = readFileSync(new URL('./components/LoadoutImportDialog.vue', import.meta.url), 'utf8')
const patchTool = readFileSync(new URL('./components/PatchTool.vue', import.meta.url), 'utf8')

test('mastery starts collapsed while the three-direction summary stays visible', () => {
  assert.match(source, /const masteryExpanded = ref\(false\)/)
  const toggle = source.indexOf('class="mastery-toggle"')
  const summary = source.indexOf('class="mastery-direction-strip"')
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

test('mastery summary is compact and distinguishes MLv from node capacity', () => {
  assert.match(source, /MLv\{\{ currentMasterLevel \}\}\/55/)
  assert.match(source, /permanentGrowth\?\.masterProgressIndex/)
  assert.match(source, /节点 \{\{ selectedMasteryHashes\.length \}\}\/50/)
  assert.match(source, /50 是专精配置节点总容量（10 \/ 10 \/ 10 \/ 20），不是专精等级/)
  assert.match(source, /direction\.activeStages/)
  assert.doesNotMatch(source, /class="direction-effect"/)
})

test('expanded mastery shows every stage effect and its real node threshold', () => {
  assert.match(source, /aria-label="当前专精逐阶效果"/)
  assert.match(source, /1阶 3 项、2阶 6 项、3阶沿用主方向 6 项/)
  assert.match(source, /v-for="row in direction\.rows"/)
  assert.match(source, /row\.active \? '已生效'/)
  assert.match(source, /\{\{ row\.effect \|\| '该阶段没有可显示的独立效果文本' \}\}/)
  assert.doesNotMatch(source, /masteryStageSkillPicked/)
})

test('result sidebar contains exactly the dynamic skill ledger and merged totals', () => {
  const titles = ['动态技能等级', '动态加成汇总']
  const positions = titles.map(title => source.indexOf(`<strong>${title}</strong>`))
  assert.ok(positions.every(position => position >= 0), `missing result heading: ${positions}`)
  assert.deepEqual([...positions].sort((a, b) => a - b), positions)
  assert.equal((source.match(/<section class="result-card/g) || []).length, 2)
  assert.doesNotMatch(source, /class="result-card (?:result-overview|skill-summary-card|mastery-summary-card)/)
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
	assert.match(source, /finalStats\?\.defenseModel\?\.zones/)
	assert.match(source, /防御分区/)
	assert.match(source, /同区相加，跨区相乘/)
	assert.match(source, /zone\.evidence/)
	assert.match(source, /预计受击倍率/)
	assert.match(source, /配装防御加成/)
	assert.match(source, /伊欧 \+5% 实测/)
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

test('dynamic calculation scope excludes fixed character progression', () => {
	const scope = '只汇总会随当前配装变化的武器数值、因子、武器技能、祝福、专精与召唤石效果。角色任务、角色强化、命运篇章和上限突破等固定基础成长保留在左侧，不在这里重复计算。'
	assert.equal(source.split(scope).length - 1, 1)
	const totalsTitle = source.indexOf('<strong>动态加成汇总</strong>')
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
	assert.match(source, /节点 \{\{ selectedMasteryHashes\.length \}\}\/50/)
	assert.match(source, /当前生效 \{\{ effectiveMasteryHashes\.length \}\}\/\{\{ masteryUnlockedCapacity \}\}/)
	assert.match(source, /离线属性暂按各阶存档顺序截取到当前容量/)
	assert.match(source, /masteryUnlockedRankCap\(p\.rank\)/)
	assert.match(source, /角色强化产生的固定 HP、攻击等基础成长只保留在左侧人物属性/)
})

test('editor final stats keep the four independent damage-cap totals in a compact drill-down', () => {
	assert.match(source, /class="final-stat-detail-disclosure[^\"]*"/)
	assert.match(source, />普通伤害上限</)
	assert.match(source, /finalStats\?\.normalDamageCap/)
	assert.match(source, />能力伤害上限</)
	assert.match(source, /finalStats\?\.abilityDamageCap/)
	assert.match(source, />奥义伤害上限</)
	assert.match(source, /finalStats\?\.skyboundDamageCap/)
	assert.match(source, />奥义连锁上限</)
	assert.match(source, /finalStats\?\.chainDamageCap/)
	assert.match(source, /\.cap-detail-grid\s*\{[^}]*grid-template-columns\s*:\s*repeat\(4,/is)
})

test('merged totals surface effective and overflow trait levels without another permanent panel', () => {
	assert.match(source, /summarizeTraitLevels/)
	assert.match(source, /const traitLevelSummary = computed/)
	assert.match(source, /class="trait-level-ledger"/)
	assert.match(source, /class="trait-level-ledger"[\s\S]*?有效[\s\S]*?\{\{\s*traitLevelSummary\.effective\s*\}\}/)
	assert.match(source, /class="trait-level-ledger"[\s\S]*?溢出[\s\S]*?\{\{\s*traitLevelSummary\.overflow\s*\}\}/)
	assert.match(source, /v-if="traitLevelSummary\.overflow > 0"/)
})

test('all passive skill sources are merged, sorted and expandable', () => {
	assert.match(source, /const dynamicSkillLedger = computed/)
	assert.match(source, /Number\(right\.rawLevel \|\| 0\) - Number\(left\.rawLevel \|\| 0\)/)
	assert.match(source, /<details v-for="bonus in dynamicSkillLedger"/)
	assert.match(source, /bonus\.effect/)
	assert.match(source, /bonus\.sources/)
	assert.match(source, /因子技能、武器技能、武器祝福与召唤石技能按同名效果合并/)
})

test('dynamic totals use the dedicated backend slice and keep source attribution', () => {
	assert.match(source, /dynamicTotals\.value = result\?\.dynamicTotals \|\| result\?\.totals \|\| \[\]/)
	assert.match(source, /const displayDynamicTotals = computed\(\(\) => groupEffectTotals\(dynamicTotals\.value\)\)/)
	assert.match(source, /v-for="total in displayDynamicTotals"/)
	assert.match(source, /v-for="source in total\.sources"/)
	assert.doesNotMatch(source, /const weaponSkills = ref/)
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
	assert.match(source, /bonus\.sources\?\.length/)
	assert.match(source, /class="dynamic-source-list"/)
	assert.match(source, /当前等级暂无可验证的数值说明/)
})

test('pre-DLC saves keep loadout preview open when summon and mastery systems are unavailable', () => {
	assert.match(source, /!statContext\.summonSystemAvailable/)
	assert.match(source, /尚未进入或初始化召唤石系统/)
	assert.match(source, /预览按无召唤石效果继续，空槽不会报错/)
	assert.match(source, /!statContext\.permanentGrowth\?\.masterSystemAvailable/)
	assert.match(source, /缺失层按未开启处理/)
})

test('mastery details are folded into dynamic totals instead of a duplicate sidebar panel', () => {
	assert.match(source, /动态加成汇总/)
	assert.doesNotMatch(source, /mastery-detail-disclosure/)
	assert.doesNotMatch(source, /专精解锁内估算/)
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
	assert.match(source, /grid-template-columns\s*:\s*clamp\(240px,\s*17vw,\s*310px\)\s+minmax\(500px,\s*1fr\)\s+clamp\(380px,\s*30vw,\s*520px\)/)
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
  assert.match(source, /writeGlobalSummons\.value = draft\.summonSlotIds\.every\(\(slotId, index\) => Number\(slotId\) > 0 \|\| generated\.has\(index\)\)/)
})

test('single-loadout import constructs independent factors and blocks partial writes', () => {
  assert.match(source, /for \(const constructed of draft\.constructedSigils \|\| \[\]\)/)
  assert.match(source, /putConstructedFactor\(/)
  assert.match(source, /importMissing\.value\.length > 0/)
  assert.match(source, /op === 'write' && importMissing\.length/)
  assert.match(source, /为避免只写入部分配装，保存已锁定/)
  assert.match(source, /missingByScope/)
  assert.match(source, /applyWeaponEnhancement:\s*!!choices\.weaponEnhancement/)
  assert.match(source, /constructedWeapon:\s*choices\.weapon/)
  assert.match(source, /createdWeaponCount/)
  assert.match(source, /applyCharacterLevel:\s*!!choices\.characterLevel/)
  assert.match(source, /applyCharacterGrowth:\s*!!choices\.characterGrowth/)
  assert.match(source, /applyCharacterWeaponCollection:\s*!!choices\.characterWeaponCollection/)
	assert.match(source, /applyCharacterWeaponWrightstones:\s*!!choices\.characterWeaponWrightstones/)
  assert.match(importDialog, /命运篇章始终保留目标存档/)
  assert.match(importDialog, /角色等级[\s\S]*基础 HP、攻击、昏厥与暴击快照/)
  assert.match(importDialog, /needsLevel100Selection[\s\S]*mastery[\s\S]*masterProgress[\s\S]*characterGrowth/)
  assert.match(importDialog, /将自动补足角色等级/)
  assert.match(importDialog, /命运篇章保持目标值/)
  assert.match(importDialog, /命运篇章 \{\{ targetFateLabel \}\}/)
  assert.match(importDialog, /只导入武器祝福/)
  assert.match(importDialog, /目标存档缺少同款[\s\S]*新增来源武器并绑定/)
  assert.match(importDialog, /随新武器同步/)
  assert.match(importDialog, /缺少时自动新增并登记/)
  assert.match(importDialog, /上限突破[\s\S]*可选择不覆盖/)
  assert.match(importDialog, /角色强化进度[\s\S]*目标未满级时联动升至 Lv100，不改命运篇章或任何武器/)
  assert.match(importDialog, /整组角色武器收藏[\s\S]*同步该角色全部武器/)
	assert.match(importDialog, /同步全部武器祝福[\s\S]*实际生效的三条附加技能/)
  assert.match(importDialog, /characterGrowth:\s*false/)
  assert.match(importDialog, /characterLevel:\s*false/)
  assert.match(importDialog, /characterWeaponCollection:\s*false/)
	assert.match(importDialog, /characterWeaponWrightstones:\s*false/)
  assert.match(importDialog, /MLv\{\{ masteryProgress \}\}/)
  assert.match(importDialog, /max="55"/)
})

test('preset cards and editor summary expose only weapon skills and complete wrightstone traits', () => {
  assert.match(viewer, /class="weapon-loadout-summary"/)
  assert.match(viewer, /v-for="skill in lo\.weapon\.skills"/)
  assert.match(viewer, /lo\.weapon\.wrightstone/)
	assert.match(viewer, /lo\.weapon\.wrightstone\?\.traits/)
  assert.match(source, /class="equipped-resource-summary"/)
  assert.match(source, /v-for="skill in selectedWeaponContext\.skills"/)
	assert.match(source, /selectedWeaponContext\.wrightstone\?\.traits/)
	const summaryStart = source.indexOf('class="equipped-resource-summary"')
	const summaryEnd = source.indexOf('class="inline-resource-panel weapon-inline-panel"', summaryStart)
	const summary = source.slice(summaryStart, summaryEnd)
	assert.match(summary, />武器技能</)
	assert.match(summary, />武器祝福</)
	assert.doesNotMatch(summary, />主动技能</)
	assert.doesNotMatch(summary, />已装因子</)
})

test('imported mastery keeps the source 3007 order until a manual node edit', () => {
	assert.match(source, /const importedMasterySnapshot = ref\(\[\]\)/)
	assert.match(source, /importedMasterySnapshot\.value = \[\.\.\.\(draft\.masteryHashes \|\| \[\]\)\]/)
	assert.match(source, /w\.masteryHashes = importedMasterySnapshot\.value\.length/)
	assert.match(source, /function toggleNode[\s\S]*?importedMasterySnapshot\.value = \[\]/)
	assert.match(source, /w\.weaponSkillHashes = \[\.\.\.importedWeaponSkillSnapshot\.value\]/)
})

test('the current live loadout is visually promoted ahead of saved presets', () => {
  assert.match(viewer, /\.loadout-card\.party\s*\{[^}]*grid-column\s*:\s*1\s*\/\s*-1[^}]*order\s*:\s*-1/is)
  assert.match(viewer, /当前实时配装/)
})

test('viewer keeps lightweight estimates beside every expandable preset', () => {
	assert.match(viewer, /LoadoutPreviewList/)
	assert.match(viewer, /class="loadout-stat-strip"/)
	assert.match(viewer, /class="expand-mark"/)
	assert.match(viewer, /v-if="expanded\.has\(lo\.unitId\)" class="detail"/)
	assert.doesNotMatch(viewer, /实战数值台|runtime-lab|DamageMeterGetStatus|LoadoutRuntimePanelStats/)
})

test('dynamic skill level and disclosure marker use separate grid columns', () => {
	assert.match(source, /\.result-sidebar\s*\{[^}]*overflow-x\s*:\s*hidden/is)
	assert.match(source, /\.dynamic-skill-entry\s*\{[^}]*max-width\s*:\s*100%[^}]*min-width\s*:\s*0/is)
	assert.match(source, /\.dynamic-skill-entry summary\s*\{[^}]*grid-template-columns\s*:\s*30px\s+minmax\(0,1fr\)\s+minmax\(44px,auto\)\s+14px/is)
	assert.match(source, /\.dynamic-skill-level\s*\{[^}]*grid-column\s*:\s*3/is)
	assert.match(source, /\.dynamic-skill-entry summary::after\s*\{[^}]*grid-column\s*:\s*4/is)
})

test('merged total names and source ledgers wrap instead of being ellipsized', () => {
  assert.match(source, /class="effect-total-sources"/)
  assert.match(source, /<i>关联<\/i>/)
  assert.match(source, /\.effect-total-copy\s*>\s*b\s*\{[^}]*white-space\s*:\s*normal/is)
  assert.match(source, /\.effect-total-sources\s*>\s*span\s*\{[^}]*overflow-wrap\s*:\s*anywhere/is)
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
	assert.match(source, /\.dynamic-skill-title\s*\{[^}]*min-width\s*:\s*0/is)
	assert.match(source, /\.dynamic-skill-level\s*\{[^}]*white-space\s*:\s*nowrap/is)
})
