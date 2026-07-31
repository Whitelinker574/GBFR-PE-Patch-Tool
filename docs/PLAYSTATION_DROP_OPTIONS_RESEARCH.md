# PlayStation 端指定掉落与奖励倍率可行性调研

调研日期：2026-08-01
调研范围：《Granblue Fantasy: Relink》PS4/PS5 零售环境、官方存档流转、联机与 Share Play，以及改机边界。本文只讨论官方资料能够支持的事实，不提供破解、绕过 DRM、存档解密或主机漏洞利用方法。

## 结论摘要

在未修改的零售 PS4/PS5 上，目前没有官方支持的方式运行 PC 内存补丁工具、编辑 Relink 存档内容、指定任意掉落，或把任务奖励统一乘以 2/4/8/16。PlayStation 官方提供的是存档的复制、云同步、整机备份和 PS4→PS5 转换，这些都是**搬运已有数据**，不是修改存档字段或游戏掉落逻辑的接口。

PS4 与 PS5 玩家可以互相联机，但 PlayStation 玩家不能与 Steam 玩家联机。因此，“由开启 PC 奖励倍率的 Steam 房主带 PS 玩家刷奖励”没有网络路径。Share Play 也只是让访客观看或远程操作房主正在运行的游戏，不会把房主的存档或奖励复制到访客账号。

对产品的直接建议是：

1. PC 版继续实现全任务奖励倍率和指定掉落，但 UI/文档必须明确标注“仅 PC/Steam 运行时有效”。
2. 不要宣传 PS4/PS5 支持、跨平台发奖、PS 存档导入编辑或“PC 房主给 PS 客户端发倍数奖励”；官方资料不支持这些能力。
3. PS 用户当前安全可用的福利只有游戏内正常联机奖励、官方发放/购买的 Add-Ons，以及官方允许的存档备份和 PS4→PS5 迁移。

## 场景判断

| 场景 | 指定掉落/奖励倍率是否可行 | 判断依据 |
|---|---:|---|
| 未修改的零售 PS5，直接运行工具 | 否 | PS5 没有官方接口加载 PC 补丁或改写游戏逻辑；单个 PS5 游戏存档也不能从 PS5 复制到 USB。 |
| 未修改的零售 PS4，USB 导出后修改 | 无官方支持路径 | PS4 可把单个存档复制到 USB，但 Sony 只把它定义为备份/迁移功能；“可复制”不能推导为“内容可编辑或可被本工具识别”。 |
| PS4 存档转到 PS5 | 只能迁移既有进度 | Relink 提供单向 `Convert Saved Data`；它不是 PC↔PS 跨存档，也不会新增倍率功能。 |
| Steam 房主开启倍率，PS 好友加入 | 不可行 | Relink 官方 FAQ 明确 Steam 与 PS4/PS5 不支持跨平台联机。 |
| PS4/PS5 房主对奖励逻辑做修改 | 零售环境无支持路径 | PS4↔PS5 虽可联机，但没有官方的主机端 mod/插件接口；也没有证据表明房主能合法改变其他客户端的结算奖励。 |
| Share Play 让好友操作 | 不能给好友存档发奖 | 访客操作的是房主会话；Share Play 不是存档、物品或奖励转移服务。 |
| 官方 DLC/特典 | 可以领取固定官方物品 | 通过 Siero's Knickknack Shack 的 `Claim Add-Ons` 领取；内容由发行方预先定义，不能自选任意掉落。 |
| 修改过的主机 | 技术上不能据官方资料作产品承诺 | 无第一方支持，涉及平台条款、安全、版本兼容与封禁风险；本文不提供破解或绕过步骤。 |

## 1. 联机与“房主倍率”

Relink 官方用户支持页面明确写明：PS4 与 PS5 用户可以一起游戏，但 PlayStation 用户不能与 Steam 用户一起游戏；PlayStation 在线多人还需要互联网连接、登录 PlayStation Network 和有效的 PlayStation Plus 订阅。[Cygames：跨平台联机 FAQ](https://relink.granbluefantasy.jp/en/usersupport/#i9ounfp2pp) [Cygames：PlayStation 在线游玩条件](https://relink.granbluefantasy.jp/en/usersupport/#h9qncicp3)

这直接排除了最简单的设想：在 Steam/PC 房主上开启任务奖励倍率，再邀请 PS4/PS5 好友进入房间。两个平台无法建立该联机房间。

Cygames 的玩法介绍说明，任务的目标和支线目标会在完成时奖励经验与宝物，而且这些任务可与其他玩家在线游玩；用户支持页进一步说明任务结束时在 Results 画面领取任务战利品。[Cygames：Gameplay—Online Co-Op](https://relink.granbluefantasy.jp/en/gameplay) [Cygames：任务结束与战利品 FAQ](https://relink.granbluefantasy.jp/en/usersupport/#yz21_oncuf)

但这些官方资料**没有**说明奖励数量完全由房主决定，也没有证明修改房主即可安全地让其他客户端得到相同的修改结果。因此即使只讨论 PS4↔PS5 联机，也不能把“房主开倍率后所有 PS 客户端一起收到倍率奖励”作为已验证功能。它至少需要在同平台的受控测试环境中验证结算权威、每一种奖励来源和异常退出行为；零售主机目前又没有受支持的补丁加载途径。

## 2. PS4→PS5 是单向转换，不是跨存档

Cygames 明确说明 PS4 与 PS5 的存档不能直接互用。PS5 版可以在标题画面使用 `Convert Saved Data` 将已有 PS4 存档转换为 PS5 存档；目标槽位由游戏自动选择，可能覆盖该槽位；PS5 创建的存档不能反向用于 PS4。[Cygames：PS4/PS5 存档兼容 FAQ](https://relink.granbluefantasy.jp/en/usersupport/#aeweeswijttb) [Cygames：Convert Saved Data FAQ](https://relink.granbluefantasy.jp/en/usersupport/#dilb8onw9h58)

游戏版本升级是另一件事：官方产品页说明 PS4 光盘版可以免费升级为数字 PS5 版，但游玩时仍需插入 PS4 光盘；数字 PS4 版也可以免费升级为数字 PS5 版。[Cygames：Products](https://relink.granbluefantasy.jp/en/products/)

因此可行的官方流程只有：

`PS4 已有存档 → 官方复制/云端搬到 PS5 → Relink 内 Convert Saved Data → PS5 存档`

它不能实现：

- Steam/PC 存档转为 PS4/PS5 存档；
- PS5 存档转回 PS4 后再编辑；
- 通过转换给存档加入新的掉落倍率逻辑；
- 把一个账号的奖励转给另一个 PSN 账号。

## 3. 云存档、USB 与数据迁移能做什么

### PS Plus 云存档

Sony 说明 PS5 可以把存档与 PlayStation Plus 云存储同步，也可以从云端下载。PS5 游戏可进行自动同步；PS4 存档上传云端后，在 PS5 上需要下载到主机存储。[Sony：PS5 cloud storage](https://www.playstation.com/en-us/support/subscriptions/ps5-ps-plus-cloud-storage/) PS4 也支持把存档上传在线存储和下载回来。[Sony：PS4 online storage](https://www.playstation.com/en-us/support/subscriptions/ps4-ps-plus-online-storage/)

云存档解决的是备份、同步和换机，不提供修改存档内容的 API，也不会把 Steam 存档转成 PlayStation 存档。

### PS4 单个存档 USB 复制

Sony 明确允许把 PS4 游戏存档复制到 USB，再复制到 PS5 或另一台 PS4；该功能在官方文档中被描述为备份与迁移。[Sony：Transfer data to PS5](https://www.playstation.com/en-us/support/hardware/transfer-games-saved-data-ps4-ps5/) [Sony：PS4 Saved Data in System Storage](https://manuals.playstation.net/document/en/ps4/settings/data_system.html)

这里必须避免一个错误推论：USB 上能看到/复制某个备份，不等于其内部字段是明文、允许编辑、与 Steam 版格式相同，或修改后能通过主机校验。本次没有找到任何 Sony 或 Cygames 第一方资料提供 Relink PS4 存档编辑、重新签名或 PC 工具导入的支持流程，所以不能把它列为产品能力。

### PS5 存档与 USB

Sony 明确写明：单个 PS5 游戏存档不能从 PS5 复制到 USB。PS5 可以做整机 USB 备份，备份可包含存档；但恢复会清除主机当前数据，备份与 PlayStation 账号绑定，恢复到另一台主机需要登录执行备份时使用的同一账号。[Sony：Transfer data to PS5](https://www.playstation.com/en-us/support/hardware/transfer-games-saved-data-ps4-ps5/) [Sony：Back up and restore PS5 data](https://www.playstation.com/en-us/support/hardware/back-up-ps5-data-USB/)

整机备份是灾难恢复功能，不适合作为逐项编辑或频繁导入掉落物的工作流；官方文档也没有提供修改其中单个游戏存档的方式。

## 4. Share Play 为什么不能传送奖励

Sony 对 Share Play 的定义是：房主可让一名访客观看、`Play as You`（代替房主操作）或 `Play with the Visitor`（一起操作支持的游戏）。一次只能有一名访客，每次会话最长一小时；部分场景和游戏可能受限制。只有房主获得奖杯。[Sony：PS5 Share Play](https://www.playstation.com/en-us/support/games/ps5-share-play/) PS4 官方说明同样采用一名访客/一小时模型，并支持 PS4 与 PS5 之间使用 Share Play。[Sony：PS4 Share Play](https://www.playstation.com/en-us/support/games/share-play-playstation/)

因此 Share Play 的真实用途是让朋友远程控制**房主的**游戏会话。任务进度、掉落和存档仍属于房主正在使用的账号/存档；它不是把奖励写入访客自己 Relink 存档的渠道。

## 5. 官方固定物品是唯一受支持的“额外物品”路径

Relink 官方 Add-Ons/特典通过 Siero's Knickknack Shack 的 `Claim Add-Ons` 领取。遇到问题时，官方排查方向是确认 PSN 购买、恢复许可证或重新下载内容。[Cygames：领取 Add-Ons](https://relink.granbluefantasy.jp/en/usersupport/#6uit7swv5) [Cygames：Add-Ons 故障排查](https://relink.granbluefantasy.jp/en/usersupport/#v3xjibzfl)

这证明零售 PS 主机可以领取**发行方签发并预先定义**的奖励，但不构成第三方工具注入任意物品的接口。若未来要为 PS 玩家提供合规福利，只有发行方合作、官方活动奖励或正式 DLC/兑换内容属于可持续路线；普通工具开发者无法自行调用这一发行渠道。

## 6. 修改过的主机：不应作为发布功能

本次没有找到 PlayStation 或 Cygames 第一方资料，说明修改过的 PS4/PS5 可以受支持地运行 Relink 内存补丁、编辑掉落表、为其他玩家发放任意物品，或把修改存档安全带回零售在线环境。

Sony 的 PlayStation 服务条款第 10.3 节禁止修改、改编、逆向工程、反编译或反汇编 Content；第 10.6 节禁止绕过或规避加密、安全、DRM 或认证机制。条款也对作弊和以技术手段操纵虚拟物品作出限制。[Sony：PlayStation Terms of Service](https://www.playstation.com/en-us/legal/psn-terms-of-service/)

所以即便社区中存在特定固件、离线主机或非官方存档工具，也不能据此承诺：

- 普通零售主机可用；
- 可安全登录 PSN 或联机；
- 不会损坏存档或触发账号/主机处罚；
- 游戏更新后仍兼容；
- 能把奖励可靠写给未修改主机上的其他玩家。

本文不提供任何破解、解密、重新签名、DRM 绕过、漏洞利用或规避平台检测的操作说明。

## 7. 对当前工具的实施边界

### 可以做

- 在 PC/Steam 版中，把“所有任务奖励倍率”做成明确开关，并覆盖实际验证过的奖励来源。
- 给倍率范围、上限、溢出和断线结算写自动测试/运行时保护。
- 在发布说明和 UI 明确注明平台范围，避免 PS 玩家误购或误解。
- 为 PS 用户提供官方联机、存档备份、PS4→PS5 转换和 Add-Ons 领取说明。

### 现在不能承诺

- Steam 与 PS4/PS5 跨平台联机发奖；
- PC 端指定掉落同步到 PS 好友存档；
- 将 Steam 存档转换为 PlayStation 存档；
- 在零售 PS4/PS5 上运行本工具；
- PS 房主开启倍率后，所有 PS 客户端稳定获得相同倍率；
- 通过 Share Play、云存档或 USB 备份完成任意物品注入。

## 来源清单

所有来源均为发行方或平台方第一方页面，访问日期为 2026-08-01：

- [Cygames：Granblue Fantasy: Relink User Support](https://relink.granbluefantasy.jp/en/usersupport/)
- [Cygames：Granblue Fantasy: Relink Gameplay](https://relink.granbluefantasy.jp/en/gameplay)
- [Cygames：Granblue Fantasy: Relink Products](https://relink.granbluefantasy.jp/en/products/)
- [Sony：Transfer data from PS4/PS5 to PS5](https://www.playstation.com/en-us/support/hardware/transfer-games-saved-data-ps4-ps5/)
- [Sony：PlayStation Plus cloud storage for PS5](https://www.playstation.com/en-us/support/subscriptions/ps5-ps-plus-cloud-storage/)
- [Sony：PlayStation Plus online storage for PS4](https://www.playstation.com/en-us/support/subscriptions/ps4-ps-plus-online-storage/)
- [Sony：PS4 Saved Data in System Storage](https://manuals.playstation.net/document/en/ps4/settings/data_system.html)
- [Sony：Back up and restore PS5 console data](https://www.playstation.com/en-us/support/hardware/back-up-ps5-data-USB/)
- [Sony：Share Play on PS5](https://www.playstation.com/en-us/support/games/ps5-share-play/)
- [Sony：Share Play on PS4](https://www.playstation.com/en-us/support/games/share-play-playstation/)
- [Sony：PlayStation Terms of Service](https://www.playstation.com/en-us/legal/psn-terms-of-service/)
