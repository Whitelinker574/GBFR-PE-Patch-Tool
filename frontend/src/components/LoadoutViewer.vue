<script setup>
import { computed, onMounted, ref } from 'vue'
import { FindSaveFiles, LoadoutList, SelectProgressionSave } from '../../wailsjs/go/main/App'

const emit = defineEmits(['status'])

const slots = ref([])
const savePath = ref('')
const loading = ref(false)
const groups = ref([])
const selectedChara = ref('')
const expanded = ref(new Set())

const CAT_LABELS = { SB_ATK: '真谛（攻击盘）', SB_DEF: '觉醒（防御盘）', SB_LIMIT: '秘义（界限盘）' }

function catLabel(cat) { return CAT_LABELS[cat] || '基础盘' }

// 专精激活等级：点满 10 个→1 级；20 个→3 级；30 个→5 级（skillboard_group 阈值）
function treeLevel(count) {
  if (count >= 30) return 5
  if (count >= 20) return 3
  if (count >= 10) return 1
  return 0
}

const currentGroup = computed(() => groups.value.find(g => g.charaName === selectedChara.value) || null)
const presetCount = computed(() => {
  let n = 0
  for (const g of groups.value) for (const lo of g.loadouts) if (!lo.isParty) n++
  return n
})

function masterySummary(lo) {
  const byCat = {}
  for (const m of lo.mastery || []) {
    const c = m.cat || ''
    byCat[c] = (byCat[c] || 0) + 1
  }
  return Object.entries(byCat).map(([cat, count]) => ({ cat, count, level: treeLevel(count) }))
}

function masteryGrouped(lo) {
  const byCat = new Map()
  for (const m of lo.mastery || []) {
    const c = m.cat || ''
    if (!byCat.has(c)) byCat.set(c, [])
    byCat.get(c).push(m)
  }
  return [...byCat.entries()].map(([cat, nodes]) => ({ cat, nodes }))
}

function toggle(lo) {
  const next = new Set(expanded.value)
  if (next.has(lo.unitId)) next.delete(lo.unitId)
  else next.add(lo.unitId)
  expanded.value = next
}

async function load(path) {
  if (!path) return
  loading.value = true
  try {
    const result = await LoadoutList(path)
    groups.value = result || []
    savePath.value = path
    if (!groups.value.find(g => g.charaName === selectedChara.value)) {
      selectedChara.value = groups.value[0]?.charaName || ''
    }
    emit('status', `已读取 ${groups.value.length} 个角色、${presetCount.value} 套配装预设`, 'success')
  } catch (err) {
    emit('status', String(err), 'error')
  } finally {
    loading.value = false
  }
}

async function browse() {
  try {
    const path = await SelectProgressionSave()
    if (path) await load(path)
  } catch (err) {
    emit('status', String(err), 'error')
  }
}

onMounted(async () => {
  try {
    slots.value = (await FindSaveFiles()) || []
  } catch { /* 找不到默认存档目录时静默，仍可手动浏览 */ }
})
</script>

<template>
  <div class="loadout-viewer">
    <section class="section">
      <div class="section-title"><span>选择存档</span><small>读取游戏内保存的配装预设（每角色 15 槽）</small></div>
      <div class="save-row">
        <button v-for="slot in slots" :key="slot.path" class="action" :class="{ primary: savePath === slot.path }" :disabled="loading" @click="load(slot.path)">存档位 {{ slot.index }}</button>
        <button class="action" :disabled="loading" @click="browse">浏览…</button>
        <button class="action" :disabled="loading || !savePath" @click="load(savePath)">刷新</button>
      </div>
      <div v-if="savePath" class="path-line" :title="savePath">{{ savePath }}</div>
    </section>

    <section v-if="groups.length" class="section">
      <div class="section-title"><span>角色</span><small>{{ groups.length }} 个角色 · {{ presetCount }} 套预设</small></div>
      <div class="chara-row">
        <button v-for="g in groups" :key="g.charaHash" class="chara-chip" :class="{ on: selectedChara === g.charaName }" @click="selectedChara = g.charaName">
          {{ g.charaName }}<i>{{ g.loadouts.filter(l => !l.isParty).length }}</i>
        </button>
      </div>
    </section>

    <section v-if="currentGroup" class="section">
      <div class="section-title"><span>{{ currentGroup.charaName }} 的配装</span><small>点击卡片展开因子与专精明细</small></div>
      <article v-for="lo in currentGroup.loadouts" :key="lo.unitId" class="loadout-card" :class="{ open: expanded.has(lo.unitId) }">
        <header @click="toggle(lo)">
          <b v-if="!lo.isParty">槽{{ String(lo.slot).padStart(2, '0') }}</b>
          <b v-else class="party-tag">队伍{{ lo.slot }}</b>
          <strong>{{ lo.name || (lo.isParty ? '当前实时配装' : '(未命名)') }}</strong>
          <span class="wep">{{ lo.weaponName || lo.weaponHash }}</span>
          <em>{{ (lo.sigils || []).length }}因子 · {{ (lo.mastery || []).length }}专精</em>
        </header>
        <div class="skills-line">
          <span>技能：</span>
          <i v-for="s in lo.skills" :key="s.hash">{{ s.name || s.hash }}</i>
        </div>
        <div class="mastery-summary">
          <span>专精：</span>
          <i v-for="t in masterySummary(lo)" :key="t.cat">{{ catLabel(t.cat) }} {{ t.count }}点<b v-if="t.level"> {{ t.level }}级</b></i>
          <i v-if="!(lo.mastery || []).length" class="dim">未保存</i>
        </div>
        <div v-if="expanded.has(lo.unitId)" class="detail">
          <div class="detail-block">
            <h4>因子（{{ (lo.sigils || []).length }}）</h4>
            <ul>
              <li v-for="s in lo.sigils" :key="s.slotId">
                {{ s.name || ('未收录 ' + s.hash) }} <small>Lv{{ s.level }}</small>
                <small v-if="s.missing" class="warn">⚠ 原因子已不存在</small>
              </li>
            </ul>
          </div>
          <div v-for="grp in masteryGrouped(lo)" :key="grp.cat" class="detail-block">
            <h4>{{ catLabel(grp.cat) }}（{{ grp.nodes.length }}点 · {{ treeLevel(grp.nodes.length) }}级）</h4>
            <ul>
              <li v-for="m in grp.nodes" :key="m.hash">
                <b v-if="m.name">★{{ m.name }} — </b>{{ m.desc || ('??? ' + m.hash) }}
              </li>
            </ul>
          </div>
        </div>
      </article>
    </section>

    <section v-else-if="!loading && savePath" class="section">
      <p class="empty">该存档中没有已保存的配装预设。</p>
    </section>
  </div>
</template>

<style scoped>
.loadout-viewer { display:flex; flex-direction:column; gap:14px; }
.section { padding:16px 18px; border:1px solid rgba(255,255,255,.08); border-radius:8px; background:rgba(255,255,255,.04); display:flex; flex-direction:column; gap:11px; }
.section-title { display:flex; align-items:baseline; gap:9px; font-size:.78rem; font-weight:700; }
.section-title small { font-weight:600; opacity:.65; }
.save-row { display:flex; flex-wrap:wrap; gap:7px; }
.action { min-height:30px; padding:0 12px; border:1px solid rgba(255,255,255,.16); border-radius:6px; background:rgba(255,255,255,.06); cursor:pointer; font-size:.78rem; }
.action.primary { border-color:rgba(103,232,249,.4); }
.action:disabled { opacity:.45; cursor:not-allowed; }
.path-line { font-size:.68rem; opacity:.6; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.chara-row { display:flex; flex-wrap:wrap; gap:6px; }
.chara-chip { min-height:29px; padding:0 11px; border:1px solid rgba(255,255,255,.14); border-radius:16px; background:rgba(255,255,255,.05); cursor:pointer; font-size:.76rem; }
.chara-chip.on { border-color:rgba(103,232,249,.45); }
.chara-chip i { font-style:normal; margin-left:5px; opacity:.6; font-size:.68rem; }
.loadout-card { border:1px solid rgba(255,255,255,.1); border-radius:7px; padding:10px 13px; display:flex; flex-direction:column; gap:7px; }
.loadout-card header { display:flex; align-items:center; gap:9px; cursor:pointer; min-height:24px; }
.loadout-card header b { font-size:.72rem; opacity:.75; flex:0 0 auto; }
.loadout-card header b.party-tag { color:#4ade80; }
.loadout-card header strong { font-size:.82rem; }
.loadout-card header .wep { font-size:.74rem; opacity:.8; margin-left:auto; }
.loadout-card header em { font-style:normal; font-size:.68rem; opacity:.55; flex:0 0 auto; }
.skills-line, .mastery-summary { display:flex; flex-wrap:wrap; gap:6px; align-items:center; font-size:.72rem; }
.skills-line span, .mastery-summary span { opacity:.6; }
.skills-line i, .mastery-summary i { font-style:normal; padding:1px 8px; border:1px solid rgba(255,255,255,.12); border-radius:12px; }
.mastery-summary i b { font-weight:800; }
.mastery-summary i.dim { opacity:.5; border-style:dashed; }
.detail { display:flex; flex-direction:column; gap:9px; padding-top:5px; border-top:1px dashed rgba(255,255,255,.12); }
.detail-block h4 { margin:0 0 5px; font-size:.74rem; }
.detail-block ul { margin:0; padding-left:17px; display:flex; flex-direction:column; gap:2px; }
.detail-block li { font-size:.72rem; line-height:1.5; }
.detail-block li small { opacity:.65; margin-left:5px; }
.detail-block li small.warn { color:#f0a05a; }
.empty { margin:0; font-size:.76rem; opacity:.6; }
</style>
