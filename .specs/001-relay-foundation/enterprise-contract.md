# Enterprise Contract v1

## 适用范围

本合同只服务需要自动签发下游模型 token、外部主体映射和批发对账的企业客户。普通直销客户继续使用 New API 原生用户与 token API。

## 身份与所有权

- `enterprise_tenant`：企业客户边界，拥有一个 Relay 内部执行用户，用于复用现有用户 quota、group 和请求级计费。
- `enterprise_subject`：`tenant_id + external_id` 唯一，只保存客户侧不可变主体标识，不复制客户身份资料。
- `enterprise_credential`：独立管理面凭据，仅保存 secret hash、显示前缀、状态、过期和最后使用时间；明文只在签发时返回一次。
- `token`：继续走现有模型数据面；企业 token 额外关联 tenant、subject 和稳定的外部 token id。

一个企业租户共享执行用户的批发额度，每个 token 继续执行自身额度、模型白名单、过期和撤销。需要成员组织、企业后台登录或独立终端用户账本时再扩展，不在 v1 内实现。

## 凭据边界

| Credential | 允许 | 禁止 |
|---|---|---|
| Relay root/admin session | 创建/停用 tenant，签发/轮换 service credential | 作为模型数据面 token |
| Enterprise service credential | 管理本 tenant 的 subject/token，查询本 tenant usage | 调用模型、访问其他 tenant、渠道、倍率、用户或系统设置 |
| Scoped model token | 调用允许的模型，查询自身 usage | 企业管理 API、Relay 后台管理 API |

`User.AccessToken` 不作为 service credential。企业管理 API 使用独立中间件，不能复用 `UserAuth`、`AdminAuth` 或 `TokenAuth` 形成隐式提权。

## HTTP API

所有企业自动化接口位于 `/api/enterprise/v1`，使用 `Authorization: Bearer <service credential>`：

- `POST /subjects/{external_id}/tokens`：幂等签发 token；请求携带客户生成的 `Idempotency-Key`。
- `GET /tokens/{external_token_id}`：查询 token 状态与 scope，不返回 secret。
- `DELETE /tokens/{external_token_id}`：幂等撤销 token。
- `GET /usage?cursor=<cursor>&limit=<limit>`：按稳定顺序增量读取本 tenant usage。

Relay 管理员使用单独的 `/api/enterprise-admin/v1` 创建/停用 tenant 和轮换 service credential。v1 不提供企业客户修改渠道、倍率、用户角色或 Relay 系统配置的接口。

## Token 请求合同

签发请求必须包含 `external_token_id`、`models`、`quota` 和 `expires_at`；可选 `name`。`external_subject_id` 来自路径。所有额度、模型和过期约束在写入现有 `Token` 前验证，重复 `Idempotency-Key` 返回第一次签发结果，不创建第二个 token。

明文 token 只在首次成功响应中返回。后续查询只返回掩码、状态、剩余额度、模型白名单和过期时间。

## 失败语义

- tenant、credential 或 subject 被停用后，不得继续签发新 token。
- service credential 失效不自动撤销已签发的模型 token；停用 tenant 必须显式禁用其全部 token。
- Cloud 不可用不影响已签发 token，除非 quota、过期、撤销或 tenant 状态要求拒绝。
- 跨 tenant 查询统一返回 `404`，不泄露资源是否存在。
