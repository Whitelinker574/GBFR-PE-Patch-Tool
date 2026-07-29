# 分享站部署

现有 R2 绑定可以直接部署短码和公开预览。社区点赞、留言使用可选的 D1 绑定：

1. 创建 D1 数据库，例如 `gbfr-loadout-community`。
2. 执行 `npx wrangler d1 migrations apply gbfr-loadout-community --remote`。
3. 在 `wrangler.jsonc` 的 `d1_databases` 中加入绑定名 `COMMUNITY_DB`、数据库名和真实 `database_id`。
4. 为 Worker 设置机密 `CATALOG_ADMIN_TOKEN`，用于一次性目录回填；不要把值写入配置文件。
5. 再部署 Worker。未配置 D1 时，短码、目录、详情和下载仍可用，互动区域会显示未启用。
6. 部署后用管理令牌分批调用 `POST /api/internal/catalog/backfill?limit=100`。响应中的 `cursor` 非空时，把它作为下一次请求的 `cursor`，直到 `truncated=false`。

目录发布采用 R2 与 D1 双写：R2 保存原始配装帧和完整脱敏预览，D1 只保存公开卡片、检索字段和互动计数。D1 查询或迁移临时失败时，公开目录会自动回退到 R2 扫描；详情和短码下载始终以 R2 为准。

不要把真实账号令牌、数据库 ID 或管理凭据提交到仓库。国内访问取决于当前域名和网络线路；先用现有自定义域名做多地实测，再决定是否增加国内只读镜像。
