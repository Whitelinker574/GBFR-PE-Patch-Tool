# 分享站部署

现有 R2 绑定可以直接部署短码和公开预览。社区点赞、留言使用可选的 D1 绑定：

1. 创建 D1 数据库，例如 `gbfr-loadout-community`。
2. 执行 `npx wrangler d1 migrations apply gbfr-loadout-community --remote`。
3. 在 `wrangler.jsonc` 的 `d1_databases` 中加入绑定名 `COMMUNITY_DB`、数据库名和真实 `database_id`。
4. 再部署 Worker。未配置 D1 时，短码、目录、详情和下载仍可用，互动区域会显示未启用。

不要把真实账号令牌、数据库 ID 或管理凭据提交到仓库。国内访问取决于当前域名和网络线路；先用现有自定义域名做多地实测，再决定是否增加国内只读镜像。
