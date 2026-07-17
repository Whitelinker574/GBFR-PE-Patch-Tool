# Composer 原子组件清单

一套自渲染的浅色羊皮纸原子类，定义在 `frontend/src/ui.css`，全局引入（`main.js`）。
**全部引用 `style.css` 的语义令牌，零硬编码色、零 `!important`。** 组件直接用这些类，
不再依赖 PatchTool 的 `:deep !important` 皮肤覆盖 —— 因此「组件侧一改就生效、不会被覆盖」。

## 为什么不会再被覆盖
PatchTool 皮肤层那 500+ 个 `:deep(button/input/select/textarea) { …!important }` 宽泛规则，
已全部加上 `:not(.ui-btn)`/`:not(.ui-input)`/`:not(.ui-select)` 排除。凡是用 `.ui-*` 原子类
构建的元素，都从 `ui.css` 渲染、皮肤层碰不到它。旧标记不动照常工作，新原子标记干净自渲染。

## 设计令牌（单一事实源，`style.css` :root）
两层：**primitive**（`--p-paper-*`/`--p-brass-*`/`--p-ink-*`/`--p-*` 状态色，仅此出裸值）
→ **semantic**（组件唯一可引用层）：
- 表面 `--surface-app/card/card-pop/field/field-hover/row/row-hover/sunken`
- 文本 `--text-primary/secondary/muted/on-accent/link`
- 描边 `--border-default/soft/strong/accent`
- 强调 `--accent/accent-hover/accent-border/accent-soft`
- **选中（全应用唯一画法=棕金实心）** `--selected-bg(#8b6737)/fg/border(#765126)/bar`
- 交互 `--state-hover/active/disabled-opacity`、焦点 `--focus-outline/ring`
- 状态 `--success/warning/danger/info`（各含 `-ink`/`-bg`）
- 尺度 `--space-1..9`、`--radius-sm/md/lg/pill`、`--shadow-1/2/3`、`--fs-2xs..xl`、`--fw-*`、`--z-*`、`--dur-*`

## 原子类清单

| 类 | 作用 | 变体 / 状态 |
|---|---|---|
| `.ui-card` | 卡片/面板容器 | 配 `.ui-panel`（内边距+竖向 gap） |
| `.ui-section-title` | 区块标题 | 内 `<small>` 为副标题 |
| `.ui-btn` | 按钮 | `.is-primary`（棕金实心）/`.is-danger`/`.is-ghost`/`.is-subtle`（链接式）/`.is-sm`；`:disabled` |
| `.ui-input` | 文本/数字输入 | `:hover`/`:focus-visible` 已含；`::placeholder` |
| `.ui-select` | 原生下拉 | 同上 |
| `.ui-field` + `.ui-field-label` | 字段包裹 + 标签 | 标签内 `<em>` 为辅助计数 |
| `.ui-chip` | 可选胶囊（角色/因子） | `.is-on` = 棕金实心选中 |
| `.ui-tab` | 顶部/区块标签页 | `.is-on` = 棕金下划线 |
| `.ui-seg` + `.ui-seg-btn` | 分段控件 | `.is-on` = 棕金实心 |
| `.ui-tag` | 状态小标签 | `.is-ok`/`.is-warn`/`.is-danger`/`.is-info` |
| `.ui-row` + `.ui-list` | 列表行/列表 | `.is-on` = 棕金左条 |
| `.ui-hint` / `.ui-empty` / `.ui-divider` / `.ui-grid` | 提示/空态/分隔/网格 | — |

## 用法示例
```vue
<button class="ui-btn is-primary">写入</button>
<input class="ui-input" v-model="name" />
<select class="ui-select"><option>…</option></select>
<button class="ui-chip" :class="{ 'is-on': on }">伊欧</button>
<div class="ui-card ui-panel"> … </div>
```

## 迁移状态（诚实标注）
- **已建立**：ui.css 原子层 + 令牌单一事实源 + 原子逃出皮肤层的 `:not(.ui-*)` 排除。
- **已用原子渲染 / 羊皮纸自渲染**：LoadoutViewer、LoadoutEditor（含模拟器面板）、CatalogSelect、SigilMemoryPicker、以及新写的模拟器/侧栏吉祥物/首页。
- **仍靠 PatchTool 皮肤层再皮肤化的旧组件**（暗色基底 + 皮肤覆盖，当前渲染正常，但未逐个改用原子类）：
  ProgressionEditor、SigilGenerator、WrightstoneGenerator、CharaStats、SaveEditor、SummonEditor、
  OverLimit、MiscTools、MonsterEnhance、SigilMemoryGenerator。
  这些页面「用原子类重写内部控件」是后续工作：把它们的 `.text-input/.btn-*/.section` 等换成 `.ui-*`，
  即逐页脱离皮肤层。因逐个改动量大、且当前显示已正常，未在本轮一次性全改（避免改坏留给你验收时是坏的）。
