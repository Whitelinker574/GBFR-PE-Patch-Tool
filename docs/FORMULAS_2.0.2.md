# GBFR 2.0.2 配装数值证据笔记

本文记录配装编辑器当前采用的数值来源、已逆向的运行时代码路径、真实存档回归点，以及仍未证明的部分。目标不是把“页面能算出一个数”当成正确，而是让每一层公式都能追溯到 2.0.2 EXE、解包表或存档字段。

## 版本指纹与证据等级

- 游戏：Granblue Fantasy: Relink 2.0.2
- EXE：`granblue_fantasy_relink.exe`
- SHA-256：`63340832BCF731FBC97796F686B05C988418E83D451D4A49B2244A85D00E297F`
- 真实回归存档：`Saved/SaveGames/SaveData2.dat`（常规测试只操作临时副本；显式 opt-in 测试才会原样写回并创建整组备份）

当前机器保存有一份独立的 2.0.2 EXE，作用仅是做指令字节、RVA 和版本守卫的静态核对。这里没有完整游戏资源，也没有正在运行的游戏进程，因此静态证据不能冒充真实进程 E2E。

证据等级：

- **A / 已闭环**：EXE 指令、解包表和真实存档至少两者互相印证，舍入点也已定位。
- **B / 表与存档已验证**：字段/哈希联表和真实存档一致，但还没有追到最终运行时消费者。
- **C / 估算**：为了页面可用而保留的静态计算模型；未命中的分组或舍入必须显式保留为未验证。

## 角色基础属性（A）

角色基础表 writer 位于 RVA `0x2BC830–0x2BCFEF`，wrapper 位于 `0x2EF6E0–0x2EF78C`。writer 从角色状态表容器按角色哈希、等级区间查找相邻关键帧，并对各列做单精度线性插值：

```text
v = low + (high - low) * (level - lowLevel) / (highLevel - lowLevel)
```

字段与运行时对象映射：

| 表记录字段 | 运行时对象 | 舍入 |
|---|---:|---|
| `row+0x00` HP | `object+0x5A28` | `vcvttss2si`，向零截断 |
| `row+0x04` 攻击力 | `object+0x5A2C` | `vcvttss2si`，向零截断 |
| `row+0x0C` 昏厥值 | `object+0x5A34` | 保留 `float` |
| `row+0x10` 暴击率 | `object+0x5A38` | `vcvttss2si`，向零截断 |

真实存档与解包表把角色的基础值和永久成长拆开保存：

| 存档 IDType | 伊欧真实存档值 | 含义 / 联表方式 |
|---:|---:|---|
| `1309` | `3156` | 基础 HP（`i32`） |
| `1310` | `666` | 基础攻击力（`i32`） |
| `1312` | `0x41000000` → `8.0` | 基础昏厥值（IEEE-754 `float32`），不是 Fate 加成 |
| `1313` | `5` | 基础暴击率 |
| `1318` | `0x7FF` | Fate / 命运剧情完成 bitmask；联 `chara_status_fate` 累计 `HP +640 / ATK +165` |
| `1323` | `3,309,499` | 累计 MSP（Master 经验）；联 `chara_master_exp` 得到索引并钳制到 Master Lv50，再联 `skillboard_unlock` 累计 `HP +6000 / ATK +3000 / DmgCap +100` |

因此伊欧在不含武器、因子、召唤石、上限突破和条件效果时的永久基础线为：

```text
HP   = 3156 + 640 + 6000 = 9796
ATK  =  666 + 165 + 3000 = 3831
Stun = 8
Crit = 5
```

编辑器离线模型按这三层证据构造基础线；不能只读取 `1309 / 1310`，也不能把 `1312 / 1313` 错当 Fate 或 Master 加成。

## 武器面板（A）

### 基础等级插值

武器基础 writer 位于 RVA `0x3197E0–0x319DAD`。`weapon_status.tbl` 同样按相邻等级关键帧线性插值：

| 属性 | 关键指令 / 写入位置 | 舍入 |
|---|---|---|
| HP | `0x319B71` → `weapon+0x5C` | 向零截断 |
| 攻击力 | `0x319A60` → `weapon+0x60` | 向零截断 |
| 昏厥值 | `0x319C9B` → `weapon+0x64` | 保留 `float` |
| 暴击率 | `0x319D99` → `weapon+0x68` | 向零截断 |

### 觉醒、强化与超越

最终武器 HP / 攻击不是“查当前阶段一行”，而是阶段增量累加：

```text
weapon = base(level) + awake(1..A) + plus(1..P) + rebuild(1..T)
```

- 觉醒 `0x31C290–0x31C658`：先累计 `1..A`，再对 HP / ATK 总和向零截断；昏厥用 toward-zero `vroundss`。
- 强化 `0x31C6A0–0x31C8C9`：等级限制 `<=99`，**每一级增量先向零截断再相加**。
- 超越 `0x31C8F0–0x31CC25`：等级限制 `<=7`，累计 `1..T` 后对 HP / ATK 向零截断；表中的阶段行是增量，不是累计快照。

最终 reader 在 RVA `0x399DCC2–0x399DD05`：

```text
ATK = [weapon+0x60 base] + [weapon+0x7C awake]
    + [weapon+0x70 plus] + [weapon+0x90 rebuild]

HP  = [weapon+0x5C base] + [weapon+0x78 awake]
    + [weapon+0x6C plus] + [weapon+0x8C rebuild]
```

这里是四个 `i32` 直接相加，再传 UI setter，没有额外的最终 round / ceil / trunc。同一公式还在 `0x39A0AF2–0x39A0C52` 与 `0x3EF22D8–0x3EF2309` 独立出现。

当前 2.0.2 官方觉醒/超越表的昏厥与暴击增量均为零；因此尚未闭环的“强化昏厥最终消费者”不会影响当前武器数值，但不能据此假定未来表也永远为零。

### 真实回归点

- 伊欧存档 Slot 52 双蛇十字权杖：总计 `17083 ATK / 1149 HP`。
- Hercules / Tweyen 武器哈希 `91DDC1F1`，Lv150 基础攻击 `2299`；强化 +99 为 `198`，超越 T1..T5 为 `4000+3500+3000+2000+2000`，所以 T5 攻击为 `16997`。
- T7 四类超越序列的累计攻击增量分别为 `17000 / 14300 / 7000 / 15500`，测试逐阶段累加，防止再次把某一行误当累计值。

## 配装来源联表（B）

### 角色上限突破

- `1606`：属性哈希。
- `1607`：单 bit 等级；等级为 `trailingZeros(value)+1`。
- 已审计曲线包括 HP、攻击、暴击、昏厥、普通/能力/奥义上限等十级值。

### 召唤石

- `1451`：当前四个装备槽。
- `1456`：背包实例 SlotID。
- `1457`：召唤石类型。
- `1458`：主加护 / 副参数哈希。
- `1459`：主加护等级 / 副参数等级。
- `1460`：Rank。

副参数已审计的面板项：攻击、HP、暴击率、昏厥值、普通伤害上限、能力伤害上限、奥义伤害上限。编辑器只允许选择当前真实存档背包中四个不重复实例。

### 因子、武器技能与专精

- 同名词条先聚合实际等级，再按 `skill_status.tbl` 的等级曲线封顶；页面同时显示有效等级、实际投入和溢出等级。
- 武器技能来自武器实例及超越技能向量，不再只显示四个主动技能。
- 专精 R1 三方向可并存；R2 起形成唯一主方向，R3 必须沿用该方向。没有形成主方向时，R2/R3 的普通子词条仍可保存，但专精方向效果不应被伪造为生效。
- 已审计的因子联动节点：攻击类主因子每个 `+10% ATK`（最多 5 个）、基础类主因子每个 `+20% damage cap`（最多 5 个）、防御/支援类主因子每个 `+10000 HP`（最多 4 个）。只统计主词条。

## 最终人物面板：游戏回读与离线估算分轨

### 游戏自身最终值的只读回读（静态 EXE 字段路径与合成单元测试；当前机器待实机 E2E）

`runtime_character_panel.go` 不再尝试在应用侧复刻整条游戏公式，而是读取游戏自己的 2.0.2 最终面板对象：

| 状态对象偏移 | 类型 | 面板项 |
|---:|---|---|
| `+0x04` | `i32` | HP |
| `+0x08` | `i32` | 攻击力 |
| `+0x10` | `float32` | 昏厥值 |
| `+0x14` | `float32` | 暴击率 |

角色状态 manager 全局指针位于 RVA `0x7C24980`。读取器按游戏自身容器布局定位目标角色：

1. 从 `manager+0x08/+0x10` 读取角色 ID 向量起止。
2. 从 `manager+0xA30` 读取哨兵、`+0xA40` 读取 bucket 表、`+0xA58` 读取 mask；bucket 步长为 `0x10`。
3. 以 ID 查链表节点：`node+0x08` 为 next、`+0x10` 为 key、`+0x30` 为状态指针。
4. 只接受 `status+0x5EBC == 1`、`status+0x5EBE != 0` 且 `status+0x59F0` 角色 hash 精确匹配的记录。

读取前逐字节核对四个独立的 2.0.2 版本守卫；任一不匹配都按旧布局拒绝读取：

| RVA | 守卫覆盖的路径 |
|---:|---|
| `0xD4321` | manager 全局指针取得 |
| `0x2DC081` | hash map / bucket 查找 |
| `0x2DC11E` | 状态 ready 标记写入 |
| `0xA296F3` | 最终 HP、攻击、昏厥、暴击输出 |

同一角色必须连续取得 **3 份完全一致的快照**，结果才返回 `source=game_runtime_2.0.2` 和 `runtimeVerified=true`；角色状态变化、版本不符、容器损坏、NaN/Inf 或越界值一律失败关闭。进程句柄只有查询与 `PROCESS_VM_READ` 权限，不申请 `PROCESS_VM_WRITE`、`PROCESS_VM_OPERATION`，也不注入代码。

这条路径设计上读取游戏已经算完的值，所以不受应用离线公式缺口影响。当前证据仅证明读取器与 2.0.2 EXE 静态布局一致并通过合成内存单元测试；字段布局尚未由真实进程回读独立验证，因此不得标为 A / 已闭环，真实进程 E2E 尚未执行。

### 聚合器静态证据与永久成长修正（A/B）

最终聚合器位于 RVA `0xA25F80–0xA2B43E`。整数核心包括：

```text
HP core = status[5A50] + status[5A28] + status[5A44]
        + weapon[78] + weapon[5C] + weapon[6C] + weapon[8C]
        + 其他固定/效果贡献

ATK core = status[5A54] + status[5A2C] + status[5A48]
         + weapon[7C] + weapon[60] + weapon[70] + weapon[90]
         + 其他固定/效果贡献
```

旧笔记把 `status+0x5A44/+0x5A48` 写成“不明额外成长项”，现已由真实存档与 `chara_master_exp`、`skillboard_unlock` 的联表回归纠正：`+0x5A44/+0x5A48` 分别是 Master HP / ATK，`+0x5A50/+0x5A54` 分别是 Fate HP / ATK，`+0x5A28/+0x5A2C` 才是角色表基础 HP / ATK。以伊欧为例，Fate 与 Master 分别贡献 `640/165` 和 `6000/3000`，与 `1309/1310` 合成 `9796 HP / 3831 ATK` 的无装备基础线。

基础 HP 在 `0xA26713` 消费，初次转整数在 `0xA26981`；基础攻击在 `0xA2698F` 消费，初次转整数在 `0xA26BDC`。已识别的固定词条按顺序逐项加入已量化结果并再次向零截断：

| 面板项 | 指令地址 | 词条 hash / 名称 | 消费字段 |
|---|---:|---|---:|
| HP | `0xA26CA5` | `F372F096` / 体力 | effect `+0x08` |
| HP | `0xA26D35` | `D3B8C21F` / 终极钳蟹因子 | effect `+0x0C` |
| HP | `0xA26DC8` | `89C66ACB` / 相扑斗力 | effect `+0x18` |
| ATK | `0xA26E55` | `50079A1C` / 攻击力 | effect `+0x08` |
| ATK | `0xA26ED2` | `082033CB` / 钳蟹的共鸣 | effect `+0x0C` |
| ATK | `0xA26EE6` | `89C66ACB` / 相扑斗力 | effect `+0x14` |

`0xA26ED9` 在最后一项 ATK 前执行 `VROUNDSS imm8=0x0B`；上述路径最终都经过 `VCVTTSS2SI`。聚合器调用 `0x298830–0x298A45` 取得上游已换算成绝对值的 effect 贡献：HP 使用索引 `0/1`，ATK 使用 `4/5`。这些路径使用 scalar `float32`，所以离线模型也保留 binary32 精度。

### 离线配装草稿（C）

没有运行游戏，或用户正在编辑尚未写入游戏的草稿时，页面仍使用可解释的静态模型：

```text
HP  ≈ trunc32((角色永久基础 + 武器 + 无条件固定 HP)
              × 各独立无条件 HP 倍率
              × (1 + 专精 HP 百分比合计))

ATK ≈ trunc32((角色永久基础 + 武器 + 无条件固定 ATK)
              × 各独立无条件 ATK 倍率
              × (1 + 专精/联动 ATK 百分比合计))
```

所有离线 HP、攻击、暴击和昏厥结果都必须显示 `≈`，不能带 `runtimeVerified`。尚未闭环的仍是部分百分比 producer 的乘区、条件效果与中间 `float32` 量化顺序；条件依赖当前血量、蓄力、目标、战斗或队伍状态的效果默认不进入静态面板。

伤害上限分别保留普通、能力、奥义三项；顶部单值只用于比较三者共同增量，展开层显示真实分拆，不把不同上限混成一个虚假总数。

## 伤害公式与 2.0.2 变更边界

反射布局中 `DamageCalculateParam` 的关键偏移已定位：

| 偏移 | 含义 |
|---:|---|
| `+0x008 / +0x030` | Enmity / Stamina curve |
| `+0x128` | 暴击伤害上限倍率 |
| `+0x170 / +0x174 / +0x178 / +0x17C` | 普通 / 能力 / 奥义 / Chain Burst 基础上限 |
| `+0x180` | 克属追加倍率 |
| `+0x184 / +0x188` | 玩家 ATK 状态上限 / DEF 状态下限 |
| `+0x194` | `addDamageLimitBonusStatusRate` |
| `+0x1B0 / +0x1D8` | Super Enmity / Super Stamina curve |

官方 2.0.2 更新说明明确改变了 ATK↑ / DEF↓ 状态的伤害计算：额外伤害不再受原伤害上限影响；同类可叠加状态改为单一加算效果；同时提高了 HP、昏厥、伤害等显示/内部上限，并让伤害上限特性影响 PWR。因此旧版计算器的“全部先乘、最后只截一次上限”不能直接当作 2.0.2 事实。

在命中新的伤害 producer 前，安全的数据流边界是：

```text
pre-cap raw damage
  -> base hit limited by the action cap
  -> 2.0.2 ATK↑ / DEF↓ extra-damage path (exact order still unproven)
  -> supplemental / echo paths (exact order still unproven)
```

参考：

- [2.0.2 官方更新说明（简中）](https://relink-ragnarok.granbluefantasy.com/chs/updates/381/)
- [2.0.2 官方更新说明（英文）](https://relink-ragnarok.granbluefantasy.com/en/updates/381/)
- [当前召唤石加护与副参数目录](https://nenkai.github.io/relink-modding/resources/summon_trait_chances/)
- [Hercules 当前武器基础值与超越阶段](https://game8.jp/gbf-relink/797383)

## 回归命令

```powershell
$env:GOCACHE='D:\gbf\.tmp\gocache'
go test -mod=mod -count=1 -timeout=10m ./...

cd frontend
node --test src/*.test.js
npm run build
```

默认的存档写入测试会先复制 `SaveData2.dat` 到临时目录，再执行写入与回读验证。只有设置 `GBFR_REAL_SAVE_WRITE_QA=1` 的显式 opt-in 测试才会将已有配装原样写回真实测试存档；该路径会先生成整组可恢复快照，再修复校验并做磁盘回读。
