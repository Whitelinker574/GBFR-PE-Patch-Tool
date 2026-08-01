# PlayStation 端指定掉落与奖励倍率可行性调研

调研日期：2026-08-01（停电后重新核对）
范围：原版《Granblue Fantasy: Relink》、`Endless Ragnarok` 扩展、2.0.3 Steam 可执行文件、用户提供的 CT 表，以及当前仓库的任务奖励 Hook。本文只记录可复现证据，不提供主机破解、存档解密、重新签名或 DRM 绕过步骤。

## 结论先行

此前把“Steam 与 PlayStation 不能跨平台”写成当前产品的绝对结论是不准确的。官方资料现在明确显示 **Endless Ragnarok 扩展支持跨平台多人游戏**，Steam 商店的 Standard/Special Upgrade Kit 都标记了 `Cross-Platform Multiplayer`，官方站点列出的平台包括 Nintendo Switch 2、PS5、PS4 和 Steam。旧的 `relink.granbluefantasy.jp` FAQ 仍描述原版发行时的 PS4/PS5 与 Steam 隔离，不能覆盖扩展的新联机服务。

但“能跨平台联机”不等于“PC 房主可以把修改后的奖励发给 PS 队友”。截至本次核对，尚未找到官方接口、公开网络协议或可复现的开源实现，证明房主的本地奖励倍率/指定掉落会被序列化并由每个 PS 客户端写入自己的存档。当前工具的 2.0.3 Hook 只在 PC 进程的本地任务结果记录上修改普通物品数量；它没有 PS 客户端写入路径，也没有经过验证的队伍奖励广播协议。因此不能对“PS 队友一定得到同样倍率或指定物品”作产品承诺。

## 平台与版本边界

| 场景 | 当前证据 | 结论 |
|---|---|---|
| 原版 Steam App `881020` | Steam AppDetails 的分类只有 `Online Co-op`，没有 `Cross-Platform Multiplayer`；原版 Cygames FAQ 说明 PS4/PS5 可互联但不能与 Steam 联机 | 原版跨平台能力未被当前一手资料证明，不能把扩展的能力倒推给原版 |
| Endless Ragnarok Steam Upgrade Kit `3839790` / `4306890` | Steam AppDetails 明确包含 `Cross-Platform Multiplayer`；官方站点列出 Switch 2、PS5、PS4、Steam | 扩展服务支持跨平台联机，包含 PC 与 PlayStation 的平台组合，但仍需以该扩展的版本、账号和联机服务为准 |
| 原版 2.0.3 Steam 更新 | 官方 Steam 公告 [Ver. 2.0.3 Update Information](https://steamstore-a.akamaihd.net/news/externalpost/steam_community_announcements/1839676055887211) 的网络修复段落注明 `Applies to the Steam platform`，没有说明把原版改为跨平台 | 不能仅凭“2.0.3”把原版与扩展的跨平台能力混为一谈 |
| Endless Ragnarok 测试/扩展公告 | 官方公告 [Open Beta Test Confirmed](https://steamstore-a.akamaihd.net/news/externalpost/steam_community_announcements/1830163047265022) 明确写 `cross-platform play`；官方站点为 [relink-ragnarok.granbluefantasy.com](https://relink-ragnarok.granbluefantasy.com/en/) | 这是当前“PC 与 PS 能否同房”的一手证据，但它不证明奖励可由房主任意改写 |

因此，用户所说“游戏支持跨平台”在 Endless Ragnarok 语境下有官方依据；旧文档的绝对否定已被本文件纠正。原版和扩展必须分开显示，应用不能根据平台名称猜测奖励同步能力。

## CT 表实际包含什么

用户提供的 `GFR_v0.8.9_CHS_(Unofficial).CT`（585,249 bytes，2026-07-31）以 XML `CheatEntry` 保存脚本。与掉落/奖励相关的条目只有本地进程修改：

- ID `31107`：`一键发现隐藏的宝箱`，AOB 后将 6 字节改为 NOP。
- ID `31109`：`自动收集任务宝箱`，清空收集条件分支。
- ID `31456`：`100% 巴武掉落率`，将分支改为 NOP。
- ID `33483`：`任务得分倍率`，把本地 `ecx` 转为浮点并乘以 `NBGFR099_flt`，默认值 2.0。
- ID `33487`：`强制获得支线目标奖励`，在三个本地判断点写入 `B0 01`。
- ID `31490`：`自动完成支线任务`，直接把本地任务进度与支线奖励字段改为已完成。

逐项脚本没有队伍广播、跨平台标识、网络消息结构、在线槽位或客户端奖励确认字段。它们可以作为“本机游戏进程的功能线索”，不能作为“PS 队友发奖接口”的证据。

## 2.0.3 本地任务奖励 Hook 的边界

当前仓库的实现见 [`internal/backend/runtime_patch_task_reward_multiplier.go`](../internal/backend/runtime_patch_task_reward_multiplier.go)：

- 本机 2.0.3 可执行文件大小为 `123,506,656` bytes，SHA-256 `1BBBEC61AAB7F75FE328CF6BFE0247EBDBCEC6C404CEC12C032B8FFA41D22102`。
- 2.0.3 本地唯一匹配入口是 RVA `0x1FDA9C0`，原始 14 字节为 `48 8B 0D 99 90 05 05 31 F6 31 D2 45 31 C0`。入口的 `E8` 相对调用目标为 RVA `0x21BD890`。
- 入口加载一个任务结果管理对象；代码洞按 `0x24` 字节步长遍历已聚合的结果记录，只处理 `item type == 1` 的普通可堆叠物品，在 2/4/8/16 倍后封顶 999。因子、祝福石、召唤石和武器实例不会被这个 Hook 放大。
- Hook 通过 `{PID, creation time}`、原字节、代码洞所有权、写后回读和恢复路径保护；测试已在本地 2.0.3 二进制上通过 `TestRuntimePatchCatalogMatchesLocalGame203`，任务奖励专用单元测试也通过。

这段代码证明的是“PC 端本地任务结果记录可改写”，不是“主机权威奖励广播”。静态调用链落在本地结果整理/提交边界，没有公开的网络序列化 DTO、在线玩家奖励确认或 PS 存档落盘函数。由于没有抓到同一跨平台任务的双方结果包，仍不能推断 PS 客户端会收到修改后的数量。

## 2026-08-01 联机整局只读抓取

在用户实际完成一整局联机任务期间，对 `PartyWin.dll!PartyEndpointSendMessage`、`PartyStartProcessingStateChanges` 的 `EndpointMessageReceived` 状态以及 RVA `0x1FDA9C0` 任务结果入口做了只读记录。探针没有修改、重放或阻断消息；结束后已卸载，游戏进程继续正常响应。原始载荷只保留在本机临时诊断目录，不进入仓库，因为其中可能包含联机标识。

- 约 555 秒内记录到 59,396 次发送、87,620 次接收和 1 次任务结果入口；入口只在最终结算边界命中。
- 任务结果向量包含两个 `0x24` 字节记录：达丽雅银奖章 ×3（`81BF605C`）与达丽雅金奖章 ×3（`5C07839A`）。这也实测确认记录 `+0x00` 是物品 hash、`+0x04` 是数量；本次两项的 `+0x08` 均为 0。
- 结果入口之后约 0.26 秒内出现多组成组三份的相同出站消息，目标数均为 1。这与向三个远端端点逐个发送相同状态相符，但仅凭发送次数仍不能把当前 PC 的房主身份当作已证明事实。
- 对整局全部原始收发载荷反查上述两个物品 hash，除本地任务结果快照外为零命中；完整的两个 36 字节奖励记录也没有进入 Party 载荷。
- 结算附近的消息仍是结构化状态：例如 `family 2:51` 的 32 字节记录、`family 6:22` 的 72 字节记录等；它们携带分数、位置或状态类数值，但没有直接携带这两项物品 ID。

同一份抓包随后成功解出了另一条明确的应用层消息：首次 `3:14 / 784 bytes` 与周期性 `2:63 / 780 bytes` 都包含队伍槽位、角色 hash、武器 hash、12 个因子、12 个副词条及等级。四个角色流能按槽位 0/1/2/3 稳定区分，三名队友各自持续发送同结构资料。这证明当前 Party 观察点能够看到未加密的游戏 DTO，且已知物品 hash 的全局反查方法有效；因此“达丽雅奖章 hash 在整局网络载荷中为零命中”不能简单归咎于抓包位置错误或 Party 载荷整体不可解码。配装帧与奖励路径是两个不同消息域，不能因为前者可解就臆造后者字段。

这次实测排除了最简单的实现假设：当前结果向量并不是原样作为“物品 ID + 数量”广播给三名队友。更符合证据的两种候选是：房主同步任务评价/随机种子/共享状态后，各客户端依据本地表生成奖励；或奖励完全在各客户端独立结算。它仍没有排除“房主上游任务状态可让队友合法获得更多奖励”，因此下一轮应做严格的 1×/2×同任务差分，比较结算前状态消息，而不是直接猜测奖励包字段。

本次抓取还暴露了当前本机倍率实现的一个边界：代码只处理 `item type == 1`，而实测的两种达丽雅奖章记录类型为 0，所以它们不会被现有倍率放大。若产品目标是“全部普通任务结算道具”，必须先按不同奖励类别补齐真实样本，不能仅移除类型判断后盲目修改所有 `0x24` 记录。

## “房主倍率让 PS 队友也获益”的验证状态

### 已证实

1. Endless Ragnarok 官方资料支持跨平台联机。
2. 当前工具可以在支持的 PC 2.0.3 进程内修改普通任务结果数量，并在退出时恢复原始指令。
3. 官方公告说明任务完成会产生奖励，2.0.3 公告也提到在线任务结算与网络错误修复。

### 尚未证实

- 奖励数量由房主、独立客户端，还是服务器分别权威计算。
- 修改 PC 房主的本地结果记录是否会进入跨平台网络消息。
- PS 客户端是否接受主机发送的普通物品数量，是否会按自己的本地规则重算。
- 指定因子、祝福石、召唤石或任意物品能否通过网络奖励消息安全传给 PS 存档。
- 断线、重复结算、重试、跨平台版本差异下是否会重复发放或丢失奖励。

### 不应继续使用的表述

在没有双方实机样本前，不要写“Steam 房主可给 PS 队友发 2/4/8/16 倍奖励”“PC 指定掉落会同步到 PS 存档”或“已经实现跨平台发奖”。这些说法把联机互通误写成奖励注入能力。

## 社区项目逐项复核：所谓“倍率”实际改在哪一层

这次不再只依据官方资料，而是进一步核对了公开 Mod、开源源码、CT 脚本和联机反馈。结果是：社区确实已经实现多种“掉落/奖励倍率”。静态奖励表和当前运行时倍率主要落在 PC 本地结算层；另一方面，强制彩虹史莱姆生成已有“联机其他玩家也受影响”的正向报告，说明房主侧共享敌人/任务事件值得单独追踪。尚未发现的是把**自定义物品奖励包**序列化后广播给队员、再由 PlayStation 客户端持久化的公开实现。

| 项目与版本 | 具体实现 | 联机证据 | 对“PS 队友共享”的含义 |
|---|---|---|---|
| [Relink Multiplier 1.0.0](https://www.nexusmods.com/granbluefantasyrelink/mods/695?tab=posts)（mjsxi，2026-07-25） | 通过 Reloaded II 提供物品、MSP、Transmarvel、券等倍率 | 作者针对“大厅里的朋友是否也获得奖励”明确回复：只放大安装者自己的内容 | 这是直接反证：现有倍率不会自动变成队伍广播 |
| [All Drops Multiplier 1.0.0](https://www.nexusmods.com/granbluefantasyrelink/mods/686?tab=posts)（redmcro，2026-07-22） | 发布 2/4/8/16 倍的本地 `reward_lot.tbl` 数据表 | 多人实测反馈称 Revenge of the Ooze 和普通 Boss 任务只有安装者得到物品增量；另有一次 Explore 收藏点由房主触发后全队似乎都翻倍 | 说明“任务共享计分”与“各客户端物品结算”可能走不同权威层。该反馈平台未注明为 PS，不能当成跨平台落盘验证 |
| [Guaranteed Spawn Prismatic Slime 1.3.2](https://www.nexusmods.com/granbluefantasyrelink/mods/561?tab=logs)（Yuu，2026-02-17；现已移除下载） | 将 Slimepede 的彩虹史莱姆生成率改为 100%；现已无法取得文件或源码，因此不猜测它修改的具体表或 Hook | 发布说明明确警告不要在线使用，因为“其他玩家也会受到影响”；2024 年社区联机截图讨论也报告加入带有同类 Mod 的房间后出现多只彩虹史莱姆 | 这是目前最强的“房主改变合法奖励机会，未安装队员也能看见”正证据，支持研究共享敌人生成层；但没有注明 PlayStation，也没有双方库存与重启回读 |
| [Summon Drop Picker 2.1.0](https://www.nexusmods.com/granbluefantasyrelink/mods/677?tab=posts) / [开源 GUI 提交 `e4cfccc`](https://github.com/Evoyn/gbfer-summon-drop-picker/tree/e4cfccc9fbd51aa57575e7eaeebd705fddbe9486) | [`src/patch.rs`](https://github.com/Evoyn/gbfer-summon-drop-picker/blob/e4cfccc9fbd51aa57575e7eaeebd705fddbe9486/src/patch.rs) 直接重写本地 `summon_lot.tbl` 的 20 字节行、权重和曲线；[README](https://github.com/Evoyn/gbfer-summon-drop-picker/blob/e4cfccc9fbd51aa57575e7eaeebd705fddbe9486/README.md) 明确说明无 HTTP、socket 或网络代码 | 作者回答多人问题时明确表示只改变自己的掉落 | 本地召唤石抽取表可复用作离线数据方案，但没有队伍奖励发送路径 |
| [alexfrljuckic/GBFRelinkMod 提交 `3517378`](https://github.com/alexfrljuckic/GBFRelinkMod/tree/3517378f00d1048fa0b350bb49e6b861d40e8d7d) | [`gbfr.quest.mspmultiplier/Mod.cs`](https://github.com/alexfrljuckic/GBFRelinkMod/blob/3517378f00d1048fa0b350bb49e6b861d40e8d7d/mods-src/gbfr.quest.mspmultiplier/Mod.cs) 从本地归档读取 `reward.tbl` / `reward_point.tbl`，放大 MSP 的 Min/Max 后用 `AddOrUpdateExternalFile` 注册外部表；[`gbfr.summon.drops/Mod.cs`](https://github.com/alexfrljuckic/GBFRelinkMod/blob/3517378f00d1048fa0b350bb49e6b861d40e8d7d/mods-src/gbfr.summon.drops/Mod.cs) 同样修改本地召唤相关表 | 源码没有 reward send/receive、网络序列化、队伍收件人或 PS 存档确认代码 | 这是用户所说“别人已有接口”的强候选，但接口是本地 `IDataManager` 表覆盖，不是跨平台发奖 API |
| 同仓库的[任务奖励链说明](https://github.com/alexfrljuckic/GBFRelinkMod/blob/3517378f00d1048fa0b350bb49e6b861d40e8d7d/docs/16-quest-reward-chain.md) | 记录 `quest_baseinfo_ex_data` → `reward.tbl` → `reward_lot.tbl`；通过新增 lot 并写入 `_100/_101` 奖励槽实现每次通关额外物品 | 文档仍把 `_100/_101` 的首次/重复结算语义列为待实机问题，没有网络层结论 | 可借鉴指定掉落的表结构，不足以证明房主表会成为客机奖励真值 |
| [tastyegg/gbfrelink-modpack 提交 `a4cb9b4`](https://github.com/tastyegg/gbfrelink-modpack/tree/a4cb9b43430f9f40aea8fb75b7869c0d12bd44eb) | `easyterminus` 只有本地 `GBFR/data/system/table/reward.tbl` 与 `ModConfig.json`，`ModDll` 为空 | 仓库没有运行时代码，更没有网络处理 | 100% 巴武掉落也是静态本地表，不是共享奖励实现 |
| [Midnight's Harder Bosses 1.3.0](https://www.nexusmods.com/granbluefantasyrelink/mods/365) | 修改敌人、任务和 `reward.tbl` / `reward_lot.tbl` | 作者记录 Boss HP/Overdrive 由房主决定，而投射物与伤害由各客户端决定，并建议联机成员都安装 | 证明这款游戏存在“房主权威”和“客户端权威”并存；不能因 Boss 状态可由房主决定，就推断物品奖励也由房主广播 |
| 用户提供的 `GFR_v0.8.9_CHS_(Unofficial).CT` | 宝箱、自动拾取、巴武掉率、任务得分、支线奖励均为本地 AOB/字段写入 | 脚本中没有 socket、奖励消息 DTO、队伍标识或客户端确认 | CT 可提供本地 Hook 线索，不含跨平台共享通道 |

### 旧版 Nexus 掉落项目与联机讨论

为避免只观察 2.0/2.0.3 新项目，又向前检查了 2024–2026 年的任务奖励表 Mod。以下项目页、作者回复和用户实测均可公开读取；Nexus 的下载文件本体需要站点登录或下载权限，未把无法下载的二进制内容猜成源码。能取得源码的项目仍以精确提交和源文件为准。

| 项目 | 版本/日期与修改点 | 是否仅安装者或需要每人安装 | PS/跨平台持久化证据 |
|---|---|---|---|
| [Easy Terminus Weapons](https://www.nexusmods.com/granbluefantasyrelink/mods/431) | 1.0.0，2024-05-28；把巴武放入低级任务的本地奖励表 | 另一项目的联机用户随后报告：该类首关巴武 Mod 给自己武器，但同场朋友没有得到 | 朋友的平台未注明，未做重启回读；这是普通联机不共享物品的直接实测，不是 PS 专项证明 |
| [Faster Terminus Weapons](https://www.nexusmods.com/granbluefantasyrelink/mods/449?tab=posts) | 1.0.0，2024-06-19/21；`reward.tbl` 提高巴武奖励包命中，附加文件修改 `reward_lot.tbl` | 2026-06 有用户明确询问能否利用跨平台帮助主机朋友；作者只列出可能不生效、掉线、异常或风险等推测，并明确说未验证能工作 | 这是目前最直接的“帮助 console 朋友”公开讨论，但没有成功任务截图、PS 存档差异或重启证据，不能算正证据 |
| [Behemoth drop rates rebalanced](https://www.nexusmods.com/granbluefantasyrelink/mods/460?tab=posts) | 1.0.0，2024-07-14，2024-09-07 更新；运行日志显示只注册本地 `system/table/reward_lot.tbl` | 页面有人询问是否全队共享，但没有作者或实测答复；作者确认同一时刻只会应用一份本地表 | 无 PS 或客机持久化证据 |
| [Endgame Rebalance Plus](https://www.nexusmods.com/granbluefantasyrelink/mods/541?tab=posts) | 1.1.0，2025-09-09；本地 `reward_lot.tbl` 调整多个任务 | 作者两次说明从未验证在线影响，只能猜测正常表、无效或异常；不应把作者猜测当结论 | 无正证据；2.0 表列移动还使旧版本失效，进一步说明它依赖客户端本地表版本 |
| [Extra Variations of Gulp...](https://www.nexusmods.com/granbluefantasyrelink/mods/549) | 0.1，2026-01-06；修改任务时限、支线目标以及 `reward.tbl` / `reward_lot.tbl` | 作者明确写可能要求所有玩家各自在客户端安装，避免联机不同步 | 这说明“房主装一次即可把自定义奖励发给所有人”并非该项目的设计；无 PS 验证 |
| [Behemoth custom drop rates](https://www.nexusmods.com/granbluefantasyrelink/mods/585?tab=posts) | 1.0，2026-06-02；作者说明只是本地 `reward_lot.tbl` | 2026-06-28 用户报告：同类“第一关掉巴武”Mod 给自己武器，但同场朋友没有拿到；作者据此认为相同表机制也不会影响别人 | 这是旧项目中最强的联机物品反证；平台未注明，且没有重启回读 |
| [More Fortitude Crystals-Slimepede](https://www.nexusmods.com/granbluefantasyrelink/mods/588) | 1，2026-06-10；把本地 `reward_lot` 中结晶数量从 50 改为 9999 | 页面明确标注不要在线使用；没有队伍发送代码或联机成功报告 | 无 PS/跨平台证据 |
| [Guaranteed Terminus and Behemoth sigil drops](https://www.nexusmods.com/granbluefantasyrelink/mods/636?tab=posts) | 1，2026-07-16；用户可从 DLL 抽出修改后的本地表，作者确认它只触碰 loot reward table | 对“是否影响其他在线玩家”的公开回复是掉落表位于客户端；该回复者不是作者，因此权重低于源码和成对实测，但与其他证据一致 | 无 PS 重启样本 |
| [Configurable Mastery Point Multiplier](https://www.nexusmods.com/granbluefantasyrelink/mods/635?tab=posts) | 1，2026-07-16；Reloaded II 配置 MSP 倍率 | 有用户询问是否提高其他在线玩家 MSP，但页面截至核对时没有作者答复；不能把沉默当支持 | 无正证据 |
| [Midnight's Overhaul](https://www.nexusmods.com/granbluefantasyrelink/mods/455) | 页面最新版本系列始于 2024，含掉率和额外物品等大量本地表改动 | 作者明确要求在线时所有玩家安装，并说明 Boss 调整不会自动传到原版客户端 | 再次证明房主对部分战斗状态的权威，不等于奖励表会传给未安装客机 |

正向现象不再只有 Explore 收藏计分：Guaranteed Spawn Prismatic Slime 的发布说明明确说在线时其他玩家也会受到影响，旧联机讨论也报告进入带有同类 Mod 的房间后全队看见多只彩虹史莱姆。它与静态 `reward_lot` 只影响安装者并不冲突：共享任务状态和场景敌人可以由房主驱动，而最终任务结果表仍可能由每台客户端本地结算。因此应把研究入口从“把房主结算列表复制给队友”转向“让房主生成客户端原本就认识、并会合法掉落的共享敌人或任务事件”。

### CT 0.7.4 → 0.8.9 历史脚本对比

本机还留有五份可直接解析的 CT，均声明适配游戏 2.0.2。逐份解析 XML `AssemblerScript`，而不是只比较功能名称：

| CT 文件 | 表内日期 | 奖励/掉落脚本演进 |
|---|---:|---|
| `0.74_CHS(完善版7月14日).ct` | 2026-07-12，v0.7.4 | 已有 `31107` 隐藏宝箱、`31490` 自动完成支线、`31109` 自动收集任务宝箱、`31456` 100% 巴武掉率 |
| `CHS 0.8_完全汉化版.ct` | 2026-07-14，v0.8.0 | 上述四段奖励相关 AOB 与 0.7.4 相同 |
| `CHS 0.8.1_.ct` | 2026-07-15，v0.8.1 | 新增 `33483` 任务得分倍率、`33487` 强制支线目标奖励 |
| `GFR_v0.8.4_CHS_祝福石修改hotfix(Uno.ct` | 2026-07-16，v0.8.4 | 六段脚本继续沿用；任务得分在本地把值转为浮点后 `mulss`，支线奖励在三个判断点强制 `B0 01` |
| `GFR_v0.8.9_CHS_(Unofficial).CT` | 2026-07-30，v0.8.9 | 六段奖励相关 AOB/写入路径仍未增加队伍收件人或网络处理 |

五份 CT 的脚本体中均未命中 `socket`、`send`、`recv`、`WSASend`、`WSARecv`、`packet`、`serialize`、`peer`、`lobby`、`online`、`PSN`、`crossplay`、`recipient` 或确认包相关标识。更关键的是，v0.8.9 **确实知道如何表达“应用全队”**：冷却修改 `33556` 下有独立子项 `33539`“应用全队”，通过额外 flag 和角色类别判断扩大 Hook 范围；队伍指针 `30967` 也显式列出三名队员。六段奖励/掉落脚本没有这种子项、flag、队伍遍历或在线收件人处理。

这使 CT 证据比“没搜到网络字符串”更强：同一张表里，作者在需要影响队伍时明确实现队伍分支，而奖励功能从 v0.7.4 延续到 v0.8.9 始终只改当前 PC 进程的任务条件、分数或结算判断。Patreon 的[公开版本说明](https://www.patreon.com/NidasBot/posts/granblue-fantasy-163310377)也只对冷却修改写明可应用队伍，同时将整张表标为离线用途；没有声称任务奖励能发给队友。

另外检查了本机留存的 Relink Logs 社区源码副本。它能识别队伍成员并处理伤害/昏厥事件，但未找到任务奖励或掉落的 send/receive DTO；因此不能把“能读队友战斗事件”外推成“能向队友发送奖励”。公开 GitHub 仓库搜索还复核了 trainer 与静态 Mod 包：找到的是本地内存写入和本地 `.tbl`，没有新的共享奖励数据通道。

### 四层机制必须分开

1. **PC 本地结果/库存层**：当前工具 Hook、Relink Multiplier、All Drops Multiplier、Summon Drop Picker 均已证明可改，这是已实现能力。
2. **房主权威任务状态层**：Boss HP、Overdrive、部分 Explore 收藏计分存在房主影响的证据；它只说明某些任务状态能共享。
3. **奖励网络序列化层**：公开源码中尚未找到“构造队员奖励包 → 指定收件人 → 发送/确认”的函数、结构或签名。
4. **PS 客户端接受与持久化层**：尚无 PS 任务前后存档差异、重启回读和重复结算样本。

因此，当前代码可以继续完善“本机全局倍率”和“本机指定掉落”，但不能把一个可点击的“队内通用”开关直接接到尚未识别的游戏消息载荷上。下文的新一轮静态核对已经证明 **Endless Ragnarok 客户端确实存在可向指定远端端点发送游戏数据的 PlayFab Party 通道**；未知的不是“有没有传输通道”，而是任务奖励是否使用这条通道、PS 客户端是否信任该载荷，以及去重与落盘规则。若要真正实现该开关，仍需先识别奖励生成后的消息类型、队员收件人、客机接收/去重逻辑，并以 PC 房主 + PS 客户端完成存档前后和重启回读。

## 替代机制反推：让未安装 Mod 的队友得到实际福利

这部分不再只从“修改 `reward_lot.tbl`”向前推，而从 PS 队友最终库存增加反向拆链：

```text
PS 存档新增物品
  <- PS 客户端调用本地 ItemManager / 保存函数
  <- 任务结果、拾取事件或账号奖励被 PS 客户端接受
  <- 游戏定义的消息载荷 / 共享任务状态 / 官方账号服务
  <- PC 房主可控制的任务状态、网络端点或场景实体
```

只要中间任意一层由房主权威控制，而且未安装 Mod 的客户端会把它转成合法库存，就存在实现空间。反过来，单纯改变房主本地结果列表发生在链条末端，当然只能影响房主。

### 新证据：Endless Ragnarok 已换成 PlayFab Party 跨平台数据通道

对本机 2.0.3 可执行文件（SHA-256 `1BBBEC61AAB7F75FE328CF6BFE0247EBDBCEC6C404CEC12C032B8FFA41D22102`）重新解析 PE 导入表，得到此前只看 CT 和掉落表时遗漏的关键事实：

- 游戏加载 `PartyWin.dll`、`PlayFabMultiplayerWin.dll`、`PlayFabCore.Win32.dll` 和 `PlayFabServices.Win32.dll`。
- `PartyWin.dll` 的导入包含 `PartyEndpointSendMessage`、`PartyStartProcessingStateChanges`、`PartyFinishProcessingStateChanges`、`PartyEndpointGetUniqueIdentifier`、`PartyNetworkAuthenticateLocalUser` 等完整的数据消息路径。
- `PlayFabCore.Win32.dll` 的导入包含 Steam 登录与 Entity Token；`PlayFabMultiplayerWin.dll` 包含 Lobby 创建、加入、成员和 owner 接口。
- 当前 `PartyWin.dll` 文件版本为 `1.10.2509.24002`，产品版本 `1.10.12`，SHA-256 `3F0C6ABBB735D81FA766A105982BDA73F1D2C2CF01109FA2E7CF64813A52CE55`。
- `PartyEndpointSendMessage` 的 IAT 跳板 RVA 为 `0x49AC330`，游戏代码中只有一个直接调用点 RVA `0x2512B27`。调用前明确设置 `targetEndpointCount = 1`、一项 `PartyDataBuffer`，也就是游戏已经具备“给某一个远端队员发送一段游戏定义载荷”的能力，而不只是在本机改 UI。
- 接收侧通过 `PartyStartProcessingStateChanges`（IAT 跳板 RVA `0x49AC400`，调用点 RVA `0x25D2F9`）取得状态数组，再交回 `PartyFinishProcessingStateChanges`（调用点 RVA `0x25D5FE`）。微软的结构定义说明，`PartyEndpointMessageReceivedStateChange` 直接提供 `senderEndpoint`、接收端点列表、`messageSize` 和连续 `messageBuffer`。

微软的一手文档进一步确认，[`PartyLocalEndpoint::SendMessage`](https://learn.microsoft.com/en-us/gaming/playfab/multiplayer/networking/reference/classes/partylocalendpoint/methods/partylocalendpoint_sendmessage) 可以定向发送或在目标数为零时广播；保证送达消息会经透明云中继转发到远端设备。[接收结构](https://learn.microsoft.com/en-us/gaming/playfab/multiplayer/networking/reference/structs/partyendpointmessagereceivedstatechange)把游戏载荷原样交给标题代码处理。由此可以修正之前过度保守的判断：**跨平台传输能力真实存在，而且导出 ABI 比游戏内部 AOB 更适合作为低维护的只读观测入口。**

这仍不证明“奖励就在消息里”。PlayFab Party 负责认证、寻址与传输，是否接受“新增道具”由 Relink 自己的消息处理器决定。不过现在已有一条可以实际验证的路线，而不是只能凭掉落 Mod 评论猜测。

Nenkai 针对 2024 原版的[网络逆向笔记](https://nenkai.github.io/relink-modding/resources/re/networking/)记录的是 Steam Networking Sockets + ChaCha20 及旧版收发函数签名。三个旧签名在当前 2.0.3 EXE 中均为零命中；这与当前 PE 明确导入 PlayFab Party 相符，说明 Endless Ragnarok 的跨平台网络层已经换代。后续不应继续把旧 Steam 加密函数当作 2.0.3 的唯一抓包入口。

### 候选路线一：利用房主权威的任务目标、评价和奖励档位

这是成功概率最高、侵入最小的一条。Nenkai 当前任务结构文档明确记录：

- `rewardRank_` 决定 `reward_lot` 中可用的奖励档位，且也会影响指向奖励表的敌人掉落；
- `rankReward[]` 按最终评价发放奖励；
- `subMissions_` 的目标会映射到独立 `reward_` / `firstReward_`；
- 隐藏目标也能触发额外奖励。

来源见 [`quest_base_info`](https://nenkai.github.io/relink-modding/resources/quests_layouts/quest_base_info/)。从所有玩家必须看到同一任务成败与目标进度这一产品行为，可以合理推断联机中至少存在一份共享或可同步的任务状态；具体哪个字段由房主权威控制仍需实测。既有 Explore 收藏点“房主触发后全队计数翻倍”的社区实测，也指向这一层。

实验不应先改物品 ID，而应在 PC 房主侧依次验证：强制真实支线目标完成、提高合法任务得分/评价、提高合法宝箱/拾取计数。若 PS 结果页因此多出原版表中本来就存在的支线或 S++ 奖励，并在重启后保留，就可以先实现“队伍奖励加成（合法奖励池）”。它未必能给任意物品，但足以让 PS 队友获得更多材料，而且比构造未知奖励包更稳。

风险：CT 的“强制支线奖励”可能只改房主自己的结果判断，而不是共享任务状态。必须以 PS 端结果页和重启回读判定，不能看到房主 UI 变化就宣称成功。

### 候选路线二：房主生成或复制可同步的稀有敌人、宝箱和掉落实体

任务结构同时列出 `placementsInfo_`，其中有 `treasureMaxCount_` / `treasureMinCount_`、`pickMaxCount_` / `pickMinCount_`；Placement 文件也包含 `TreasureSet` 与 `Treasure` 实体。2.0.3 EXE 的运行时类型字符串还包含 `DropReward`、`PstDropReward` 和 `RewardTreasureItem`。[Nenkai 的任务掉落资料](https://nenkai.github.io/relink-modding/resources/quest_drop_rates/)还记录了 Slimepede 每波 1% 概率出现彩虹史莱姆并掉落 5 个苍穹之辉。这给出另一条链：

```text
房主触发合法稀有敌人，或创建共享 Treasure/DropReward 实体
  -> 场景同步消息把实体或拾取事件发给队员
  -> PS 客户端按自己的合法表解析奖励
  -> PS 本地 ItemManager 入库
```

这和“修改房主结果列表”是不同入口，也能解释为何 Explore 场景收集可能全队受益，而结算页巴武表不会共享。可以先强制任务本来就支持的彩虹史莱姆/奖励敌人分支，或复制任务本来已有的一只普通蓝光拾取物/宝箱：观察 PS 是否看见同一敌人或实体、是否能独立拾取、奖励是否持久化。只使用客户端原本认识的敌人 ID 和奖励表时，PS 可以继续按自己的合法数据结算；即使不能指定任意物品，也可能实现“房主开启稀有敌人和宝箱加倍，全队各自领取”。若只同步外观、不发送拾取事件，或拾取权完全属于各客户端本地随机分支，则该路线失败。

社区已经给出与这条链直接一致的正证据：Guaranteed Spawn Prismatic Slime 的说明明确警告联机队员也会受到影响，另有独立联机帖子把同房多只彩虹史莱姆归因于房间内玩家使用 100% 生成 Mod。它仍不是 PS 持久化证明，而且原 Mod 已移除、没有源码可审；但至少说明“房主侧稀有敌人分支可能传播到未安装者”不是纯推测。首个实验应复现**一只合法彩虹史莱姆**，不要先复制多只实体，以免把生成同步、击杀归属和重复掉落混在一次测试中。

风险：重复网络实体 ID、多人同时拾取、房主迁移、断线重连和重复结算都可能造成假象或重复入库。第一阶段只能在私密房间、复制存档和普通材料上验证。

### 候选路线三：在稳定的 PlayFab Party ABI 上做“只读消息差分”，再决定是否有奖励载荷

由于 `PartyEndpointSendMessage` 和 `PartyStartProcessingStateChanges` 是版本化 DLL 导出，不需要先猜游戏内部网络 AOB。可以在现有内置 DLL 中先做一个**不修改、不重放**的实验记录器：

1. Hook PC 端 `PartyEndpointSendMessage`，只记录时间、房主/客机角色、目标端点匿名 ID、选项、buffer 数量、长度和载荷哈希；载荷原文默认不落盘。
2. 在 `PartyStartProcessingStateChanges` 返回后识别 `EndpointMessageReceived`，同样记录匿名元数据和哈希；处理完仍原样交给游戏。
3. 对同一任务做严格事件标记：空闲、击杀、完成支线、打开宝箱、任务结束、结果页确认、返回城镇。
4. 比较 1× 与只修改房主本地结果为 2× 的两轮。如果出站消息的长度/哈希完全不变，说明当前倍率 Hook 位于网络发送之后，无法通过**这个入口**共享；转测路线一和二。
5. 如果差异恰好随奖励数量变化，再用两个 PC 测试号、复制存档和一种普通材料识别字段、去重序号与确认消息。PC 客机重启回读通过后，最后才做 PC 房主 + PS 客机测试。

这条路线的价值首先是**回答数据路径问题**，不是立即发任意物品。若消息只同步任务状态，记录结果会直接指导路线一；若消息包含奖励记录，才继续分析收件人、类型、数量、序列号和客户端校验。若奖励完全不经过 Party，而由每台客户端自行结算，这条路线也会用实测把问题关闭，而不是继续争论。

### 官方账号奖励通道：真实存在，但不能当作房主发奖接口

EXE 中还出现连续字符串 `has_cross_reward`、`received_cross_reward`、`received_link_reward`、`cygames_id/check_reward`。Cygames 的[账号联动奖励说明](https://cygames.com/en/reward/gbf-er-gamelink/)确认：奖励需要联网，可在 Switch 2、PS5、PS4、Steam 上领取，但每份存档只可领取一次。这证明 Cygames 后端确实有一个跨平台“服务器确认资格 → 本地领取并落盘”的官方路径。

它不能用于玩家互赠或房主自定义物品：资格和一次性状态由 Cygames 服务控制，伪造服务响应既不是现有项目能力，也不应做成产品功能。它的研究价值是帮助区分两条网络：PlayFab Party 承载房间内游戏消息，Cygames ID 承载账户级权益。

### 双方安装轻量模组、自动下发和 PS 存档路线

- 如果双方都是 PC，最稳的方案仍是双方加载同一轻量配置/Mod，再以配置哈希握手；这无需让房主伪造客机库存。
- 未安装 Mod 的零售 PS 客户端没有 GBFR 的 Mod 自动下发机制。可类比但不能照搬的是《Deep Rock Galactic》的[官方 Mod 支持](https://www.deeprockgalactic.com/modding-support-faq)：房主使用会改变进度的 Mod 时，游戏会提示加入者下载并在离开时停用。这依赖开发商集成 mod.io；GBFR/PlayFab Party 本身只传游戏消息，不会把 PC DLL 安装到 PS。
- 《Monster Hunter: World》的[自定义任务包](https://www.nexusmods.com/monsterhunterworld/mods/5461)也明确要求联机朋友下载任务。它说明同类型 PVE 的“任务表在客户端”常见，但只能类比，不能替代 GBFR 实测。
- PS4 路线存在第三方 Save Wizard 解密后编辑的公开工具和样本，但它是离线存档处理，不是跨平台房主福利；PS5 原生存档、账号签名和重新封装也不属于当前应用可安全自动化的范围。本文不提供解密、签名或主机破解步骤。

### 路线优先级

| 优先级 | 实验路线 | 能否让未安装 Mod 的 PS 持久化 | 当前把握 | 产品化前置条件 |
|---|---|---|---|---|
| 1 | 房主强制合法稀有敌人分支；首测彩虹史莱姆 | 有现实可能；已有“联机其他玩家也受影响”的社区正例 | 中偏高（队内可见性）；PS 落盘未知 | 先复现单只彩虹史莱姆，证明 PS 看见、击杀后入库且重启不丢失 |
| 2 | 房主强制合法支线目标/评价档位 | 有现实可能；奖励仍由 PS 自己的表结算 | 中 | 一次 PC+PS 私房任务、结果页、存档差分、重启回读 |
| 3 | PlayFab Party 只读消息差分，命中后再做字段实验 | 取决于游戏消息是否携带奖励或可驱动合法领取 | 通道高、奖励语义未知 | 先做元数据记录器；双 PC 识别协议；最后 PC+PS |
| 4 | Cygames ID 官方奖励 | 官方奖励可以跨平台持久化，但不能由房主任意发放 | 高但不可泛化 | 只能使用官方资格与官方领取流程 |
| 5 | PS 离线存档编辑 | PS4 特定工具链可能可行；不是联机共享 | 独立路线 | 用户自行提供已解密副本、备份和合规环境 |

最值得继续投入的不是直接造一个“队内通用”开关，而是先加入内部实验用的 **Party 消息只读差分器**，并用同一轮跨平台实机把“支线/评价”和“共享宝箱”两条路线一起测完。这样一次测试就能判断：奖励跟随共享任务状态、共享实体、网络奖励载荷，还是完全由客机本地计算。

## 可实施的下一步验证

若用户要把它做成真实能力，必须使用 Endless Ragnarok 的真实跨平台房间进行成对验收：

1. PC 房主与 PS4/PS5 队友各自备份存档，记录任务前普通物品数量。
2. 仅开启 PC 端 2× Hook，完成同一普通任务，分别记录双方 Results 画面和任务后存档数量；再以 1×、4× 重复。
3. 分别测试普通堆叠物品、因子实例、祝福石、召唤石、武器实例，以及主目标/宝箱/支线奖励。
4. 对比房主退出、队友掉线、重试、重复清任务后的数量；每次都恢复备份存档。
5. 只有在 PS 客户端的字段回读与重启后仍一致时，才能把“跨平台奖励同步”列为实测能力；否则保留为研究项。

在找到公开网络消息结构或可复现的第三方实现之前，代码层面不应猜测、伪造或注入 PS 奖励包。当前最安全的产品行为是：UI 将“PC 本地任务倍率”和“跨平台队友是否受益”分成两条状态，后者显示“需要跨平台实机验收”，而不是静默宣称成功。

## 2026-08-01 一手源码复核与集成判定

为避免把 Nexus 的功能标题或用户评论误当实现证据，本轮又按固定提交、文件和脚本体复核了一次。结论没有发现一条此前遗漏的“房主构造奖励包并让未安装 Mod 的队友落盘”的公开实现，但把最可能成功的研究入口进一步收窄到了共享任务事件/敌人生成层。

### 可重复的源码与脚本检查

- 将 [`alexfrljuckic/GBFRelinkMod@3517378`](https://github.com/alexfrljuckic/GBFRelinkMod/tree/3517378f00d1048fa0b350bb49e6b861d40e8d7d) 以 detached HEAD 取回，实际提交为 `3517378f00d1048fa0b350bb49e6b861d40e8d7d`（2026-07-23，`release(summon-drops): v1.1`）。[`gbfr.quest.mspmultiplier/Mod.cs`](https://github.com/alexfrljuckic/GBFRelinkMod/blob/3517378f00d1048fa0b350bb49e6b861d40e8d7d/mods-src/gbfr.quest.mspmultiplier/Mod.cs) 只读取本地 `reward.tbl` / `reward_point.tbl`，修改 MSP 行的 `Min` / `Max`，再调用 `IDataManager.AddOrUpdateExternalFile()` 与 `UpdateIndex()`。整个固定提交中未出现 `PartyEndpointSendMessage`、`PartyWin`、`PlayFab`、奖励收件人或远端确认实现。
- 同一固定提交中的 [`gbfr.summon.drops/Mod.cs`](https://github.com/alexfrljuckic/GBFRelinkMod/blob/3517378f00d1048fa0b350bb49e6b861d40e8d7d/mods-src/gbfr.summon.drops/Mod.cs) 在 `ApplyGuaranteedDrops()` 中逐行把本机 `reward_summon.tbl` 的 `Chance` 提至 100，再注册本地外部表；[`gbfr.qol.instantloot/Mod.cs`](https://github.com/alexfrljuckic/GBFRelinkMod/blob/3517378f00d1048fa0b350bb49e6b861d40e8d7d/mods-src/gbfr.qol.instantloot/Mod.cs) 的两个 AOB 只清本机掉落倒计时与结果页倒计时。两者名字虽然涉及“掉落/结算”，仍没有网络收件人或队友入库路径。
- 该仓库的 [`docs/16-quest-reward-chain.md`](https://github.com/alexfrljuckic/GBFRelinkMod/blob/3517378f00d1048fa0b350bb49e6b861d40e8d7d/docs/16-quest-reward-chain.md) 明确把“保证每次通关给物品”实现为：在本地 `reward_lot.tbl` 追加权重 10000 的行，再把本地 `reward.tbl` 的空 `RewardLotId` 槽指向它；文档自己仍把 `_100` / `_101` 首通与重复语义列为待实机问题。它没有网络序列化或客机入库步骤。
- Nenkai 的 [`IDataManager` 官方开发文档](https://nenkai.github.io/relink-modding/modding/mod_manager_api/) 将 `AddOrUpdateExternalFile()` 定义为取出游戏归档文件、修改后注册回当前 Mod Loader，并更新当前游戏的 `data.i` 索引；这解释了为何这类“接口”能适配本地表更新，却不是队伍发奖 API。
- 用户提供 CT `GFR_v0.8.9_CHS_(Unofficial).CT` 本次复核 SHA-256 为 `39C0205AF7BEB8F48B1705AEEEB32F23F11991164EE96F0F641DE1225055017B`。奖励条目仍是 `31109` 本地宝箱条件、`31456` 本地巴武分支、`33483` 本地 `ecx` 得分乘法和 `33487` 三处本地布尔判断；脚本体没有 Party endpoint、网络消息、远端槽位、PSN 身份或奖励确认处理。
- [Relink Multiplier 1.0.0](https://www.nexusmods.com/granbluefantasyrelink/mods/695?tab=posts) 的作者对“朋友是否也获得奖励”直接答复为只影响安装者；[Max Summon Skill Levels](https://www.nexusmods.com/granbluefantasyrelink/mods/624?tab=posts) 的作者同样明确写出 loot/rates 在客户端侧。两条都是作者对自己实现边界的一手说明。
- [All Drops Multiplier 1.0.0](https://www.nexusmods.com/granbluefantasyrelink/mods/686) 的项目说明明确标注它是无游戏代码 Hook 的静态 `reward_lot.tbl`，并建议离线使用。它覆盖更多奖励来源不代表改变了权威层；覆盖面和网络传播是两个独立维度。
- [Midnight's Harder Bosses 1.3.0](https://www.nexusmods.com/granbluefantasyrelink/mods/365) 的作者说明 Boss HP/Overdrive 由房主决定，投射物和伤害按客户端决定，并建议所有联机成员安装同一 Mod。该项目同时修改 `reward.tbl` / `reward_lot.tbl`，却没有宣称奖励表由房主广播，正好证明游戏内存在多个不同权威域。
- [`OnonokiYotsugii/GBFR-Guaranteed-Terminus-Weapon@bc01735`](https://github.com/OnonokiYotsugii/GBFR-Guaranteed-Terminus-Weapon/tree/bc0173591c62452aae91afd1ddc69fd15201ee4d) 的完整树只有 `ModConfig` 与本地 `GBFR/data/system/table/reward.tbl`，没有 DLL、脚本或网络代码；“保证巴武”在该项目中仍是客户端表替换。
- 原始上游 [`BitterG/GBFR-PE-Patch-Tool@44bf2f9`](https://github.com/BitterG/GBFR-PE-Patch-Tool/tree/44bf2f964a696d810592ca29c1fd164827c827da) 的固定树也未包含任务奖励倍率或网络发奖模块；相关代码只有本地队伍监视与线上配装分享，不能从 upstream 借到队友发奖实现。

### 四跳证据矩阵

| 候选实现 | 本机结算 | 房主权威生成 | 网络序列化 | 队友持久化 | 判定 |
|---|---|---|---|---|---|
| 当前 `0x1FDA9C0` 任务结果倍率 Hook | 已证明，发生在本机已聚合 `0x24` 结果记录 | 未命中 | 整局 Party 载荷中未出现结果物品 hash/完整记录 | 未证明 | 不能接“队内通用” |
| `reward.tbl` / `reward_lot.tbl` / `reward_point.tbl` 表 Mod | 已证明，各客户端按自己的表生成 | 无房主调用点 | 开源项目无网络代码 | 作者实测只影响安装者 | 只适合本机倍率/指定掉落 |
| CT 得分、支线、巴武、宝箱脚本 | 本机判断或本机数值已证明 | 没有队伍分支证据 | 无消息 DTO/发送调用 | 未证明 | 可作为共享任务状态探针的地址线索，不能直接当共享奖励 |
| 强制彩虹史莱姆/共享奖励敌人 | 项目说明与联机记录支持其他玩家会看见共享敌人 | 有正向现象，具体函数/AOB 未公开 | 敌人/任务事件必然经过某个同步域，但公开文件已移除 | 未提供 PS 库存差分和重启回读 | 当前最有希望的合法福利路线，仍需复现源码和 PC+PS 验收 |
| 自定义 Party 奖励消息 | 本机可以构造数据不等于客户端接受 | 未定位权威函数 | Party ABI 存在，但奖励消息类型/序号/确认未知 | 完全未知 | 研究成本最高，禁止先做发送/重放 |

### 维护成本与推荐顺序

| 路线 | 版本维护成本 | 原因 |
|---|---|---|
| 本地静态奖励表 | 中 | 每次游戏更新需要核对行大小、表头和奖励链；无需维护 EXE AOB，但只影响安装者 |
| 本地结果倍率 AOB | 中至高 | 需要按 EXE 版本校验唯一签名、原字节、结果记录布局和恢复；仍然只在本机结算层 |
| PlayFab Party 只读观察器 | 低至中 | `PartyWin.dll` 导出 ABI 比游戏内部 RVA 稳定，但 Relink 自有 payload 仍需逐版本做长度/消息族契约；只读接入可以长期维护 |
| 房主共享任务事件/敌人生成 | 中至高 | 需要找到 2.0.3 的真实生成点、实体唯一 ID 和房主迁移语义；成功后可让客机走原生奖励逻辑，价值最高 |
| 直接构造/重放奖励包 | 极高 | 还缺消息类型、收件人、序列号、去重、确认和落盘五个关键环节；错误会造成重复结算或断线，不应在公开版本猜测实现 |

因此下一阶段不是把现有本机倍率 Hook 改名为“队内通用”，而是两条并行的只读验证：其一把 Party 生命周期观察器接到真实会话，给结算前后的消息族建立匿名端点与事件时间线；其二定位并只观察 Slimepede 合法稀有敌人生成分支。只有房主侧的单一受控变化能让未安装的 PC 客机出现同一事件、随后又能让 PS 客机库存增加并在重启后保留，才具备产品化开关的证据。

## 来源

- [Steam AppDetails：原版 Relink App 881020](https://store.steampowered.com/api/appdetails?appids=881020&l=english&cc=us)（分类无 Cross-Platform Multiplayer）
- [Steam AppDetails：Endless Ragnarok Standard Upgrade Kit 3839790](https://store.steampowered.com/api/appdetails?appids=3839790&l=english&cc=us)
- [Steam AppDetails：Endless Ragnarok Special Upgrade Kit 4306890](https://store.steampowered.com/api/appdetails?appids=4306890&l=english&cc=us)
- [Cygames：原版用户支持 FAQ](https://relink.granbluefantasy.jp/en/usersupport/)
- [Cygames：Endless Ragnarok 官方网站](https://relink-ragnarok.granbluefantasy.com/en/)
- [Cygames/Steam：Ver. 2.0.3 Update Information](https://steamstore-a.akamaihd.net/news/externalpost/steam_community_announcements/1839676055887211)
- [Cygames/Steam：Open Beta Test Confirmed（cross-platform play）](https://steamstore-a.akamaihd.net/news/externalpost/steam_community_announcements/1830163047265022)
- [Cygames ID：Granblue Fantasy 与 Endless Ragnarok 账号联动奖励](https://cygames.com/en/reward/gbf-er-gamelink/)
- [Microsoft PlayFab Party：PartyLocalEndpoint::SendMessage](https://learn.microsoft.com/en-us/gaming/playfab/multiplayer/networking/reference/classes/partylocalendpoint/methods/partylocalendpoint_sendmessage)
- [Microsoft PlayFab Party：PartyEndpointMessageReceivedStateChange](https://learn.microsoft.com/en-us/gaming/playfab/multiplayer/networking/reference/structs/partyendpointmessagereceivedstatechange)
- [Microsoft PlayFab Party：对象与端点关系](https://learn.microsoft.com/en-us/gaming/playfab/multiplayer/networking/concepts-objects)
- [Nenkai：Relink 原版网络逆向笔记](https://nenkai.github.io/relink-modding/resources/re/networking/)
- [Nenkai：任务目标、评价、支线奖励与场景拾取结构](https://nenkai.github.io/relink-modding/resources/quests_layouts/quest_base_info/)
- [Nenkai：任务掉落与彩虹史莱姆生成资料](https://nenkai.github.io/relink-modding/resources/quest_drop_rates/)
- [Nexus：Guaranteed Spawn Prismatic Slime 1.3.2（活动日志；文件已被作者移除）](https://www.nexusmods.com/granbluefantasyrelink/mods/561?tab=logs)
- [社区联机记录：同房出现多只彩虹史莱姆，玩家将其归因于 100% 生成 Mod](https://www.reddit.com/r/GranblueFantasyRelink/comments/1bykf34/)
- [Deep Rock Galactic 官方 Mod 支持 FAQ](https://www.deeprockgalactic.com/modding-support-faq)
- [Monster Hunter: World 自定义任务包（联机朋友需下载）](https://www.nexusmods.com/monsterhunterworld/mods/5461)
- [当前仓库：任务奖励倍率实现](../internal/backend/runtime_patch_task_reward_multiplier.go)
- 用户提供的 `GFR_v0.8.9_CHS_(Unofficial).CT`：XML `CheatEntry` IDs `31107`, `31109`, `31456`, `31490`, `33483`, `33487`
