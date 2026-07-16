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
  if (!latest.value) return '等待首份备份'
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
      class="protection-trigger"
      :class="{ active: open }"
      type="button"
      aria-label="打开存档保护时间线"
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
          <section class="backup-drawer" role="dialog" aria-modal="false" aria-label="存档保护时间线" data-testid="save-backup-drawer">
            <header class="drawer-head">
              <div class="drawer-emblem" aria-hidden="true">存</div>
              <div>
                <small>SAFE ARCHIVE · DATA 1–3</small>
                <h2>存档保护时间线</h2>
                <p>写入前自动备份已启用</p>
              </div>
              <button class="drawer-close" type="button" aria-label="关闭备份时间线" @click="open = false">×</button>
            </header>

            <div class="protection-note">
              <span class="note-mark">✓</span>
              <p><strong>每次写入前，自动保存所有实际存在的存档。</strong><br>恢复旧版本前还会再备份当前状态，不会覆盖唯一的回退路径。</p>
            </div>

            <div class="drawer-actions">
              <button class="manual-backup" type="button" :disabled="creating" @click="createManualBackup">
                <span>＋</span>{{ creating ? '正在备份…' : '立即备份' }}
              </button>
              <button class="refresh-backups" type="button" :disabled="loading" @click="refresh()">{{ loading ? '读取中…' : '刷新' }}</button>
            </div>

            <div class="timeline-heading">
              <span>恢复点</span>
              <small>{{ snapshots.length }} 份 · 新到旧</small>
            </div>

            <div class="timeline" :class="{ loading }" data-testid="save-backup-timeline">
              <div v-if="!loading && snapshots.length === 0" class="timeline-empty">
                <span>◇</span><strong>暂无存档备份</strong>
                <p>首次执行写入时，会在这里留下带日期和分钟的恢复点。</p>
              </div>

              <article v-for="(snapshot, index) in snapshots" :key="snapshot.id" class="snapshot-card" :class="{ latest: index === 0 }">
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
                <button class="restore-point" type="button" :disabled="Boolean(restoringID)" @click="restore(snapshot)">
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
.save-protection{position:relative;display:flex;align-items:center}
.protection-trigger{height:32px;display:flex;align-items:center;gap:7px;padding:3px 7px;border:0;border-left:1px solid rgba(123,91,42,.2);border-radius:0;color:#5f523f;background:transparent;box-shadow:none;cursor:pointer;transition:color .1s ease,background-color .1s ease}
.protection-trigger:hover,.protection-trigger.active{color:#4d402e;background:#eee0c1;box-shadow:none}
.shield{width:22px;height:22px;display:grid;place-items:center;color:#765528;background:transparent}.shield svg{width:17px;height:17px;fill:none;stroke:currentColor;stroke-width:1.8;stroke-linecap:round;stroke-linejoin:round}
.trigger-copy{display:block;text-align:left;line-height:1.05}.trigger-copy strong,.trigger-copy small{display:block;white-space:nowrap}.trigger-copy strong{font-size:10px;font-weight:800}.trigger-copy small{margin-top:3px;color:#8a7961;font-size:8px;font-weight:650}.trigger-arrow{margin-left:2px;color:#8d7651;font:900 13px/1 Georgia,serif;transition:transform .12s ease}.trigger-arrow.open{transform:rotate(180deg)}
.drawer-catcher{position:fixed;z-index:4200;inset:38px 0 0;background:transparent}
.backup-drawer{position:absolute;top:45px;right:17px;width:min(445px,calc(100vw - 34px));max-height:calc(100vh - 100px);display:flex;flex-direction:column;overflow:hidden;border:1px solid #9f7a45;border-radius:1px;color:#55493a;background:#f4e6c6;box-shadow:0 24px 60px rgba(40,26,8,.42),0 4px 16px rgba(40,26,8,.28);font-family:var(--font-ui,"Microsoft YaHei UI",sans-serif)}
.drawer-head{position:relative;display:flex;align-items:center;gap:12px;padding:17px 20px 13px;border-bottom:1px solid rgba(134,96,45,.26);background:#f7ebd0}
.drawer-emblem{width:37px;height:37px;display:grid;place-items:center;flex:0 0 37px;border:1px solid #8b6737;border-radius:50%;color:#765528;background:transparent;font:800 16px/1 var(--font-ui,"Microsoft YaHei UI",sans-serif);box-shadow:none}
.drawer-head small{display:block;color:#8f6a36;font-size:8px;font-weight:800;letter-spacing:.13em}.drawer-head h2{margin:3px 0 0;color:#4b4135;font-size:18px;font-weight:800;letter-spacing:0}.drawer-head p{margin:3px 0 0;color:#76572e;font-size:10px;font-weight:750}
.drawer-close{position:absolute;top:8px;right:9px;width:27px;height:27px;border:0;color:#765f42;background:transparent;font:400 24px/1 Georgia,serif;cursor:pointer;box-shadow:none}.drawer-close:hover{color:#4c3b28;background:#ead8b2}
.protection-note{display:flex;gap:10px;margin:13px 18px 0;padding:10px 11px;border:1px solid rgba(127,91,42,.3);border-left:3px solid #8b6737;background:#edddba}.note-mark{width:20px;height:20px;display:grid;place-items:center;flex:0 0 20px;border:1px solid rgba(126,91,42,.45);border-radius:50%;color:#765528;background:transparent;font-size:11px;font-weight:800}.protection-note p{margin:0;color:#645541;font-size:10px;font-weight:650;line-height:1.6}.protection-note strong{color:#4f493a;font-weight:800}
.drawer-actions{display:flex;gap:8px;padding:12px 18px}.drawer-actions button{min-height:32px;border-radius:1px;font:800 10px/1 var(--font-ui,"Microsoft YaHei UI",sans-serif);cursor:pointer;box-shadow:none}.manual-backup{flex:1;color:#fff9e9;border:1px solid #765126;background:#8b6737}.manual-backup:hover:not(:disabled){background:#76552d}.manual-backup span{margin-right:5px;font-size:14px}.refresh-backups{width:70px;color:#66543c;border:1px solid rgba(130,92,40,.38);background:#ead8b2}.refresh-backups:hover:not(:disabled){border-color:#8a6635;background:#f2e4c5}.drawer-actions button:disabled{opacity:.58;cursor:wait}
.timeline-heading{display:flex;align-items:center;justify-content:space-between;margin:0 18px 7px;padding:0 2px 7px;border-bottom:1px solid rgba(124,88,40,.28)}.timeline-heading span{color:#574b3d;font-size:11px;font-weight:800}.timeline-heading small{color:#89755a;font-size:9px;font-weight:700}
.timeline{position:relative;min-height:150px;max-height:370px;overflow-y:auto;padding:2px 18px 8px 33px;scrollbar-width:thin;scrollbar-color:rgba(137,96,44,.62) transparent}.timeline::-webkit-scrollbar{width:7px}.timeline::-webkit-scrollbar-track{background:transparent}.timeline::-webkit-scrollbar-thumb{border:2px solid transparent;border-radius:999px;background:rgba(137,96,44,.62) padding-box}.timeline::-webkit-scrollbar-button{display:none;width:0;height:0}.timeline.loading{opacity:.62}
.timeline::before{content:"";position:absolute;left:22px;top:5px;bottom:12px;width:1px;background:rgba(137,96,44,.35)}
.snapshot-card{position:relative;display:grid;grid-template-columns:minmax(0,1fr) auto;align-items:center;gap:10px;min-height:82px;margin-bottom:6px;padding:10px 10px 10px 12px;border:1px solid rgba(132,96,45,.28);border-radius:0;background:#f8edcf;box-shadow:none;transition:border-color .1s ease,background-color .1s ease}.snapshot-card:nth-child(even){background:#f0dfba}.snapshot-card:hover{border-color:rgba(126,91,42,.52);background:#f5e7c7}.snapshot-card.latest{border-left:3px solid #8b6737}.timeline-dot{position:absolute;left:-16px;top:34px;width:7px;height:7px;border:1px solid #9a8058;border-radius:50%;background:#f4e6c6;box-shadow:none}.snapshot-card.latest .timeline-dot{background:#8b6737;box-shadow:none}
.snapshot-main{min-width:0}.snapshot-time{display:flex;align-items:center;gap:7px}.snapshot-time time{color:#493f34;font:800 12px/1.2 var(--font-number,"Microsoft YaHei UI",sans-serif);font-variant-numeric:tabular-nums}.snapshot-time span{padding:2px 5px;border-radius:1px;color:#fff9e9;background:#8b6737;font-size:8px;font-weight:800}.snapshot-reason{display:block;margin-top:6px;overflow:hidden;color:#79664b;font-size:10px;font-weight:750;text-overflow:ellipsis;white-space:nowrap}.slot-row{display:flex;align-items:center;gap:4px;margin-top:7px}.slot-chip{padding:3px 5px;border:1px solid rgba(126,91,42,.28);border-radius:1px;color:#6e5738;background:#ead8b2;font:800 8px/1 var(--font-number,"Microsoft YaHei UI",sans-serif)}.slot-row small{margin-left:3px;color:#8c785e;font:750 8px/1 var(--font-number,"Microsoft YaHei UI",sans-serif)}
.restore-point{min-width:78px;min-height:31px;padding:0 9px;border:1px solid rgba(132,94,41,.38);border-radius:1px;color:#665338;background:#ead8b2;font:800 9px/1 var(--font-ui,"Microsoft YaHei UI",sans-serif);cursor:pointer;box-shadow:none}.restore-point:hover:not(:disabled){color:#4d3d28;border-color:#8a6635;background:#f3e5c5}.restore-point:disabled{opacity:.55;cursor:wait}
.timeline-empty{display:grid;place-items:center;padding:28px 20px;color:#8e7b5f;text-align:center}.timeline-empty>span{color:#765528;font-size:22px}.timeline-empty strong{margin-top:6px;color:#665744;font-size:12px}.timeline-empty p{max-width:280px;margin:7px 0 0;font-size:9px;font-weight:650;line-height:1.6}
.drawer-foot{display:flex;justify-content:space-between;gap:10px;padding:10px 18px;border-top:1px solid rgba(128,92,43,.24);color:#806f57;background:#ead8b2;font-size:8px;font-weight:700}
.backup-drawer-enter-active,.backup-drawer-leave-active{transition:opacity .12s ease}.backup-drawer-enter-from,.backup-drawer-leave-to{opacity:0}
@media (max-width:720px){.trigger-copy small{display:none}.backup-drawer{right:8px;width:calc(100vw - 16px)}.snapshot-card{grid-template-columns:1fr}.restore-point{width:100%}}
</style>
