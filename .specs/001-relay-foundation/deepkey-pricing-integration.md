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
