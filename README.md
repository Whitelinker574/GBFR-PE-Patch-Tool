<p align="center">
  <img src="build/appicon.png" width="128" />
</p>

# GBFR PE Patch Tool

Granblue Fantasy: Relink (碧蓝幻想：Relink) 静态 PE 补丁工具，通过修改游戏可执行文件中的指令立即数来实现数值修改。

不修改游戏存档，Steam 验证文件完整性即可恢复原始状态。

## 功能

- **挑战次数** — 修改任务挑战次数上限
- **点赞数值** — 修改被点赞时获得的数值（被点赞后生效）
- **自动识别** — 启动时从 Steam 注册表和库目录自动定位游戏 exe
- **备份/恢复** — 补丁前创建 `.bak` 备份，一键恢复
- 每名角色使用次数可能后续再更新

## 补丁原理

两个补丁点均为直接替换 `mov eax, imm32` 指令的立即数，并将后续的条件传送指令（`cmovb`）替换为等长 NOP：

| 补丁 | RVA | 原始指令 | 补丁后 |
|------|-----|---------|--------|
| 挑战次数 | `0x3583FF` | `mov eax, 999999; cmovb eax, r8d` (9B) | `mov eax, <value>; nop4` |
| 点赞数值 | `0xA919CF` | `mov eax, 999999; cmovb eax, esi` (8B) | `mov eax, <value>; nop3` |

## 环境要求

- Go 1.23+
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

1. **关闭游戏**
2. 运行补丁工具，程序会自动扫描 Steam 安装路径定位游戏 exe
3. 如未自动识别，手动粘贴 `granblue_fantasy_relink.exe` 的完整路径
4. 建议先点击「备份」创建 `.bak` 文件
5. 输入数值，点击「应用」写入补丁
6. 启动游戏验证效果

## 恢复原始文件

任选其一：

- 工具内点击「恢复」从 `.bak` 还原
- Steam → 游戏属性 → 本地文件 → 验证游戏文件完整性

## 项目结构

```
├── main.go          # Wails 入口，窗口配置
├── app.go           # Go 后端：PE 解析、补丁逻辑、Steam 路径扫描
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
