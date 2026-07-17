<script setup>
import { computed, ref, watch } from 'vue'
import { LoadoutApply, LoadoutEditContext, LoadoutSimulate, MasteryNodePool } from '../../wailsjs/go/main/App'
import ConfirmDialog from './ConfirmDialog.vue'

const props = defineProps({
  savePath: { type: String, default: '' },
  charaHash: { type: String, default: '' },
  charaName: { type: String, default: '' },
})
const emit = defineEmits(['status', 'reload'])

const confirmDialog = ref(null)
const ctx = ref(null)
const loading = ref(false)
const applying = ref(false)

const targetSlot = ref(0)          // 目标预设槽 unitId
const op = ref('write')            // write | clone | clear
const form = ref({ name: '', weaponSlotId: 0, sigilSlotIds: [], skillHashes: [], masterySource: 0 })
const cloneFrom = ref(0)

// 名称字节数（后端上限 63）
function utf8Bytes(s) { return new TextEncoder().encode(s || '').length }
const nameBytes = computed(() => utf8Bytes(form.value.name))
const nameTooLong = computed(() => nameBytes.value > 63)

const slots = computed(() => ctx.value?.slots || [])
const occupiedSlots = computed(() => slots.value.filter(s => s.occupied))
const masterySources = computed(() => ctx.value?.masterySources || [])

// 专精：复制现有 or 自由配置（4 档 10/10/10/20）
const masteryMode = ref('copy')     // copy | free
const masteryPool = ref([])         // [{rank,label,cap,nodes}]
const masteryPick = ref({})         // { R1:[hash...], R2:[], R3:[], EX:[] }
const masteryRankTab = ref('R1')
const CAT_ABBR = { SB_ATK: '攻', SB_DEF: '防', SB_LIMIT: '界' }
function catAbbr(cat) { return CAT_ABBR[cat] || '基' }
const activeRankPool = computed(() => masteryPool.value.find(p => p.rank === masteryRankTab.value) || null)
function rankPicked(rank) { return (masteryPick.value[rank] || []).length }
const masteryTotal = computed(() => Object.values(masteryPick.value).reduce((n, a) => n + a.length, 0))
function toggleNode(rank, hash, cap) {
  const arr = masteryPick.value[rank] || (masteryPick.value[rank] = [])
  const i = arr.indexOf(hash)
  if (i >= 0) arr.splice(i, 1)
  else if (arr.length < cap) arr.push(hash)
}
async function loadMasteryPool() {
  masteryPool.value = []
  masteryPick.value = { R1: [], R2: [], R3: [], EX: [] }
  if (!ctx.value?.ownerCode) return
  try { masteryPool.value = (await MasteryNodePool(ctx.value.ownerCode)) || [] }
  catch (err) { emit('status', String(err), 'error') }
}

const selectedSlot = computed(() => slots.value.find(s => s.unitId === targetSlot.value) || null)

// ── 配装模拟器：随所选因子实时算「词条加成汇总」 ──
const bonuses = ref([])
const simulating = ref(false)
let simTimer = null
function refreshSim() {
  clearTimeout(simTimer)
  simTimer = setTimeout(async () => {
    const ids = form.value.sigilSlotIds.filter(Boolean)
    if (!props.savePath || !ids.length) { bonuses.value = []; return }
    simulating.value = true
    try { bonuses.value = (await LoadoutSimulate(props.savePath, ids)) || [] }
    catch { bonuses.value = [] }
    finally { simulating.value = false }
  }, 180)
}
watch(() => form.value.sigilSlotIds.slice(), refreshSim, { deep: true })
const catClass = (label) => ({ '攻击类': 'atk', '基础能力': 'base', '防御类': 'def', '支援类': 'sup' }[label] || 'misc')

async function loadCtx() {
  if (!props.savePath || !props.charaHash) return
  loading.value = true
  ctx.value = null
  try {
    ctx.value = await LoadoutEditContext(props.savePath, props.charaHash)
    // 默认选第一个空槽，没有就第一个槽
    const empty = ctx.value.slots.find(s => !s.occupied)
    targetSlot.value = (empty || ctx.value.slots[0])?.unitId || 0
    if (occupiedSlots.value.length) cloneFrom.value = occupiedSlots.value[0].unitId
    if (masterySources.value.length) form.value.masterySource = masterySources.value[0].unitId
  } catch (err) {
    emit('status', String(err), 'error')
  } finally {
    loading.value = false
  }
}

watch(() => [props.savePath, props.charaHash], loadCtx, { immediate: true })

function toggleSigil(slotId) {
  const arr = form.value.sigilSlotIds
  const i = arr.indexOf(slotId)
  if (i >= 0) arr.splice(i, 1)
  else if (arr.length < 12) arr.push(slotId)
}
function toggleSkill(hash) {
  const arr = form.value.skillHashes
  const i = arr.indexOf(hash)
  if (i >= 0) arr.splice(i, 1)
  else if (arr.length < 4) arr.push(hash)
}

const writeInvalid = computed(() => {
  if (op.value === 'clear') return false
  if (op.value === 'clone') return !cloneFrom.value || cloneFrom.value === targetSlot.value
  return !form.value.name.trim() || nameTooLong.value
})

function opLabel() {
  return op.value === 'write' ? '写入' : op.value === 'clone' ? '克隆' : '清空'
}

async function apply() {
  if (!props.savePath || !targetSlot.value) return
  const slotNo = selectedSlot.value?.slot ?? '?'
  const occupied = selectedSlot.value?.occupied
  const verb = op.value === 'clear' ? '清空' : (occupied ? '覆盖' : '写入')
  const detail = op.value === 'clear'
    ? `将清空【${props.charaName}·槽${String(slotNo).padStart(2, '0')}】的配装。`
    : `将${verb}【${props.charaName}·槽${String(slotNo).padStart(2, '0')}】的配装。请确认游戏已完全退出；工具会先自动备份原存档，再写入并回读验证。`
  const confirmed = await confirmDialog.value?.ask({
    title: '写入存档前确认',
    detail,
    tone: 'warning',
    confirmLabel: '备份并写入',
  })
  if (!confirmed) return

  const w = { unitId: targetSlot.value, expectCharaHash: props.charaHash, op: op.value }
  if (op.value === 'write') {
    w.name = form.value.name
    w.weaponSlotId = form.value.weaponSlotId || 0
    w.sigilSlotIds = [...form.value.sigilSlotIds]
    w.skillHashes = [...form.value.skillHashes]
    if (masteryMode.value === 'free') {
      w.masteryHashes = ['R1', 'R2', 'R3', 'EX'].flatMap(r => masteryPick.value[r] || [])
    } else {
      const src = masterySources.value.find(m => m.unitId === form.value.masterySource)
      w.masteryHashes = src ? [...src.nodeHashes] : []
    }
  } else if (op.value === 'clone') {
    w.cloneFromUnitId = cloneFrom.value
  }

  applying.value = true
  try {
    const res = await LoadoutApply(props.savePath, '', [w])
    emit('status', `已${opLabel()}并验证 ${res.verifiedFields} 项${res.backupPath ? '（已自动备份）' : ''}`, 'success')
    emit('reload')
    await loadCtx()
  } catch (err) {
    emit('status', String(err), 'error')
  } finally {
    applying.value = false
  }
}
</script>

<template>
  <div class="loadout-editor">
    <div v-if="loading" class="hint">正在读取该角色可用资源…</div>
    <template v-else-if="ctx">
      <div class="ed-head">
        <strong>{{ charaName }}</strong>
        <span v-if="ctx.ownerCode" class="owner">{{ ctx.ownerCode }}</span>
        <span v-else class="owner warn">未能确定角色码（仅可用通用武器）</span>
      </div>

      <!-- 目标槽 -->
      <div class="ed-field">
        <label>目标槽位</label>
        <div class="slot-grid">
          <button v-for="s in slots" :key="s.unitId" class="slot-btn" :class="{ on: targetSlot === s.unitId, occ: s.occupied }"
            @click="targetSlot = s.unitId" :title="s.occupied ? s.name : '空槽'">
            {{ String(s.slot).padStart(2, '0') }}
          </button>
        </div>
        <small v-if="selectedSlot?.occupied" class="occ-warn">该槽已有配装「{{ selectedSlot.name || '(未命名)' }}」，写入将覆盖它</small>
      </div>

      <!-- 操作 -->
      <div class="ed-field">
        <label>操作</label>
        <div class="op-row">
          <button class="op-btn" :class="{ on: op === 'write' }" @click="op = 'write'">自定义写入</button>
          <button class="op-btn" :class="{ on: op === 'clone' }" @click="op = 'clone'" :disabled="!occupiedSlots.length">克隆现有</button>
          <button class="op-btn" :class="{ on: op === 'clear' }" @click="op = 'clear'">清空</button>
        </div>
      </div>

      <!-- 克隆源 -->
      <div v-if="op === 'clone'" class="ed-field">
        <label>克隆来源</label>
        <select v-model.number="cloneFrom" class="ed-select">
          <option v-for="s in occupiedSlots" :key="s.unitId" :value="s.unitId" :disabled="s.unitId === targetSlot">
            槽{{ String(s.slot).padStart(2, '0') }} · {{ s.name || '(未命名)' }}
          </option>
        </select>
      </div>

      <!-- 自定义写入表单 -->
      <template v-if="op === 'write'">
        <div class="ed-field">
          <label>配装名称 <em :class="{ over: nameTooLong }">{{ nameBytes }}/63 字节</em></label>
          <input v-model="form.name" class="ed-input" maxlength="30" placeholder="给这套配装起个名字" />
        </div>

        <div class="ed-field">
          <label>武器（{{ ctx.weapons.length }} 可选）</label>
          <select v-model.number="form.weaponSlotId" class="ed-select">
            <option :value="0">— 不设置武器 —</option>
            <option v-for="w in ctx.weapons" :key="w.slotId" :value="w.slotId">
              {{ w.name }}{{ w.ownerCode ? '' : '（通用）' }}
            </option>
          </select>
        </div>

        <div class="ed-field">
          <label>因子（{{ form.sigilSlotIds.length }}/12 · 池 {{ ctx.sigils.length }}）</label>
          <div class="pick-grid sigils">
            <button v-for="s in ctx.sigils" :key="s.slotId" class="pick" :class="{ on: form.sigilSlotIds.includes(s.slotId) }"
              @click="toggleSigil(s.slotId)" :title="s.generic ? '通用因子' : '角色因子'">
              {{ s.name }}<i v-if="s.level">Lv{{ s.level }}</i>
            </button>
          </div>
        </div>

        <!-- 配装模拟器：所选因子的词条加成汇总（同名相加→封顶→取游戏原表数值） -->
        <div class="ed-field" v-if="form.sigilSlotIds.length">
          <label>加成汇总 <em>{{ simulating ? '计算中…' : bonuses.length + ' 条词条' }}</em></label>
          <div v-if="bonuses.length" class="sim-list">
            <div v-for="b in bonuses" :key="b.traitId" class="sim-row" :class="catClass(b.catLabel)">
              <span class="sim-name">{{ b.name }}<i v-if="b.capped" class="sim-cap" title="已达词条上限">封顶</i></span>
              <span class="sim-lv">Lv{{ b.level }}<small v-if="b.rawLevel !== b.level">/{{ b.rawLevel }}</small></span>
              <span class="sim-eff">{{ b.effect }}</span>
            </div>
          </div>
          <small v-else-if="!simulating" class="hint">所选因子暂无可汇总的词条（或词条未收录数值）</small>
        </div>

        <div class="ed-field">
          <label>技能（{{ form.skillHashes.length }}/4 · 池 {{ ctx.skills.length }}）</label>
          <div class="pick-grid">
            <button v-for="s in ctx.skills" :key="s.hash" class="pick" :class="{ on: form.skillHashes.includes(s.hash) }" @click="toggleSkill(s.hash)">
              {{ s.name || s.hash }}
            </button>
            <span v-if="!ctx.skills.length" class="empty">该角色现有配装未记录技能，无法自定义技能</span>
          </div>
        </div>

        <div class="ed-field">
          <label>专精 <em v-if="masteryMode === 'free'">已点 {{ masteryTotal }}/50</em></label>
          <div class="op-row">
            <button class="op-btn" :class="{ on: masteryMode === 'copy' }" @click="masteryMode = 'copy'">复制现有</button>
            <button class="op-btn" :class="{ on: masteryMode === 'free' }" @click="masteryMode = 'free'; loadMasteryPool()" :disabled="!ctx.ownerCode">自由配置</button>
          </div>

          <template v-if="masteryMode === 'copy'">
            <select v-model.number="form.masterySource" class="ed-select">
              <option :value="0">— 不设置专精 —</option>
              <option v-for="m in masterySources" :key="m.unitId" :value="m.unitId">
                复制自 槽{{ String(m.slot).padStart(2, '0') }}「{{ m.name || '未命名' }}」（{{ m.nodeCount }} 节点）
              </option>
            </select>
            <small class="hint">整套复制自该角色已有配装，保证游戏认可</small>
          </template>

          <template v-else>
            <div class="rank-tabs">
              <button v-for="p in masteryPool" :key="p.rank" class="rank-tab" :class="{ on: masteryRankTab === p.rank }" @click="masteryRankTab = p.rank">
                {{ p.label }}<i :class="{ full: rankPicked(p.rank) >= p.cap }">{{ rankPicked(p.rank) }}/{{ p.cap }}</i>
              </button>
            </div>
            <div v-if="activeRankPool" class="node-grid">
              <button v-for="n in activeRankPool.nodes" :key="n.hash" class="node" :class="['cat-' + catAbbr(n.cat), { on: (masteryPick[activeRankPool.rank] || []).includes(n.hash) }]"
                @click="toggleNode(activeRankPool.rank, n.hash, activeRankPool.cap)" :title="n.desc">
                <b>{{ catAbbr(n.cat) }}</b>{{ n.name || n.desc.slice(0, 16) }}
              </button>
            </div>
            <small class="hint">满级需正好点满 10/10/10/20；各档分类（攻/防/界）自由混搭。写入前会校验合法性。</small>
          </template>
        </div>
      </template>

      <div class="ed-actions">
        <button class="apply-btn" :disabled="applying || writeInvalid" @click="apply">
          {{ applying ? '写入中…' : opLabel() + '到槽' + String(selectedSlot?.slot ?? 0).padStart(2, '0') }}
        </button>
        <small class="safety">写入前自动备份 · 写后回读验证 · 建议先在副本存档上试</small>
      </div>
    </template>
    <ConfirmDialog ref="confirmDialog" />
  </div>
</template>

<style scoped>
.loadout-editor { display:flex; flex-direction:column; gap:13px; }
.hint { font-size:.72rem; color:var(--text-muted); }
.ed-head { display:flex; align-items:baseline; gap:9px; }
.ed-head strong { font-size:.9rem; color:var(--text-primary); }
.ed-head .owner { font-size:.68rem; color:var(--gold); padding:1px 7px; border:1px solid var(--line-soft); border-radius:10px; }
.ed-head .owner.warn { color:var(--red); }
.ed-field { display:flex; flex-direction:column; gap:6px; }
.ed-field > label { font-size:.74rem; font-weight:700; color:var(--text-secondary); }
.ed-field > label em { font-style:normal; font-weight:600; color:var(--text-muted); margin-left:6px; }
.ed-field > label em.over { color:var(--red); }
.slot-grid { display:grid; grid-template-columns:repeat(auto-fill, minmax(38px, 1fr)); gap:5px; }
.slot-btn { min-height:30px; border:1px solid var(--line-soft); border-radius:5px; background:var(--sky-900); color:var(--text-muted); font-size:.72rem; cursor:pointer; user-select:none; }
.slot-btn.occ { color:var(--text-primary); border-color:var(--line-gold); }
.slot-btn.on { border-color:#765126; background:#8b6737; color:#fff9e9; }
.occ-warn { font-size:.68rem; color:var(--amber); }
.op-row, .ed-actions { display:flex; flex-wrap:wrap; gap:7px; align-items:center; }
.op-btn { min-height:30px; padding:0 13px; border:1px solid var(--line-gold); border-radius:6px; background:var(--sky-900); color:var(--text-primary); font-size:.76rem; cursor:pointer; user-select:none; }
.op-btn.on { border-color:#765126; background:#8b6737; color:#fff9e9; }
.op-btn:disabled { opacity:.4; cursor:not-allowed; }
.ed-input, .ed-select { min-height:32px; padding:0 10px; border:1px solid var(--line-gold); border-radius:6px; background:var(--panel-solid); color:var(--text-primary); font-size:.78rem; }
.ed-input:focus, .ed-select:focus { outline:2px solid rgba(154,116,64,.5); outline-offset:1px; }
.pick-grid { display:flex; flex-wrap:wrap; gap:6px; max-height:180px; overflow-y:auto; padding:2px; }
.pick-grid.sigils { max-height:220px; }
.pick { padding:3px 9px; border:1px solid var(--line-soft); border-radius:12px; background:var(--panel-solid); color:var(--text-secondary); font-size:.72rem; cursor:pointer; user-select:none; }
.pick:hover { border-color:var(--line-gold); }
.pick.on { border-color:#765126; background:#8b6737; color:#fff9e9; }
.pick i { font-style:normal; margin-left:5px; opacity:.7; font-size:.64rem; }
.empty { font-size:.7rem; color:var(--text-muted); }
.apply-btn { min-height:34px; padding:0 18px; border:1px solid #765126; border-radius:6px; background:#8b6737; color:#fff9e9; font-size:.8rem; font-weight:700; cursor:pointer; }
.apply-btn:hover:not(:disabled) { background:#76552d; }
.apply-btn:disabled { opacity:.45; cursor:not-allowed; }
.safety { font-size:.66rem; color:var(--text-muted); }
/* 模拟器加成汇总表 */
.sim-list { display:flex; flex-direction:column; gap:2px; max-height:240px; overflow-y:auto; border:1px solid var(--border-soft); border-radius:var(--radius-md); background:var(--surface-card-pop); padding:4px; }
.sim-row { display:grid; grid-template-columns:5.5em 4em 1fr; gap:8px; align-items:baseline; padding:3px 7px; border-radius:var(--radius-sm); font-size:.7rem; border-left:3px solid var(--border-soft); }
.sim-row.atk { border-left-color:var(--danger); }
.sim-row.base { border-left-color:var(--accent); }
.sim-row.def { border-left-color:var(--info); }
.sim-row.sup { border-left-color:var(--success); }
.sim-row:nth-child(even) { background:var(--surface-row); }
.sim-name { font-weight:800; color:var(--text-primary); white-space:nowrap; }
.sim-cap { font-style:normal; margin-left:4px; font-size:.6rem; color:var(--warning-ink); }
.sim-lv { color:var(--text-secondary); font-variant-numeric:tabular-nums; }
.sim-lv small { color:var(--text-muted); }
.sim-eff { color:var(--text-secondary); white-space:pre-line; line-height:1.4; }
/* 专精自由配置 */
.rank-tabs { display:flex; gap:6px; }
.rank-tab { min-height:28px; padding:0 12px; border:1px solid var(--border-strong); border-radius:var(--radius-sm); background:var(--surface-card-pop); color:var(--text-secondary); font-size:.74rem; cursor:pointer; user-select:none; }
.rank-tab.on { background:var(--selected-bg); border-color:var(--selected-border); color:var(--selected-fg); }
.rank-tab i { font-style:normal; margin-left:5px; font-size:.64rem; opacity:.8; }
.rank-tab i.full { color:var(--success); font-weight:800; }
.rank-tab.on i.full { color:#bfe6cd; }
.node-grid { display:grid; grid-template-columns:repeat(auto-fill, minmax(150px, 1fr)); gap:5px; max-height:260px; overflow-y:auto; padding:2px; }
.node { display:flex; align-items:center; gap:5px; padding:4px 8px; border:1px solid var(--border-soft); border-radius:var(--radius-sm); background:var(--surface-card-pop); color:var(--text-secondary); font-size:.68rem; text-align:left; cursor:pointer; user-select:none; line-height:1.3; }
.node b { flex:0 0 auto; width:15px; height:15px; display:grid; place-items:center; border-radius:3px; font-size:.6rem; font-weight:900; color:#fff; }
.node.cat-攻 b { background:var(--danger); } .node.cat-防 b { background:var(--info); } .node.cat-界 b { background:var(--accent); }
.node.on { border-color:var(--selected-border); background:var(--selected-bg); color:var(--selected-fg); }
.node.on b { background:rgba(255,255,255,.3); }
.ed-field .hint { font-size:.66rem; color:var(--text-muted); }
</style>
