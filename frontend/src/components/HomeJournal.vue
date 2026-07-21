<script setup>
import journalScene from '../assets/gbfr/journal-scene-4k.webp'

defineProps({ version: { type: String, default: '—' } })
const emit = defineEmits(['open', 'warm'])

// 首页把只读内存监测单列，避免与存档修改或运行时注入混淆。
const groups = [
  {
    id: 'save', mark: '档', label: '存档修改', hint: '退出游戏后离线改存档文件，可批量、可回滚',
    items: [
      { id: 'loadoutPresets', icon: '❖', title: '配装预设', copy: '查看与写入配装、因子加成模拟' },
      { id: 'sigil', icon: '◇', title: '因子修改', copy: '批量生成因子与合法性校验' },
      { id: 'progression', icon: '⚔', title: '物品与武器', copy: '素材、武器等级与养成资源' },
      { id: 'wrightstone', icon: '✦', title: '祝福修改', copy: '生成祝福石与三条词条' },
      { id: 'summonSave', icon: '☾', title: '召唤石存档修改', copy: '新增或完整修改召唤石' },
    ],
  },
  {
    id: 'memory', mark: '注', label: '内存注入', hint: '连接运行中的游戏改进程内存，实时生效',
    items: [
      { id: 'runtime', icon: '✧', title: '游戏内实时修改', copy: '金币、MSP、药水、素材与任务掉落' },
      { id: 'sigilMemory', icon: '◈', title: '因子即时编辑', copy: '改游戏中当前选中的因子' },
      { id: 'wrightstoneMemory', icon: '✦', title: '祝福石即时编辑', copy: '改游戏中当前选中的祝福石' },
      { id: 'loadout', icon: '❖', title: '配装录制与复刻', copy: '记录、分享并逐项复刻十二个因子' },
      { id: 'summon', icon: '☾', title: '召唤石修改', copy: '因子、副参数与等级' },
      { id: 'overlimit', icon: '✪', title: '角色上限突破', copy: '四个能力槽的突破值' },
      { id: 'ctCombat', icon: '斗', title: '战斗规则补丁', copy: '闪避、格挡、Link 与召唤限制' },
      { id: 'ctCharacters', icon: '角', title: '角色机制补丁', copy: '按角色管理专属机制与冲突' },
      { id: 'ctQuest', icon: '任', title: '任务与便利补丁', copy: '倒计时、宝箱、结算与支线奖励' },
    ],
  },
  {
    id: 'monitor', mark: '测', label: '内存监测', hint: '只读读取运行中游戏数据，不修改物品或存档',
    items: [
      { id: 'ctMonitor', icon: '测', title: '运行监测', copy: '队伍快照、选中素材与关键物品' },
      { id: 'formulaSampler', icon: '证', title: '公式采样', copy: '单变量 A/B/A/B 角色面板证据' },
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
          <h1>GBFR 存档修改工具</h1>
          <p>DLC 2.0.2 本地功能整合版</p>
          <p class="mode-guide">改存档：先<b>完全退出游戏</b>；游戏内实时改：先<b>启动并进入游戏</b>。同一份存档，两种方式别同时用。</p>
        </header>

        <nav class="home-groups" aria-label="功能入口">
          <section v-for="group in groups" :key="group.id" class="home-group">
            <div class="home-group-head"><span class="home-group-mark">{{ group.mark }}</span><div><strong>{{ group.label }}</strong><small>{{ group.hint }}</small></div></div>
            <div class="home-group-items">
              <button v-for="item in group.items" :key="item.id" class="chapter-ribbon ui-card" @pointerenter="emit('warm', item.id)" @focus="emit('warm', item.id)" @click="emit('open', item.id)">
                <span class="chapter-icon">{{ item.icon }}</span>
                <span><strong>{{ item.title }}</strong><small>{{ item.copy }}</small></span>
                <b>›</b>
              </button>
            </div>
          </section>
        </nav>

        <div class="small-tabs">
          <button class="ui-btn is-ghost is-sm" @pointerenter="emit('warm', 'compatibility')" @focus="emit('warm', 'compatibility')" @click="emit('open', 'compatibility')"><i>⚙</i>工具与设置</button>
          <span>工具版本 {{ version }}</span>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.journal-home {
  width:100%;
  height:100%;
  min-height:0;
  display:flex;
  flex-direction:column;
  padding:var(--space-5);
  color:var(--text-primary);
  font-family:var(--font-ui);
}
.illustrated-journal {
  position:relative;
  width:100%;
  height:100%;
  min-height:520px;
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
  background:linear-gradient(90deg,var(--surface-card-pop) 0%,color-mix(in srgb,var(--surface-card-pop) 94%,transparent) 39%,color-mix(in srgb,var(--surface-card-pop) 36%,transparent) 61%,transparent 78%);
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
  width:clamp(500px,44vw,680px);
  height:100%;
  display:flex;
  flex-direction:column;
  justify-content:center;
  padding:clamp(32px,5vh,56px) clamp(28px,3vw,48px) clamp(32px,5vh,56px) clamp(36px,5vw,72px);
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

@media (max-width:960px) {
  .page-menu {
    width:clamp(460px,58vw,560px);
    padding-left:clamp(28px,5vw,48px);
  }
  .project-heading h1 { font-size:24px; }
}
@media (max-width:760px) {
  .journal-home { padding:var(--space-3); }
  .illustrated-journal { height:auto; min-height:720px; }
  .illustrated-journal::before { background:color-mix(in srgb,var(--surface-card-pop) 84%,transparent); }
  .journal-scene { object-position:38% center; opacity:.46; }
  .page-menu {
    width:100%;
    height:auto;
    min-height:720px;
    padding:var(--space-8) var(--space-6);
  }
  .home-group-items { grid-template-columns:minmax(0,1fr); }
  .small-tabs > span { width:100%; margin-left:0; }
}
/* Once the complete catalog is taller than a normal desktop viewport, let the
   outer workspace own scrolling and keep the project heading at the top. */
@media (max-height:920px) and (min-width:761px) {
  .journal-home { height:auto; min-height:100%; padding:var(--space-3); }
  .illustrated-journal { height:auto; min-height:500px; }
  .page-menu { height:auto; justify-content:flex-start; padding-block:var(--space-3); }
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
