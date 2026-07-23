# DeepKey 模型定价同步

## 目标

- 将 `https://deepkey.top/api/pricing` 作为内置价格源。
- 公开目录只提供候选模型和描述，不作为用户分组授权或渠道可调用性的依据。
- 管理员显式同步时，只发布 Relay 中已有启用渠道的同名分组。
- 客户只使用 Relay 本地 Token；DeepKey Key 仅保存在 Relay 渠道配置中。

## 定价与权限

- Token 模型的统一加价只作用于 `model_ratio`，按次模型只作用于 `model_price`。
- `completion_ratio`、缓存、图片和音频倍率不重复加价。
- 加价百分比必须在 `0` 到 `1000` 之间，后端独立校验。
- 目录模型在价格接口中标记为 `catalog_only` 和 `catalog_source=DeepKey`。
- 本地同名模型优先于目录模型；目录模型必须通过当前用户的 `GetUserUsableGroups` 过滤。
- 目录中的分组倍率不能扩大用户权限，只能补充当前用户已获准分组的描述性数据。

## 缓存与同步

- 目录缓存有效期为 15 分钟，远程请求超时为 8 秒，响应上限为 5 MiB。
- 缓存过期后继续返回旧值并在后台刷新；并发刷新通过 `singleflight` 合并。
- 刷新失败后短期退避，避免上游故障时每个价格请求都触发远程访问。
- 管理员分组同步等待一次最新目录，不使用 stale-while-revalidate 的旧值。
- 分组同步在同一事务中更新 `GroupRatio`、`UserUsableGroups` 和 `AutoGroups`。
- 同步仅保留至少有一个启用本地渠道的分组，并拒绝非正数、NaN、Inf 或过大的倍率。

## 可复核审计

使用脱敏脚本读取 DeepKey 当前公开目录、Relay 渠道列表，并通过 Relay 的模型探测接口核对每个 DeepKey 渠道。脚本不会读取或输出渠道 Key，结果包含 UTC 时间、Relay 版本和 SHA-256：

```powershell
./.specs/001-relay-foundation/audit-deepkey-production.ps1 `
  -RelayBaseUrl https://relay.example.com `
  -AdminAccessToken $env:RELAY_ADMIN_ACCESS_TOKEN `
  -AdminUserId 1
```

目录、分组和渠道数量以脚本当次输出为准，不在规格中固定，也不作为自动化测试的替代品。

## 渠道模型倍率覆盖

- 管理员可在渠道编辑器的“渠道模型倍率”区域，为当前渠道已发布的模型设置倍率。
- 配置保存在渠道 `settings.model_ratios` 中，键为客户端请求模型名，值为非负倍率，最大值为 `1000000`。
- 按量计费时优先使用当前渠道的模型倍率；没有覆盖值时回退到全局模型倍率。固定价格模型和 tiered expression 暂不改变其原有计费路径。
- 删除模型或删除覆盖值后，保存渠道会清理无效配置，避免隐藏模型继续影响计费。

## 客户文档品牌与入口

- 客户文档随默认前端发布在 `/docs/`，生产入口为 `https://goodbyeri.cc/docs/`。
- 文档中的站点品牌、注册链接、Claude API 地址、Codex API 地址和页脚统一使用 `goodbyeri.cc`，不向客户展示上游站点。
- `web/default/scripts/sync-goodbyeri-docs.mjs` 用于同步文章和图片；同步结果必须移除脚本、事件属性、嵌入对象和危险 URL，并通过 CSP、iframe sandbox 与本地资源校验。
- 旧配置值 `https://doc.deepkey.top` 和 `https://doc.deepkey.top/` 在数据库迁移时更新为新文档入口；管理员配置的其它自定义地址保持不变。
- DeepKey 价格目录、分组同步和渠道地址仍属于内部上游集成，不随客户文档域名替换。

## 兼容性

DeepKey 的价格接口曾返回过 JSON 字符串封装的 JSON 文档。同步器同时接受标准 JSON 对象和该双重编码格式。

## 2026-07-22 验证快照

- 上游返回 `226` 个模型，其中 `214` 个按 Token 计费、`12` 个按次计费。
- 返回 `7` 个供应商和 `8` 类端点。
- `gpt-3.5-turbo` 输入倍率由 `0.25` 加价为 `0.325`，输出倍率保持 `0.25`。
- 后端定向单元测试通过。
- 前端 TypeScript 类型检查、变更文件 lint、生产构建通过。
- 全仓 lint 存在与本次改动无关的既有错误，不作为本功能验收项。
