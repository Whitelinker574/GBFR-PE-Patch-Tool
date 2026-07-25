# GBFR 配装分享服务

这是桌面端 GBLC 短码的兼容 Worker，同时提供一个轻量的公开配装图鉴：

- `/`：按角色浏览公开配装；
- `/s/<短码>`：脱敏预览、下载和复制短码；
- `POST /api/v1/loadouts`：保留的二进制发布接口；
- `GET /api/v1/loadouts/<短码>`：桌面端使用的原始 GBLC 帧；
- `GET /api/v1/loadouts/<短码>/meta`：网页使用的预览元数据；
- `GET /api/v1/loadouts?character=伊欧`：目录查询；
- `POST .../like`、`POST .../comments`：可选 D1 社区互动。

网页预览只接受桌面端传来的白名单字段，绝不公开 `OwnerCode`、SlotID、存档路径、PID 或原始内存。旧客户端只发布二进制帧时，仍可正常导入；网页会显示有限的基础信息。

部署顺序：先部署 R2 版本并绑定自定义域名，再按 `migrations/README.md` 启用 D1。不要把 API 令牌、R2 密钥或真实 D1 ID 写入仓库。
