# DeepKey 控制台首版上线对齐

## 目标

在不复制 DeepKey 私有代码、不共享数据库或服务凭据的前提下，按已登录的 DeepKey 控制台页面结构快速整理 Relay 用户端。页面使用 Relay 自身真实用户、密钥、用量、计费和支付接口。

## 页面映射

| DeepKey | Relay | 用户端名称 |
| --- | --- | --- |
| `/console` | `/dashboard/models` | 数据总览 |
| `/console/log` | `/usage-logs/common` | 计费详情 |
| `/console/topup` | `/wallet` | 我的账户 |
| `/console/token` | `/keys` | API 管理 |
| `/console/personal` | `/profile` | 账户设置 |

## 首版范围

- 数据总览展示账户数据、使用统计、资源消耗、性能指标、消耗图表、API 信息和系统公告。
- 我的账户采用充值与邀请奖励双栏布局，保留真实支付、订单、兑换码和邀请奖励逻辑。
- 首版隐藏暂不可用的旧易支付“支付宝”和“微信”入口，保留已对接的微信 Native 扫码能力以便后续启用。
- API 管理保留 Relay 真实密钥的创建、筛选、额度、分组、模型和 IP 限制能力。
- 计费详情使用 Relay 原始用量日志，默认每页 10 条；前端最多展示最近 500 条，数据库中的客户消费日志保留 7 天。
- 客户用量和费用始终归属于不可变的 `user_id`；API 密钥仅作为可更换的访问凭证和请求审计信息，不作为客户账务身份。
- 用户导航关闭聊天、游乐场、概览、任务日志、绘图日志、分流、用户统计和排行榜。
- 管理员导航保持现有管理能力；普通用户看不到管理员分组。

## 路由约束

- `/dashboard` 默认进入 `/dashboard/models`。
- `/dashboard/overview`、`/dashboard/flow`、`/dashboard/users` 重定向到 `/dashboard/models`。
- `/playground`、`/chat/*` 重定向到 `/dashboard/models`。
- `/usage-logs/task`、`/usage-logs/drawing` 重定向到 `/usage-logs/common`。

## 验证

- 前端 TypeScript、Oxlint、i18n 同步和生产构建通过。
- Docker 生产镜像构建成功，本地容器在 `http://127.0.0.1:3010` 启动。
- 桌面和手机宽度核对数据总览、我的账户、API 管理和计费详情，无页面级横向溢出。
- 充值套餐显示 `10/20/50/100/200/500/1000/2000`，显示额度与待支付金额保持 1:1。
- 微信和支付宝扫码支付继续使用现有真实支付接口，不使用 DeepKey 的支付账户或订单。
