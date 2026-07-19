<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import {
  CreateSaveSnapshot,
  ListSaveSnapshots,
  RestoreSaveSnapshot,
} from '../../wailsjs/go/main/App'
import ConfirmDialog from './ConfirmDialog.vue'

const emit = defineEmits(['status'])
const open = ref(false)
const loading = ref(false)
const creating = ref(false)
const restoringID = ref('')
const snapshots = ref([])
const confirmDialog = ref(null)

const latest = computed(() => snapshots.value[0] || null)
const triggerDetail = computed(() => {
  if (loading.value) return '读取中'
  if (!latest.value) return '暂无恢复点'
  return `${snapshots.value.length} 个恢复点 · ${shortTime(latest.value.displayTime)}`
})

function shortTime(value = '') {
  const match = String(value).match(/(\d{2}-\d{2})\s+(\d{2}:\d{2})/)
  return match ? `${match[1]} ${match[2]}` : value
}

function formatSize(bytes = 0) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(2)} MB`
}

function slotSummary(snapshot) {
  return (snapshot.slots || []).map(slot => `DATA ${slot.slot}`).join('、')
}

async function refresh({ quiet = false } = {}) {
  if (!quiet) loading.value = true
  try {
    snapshots.value = await ListSaveSnapshots() || []
  } catch (error) {
    if (!quiet) emit('status', `读取备份时间线失败：${String(error)}`, 'error')
  } finally {
    loading.value = false
  }
}

async function createManualBackup() {
  if (creating.value) return
  creating.value = true
  try {
    const snapshot = await CreateSaveSnapshot('手动安全备份')
    await refresh({ quiet: true })
    emit('status', `备份创建成功：${snapshot.displayTime} · ${snapshot.slots.length} 个存档`, 'success')
  } catch (error) {
    emit('status', `备份失败：${String(error)}`, 'error')
  } finally {
    creating.value = false
  }
}

async function restore(snapshot) {
  if (restoringID.value) return
  const accepted = await confirmDialog.value?.ask({
    title: '恢复存档前确认',
    message: `将回退到 ${snapshot.displayTime} 的存档状态。`,
    detail: `包含：${slotSummary(snapshot)}\n当前存档会先自动创建一个新的安全恢复点；请确认游戏已完全退出。`,
    tone: 'warning',
    confirmLabel: '安全恢复',
  })
  if (!accepted) return
  restoringID.value = snapshot.id
  try {
    const result = await RestoreSaveSnapshot(snapshot.id)
    await refresh({ quiet: true })
    emit('status', `已恢复 ${result.restored} 个存档，并保留恢复前安全点`, 'success')
  } catch (error) {
    emit('status', `恢复失败：${String(error)}`, 'error')
  } finally {
    restoringID.value = ''
  }
}

function toggle() {
  open.value = !open.value
  if (open.value) refresh()
}

function onKeydown(event) {
  if (event.key === 'Escape') open.value = false
}

onMounted(() => {
  window.addEventListener('keydown', onKeydown)
  refresh({ quiet: true })
})

onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
</script>

<template>
  <div class="save-protection">
    <button
      class="protection-trigger ui-btn is-subtle"
      :class="{ active: open }"
      type="button"
      aria-label="打开存档保护时间线"
      aria-controls="save-backup-flyout"
      :aria-expanded="open"
      aria-haspopup="dialog"
      data-testid="save-protection-trigger"
      @click="toggle"
    >
      <span class="shield" aria-hidden="true">
        <svg viewBox="0 0 24 24"><path d="M12 2.8 19 5.4v5.8c0 4.5-2.8 8.2-7 10-4.2-1.8-7-5.5-7-10V5.4L12 2.8Z"/><path d="m8.5 12 2.2 2.2 4.8-5"/></svg>
      </span>
      <span class="trigger-copy"><strong>存档保护</strong><small>{{ triggerDetail }}</small></span>
      <span class="trigger-arrow" :class="{ open }">⌄</span>
    </button>

    <Teleport to="body">
      <Transition name="backup-drawer">
        <div v-if="open" class="drawer-catcher" @mousedown.self="open = false">
          <section id="save-backup-flyout" class="backup-drawer ui-card" role="dialog" aria-modal="false" aria-label="存档保护时间线" data-testid="save-backup-drawer">
            <header class="drawer-head">
              <div class="drawer-emblem" aria-hidden="true">存</div>
              <div>
                <small>SAFE ARCHIVE · DATA 1–3</small>
                <h2>存档保护时间线</h2>
                <p>创建、查看和恢复存档快照</p>
              </div>
              <button class="drawer-close ui-btn is-subtle is-icon" type="button" aria-label="关闭备份时间线" @click="open = false">×</button>
            </header>

            <div class="drawer-actions ui-actions">
              <button class="manual-backup ui-btn is-primary" type="button" :disabled="creating" @click="createManualBackup">
                <span>＋</span>{{ creating ? '正在备份…' : '立即备份' }}
              </button>
              <button class="refresh-backups ui-btn is-ghost" type="button" :disabled="loading" @click="refresh()">{{ loading ? '读取中…' : '刷新' }}</button>
            </div>

            <div class="timeline-heading">
              <span>恢复点</span>
              <small>{{ snapshots.length }} 份 · 新到旧</small>
            </div>

            <div class="timeline ui-scroll-region" :class="{ loading }" data-testid="save-backup-timeline">
              <div v-if="!loading && snapshots.length === 0" class="timeline-empty">
                <span>◇</span><strong>暂无存档备份</strong>
                <p>创建备份后，会在这里显示带日期和时间的恢复点。</p>
              </div>

              <article v-for="(snapshot, index) in snapshots" :key="snapshot.id" class="snapshot-card ui-card" :class="{ latest: index === 0 }">
                <span class="timeline-dot" aria-hidden="true"></span>
                <div class="snapshot-main">
                  <div class="snapshot-time">
                    <time>{{ snapshot.displayTime }}</time>
                    <span v-if="index === 0">最新</span>
                  </div>
                  <strong class="snapshot-reason">{{ snapshot.reason }}</strong>
                  <div class="slot-row">
                    <span v-for="slot in snapshot.slots" :key="slot.slot" class="slot-chip">DATA {{ slot.slot }}</span>
                    <small>{{ formatSize(snapshot.totalSize) }}</small>
                  </div>
                </div>
                <button class="restore-point ui-btn is-ghost is-sm" type="button" :disabled="Boolean(restoringID)" @click="restore(snapshot)">
                  {{ restoringID === snapshot.id ? '恢复中…' : '恢复到这里' }}
                </button>
              </article>
            </div>

            <footer class="drawer-foot">
              <span>备份完整性使用 SHA-256 校验</span>
              <span>恢复时请先退出游戏</span>
            </footer>
          </section>
        </div>
      </Transition>
    </Teleport>
    <ConfirmDialog ref="confirmDialog" />
  </div>
</template>

<style scoped>
.save-protection {
  position:relative;
  display:flex;
  align-items:center;
}
.protection-trigger {
  height:var(--control-height-sm);
  max-width:260px;
  gap:var(--space-2);
  padding-inline:var(--space-3);
  border-left:1px solid var(--border-soft);
  border-radius:var(--radius-sm);
}
.protection-trigger.active {
  color:var(--accent-hover);
  background:var(--state-active);
}
.shield {
  width:22px;
  height:22px;
  flex:0 0 22px;
  display:grid;
  place-items:center;
  color:var(--accent);
}
.shield svg {
  width:18px;
  height:18px;
  fill:none;
  stroke:currentColor;
  stroke-width:1.8;
  stroke-linecap:round;
  stroke-linejoin:round;
}
.trigger-copy {
  min-width:0;
  display:block;
  text-align:left;
  line-height:var(--lh-tight);
}
.trigger-copy strong,.trigger-copy small { display:block; }
.trigger-copy strong { color:var(--text-primary); font-size:var(--fs-sm); font-weight:var(--fw-bold); }
.trigger-copy small {
  margin-top:1px;
  overflow:hidden;
  color:var(--text-muted);
  font-size:var(--fs-xs);
  font-weight:var(--fw-normal);
  text-overflow:ellipsis;
  white-space:nowrap;
}
.trigger-arrow {
  margin-left:auto;
  color:var(--text-muted);
  font-size:var(--fs-base);
  line-height:1;
  transition:transform var(--dur-fast) var(--ease-out);
}
.trigger-arrow.open { transform:rotate(180deg); }

.drawer-catcher {
  position:fixed;
  z-index:var(--z-drawer);
  inset:0;
  background:transparent;
}
.backup-drawer {
  position:fixed;
  top:94px;
  right:12px;
  width:min(420px,calc(100vw - 24px));
  max-height:calc(100dvh - 106px);
  display:flex;
  flex-direction:column;
  overflow:hidden;
  color:var(--text-primary);
  background:var(--surface-card);
  box-shadow:var(--shadow-3);
  font-family:var(--font-ui);
}
.drawer-head {
  position:relative;
  display:flex;
  align-items:center;
  gap:var(--space-4);
  padding:var(--space-6) 52px var(--space-5) var(--space-6);
  border-bottom:1px solid var(--border-soft);
  background:var(--surface-card-pop);
}
.drawer-emblem {
  width:40px;
  height:40px;
  flex:0 0 40px;
  display:grid;
  place-items:center;
  border:1px solid var(--accent-border);
  border-radius:var(--radius-md);
  color:var(--text-on-accent);
  background:var(--accent);
  font-size:var(--fs-base);
  font-weight:var(--fw-bold);
}
.drawer-head > div:nth-child(2) { min-width:0; }
.drawer-head small {
  display:block;
  color:var(--accent);
  font-size:var(--fs-xs);
  font-weight:var(--fw-bold);
  letter-spacing:.08em;
}
.drawer-head h2 {
  margin:2px 0 0;
  color:var(--text-primary);
  font-family:var(--font-display);
  font-size:var(--fs-lg);
  font-weight:var(--fw-bold);
  line-height:var(--lh-tight);
}
.drawer-head p {
  margin:2px 0 0;
  color:var(--text-muted);
  font-size:var(--fs-xs);
  line-height:var(--lh-normal);
}
.drawer-close {
  position:absolute;
  top:var(--space-3);
  right:var(--space-3);
  color:var(--text-secondary);
  font-size:20px;
}
.drawer-actions {
  flex:0 0 auto;
  padding:var(--space-5) var(--space-6);
}
.manual-backup { flex:1 1 180px; }
.refresh-backups { flex:0 0 auto; }
.drawer-actions button:disabled { cursor:wait; }

.timeline-heading {
  flex:0 0 auto;
  display:flex;
  align-items:center;
  justify-content:space-between;
  gap:var(--space-4);
  margin:0 var(--space-6);
  padding:0 var(--space-1) var(--space-3);
  border-bottom:1px solid var(--border-soft);
}
.timeline-heading span { color:var(--text-primary); font-size:var(--fs-sm); font-weight:var(--fw-bold); }
.timeline-heading small { color:var(--text-muted); font-size:var(--fs-xs); }
.timeline {
  position:relative;
  min-height:138px;
  flex:1 1 auto;
  overflow-y:auto;
  padding:var(--space-3) var(--space-6) var(--space-4);
}
.timeline.loading { opacity:.62; }
.snapshot-card {
  position:relative;
  min-width:0;
  display:grid;
  grid-template-columns:minmax(0,1fr) auto;
  align-items:center;
  gap:var(--space-3);
  margin-bottom:var(--space-2);
  padding:var(--space-4);
  background:var(--surface-card-pop);
  box-shadow:none;
}
.snapshot-card:hover { border-color:var(--border-strong); background:var(--surface-field-hover); }
.snapshot-card.latest {
  border-left:4px solid var(--selected-bar);
  background:color-mix(in srgb,var(--accent-soft) 28%,var(--surface-card-pop));
}
.timeline-dot { display:none; }
.snapshot-main { min-width:0; }
.snapshot-time {
  min-width:0;
  display:flex;
  align-items:center;
  flex-wrap:wrap;
  gap:var(--space-2);
}
.snapshot-time time {
  color:var(--text-primary);
  font-family:var(--font-data);
  font-size:var(--fs-sm);
  font-weight:var(--fw-bold);
  font-variant-numeric:tabular-nums;
}
.snapshot-time span {
  padding:2px var(--space-2);
  border-radius:var(--radius-pill);
  color:var(--selected-fg);
  background:var(--selected-bg);
  font-size:var(--fs-xs);
  font-weight:var(--fw-semibold);
}
.snapshot-reason {
  display:block;
  margin-top:var(--space-2);
  overflow:hidden;
  color:var(--text-secondary);
  font-size:var(--fs-xs);
  font-weight:var(--fw-semibold);
  text-overflow:ellipsis;
  white-space:nowrap;
}
.slot-row {
  display:flex;
  align-items:center;
  flex-wrap:wrap;
  gap:var(--space-1);
  margin-top:var(--space-2);
}
.slot-chip {
  padding:2px var(--space-2);
  border:1px solid var(--border-default);
  border-radius:var(--radius-pill);
  color:var(--text-secondary);
  background:var(--surface-field);
  font-family:var(--font-data);
  font-size:var(--fs-xs);
  font-weight:var(--fw-semibold);
}
.slot-row small {
  margin-left:var(--space-1);
  color:var(--text-muted);
  font-family:var(--font-data);
  font-size:var(--fs-xs);
}
.restore-point { min-width:96px; }
.timeline-empty {
  display:grid;
  place-items:center;
  padding:var(--space-8) var(--space-6);
  color:var(--text-muted);
  text-align:center;
}
.timeline-empty > span { color:var(--accent); font-size:var(--fs-xl); }
.timeline-empty strong { margin-top:var(--space-2); color:var(--text-primary); font-size:var(--fs-sm); }
.timeline-empty p {
  max-width:280px;
  margin:var(--space-2) 0 0;
  font-size:var(--fs-xs);
  line-height:var(--lh-normal);
}
.drawer-foot {
  flex:0 0 auto;
  display:flex;
  justify-content:space-between;
  flex-wrap:wrap;
  gap:var(--space-2) var(--space-4);
  padding:var(--space-4) var(--space-6);
  border-top:1px solid var(--border-soft);
  color:var(--text-muted);
  background:var(--surface-field);
  font-size:var(--fs-xs);
}
.backup-drawer-enter-active,.backup-drawer-leave-active {
  transition:opacity var(--dur-fast) var(--ease-out),transform var(--dur-fast) var(--ease-out);
}
.backup-drawer-enter-from,.backup-drawer-leave-to { opacity:0; transform:translateY(-4px); }

@media (max-width:720px) {
  .trigger-copy small { display:none; }
  .protection-trigger { max-width:150px; }
}
@media (max-width:520px) {
  .backup-drawer {
    top:76px;
    right:12px;
    left:12px;
    width:auto;
    max-height:calc(100dvh - 88px);
  }
  .snapshot-card { grid-template-columns:minmax(0,1fr); }
  .restore-point { width:100%; }
  .drawer-actions > * { flex:1 1 140px; }
}
@media (max-height:620px) {
  .backup-drawer { top:76px; max-height:calc(100dvh - 88px); }
  .drawer-head { padding-block:var(--space-4); }
  .timeline { min-height:104px; }
}
@media (prefers-reduced-motion:reduce) {
  .backup-drawer-enter-active,.backup-drawer-leave-active,.trigger-arrow { transition:none; }
}
</style>
