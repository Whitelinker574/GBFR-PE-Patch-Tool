<p align="center">
  <img src="build/appicon.png" width="128" />
</p>

# GBFR 存档修改工具

Granblue Fantasy: Relink (碧蓝幻想：Relink) 存档修改工具，包含 PE 补丁、角色使用次数、因子生成三部分功能。

## 功能

### PE 补丁（关闭游戏后使用）

- **挑战次数** — 修改任务挑战次数上限（不影响存档）
- **点赞数值** — 修改被点赞时获得的数值（被点赞后生效，影响存档）
- **自动识别** — 启动时从 Steam 注册表和库目录自动定位游戏 exe
- **备份/恢复** — 补丁前创建 `.bak` 备份（仅 exe），一键恢复

### 角色使用次数（游戏运行中使用）

- **查看次数** — 连接游戏进程，读取所有角色的使用次数
- **排序查看** — 支持按次数倒序排列
- **修改次数** — 设置单个或批量修改所有角色次数（需自行编译，影响存档）
- 修改后需对应角色结算一局生效

### 因子生成（离线存档修改）

- **生成因子** — 选择因子、等级、主特性、副特性，写入存档
- **队列批量** — 支持同时添加多个不同配置的因子
- **清除因子** — 一键清除输出存档中的所有因子

## 补丁原理

两个 PE 补丁点均为直接替换 `mov eax, imm32` 指令的立即数，并将后续的条件传送指令（`cmovb`）替换为等长 NOP：

| 补丁 | RVA | 原始指令 | 补丁后 |
|------|-----|---------|--------|
| 挑战次数 | `0x3583FF` | `mov eax, 999999; cmovb eax, r8d` (9B) | `mov eax, <value>; nop4` |
| 点赞数值 | `0xA919CF` | `mov eax, 999999; cmovb eax, esi` (8B) | `mov eax, <value>; nop3` |

角色使用次数通过运行时读写游戏进程内存实现，基于 `sys::data::CharaList` 数据结构（IDA 逆向分析）。

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

# 生产构建
wails build
```

构建产物在 `build/bin/GBFR PE Patch Tool.exe`。

## 使用说明

### PE 补丁

1. **关闭游戏**
2. 运行补丁工具，程序会自动扫描 Steam 安装路径定位游戏 exe
3. 如未自动识别，手动粘贴 `granblue_fantasy_relink.exe` 的完整路径
4. 建议先点击「备份」创建 `.bak` 文件
5. 输入数值，点击「应用」写入补丁
6. 启动游戏验证效果

### 角色使用次数

1. **启动游戏并进入存档**
2. 点击「连接游戏进程」，确认显示 PID
3. 查看各角色当前使用次数
4. 如需修改（需自行编译开启），输入数值后点击设置
5. 修改后需用对应角色结算一局生效

## 恢复原始文件

任选其一：

- 工具内点击「恢复」从 `.bak` 还原
- Steam → 游戏属性 → 本地文件 → 验证游戏文件完整性

## 项目结构

```
├── main.go          # Wails 入口，窗口配置
├── app.go           # Go 后端：PE 补丁、进程内存读写、Steam 路径扫描
├── wails.json       # Wails 项目配置
├── go.mod           # Go 模块依赖
├── build/           # 构建资源（图标、manifest）
└── frontend/
    ├── index.html
    ├── package.json
    ├── vite.config.js
    └── src/
        ├── main.js
        ├── App.vue
        ├── style.css
        └── components/
            └── PatchTool.vue   # 主界面组件
```

## 免责声明

本工具仅供学习研究使用。使用本工具修改游戏文件所产生的一切后果由使用者自行承担。
