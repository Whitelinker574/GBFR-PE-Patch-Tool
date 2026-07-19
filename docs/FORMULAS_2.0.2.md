# GBFR 2.0.2 配装数值证据笔记

本文记录配装编辑器当前采用的数值来源、已逆向的运行时代码路径、真实存档回归点，以及仍未证明的部分。目标不是把“页面能算出一个数”当成正确，而是让每一层公式都能追溯到 2.0.2 EXE、解包表或存档字段。

## 版本指纹与证据等级

- 游戏：Granblue Fantasy: Relink 2.0.2
- EXE：`granblue_fantasy_relink.exe`
- SHA-256：`63340832BCF731FBC97796F686B05C988418E83D451D4A49B2244A85D00E297F`
- 真实回归存档：`Saved/SaveGames/SaveData2.dat`（常规测试只操作临时副本；显式 opt-in 测试才会原样写回并创建整组备份）

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

当前存档同时把角色 HP / 攻击的整数基础值保存在 `1309 / 1310`，编辑器优先读取存档，不重复猜测升级状态。真实伊欧 Lv100 回归为 `3156 HP / 666 ATK / 8 Stun / 5% Crit`。

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

## 最终人物面板（C，部分已证实）

最终聚合器位于 RVA `0xA25F80–0xA2B43E`，输出结构 `+0x04` 为 HP，`+0x08` 为攻击。已经闭环的整数核心为：

```text
HP core = status[5A50] + status[5A28] + status[5A44]
        + weapon[78] + weapon[5C] + weapon[6C] + weapon[8C]
        + 其他固定/效果贡献

ATK core = status[5A54] + status[5A2C] + status[5A48]
         + weapon[7C] + weapon[60] + weapon[70] + weapon[90]
         + 其他固定/效果贡献
```

基础 HP 在 `0xA26713` 消费，初次转整数在 `0xA26981`；基础攻击在 `0xA2698F` 消费，初次转整数在 `0xA26BDC`。后续已识别的固定词条不是先塞回同一个“白值”再统一乘倍率，而是按下列顺序逐项加到已量化结果并再次向零截断：

| 面板项 | 指令地址 | 词条 hash / 名称 | 消费字段 |
|---|---:|---|---:|
| HP | `0xA26CA5` | `F372F096` / 体力 | effect `+0x08` |
| HP | `0xA26D35` | `D3B8C21F` / 终极钳蟹因子 | effect `+0x0C` |
| HP | `0xA26DC8` | `89C66ACB` / 相扑斗力 | effect `+0x18` |
| ATK | `0xA26E55` | `50079A1C` / 攻击力 | effect `+0x08` |
| ATK | `0xA26ED2` | `082033CB` / 钳蟹的共鸣 | effect `+0x0C` |
| ATK | `0xA26EE6` | `89C66ACB` / 相扑斗力 | effect `+0x14` |

`0xA26ED9` 在最后一项 ATK 前执行 `VROUNDSS imm8=0x0B`，同样明确指定 toward-zero；所有写入输出结构 `+0x04/+0x08` 的上述路径最终都经过 `VCVTTSS2SI`。因此“最终及这些已识别中间写入采用 float32 向零截断”已经闭环，未闭环的是上游百分比 producer，而不是整数转换方向。

聚合器调用 `0x298830–0x298A45` 取得效果贡献。该函数只筛选运行时 effect 记录的类别与索引，再用 `VADDSS` 累加记录中的标量返回；HP 使用索引 `0/1`，ATK 使用索引 `4/5`。这证明最终聚合器消费的是上游已经换算好的绝对浮点贡献，不能仅凭说明文本里的百分比反推出它们属于同一个加算或乘算组。

相关生产与聚合路径使用 scalar `float32` 指令；最终聚合段主要为 `VCVTSI2SS / VADDSS / VCVTTSS2SI`。编辑器因此也保留 binary32 精度；用 `float64` 会在整数边界产生与游戏不同的结果。

已经证实的输入层还包括召唤石/上限突破字段、因子与专精的表值。尚未闭环的是**最终 HP / 攻击百分比如何在上游变成绝对贡献，以及各组进入上述多次量化的顺序**；`status+0x5A44/+0x5A48` 很可能是额外成长项，但当前证据不足以给它们命名。

当前静态页面采用的可解释模型为：

```text
HP  ≈ trunc32((角色基础 + 武器 + 无条件固定 HP)
              × 各独立无条件 HP 倍率
              × (1 + 专精 HP 百分比合计))

ATK ≈ trunc32((角色基础 + 武器 + 无条件固定 ATK)
              × 各独立无条件 ATK 倍率
              × (1 + 专精/联动 ATK 百分比合计))
```

它用于实时比较草稿，不应被描述成“2.0.2 最终面板公式已完全验证”。条件依赖当前血量比例、蓄力、目标状态、战斗状态或队伍状态的效果默认不进入静态面板；能由当前配置唯一判定的 HP 阈值效果才会进入。

伤害上限分别保留普通、能力、奥义三项；顶部单值取三者共同增量的最小值，展开层显示三项真实分拆，不把不同上限混成一个虚假总数。

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
go test ./...

cd frontend
node --test src/*.test.js
npm run build
```

默认的存档写入测试会先复制 `SaveData2.dat` 到临时目录，再执行写入与回读验证。只有设置 `GBFR_REAL_SAVE_WRITE_QA=1` 的显式 opt-in 测试才会将已有配装原样写回真实测试存档；该路径会先生成整组可恢复快照，再修复校验并做磁盘回读。
