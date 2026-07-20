<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import {
  FormulaSamplerAttach,
  FormulaSamplerCaptureOwned,
  FormulaSamplerCloseOwned,
  FormulaSamplerExport,
	FormulaSamplerObserveOwned,
	FormulaSamplerRuntimeObjects,
	FormulaSamplerStartChangeRecordingOwned,
  FormulaSamplerStatus,
	FormulaSamplerStopChangeRecordingOwned,
} from '../../wailsjs/go/main/App'
import { characterAssetIcon } from '../gameAssetIcons.js'
import { language } from '../i18n.js'
import {
  FORMULA_PHASES,
  formulaNextPhase,
  formulaPhaseCopy,
	normalizeFormulaPanel,
  normalizeFormulaSamplerStatus,
} from '../formulaSamplerView.js'

const emit = defineEmits(['status'])

const characters = Object.freeze([
  ['2A26B1B2', '古兰', 'Gran'], ['A4ACBA76', '姬塔', 'Djeeta'],
  ['18E2F9F9', '卡塔莉娜', 'Katalina'], ['079DF0CC', '拉卡姆', 'Rackam'],
  ['4D0A60C3', '伊欧', 'Io'], ['DD7A151E', '欧根', 'Eugen'],
  ['C8616284', '萝赛塔', 'Rosetta'], ['978E4B18', '冈达葛萨', 'Ghandagoza'],
  ['C3FFD418', '菲莉', 'Ferry'], ['22E437E5', '兰斯洛特', 'Lancelot'],
  ['2EBE91D5', '巴恩', 'Vane'], ['BDEF7181', '珀西瓦尔', 'Percival'],
  ['627BCB0D', '齐格飞', 'Siegfried'], ['FD3BE362', '夏洛特', 'Charlotta'],
  ['BAD16E3B', '索恩', 'Tweyen'], ['FC6CDF7B', '尤达拉哈', 'Yodarha'],
  ['E7053919', '娜露梅亚', 'Narmaya'], ['1BB37EF0', '伽兰查', 'Gallanza'],
  ['0D21B430', '泽塔', 'Zeta'], ['A3A3CB2F', '伊德', 'Id'],
  ['F0EB77EF', '巴萨拉卡', 'Vaseraga'], ['AA66178A', '卡莉奥斯特罗', 'Cagliostro'],
  ['718E1A14', '圣德芬', 'Sandalphon'], ['296471BE', '希耶提', 'Seofon'],
  ['74DD4C79', '菲迪埃尔', 'Fediel'], ['9A8AF295', '贝阿朵丽丝', 'Beatrix'],
  ['25D46F4B', '玛琪拉菲菈', 'Maglielle'], ['9B15CFB1', '尤斯提斯', 'Eustace'],
  ['646C3168', '芙劳', 'Fraux'],
].map(([hash, zh, en]) => Object.freeze({ hash, zh, en })))

const phaseLabels = Object.freeze({ A1: 'A1', B1: 'B1', A2: 'A2', B2: 'B2' })
const experimentTypes = Object.freeze([
  ['sigil', '因子', 'Sigil'], ['weapon', '武器实例', 'Weapon'], ['weapon_skill', '武器技能', 'Weapon skill'],
  ['mastery', '专精', 'Mastery'], ['overlimit', '上限突破', 'Over Mastery'], ['summon', '召唤石', 'Summon'],
  ['hp_condition', 'HP 条件', 'HP condition'], ['battle_condition', '战斗条件', 'Battle condition'],
  ['defense', '防御力', 'Defense'], ['damage_cap', '伤害上限', 'Damage cap'],
  ['control', '空白对照', 'No-change control'], ['other', '其他单项', 'Other one-variable'],
].map(([value, zh, en]) => Object.freeze({ value, zh, en })))
const selectedHash = ref(characters[0].hash)
const selectedExperimentType = ref('sigil')
const sampler = ref(normalizeFormulaSamplerStatus(null))
const busy = ref(false)
const message = ref(copy('ready'))
const tone = ref('info')
const lastExportPath = ref('')
const currentPanel = ref(null)
const runtimeCatalog = ref(null)
const runtimeCatalogMessage = ref('')
const recording = ref(false)
const transitionSamples = ref([])
const changeAnalysis = ref(null)
let observeTimer = null
let observing = false
let disposed = false

const connected = computed(() => sampler.value.connected)
const complete = computed(() => sampler.value.complete)
const nextPhase = computed(() => formulaNextPhase(sampler.value.events))
const selectedCharacter = computed(() => characters.find(item => item.hash === selectedHash.value) || characters[0])
const controlMode = computed(() => (sampler.value.experimentType || selectedExperimentType.value) === 'control')
const selectedRuntimeObject = computed(() => runtimeCatalog.value?.objects?.find(item => item.directoryHash === selectedHash.value) || null)

function fieldEvidence(field) {
	if (!field) return '—'
	return `${field.rawType} @ +0x${Number(field.relativeOffset).toString(16).toUpperCase()} · ×${field.displayScale} · ${field.stableReads}/3`
}

async function refreshRuntimeObjects() {
	try {
		runtimeCatalog.value = await FormulaSamplerRuntimeObjects()
		runtimeCatalogMessage.value = runtimeCatalog.value?.selectionObservation || ''
	} catch (error) {
		runtimeCatalog.value = null
		runtimeCatalogMessage.value = errorText(error)
	}
}

async function observeCurrent() {
	if (!sampler.value.connected || !sampler.value.sessionToken || observing || disposed) return
	observing = true
	try {
		const panel = normalizeFormulaPanel(await FormulaSamplerObserveOwned(sampler.value.sessionToken))
		currentPanel.value = panel
		if (recording.value) {
			const previous = transitionSamples.value.at(-1)
			const signature = `${panel.hp}/${panel.attack}/${panel.critRate}/${panel.stunPower}`
			const previousSignature = previous ? `${previous.hp}/${previous.attack}/${previous.critRate}/${previous.stunPower}` : ''
			if (signature !== previousSignature) transitionSamples.value = [...transitionSamples.value, panel]
		}
	} catch (error) {
		if (!disposed) announce(errorText(error), 'danger')
	} finally {
		observing = false
	}
}

async function startChangeRecording() {
	if (!connected.value || busy.value || recording.value) return
	busy.value = true
	try {
		const started = await FormulaSamplerStartChangeRecordingOwned(sampler.value.sessionToken)
		const before = normalizeFormulaPanel(started.before)
		currentPanel.value = before
		transitionSamples.value = [before]
		changeAnalysis.value = null
		recording.value = true
		announce(language.value === 'en' ? 'Change recording started. Change exactly one item.' : '变化记录已开始；现在只切换一个项目。', 'ok')
	} catch (error) {
		announce(errorText(error), 'danger')
	} finally {
		busy.value = false
	}
}

async function stopChangeRecording() {
	if (!connected.value || busy.value || !recording.value) return
	busy.value = true
	try {
		const analysis = await FormulaSamplerStopChangeRecordingOwned(sampler.value.sessionToken)
		changeAnalysis.value = Object.freeze({ ...analysis, before: normalizeFormulaPanel(analysis.before), after: normalizeFormulaPanel(analysis.after) })
		currentPanel.value = changeAnalysis.value.after
		recording.value = false
		announce(analysis.evidenceLevel === 'negative_observation'
			? (language.value === 'en' ? 'Stopped: negative observation, not proof of no effect.' : '已停止：这是负观察，不代表无效果。')
			: (language.value === 'en' ? 'Stopped and produced transition candidates.' : '已停止并生成变化候选。'), 'ok')
	} catch (error) {
		announce(errorText(error), 'danger')
	} finally {
		busy.value = false
	}
}

function copy(key) {
  const texts = {
    ready: ['选择角色后连接游戏；页面只读最终面板与角色状态对象，不安装 Hook。', 'Choose a character and attach; this page reads the final panel and character status object without installing hooks.'],
    strict: ['严格只读', 'Strict read-only'],
    attach: ['连接只读采样器', 'Attach read-only sampler'],
    close: ['安全断开', 'Close safely'],
    capture: ['采集当前阶段', 'Capture current phase'],
    export: ['导出脱敏证据包', 'Export redacted evidence'],
    complete: ['四阶段已闭环，可以导出给开发者分析。', 'The four-phase loop is complete and ready to export.'],
    disconnected: ['采样器已断开。', 'Sampler closed.'],
    oneChange: ['每轮只改变一个可逆项目；不要同时换武器、因子、专精或召唤石。', 'Change one reversible item per run; never change weapon, sigil, mastery, or summon together.'],
    panel: ['角色最终面板', 'Final character panel'],
    noSample: ['尚未采集', 'Not captured'],
    character: ['目标角色', 'Target character'],
  }
  const pair = texts[key]
  return pair?.[language.value === 'en' ? 1 : 0] ?? key
}

function characterName(item) {
  return language.value === 'en' ? item.en : item.zh
}

function phaseCopyForRun(phase) {
  if (!controlMode.value) return formulaPhaseCopy(phase, language.value)
  const copy = {
    A1: ['A1 · 空白基准', 'A1 · Control baseline'],
    B1: ['B1 · 不变复测', 'B1 · Unchanged repeat'],
    A2: ['A2 · 不变复测', 'A2 · Unchanged repeat'],
    B2: ['B2 · 最终复测', 'B2 · Final unchanged repeat'],
  }[phase]
  return {
    title: copy[language.value === 'en' ? 1 : 0],
    instruction: language.value === 'en'
      ? 'Do not change any equipment or state; wait for the same stable panel, then capture.'
      : '不要改变任何装备、专精或状态；等待同一面板稳定后直接采集。',
  }
}

function errorText(error) {
  return (error instanceof Error ? error.message : String(error || '')).replace(/^Error:\s*/i, '')
}

function announce(text, nextTone = 'info') {
  message.value = text
  tone.value = nextTone
  emit('status', text, nextTone === 'danger' ? 'error' : nextTone === 'ok' ? 'success' : nextTone)
}

async function attach() {
  if (busy.value || connected.value) return
  busy.value = true
  lastExportPath.value = ''
  try {
    const status = normalizeFormulaSamplerStatus(await FormulaSamplerAttach(selectedHash.value, selectedExperimentType.value))
    if (disposed) {
      if (status.sessionToken) await FormulaSamplerCloseOwned(status.sessionToken).catch(() => {})
      return
    }
    sampler.value = status
		await observeCurrent()
    announce(language.value === 'en' ? 'Read-only sampler attached.' : '只读采样器已连接。', 'ok')
  } catch (error) {
    try {
      const status = normalizeFormulaSamplerStatus(await FormulaSamplerStatus())
      sampler.value = status.sessionToken === sampler.value.sessionToken
        ? status
        : normalizeFormulaSamplerStatus(null)
    } catch {
      sampler.value = normalizeFormulaSamplerStatus(null)
    }
    announce(errorText(error), 'danger')
  } finally {
    busy.value = false
  }
}

async function capture() {
  const phase = nextPhase.value
  if (!phase || busy.value || !connected.value) return
  busy.value = true
  try {
    const event = await FormulaSamplerCaptureOwned(sampler.value.sessionToken, phase)
    sampler.value = normalizeFormulaSamplerStatus({
      connected: true,
      sessionToken: sampler.value.sessionToken,
		experimentType: sampler.value.experimentType,
      events: [...sampler.value.events, event],
    })
    announce(complete.value ? copy('complete') : `${phase} ${language.value === 'en' ? 'captured.' : '采集完成。'}`, 'ok')
  } catch (error) {
    try {
      const status = normalizeFormulaSamplerStatus(await FormulaSamplerStatus())
      sampler.value = status.sessionToken === sampler.value.sessionToken
        ? status
        : normalizeFormulaSamplerStatus(null)
    } catch {
      sampler.value = normalizeFormulaSamplerStatus(null)
    }
    announce(errorText(error), 'danger')
  } finally {
    busy.value = false
  }
}

async function exportBundle() {
  if (!complete.value || busy.value) return
  busy.value = true
  lastExportPath.value = ''
  try {
    const path = await FormulaSamplerExport(sampler.value.sessionToken)
    if (path) {
      lastExportPath.value = path
      announce(language.value === 'en' ? `Redacted evidence exported to ${path}` : `脱敏证据包已导出到 ${path}`, 'ok')
    }
  } catch (error) {
    announce(errorText(error), 'danger')
  } finally {
    busy.value = false
  }
}

async function close() {
  if (busy.value) return
  busy.value = true
  try {
    await FormulaSamplerCloseOwned(sampler.value.sessionToken)
    sampler.value = normalizeFormulaSamplerStatus(null)
		currentPanel.value = null
		recording.value = false
		transitionSamples.value = []
		changeAnalysis.value = null
    lastExportPath.value = ''
    announce(copy('disconnected'), 'info')
  } catch (error) {
    announce(errorText(error), 'danger')
  } finally {
    busy.value = false
  }
}

function phaseState(phase) {
  const index = FORMULA_PHASES.indexOf(phase)
  if (sampler.value.events[index]) return 'done'
  if (phase === nextPhase.value) return 'current'
  return 'waiting'
}

function phasePanel(phase) {
  return sampler.value.events.find(event => event.phase === phase)?.panel || null
}

function formatStat(value, digits = 0) {
  const number = Number(value)
  return Number.isFinite(number) ? number.toLocaleString(undefined, { maximumFractionDigits: digits }) : '—'
}

onMounted(() => {
	void refreshRuntimeObjects()
	observeTimer = window.setInterval(() => { void observeCurrent() }, 750)
})

onBeforeUnmount(() => {
  disposed = true
	if (observeTimer != null) window.clearInterval(observeTimer)
  if (sampler.value.sessionToken) void FormulaSamplerCloseOwned(sampler.value.sessionToken).catch(() => {})
})
</script>

<template>
  <section class="formula-sampler ui-page ui-page-stack is-fluid" data-page="formula-sampler" :aria-busy="busy">
    <section class="sampler-toolbar ui-card ui-panel is-compact">
      <label class="character-picker">
        <span>{{ copy('character') }}</span>
        <span class="character-select-shell">
          <img :src="characterAssetIcon(selectedCharacter.hash)" alt="">
          <select v-model="selectedHash" class="ui-select" :disabled="connected || busy">
            <option v-for="character in characters" :key="character.hash" :value="character.hash">
              {{ characterName(character) }} · {{ character.hash }}
            </option>
          </select>
        </span>
      </label>
      <label class="experiment-picker">
        <span>{{ language === 'en' ? 'One changed variable' : '本轮唯一变更类型' }}</span>
        <select v-model="selectedExperimentType" class="ui-select" :disabled="connected || busy">
          <option v-for="item in experimentTypes" :key="item.value" :value="item.value">
            {{ language === 'en' ? item.en : item.zh }}
          </option>
        </select>
      </label>
      <div class="toolbar-actions">
        <span class="readonly-seal"><i>◉</i>{{ copy('strict') }}</span>
        <button v-if="!connected" class="ui-btn is-primary" :disabled="busy" @click="attach">{{ copy('attach') }}</button>
        <button v-else class="ui-btn is-secondary" :disabled="busy" @click="close">{{ copy('close') }}</button>
      </div>
    </section>

    <div class="sampler-message ui-notice" :class="`is-${tone}`" aria-live="polite">{{ message }}</div>
    <div v-if="lastExportPath" class="export-path ui-card" aria-live="polite">
      <b>{{ language === 'en' ? 'Saved to' : '保存路径' }}</b><span>{{ lastExportPath }}</span>
    </div>

		<section v-if="currentPanel" class="live-panel ui-card ui-panel" aria-live="polite">
			<header class="live-panel-heading">
				<div><span>LIVE · 3× STABLE</span><h2>{{ language === 'en' ? 'Final game panel' : '游戏最终面板连续观测' }}</h2></div>
				<code>{{ currentPanel.characterHash }} ⇄ {{ currentPanel.runtimeId }}</code>
			</header>
			<div class="live-stat-grid">
				<article><span>HP</span><b>{{ formatStat(currentPanel.hp) }}</b><small>{{ fieldEvidence(currentPanel.hpField) }}</small></article>
				<article><span>{{ language === 'en' ? 'ATK' : '攻击力' }}</span><b>{{ formatStat(currentPanel.attack) }}</b><small>{{ fieldEvidence(currentPanel.attackField) }}</small></article>
				<article><span>{{ language === 'en' ? 'CRIT' : '暴击率' }}</span><b>{{ formatStat(currentPanel.critRate, 2) }}%</b><small>{{ fieldEvidence(currentPanel.critField) }}</small></article>
				<article><span>{{ language === 'en' ? 'STUN' : '昏厥值' }}</span><b>{{ formatStat(currentPanel.stunPower, 2) }}</b><small>{{ fieldEvidence(currentPanel.stunField) }} · raw {{ currentPanel.rawStunPower }}</small></article>
			</div>
			<p class="identity-evidence">{{ selectedRuntimeObject?.directoryName || selectedCharacter.zh }} · directory {{ currentPanel.characterHash }} · runtime {{ currentPanel.runtimeId }} · {{ currentPanel.identitySource }}</p>
		</section>

		<section class="change-recorder ui-card ui-panel is-compact">
			<header><div><span>AUTO TRANSITION</span><h3>{{ language === 'en' ? 'Record one change' : '自动记录一次变化' }}</h3></div>
				<div class="change-actions">
					<button v-if="!recording" class="ui-btn is-primary" :disabled="!connected || busy" @click="startChangeRecording">{{ language === 'en' ? 'Start recording' : '开始记录变化' }}</button>
					<button v-else class="ui-btn is-danger" :disabled="busy" @click="stopChangeRecording">{{ language === 'en' ? 'Stop & analyse' : '停止并分析' }}</button>
				</div>
			</header>
			<p>{{ recording ? (language === 'en' ? 'Recording stable endpoints and transition states; change one item only.' : '正在记录稳定前值、过渡状态与稳定后值；只改变一个项目。') : (language === 'en' ? 'This produces candidates from one stable transition. Strict A/B/A/B remains below for formula verification.' : '单次稳定变化只生成候选；下方 A/B/A/B 仍用于严格公式验证。') }}</p>
			<div v-if="changeAnalysis" class="change-result">
				<b>{{ changeAnalysis.evidenceLevel }}</b>
				<span>ΔHP {{ changeAnalysis.panelDelta?.hp ?? 0 }} · ΔATK {{ changeAnalysis.panelDelta?.attack ?? 0 }} · ΔCRIT {{ changeAnalysis.panelDelta?.critRate ?? 0 }} · ΔSTUN {{ changeAnalysis.panelDelta?.stunPower ?? 0 }}</span>
				<small>{{ changeAnalysis.candidates?.length || 0 }} {{ language === 'en' ? 'relative scalar candidates' : '个相对标量候选' }} · {{ transitionSamples.length }} {{ language === 'en' ? 'observed panel states' : '个已观测面板状态' }}</small>
				<em v-if="changeAnalysis.negativeObservation">{{ changeAnalysis.negativeObservation }}</em>
			</div>
		</section>

		<details class="runtime-objects ui-card ui-panel is-compact">
			<summary>{{ language === 'en' ? 'Runtime object enumeration' : '运行时对象枚举' }} · {{ runtimeCatalog?.objects?.length || 0 }}</summary>
			<div class="runtime-object-copy">
				<p>{{ runtimeCatalogMessage || (language === 'en' ? 'Refresh to enumerate the guarded manager.' : '刷新以枚举已通过版本守卫的 manager。') }}</p>
				<button class="ui-btn is-secondary" :disabled="busy" @click="refreshRuntimeObjects">{{ language === 'en' ? 'Refresh objects' : '刷新对象' }}</button>
			</div>
			<div v-if="runtimeCatalog?.objects?.length" class="runtime-object-table">
				<div v-for="item in runtimeCatalog.objects" :key="item.runtimeId" :class="{ 'is-selected': item.directoryHash === selectedHash }">
					<b>{{ item.directoryName || (language === 'en' ? 'Unmapped' : '未映射') }}</b><code>{{ item.directoryHash }} / {{ item.runtimeId }}</code>
					<span>map {{ item.mapKey }} · hash {{ item.candidateObjectHash }} · ready {{ item.ready }} · eligibility {{ item.eligibility }}</span>
					<small>{{ item.evidenceLevel }}<template v-if="item.negativeObservation"> · {{ item.negativeObservation }}</template></small>
				</div>
			</div>
		</details>

    <details class="sampling-scope ui-card ui-panel is-compact">
      <summary>{{ language === 'en' ? 'Advanced sampling scope' : '高级采样范围' }}</summary>
      <p>{{ language === 'en'
        ? 'Each phase checks six bit-exact final-panel reads around three stable reads of a 24 KiB character-status window. Mastery, defense, and damage-cap runs may keep all four known panel values unchanged; such exports are labelled as candidate scans or negative observations, never verified formulas.'
        : '每个阶段会在角色状态对象前后各核对三次位精确最终面板，并对 24 KiB 状态窗口连续只读三次。专精、防御力与伤害上限允许已知四项面板不变；导出会如实标成候选扫描或负观察，绝不冒充已验证公式。' }}</p>
    </details>

    <section class="workflow-card ui-card ui-panel">
      <header class="workflow-heading">
        <div><span>REVERSIBLE EVIDENCE LOOP</span><h2>A / B / A / B</h2></div>
        <p>{{ controlMode ? (language === 'en' ? 'Control run: change nothing in all four phases.' : '空白对照轮：四个阶段都不要改变任何项目。') : copy('oneChange') }}</p>
      </header>

      <div class="phase-grid">
        <article v-for="phase in FORMULA_PHASES" :key="phase" class="phase-card" :class="`is-${phaseState(phase)}`">
          <header><b>{{ phaseLabels[phase] }}</b><span>{{ phaseCopyForRun(phase).title }}</span></header>
          <p>{{ phaseCopyForRun(phase).instruction }}</p>
          <dl v-if="phasePanel(phase)" class="panel-values">
            <div><dt>HP</dt><dd>{{ formatStat(phasePanel(phase).hp) }}</dd></div>
            <div><dt>{{ language === 'en' ? 'ATK' : '攻击' }}</dt><dd>{{ formatStat(phasePanel(phase).attack) }}</dd></div>
            <div><dt>{{ language === 'en' ? 'CRIT' : '暴击率' }}</dt><dd>{{ formatStat(phasePanel(phase).critRate, 2) }}%</dd></div>
            <div><dt>{{ language === 'en' ? 'STUN' : '昏厥值' }}</dt><dd>{{ formatStat(phasePanel(phase).stunPower, 2) }}</dd></div>
          </dl>
          <div v-else class="phase-empty">{{ copy('noSample') }}</div>
        </article>
      </div>

      <footer class="sampler-actions">
        <div class="next-copy">
          <span>{{ complete ? copy('complete') : phaseCopyForRun(nextPhase || 'B2').title }}</span>
          <small>{{ complete ? copy('export') : phaseCopyForRun(nextPhase || 'B2').instruction }}</small>
        </div>
        <button class="ui-btn is-primary" :disabled="!connected || busy || complete" @click="capture">
          {{ copy('capture') }}<b v-if="nextPhase">{{ nextPhase }}</b>
        </button>
        <button class="ui-btn is-secondary" :disabled="!complete || busy" @click="exportBundle">{{ copy('export') }}</button>
      </footer>
    </section>
  </section>
</template>

<style scoped>
.formula-sampler {
  min-width:0;
  container-name:formula-sampler;
  container-type:inline-size;
  gap:var(--space-3);
}
.sampler-toolbar { display:flex; align-items:flex-end; justify-content:space-between; gap:var(--space-4); }
.character-picker { min-width:0; flex:1; display:grid; gap:var(--space-2); color:var(--text-secondary); font-size:var(--fs-xs); font-weight:var(--fw-bold); }
.experiment-picker { min-width:180px; display:grid; gap:var(--space-2); color:var(--text-secondary); font-size:var(--fs-xs); font-weight:var(--fw-bold); }
.character-select-shell { min-width:0; display:grid; grid-template-columns:38px minmax(0,1fr); align-items:center; gap:var(--space-2); }
.character-select-shell img { width:38px; height:38px; object-fit:contain; border:1px solid var(--border-default); border-radius:var(--radius-sm); background:var(--surface-card-pop); }
.character-select-shell select { width:100%; min-width:0; }
.toolbar-actions { display:flex; flex-wrap:wrap; align-items:center; justify-content:flex-end; gap:var(--space-2); }
.readonly-seal { height:36px; display:inline-flex; align-items:center; gap:var(--space-2); padding:0 var(--space-3); border:1px solid var(--border-default); border-radius:999px; color:var(--accent-hover); background:var(--accent-soft); font-size:var(--fs-xs); font-weight:var(--fw-bold); }
.readonly-seal i { font-style:normal; }
.sampler-message { min-height:40px; display:flex; align-items:center; }
.export-path { min-width:0; display:grid; grid-template-columns:auto minmax(0,1fr); gap:var(--space-3); align-items:center; padding:var(--space-3) var(--space-4); }
.export-path b { color:var(--text-primary); font-size:var(--fs-xs); }
.export-path span { min-width:0; overflow-wrap:anywhere; color:var(--text-secondary); font-family:var(--font-mono); font-size:var(--fs-xs); }
.live-panel { display:grid; gap:var(--space-4); }
.live-panel-heading,.change-recorder > header { display:flex; align-items:center; justify-content:space-between; gap:var(--space-4); }
.live-panel-heading span,.change-recorder > header span { color:var(--accent); font-size:10px; font-weight:var(--fw-bold); letter-spacing:.12em; }
.live-panel-heading h2,.change-recorder h3 { margin:2px 0 0; color:var(--text-primary); font-size:var(--fs-lg); }
.live-panel-heading code { color:var(--text-secondary); font-size:var(--fs-xs); }
.live-stat-grid { display:grid; grid-template-columns:repeat(4,minmax(0,1fr)); gap:var(--space-3); }
.live-stat-grid article { min-width:0; display:grid; gap:4px; padding:var(--space-3); border:1px solid var(--border-default); border-radius:var(--radius-md); background:var(--surface-sunken); }
.live-stat-grid span { color:var(--text-muted); font-size:10px; font-weight:var(--fw-bold); }
.live-stat-grid b { color:var(--text-primary); font-size:clamp(20px,2.4cqi,30px); font-variant-numeric:tabular-nums; }
.live-stat-grid small,.identity-evidence { color:var(--text-secondary); font-family:var(--font-mono); font-size:10px; overflow-wrap:anywhere; }
.identity-evidence { margin:0; }
.change-recorder { display:grid; gap:var(--space-3); }
.change-recorder p { margin:0; color:var(--text-secondary); font-size:var(--fs-xs); }
.change-actions { display:flex; gap:var(--space-2); }
.change-result { display:grid; grid-template-columns:auto 1fr; gap:4px var(--space-3); padding:var(--space-3); border:1px solid var(--accent-border); border-radius:var(--radius-sm); background:var(--accent-soft); }
.change-result b { color:var(--accent-hover); font-family:var(--font-mono); font-size:var(--fs-xs); }
.change-result span,.change-result small,.change-result em { color:var(--text-secondary); font-size:var(--fs-xs); }
.change-result small,.change-result em { grid-column:1/-1; }
.runtime-objects summary { cursor:pointer; color:var(--text-primary); font-weight:var(--fw-bold); }
.runtime-object-copy { display:flex; align-items:center; justify-content:space-between; gap:var(--space-3); margin-top:var(--space-3); }
.runtime-object-copy p { margin:0; color:var(--text-secondary); font-size:var(--fs-xs); }
.runtime-object-table { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:var(--space-2); max-height:360px; margin-top:var(--space-3); overflow:auto; }
.runtime-object-table > div { min-width:0; display:grid; gap:2px; padding:var(--space-2); border:1px solid var(--border-soft); border-radius:var(--radius-sm); background:var(--surface-sunken); }
.runtime-object-table > div.is-selected { border-color:var(--accent-border); box-shadow:inset 3px 0 0 var(--selected-bar); }
.runtime-object-table b,.runtime-object-table code,.runtime-object-table span,.runtime-object-table small { overflow-wrap:anywhere; font-size:10px; }
.runtime-object-table span,.runtime-object-table small { color:var(--text-secondary); }
.sampling-scope { padding-block:var(--space-3); }
.sampling-scope summary { color:var(--text-primary); font-size:var(--fs-sm); font-weight:var(--fw-bold); cursor:pointer; }
.sampling-scope p { margin:var(--space-3) 0 0; color:var(--text-secondary); font-size:var(--fs-xs); line-height:var(--lh-normal); }
.workflow-card { min-width:0; background:color-mix(in srgb,var(--surface-card) 94%,transparent); }
.workflow-heading { display:flex; align-items:flex-end; justify-content:space-between; gap:var(--space-5); padding-bottom:var(--space-4); border-bottom:1px solid var(--border-default); }
.workflow-heading > div span { color:var(--accent); font-size:var(--fs-xs); font-weight:var(--fw-bold); letter-spacing:.12em; }
.workflow-heading h2 { margin:var(--space-1) 0 0; color:var(--text-primary); font-family:var(--font-display); font-size:clamp(24px,3cqi,34px); line-height:1; }
.workflow-heading p { max-width:560px; margin:0; color:var(--text-secondary); font-size:var(--fs-sm); line-height:var(--lh-normal); }
.phase-grid { display:grid; grid-template-columns:repeat(4,minmax(0,1fr)); gap:var(--space-3); margin-top:var(--space-4); }
.phase-card { min-width:0; min-height:230px; display:flex; flex-direction:column; gap:var(--space-3); padding:var(--space-4); border:1px solid var(--border-default); border-radius:var(--radius-md); background:color-mix(in srgb,var(--surface-card-pop) 78%,transparent); transition:var(--transition-control); }
.phase-card.is-current { border-color:var(--accent-border); box-shadow:inset 3px 0 0 var(--selected-bar),var(--shadow-1); }
.phase-card.is-done { border-color:color-mix(in srgb,var(--success) 42%,var(--border-default)); background:color-mix(in srgb,var(--success-soft) 38%,var(--surface-card-pop)); }
.phase-card > header { display:flex; align-items:center; gap:var(--space-2); }
.phase-card > header b { width:34px; height:28px; flex:0 0 34px; display:grid; place-items:center; border-radius:var(--radius-sm); color:var(--surface-card-pop); background:var(--accent); font-size:var(--fs-xs); }
.phase-card > header span { min-width:0; overflow:hidden; color:var(--text-primary); font-size:var(--fs-sm); font-weight:var(--fw-bold); text-overflow:ellipsis; white-space:nowrap; }
.phase-card > p { min-height:60px; margin:0; color:var(--text-secondary); font-size:var(--fs-xs); line-height:var(--lh-normal); }
.panel-values { display:grid; grid-template-columns:repeat(2,minmax(0,1fr)); gap:var(--space-2); margin:auto 0 0; }
.panel-values > div { min-width:0; padding:var(--space-2); border:1px solid var(--border-soft); border-radius:var(--radius-sm); background:var(--surface-sunken); }
.panel-values dt { color:var(--text-muted); font-size:10px; font-weight:var(--fw-bold); }
.panel-values dd { margin:2px 0 0; overflow:hidden; color:var(--text-primary); font-size:var(--fs-sm); font-variant-numeric:tabular-nums; font-weight:var(--fw-bold); text-overflow:ellipsis; }
.phase-empty { margin:auto 0 0; padding:var(--space-3); border:1px dashed var(--border-default); border-radius:var(--radius-sm); color:var(--text-muted); font-size:var(--fs-xs); text-align:center; }
.sampler-actions { display:grid; grid-template-columns:minmax(0,1fr) auto auto; align-items:center; gap:var(--space-3); margin-top:var(--space-4); padding-top:var(--space-4); border-top:1px solid var(--border-default); }
.next-copy { min-width:0; }
.next-copy span,.next-copy small { display:block; overflow:hidden; text-overflow:ellipsis; }
.next-copy span { color:var(--text-primary); font-size:var(--fs-sm); font-weight:var(--fw-bold); white-space:nowrap; }
.next-copy small { margin-top:2px; color:var(--text-secondary); font-size:var(--fs-xs); line-height:var(--lh-normal); }
.sampler-actions button { white-space:nowrap; }
.sampler-actions button b { margin-left:var(--space-2); }

@container formula-sampler (max-width:1000px) {
  .phase-grid { grid-template-columns:repeat(2,minmax(0,1fr)); }
  .phase-card { min-height:200px; }
}
@container formula-sampler (max-width:620px) {
  .sampler-toolbar,.workflow-heading { align-items:stretch; flex-direction:column; }
  .toolbar-actions { justify-content:stretch; }
  .toolbar-actions > * { flex:1; justify-content:center; }
  .phase-grid { grid-template-columns:minmax(0,1fr); }
  .phase-card { min-height:0; }
  .phase-card > p { min-height:0; }
  .sampler-actions { grid-template-columns:minmax(0,1fr); }
  .sampler-actions button { width:100%; }
	.live-panel-heading,.change-recorder > header,.runtime-object-copy { align-items:stretch; flex-direction:column; }
	.live-stat-grid,.runtime-object-table { grid-template-columns:minmax(0,1fr); }
}
</style>
