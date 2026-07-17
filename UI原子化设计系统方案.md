# UI 原子化设计系统 + 迁移方案

> 由 6-agent 工作流产出。审计发现：1015 !important / 1186 :deep / 387 色 / 令牌 4 处冲突定义 / --green,--cyan 被染成 #48c9df。

## 设计令牌（两层：primitive 调色板 → semantic 语义角色）
```css
/* ============================================================
   GBFR Patch Tool — 设计令牌单一事实源
   放入 frontend/src/style.css 的 :root，替换现有 19 个零散令牌。
   两层结构：①primitive 原始调色板(只在此定义一次) ②semantic 语义角色(组件只引用这一层)。
   禁止任何组件/后代选择器(.app-window 等)重定义这些变量。
   ============================================================ */
:root {
  color-scheme: light;

  /* ---------- 字体族 (沿用现值) ---------- */
  --font-ui:   "Noto Sans SC","Microsoft YaHei UI","Microsoft YaHei","Segoe UI",sans-serif;
  --font-data: "Noto Sans SC","Segoe UI","Microsoft YaHei UI",sans-serif; /* tabular-nums 数字 */

  /* ==========================================================
     PRIMITIVE — 原始调色板 (仅此处出现裸值，语义层引用它)
     ========================================================== */

  /* 羊皮纸面 paper：浅→深，收敛现有数十种近似棕 */
  --p-paper-100:#fffaf0;  /* 抬起/弹层顶面 (原 --panel-solid/#fff9e8) */
  --p-paper-200:#fdf6e4;  /* 卡片默认面 */
  --p-paper-300:#f6eacf;  /* 输入/内嵌字段面 (原 #f4e8cd) */
  --p-paper-400:#efe1c0;  /* 列表行/斑马纹/hover 填充 (原 #edddba) */
  --p-paper-500:#e8dcc0;  /* 下沉面/分隔 (原 --sky-1000) */
  --p-app-bg:  #ede1c5;   /* 应用最底色 (原羊皮纸 --bg) */

  /* 胡桃木/黄铜 walnut-brass：品牌主色阶 (选中/主按钮/描边) */
  --p-brass-050:#f0e2c2;  /* 极浅底纹 */
  --p-brass-100:#ead8b2;  /* 柔和/禁用填充 (原 language 禁用底) */
  --p-brass-300:#d7b66f;  /* 高光 (原 --gold-soft) */
  --p-brass-500:#a77d3d;  /* 黄铜线 (原 --gold) */
  --p-brass-600:#9a7440;  /* 强调/焦点/左条 == rgb(154,116,64) */
  --p-brass-650:#8b6737;  /* 选中实心 主品牌 */
  --p-brass-700:#76552d;  /* 选中 hover 加深 */
  --p-brass-800:#765126;  /* 选中描边/最深 */

  /* 墨 ink：文字阶 */
  --p-ink-900:#51483e;    /* 主文本 */
  --p-ink-700:#766a5d;    /* 次文本 */
  --p-ink-500:#998a77;    /* 弱文本/占位 */
  --p-ink-on-brass:#fff9e9;/* 棕金实心上的文字 */

  /* 线 line (半透明，随底色自适应) */
  --p-line-22:rgba(139,106,55,.22);
  --p-line-15:rgba(139,106,55,.15);
  --p-line-gold:rgba(156,117,53,.38);

  /* 功能状态 status (真值，不再被污染) */
  --p-success:#4c9c76;  --p-success-ink:#2b6547;  --p-success-bg:#dcebdd; --p-success-bar:#4a7658;
  --p-warning:#b78237;  --p-warning-ink:#8e682d;  --p-warning-bg:#efdfb9;
  --p-danger:#bd625e;   --p-danger-ink:#a24b47;   --p-danger-bg:#f6ded9;
  --p-info:#2e8f98;     --p-info-ink:#246a70;     --p-info-bg:#dcebed; /* 静音青，仅"信息/当前值/合法"，永不用于选中 */

  /* 焦点 */
  --p-focus-outline:rgba(154,116,64,.55);
  --p-focus-ring:rgba(214,182,111,.30);

  /* ==========================================================
     SEMANTIC — 语义角色 (组件唯一可引用层)
     ========================================================== */

  /* 表面 surface */
  --surface-app:      var(--p-app-bg);
  --surface-card:     var(--p-paper-200);
  --surface-card-pop: var(--p-paper-100);
  --surface-field:    var(--p-paper-300);
  --surface-field-hover: var(--p-paper-100);
  --surface-row:      var(--p-paper-400);
  --surface-row-hover:#f2e4c5;
  --surface-sunken:   var(--p-paper-500);

  /* 文本 text */
  --text-primary:   var(--p-ink-900);
  --text-secondary: var(--p-ink-700);
  --text-muted:     var(--p-ink-500);
  --text-on-accent: var(--p-ink-on-brass);
  --text-link:      var(--p-info-ink);

  /* 描边 border */
  --border-default: var(--p-line-22);
  --border-soft:    var(--p-line-15);
  --border-strong:  var(--p-line-gold);
  --border-accent:  var(--p-brass-800);

  /* 强调/品牌 accent (主按钮) */
  --accent:         var(--p-brass-650);
  --accent-hover:   var(--p-brass-700);
  --accent-border:  var(--p-brass-800);
  --accent-soft:    var(--p-brass-100);

  /* 选中态 selected —— 全应用唯一画法：棕金实心 */
  --selected-bg:     var(--p-brass-650); /* #8b6737 实心 */
  --selected-fg:     var(--p-ink-on-brass);
  --selected-border: var(--p-brass-800); /* #765126 */
  --selected-bar:    var(--p-brass-600); /* 行/标签的左条·下划线，同色相 #9a7440/#98703a 家族收敛为一 */

  /* 交互态 interaction (统一 overlay/透明度，禁各写一套) */
  --state-hover:    rgba(139,106,55,.06);
  --state-active:   rgba(139,106,55,.12);
  --state-disabled-opacity:.45;

  /* 焦点 focus (全局一次，禁组件各写青边) */
  --focus-outline: 2px solid var(--p-focus-outline);
  --focus-offset:  2px;
  --focus-ring:    0 0 0 3px var(--p-focus-ring);

  /* 状态语义 status roles */
  --success:var(--p-success); --success-ink:var(--p-success-ink); --success-bg:var(--p-success-bg); --success-bar:var(--p-success-bar);
  --warning:var(--p-warning); --warning-ink:var(--p-warning-ink); --warning-bg:var(--p-warning-bg);
  --danger: var(--p-danger);  --danger-ink: var(--p-danger-ink);  --danger-bg: var(--p-danger-bg);
  --info:   var(--p-info);    --info-ink:   var(--p-info-ink);    --info-bg:   var(--p-info-bg);

  /* ==========================================================
     结构性 scale (原缺失，0 命中；原子化必需)
     ========================================================== */

  /* 间距 spacing (8 基准 + 细分，收敛散落 px) */
  --space-0:0; --space-1:2px; --space-2:4px; --space-3:6px; --space-4:8px;
  --space-5:12px; --space-6:16px; --space-7:20px; --space-8:24px; --space-9:32px; --space-10:48px;

  /* 圆角 radius (杀死不对称装饰圆角，统一扁平羊皮纸) */
  --radius-0:0;
  --radius-sm:2px;   /* 按钮/输入/chip/标签 */
  --radius-md:4px;   /* 卡片/面板 */
  --radius-lg:8px;   /* 对话框/抽屉 */
  --radius-pill:999px; /* 状态点/胶囊 */

  /* 阴影 elevation (收敛 128 处硬编码 box-shadow) */
  --shadow-0:none;
  --shadow-1:0 4px 12px rgba(90,65,29,.08);                 /* 卡片 */
  --shadow-2:0 9px 22px rgba(77,54,25,.12);                 /* 抬起/弹层 */
  --shadow-3:0 18px 48px rgba(60,42,18,.28);                /* 对话框 */
  --shadow-bevel:inset 0 1px 0 rgba(255,255,255,.5);        /* 顶部高光棱 */
  --shadow-focus:var(--focus-ring);

  /* 字号 fontSize (收敛 19 种，base 14) */
  --fs-2xs:9px; --fs-xs:10px; --fs-sm:11px; --fs-md:12px;
  --fs-base:14px; --fs-lg:17px; --fs-xl:20px; --fs-2xl:24px;

  /* 字重 fontWeight (收敛 560/650/680/720/800/900) */
  --fw-normal:500; --fw-medium:600; --fw-semibold:680; --fw-bold:800;

  /* 行高 lineHeight */
  --lh-tight:1.2; --lh-normal:1.5; --lh-relaxed:1.6;

  /* 层级 z-index */
  --z-base:1; --z-sticky:10; --z-dropdown:100; --z-drawer:500; --z-dialog:1000; --z-toast:1200;

  /* 动效 motion */
  --dur-fast:120ms; --dur-base:160ms; --dur-slow:240ms;
  --ease-standard:cubic-bezier(.2,.6,.2,1); --ease-out:ease;
  --transition-control:border-color var(--dur-base) var(--ease-out),
                       background-color var(--dur-base) var(--ease-out),
                       color var(--dur-base) var(--ease-out),
                       opacity var(--dur-base) var(--ease-out);
}
/* 说明：
   1) 已弃用/污染令牌一律删除：--cyan/--cyan-bright/--green/--sky-700/--art-right/--art-bottom/--official-*，
      以及 PatchTool.vue L646-647 / L729-730 / L886-895 三套本地重定义。选中一律 --selected-*，青蓝 #48c9df/#67e8f9 禁止入库。
   2) 无暗色主题分支：本应用只有浅色羊皮纸，删除 dark-theme 令牌层，杜绝"暗色皮 + 外壳强刷"双层。 */
```

## 原子组件库

### AppButton
- 作用：唯一按钮原子。以 variant/size prop 输出所有按钮，全部走 --accent/--surface/--radius-sm 令牌，禁止组件再写按钮样式。
- 变体/状态：variant: primary(棕金实心 --accent, 文字 --text-on-accent, hover --accent-hover) / secondary(纸面 --surface-field + --border-default 描边) / subtle(透明底, hover --state-hover) / danger(--danger) / link(无框, 文字 --text-link) / icon(方形图标钮)。size: sm(28px)/md(34px)。状态: hover/active/disabled(opacity var(--state-disabled-opacity))/loading。
- 取代：.action/.primary/.save/.apply-btn/.op-btn/.slot-btn/.plain-btn/.save-btn/.btn-batch/.btn-refresh/.btn-sort/.btn-action/.btn-green/.btn-purple/.btn-cyan/.btn-red/.btn/.btn-connect/.btn-disconnect/.btn-max/.btn-warn/.mode-btn/.language-button/.ed-max-btn/.add-btn/.write-btn/.primary-btn/.win-btn（10+ 按钮 class 家族全部收敛）

### AppSegmented
- 作用：分段/标签切换原子(受控 modelValue)。选中态唯一走 --selected-*，杜绝五套 tab 各自青/棕不一。
- 变体/状态：variant: solid(选中项 --selected-bg 棕金实心) / underline(选中项透明底 + --selected-bar 下划线)。状态: selected/hover/disabled。可带 count 徽标。
- 取代：.section-tabs+.mini-tabs / .runtime-tabs / .tabs+.tab+.tab-count / .tool-switcher / .mode-toggle+.mode-btn / .chapter-mode / .small-tabs（分段控件 5 套实现）

### AppCard
- 作用：容器卡片原子(可含 header/body/footer slot)。统一圆角 --radius-md、描边 --border-default、抬起 --shadow-1、顶棱 --shadow-bevel。
- 变体/状态：variant: default(--surface-card) / pop(--surface-card-pop + --shadow-2) / flat(无阴影)。state: selected(--selected-border 描边 + 左条 --selected-bar) / active(连接态, --success-bar 左条) / status(success/warning/danger 语义底)。
- 取代：.section/.save-card/.editor-card/.detail-panel/.memory-card/.trait-card/.loadout-card/.process-card/.list-panel/.editor-panel/.calibration-card/.custom-note-card/.language-panel（.section 别名在 9+ 组件各写一遍，圆角 8/12/16 与青/棕激活态全部统一）

### AppSection
- 作用：带标题+说明的内容区块原子(标题/副标题/操作区 + 主体)。组合 AppCard 使用，统一 section 标题排版与间距。
- 变体/状态：标题级别 h2/h3；可选 headerAction 插槽(右上按钮组)；dense/comfortable 间距。
- 取代：各组件手写的 .section-header/.section-title/.save-title/.editor-title/.header 结构与被 :deep 强刷的标题色，收敛为一处排版

### AppInput
- 作用：文本/数字输入原子。焦点态统一 --focus-outline(棕金)，数字自动挂 --font-data tabular-nums；禁止各组件写青焦点边。
- 变体/状态：type: text / number(tabular) / search(带放大镜) / password。带 prefix/suffix(单位) 插槽。状态: focus/disabled/invalid(--danger)。
- 取代：.number-input/.batch-input/.value-input/.text-input/.currency-input/.search-input/.filter/.ed-input/.rename-input(青边)/.countdown-input（输入实现全部分散，青焦点边统一改棕金）

### AppSelect
- 作用：可搜索下拉原子(BaseSelect)。泛型 modelValue(String internalId 或 Number hash 由 valueKey 决定)，内建 open/filter/highlight/键盘导航/外点关闭。合并两套逐行重复实现。
- 变体/状态：支持 option 富展示插槽(hex 色块/opt-tag/level)；hover 与 selected 视觉区分(hover=--surface-row-hover, selected=--selected-bar 左条 + 勾选);可 clearable。native 模式包装原生 <select>(od-select 场景)。
- 取代：CatalogSelect.vue + SigilMemoryPicker.vue(近乎逐行重复)合并为一；.od-select×3 / .select-input+.sigil-select / .ed-select / label select（~5 种下拉实现）

### AppChip
- 作用：标签/状态芯片原子。承载 selected/status 语义，选中一律棕金实心。LegalityIndicator 作为其预设。
- 变体/状态：tone: neutral / accent(选中 --selected-bg 实心) / success(legal) / warning(forced) / danger(impossible) / info(unknown, 静音青)。size: sm/md。可 removable。预设: <AppChip preset='legality'>。
- 取代：.chip/.state/.build-chip/.info-dot(青×4处)/.default-badge/.chara-chip/.slot-chip/.row-chip-tag/.safety-note/.switcher-tag + LegalityIndicator.vue 内联样式

### AppCheckbox / AppToggle
- 作用：勾选与开关原子。accent-color 统一 var(--accent) 棕金，消除 #9a7440 与 #67e8f9 两套 accent 分裂。
- 变体/状态：AppCheckbox(单选/全选/半选 indeterminate)；AppToggle(开关行, on=--accent)。状态: checked/disabled。
- 取代：散落 input[type=checkbox]{accent-color:#9a7440} 与 #67e8f9 两套；.toggle-row/.owned-toggle/.selection/.select-all

### AppField
- 作用：字段布局原子：label + 控件 + hint/单位。统一标签位置、间距(--space-*)与数量步进组合。
- 变体/状态：layout: stacked(上下) / inline(标签在左)。可含 quantity-combo(数量+步进按钮)、level-field(等级+上限提示)、suffix 单位。
- 取代：.field/.level-field/.field-group/.quantity-field+.quantity-combo(×2)/.ed-field/.number-combo/.batch-row 内字段拼装

### AppDialog / AppDrawer
- 作用：模态原子。ConfirmDialog(Promise 式 ask())为唯一确认框；AppDrawer 为侧滑抽屉。统一 --z-dialog/--shadow-3/--radius-lg。
- 变体/状态：AppDialog: variant default/danger; 具 Teleport + backdrop + 标题/正文/footer(cancel/confirm)。AppDrawer: side right; 头部+动作区+内容。
- 取代：复用 ConfirmDialog.vue 并删除 MiscTools.vue 自造 .confirm-dialog；SaveBackupDrawer 的 .backup-drawer 收敛为 AppDrawer

### AppGrid / AppRow
- 作用：布局原子。AppGrid=响应式卡片/槽位网格；AppRow=可选中列表行(左条选中态走 --selected-bar)。
- 变体/状态：AppGrid: cols=auto-fit minmax / 固定列; gap 走 --space-*。AppRow: selectable(selected=--selected-bar 左条+--surface-row)、hover=--surface-row-hover、可带前缀 dot/后缀操作。
- 取代：.card-grid/.slot-grid/.pick-grid/.language-grid/.preflight-grid + .row(×2)/.batch-row/.summon-row/.catalog-row/.existing-row/.ed-row/.snapshot-card 行选中

### AppStatusDot
- 作用：状态圆点原子。tone 决定颜色，统一 --radius-pill 与发光；替换四处同源小圆点。
- 变体/状态：tone: neutral/stable(success)/live(info)/calibrate(warning)/waiting(danger)。可选 glow。
- 取代：.state-dot/.status-light/.target-dot/.switcher-dot + 各处 .info-dot 的点形态

## 一致性原则
 - 单一事实源令牌：颜色/间距/圆角/阴影/字号/交互态只能引用 :root 语义令牌 var(--…)，组件内禁止出现裸 hex/rgba/px 字面量。primitive 原始调色板只在 :root 出现一次，semantic 层引用 primitive，组件只引用 semantic 层——三层单向依赖，绝不反向。
 - 令牌禁止后代重定义：任何选择器(尤其 .app-window/.tool-panel)不得重写 --cyan/--green/--panel/--line 等令牌。删除 PatchTool.vue L646-647 深色、L729-730 羊皮纸、L886-895 atlas 三套本地重定义与 --official-* 影子令牌；var() 解析值必须可静态推断，不随子树漂移。
 - 选中态唯一画法=棕金实心：selected 一律 --selected-bg #8b6737 实心 + --selected-fg #fff9e9 + --selected-border #765126；列表行/标签的次级选中用 --selected-bar 同色相左条或下划线，色相始终是 brass。青蓝 #48c9df / #67e8f9 / #4ba8b6 等一律禁止出现在选中或焦点，从调色板中彻底移除。
 - 语义分离不复用：selection(brass) ≠ info(teal) ≠ success(green) ≠ accent。--info 静音青仅表达'当前值/信息/合法'，永不表达选中；--success/--warning/--danger 各有独立令牌，禁止用 --green 兼表选中(此前正是 --green 被 atlas 染蓝的根因)。
 - 焦点态全局一次：:focus-visible 统一 var(--focus-outline) 棕金 outline(+可选 var(--focus-ring))，在 style.css 定义一次。组件不得各写 :focus{border-color:青}，PatchTool 内部三处相反 focus 规则(青 outline→outline:0→金 ring)必须删除只留一条。
 - 消灭 !important 跨组件皮肤覆盖：删除 PatchTool.vue 末尾 5 个追加 <style> 块的 1015 处 !important 与 1186 处 :deep()。样式真相回到组件本体(单层)：每个组件用令牌自身即渲染成正确的羊皮纸，外壳不再'重刷'子组件，改一处不会被另一处压过。
 - 原子优先，禁止复制粘贴：新增控件必须复用原子组件；同一段样式不得在多组件重复(消除 .od-select×3、.slot-btn×3、.text-input×2、.info-dot×4、.btn-batch/refresh/sort×3 等)。发现重复即上抽为原子或原子变体。
 - 一个概念一个组件：可搜索下拉只保留一个 AppSelect(合并 CatalogSelect+SigilMemoryPicker)；确认框只用 ConfirmDialog(删 MiscTools 自造 .confirm-dialog)；容器只用 AppCard(收敛 .section 全部别名)。
 - 圆角/字号/间距只取 scale：禁止装饰性不对称圆角(2px 7px 2px 7px 等)与散落 px/font-size。控件 --radius-sm(2px)、卡片 --radius-md(4px)、弹窗 --radius-lg(8px)；字号仅取 --fs-* 档；间距仅取 --space-* 档。
 - 阴影/交互态令牌化：box-shadow 只用 --shadow-1/2/3/bevel；disabled 统一 --state-disabled-opacity(.45)；hover/active 统一 --state-hover/--state-active overlay，不再各组件自定不同透明度。
 - 单一浅色主题：本应用只有羊皮纸浅色，删除所有暗色主题基础样式与 dark-theme 令牌层，杜绝'子组件暗色皮 + 外壳强刷'的双层杂糅；组件从一开始就直接写成羊皮纸令牌。
 - 死代码/死令牌先清后建：重构前先裁剪 0 引用令牌(--sky-700/--cyan-bright)、失效定位系统(--art-right/--art-bottom)与已弃用青色 option:checked 残留，避免新系统继承噪声。
 - 适配约束不破坏：全部令牌内联在 style.css :root(Wails 无外部 CDN)；原子组件用 Vue3 <script setup> + 组件直写中文文案；不引入运行期换肤，保证现有 i18n 运行时 DOM 翻译不受影响。

## 迁移策略
渐进式「绞杀者(strangler)」迁移，绝不整体重写。核心是三条并行推进、互不阻塞：①先把新的三层令牌(primitive→semantic)【追加】进 style.css:root，保留旧令牌名作别名指向新 primitive（--gold/--line/--panel-solid/--text-* 等继续可用），做到零视觉变化地让新系统全局可用；②在 components/ui/ 新建原子组件，每个原子只引用 semantic var(--…)、自身即渲染成正确羊皮纸，不依赖外壳强刷；③用「按工具门控外壳皮肤」的开关，让 PatchTool 末尾 5 个 <style> 块的 :deep !important 皮肤只作用于【未迁移】的工具，已迁移工具由自身令牌渲染——从而一次迁一个工具、其余像素不动。样式真相逐步从「组件暗色本体 + 外壳羊皮纸覆盖」双层收敛回「组件本体单层」。皮肤块、死令牌、旧别名一律最后（确认 0 引用后）删除。关键约束：i18n 是 main.js installI18nObserver 的运行期 DOM MutationObserver 翻译(配 i18n-ui.js 整句词典)，原子必须把中文文案保留为字面文本节点，禁止用 CSS content/图片/JS 计算文案，否则英文界面失效；Wails 无 CDN，令牌与字体全部内联在 style.css。

## 迁移步骤
 - 步骤0 立基线：迁移前先对约14个工具页逐页截图(浅色羊皮纸态)，作为每步的视觉回归对照。这是判断“令牌收敛属有意收拢 vs 误伤”的唯一依据。
 - 步骤1 追加令牌层(零视觉变化)：把已设计的 primitive+semantic 令牌粘进 style.css:5-26 的 :root，但【不删】旧令牌——在同一 :root 里把旧名映射到新 primitive 作别名：--gold:var(--p-brass-500)、--line:var(--p-line-22)、--panel-solid:var(--p-paper-100)、--text-primary:var(--p-ink-900)、--cyan 暂别名 var(--info)、--green 暂别名 var(--success)。绝不整体替换 :root。此步后所有既有 var() 仍可解析，界面不变。
 - 步骤2 消灭“蓝色选中”根因(高价值/低风险)：删除 PatchTool.vue:888-890 的 --cyan:#48c9df/--green:#48c9df/--official-cyan:#48c9df 三行(以及 646-647 已被 730 覆盖的死暗色本地令牌集)。删后 .app-window 上 730 行的真值(--cyan:#3aa9b3、--green:#4c9c76)重新生效，凡 var(--green)/var(--cyan) 不再被染成青蓝、不随子树漂移。注意：PatchTool 外壳自身 chrome(.action/.nav-item/.state-dot/.calibration-card 等 646-762)依赖 728-730 这套【本地】羊皮纸令牌，故 730 整块要保留，只摘除 888-895 的污染与 --official-* 影子令牌(改用 :root 的 --gold/--panel/--text)。逐页截图核对无回归。
 - 步骤3 装“皮肤门控”开关(绞杀者机关)：给 PatchTool.vue:549 的 <main class="tool-panel"> 加 :class="{'skin-legacy': !migratedTools.has(activeTab)}"，并对 644/1746/1803/1818/1835 五个 <style> 块做一次机械替换：把选择器里的 `.tool-panel :deep(` → `.tool-panel.skin-legacy :deep(`、`.tool-panel[data-tool=` → `.tool-panel.skin-legacy[data-tool=`；纯布局的 `.tool-panel{...}`(684/797 等)保持不加类。migratedTools 初值为空 → 行为完全不变。此后每迁一个工具，只需把工具名加入 migratedTools，皮肤即对它失效。
 - 步骤4 造核心原子(components/ui/，只用令牌、script setup、中文文案走 slot/文本节点)：按复用杠杆排序先做 AppButton(收敛 10+ 按钮族)、AppCard+AppSection(收敛 .section 9+ 别名)、AppInput(收敛 .*-input 多套，焦点统一棕金)、AppSelect(把 CatalogSelect.vue 139 行与 SigilMemoryPicker.vue 150 行合并为一，用 valueKey 泛化 String internalId/Number hash)。每个原子在隔离页面对照基线验收羊皮纸外观后再用。
 - 步骤5 造其余原子：AppSegmented(收敛 5 套 tab)、AppChip+AppStatusDot(含 legality 预设，收敛 .info-dot×4/.chip/.build-chip)、AppCheckbox/AppToggle(accent-color 统一 var(--accent) 棕金)、AppField、AppGrid/AppRow(左条选中走 --selected-bar)、复用 ConfirmDialog.vue 并封 AppDrawer。
 - 步骤6 逐工具迁移(每个组件一次闭环)：a)模板用原子替换自造 class；b)删掉该组件 scoped 里的暗色本体 CSS 与 #67e8f9/#a5b4fc/#4ade80/#48c9df 硬编码；c)把工具名加入 migratedTools；d)从 PatchTool 五块皮肤里删掉该工具专属的 :deep 规则；e)逐页截图 diff 对照基线。因门控已排除已迁移工具，其它工具保持像素稳定。
 - 步骤7 按 order 给出的优先级依次迁移各业务组件；每完成一批就回归验收一次，避免长分支。
 - 步骤8 迁移外壳自身：待 migratedTools 覆盖全部工具后，整块删除五个 legacy 皮肤 <style>，把 PatchTool 剩余 chrome(.win-btn/.nav-item/.tool-switcher/.build-chip/.calibration-card/.patch-card/.backup-policy)改用原子+令牌，并把外壳选中/连接态从青蓝改棕金(--selected-*/--success-*)；.tool-switcher 在 681/719/942/1294/1490/1570/1682/1762/1852 的~9 次重复定义收敛为一处。
 - 步骤9 清死代码/死令牌(先确认 0 引用再删)：--sky-700/--cyan-bright(0 引用)、--art-right/--art-bottom 失效定位(812/815-832，真正生效的是 1718-1733 的 --ax/--ay/--ah)、--official-*、SigilGenerator.vue:804-826 与 WrightstoneGenerator.vue:404-406 已弃用的 .*-select option:checked 青色残留、MiscTools.vue:848/995 自造 .confirm-dialog(改用 ConfirmDialog)。最后 grep 确认无引用后删除步骤1 的旧令牌别名(--cyan/--green/--gold/--line 等)。
 - 步骤10 立护栏防回归：加一条 CI/lint grep 门禁——components/ 下出现 #48c9df/#67e8f9/#4ba8b6、组件 <style> 内出现 !important、components/ 内出现裸 hex/px(仅 style.css 允许)即失败；并在 CONTRIBUTING 写明‘组件只引用 semantic var(--…)、任何后代选择器(.app-window/.tool-panel)禁止重定义令牌’。

## 先迁顺序
优先级=价值/风险比：先做步骤1-3(令牌层+门控+去污染)——直接消灭用户最在意的‘蓝色选中’、解锁全部后续、几乎零风险，最高杠杆。原子按复用频次做：AppButton→AppCard/Section→AppInput→AppSelect 优先(命中 537 处 !important、4 路令牌冲突、10+ 按钮族、2 个重复下拉这几个最痛点)。组件迁移顺序按‘去重收益×简单度’：①先迁 SaveEditor+CharaStats——最小且 .slot-btn/.plain-btn/.save-btn/.row 几乎逐字同源，用来跑通原子模式、去重面最小；②AppSelect 合并 CatalogSelect+SigilMemoryPicker(两文件近乎逐行重复→一个)，单点去重收益最大且解锁生成器；③生成器对 SigilGenerator+WrightstoneGenerator——.text-input/.select-input/.quantity-combo/.field/.btn-* 大段逐字重复，且携带待删的青色 option:checked 死码；④运行时三件套 MiscTools+OverLimit+MonsterEnhance——.od-select×3/.btn-batch/refresh/sort×3/.memory-card/.info-dot 重复最密，MiscTools 顺带删自造确认框，且能摘掉最大一坨 runtime-tabs/memory-card :deep 皮肤；⑤SummonEditor(单文件内 .section/.primary 自我覆盖3次)与 SigilMemoryGenerator(.rename-input 青色泄漏)；⑥已是羊皮纸原生的 LoadoutViewer/LoadoutEditor 放最后(外观本就对，迁移只为统一原子并去掉 #3f7d5c 硬编码规避)；⑦PatchTool 外壳 chrome + 删皮肤块 + 清死令牌 最后做。理由：把‘样式杂糅’根因(令牌污染、双层皮肤、最密集重复)在前 60% 精力内解决，外壳重写边际价值低、可机会性排期，不作阻塞项。

## 风险
 - 整体替换 :root 会瞬间打爆所有 var(--gold)/var(--line)/var(--cyan)/var(--green) 引用(遍布各组件与 PatchTool 自身 1300 行 CSS)。必须追加式加令牌+旧名别名，永不 wholesale 替换 :root(步骤1)。
 - PatchTool 外壳自身 chrome(.action/.nav-item/.state-dot/.calibration-card/.patch-card 等 646-762)依赖 .app-window 上 728-730 这套【本地】羊皮纸令牌(--panel/--line/--text/--amber/--red/--cyan/--green)。若整块删本地令牌集，外壳会丢失羊皮纸皮。只能删 888-895 污染层，730 真值层要保留(或先把这些值真值化搬进 :root 再删)。
 - i18n 是运行期 DOM MutationObserver 翻译(main.js installI18nObserver + i18n-ui.js 整句词典)。原子必须把中文保留为字面文本节点；禁止把文案搬进伪元素 content、图片或 JS 计算标签，否则英文界面回退成混串。同时 utils/uiScale 也钩 DOM，迁移后需验证不受影响。
 - 原生 <select> 的 option:checked 无法被父级 :deep 覆盖(现存 SigilGenerator #67e8f9 青色泄漏即因此)。AppSelect 的 native 包装路径要么避免原生多选列表、要么接受系统绘制的选中色——迁移生成器列表路径时须专门验证。
 - 迁移期新原子样式与 legacy 皮肤共存，最终取值仍由源码顺序+特异性决定。skin-legacy 门控类必须验证对已迁移工具‘完全’排除(尤其无 !important 的 accent-color:#9a7440 只有 (0,2,1) 特异性，脆弱)；每步都要截图 diff 兜底。
 - 羊皮纸把数十种近似棕收敛成~6 个令牌，部分页面会有轻微色移。必须靠步骤0 的逐页基线截图区分‘有意收拢’与‘误伤’，否则无法证明未破坏功能。
 - 范围蔓延风险：PatchTool 外壳(win-btn/nav/tool-switcher/立绘 --ax/--ay/--ah 双定位系统)最缠结且用户价值低。须推迟到最后，别让外壳重写阻塞高价值的组件去重。
 - Wails 无外部 CDN：令牌/字体必须始终内联在 style.css，原子须自包含，不得引入外部样式或运行期换肤。

## 工作量
粗估工作量(单人 dev-day)：步骤1-3 令牌层+门控+去污染 ≈0.5-1 天，ROI 最高(直接修掉用户头号‘蓝色选中’bug、解锁全部后续、近零风险)。约 12 个原子 ≈3-5 天(AppSelect 最重，要合并两个组件并做 valueKey 泛化)。约 14 个业务组件迁移 ≈0.5-1 天/个(生成器/运行时组件大、SaveEditor/CharaStats 小)合计 ≈8-12 天。外壳 chrome+删皮肤块+清死令牌 ≈2-3 天。全量迁移总计约 15-22 dev-day。是否值得：前半程明确值得——令牌层+去污染+三大去重(下拉合并、生成器对、运行时三件套)约占‘样式杂糅’痛点的 60%(537 处 !important 皮肤链、4 路令牌冲突、10+ 按钮族、2 个重复下拉)，却只需约 40% 精力，做完后头号 bug 消失、可维护性显著改善。建议按 order 分阶段交付，在迁完‘运行时三件套’后停下复盘：届时根因基本解除；外壳整改边际价值低，作为机会性任务排期而非阻塞项。若资源紧张，最低可交付集=步骤1-3 + AppButton/AppCard/AppInput/AppSelect + 下拉合并 + 生成器对去重，即可拿到绝大部分收益。
