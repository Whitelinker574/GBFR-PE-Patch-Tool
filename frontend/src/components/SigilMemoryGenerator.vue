<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { SigilMemoryGetOptions, SigilMemoryGetStatus, SigilMemoryEnable, SigilMemoryUpdate } from '../../wailsjs/go/main/App'

const emit = defineEmits(['status'])
const status = reactive({ found: false, hooked: false, selectedAddr: 0, sigilHash: 0, sigilLevel: 0, primaryTraitHash: 0, primaryTraitLevel: 0, secondaryTraitHash: 0, secondaryTraitLevel: 0, sigilName: '', primaryTraitName: '', secondaryTraitName: '' })
const form = reactive({ sigilHash: 0, sigilLevel: 0, primaryTraitHash: 0, primaryTraitLevel: 0, secondaryTraitHash: 0, secondaryTraitLevel: 0 })
const options = reactive({ sigils: [], traits: [] })
const loading = ref(false)
const applying = ref(false)
const sigilSearch = ref('')
const primarySearch = ref('')
const secondarySearch = ref('')

function show(msg, type) { emit('status', msg, type) }
function applyStatus(next, syncForm = false) {
  Object.assign(status, next)
  if (syncForm) {
    form.sigilHash = next.sigilHash >>> 0
    form.sigilLevel = next.sigilLevel >>> 0
    form.primaryTraitHash = next.primaryTraitHash >>> 0
    form.primaryTraitLevel = next.primaryTraitLevel >>> 0
    form.secondaryTraitHash = next.secondaryTraitHash >>> 0
    form.secondaryTraitLevel = next.secondaryTraitLevel >>> 0
  }
}
function hex(value) { return `0x${(Number(value) >>> 0).toString(16).toUpperCase().padStart(8, '0')}` }
function filter(items, query) {
  const q = query.trim().toLowerCase()
  return !q ? items : items.filter(item => item.displayName.toLowerCase().includes(q) || hex(item.hash).toLowerCase().includes(q))
}
const filteredSigils = computed(() => filter(options.sigils, sigilSearch.value))
const filteredPrimaryTraits = computed(() => filter(options.traits, primarySearch.value))
const filteredSecondaryTraits = computed(() => filter(options.traits, secondarySearch.value))

async function loadOptions() {
  try { Object.assign(options, await SigilMemoryGetOptions()) }
  catch (e) { show('读取因子数据失败: ' + String(e), 'error') }
}
async function refresh(syncForm = false) {
  loading.value = true
  try {
    const next = await SigilMemoryGetStatus()
    applyStatus(next, syncForm)
    if (!next.hooked) show('已定位。点击开启读取后，在游戏内选中因子。', 'success')
    else if (!next.selectedAddr) show('读取已开启。请在游戏内因子列表选中一个因子。', 'success')
    else show(`已读取: ${next.sigilName}`, 'success')
  } catch (e) { show(String(e), 'error') }
  finally { loading.value = false }
}
async function enable() {
  loading.value = true
  try {
    applyStatus(await SigilMemoryEnable(), true)
    show('读取已开启。请在游戏内因子列表选中一个因子。', 'success')
  } catch (e) { show(String(e), 'error') }
  finally { loading.value = false }
}
async function write() {
  if (!status.hooked || !status.selectedAddr) { show('请先开启读取，并在游戏内选中一个因子', 'error'); return }
  applying.value = true
  try {
    const next = await SigilMemoryUpdate({ ...form })
    applyStatus(next, true)
    show(`已写入并提交保存: ${next.sigilName}`, 'success')
  } catch (e) { show(String(e), 'error') }
  finally { applying.value = false }
}
function chooseSigil(item) { form.sigilHash = item.hash; sigilSearch.value = item.displayName }
function choosePrimary(item) { form.primaryTraitHash = item.hash; primarySearch.value = item.displayName }
function chooseSecondary(item) { form.secondaryTraitHash = item.hash; secondarySearch.value = item.displayName }

onMounted(async () => { await loadOptions(); await refresh(true) })
</script>

<template>
  <div class="memory-sigil">
    <div class="section">
      <div class="section-title">内存连接</div>
      <div class="actions">
        <button class="btn btn-cyan" :disabled="loading" @click="enable">{{ status.hooked ? '读取已开启' : '开启读取' }}</button>
        <button class="btn" :disabled="loading" @click="refresh(true)">{{ loading ? '读取中...' : '刷新选中因子' }}</button>
      </div>
      <div class="hint">游戏内打开因子列表并选中目标因子。修改后切换选择或关闭界面触发游戏保存。</div>
      <div class="state" :class="{ ready: status.selectedAddr }">
        {{ status.selectedAddr ? `已选中 ${hex(status.selectedAddr)}` : status.hooked ? '等待游戏内选中因子' : '未开启读取' }}
      </div>
    </div>

    <div class="section" :class="{ muted: !status.selectedAddr }">
      <div class="section-title">当前选中因子</div>
      <div class="current-grid">
        <span>因子</span><strong>{{ status.sigilName }} · Lv {{ status.sigilLevel }}</strong>
        <span>主词条</span><strong>{{ status.primaryTraitName }} · Lv {{ status.primaryTraitLevel }}</strong>
        <span>副词条</span><strong>{{ status.secondaryTraitName }} · Lv {{ status.secondaryTraitLevel }}</strong>
      </div>
    </div>

    <div class="section" :class="{ muted: !status.selectedAddr }">
      <div class="section-title">修改目标</div>
      <div class="field">
        <label>因子</label>
        <input v-model="sigilSearch" class="input" placeholder="搜索因子或 0x 哈希" />
        <select class="select list" size="5" :value="form.sigilHash" @change="chooseSigil(filteredSigils.find(v => v.hash === Number($event.target.value)))">
          <option v-for="item in filteredSigils" :key="item.hash" :value="item.hash">{{ item.displayName }} · {{ hex(item.hash) }}</option>
        </select>
      </div>
      <div class="level-row">
        <label>因子等级</label><input v-model.number="form.sigilLevel" class="input level" type="number" min="0" max="999" />
      </div>
      <div class="field">
        <label>主词条</label>
        <input v-model="primarySearch" class="input" placeholder="搜索主词条" />
        <select class="select list" size="5" :value="form.primaryTraitHash" @change="choosePrimary(filteredPrimaryTraits.find(v => v.hash === Number($event.target.value)))">
          <option v-for="item in filteredPrimaryTraits" :key="item.hash" :value="item.hash">{{ item.displayName }} · {{ hex(item.hash) }}</option>
        </select>
      </div>
      <div class="level-row">
        <label>主词条等级</label><input v-model.number="form.primaryTraitLevel" class="input level" type="number" min="0" max="999" />
      </div>
      <div class="field">
        <label>副词条</label>
        <input v-model="secondarySearch" class="input" placeholder="搜索副词条" />
        <select class="select list" size="5" :value="form.secondaryTraitHash" @change="chooseSecondary(filteredSecondaryTraits.find(v => v.hash === Number($event.target.value)))">
          <option v-for="item in filteredSecondaryTraits" :key="item.hash" :value="item.hash">{{ item.displayName }} · {{ hex(item.hash) }}</option>
        </select>
      </div>
      <div class="level-row">
        <label>副词条等级</label><input v-model.number="form.secondaryTraitLevel" class="input level" type="number" min="0" max="999" />
      </div>
      <button class="btn btn-green write" :disabled="applying || !status.selectedAddr" @click="write">{{ applying ? '写入中...' : '写入选中因子' }}</button>
    </div>
  </div>
</template>

<style scoped>
.memory-sigil { width:100%; display:flex; flex-direction:column; gap:14px; }
.section { padding:14px 16px; border:1px solid rgba(255,255,255,.08); border-radius:8px; background:rgba(255,255,255,.04); display:flex; flex-direction:column; gap:10px; }
.section.muted { opacity:.55; }
.section-title { color:rgba(255,255,255,.4); font-size:.78rem; font-weight:600; letter-spacing:1px; }
.actions, .level-row { display:flex; gap:8px; align-items:center; }
.hint, .state { font-size:.74rem; color:rgba(255,255,255,.35); line-height:1.5; }
.state.ready { color:#4ade80; }
.current-grid { display:grid; grid-template-columns:70px 1fr; gap:6px 10px; font-size:.78rem; }
.current-grid span { color:rgba(255,255,255,.35); }.current-grid strong { color:rgba(255,255,255,.68); overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.field { display:flex; flex-direction:column; gap:5px; }.field label, .level-row label { color:rgba(255,255,255,.35); font-size:.72rem; }.level-row label { flex:1; }
.input, .select { box-sizing:border-box; width:100%; padding:8px 10px; border:1px solid rgba(255,255,255,.12); border-radius:6px; background:rgba(255,255,255,.06); color:#fff; outline:none; font:inherit; font-size:.8rem; }.input:focus, .select:focus { border-color:rgba(103,232,249,.5); }.select option { background:#1b2636; color:#fff; }.list { min-height:106px; padding:3px; }.list option { padding:4px 6px; }.level { width:110px; }.btn { padding:8px 14px; border:1px solid rgba(255,255,255,.16); border-radius:6px; background:rgba(255,255,255,.06); color:rgba(255,255,255,.75); font-size:.8rem; font-weight:600; cursor:pointer; }.btn:disabled { opacity:.4; cursor:not-allowed; }.btn-cyan { border-color:rgba(103,232,249,.35); color:#67e8f9; background:rgba(103,232,249,.1); }.btn-green { border-color:rgba(74,222,128,.35); color:#4ade80; background:rgba(74,222,128,.1); }.write { align-self:flex-end; }
</style>
