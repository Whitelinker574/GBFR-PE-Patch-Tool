# 游戏 2.0.5 更新与上游适配研究（2026-08-29）

## 结论摘要

- 官方于 2026-08-28 发布 `Ver. 2.0.5`。这不是只改了一个地址的小补丁：除战斗与镜头问题外，还调整了菲迪埃尔的战斗数值、把 MSP 持有上限从 `999,999` 提高到 `9,999,999`、提高多项任务的 MSP 奖励，并修复了调查宝物库存、小钳蟹数量及奖杯状态。[Cygames 简中公告](https://relink-ragnarok.granbluefantasy.com/chs/updates/401/)、[官方 Steam 公告](https://store.steampowered.com/news/app/881020/view/1842212951302112)
- 上游 `BitterG/GBFR-PE-Patch-Tool` 已发布 [`v1.10.3`](https://github.com/BitterG/GBFR-PE-Patch-Tool/releases/tag/v1.10.3)，标签与 `master` 均指向提交 [`8b59dde6f66cab2b9cc10c28a428871cd75f8ce5`](https://github.com/BitterG/GBFR-PE-Patch-Tool/commit/8b59dde6f66cab2b9cc10c28a428871cd75f8ce5)。它明确迁移了物品数量更新、玩家受伤倍率、两条 OD 累计路径及自动聊天地址；这证明 2.0.5 对多组运行时代码位置造成了实质影响。
- 官方公告没有宣布新增角色、因子、祝福、召唤石或存档格式版本；因此不能仅凭公告声称这些表结构改变。但菲迪埃尔公式、MSP 合法范围、调查库存/小钳蟹修复和“专精超限装备”合法性判断必须重新核对。
- 运行时功能不能只把版本字符串改成 2.0.5。每个 Hook、指针链和 AOB 都应对 2.0.5 游戏文件单独验证原字节、唯一性、写后回读和恢复路径。

## 一、官方 2.0.5 公告逐项结论

官方一手来源：

- [Cygames 简中：ver.2.0.5 版本更新日志](https://relink-ragnarok.granbluefantasy.com/chs/updates/401/)
- [Cygames 英文：Ver. 2.0.5 Update Information](https://relink-ragnarok.granbluefantasy.com/en/updates/401/)
- [Steam：Ver. 2.0.5 Update Information](https://store.steampowered.com/news/app/881020/view/1842212951302112)，新闻 GID `1842212951302112`
- [Steam 官方新闻 API（App 881020）](https://api.steampowered.com/ISteamNews/GetNewsForApp/v2/?appid=881020&count=100&maxlength=0&format=json)
- [Cygames Endless Ragnarok 更新索引](https://relink-ragnarok.granbluefantasy.com/en/updates/)

官方此前在 2026-08-14 的 [Notice about the Next Update](https://relink-ragnarok.granbluefantasy.com/en/news/400/) 中说明正在审查已知问题，并调整角色玩法、MSP 获取和游戏平衡；2.0.5 的实际公告与该预告范围一致。

### 1. 战斗与角色

- 修复加兰查在进入 `Deathmatch Finale` 或 `Wild Showman` 达到最高层时，奥义槽未满也能发动奥义的问题。
- 菲迪埃尔的 `Insight: Finishing Touch` 所授予的 `Enhanced Combo Finishers` 最高层数由 `10` 降为 `5`。
- 菲迪埃尔未获得 `Enhanced Combo Finishers` 时，魔法球攻击力被下调；随层数提升的伤害成长被重新调整，最高层伤害约与调整前相同。
- 工具影响：角色配装推荐、动作系数/伤害模型及技能说明若包含菲迪埃尔，不能沿用 2.0.4 的精确数值。公告未给出每层系数，必须从 2.0.5 数据表或实测重新标定，不能猜值。

### 2. 任务与镜头

- 修复 `The Not Forgotten Sky` 中 Boss 永久离开可攻击范围、导致任务软锁的问题。
- 修复在线合作游玩 `New World Order`、`On the Threshold of The World` 时的间歇性镜头失效。
- 修复 The World 使用 `Celestial Sphere` 后镜头异常。
- 修复 Beelzebub 在 `Let Chaos Reign` 或 `On the Threshold of Chaos` 使用 `Hail to the King` 后镜头异常。
- 工具影响：镜头工坊若 Hook 同一相机更新链，应重点回归上述任务、技能和在线场景；不能把官方修复后的镜头状态误当成工具自己的恢复结果。

### 3. 调查库存、存档修复与奖杯

- 修复一种会把限量 Conflux 宝物包库存错误降为 `0` 的情况：玩家在调查开始前查看宝物包，随后未获得任何 boon 就退出调查。
- 官方按玩家是否已拥有 `Chaos Refinium`、`Mythic Artifacts` 及是否已锻造相应武器，分条件恢复宝物包库存；已持有不可出售/不可移除关键素材的玩家不会再次获得对应宝物包。
- 修复关键物品中的已救小钳蟹数量与章节选择记录不一致；缺失的小钳蟹会在与谢洛卡特交互时补回库存。
- 修复 `Demon King` 奖杯条件错误应用到整次调查、而非仅 Beelzebub Conflux 战的问题。
- 修复 `My Evil Eye Approves!` 的两类奖杯问题，涉及领取芙劳/菲迪埃尔 DLC 专属因子，以及一个存档槽的解锁状态错误影响其他槽。
- 工具影响：存档编辑、存档差异和复制功能要保留 2.0.5 官方修复后的库存/奖杯状态，不应以 2.0.4 默认值覆盖；跨槽复制奖杯状态尤其需要隔离。公告本身没有公开字段位置，仍需由 2.0.5 解包表和真实存档前后对照确认。

### 4. UI、专精与 MSP

- 修复通过特定菜单操作装备超过上限的 master traits 的问题。
- MSP 持有上限从 `999,999` 提高到 `9,999,999`。
- 工具影响：所有离线存档编辑、运行时编辑、导入规范化和输入校验中的 MSP 上限要统一更新；不能只改一个输入框。专精/装备合法性检测不能继续把旧版菜单漏洞产生的超限状态当作 2.0.5 天然合法状态。

### 5. 任务 MSP 奖励

官方调整如下：

| 难度 | 任务 | 旧值 | 2.0.5 |
| --- | --- | ---: | ---: |
| Chaos++ | The Myths Are Real | 30,000 | 50,000 |
| Chaos++ | In the Wrong Hands | 30,000 | 50,000 |
| Chaos++ | The Tempest Rises | 30,000 | 50,000 |
| Chaos++ | Reclusive Vengeance | 30,000 | 50,000 |
| Chaos++ | Four Dragons of the Apocalypse | 30,000 | 140,000 |
| Chaos++ | Rock Around the Clock | 30,000 | 75,000 |
| Chaos++ | An Act of Automagod | 30,000 | 75,000 |
| Chaos++ | Beneath Primeval Wings | 30,000 | 75,000 |
| Chaos++ | Eternal Pride | 30,000 | 75,000 |
| Defy Infinity | 全部任务 | 50,000 | 250,000 |

工具影响：任务资料、收益展示、奖励倍率预览和基准测试若内置旧 MSP 值，需要以 2.0.5 表重新生成；全局奖励倍率还需确认是在官方新基数上乘算，而不是继续套用旧基数。

## 二、上游 v1.10.3 的 2.0.5 适配证据

上游发布页只概括为“修复游戏 2.0.5 更新导致的失效功能，并移除直接设置药水，改用材料不消耗”。逐提交查看后，可确认以下变化。

### 1. 物品数量更新 / 材料不消耗 / 小钳蟹

提交 [`1a68aa7c26dd70ac81a49b81fb1970cd8dfff799`](https://github.com/BitterG/GBFR-PE-Patch-Tool/commit/1a68aa7c26dd70ac81a49b81fb1970cd8dfff799)：

- 共享物品数量更新指令 RVA 从 `0x34F8F1` 移到 `0x34F8C1`。
- 原指令仍为 `41 01 76 04 4C 89 E1`。
- 上游新增固定 RVA 原字节校验和唯一 AOB 回退，并把同一入口用于材料不消耗与小钳蟹数量功能。

可能影响：材料不消耗、库存数量改写、药水替代方案及任何复用该共享指令的功能。

### 2. 玩家受伤倍率

提交 [`d0457cb0c31d09e0e1ccd10b1f67b4596742c1fa`](https://github.com/BitterG/GBFR-PE-Patch-Tool/commit/d0457cb0c31d09e0e1ccd10b1f67b4596742c1fa)：

- 最终伤害结算 Hook 从 `0x1FB878E` 移到 `0x1FB890E`。
- 玩家识别所用 vtable slot 2 从 `0x9CB970` 变为 `0x9CB360`。
- AOB 扩展为包含前置 `mov eax,[rsi+D4]` 的 11 字节序列，再以 `+6` 落到比较指令，降低短特征误命中风险。

可能影响：玩家/全队受伤倍率、实体归属识别，以及依赖相同玩家对象/vtable 的其他运行时功能。

### 3. 普通与 Beelzebub 类 Boss 的 OD 累计

提交 [`729acf6b51a1e20e94ad98d3ab1612cb1438d594`](https://github.com/BitterG/GBFR-PE-Patch-Tool/commit/729acf6b51a1e20e94ad98d3ab1612cb1438d594)：

- 上游源码把常规 OD 入口从 2.0.4 的 `0x23C6DF0` 更新为 2.0.5 的 `0x2C6FD0`，并继续使用唯一签名 `80 79 50 00 74 13 48 03 51 18`。
- Beelzebub 类 Boss 的第二条内联累计路径从 `0x2B3F77E` 移到 `0x2B3F92E`。
- 对第二条路径改为校验完整 19 字节原块后再安装 Hook，发现未知字节时失败关闭。

可能影响：OD 变化率必须分别验证常规 Boss 与 Beelzebub 类 Boss，不能只测一条路径后宣布完成。上述地址是上游源码证据，仍应在本机 2.0.5 EXE 中独立校验。

### 4. 自动聊天

提交 [`b81bf1bd7dd50bb2e1cc27a5a74a7542d4aeb5b7`](https://github.com/BitterG/GBFR-PE-Patch-Tool/commit/b81bf1bd7dd50bb2e1cc27a5a74a7542d4aeb5b7)：

- `sendMessage` 入口从 `0x9049F0` 移到 `0x904D30`。
- Manager slot 全局地址从 `0x7C23460` 移到 `0x7C236E0`。
- 上游保留 AOB 作为固定地址失效后的回退。

可能影响：自动消息、快速短语或任何复用 HUD/聊天管理器对象的功能。

### 5. 药水编辑被上游移除

提交 [`8b59dde6f66cab2b9cc10c28a428871cd75f8ce5`](https://github.com/BitterG/GBFR-PE-Patch-Tool/commit/8b59dde6f66cab2b9cc10c28a428871cd75f8ce5)：

- 上游删除复活药水和群疗药水的直接指针链读写 API 与 UI。
- 上游把用户流程改为启用“材料不消耗”来获得无限药水效果。

这项提交证明上游不再维护直接药水链，但不能单凭删除代码断言药水在 2.0.5 中不存在或两种实现语义完全相同。若空域工坊继续保留直接药水数量编辑，仍需在实际副本内验证新链、数量字段、读写回读和消耗行为；无法验证时应避免写入未知地址。

## 三、v1.10.2 到 v1.10.3 的其余上游变化

[`v1.10.2...v1.10.3` 比较](https://github.com/BitterG/GBFR-PE-Patch-Tool/compare/v1.10.2...v1.10.3) 共 8 个提交。除上述五个 2.0.5 适配提交外，还有三项较早变化：

- [`65790a7c6cf942458c71ee806240b184b956256a`](https://github.com/BitterG/GBFR-PE-Patch-Tool/commit/65790a7c6cf942458c71ee806240b184b956256a)：为自动聊天加入 AOB/外部调用基础。
- [`a1b315118fcd95c2d9fa7d2f639f98e96d328fc4`](https://github.com/BitterG/GBFR-PE-Patch-Tool/commit/a1b315118fcd95c2d9fa7d2f639f98e96d328fc4)：参考 Relink Logs 调整因子合规检测，提交说明明确承认“减少误判，但会有遗漏”。
- [`9dd9942ba033d2093d9336bd4a6e2304ee43472c`](https://github.com/BitterG/GBFR-PE-Patch-Tool/commit/9dd9942ba033d2093d9336bd4a6e2304ee43472c)：修复召唤石编辑错误共用角色连接对象，导致缺少部分角色的新存档无法使用功能。

这些不是官方 2.0.5 更新内容，但属于上游最新发布差异，仍值得与当前实现对照。

## 四、上游 PR 与 Issue 状态

- 截至 2026-08-29，上游 `master` 与 `v1.10.3` 同为 `8b59dde6...`；本轮 2.0.5 适配均直接提交到 `master`，没有对应的新 PR。
- 最新 PR 仍是已合并的 [#38：因子/祝福生成改为纯 AOB](https://github.com/BitterG/GBFR-PE-Patch-Tool/pull/38)，于 2026-08-01 合并；当前没有开放 PR。
- [#16：因子副词条合规检测](https://github.com/BitterG/GBFR-PE-Patch-Tool/issues/16) 仍开放，说明上游当前合法性规则并非完整真值。
- [#41：召唤石因子等级修改被限制](https://github.com/BitterG/GBFR-PE-Patch-Tool/issues/41) 已关闭，但讨论只包含社区试验，且最后没有形成完整游戏表证据；不能把评论中的等级结论直接固化为 2.0.5 规则。
- [#43：召唤石词条修改会导致部分召唤石不可用](https://github.com/BitterG/GBFR-PE-Patch-Tool/issues/43) 最终由报告者确认是使用其他修改器造成异常，并非上游召唤石编辑本身的问题。

## 五、建议的 2.0.5 排查顺序

1. 以本机 2.0.5 EXE 的哈希为基线，对所有运行时能力执行原字节/AOB 唯一性检查；优先覆盖物品数量、受伤倍率、常规 OD、内联 OD、聊天管理器。
2. 重新核对所有硬编码版本门槛、游戏版本显示、DLL/Go 双侧补丁表，防止只更新一侧。
3. 对 MSP 的离线编辑、实时编辑、导入、默认值、输入上限、英文文案和回读测试做全链路更新到 `9,999,999`。
4. 从 2.0.5 解包数据重新生成或核对任务 MSP 奖励，并确认奖励倍率以官方新基数计算。
5. 单独核对菲迪埃尔的技能层数、魔法球倍率和优化器角色模板；没有数据表精确值时标记未知，不填猜测值。
6. 用复制存档对照调查宝物库存、小钳蟹、奖杯和多存档槽状态；保留官方 2.0.5 自动修复结果。
7. 回归镜头工坊在官方公告点名的四组任务/技能场景，尤其包含在线合作切场景。
8. 最后再跑因子、祝福、召唤石、实时编辑、存档写入、虚拟因子和其他 Hook 的全量 2.0.5 回归；官方未点名不等于二进制入口未变。

## 六、本机 2.0.5 文件与运行时核查

本节使用本机 Steam 正式更新文件和与 2.0.3/2.0.4 相同的 GBFRDataTools 2.0.0 流程复核；它补充官方与上游证据，不代表其他平台文件身份。

| 项目 | 2.0.5 结果 |
| --- | --- |
| Steam 构建 | Build ID `24719688` |
| 游戏 EXE | `123,517,408` bytes；SHA-256 `7189B958FF0FE5238CEA28A2939FFDAD6E3A9ACB14DD274A9FCC8E7E275BD175`；文件版本 `2.0.5` |
| `data.i` | `13,510,144` bytes；SHA-256 `4FA5FDE9A94D3F995CB3DCFADB1238DCA739CDD313C15BD59109E9DBF3385802` |
| 全量表差分 | 2.0.3/2.0.4 与 2.0.5 均提取 `2,120` 份 `system/table` 文件；没有新增或删除，仅 `reward_point.tbl` 与 `skillboard_effect_action_parts.tbl` 改变 |
| 掉落部署输入 | 应用实际使用的 11 张召唤石、祝福、因子和普通物品掉落表全部与既有内置原表逐字节一致 |
| 运行时目录 | 60 个功能的 83 个站点全部唯一命中；仅 `GBFR_PATCH_014_1` 与 `GBFR_PATCH_035_1` 需要 2.0.5 原字节变体 |
| 内置运行时 | `patch_core.dll` 源码中的 36 条 AOB 字面量全部在 2.0.5 EXE 的可执行节中唯一命中 |
| 真实进程只读回读 | 严格只读句柄识别为 2.0.5；角色面板、召唤石库存、状态管理器和虚拟因子系统全局均解析为有效对象；队伍配装路径读取到 `PL0400` 的完整武器与 12 颗因子 |
| 原生组件短生命周期 | 镜头、音频、虚拟因子及 QOL（含自由团长和全部显示项）分别完成启动、状态回读、停用与恢复；测试结束后没有保留活动组件 |

逐行表差分确认：

- `reward_point.tbl` 有 14 行 `Min/Max` 改变，对应官方列出的 Chaos++ 与 Defy Infinity MSP 奖励上调。
- `skillboard_effect_action_parts.tbl` 只有两个菲迪埃尔记录变化：`463BC1BE` 的 `Value1 10→5`、`Value2 1→10`，以及 `03DB2C57` 的 `Value2 0.5→15`。项目不把这些字段直接当作完整动作系数，避免用未知字段含义生成虚假精确伤害。

独立扫描还发现，上游提交 `729acf6...` 将常规 OD 地址写成 `0x2C6FD0`，但本机 2.0.5 完整签名唯一命中实际为 `0x22C6FD0`；项目采用本机签名结果。巴布类第二条 OD 路径则在 `0x2B3F92E`，其 19 字节原块已单独校验。

## 证据边界

- 官方公告能证明产品行为和数值规则发生变化，但不公开存档字段或机器码地址。
- 上游提交能证明其作者在自己的 2.0.5 环境中修改了哪些地址和签名，但不替代空域工坊对本机游戏文件的独立校验。
- 本报告是外部一手来源的只读研究；除新增本报告外，没有修改功能代码、提交、推送或发布。
