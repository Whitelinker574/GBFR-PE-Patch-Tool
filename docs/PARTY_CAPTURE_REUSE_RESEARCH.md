# PlayFab Party 整局抓包对连接型功能的可复用性研究

日期：2026-08-01  
范围：只读分析本机整局抓包、当前仓库的队伍/配装/Logs 实现、PlayFab Party 1.10.12 官方 ABI，以及本机 `GBFR.ChatOverlay` 源码。本文不记录端点句柄、端点唯一号、Entity ID、网络描述符、玩家名或原始报文内容。

## 结论

这批数据不仅足以验证“连接生命周期与采集时机”，还已经解出了 2.0.3 Party 配装帧的核心字段。初步聚类完成后，又用任务结束后的本机只读配装快照、29 人 roster、武器表和因子表反查整局 payload，确认首次 `3:14 / 784 bytes` 与周期性 `2:63 / 780 bytes` 都携带完整角色资料块。

- **持续连接**：证据充分。任务阶段存在持续且高频的 Party 消息流；当前 `runtimeLoadoutDetectorSession` 的后台运行、进程重连和页面无关生命周期方向正确。
- **远端端点数量**：本局强烈支持“三个远端端点始终存在”，但抓包只保存了目标数量，没有保存端点身份，因此只能作为数量证据，不能当成员名册。
- **加入/离开**：本局抓包没有采集 `EndpointCreated` / `EndpointDestroyed` 等生命周期事件，无法精确还原加入和离开。消息静默不能替代销毁事件。
- **稳定队友身份**：payload 自带 0–3 的队伍槽位和官方角色 hash，本局四份资料恰好覆盖槽位 0/1/2/3；三条入站流可稳定映射为夏洛特、欧根和玛琪拉菲菈，本机出站流为槽位 1 的伊欧。它能作为会话内角色/槽位身份，但仍不能替代缺失的 sender endpoint 生命周期，也不能当永久玩家 ID。
- **角色/配装捕获时机**：首次成员同步直接交换四份完整资料；任务期间三名队友各自持续发送同结构的 780 字节资料帧。已经确认角色、武器、12 个因子物品 hash、12 个副词条 hash 与 12 个等级字段；尚未逐字段完成专精位图、召唤石、上限突破和面板数值的双运行对照。
- **断线恢复**：仓库目前能恢复游戏进程退出/重启，但网络会话边界仍主要靠“连续两次读不到队友”推断。官方 Party 生命周期事件可提供更明确的断线和重连门控。
- **Logs 联动**：`logs.db` 已经包含四名玩家、角色、因子、召唤石、技能、武器与战斗事件，是补齐装备 DTO 的更可靠来源。Party 会话适合提供时间边界，不能把 Party endpoint 直接等同于 Logs actor 或游戏 online slot。

## 证据基线

### 本局抓包

本机只读数据集：`%LOCALAPPDATA%\Temp\gbfr-party-capture-full-20260801-135349`。

聚合结果如下：

| 指标 | 结果 | 能证明什么 |
| --- | ---: | --- |
| 探针就绪至首条 Party 消息 | 约 141.447 秒 | 探针可在无流量时保持挂载；不是“已加入房间”的证据 |
| Party 消息活跃时段 | 约 413.874 秒 | 覆盖一次完整在线任务的主要网络阶段 |
| 出站 / 入站消息 | 59,396 / 87,620 | 数据量足够做消息族和时序聚类 |
| 出站 / 入站 payload | 约 2.88 MiB / 6.45 MiB | 捕获了完整 payload，而非只有 API 计数 |
| 进入稳定任务阶段后的最大消息间隙 | 77 ms | 任务内通信持续；不能替代连接存活 API |
| 连续相同出站 payload 恰好出现三次 | 18,946 / 19,238 组（98.482%） | 与三个远端目标分别定向发送高度一致 |
| 连续相同出站 payload 次数为三的倍数 | 99.174% | 少数相邻 tick 被合并后仍保留“三目标”规律 |
| 最终奖励入口 | 1 次 | 可作为本局任务结束的高价值时间锚点 |

所有出站记录的 `targetEndpointCount` 都为 1。按照官方 `SendMessage` 语义，这代表应用进行显式定向发送；相同 payload 连续三次是三次调用，不是 Party 的一次广播。它强烈支持本局存在三个远端目标，但**不能仅凭此断言本机是房主，也不能证明该消息是奖励权威数据**。[PartyLocalEndpoint::SendMessage](https://learn.microsoft.com/en-us/gaming/playfab/multiplayer/networking/reference/classes/partylocalendpoint/methods/partylocalendpoint_sendmessage)

### 三个稳定消息簇

任务实时阶段有一类 780 字节的入站消息，共 403 条。固定位置的短前缀在整局内恰好分成三个簇：

| 匿名簇 | 条数 | 首次出现（相对首条 Party 消息） | 最后出现 | 单簇最大间隔 |
| --- | ---: | ---: | ---: | ---: |
| A | 136 | 约 20.526 秒 | 约 410.481 秒 | 小于 30 秒 |
| B | 135 | 约 20.272 秒 | 约 410.485 秒 | 小于 30 秒 |
| C | 132 | 约 20.201 秒 | 约 410.483 秒 | 小于 30 秒 |

后续按 780 字节帧的固定字段重新解析，三个簇分别为角色 `FD3BE362`（夏洛特，槽位 0）、`DD7A151E`（欧根，槽位 2）和 `25D46F4B`（玛琪拉菲菈，槽位 3）；本机 `4D0A60C3`（伊欧，槽位 1）以三份定向出站帧发送。首次同步时，本机三份 `3:14 / 784 bytes` 出站帧后紧接三份对应队友的入站帧。任务期 `2:63 / 780 bytes` 中，三名队友分别出现 132、135、136 次，本机出现 399 次，恰好约为 133 轮乘三个目标。

两种消息的核心布局一致：武器 hash 位于 `0x1BC`，12 个因子物品 hash 位于 `0x1F4..0x223`，12 个副词条 hash 位于 `0x224..0x253`，12 个等级字节位于 `0x25C..0x267`，队伍槽位位于 `0x2B4`，角色 hash 位于 `0x2B8`。以本机资料为例，因子数组能逐项解析为浪迹天涯、追击、昏厥、狂战士、天星之止息、属性克制转换、伤害上限、天星之界、魔法师觉醒与战气等已知 2.0.2/2.0.3 因子；副词条区同样命中官方技能目录。三名队友的 12 槽也全部形成稳定、可解析的数组。

同一角色整局 780 字节帧只有少量计数位变化：四条流共同变化 `0x1A6..0x1A7`，部分流还变化 `0x1F1..0x1F3`、`0x269..0x26B` 或尾部 `0x30A..0x30B`；上述装备字段保持不变。因此产品应只对已确认的装备字段做稳定指纹，不应把整包摘要当作“换装”判断。

仓库已新增纯解析与稳定聚合模块 `internal/backend/runtime_party_network_profile.go` 及单元测试。它不安装 Hook、不连接网络，也不保存原始 payload；只接受已验证的两种消息族、精确长度、协议版本、0–3 槽位和 29 人 roster，并把上述动态计数位排除在装备指纹之外。聚合器按收发方向区分本机与远端，要求同一远端槽位连续三份装备指纹一致才暴露候选；装备变化后旧候选立即失效并重新稳定。对本次完整抓包回放时，它识别出本机网络槽位 1 和三个稳定远端槽位，6 份首次资料与 802 份周期资料全部通过。

稳定网络资料还能转为现有 `RuntimePatchPartyLoadout` 的“已验证核心候选”：角色、官方武器、12 个因子壳、由因子壳固定映射的主词条、资料帧中的副词条及存储等级进入同一 DTO；召唤石、技能、专精、上限突破和面板保持未记录。跨源匹配以角色、武器和逐槽因子指纹为准，故意不比较 Party 网络槽位与内存数组序号。这样后续内存或 Logs 提供更多字段时，可以沿用 detector 已有的字段超集更新，不必为网络来源另建一套历史/分享格式。当前运行时 Party 生命周期观察器已经接入 detector：候选通过三次稳定门槛并与内存快照按角色、武器及逐槽因子指纹一致后，会进入现有配装检测 UI；没有本局真实队伍资料时仍不能宣称已完成端到端捕获验收。

这也纠正了一个容易造成误存的假设：本局本机网络槽位是 1，不是固定槽位 0。因此不能把网络 `partyIndex` 直接套到代码的 `player/party1/party2/party3` 固定角色名上；未来融合必须以“出站资料帧属于本机、入站资料帧属于远端”为会话内身份依据，再与内存 party slot 做受控匹配。否则可能排除真正的槽位 0 队友，反而把本机槽位 1 记录成队友。

### 抓包明确没有的内容

- 探针只保留 `EndpointMessageReceived`（state change 21），没有保存 `EndpointCreated`（12）、`EndpointDestroyed`（13）、远端设备加入/离开或网络销毁事件。
- 记录只有收发数量、options、payload 大小/摘要和原始 payload，没有 sender/target endpoint 的会话内标识。
- 原探针没有同步保存同一时刻的队伍内存快照；任务结束后的本机只读快照与目录反查已经足以定位角色、武器和因子核心字段，但召唤石、专精、上限突破和面板字段仍缺少严格的同步 A/B 对照。
- 91,534 个不同的收发 payload 中没有发现 `PL####` 角色码、JSON、自描述文本或明显的通用压缩文件头。这只能说明协议不是自描述文本，不能证明数值型配装 DTO 不存在。
- 没有覆盖队友中途加入、离开、断网后重连、同角色队友、切角色或连续第二场任务。

## 当前仓库已经做对的部分

### 持续后台运行与进程恢复

`internal/backend/runtime_loadout_detector.go`：

- 第 22–24 行定义 2.5 秒后台轮询、两次稳定确认和两次缺席确认。
- `startRuntimeLoadoutDetector`（约第 199 行）创建独立后台 session；`run`（约第 317 行）使用 ticker 持续运行。
- `tick`（约第 384 行）在游戏未启动时等待，启动后以只读句柄连接；句柄失效后关闭并重新等待游戏。
- 前端页面卸载不会停止服务；`internal/backend/app.go:229-230` 会按用户保存的开关在应用启动时恢复服务，`beforeClose` / `shutdown` 才统一收尾。

`internal/backend/readonly_game_process.go:30-70,178-197` 将连接绑定到 PID、creation time、模块基址与版本 guard，并在每次采集前重新校验。这比用窗口标题或单独 PID 判断可靠，应继续保留。

因此，“点击开启后持续运行，直到用户主动关闭，并跨页面保持”的基础生命周期已经存在。本局连续网络流证明这种后台服务形态与真实长任务相符，不需要把服务绑定到某个 Vue 页面。

### 配装读取的真实性边界

`internal/backend/runtime_party_monitor.go` 和 `runtime_party_loadout.go` 没有从网络包猜装备：

- `readStableRuntimePatchPartySnapshots`（`runtime_party_monitor.go:159`）要求连续三次队伍拓扑一致，并要求角色、武器和因子构成的稳定键一致。
- `resolveRuntimePatchPartyLoadoutHandleWithLayout` 同时核对 party handle 的实体指针、表项 ID 和指定实例。
- `validateRuntimePatchPartyLoadoutSlot`（`runtime_party_loadout.go:271`）要求在线配装记录绑定到正确 party slot，避免把本机 profile 误配给正在填充的队友槽。
- 实际字段来自受限块：面板数值、武器、因子、上限突破，以及扩展的技能、角色 hash、专精等级、召唤石和专精节点；每层都有目录和合理性校验。

抓包现在可以作为角色、武器和十二因子的第二来源，并与这套内存读取互相校验；专精、召唤石、上限突破等未完成字段仍以受限内存布局为准，不能因为同一大包中存在未知区域就猜测完成。

### 本地历史的隐私控制

`runtimeLoadoutDetectorMembers` 会排除本机和 companion，并且现在还要求候选配装的运行时 `Online` 标志为真，避免离线/NPC 槽位进入新的队友历史。持久化记录不包含 PID、模块基址、内存地址或运行时根指针；`runtime_loadout_detector_test.go` 也明确检查这些字段不能进入历史 JSON。已有本地历史不会在只读加载时被擅自删除。引入 Party 身份层后，应延续这一原则：历史记录只留角色与配装内容；会话内 endpoint 映射在断线后丢弃。

## 可以安全完善的功能

### 复用范围总表

| 功能 | 本次数据能直接提供 | 仍需其他来源 | 当前落地状态 |
| --- | --- | --- | --- |
| 持续队友配装检测 | 收发方向区分本机/远端；网络槽位；角色；武器；12 因子与等级；装备变更指纹 | sender endpoint；内存侧剩余字段；真实在线队伍端到端验收 | 解析、三次稳定、部分 DTO、生命周期观察与 detector 融合已实现 |
| 配装预览、导出、分享码、上传 | 可复用现有 `RuntimePatchPartyLoadout` 和规范分享转换；缺失范围能保持未记录 | 用户主动确认公开上传；完整 DTO 需要内存或 Logs | 后端数据形态与 UI 路径已统一；网络候选需本局真实队伍资料通过稳定门槛后才显示 |
| Logs 战斗档案联动 | 任务时间窗、四人角色集合、武器/因子指纹 | 同局 `logs.db`、actor 与 party slot 的无歧义匹配 | 融合规则明确；本机没有本局对应 Logs 数据，未做伪匹配 |
| 任务开始/结束门控 | 首次资料同步、周期资料流、结算入口时间锚点 | 官方 Party lifecycle 或已验证游戏任务状态 | 可辅助门控，不能用“最后一包”替代精确任务结束 |
| 伤害归属 | 角色/网络槽位候选 | 每条伤害事件的 actor/entity/owner 与槽位关联，宠物/召唤物所有者链 | 本次数据不足，不能把全局伤害包装成本机/队友 DPS |
| 队内奖励同步 | 能证明配装 DTO 可见，且奖励物品 ID 不在同一消息域 | 房主/客机成对差分、共享任务状态或奖励载荷、PS 端重启回读 | 本次数据进一步证明它与配装链分离，不能据此开启“队内通用”承诺 |

对分享与上传必须继续遵守已有产品契约：网络捕获生成的是本地候选；只有用户明确选择并确认公开上传时才调用线上图鉴。不能因为后台检测已经开启，就自动公开队友资料。

### 1. 连接状态门控

安全实现不是另开线程调用 `PartyStartProcessingStateChanges`，而是 Hook 游戏原本的 Start/Finish 调用：

1. 原始 `StartProcessingStateChanges` 只调用一次。
2. 在原始批次仍有效时复制需要的生命周期字段和 payload。
3. 原数组、原指针、原顺序交给游戏。
4. 原始 `FinishProcessingStateChanges` 正常归还批次后，后台只处理复制出的不可变快照。

官方要求每个 state change 必须恰好归还一次，资源仅保证在归还前有效；`StartProcessingStateChanges` 还会使此前 `GetEndpoints` 返回的数组失效。[StartProcessingStateChanges](https://learn.microsoft.com/en-us/gaming/playfab/multiplayer/networking/reference/classes/partymanager/methods/partymanager_startprocessingstatechanges)；[PlayFab Party 异步操作与通知](https://learn.microsoft.com/en-us/gaming/playfab/multiplayer/networking/concepts-async-operations)

本机社区源码 `GBFR.ChatOverlay@56cfcf5a37435a78598118dcf28d9fbc481b4744` 的 `Native/PartyLifecycleProbe.cs:371-542` 已实现这类非消费观察范式，可借鉴生命周期所有权设计。它没有解析队友 payload 或远端成员映射，不能直接当产品实现。

### 2. 精确端点数量与加入/离开

应观察并复制：

- `EndpointCreated`：network + endpoint。
- `EndpointDestroyed`：network + endpoint + reason + error。
- `RemoteDeviceJoinedNetwork` / `RemoteDeviceLeftNetwork`：用于设备层连接诊断。
- `NetworkDestroyed` 和本地 endpoint/user teardown：结束整个会话。

官方结构定义见本机 Party 1.10.12 `Party_c.h:666-680`，对应官方文档：[EndpointCreated](https://learn.microsoft.com/en-us/gaming/playfab/multiplayer/networking/reference/structs/partyendpointcreatedstatechange)；[EndpointDestroyed](https://learn.microsoft.com/en-us/gaming/playfab/multiplayer/networking/reference/structs/partyendpointdestroyedstatechange)。

`PartyNetworkGetEndpoints` 可以枚举本机当前可见端点，但返回数组会被下一次 `StartProcessingStateChanges` 或成功的 `CreateEndpoint` 使失效，所以只能在安全批次内读取并复制分类结果。[PartyNetwork::GetEndpoints](https://learn.microsoft.com/en-us/gaming/playfab/multiplayer/networking/reference/classes/partynetwork/methods/partynetwork_getendpoints)

### 3. 会话内稳定身份

推荐的临时键为：

```text
sessionToken + endpointUniqueIdentifier
```

- `PartyEndpointGetUniqueIdentifier` 返回 16 位、仅在当前 network 内唯一、网络中各设备一致的 ID；跨 network 不唯一。不同设备看到新 endpoint 的时序可能不同，所以消息可先暂存，等 `EndpointCreated` 后补映射。[PartyEndpoint::GetUniqueIdentifier](https://learn.microsoft.com/en-us/gaming/playfab/multiplayer/networking/reference/classes/partyendpoint/methods/partyendpoint_getuniqueidentifier)
- `PartyEndpointGetDevice` 可把同一设备上的多个 endpoint 分组。
- `PartyEndpointGetEntityId` 可能为空，并且是 PlayFab Entity ID，不是 SteamID、PSN ID 或游戏 online slot；不应持久化原值。

这个键只用于一次 Party network 会话。断线重连必须生成新 `sessionToken`，不能让同一个 16 位 endpoint ID 跨会话延续身份。

### 4. Party endpoint 与游戏槽位的关联

不能假设“端点创建顺序 = party1/party2/party3”。建议采用逐级证据：

1. Party 生命周期层确认当前远端 endpoint 集合。
2. 游戏内存层确认 party slot、handle entity、handle ID、角色 hash 与 online 标志。
3. 在成员加入、角色切换或任务开始后的短窗口内，按 `senderEndpoint` 给消息族分组。
4. 只有当端点集合、槽位集合和角色/配装连续三快照都稳定时，建立会话内候选映射。
5. 如果存在同角色队友、成员数量不一致、两个候选都能匹配或消息先于 `EndpointCreated`，保持“未关联”，不能借槽位顺序猜测。

本局 payload 自带槽位和角色 hash，因此已经能完成“消息流 → online slot → 角色”的核心关联。仍缺的是 `senderEndpoint → 该消息流` 的显式绑定，以及队友中途加入/离开时的生命周期重建。

### 5. 角色/配装捕获时机

本局在首条 Party 消息后约 6.3–6.6 秒出现一组短时成员同步候选消息族，约 20.1 秒后出现持续到结算前的任务实时消息族。这提供了两个合理窗口：

- **成员同步窗口**：远端 endpoint 集合稳定后，开始尝试三快照内存读取。
- **任务实时窗口**：实时消息族稳定、party slots 齐全后，记录本场队友配装候选；若后续同一角色/武器不变而更多字段变为可用，只允许用“字段超集”完善当前记录。

当前 `observeActiveTeamChange` 和 `runtimeLoadoutDetectorMembersMoreComplete` 已按字段超集更新，不允许换角色、换武器或降低已知数值后覆盖旧候选，这一点应保留。

任务结束边界优先使用已经命中的游戏结算入口或另一个可验证的任务状态，而不是仅靠 Party 流量静默。本局结算入口之后仍有少量 Party 消息，说明“最后一条消息时间”不是精确结算点。

### 6. 断线与恢复

当前 detector 用连续两次缺席清空 active fingerprint，即约 5 秒。这可以作为内存侧兜底，但不应是唯一网络边界：

- `EndpointDestroyed`：只移除对应远端候选，保留其他队友。
- 本地 endpoint/user teardown、`LeaveNetworkCompleted` 或 `NetworkDestroyed`：立即结束 Party session，清空所有 endpoint-to-slot 临时映射。
- 重连后重新生成 session token，重新等端点与三快照稳定。
- Party 层失去连接、但游戏进程仍在时，不要关闭后台 detector；回到 `waiting_party`，保持服务启用并等待新会话。
- 游戏进程退出或 PID/creation time 改变时，沿用现有只读进程重连逻辑，同时清空 Party session。

本局没有断线段，以上恢复策略来自官方生命周期语义和现有进程生命周期实现，仍需要一次“队友中退 + 重连 + 第二场”的实测。

## 与 Logs 采集的组合方式

`internal/backend/logs_loadout_import.go` 的协议 v1 `logsLoadoutEncounter` 已包含 `[4]*logsLoadoutPlayer`；单个玩家包含 actor index、显示名、角色类型、因子、召唤石、技能、武器、专精、上限突破和面板数值。数据库读取使用只读 DSN、8 MiB 单记录上限、zstd 内存/窗口上限和 CBOR 解码边界。

安全融合顺序：

1. 以经过验证的任务开始/结算时间生成 `taskEpoch`。
2. Party 层只提供会话是否活动、远端端点数量和匿名会话键。
3. 运行时内存提供 party slot、角色 hash 和稳定配装候选。
4. Logs 提供同一战斗记录内的 actor、角色与完整配装 DTO。
5. 通过任务时间、队伍人数、角色 hash、武器/因子指纹联合匹配；出现同角色同配装或时间歧义时不自动绑定。

不要使用原始玩家名、Entity ID 或 endpoint unique ID 作为跨来源永久主键。Logs actor index 也只保证在对应战斗记录内有意义。

本局抓包目录内没有配套 `logs.db` 记录或同步内存快照，因此只能给出融合设计，不能声称已经完成一次真实三源匹配。

## 建议的最小下一轮实验

无需再次抓取整局所有消息。更高信息量的只读实验是：

1. 捕获所有 Party lifecycle state change，但只保存类型、匿名会话序号、匿名 endpoint 序号和时间，不保存 Entity ID/descriptor。
2. 对候选消息族只保存 `(会话匿名端点, 消息族, 大小, 摘要, 时间)`；原始 payload 仅在临时目录短期保留。
3. 同时每 2.5 秒保存脱敏后的 party slot、角色 hash、武器 hash、12 槽因子指纹和三快照是否稳定。
4. 做一次明确操作序列：房间内等待 → 队友加入 → 进入任务 → 一名队友切角色或更换一项装备 → 完成任务 → 一名队友离开 → 同队再开一场。
5. 只有当某一消息字段随单一受控变化唯一变化，并在第二次独立运行复现，才把它升级为 DTO 字段。

这一步能直接验证端点到槽位、角色切换、任务边界和断线恢复；比继续无标签地积累数十万条 payload 更有效。

## 官方与源码依据

- Microsoft 官方 NuGet：[`Microsoft.PlayFab.PlayFabParty.Cpp.Windows` 1.10.12](https://www.nuget.org/packages/Microsoft.PlayFab.PlayFabParty.Cpp.Windows/1.10.12)。本机 `Party_c.h` SHA-256：`4499E48F781A2E737B8B6068D7A62B434B3656EA1F0B8DA631FAB7B43F661DB0`。
- [`PartyEndpointMessageReceivedStateChange`](https://learn.microsoft.com/en-us/gaming/playfab/multiplayer/networking/reference/structs/partyendpointmessagereceivedstatechange)：接收端结构与 payload 生命周期。
- [`PartyLocalEndpoint::SendMessage`](https://learn.microsoft.com/en-us/gaming/playfab/multiplayer/networking/reference/classes/partylocalendpoint/methods/partylocalendpoint_sendmessage)：定向/广播、多个 data buffer 与交付边界。
- [`PartyManager::StartProcessingStateChanges`](https://learn.microsoft.com/en-us/gaming/playfab/multiplayer/networking/reference/classes/partymanager/methods/partymanager_startprocessingstatechanges) 与 [异步操作/通知](https://learn.microsoft.com/en-us/gaming/playfab/multiplayer/networking/concepts-async-operations)：state change 所有权、有效期和线程安全。
- [`PartyEndpointCreatedStateChange`](https://learn.microsoft.com/en-us/gaming/playfab/multiplayer/networking/reference/structs/partyendpointcreatedstatechange)、[`PartyEndpointDestroyedStateChange`](https://learn.microsoft.com/en-us/gaming/playfab/multiplayer/networking/reference/structs/partyendpointdestroyedstatechange)、[`PartyNetwork::GetEndpoints`](https://learn.microsoft.com/en-us/gaming/playfab/multiplayer/networking/reference/classes/partynetwork/methods/partynetwork_getendpoints)：成员集合和加入/离开。
- [`PartyEndpoint::GetUniqueIdentifier`](https://learn.microsoft.com/en-us/gaming/playfab/multiplayer/networking/reference/classes/partyendpoint/methods/partyendpoint_getuniqueidentifier)：network 内稳定、跨 network 不稳定的 16 位标识。
- 本机社区源码：`D:\gbf\_community-scan-20260727\GBFR.ChatOverlay`，commit `56cfcf5a37435a78598118dcf28d9fbc481b4744`。可借鉴 `Native/PartyLifecycleProbe.cs` 的非消费观察流程和 `Native/PartyRoomSessionTracker.cs` 的本地会话 teardown；该项目没有现成的远端 endpoint 到游戏槽位映射。
