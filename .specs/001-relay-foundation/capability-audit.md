# New API Capability Audit

## 结论

锁定基线已经具备可复用的数据面、token、usage 日志和基础计费能力。BlackRain 不应重写模型中转核心；下一阶段应增加薄的 enterprise control plane，补齐 tenant/subject、service credential、受控发 token 和可重放 usage export。

## 能力矩阵

| BlackRain 需求 | New API 现有能力 | 差距 | 处理建议 |
|---|---|---|---|
| 模型数据面 | `/v1/chat/completions`、`/v1/responses`、Claude、Gemini、Realtime 等路由统一经过 `TokenAuth`、分发和计费 | 尚未用 BlackRain 真实授权渠道完成 E2E | 直接复用数据面，先做协议与计费矩阵 |
| Scoped model token | `Token` 已有过期时间、剩余额度、已用额度、模型白名单、IP 白名单、分组、状态和撤销 | 没有 enterprise tenant、external subject、用途/audience、每 token 速率策略 | 保留现有 token 执行路径，增加企业归属与 scope 元数据 |
| 管理身份 | `User.AccessToken` 可用于系统管理，管理员路由已有 RBAC | 不是独立的企业 service credential；现有 token 创建 API 只能给当前登录用户创建 token | 增加独立 service credential 与最小权限发行 API，不把 Cloud 设为 Relay 管理员 |
| 管理面/数据面隔离 | 模型请求使用 `TokenAuth`，后台管理使用 `UserAuth`/`AdminAuth`/`RootAuth` | 尚无自动化测试证明 Cloud credential 不能访问数据面、Desktop token 不能访问管理面 | 将负向鉴权测试作为合同冻结门槛 |
| Usage 原始记录 | `Log` 已记录 `request_id`、`upstream_request_id`、token、模型、渠道、token 数和 quota，并支持管理员/用户/token 查询 | 没有稳定 cursor、不可变 event id、增量导出、usage webhook 和重放协议；日志 `request_id` 不是全局唯一事件键 | 增加 usage outbox/event 表与版本化增量 API/Webhook，不用通知 webhook 代替账务事件 |
| 幂等计费 | subscription 预扣记录以 `request_id` 唯一，退款路径具备幂等基础 | 幂等范围主要服务内部预扣/退款，不等于 Cloud 商业账本幂等 | Relay 输出稳定 usage event id，Cloud 以 event id 幂等入账 |
| 配额与结算 | 用户/token quota、预扣/结算、模型倍率和渠道成本已有成熟路径 | 缺少 enterprise 批发账户、结算周期、超额策略和对账状态 | 复用请求级计费，只新增批发结算边界，不复制 Cloud credit ledger |
| Webhook | 已有带 HMAC 的用户通知 webhook | 负载是通知文案，不是版本化 usage 事件，也没有 delivery/outbox/retry 状态 | 单独实现 usage webhook，不扩展通知 payload 承担账务 |
| 企业客户映射 | 用户与 group 可表达基本隔离 | 没有 tenant、external subject、Cloud account 映射和客户级停用 | 增加独立 enterprise tenant/subject 映射层，禁止依赖 Supabase 表 |
| 速率限制 | 存在全局和 group 模型请求限流 | token 模型中没有每 token/subject 的速率参数 | 在企业 scope 上定义速率策略并接入现有限流中间件 |

## 可直接复用

- provider adaptor、协议转换、渠道路由和 fallback。
- token 校验、模型白名单、额度、过期、IP 限制、禁用和撤销。
- 请求级计量、预扣费、结算、退款与渠道成本。
- request id、上游 request id 和消费日志。
- 用户/group 作为内部执行主体的基础设施。

## 必须新增

1. `enterprise_tenant`、`external_subject` 与内部 user/token 的稳定映射。
2. 与 model token 分离的 service credential，以及只允许发行/撤销/查询本 tenant token 的管理 API。
3. token audience/scope、客户/subject 归属和每 token/subject 速率策略。
4. 不可变 usage event/outbox、稳定 event id、cursor、重放和 delivery 状态。
5. 版本化 OpenAPI/Webhook 合同、负向鉴权测试和 Cloud 幂等对账测试。
6. enterprise 批发额度、结算周期、超额与差错补偿状态。

## 暂不新增

- 不复制 Supabase identity、BlackRain 套餐、支付或商业 credit ledger。
- 不重写 provider adaptor、Responses/Chat 协议转换或请求级 billing。
- 不让 Cloud service credential 成为普通管理员或模型数据面 token。
