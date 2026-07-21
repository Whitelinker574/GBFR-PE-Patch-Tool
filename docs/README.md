# 项目文档

仓库只保留运行、维护和复现当前版本所需的说明。现场交接包、截图、构建产物、临时审计和历史工作流不进入 Git；需要共享时应放在已忽略的 `docs/local/`、`docs/handoffs/` 或独立交付目录。

| 文档 | 用途 |
| --- | --- |
| [`FORMULAS_2.0.2.md`](FORMULAS_2.0.2.md) | 配装数值来源、证据等级与已知边界 |
| [`角色公式采样操作说明.md`](角色公式采样操作说明.md) | 严格只读运行时公式采样操作 |
| [`evidence/save-memory-table-parity.md`](evidence/save-memory-table-parity.md) | 因子、祝福和召唤石在存档/内存/配装入口的目录一致性与写入策略 |
| [`evidence/sigil-table-audit-202.json`](evidence/sigil-table-audit-202.json) | 2.0.2 因子表的脱敏、机器可读审计摘要；由回归测试读取 |

所有数字、布局和运行时结论都应以当前代码、回归测试和实际游戏验证为准。未闭环的结论必须标注为候选或估算，不能作为确定游戏公式发布。
