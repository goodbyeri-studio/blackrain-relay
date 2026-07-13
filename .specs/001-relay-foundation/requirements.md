# Requirements

## 背景

BlackRain Relay 是基于 New API 的独立公开模型中转产品，直接向个人和企业客户销售。BlackRain Cloud 只是其中一个企业客户，不拥有 Relay 的身份、计费或运营边界。

## 目标

- 在保留上游 AGPL、Section 7、署名和原项目链接的前提下建立可持续同步的 Relay 基线。
- 直接复用 New API 的用户、分组、充值、token 和请求级计费能力形成直销闭环。
- 为 Cloud 企业客户提供受控的 scoped token、模型白名单、额度、过期、撤销和 usage 查询/事件。
- 对 WORK Chat 和 CODE Responses 提供稳定流式、工具调用、计量和结构化错误。
- 建立渠道成本、限流、批发结算、日志脱敏、备份、恢复、监控和升级能力。

## 非目标

- 不复制 BlackRain Cloud 的 Supabase 身份、套餐、支付或商业 credit ledger。
- 不为普通直销客户重复建设第二套 tenant、token 或计费系统。
- 不把 Desktop 用户项目、工作台或本地凭据上传到 Relay 管理面。
- foundation 阶段不删除 CODE 本地 Responses 翻译网关；New API Responses 必须先通过 codex 严格协议探针。
- 不使用订阅共享、非授权账号池或违反模型厂商条款的渠道。

## 成功标准

- 上游版本、commit、License、NOTICE 和 patch 集可审计、可重复同步。
- 直销用户可以完成注册/开户、充值或授额、签发 token、调用模型、查询 usage 和撤销 token。
- Cloud service account 不能用于模型数据面；Desktop model token 不能调用管理 API。
- token 支持客户、外部主体、模型、额度、速率、过期和撤销约束。
- usage 具有稳定 request/event id，可供 Cloud 幂等入账和日终对账。
- WORK Chat 与 CODE Responses/本地翻译路径在真实授权渠道完成流式、工具、错误和计量矩阵。
- 发布前完成数据库 migration/rollback、备份恢复、Secret 轮换、监控告警和 AGPL 源码提供验证。

## 开放问题

- [ ] usage 使用 webhook、增量拉取还是二者结合。
- [ ] New API `/v1/responses` 对 codex app-server 的兼容上限。
- [ ] BlackRain Cloud 批发额度与 Relay 执行 quota 的结算周期和超额策略。
- [ ] 首批模型渠道、区域、支付、备案、内容安全和日志留存要求。
