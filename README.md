<p align="center">
  <img src="build/appicon.png" width="128" />
</p>

# GBFR 存档修改工具

[English README](README_EN.md)

Granblue Fantasy: Relink (碧蓝幻想：Relink) 存档与运行时修改工具，包含 PE 补丁、因子/祝福生成、副本次数查看、角色使用次数统计、杂项内存修改和怪物增强等功能。
<img src="https://img.shields.io/github/downloads/BitterG/GBFR-PE-Patch-Tool/total"/>


## 功能

当前数据、运行时签名和配装计算均以游戏 **2.0.2** 为目标版本；离线生成与游戏内选中即时编辑是两套独立入口。

### 存档相关

- **因子生成** — 搜索选择因子，配置等级、主/副特性，写入输出存档
- **祝福生成** — 搜索选择祝福，配置三个特性和等级，支持队列批量生成
- **配装预设** — 读取并写入单个角色的单套配装，完整配置武器、12 个因子、4 个角色技能、三方向三阶段专精和召唤石；支持单套 JSON 导入/导出
- **配装结算** — 合并同类因子、武器技能、专精、角色上限突破与召唤石加成，展示 HP、攻击力、暴击率、昏厥值和伤害上限；其中 HP/攻击百分比乘区与量化顺序尚未完全闭环，界面会明确标记为估算
- **副本次数查看** — 自动扫描存档槽位，查看任务/副本通关次数与存档摘要
- **原地修改** — 因子/祝福生成可选直接覆盖输入存档，建议先备份
- **中文翻译** — 因子、祝福、特性、任务名均有简体中文显示

### PE 补丁

- **挑战次数** — 修改任务挑战次数上限，不影响存档
- **点赞数值** — 修改被点赞时获得的数值，被点赞后生效，影响存档
- **自动识别** — 从 Steam 注册表和库目录自动定位游戏 exe
- **备份/恢复** — 补丁前创建 `.bak` 备份，仅 exe，一键恢复

### 运行时功能

- **因子即时编辑** — 在游戏背包中选中因子后读取整条记录，修改并保存；每次写入绑定捕获槽位并由后端重新校验
- **祝福石即时编辑** — 覆盖上游 v1.8.3/v1.8.4 的“祝福生成-新”流程，支持选中读取、三个词条编辑、整条写入回读、寄存器保护与恢复；写入成功后自动停止 Hook
- **因子配装录制/复刻** — 从游戏背包逐项记录 12 个因子并导出，也可将单套记录复刻到备用因子
- **召唤石修改** — 读取召唤石背包，修改已验证的主因子、副参数和等级，并提供明确的连接/断开边界
- **角色使用次数** — 连接游戏进程，查看和修改角色使用次数
- **怪物增强** — 怪物多倍血、怪物伤害、昏厥条、Overdrive 状态、奥义接续计时、Link Time、蓝条/紫条控制等
- **巴武掉落** — 开启后，每一把至少会从黄金宝箱中获得一把未拥有巴武
- **上限突破** — 先扫描再打开游戏内突破界面后点刷新，切换角色后需刷新，修改后要点保存
- **运行时生命周期保护** — 钩子按游戏进程实例归属，页面停止、断开和应用退出时恢复；不确定的远程保存结果会锁定该进程实例，避免继续写入
- **检查更新** — 从 GitHub Releases 获取最新版本并打开发布页

### 兼容实验室（尚未按 2.0.2 生产验收）

以下旧版功能保留了资料、界面或实验实现，但当前版本会标成“等待适配/实验”，不能视为已经在游戏 2.0.2 上完成：任务结算倒计时与零帧开箱、无限挑战、脸部/其他皮肤紫色符文、全称号解锁、全队伤害统计，以及任务得分倍率、强制支线目标奖励和任务内倍率。

## 使用说明

### 存档类功能

1. 切换到「因子生成」「祝福生成」或「副本次数」标签页
2. 点击「浏览」选择存档文件，或使用自动扫描的存档槽位
3. 配置需要生成或查看的内容
4. 写入前建议先备份存档

默认存档路径：

```text
C:\Users\<用户名>\AppData\Local\GBFR\Saved\SaveGames\
```

### PE 补丁

1. 关闭游戏
2. 切换到「补丁修改」标签页
3. 自动识别或手动选择 `granblue_fantasy_relink.exe`
4. 点击「备份」创建 `.bak`
5. 输入数值并点击「应用」
6. 启动游戏验证效果

### 运行时功能

1. 启动游戏并进入存档
2. 切换到「角色次数统计」「杂项」或「怪物增强」标签页
3. 连接或刷新游戏进程状态
4. 开启、应用或恢复需要的功能
5. 重启游戏后需要重新连接并重新设置

### 怪物增强说明

- 「怪物多倍血」和「鳄鱼多倍血」输入 `10` 表示等效 `10 倍血`
- 「怪物 Overdrive 状态」支持 `1 满红条`、`4 满黄条` 和「自动OD」
- 「锁定」会持续写入状态，「应用」只写入一次后恢复原始指令
- 「自动OD」会在非红条时写入一次满黄条，红条中不重复触发
- 「奥义接续计时」默认 `3 秒`，可输入自定义秒数并恢复默认
- 部分怪物增强功能依赖内置 `patch_core.dll`

## 实现简述

- PE 补丁直接修改 exe 中指定指令的立即数，并保留备份用于恢复
- 存档功能基于 FlatBuffer 解析与 XXHash64 校验写回
- 配装人物属性、武器四段成长与 2.0.2 伤害公式的证据等级见 [`docs/FORMULAS_2.0.2.md`](docs/FORMULAS_2.0.2.md)
- 运行时功能通过打开游戏进程并读写内存实现；实时因子/祝福、召唤石和上限突破均在写入前后校验目标进程与记录
- 怪物增强中简单功能由 Go 直接写内存，复杂功能通过 `patch_core.dll` 注入并写入跳板或补丁
- `patch_core.dll` 仅输出调试信息，不弹出对话框

## 恢复方法

任选其一：

- 工具内点击「恢复」从 `.bak` 还原
- Steam → 游戏属性 → 本地文件 → 验证游戏文件完整性

## 环境要求

- Go 1.23+（必须 amd64 版本，游戏为 64 位进程）
- Node.js 16+
- [Wails CLI v2](https://wails.io/docs/gettingstarted/installation)
- Visual Studio / MSBuild（修改 `src_dll/patch_core` 后需要编译 DLL）

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

## 编译打包

```bash
# 安装前端依赖
cd frontend && npm install && cd ..

# 开发模式
wails dev

# 完整构建
wails build

# 仅构建 Go，跳过前端编译
wails build -s
```

如修改 `src_dll/patch_core`，先用 Visual Studio 构建 `Release x64`，确认输出覆盖：

```text
build/bin/patch_core.dll
```

### Windows 一键编译

项目根目录提供 `build-windows.bat`：

```powershell
.\build-windows.bat
```

如遇 `go: no such tool "compile"`，指定正确的 GOROOT：

```powershell
$env:GOROOT="D:\GO1.26.1"; wails build -s
```

构建产物：

```text
build/bin/GBFR PE Patch Tool.exe
```

## 数据说明

因子、祝福和任务翻译数据位于 `data/` 目录，并嵌入最终二进制：

| 文件 | 说明 |
|------|------|
| `sigils.json` | 因子定义 |
| `traits.json` | 因子特性定义 |
| `secondary-trait-rules.json` | 副特性兼容性规则 |
| `wrightstones.json` | 祝福定义 |
| `wrightstone_traits.json` | 祝福特性定义 |
| `quest_names_i18n.csv` | 任务 ID 到中文名映射 |

中文翻译主要位于：

- `sigil_locale.go`
- `wrightstone_locale.go`

## 项目结构

```text
.
├── app.go                         # PE 补丁、运行时内存修改、Steam 路径扫描、更新检测
├── main.go                        # Wails 入口、窗口配置、祝福 CLI 模式
├── save_app.go                    # 存档扫描、任务次数读取、中文任务名映射
├── save_parse.go                  # 存档 FlatBuffer 解析、摘要提取
├── sigil_data.go                  # 因子/特性数据加载
├── sigil_ctdata.go                # 因子合成表数据
├── sigil_gen.go                   # 因子生成业务逻辑
├── sigil_locale.go                # 因子/特性中文翻译
├── sigil_store.go                 # 因子槽位读写与校验
├── wrightstone_data.go            # 祝福/祝福特性数据加载
├── wrightstone_gen.go             # 祝福生成业务逻辑与 CLI 模式
├── wrightstone_locale.go          # 祝福/特性中文翻译
├── wrightstone_store.go           # 祝福槽位读写
├── damage_overlay_windows.go      # 伤害统计悬浮窗
├── data/                          # 嵌入式 JSON/CSV 数据
├── build/                         # 图标、manifest、内置 DLL 与构建产物
│   └── bin/
│       └── patch_core.dll         # 怪物增强注入 DLL
├── src_dll/
│   ├── patch_core.slnx            # patch_core Visual Studio 解决方案
│   ├── patch_core/                # patch_core DLL 源码
│   └── thirdparty/libmem/         # DLL 使用的 libmem 依赖
├── frontend/
│   ├── package.json
│   ├── vite.config.js
│   ├── wailsjs/                   # Wails 生成的前端绑定
│   └── src/
│       ├── main.js
│       ├── App.vue
│       ├── style.css
│       └── components/
│           ├── PatchTool.vue           # 主窗口与标签导航
│           ├── SigilGenerator.vue      # 离线因子生成
│           ├── SigilMemoryGenerator.vue # 游戏内选中因子即时编辑
│           ├── SigilLoadoutRestore.vue # 游戏内因子配装录制/复刻
│           ├── WrightstoneGenerator.vue # 离线祝福生成
│           ├── WrightstoneMemoryGenerator.vue # 游戏内选中祝福即时编辑
│           ├── LoadoutViewer.vue       # 配装预设读取、导入导出与写入
│           ├── LoadoutEditor.vue       # 全屏配装编辑、专精、召唤石与结算
│           ├── SummonEditor.vue        # 游戏内召唤石编辑
│           ├── OverLimit.vue           # 游戏内角色上限突破
│           ├── SaveEditor.vue          # 副本次数
│           ├── CharaStats.vue          # 角色使用次数
│           ├── MiscTools.vue           # 杂项运行时修改
│           └── MonsterEnhance.vue      # 怪物增强
├── build-windows.bat              # Windows 构建脚本
├── wails.json                     # Wails 配置
├── go.mod
└── README.md
```

## 支持
如果本项目帮你省了时间或带来更多乐趣，欢迎请我喝杯咖啡。完全自愿，不是契约——捐赠不会影响功能优先级，也不会改变问题处理的顺序。
— 微信支付（扫码）
<p align="center">
  <img src="./QRcode.png" width="256" />
</p>

## 免责声明

本工具仅供学习研究使用。使用本工具修改游戏文件、存档或运行时内存所产生的一切后果由使用者自行承担。

存档因子相关部分解析方法来自 [GBFR-Sigil-Generator](https://github.com/Xzire91x/GBFR-Sigil-Generator)。

祝福添加相关部分解析方法来自 [GBFR-Wrightstone-Generator](https://github.com/Xzire91x/GBFR-Wrightstone-Generator)。

存档解析基于 [GBFRDataTools.SaveFile](https://github.com/Nenkai/GBFRDataTools/tree/master/GBFRDataTools.SaveFile)。
