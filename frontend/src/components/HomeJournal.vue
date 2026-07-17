<script setup>
import journalScene from '../assets/gbfr/journal-scene-4k.webp'

defineProps({ version: { type: String, default: '—' } })
const emit = defineEmits(['open', 'warm'])

// 首页按两大方式分组：存档修改（离线改文件）/ 内存注入（连游戏改内存）
const groups = [
  {
    id: 'save', mark: '档', label: '存档修改', hint: '退出游戏后离线改存档文件，可批量、可回滚',
    items: [
      { id: 'progression', icon: '⚔', title: '物品与武器', copy: '素材、武器等级与养成资源' },
      { id: 'sigil', icon: '◇', title: '因子修改', copy: '批量生成因子与合法性校验' },
      { id: 'loadoutPresets', icon: '❖', title: '配装预设', copy: '查看与写入配装、因子加成模拟' },
      { id: 'wrightstone', icon: '✦', title: '祝福修改', copy: '生成祝福石与三条词条' },
    ],
  },
  {
    id: 'memory', mark: '注', label: '内存注入', hint: '连接运行中的游戏改进程内存，实时生效',
    items: [
      { id: 'runtime', icon: '✧', title: '游戏内实时修改', copy: '金币、MSP、药水、素材与任务掉落' },
      { id: 'sigilMemory', icon: '◈', title: '因子即时编辑', copy: '改游戏中当前选中的因子' },
      { id: 'summon', icon: '☾', title: '召唤石修改', copy: '因子、副参数与等级' },
      { id: 'overlimit', icon: '✪', title: '角色上限突破', copy: '四个能力槽的突破值' },
    ],
  },
]
</script>

<template>
  <div class="journal-home">
    <section class="illustrated-journal">
      <img class="journal-scene" :src="journalScene" alt="格兰、露莉亚与碧围绕冒险记事本的插画" loading="eager" decoding="async" fetchpriority="high">
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
              <button v-for="item in group.items" :key="item.id" class="chapter-ribbon" @pointerenter="emit('warm', item.id)" @focus="emit('warm', item.id)" @click="emit('open', item.id)">
                <span class="chapter-icon">{{ item.icon }}</span>
                <span><strong>{{ item.title }}</strong><small>{{ item.copy }}</small></span>
                <b>›</b>
              </button>
            </div>
          </section>
        </nav>

        <div class="small-tabs">
          <button @pointerenter="emit('warm', 'compatibility')" @focus="emit('warm', 'compatibility')" @click="emit('open', 'compatibility')"><i>⚙</i>工具与设置</button>
          <span>工具版本 {{ version }}</span>
        </div>
      </div>

      <div class="safety-note">
        <i>✓</i>
        <span><strong>写入保护已启用</strong><small>自动备份 · 范围校验 · 回读验证</small></span>
      </div>
    </section>
  </div>
</template>

<style scoped>
.journal-home { height:100%; padding:13px; color:#564a3b; font-family:var(--font-ui); font-weight:700; }
.illustrated-journal { position:relative; width:100%; height:100%; min-height:520px; overflow:hidden; border:1px solid rgba(154,111,47,.38); border-radius:4px 14px 5px 14px; background:#fbefd5; box-shadow:0 18px 42px rgba(87,62,26,.2),inset 0 0 0 6px rgba(255,250,228,.5); isolation:isolate; }
.journal-scene { position:absolute; inset:0; z-index:0; width:100%; height:100%; object-fit:cover; object-position:center; }
.illustrated-journal::after { content:"";position:absolute;inset:9px;z-index:1;border:1px solid rgba(166,120,53,.22);border-radius:2px 9px 3px 9px;pointer-events:none }
.page-menu { position:relative;z-index:2;width:52%;min-width:520px;height:100%;display:flex;flex-direction:column;justify-content:center;padding:48px 34px 68px 64px }
.project-heading { margin:0 13px 18px;padding-bottom:18px;border-bottom:1px solid rgba(155,112,51,.24) }.project-heading span { display:block;color:#755326;font-size:9px;font-weight:900;letter-spacing:.2em }.project-heading h1 { margin:7px 0 0;color:#514437;font-size:27px;font-weight:900;letter-spacing:.045em;text-shadow:0 1px #fff8e4 }.project-heading p { margin:6px 0 0;color:#65533d;font-size:11px;font-weight:800 }.project-heading .mode-guide { margin-top:9px;padding:7px 10px;border-left:2px solid #b78237;border-radius:0 5px 5px 0;background:rgba(183,130,55,.1);color:#6a5636;font-size:10px;line-height:1.6 }.project-heading .mode-guide b { color:#4d3f2d;font-weight:900 }
.home-groups { display:flex; flex-direction:column; gap:14px; }
.home-group-head { display:flex; align-items:center; gap:9px; margin:0 13px 6px; }
.home-group-mark { width:22px;height:22px;flex:0 0 auto;display:grid;place-items:center;border:1px solid rgba(150,110,52,.4);border-radius:6px;color:#7a5b2e;background:rgba(249,240,214,.5);font-size:11px;font-weight:900 }
.home-group-head strong { display:block;color:#514437;font-size:12px;font-weight:900 }.home-group-head small { display:block;margin-top:2px;color:#6a5636;font-size:9px;font-weight:800 }
.home-group-items { display:grid; grid-template-columns:1fr 1fr; gap:6px; }
.chapter-ribbon { position:relative;min-height:50px;display:grid;grid-template-columns:28px 1fr 15px;align-items:center;gap:9px;padding:7px 11px;border:1px solid rgba(123,88,40,.22);border-radius:7px;color:#574b3d;background:rgba(255,251,238,.6);text-align:left;cursor:pointer;transition:background-color .12s ease,color .12s ease,border-color .12s ease,box-shadow .12s ease }
.chapter-ribbon:hover,.chapter-ribbon:focus-visible { color:#4d3f2d;background:rgba(255,249,229,.92);border-color:rgba(154,116,64,.5);box-shadow:inset 3px 0 #9a7440 }
.chapter-icon { width:26px;height:26px;display:grid;place-items:center;color:#745a35;font-size:13px;font-weight:900 }
.chapter-ribbon strong,.chapter-ribbon small { display:block }.chapter-ribbon strong { font-size:11.5px;font-weight:900 }.chapter-ribbon small { margin-top:2px;color:#554838;font-size:8.5px;font-weight:800 }.chapter-ribbon b { color:#796444;font-size:15px;font-weight:900 }
.small-tabs { display:flex;flex-wrap:wrap;align-items:center;gap:8px;margin:15px 13px 0;padding-top:13px;border-top:1px solid rgba(157,113,51,.2) }.small-tabs button { flex:0 0 auto;white-space:nowrap;display:inline-flex;align-items:center;gap:5px;min-height:27px;padding:0 12px;border:1px solid rgba(150,110,52,.32);border-radius:20px;color:#6a4e25;background:rgba(249,240,214,.42);font-size:9px;font-weight:900;cursor:pointer;transition:background-color .12s ease,border-color .12s ease,color .12s ease }.small-tabs button i { font-style:normal;font-size:10px;color:#9a7440 }.small-tabs button:hover { color:#59410f;border-color:rgba(154,116,64,.62);background:rgba(255,249,229,.85);box-shadow:0 1px 4px rgba(120,86,40,.12) }.small-tabs span { margin-left:auto;color:#6b5944;font-size:8px;font-weight:900 }
.safety-note { position:absolute;z-index:3;left:50%;bottom:20px;display:flex;align-items:center;gap:9px;padding:8px 13px 8px 9px;color:#5e5141;background:rgba(255,247,219,.94);border:1px solid rgba(157,113,51,.26);border-radius:34px;box-shadow:0 5px 13px rgba(80,56,25,.11);transform:translateX(-50%) rotate(-1deg) }.safety-note i { width:28px;height:28px;display:grid;place-items:center;border:2px solid #246a70;border-radius:50%;color:#246a70;font-style:normal;font-size:11px;font-weight:900 }.safety-note strong,.safety-note small { display:block }.safety-note strong { font-size:9px;font-weight:900 }.safety-note small { margin-top:2px;color:#66533d;font-size:8px;font-weight:900 }
@media(max-width:1100px){.page-menu{width:48%;min-width:390px;padding-left:48px}.project-heading h1{font-size:23px}.illustrated-journal{min-height:590px}.safety-note{left:auto;right:25px;transform:rotate(-1deg)}}
@media(max-width:760px){.illustrated-journal{aspect-ratio:auto;min-height:650px}.journal-scene{object-position:34% center;opacity:.4}.page-menu{width:100%;min-width:0;padding:44px 28px}.safety-note{display:none}}
</style>
