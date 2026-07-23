# 项目文档

仓库只保留运行、维护和复现当前版本所需的说明。`docs/screenshots/` 中经过脱敏并由当前公开页面生成的 README 截图属于维护文档；临时记录、构建产物和本机专用材料不进入 Git，需要保留时应放在已忽略的 `docs/local/` 或仓库外目录。

| 文档 | 用途 |
| --- | --- |
| [`ARCHITECTURE.md`](ARCHITECTURE.md) | 运行结构、代码域、写入边界与测试布局 |
| [`IMPLEMENTATION_STATUS.md`](IMPLEMENTATION_STATUS.md) | 当前功能的已实现、现场验证和仍未闭环清单 |
| [`FORMULAS_2.0.2.md`](FORMULAS_2.0.2.md) | 配装数值来源、证据等级与已知边界 |
| [`community-update-v1.91.5-v1.91.12.md`](community-update-v1.91.5-v1.91.12.md) | 面向社区的 v1.91.5-v1.91.12 更新公告 |
| [`角色公式采样操作说明.md`](角色公式采样操作说明.md) | 严格只读运行时公式采样操作 |
| [`evidence/save-memory-table-parity.md`](evidence/save-memory-table-parity.md) | 因子、祝福和召唤石在存档/内存/配装入口的目录一致性与写入策略 |
| [`evidence/sigil-table-audit-202.json`](evidence/sigil-table-audit-202.json) | 2.0.2 因子表的脱敏、机器可读审计摘要；由回归测试读取 |
| [`screenshots/`](screenshots/) | README 使用的脱敏公开页面截图；只展示示例存档与示例目录数据 |

所有数字、布局和运行时结论都应以当前代码、回归测试和实际游戏验证为准。未闭环的结论必须标注为候选或估算，不能作为确定游戏公式发布。

## 维护者约定

- 文档只记录当前版本仍可复现的操作、证据和限制，不保留聊天记录、临时排错过程或机器专用路径。
- 表格数据必须注明游戏版本和校验身份；第三方公开资料只能作为交叉检查，不能替代本地游戏表、EXE 守卫和真实回读。
- 公开截图必须由当前应用页面生成，并使用示例数据；不得出现真实用户名、存档路径、PID、模块基址或绝对地址。
- 后端文件按功能前缀索引在 [`internal/backend/README.md`](../internal/backend/README.md)。维护脚本的输入、输出和运行时机统一记录在 [`tools/README.md`](../tools/README.md)。一次性脚本放在已忽略的本地目录，不进入发布分支。
