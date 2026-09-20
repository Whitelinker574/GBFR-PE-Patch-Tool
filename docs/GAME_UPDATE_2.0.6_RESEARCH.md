# 游戏 2.0.6 更新与上游适配研究（2026-09-21）

## 范围与结论

## 本机文件核对与实现结果

- Steam Build ID：`24955526`；游戏 EXE 文件版本 `2.0.6`，大小 `123520480`，SHA256 `4B3ACE0FD03DF5D3D08C6453E12DF2279C6259ABF7D2FB3622921FCBEA775145`。
- `data.i` SHA256：`0A6B9989F0F0E59C4E67D5128A115DFB12560397A3421BDCCFB65D1DEBBB73F9`。使用现有 GBFRDataTools 2.0.0 解包，2.0.5/2.0.6 各 2120 份 system/table 文件，无新增/删除，仅 `item_junk_appear_rate.tbl` 变化。
- 数据库逐行对照（保留重复 Key）：4518 行中只有行 2066、2068、2073、2074、2076 的 GemLevel 从 0 改为 15，对应 GEEN_146_04、150_04、156_04、157_04、159_04。原表 0 与公告中的表现 Lv11 不应混为同一个存储值。11 张现有内置掉落部署输入表均保持一致，无需替换。
- 83 个运行时目录站点逐项验证。014/028/035/060 的 RIP 相对原字节采用独立 2.0.6 变体；054_11 的旧泛化签名出现两个命中，其中一个位于其他指令内部，现使用完整指令上下文将目标唯一定位至 0x4012558。
- 36 条内置 DLL AOB 均唯一命中；角色面板、队伍、物品保存、坐标/跳跃、虚拟因子 getter/分支、QOL 与额外武器技能管理器迁移到独立 2.0.6 布局。虚拟因子分支落点为参数准备段 0xA205F0，不是其后 CALL 指令。
- OD 现在拥有主路径与两条内联路径的独立恢复记录；尾声分别校验，部分安装失败会恢复已安装站点。测试覆盖仅辅助安装、主入口和一个辅助安装、完整安装、辅助被外部改动后的恢复状态保留及重试。
- 修复旧版前端只列出 2.0.2/2.0.3 的运行时响应校验遗漏，补齐 2.0.4–2.0.6。共享进程发布时保留已验证的 EXE 摘要，因子/祝福入口地址相同也能选择各自的正确保存函数。
- 本轮未启动游戏进入任务，不能将文件/AOB 校验和本进程 Hook 恢复测试描述为任务内实机效果验证。

## 外部来源核对

本报告核对官方公告、上游公开 GitHub API/source diff，以及本地 `b888939`（工具 v2.0.22）的实现；不把上游注释中的 CE 验证当成本机实测。本机 2.0.6 EXE、数据表差分与运行时安装/恢复证据由单独验证流程补充。

- 游戏 2.0.6 于 2026-09-17 发布；公告涉及连动武器、世界首领伤害修复、五种单等级因子的 Lv11→Lv15 修复，以及巴西评级。没有宣布新角色、召唤石或存档格式变更。[官方公告](https://relink-ragnarok.granbluefantasy.com/en/updates/408/)
- 上游 v1.10.4 与 master 均为 `b67bce7704719e331359c1a3393ec8f090bcc192`，比 v1.10.3 多 5 个提交。最重要的本地遗漏不是版本字符串，而是 **第二条通用内联 OD 累计路径及其不同的函数尾声**。[比较](https://github.com/BitterG/GBFR-PE-Patch-Tool/compare/v1.10.3...b67bce7704719e331359c1a3393ec8f090bcc192)
- 上游聊天和 defense_multiplier 的更新有参考价值，但本地没有对应聊天发送 Hook / PatchDefenseHook，不应为了追齐提交额外引入功能或把上游的伤害语义替换本地语义。

## 官方变更与工具影响

以下事实均来自 [Cygames 英文 2.0.6 公告](https://relink-ragnarok.granbluefantasy.com/en/updates/408/)；影响分析为本报告推断。

1. 同一设备、同一用户保存两款游戏的数据后，可以在榭洛杂货屋领取与 Versus: Rising 连动的 Karmic Sword of the Apocalypse。应比对武器、物品、连动领取标记相关数据，但不能从公告推断具体 ID 或自动写入领取状态。
2. 修复 New World Order / On the Threshold of The World 中 Celestial Sphere 导致意外伤害。伤害 Hook 必须重新核验，但公告没有公开具体代码地址。
3. War Elemental、Untouchable、Potent Greens、Roll of the Die、Flight over Fight 有时以 Lv11 获得的问题已修复；库存中的受影响因子会调整到 Lv15。现有数据校验/存档编辑不能把官方修复后的等级重新降回11；也不能把所有 Lv11 因子一律改为15。公告未提供字段布局，必须按因子身份及表真值限定。
4. 巴西 ClassInd 评级由12调整为14。没有直接的工具运行时适配需求。

## 上游提交精确清单

GitHub API 于本次研究时返回：v1.10.3 基线 `8b59dde6f66cab2b9cc10c28a428871cd75f8ce5`；v1.10.4 于 `2026-09-17T09:36:29Z` 发布。[发布页](https://github.com/BitterG/GBFR-PE-Patch-Tool/releases/tag/v1.10.4)、[标签 API](https://api.github.com/repos/BitterG/GBFR-PE-Patch-Tool/git/ref/tags/v1.10.4)、[master API](https://api.github.com/repos/BitterG/GBFR-PE-Patch-Tool/git/ref/heads/master)

| UTC 时间（2026-09-17） | 提交 | 实际修改 |
| --- | --- | --- |
| 07:19:53 | [a489d5fe4fd105ced9f80fae314124d7bc793fbb](https://github.com/BitterG/GBFR-PE-Patch-Tool/commit/a489d5fe4fd105ced9f80fae314124d7bc793fbb) | 更新受伤/OD 地址；新增通用内联 OD、不同尾声、延迟重试；主入口已 Hook 时仍安装辅助入口 |
| 07:39:06 | [9966ac992ef91542f120457298212e17e17a2147](https://github.com/BitterG/GBFR-PE-Patch-Tool/commit/9966ac992ef91542f120457298212e17e17a2147) | 更新2.0.6聊天地址 |
| 07:53:00 | [a3019de4eb22f2dd9461d85ec67c7fcdd4fe3111](https://github.com/BitterG/GBFR-PE-Patch-Tool/commit/a3019de4eb22f2dd9461d85ec67c7fcdd4fe3111) | 聊天改为 AOB 优先、严格/宽松签名、完整序言校验、代码段扫描；工程 UTF-8 编译 |
| 08:33:38 | [7ce91d61c027dcf6482ff459a2b61a508ccb4c75](https://github.com/BitterG/GBFR-PE-Patch-Tool/commit/7ce91d61c027dcf6482ff459a2b61a508ccb4c75) | 两个内联 OD 的带尾声签名；按可读内存区扫描；修复 LM_ADDRESS_BAD 未识别 |
| 08:50:04 | [b67bce7704719e331359c1a3393ec8f090bcc192](https://github.com/BitterG/GBFR-PE-Patch-Tool/commit/b67bce7704719e331359c1a3393ec8f090bcc192) | 受伤玩家识别通过 AOB→玩家→vtable→slot2 动态获取，并在玩家出现后刷新 |

## 地址与签名：候选证据，不是本机验证结果

来源：[上游最终源码](https://github.com/BitterG/GBFR-PE-Patch-Tool/blob/b67bce7704719e331359c1a3393ec8f090bcc192/src_dll/patch_core/dllmain.cpp)、[Go 配置](https://github.com/BitterG/GBFR-PE-Patch-Tool/blob/b67bce7704719e331359c1a3393ec8f090bcc192/app.go)。

| 功能 | 上游旧 RVA | 2.0.6 RVA | 定位依据 |
| --- | --- | --- | --- |
| 常规 OD 累计 | `0x2C6FD0`（上游原文，可能是历史笔误，不能照搬） | `0x22C75F0` | `80 79 50 00 74 13 48 03 51 18` |
| 巴布类内联 OD | `0x2B3F92E` | `0x2B3FFFE` | 19字节累计块 + `48 83 C4 20 5B 5F 5E C3` |
| 通用内联 OD | 上个版本未覆盖 | `0x2AAC2F2` | 同一累计块 + `48 83 C4 28 5F 5E C3` |
| defense_multiplier | `0x1FB890E` | `0x1FB8F2E` | `8B 86 D4 00 00 00 3D 00 E1 F5 05`，偏移+6 |
| 玩家 vtable slot2 指向函数 | `0x9CB360` | `0x9CB740` | 玩家指针链动态读取优先 |
| 聊天 sendMessage | `0x904D30` | `0x905110` | 严格/宽松序言 AOB |
| 聊天 Manager slot | `0x7C236E0` | `0x7C24720` | `mov rdi,[rip+disp32]` 引用解码 |

内联 OD 公共块：

```text
48 03 7E 18 48 C7 C0 FF FF FF FF 48 0F 43 C7 48 89 46 18
```

这是 add rdi,[rsi+18] / mov rax,-1 / cmovae rax,rdi / mov [rsi+18],rax。两个尾声的栈恢复不同：巴布路径 `add rsp,20; pop rbx; pop rdi; pop rsi; ret`，通用路径 `add rsp,28; pop rdi; pop rsi; ret`。只用公共19字节做扫描不能分辨两类路径；将旧洞穴原样用于新通用入口会错误恢复寄存器/栈。

玩家定位 AOB：`48 8B 0D ?? ?? ?? ?? 48 85 C9 74 ?? 48 8B 01 FF 90 C0 04 00 00 48 85 C0 74 ??`。上游称有2个匹配，均指向同一全局 slot；链为 RIP相对slot→player→vtable→[vtable+0x10]。此“两个匹配指向同一结果”需要逐个解码确认，不能直接取任意匹配视作唯一。

聊天严格序言：`41 57 41 56 41 55 41 54 56 57 55 53 48 81 EC F8 02 00 00 C5 F8 29 B4 24 E0 02 00 00 4D 89 CE 44 89 C5 48 89 D7 48 89 CE`。宽松版仅将两处栈布局立即数替换为通配符。Manager AOB：`48 8B 3D ?? ?? ?? ?? 48 8D 05 ?? ?? ?? ?? 48 89 44 24 38 48 C7 44 24 40 00 00 00 00 48 89 74 24 28 48 89 F1 E8 ?? ?? ?? ??`，slot = match+7+disp32。

## 本地实现对照与遗漏

以下定位针对修改前 `b888939`，后续实现可能改变行号。

### 必须处理：OD 三路径与生命周期

- `internal/backend/monster_enhance_safety.go:85` 的 `prepareOdRateInlineAuxiliaryHook` 只有203/204/205三个固定地址，且 `monsterEnhanceOwnedPatch` 仅支持一组 AuxTarget/AuxOriginal/AuxPatched/AuxCave。仅增加206地址仍会漏通用内联路径。
- `src_dll/patch_core/dllmain.cpp:1298` 的 `PatchOdRateHookInline` 固定生成巴布尾声；需要区分两类尾声并验证完整入口形状。
- 同文件约4989行 `inlineRvas` 只有 `0x2B3E7DE,0x2B3F77E,0x2B3F92E`。Go预检、DLL安装、Go接管/回读、停用恢复必须使用一致的两条辅助路径集合。
- 约4950行发现主入口首字节E9即提前continue；上游此次修复正是防止这种情况下跳过辅助入口。应按本地所有权验证机制处理，不能把任意E9当作已由本工具正确安装。
- 上游增加2400次×250ms后台重试，理由是内联代码页可能尚未解密。这只是上游经验；本地若采用延迟安装，必须由当前会话/owner token约束取消与回滚，不能在用户停用后继续异步打补丁。保留“原字节不符则拒绝”也是安全方案，但须准确报告未安装路径。

### 不直接移植：受伤识别和聊天

本地 `PatchMonsterDamageHook` 使用共享映射中最多8个已捕获玩家指针进行对象比较，不使用上游 `PatchDefenseHook` 的 player-only vtable slot2。因此 `0x9CB740` 不是本地必改常量；本地应验证捕获入口、玩家偏移、共享指针更新和真实受伤语义。上游 defense_multiplier 是对最终 eax 作除法，本地怪物伤害倍率路径具有自己的语义，不能仅按功能名混同。

本地未发现 `ResolveAutoChatTargets` / sendMessage 注入；app.go 中的 `group_chat` 是群疗药水而不是聊天。聊天三项提交不构成本地功能回归修复要求。

### 通用安全经验及上游残留风险

- `LM_SigScan` 的未找到值是 `LM_ADDRESS_BAD`，不仅是0；上游7ce91d6修复漏判。审查本地其他扫描路径时应核对这一点。
- 跨不可读页的整模块扫描会提前异常；上游改为 VirtualQuery 可读区扫描。但其实现不能替代本地唯一性、多匹配、页保护及边界校验。
- 上游 Manager slot 仍保留“仅模块范围验证”的未验证固定RVA回退；这不满足本地严格安全门槛，不应复制。
- 上游 `ScanAobReadable` 返回首个匹配；内联入口只校验19字节，固定RVA回退没有校验完整尾声；应在本地加严格尾声校验和所有权标记。
- 上游防御刷新线程会写运行中的洞穴立即数，且没有本地owner token生命周期；不应当成可直接粘贴的安全实现。

## 验证清单

1. 以本机 EXE SHA256 和 file/product version 建立206身份，不用上游地址替代本机字节验证。
2. 验证常规OD、巴布内联、通用内联三个入口及签名数量；两类辅助洞穴分别测试正确尾声、倍率更新、失败回滚、停用恢复、退出清理。
3. 确认新的数据表是否有武器/领取/因子等级相关差分；没有差分也要保留证据，不从公告猜表ID。
4. 原有功能继续逐项迁移，不以“OD已工作”概括所有实时功能支持。
5. 本报告没有对公开 Issue 留言、关闭 Issue、发布版本或提交远程修改。Issue25/23状态与后续沟通由主任务独立确认。
