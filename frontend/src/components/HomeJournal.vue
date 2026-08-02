<script setup>
import journalScene from '../assets/gbfr/journal-scene-4k.webp'

defineProps({ version: { type: String, default: '—' } })
const emit = defineEmits(['open', 'warm'])

const groups = [
  {
    id: 'save', mark: '档', label: '存档与配装（离线）', hint: '完全退出游戏后编辑；保存前自动备份，写入后回读',
    items: [
      { id: 'loadoutPresets', icon: '❖', title: '配装预设', copy: '查看整套配装，手动编辑或按技能目标配因子' },
      { id: 'sigil', icon: '◇', title: '因子修改', copy: '给存档新增、批量生成或删除因子' },
      { id: 'wrightstone', icon: '✦', title: '祝福修改', copy: '给存档新增祝福石并设置三条技能' },
      { id: 'summonSave', icon: '☾', title: '召唤石存档修改', copy: '新增召唤石，或修改已有实例和装备引用' },
      { id: 'progression', icon: '⚔', title: '物品与武器', copy: '补素材，调整数量、武器等级与强化进度' },
      { id: 'saveDiff', icon: '⇄', title: '存档对比与复制', copy: '并排找差异，直接把右侧记录复制到左侧或反向复制' },
    ],
  },
  {
    id: 'memory', mark: '改', label: '游戏内即时编辑', hint: '先启动并连接游戏，再修改当前选中的装备或会话资源',
    items: [
      { id: 'sigilMemory', icon: '◈', title: '因子即时编辑', copy: '读取并修改游戏里当前高亮的因子' },
      { id: 'wrightstoneMemory', icon: '✦', title: '祝福石即时编辑', copy: '读取并修改游戏里当前高亮的祝福石' },
      { id: 'weaponMemory', icon: '⚔', title: '武器技能即时编辑', copy: '修改五个存档技能槽，并可常驻追加第六条及以后技能' },
      { id: 'summon', icon: '☾', title: '召唤石修改', copy: '修改当前召唤石的技能、副参数与等级' },
      { id: 'overlimit', icon: '✪', title: '角色上限突破', copy: '读取突破结果页并保存四项能力值' },
      { id: 'runtime', icon: '✧', title: '货币、素材与任务掉落', copy: '调整当前会话的金币、MSP、素材与掉落功能' },
    ],
  },
  {
    id: 'loadoutFlow', mark: '配', label: '配装采集与复刻', hint: '检测不会默认开启；点击后可持续后台采集',
    items: [
      { id: 'runtimeMonitor', icon: '队', title: '队友配装持续检测', copy: '点击开启后持续后台归档稳定队伍配装' },
      { id: 'loadout', icon: '❖', title: '配装录制与复刻', copy: '逐颗记录十二个因子，导出分享或写到备用因子' },
    ],
  },
  {
    id: 'runtimeTools', mark: '运', label: '单机运行时工具', hint: '按需主动开启；切页后保持运行，停用时安全恢复',
    items: [
      { id: 'runtimeQOL', icon: '显', title: '显示与房间工具', copy: '精确显示、房间 ID 与主线队长替换' },
      { id: 'virtualSigils', icon: '◇', title: '虚拟因子槽', copy: '运行时读取额外库存因子，不扩存档十二槽' },
      { id: 'audioMixer', icon: '声', title: '角色语音混音台', copy: '按角色调整后续语音与界面音效音量' },
      { id: 'camera', icon: '镜', title: '城镇镜头工坊', copy: '调整城镇镜头距离、高度与滚轮缩放' },
      { id: 'spatialTools', icon: '标', title: '坐标与移动工具', copy: '离线使用书签、传送、原生方向控制和连续跳跃' },
      { id: 'patchCombat', icon: '斗', title: '战斗规则补丁', copy: '闪避、格挡、Link 与召唤限制' },
      { id: 'patchCharacters', icon: '角', title: '角色机制补丁', copy: '按角色管理专属机制与冲突' },
      { id: 'patchQuest', icon: '任', title: '任务与便利补丁', copy: '倒计时、宝箱、结算与支线奖励' },
      { id: 'monster', icon: '怪', title: '怪物倍率与状态控制', copy: '离线实验怪物倍率、昏厥与 Overdrive 状态' },
    ],
  },
]
</script>

<template>
  <div class="journal-home ui-page is-fluid">
    <section class="illustrated-journal ui-card">
      <img class="journal-scene" :src="journalScene" alt="古兰、露莉亚与碧围绕冒险记事本的插画" loading="eager" decoding="async" fetchpriority="high">
      <div class="page-menu">
        <header class="project-heading">
          <span>GRANBLUE FANTASY: RELINK</span>
          <h1>碧蓝幻想：Relink 空域工坊</h1>
          <p>DLC 2.0.3 本地功能整合版</p>
          <p class="mode-guide">改存档：先<b>完全退出游戏</b>；游戏内实时改：先<b>启动并进入游戏</b>。同一份存档，两种方式别同时用。</p>
        </header>

        <nav class="home-groups" aria-label="功能入口">
          <section v-for="group in groups" :key="group.id" class="home-group" :data-group="group.id">
            <div class="home-group-head"><span class="home-group-mark">{{ group.mark }}</span><div><strong>{{ group.label }}</strong><small>{{ group.hint }}</small></div></div>
            <div class="home-group-items">
              <button v-for="item in group.items" :key="item.id" class="chapter-ribbon ui-card" @pointerenter="emit('warm', item.id)" @pointerdown="emit('warm', item.id)" @focus="emit('warm', item.id)" @click="emit('open', item.id)">
                <span class="chapter-icon">{{ item.icon }}</span>
                <span><strong>{{ item.title }}</strong><small>{{ item.copy }}</small></span>
                <b>›</b>
              </button>
            </div>
          </section>
        </nav>

        <div class="small-tabs">
          <button class="ui-btn is-ghost is-sm" @pointerenter="emit('warm', 'naturalDrop')" @pointerdown="emit('warm', 'naturalDrop')" @focus="emit('warm', 'naturalDrop')" @click="emit('open', 'naturalDrop')"><i>⚙</i>游戏文件、诊断与设置</button>
          <span>工具版本 {{ version }}</span>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.journal-home {
  width:100%;
  height:auto;
  min-height:100%;
  display:flex;
  flex-direction:column;
  padding:var(--space-5);
  color:var(--text-primary);
  font-family:var(--font-ui);
}
.illustrated-journal {
  position:relative;
  width:100%;
  height:auto;
  min-height:100%;
  flex:1 0 auto;
  overflow:hidden;
  border-radius:var(--radius-lg);
  background:var(--surface-card);
  box-shadow:var(--shadow-2);
  isolation:isolate;
}
.illustrated-journal::before {
  content:"";
  position:absolute;
  z-index:1;
  inset:0;
  background:linear-gradient(90deg,var(--surface-card-pop) 0%,color-mix(in srgb,var(--surface-card-pop) 96%,transparent) 50%,color-mix(in srgb,var(--surface-card-pop) 68%,transparent) 66%,color-mix(in srgb,var(--surface-card-pop) 12%,transparent) 80%,transparent 90%);
  pointer-events:none;
}
.illustrated-journal::after {
  content:"";
  position:absolute;
  z-index:1;
  inset:var(--space-3);
  border:1px solid var(--border-default);
  border-radius:var(--radius-md);
  pointer-events:none;
}
.journal-scene {
  position:absolute;
  z-index:0;
  inset:0;
  width:100%;
  height:100%;
  object-fit:cover;
  object-position:center;
}
.page-menu {
  position:relative;
  z-index:2;
  width:min(72%,920px);
  height:auto;
  min-height:100%;
  display:flex;
  flex-direction:column;
  justify-content:flex-start;
  padding:clamp(24px,3.2vh,40px) clamp(24px,2.4vw,40px) clamp(28px,4vh,48px) clamp(32px,4vw,64px);
}
.project-heading {
  margin:0 0 var(--space-6);
  padding:0 var(--space-3) var(--space-5);
  border-bottom:1px solid var(--border-default);
}
.project-heading > span {
  display:block;
  color:var(--accent);
  font-size:var(--fs-xs);
  font-weight:var(--fw-bold);
  letter-spacing:.14em;
}
.project-heading h1 {
  margin:var(--space-2) 0 0;
  color:var(--text-primary);
  font-family:var(--font-display);
  font-size:clamp(24px,2.5vw,30px);
  font-weight:var(--fw-bold);
  line-height:var(--lh-tight);
  letter-spacing:.02em;
}
.project-heading p {
  margin:var(--space-2) 0 0;
  color:var(--text-secondary);
  font-size:var(--fs-sm);
  font-weight:var(--fw-semibold);
}
.project-heading .mode-guide {
  margin-top:var(--space-3);
  padding:var(--space-3) var(--space-4);
  border-left:3px solid var(--accent);
  border-radius:0 var(--radius-sm) var(--radius-sm) 0;
  color:var(--text-secondary);
  background:color-mix(in srgb,var(--accent-soft) 48%,transparent);
  font-size:var(--fs-sm);
  font-weight:var(--fw-normal);
  line-height:var(--lh-normal);
}
.project-heading .mode-guide b { color:var(--text-primary); font-weight:var(--fw-bold); }
.home-groups {
  min-width:0;
  display:flex;
  flex-direction:column;
  gap:var(--space-5);
}
.home-group { min-width:0; }
.home-group-head {
  min-width:0;
  display:flex;
  align-items:center;
  gap:var(--space-3);
  margin:0 var(--space-3) var(--space-2);
}
.home-group-mark {
  width:28px;
  height:28px;
  flex:0 0 28px;
  display:grid;
  place-items:center;
  border:1px solid var(--border-strong);
  border-radius:var(--radius-sm);
  color:var(--accent-hover);
  background:var(--surface-card-pop);
  font-size:var(--fs-sm);
  font-weight:var(--fw-bold);
}
.home-group-head > div { min-width:0; }
.home-group-head strong,.home-group-head small { display:block; }
.home-group-head strong { color:var(--text-primary); font-size:var(--fs-md); font-weight:var(--fw-bold); }
.home-group-head small {
  margin-top:2px;
  overflow:hidden;
  color:var(--text-secondary);
  font-size:var(--fs-xs);
  font-weight:var(--fw-normal);
  text-overflow:ellipsis;
  white-space:nowrap;
}
.home-group-items {
  display:grid;
  grid-template-columns:repeat(2,minmax(0,1fr));
  gap:var(--space-2);
}
.chapter-ribbon {
  position:relative;
  min-width:0;
  min-height:58px;
  display:grid;
  grid-template-columns:30px minmax(0,1fr) auto;
  align-items:center;
  gap:var(--space-3);
  padding:var(--space-3) var(--space-4);
  color:var(--text-primary);
  background:color-mix(in srgb,var(--surface-card-pop) 88%,transparent);
  box-shadow:none;
  text-align:left;
  cursor:pointer;
  transition:var(--transition-control);
}
.chapter-ribbon:hover,.chapter-ribbon:focus-visible {
  border-color:var(--accent-border);
  background:var(--surface-card-pop);
  box-shadow:3px 0 0 var(--selected-bar),var(--shadow-1);
}
.chapter-icon {
  width:30px;
  height:30px;
  display:grid;
  place-items:center;
  border-radius:var(--radius-sm);
  color:var(--accent-hover);
  background:var(--accent-soft);
  font-size:var(--fs-base);
  font-weight:var(--fw-bold);
}
.chapter-ribbon > span:nth-child(2) { min-width:0; }
.chapter-ribbon strong,.chapter-ribbon small { display:block; }
.chapter-ribbon strong {
  overflow:hidden;
  color:var(--text-primary);
  font-size:var(--fs-sm);
  font-weight:var(--fw-bold);
  text-overflow:ellipsis;
  white-space:nowrap;
}
.chapter-ribbon small {
  margin-top:2px;
  overflow:hidden;
  color:var(--text-secondary);
  font-size:var(--fs-xs);
  font-weight:var(--fw-normal);
  text-overflow:ellipsis;
  white-space:nowrap;
}
.chapter-ribbon b { color:var(--accent-hover); font-size:var(--fs-lg); }
.small-tabs {
  display:flex;
  flex-wrap:wrap;
  align-items:center;
  gap:var(--space-3);
  margin:var(--space-6) var(--space-3) 0;
  padding-top:var(--space-5);
  border-top:1px solid var(--border-default);
}
.small-tabs button i { color:var(--accent); font-style:normal; font-size:var(--fs-sm); }
.small-tabs > span {
  margin-left:auto;
  color:var(--text-muted);
  font-size:var(--fs-xs);
  font-weight:var(--fw-semibold);
}

/* On a wide desktop the directory becomes a compact catalog instead of a
   narrow phone-like column. The illustration remains a secondary visual rail
   on the right, while every functional group gets enough room to stay useful. */
/* Windows 150–160% scaling turns a 2048px fullscreen monitor into roughly
   1280 CSS pixels inside WebView2, so enter the desktop catalog layout before
   1440px instead of treating a real fullscreen window like a compact one. */
@media (min-width:1180px) {
  .page-menu { width:min(68%,1240px); }
  .project-heading {
    display:grid;
    grid-template-columns:minmax(250px,.72fr) minmax(440px,1.28fr);
    gap:0 var(--space-6);
    align-items:end;
    margin-bottom:var(--space-4);
    padding-bottom:var(--space-4);
  }
  .project-heading > span,
  .project-heading h1,
  .project-heading > p:not(.mode-guide) { grid-column:1; }
  .project-heading .mode-guide {
    grid-column:2;
    grid-row:1 / span 3;
    align-self:center;
    margin:0;
  }
  .home-groups {
    display:grid;
    grid-template-columns:repeat(2,minmax(0,1fr));
    gap:var(--space-4) var(--space-5);
  }
  .home-group[data-group="loadoutFlow"],
  .home-group[data-group="runtimeTools"] { grid-column:1 / -1; }
  .home-group[data-group="loadoutFlow"] .home-group-items { grid-template-columns:repeat(2,minmax(0,1fr)); }
  .home-group[data-group="runtimeTools"] .home-group-items { grid-template-columns:repeat(3,minmax(0,1fr)); }
  .chapter-ribbon { min-height:52px; padding-block:var(--space-2); }
  .small-tabs { margin-top:var(--space-4); padding-top:var(--space-3); }
}

@media (max-width:960px) {
  .page-menu {
    width:min(82%,680px);
    padding-left:clamp(28px,5vw,48px);
  }
  .project-heading h1 { font-size:24px; }
}
@media (max-width:760px) {
  .journal-home { padding:var(--space-3); }
  .illustrated-journal { min-height:100%; }
  .illustrated-journal::before { background:color-mix(in srgb,var(--surface-card-pop) 84%,transparent); }
  .journal-scene { object-position:38% center; opacity:.46; }
  .page-menu {
    width:100%;
    height:auto;
    min-height:100%;
    padding:var(--space-8) var(--space-6);
  }
  .home-group-items { grid-template-columns:minmax(0,1fr); }
  .small-tabs > span { width:100%; margin-left:0; }
}
/* Short desktop windows keep the same scroll ownership and use denser cards. */
@media (max-height:920px) and (min-width:761px) {
  .journal-home { padding:var(--space-3); }
  .illustrated-journal { min-height:100%; }
  .page-menu { padding-block:var(--space-3); }
  .project-heading { margin-bottom:var(--space-4); padding-bottom:var(--space-3); }
  .project-heading h1 { font-size:22px; }
  .project-heading .mode-guide { margin-top:var(--space-2); padding-block:var(--space-2); }
  .home-groups { gap:var(--space-2); }
  .home-group-head { margin-bottom:var(--space-1); }
  .home-group-items { gap:var(--space-1); }
  .chapter-ribbon { min-height:44px; padding-block:var(--space-2); }
  .chapter-ribbon small { display:none; }
  .small-tabs { margin-top:var(--space-3); padding-top:var(--space-2); }
}
</style>
