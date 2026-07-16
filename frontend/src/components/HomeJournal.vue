<script setup>
import journalScene from '../assets/gbfr/journal-scene-4k.webp'

defineProps({ version: { type: String, default: '—' } })
const emit = defineEmits(['open', 'warm'])

const chapters = [
  { id: 'progression', icon: '⚔', title: '物品与武器', copy: '素材、武器等级与养成', mode: 'offline' },
  { id: 'sigil', icon: '◇', title: '因子修改（存档）', copy: '批量生成因子与合法性校验', mode: 'offline' },
  { id: 'runtime', icon: '✦', title: '游戏内实时修改', copy: '金币、MSP、药水、素材与任务掉落', mode: 'live' },
  { id: 'chara', icon: '✎', title: '次数统计', copy: '角色使用次数与任务完成次数', mode: 'offline' },
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

        <nav class="chapter-list" aria-label="常用功能">
          <button v-for="(chapter, index) in chapters" :key="chapter.id" class="chapter-ribbon" :style="`--delay:${index * 45}ms`" @pointerenter="emit('warm', chapter.id)" @focus="emit('warm', chapter.id)" @click="emit('open', chapter.id)">
            <span class="chapter-icon">{{ chapter.icon }}</span>
            <span><strong>{{ chapter.title }}</strong><small>{{ chapter.copy }}</small></span>
            <em class="chapter-mode" :class="chapter.mode">{{ chapter.mode === 'live' ? '实时·需开游戏' : '离线·需关游戏' }}</em>
            <b>›</b>
          </button>
        </nav>

        <div class="small-tabs">
          <button @pointerenter="emit('warm', 'summon')" @focus="emit('warm', 'summon')" @click="emit('open', 'summon')"><i>☾</i>召唤石修改</button>
          <button @pointerenter="emit('warm', 'wrightstone')" @focus="emit('warm', 'wrightstone')" @click="emit('open', 'wrightstone')"><i>✦</i>祝福修改</button>
          <button @pointerenter="emit('warm', 'compatibility')" @focus="emit('warm', 'compatibility')" @click="emit('open', 'compatibility')"><i>⚙</i>版本适配</button>
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
.journal-home { min-height:100%; padding:13px; color:#564a3b; font-family:var(--font-ui); font-weight:700; }
.illustrated-journal { position:relative; width:100%; min-height:650px; aspect-ratio:16/9; overflow:hidden; border:1px solid rgba(154,111,47,.38); border-radius:4px 14px 5px 14px; background:#fbefd5; box-shadow:0 18px 42px rgba(87,62,26,.2),inset 0 0 0 6px rgba(255,250,228,.5); isolation:isolate; }
.journal-scene { position:absolute; inset:0; z-index:0; width:100%; height:100%; object-fit:cover; object-position:center; }
.illustrated-journal::after { content:"";position:absolute;inset:9px;z-index:1;border:1px solid rgba(166,120,53,.22);border-radius:2px 9px 3px 9px;pointer-events:none }
.page-menu { position:relative;z-index:2;width:42%;min-width:430px;height:100%;display:flex;flex-direction:column;justify-content:center;padding:58px 34px 75px 74px }
.project-heading { margin:0 13px 18px;padding-bottom:18px;border-bottom:1px solid rgba(155,112,51,.24) }.project-heading span { display:block;color:#755326;font-size:9px;font-weight:900;letter-spacing:.2em }.project-heading h1 { margin:7px 0 0;color:#514437;font-size:27px;font-weight:900;letter-spacing:.045em;text-shadow:0 1px #fff8e4 }.project-heading p { margin:6px 0 0;color:#65533d;font-size:11px;font-weight:800 }.project-heading .mode-guide { margin-top:9px;padding:7px 10px;border-left:2px solid #b78237;border-radius:0 5px 5px 0;background:rgba(183,130,55,.1);color:#6a5636;font-size:10px;line-height:1.6 }.project-heading .mode-guide b { color:#4d3f2d;font-weight:900 }
.chapter-list { display:flex;flex-direction:column;gap:0;border-top:1px solid rgba(123,88,40,.18) }.chapter-ribbon { position:relative;width:100%;min-height:56px;display:grid;grid-template-columns:31px 1fr auto 17px;align-items:center;gap:10px;padding:7px 12px;border:0;border-bottom:1px solid rgba(123,88,40,.2);border-radius:0;color:#574b3d;background:rgba(249,240,214,.28);box-shadow:none;clip-path:none;text-align:left;cursor:pointer;transition:background-color .1s ease,color .1s ease }.chapter-mode { font-style:normal;font-size:8px;font-weight:900;letter-spacing:.02em;padding:3px 7px;border-radius:20px;white-space:nowrap }.chapter-mode.offline { color:#2f7d5c;background:rgba(47,125,92,.13);border:1px solid rgba(47,125,92,.26) }.chapter-mode.live { color:#256e74;background:rgba(37,110,116,.13);border:1px solid rgba(37,110,116,.26) }.chapter-ribbon:hover,.chapter-ribbon:focus-visible { color:#4d3f2d;background:rgba(255,249,229,.55);box-shadow:inset 3px 0 #9a7440 }.chapter-icon { width:27px;height:27px;display:grid;place-items:center;border:0;border-radius:0;color:#745a35;background:transparent;font-size:12px;font-weight:900;text-shadow:none }.chapter-ribbon strong,.chapter-ribbon small { display:block }.chapter-ribbon strong { font-size:12px;font-weight:900;letter-spacing:.01em }.chapter-ribbon small { margin-top:3px;color:#554838;font-size:9px;font-weight:800 }.chapter-ribbon b { color:#796444;font-size:16px;font-weight:900 }
.small-tabs { display:flex;align-items:center;gap:8px;margin:15px 13px 0;padding-top:13px;border-top:1px solid rgba(157,113,51,.2) }.small-tabs button { display:inline-flex;align-items:center;gap:5px;min-height:27px;padding:0 11px;border:1px solid rgba(150,110,52,.32);border-radius:20px;color:#6a4e25;background:rgba(249,240,214,.42);font-size:9px;font-weight:900;cursor:pointer;transition:background-color .12s ease,border-color .12s ease,color .12s ease }.small-tabs button i { font-style:normal;font-size:10px;color:#9a7440 }.small-tabs button:hover { color:#59410f;border-color:rgba(154,116,64,.62);background:rgba(255,249,229,.85);box-shadow:0 1px 4px rgba(120,86,40,.12) }.small-tabs span { margin-left:auto;color:#6b5944;font-size:8px;font-weight:900 }
.safety-note { position:absolute;z-index:3;left:50%;bottom:20px;display:flex;align-items:center;gap:9px;padding:8px 13px 8px 9px;color:#5e5141;background:rgba(255,247,219,.94);border:1px solid rgba(157,113,51,.26);border-radius:34px;box-shadow:0 5px 13px rgba(80,56,25,.11);transform:translateX(-50%) rotate(-1deg) }.safety-note i { width:28px;height:28px;display:grid;place-items:center;border:2px solid #246a70;border-radius:50%;color:#246a70;font-style:normal;font-size:11px;font-weight:900 }.safety-note strong,.safety-note small { display:block }.safety-note strong { font-size:9px;font-weight:900 }.safety-note small { margin-top:2px;color:#66533d;font-size:8px;font-weight:900 }
@media(max-width:1100px){.page-menu{width:48%;min-width:390px;padding-left:48px}.project-heading h1{font-size:23px}.illustrated-journal{min-height:590px}.safety-note{left:auto;right:25px;transform:rotate(-1deg)}}
@media(max-width:760px){.illustrated-journal{aspect-ratio:auto;min-height:650px}.journal-scene{object-position:34% center;opacity:.4}.page-menu{width:100%;min-width:0;padding:44px 28px}.safety-note{display:none}}
</style>
