# DeepKey 30% 分组倍率迁移

## 背景

DeepKey 价格目录现在保留模型基础价，并把统一 30% 加价放入分组倍率。历史版本可能已经把 1.3 倍写入 `ModelRatio` 或 `ModelPrice`；如果直接再次同步分组倍率，会产生 1.69 倍的重复加价。

## 迁移规则

- 只处理启用 DeepKey 渠道的模型；同名但未被启用 DeepKey 渠道确认的模型保持不变。
- 基础模型价从最新 DeepKey 公开目录读取，分组倍率从最新目录读取并保留 30% 加价。
- 目录同步不做除法猜测，不会修改其他上游或管理员手工维护的未确认模型。
- Root 管理员必须先预览，再携带快照哈希确认。预览列出模型变更、分组变更和同名分组冲突。
- 确认时在一个数据库事务中写入 `ModelRatio`、`ModelPrice`、`GroupRatio` 和 `DeepKeyPricingMigration` 来源/版本标记。
- 快照发生变化时拒绝应用，要求重新预览；重复应用同一结果只写入相同基础价，不会再次乘以 1.3。

## 管理接口

预览：

```powershell
Invoke-RestMethod -Method Post `
  -Uri https://relay.example.com/api/ratio_sync/deepkey/migrate `
  -Headers @{ Authorization = "Bearer $env:RELAY_ADMIN_ACCESS_TOKEN"; "New-Api-User" = "1" } `
  -ContentType "application/json" `
  -Body '{"apply":false}'
```

确认时把预览返回的 `data.snapshot_hash` 放入请求：

```powershell
Invoke-RestMethod -Method Post `
  -Uri https://relay.example.com/api/ratio_sync/deepkey/migrate `
  -Headers @{ Authorization = "Bearer $env:RELAY_ADMIN_ACCESS_TOKEN"; "New-Api-User" = "1" } `
  -ContentType "application/json" `
  -Body '{"apply":true,"snapshot_hash":"<preview snapshot_hash>"}'
```

接口不会返回或读取 DeepKey 渠道 Key。`409` 表示预览已过期或数据库在确认期间发生变化，应重新执行预览。
