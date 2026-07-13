# Design

> 本文描述 BlackRain 自有目标边界；New API 实际实现以锁定上游源码为准。目标图不能替代验证。

## 总体拓扑

```text
Direct customer control/data plane
  --user session---------> Relay account/top-up/token API
  --scoped model token---> Relay model API

BlackRain Cloud control plane
  --enterprise credential--> Relay management API
  <--scoped model token----- Relay

BlackRain Desktop data plane
  WORK/Hermes Chat ---------------------> Relay
  CODE/codex Responses -> local gateway -> Relay
                                            -> authorized model providers

Relay usage/API --idempotent records--> BlackRain Cloud reconciliation
```

## 所有权

- Relay：直销客户身份与商业账本、企业客户、模型渠道、路由、scoped token、原始 usage、执行 quota、速率限制、渠道成本和批发结算。
- Cloud：Supabase 身份、BlackRain 套餐、商业 credit ledger、支付、退款、市场和创作者结算。
- Desktop：本地项目、工作台、双引擎、系统凭据和本地 CODE 协议翻译。

## 安全边界

- 管理 API 和模型数据面使用不同 credential、权限和审计策略。
- Desktop token 只能访问允许模型和数据面端点；不能创建 token、渠道、用户或更改倍率。
- 上游模型 key 只存在 Relay Secret/数据库受控字段，不进入 Cloud/Desktop 或日志。
- 默认不记录请求/响应正文；如上游功能存在内容日志，生产 profile 必须显式关闭并以测试/审计证明。
- 所有外部 usage 事件有稳定 id，Cloud 重放不会重复扣款。

## 上游策略

- `main` 基于评估过的 exact tag/commit，不跟随上游 HEAD。
- 上游源码尽量保持原状；BlackRain 自有文档、合同和适配优先放独立文件/模块，降低升级冲突。
- 任何 UI 修改继续保留 New API 指定署名和原项目链接。

## 客户模型

- 普通直销客户直接使用 New API 现有 `User`、`Group` 和 `Token`，不增加平行账户体系。
- 需要自动签发下游 token、外部 subject 映射和批发结算的客户才进入 enterprise tenant 控制面。
- BlackRain Cloud 按上述企业合同接入，不获得 Relay 管理员权限，也不形成专用业务分支。

## 失败原则

- Cloud 不可用：是否允许已签发 token 继续工作由 token 合同决定，不静默扩大权限。
- Relay 不可用：不回退为 Desktop 直持平台模型 key。
- 渠道失败：仅在同模型/权限/计费语义内受控 fallback，记录实际渠道和成本。
- usage 无法结算：保留原始不可变记录并进入差错队列，不覆盖或伪造成功。
