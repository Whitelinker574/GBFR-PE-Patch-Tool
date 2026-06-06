<p align="center">
  <img src="build/appicon.png" width="128" />
</p>

# GBFR 存档修改工具

Granblue Fantasy: Relink (碧蓝幻想：Relink) 存档修改工具，包含 PE 补丁、角色使用次数、因子生成、祝福生成、副本次数查看五部分功能。

## 功能

### PE 补丁（关闭游戏后使用）

- **挑战次数** — 修改任务挑战次数上限（不影响存档）
- **点赞数值** — 修改被点赞时获得的数值（被点赞后生效，影响存档）
- **自动识别** — 启动时从 Steam 注册表和库目录自动定位游戏 exe
- **备份/恢复** — 补丁前创建 `.bak` 备份（仅 exe），一键恢复

### 角色使用次数（游戏运行中使用）

- **查看次数** — 连接游戏进程，读取所有角色的使用次数
- **排序查看** — 支持按次数倒序排列
- 修改后需对应角色结算一局生效

### 祝福生成（离线存档修改）

- **生成祝福** — 搜索选择祝福（Wrightstone），配置三个特性和等级，写入输出存档
- **队列批量** — 支持同时添加多个不同配置的祝福，按数量批量生成
- **查看已有祝福** — 加载存档后展示所有已有祝福，显示 ID、特性信息
- **CLI 模式** — 支持命令行模式，无需 GUI 即可批量生成祝福
- **中文翻译** — 祝福名、特性名均已本地化为简体中文

### 因子生成（离线存档修改）

- **生成因子** — 搜索选择因子，配置等级、主特性、副特性，写入输出存档
- **队列批量** — 支持同时添加多个不同配置的因子，按数量批量生成
- **查看已有因子** — 加载存档后展示所有已有因子，显示中文名、等级、特性
- **批量删除** — 勾选已有因子批量删除，也可一键清除全部因子
- **中文翻译** — 因子名、特性名均已本地化为简体中文

### 副本次数（离线存档查看）

- **存档扫描** — 自动扫描存档目录，列出所有存档槽位
- **任务统计** — 查看每个存档中所有任务/副本的通关次数
- **排序查看** — 支持按通关次数正序/倒序排列
- **存档概览** — 显示卢比、MSP、赞数、道具/武器/因子数量等存档摘要

## 使用说明

### 祝福生成

1. 切换到「祝福生成」标签页
2. **加载存档** — 点击「选择存档」打开存档文件（`.dat`）
   > 默认存档路径：`C:\Users\<用户名>\AppData\Local\GBFR\Saved\SaveGames\` 下的 `.dat` 文件
3. **查看已有祝福** — 加载后自动显示，可查看当前存档中的祝福
4. **配置祝福** — 在搜索框输入关键词过滤，从列表选中祝福
5. 依次选择三个特性及对应等级（每个特性等级选项由数据定义）
6. 设置数量，点击「添加到队列」，可重复添加多个不同配置
7. 在「输出」栏填写输出路径（默认自动生成 `_modified.dat`）
8. 点击「应用写入」，祝福将写入输出存档（原存档不被修改）

### 因子生成

1. 切换到「因子生成」标签页
2. **加载存档** — 输入存档文件路径（`.dat`），点击加载
   > 默认存档路径：`C:\Users\<用户名>\AppData\Local\GBFR\Saved\SaveGames\` 下的 `.dat` 文件
3. **查看已有因子** — 加载后自动显示，可勾选删除
4. **配置因子** — 在搜索框输入关键词过滤，从列表选中因子
5. 自动显示因子等级、主特性名称和等级（均为只读）
6. 如有副特性，从下拉框选择；数量默认为 1
7. 点击「添加到队列」，可重复添加多个不同配置
8. 在「输出」栏填写输出路径（默认自动生成 `_modified.dat`）
9. 点击「应用写入」，因子将写入输出存档（原存档不被修改）

### PE 补丁

1. **关闭游戏**
2. 切换到「补丁修改」标签页
3. 程序会自动扫描 Steam 安装路径定位游戏 exe
4. 如未自动识别，手动粘贴 `granblue_fantasy_relink.exe` 的完整路径
5. 建议先点击「备份」创建 `.bak` 文件
6. 输入数值，点击「应用」写入补丁
7. 启动游戏验证效果

### 角色使用次数

1. **启动游戏并进入存档**
2. 切换到「角色次数统计」标签页
3. 点击「连接游戏进程」，确认显示 PID
4. 查看各角色当前使用次数
5. 支持按次数排序查看
6. 可直接修改单个角色次数，或批量设置全部角色
7. 修改后需用对应角色结算一局生效

### 副本次数

1. 切换到「副本次数」标签页
2. 自动扫描存档目录，点击存档槽位加载
3. 查看各任务/副本通关次数
4. 支持按次数正序/倒序排列
5. 高亮显示高通关次数（>100 次）的任务

## 补丁原理

两个 PE 补丁点均为直接替换 `mov eax, imm32` 指令的立即数，并将后续的条件传送指令（`cmovb`）替换为等长 NOP：

| 补丁 | RVA | 原始指令 | 补丁后 |
|------|-----|---------|--------|
| 挑战次数 | `0x3583FF` | `mov eax, 999999; cmovb eax, r8d` (9B) | `mov eax, <value>; nop4` |
| 点赞数值 | `0xA919CF` | `mov eax, 999999; cmovb eax, esi` (8B) | `mov eax, <value>; nop3` |

角色使用次数通过运行时读写游戏进程内存实现，基于 `sys::data::CharaList` 数据结构（IDA 逆向分析）。

## 恢复原始文件

任选其一：

- 工具内点击「恢复」从 `.bak` 还原
- Steam → 游戏属性 → 本地文件 → 验证游戏文件完整性

## 环境要求

- Go 1.23+（**必须 amd64 版本**，游戏为 64 位进程）
- Node.js 16+
- [Wails CLI v2](https://wails.io/docs/gettingstarted/installation)

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

## 编译打包

```bash
# 安装前端依赖
cd frontend && npm install && cd ..

# 开发模式（热重载）
wails dev

# 完整构建（前端 + Go）
wails build

# 仅构建 Go（跳过前端编译，用于前端已预先 build 的情况）
wails build -s
```

> 如遇 `go: no such tool "compile"` 错误，需要指定正确的 GOROOT：
> ```powershell
> $env:GOROOT="D:\GO1.26.1"; wails build -s
> ```

构建产物在 `build/bin/GBFR PE Patch Tool.exe`。

## 数据说明

因子和祝福数据文件位于 `data/` 目录（嵌入二进制）：

| 文件 | 说明 |
|------|------|
| `sigils.json` | 因子定义（ID、哈希、名称、允许等级、主/副特性列表） |
| `traits.json` | 特性定义（ID、哈希、名称、最大等级） |
| `secondary-trait-rules.json` | 副特性兼容性规则 |
| `wrightstones.json` | 祝福定义（ID、哈希、名称、默认特性） |
| `wrightstone_traits.json` | 祝福特性定义（ID、哈希、名称、最大等级、允许等级） |
| `quest_names_i18n.csv` | 任务 ID 到中文名的翻译表 |

因子/特性中文翻译位于 `sigil_locale.go`（`sigilCN` 和 `traitCN` 两个 map），可直接编辑英文名→中文名的映射。
祝福/特性中文翻译位于 `wrightstone_locale.go`（`wscn` 和 `wstCN` 两个 map）。

目前部分因子缺失，常用因子基本都有。

## 项目结构

```
├── main.go              # Wails 入口，窗口配置、祝福 CLI 模式
├── app.go               # PE 补丁、进程内存读写、Steam 路径扫描、角色次数管理
├── save_app.go          # 存档文件扫描、任务次数读取、中文任务名映射
├── save_parse.go        # 存档文件 FlatBuffer 解析、存档摘要提取
├── sigil_data.go        # 因子/特性数据模型，嵌入式 JSON 加载与解析
├── sigil_ctdata.go      # 因子合成表（Factor Synthesis）数据
├── sigil_store.go       # 存档文件因子槽位读写、XXHash64 校验和
├── sigil_gen.go         # 因子生成业务逻辑（Wails 前端绑定）
├── sigil_locale.go      # 因子/特性中文翻译映射表
├── wrightstone_data.go  # 祝福/祝福特性数据模型，嵌入式 JSON 加载与解析
├── wrightstone_store.go # 存档文件祝福槽位读写
├── wrightstone_gen.go   # 祝福生成业务逻辑（Wails 前端绑定 + CLI 模式）
├── wrightstone_locale.go # 祝福/特性中文翻译映射表
├── data/                # 嵌入式 JSON/CSV 数据文件
│   ├── sigils.json
│   ├── traits.json
│   ├── secondary-trait-rules.json
│   ├── wrightstones.json
│   ├── wrightstone_traits.json
│   └── quest_names_i18n.csv
├── wails.json           # Wails 项目配置
├── go.mod               # Go 模块依赖
├── build/               # 构建资源（图标、manifest）
└── frontend/
    ├── index.html
    ├── package.json
    ├── vite.config.js
    └── src/
        ├── main.js
        ├── App.vue                     # 根组件
        ├── style.css                   # 全局样式（深色主题）
        └── components/
            ├── PatchTool.vue           # 主窗口：补丁修改 + 标签导航
            ├── SigilGenerator.vue      # 因子生成器界面
            ├── WrightstoneGenerator.vue # 祝福生成器界面
            ├── CharaStats.vue          # 角色次数统计界面
            └── SaveEditor.vue          # 副本次数查看界面
```

## 免责声明

本工具仅供学习研究使用。使用本工具修改游戏文件所产生的一切后果由使用者自行承担。

存档因子相关部分解析方法来自 [GBFR-Sigil-Generator](https://github.com/Xzire91x/GBFR-Sigil-Generator)

祝福添加相关部分解析方法来自 [GBFR-Wrightstone-Generator](https://github.com/Xzire91x/GBFR-Wrightstone-Generator)

存档解析基于 [GBFRDataTools.SaveFile](https://github.com/Nenkai/GBFRDataTools/tree/master/GBFRDataTools.SaveFile)
