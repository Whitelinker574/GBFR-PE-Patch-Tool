<p align="center">
  <img src="build/appicon.png" width="128" alt="GBFR PE Patch Tool" />
</p>

# GBFR PE Patch Tool · DLC 2.0.2

[![Release](https://img.shields.io/github/v/release/Whitelinker574/GBFR-PE-Patch-Tool)](https://github.com/Whitelinker574/GBFR-PE-Patch-Tool/releases/latest)
[![CI](https://github.com/Whitelinker574/GBFR-PE-Patch-Tool/actions/workflows/ci.yml/badge.svg)](https://github.com/Whitelinker574/GBFR-PE-Patch-Tool/actions/workflows/ci.yml)

[下载最新版](https://github.com/Whitelinker574/GBFR-PE-Patch-Tool/releases/latest) · [English](README_EN.md) · [文档索引](docs/README.md)

面向《碧蓝幻想：Relink》DLC 2.0.2 的 Windows 本地存档编辑、受控运行时工具与只读公式校准项目。它不是官方工具；请只对自己的离线存档和本地单人环境使用，并始终保留可恢复备份。

当前稳定版为 **v1.10.0**。Release 提供 Windows amd64 压缩包和 SHA-256 校验文件；程序内更新检查指向本仓库的正式 Release。

## 当前能力

| 范围 | 功能 |
| --- | --- |
| 离线存档 | 因子、祝福、召唤石、配装预设、武器技能、角色成长、任务次数与称号记录。写入统一经过备份、校验和修复、原子替换和回读。 |
| 配装工作台 | 角色/武器/12 因子/专精/召唤石编辑，草稿与当前游戏最终 HP、攻击、暴击率、昏厥值对照；未验证公式继续标为估算。 |
| 运行时编辑 | 选中的因子、祝福石和召唤石记录，以及已验证的角色/任务功能。每次写入绑定进程、所有权令牌、选中目标和写后回读。 |
| 只读校准 | 角色最终面板回读、稳定性检查、严格 A/B/A/B 实验和脱敏证据包导出。该入口不注入、不写内存、不改存档。 |
| EXE/兼容功能 | 已审核的本地补丁、备份/恢复和版本兼容诊断；实验项目会在界面中明确标识。 |

## 目录与写入策略

因子、祝福和召唤石目录来自当前 2.0.2 表并在离线、内存和配装入口共享。天然词池、组合和观察到的等级仅用于默认值与提示：只要选择可编码，工具默认按选择写入，不需要额外“强制模式”。

这不代表跳过安全检查。目标所有权、过期快照、容器容量、整数编码、校验和、事务回滚和逐字段回读仍是硬性要求。未开放 DLC 的存档可以写已有预分配召唤石记录，但这不会替游戏解锁系统，也不保证游戏会消费该记录。

## 使用前

1. 复制一份存档；原地写入前再确认工具显示的备份路径。
2. 修改 EXE 前先创建 `.bak`，或用 Steam 验证文件恢复。
3. 运行时写入前确认当前游戏角色、背包条目或召唤石正是目标；切角色、重载存档或切页后重新读取。
4. 不要把运行时修改带入联机房间或影响其他玩家。

默认存档位置：

```text
C:\Users\<用户名>\AppData\Local\GBFR\Saved\SaveGames\
```

## 构建与验证

需要 Windows amd64、Go 1.25+、Node.js/npm、Wails CLI v2.13 和 WebView2 Runtime。仅在重建 `src_dll/patch_core` 时需要 Visual Studio/MSBuild。

```powershell
cd frontend
npm ci
npm run build
cd ..

go test ./...
node --test frontend/src/*.test.js
wails build -platform windows/amd64 -clean
```

构建结果：`build\bin\GBFR PE Patch Tool.exe`。

## 仓库结构

```text
app.go / *_store.go / *_gen.go     存档、运行时和事务逻辑
data/                              嵌入的目录与表数据
frontend/                          Vue/Wails 界面与前端测试
resources/                         发布时嵌入的资源
src_dll/                           可选 patch_core 原生组件
tools/                             可复现的数据审计、图标同步和 QA 脚本
docs/                              当前维护文档与机器可读证据
```

## 证据与边界

- [公式与证据等级](docs/FORMULAS_2.0.2.md)
- [运行时公式采样说明](docs/角色公式采样操作说明.md)
- [存档/内存目录一致性](docs/evidence/save-memory-table-parity.md)

已验证的运行时读回不会被当作全部公式的证明。条件 Buff、受击减伤、伤害上限和其他战斗结算仍需相应的训练场或木桩样本；界面会保留“候选/估算/未闭环”而非伪造精确结论。

## 致谢与免责声明

存档解析参考 [GBFRDataTools.SaveFile](https://github.com/Nenkai/GBFRDataTools/tree/master/GBFRDataTools.SaveFile)，因子和祝福相关历史研究参考 GBFR-Sigil-Generator 与 GBFR-Wrightstone-Generator。原项目逻辑由 BitterG 社区维护。

本项目仅供学习和个人本地使用。使用者自行承担修改存档、游戏文件或运行时内存的风险。
