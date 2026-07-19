# v1.8.4 新祝福与实时编辑安全升级实施计划

> 在 `feature/loadout-fullscreen` 的既有 worktree 中执行。该分支包含尚未提交的关联功能，所有改动必须避开和保留无关用户变更。

## 任务 1：锁定现有因子钩子崩溃回归

**文件：** `sigil_memory.go`、`sigil_memory_test.go`

1. 添加失败测试，断言新代码洞字节序列先 `push r10`、保存选中指针、再 `pop r10`，并断言原指令偏移正确。
2. 添加旧代码洞识别测试，要求只支持恢复原字节，不把旧洞当作可继续写入的安全洞。
3. 运行定向测试确认红灯。
4. 调整代码洞布局、恢复解析和常量；增加 `SigilMemoryDisable`。
5. 再跑定向测试确认绿灯。

## 任务 2：实现祝福整条记录事务层

**文件：** 新增 `wrightstone_memory_safety.go`、`wrightstone_memory_safety_test.go`

1. 先写记录编码与校验测试：第一槽必填、空槽等级为 0、槽位上限、未知 hash 拒绝。
2. 写事务测试：部分写入、第一次回读、保存调用、最终回读失败均恢复旧记录。
3. 实现纯函数编码、验证和 `writeWrightstoneMemoryRecordAtomic`。
4. 运行定向测试与 race 测试。

## 任务 3：实现祝福内存捕获后端

**文件：** 新增 `wrightstone_memory.go`、`wrightstone_memory_test.go`，修改 `app.go`

1. 先写代码洞布局、拥有权恢复、状态转换和捕获指针清空测试。
2. 实现选项、扫描、状态、启用、写入、关闭六个接口。
3. 写入复用任务 2 的事务层和现有实时存档快照。
4. 在 Detach/Shutdown 路径恢复祝福钩子并清空状态。
5. 将相同 PID 的重复连接改为幂等；给部分读写检测添加可测试的长度校验。

## 任务 4：补齐祝福与因子真值

**文件：** `data/wrightstone_traits.json`、`wrightstone_locale.go`、`wrightstone_data_test.go`、`data/sigils.json`、`data/traits.json`、`sigil_catalog_truth_test.go`、`sigil_memory_names.go`

1. 添加失败数据测试固定 18 个真实 `SKILL_*` ID/hash 与允许等级。
2. 添加失败测试固定 Firm Stance V 与 Ferry 两个词条映射。
3. 写入经 `ids.txt` 验证的数据、中文名和来源说明。
4. 修正实时因子别名；新增 DLC 因子必须通过固定主词条与等级测试，不确定组合保持不可写。
5. 运行目录、生成器和内存校验测试。

## 任务 5：新增祝福实时编辑页面

**文件：** 新增 `frontend/src/components/WrightstoneMemoryGenerator.vue`、`frontend/src/wrightstoneMemoryUi.test.js`，修改 `frontend/src/components/PatchTool.vue`

1. 先写前端契约测试，固定导航、连接条、三槽、显式停止、写后失效和响应式样式。
2. 实现三槽记录卡、折叠变更摘要、粘性操作栏与状态轮询。
3. 页面卸载时关闭钩子；所有对话使用共享 `ConfirmDialog`，不使用浏览器 alert/confirm。
4. 接入现有祝福立绘/贴纸与实时工具导航。
5. 运行全部 Node 测试与生产构建。

## 任务 6：生成绑定与端到端验证

**文件：** `frontend/wailsjs/go/main/App.js`、`App.d.ts`、`frontend/wailsjs/go/models.ts`（由 Wails 生成器决定）

1. 使用项目生成命令刷新 Wails 绑定，不手写生成文件。
2. 运行 `gofmt`、`go test ./...`、`go test -race ./...`（平台支持时）、`node --test src/*.test.js`、`npm run build`。
3. 构建 Windows 应用，并在 375/768/1024/1440 四个宽度截图审查。
4. 有游戏进程时用测试存档完成捕获、写入、回读、停止捕获和退出恢复；没有游戏进程时至少验证扫描失败关闭与离线数据。
5. 交给独立代理做代码、安全和 UI 审查，修复所有高优先级问题后重跑全套验证。
