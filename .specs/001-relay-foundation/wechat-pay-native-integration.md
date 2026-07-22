# 微信支付 Native 扫码支付接入规范

> 状态：设计与实施依据
> 适用范围：BlackRain Relay 的普通商户模式（商户自有 `mchid`）微信支付 Native 扫码支付
> 最后核对：2026-07-22
> 上游规范：微信支付 API v3 商户文档；接口、字段、错误码以官方文档实时版本为准。

## 1. 目标、边界与决策

本文定义 Relay 接入微信支付的完整技术和运营要求，使用户能够在充值/购买额度页面通过微信扫码完成支付，并使本地支付订单、微信支付交易、Relay 额度入账和退款形成可审计、可重试、不可重复记账的闭环。

本期选择 **普通商户模式 + Native 支付（PC/网页展示二维码，由用户使用微信扫一扫付款）**：

- 商户后端调用 `POST /v3/pay/transactions/native`，从响应取得 `code_url`。
- Relay 前端只将 `code_url` 转为二维码展示；私钥、API v3 密钥、签名、查单、回调验签与额度入账均只能在 Relay 后端完成。
- 成功的唯一业务事实是：已验签、已解密、与本地订单逐项核验通过的微信支付成功通知，或在通知缺失/处理失败时由后端查单得到的 `SUCCESS` 状态。
- 本期不接入服务商模式、子商户、合单支付、付款码支付、JSAPI、小程序支付或 H5 支付。它们的 URL、签名主体、字段及商户号语义不同，不能把本规范直接套用。

Relay 只拥有支付接入、充值订单、原始支付交易证据、额度发放与本地退款流程。BlackRain Cloud 仍仅通过版本化 HTTP 合同取得必要的对账结果；不得共享数据库、商户私钥、API v3 密钥、证书或其他服务凭据，不得将 Relay 绑定到 Cloud 的 Supabase 表和私有账本逻辑。

## 2. 上线前必须具备的条件

### 2.1 商务与账户

1. 已注册并完成微信支付普通商户入驻，商户号（`mchid`）状态正常，且经营类目、结算账户、风控资料均已审核通过。
2. 商户平台已开通 Native 支付产品；不要假定所有普通商户自动拥有该权限。未开通时由商户管理员在微信支付商户平台按产品申请流程处理。
3. 至少有一个与 `mchid` 绑定的 `appid`。Native 下单也必须传 `appid`，而且该绑定关系必须存在。应使用 Relay 运营主体实际持有、可审计管理的公众号/小程序/开放平台应用 ID，不得借用无授权主体的 `appid`。
4. 明确售卖内容、用户协议、退款规则、客服入口、发票和税务处理。微信账单会展示 `description`，该描述必须真实反映充值/套餐商品，不能写成不透明或与实际交易不符的内容。
5. 生产前由法务/财务确认数字化额度、模型服务或套餐的经营资质、税务、消费者权益、退款和争议处置要求。本文件不构成合规意见。

### 2.2 环境与网络

| 项目 | 开发/联调 | 生产 |
| --- | --- | --- |
| 回调域名 | 可用公网 HTTPS 测试域名 | 固定、受控的生产 HTTPS 域名 |
| 回调路径 | `/api/payment/wechat/notify` | 同左，禁止依赖前端路由 |
| TLS | 有效证书，TLS 1.2+ | 有效证书、自动续期、TLS 1.2+、WAF 放行微信回调 |
| 出站访问 | 能访问微信支付 API 域名 | 同左，配置 DNS、代理、超时和出口监控 |
| 入站访问 | 微信支付服务器可访问 | 不经登录、IP 白名单误拦、验证码、前端重定向或人机验证 |
| 时钟 | NTP 同步 | 多节点统一 NTP，监控时钟漂移 |

回调地址必须由微信支付服务器可通过公网访问。它不是浏览器跳转地址：不能使用内网 IP、`localhost`、需登录的 URL、短期临时隧道、依赖 Cookie 的鉴权或会返回 HTML 登录页的网关。部署反向代理时必须把原始请求体无损转发给应用；验签使用的就是微信实际发送的原始字节。

### 2.3 需要向商户管理员索取的资料

| 资料 | 用途 | 保管与轮换要求 |
| --- | --- | --- |
| 商户号 `mchid` | 请求体、商户身份 | 可作为配置读取，不是密码 |
| 绑定 `appid` | Native 下单请求体 | 可作为配置读取 |
| API v3 密钥 | 解密回调/敏感资源 | 32 字节密钥；只进 Secret Manager，绝不进仓库、日志、前端或 Cloud |
| 商户 API 证书私钥（`apiclient_key.pem`） | 对 API 请求 RSA-SHA256 签名 | 高敏感；最小权限挂载、不可导出到前端 |
| 商户 API 证书序列号 | `Authorization` 的 `serial_no` | 可作为配置读取；证书轮换时同步更新 |
| 商户 API 证书（`apiclient_cert.pem`） | SDK 初始化/运维核验 | 与私钥配对，按微信平台要求更新 |
| 微信支付平台证书或平台公钥 | 验证微信响应与回调签名 | 仅信任从官方渠道获取且经过校验的材料；按轮换机制更新 |
| 回调 URL | 下单时的 `notify_url` | 固定生产 URL，变更需发布审批 |

**禁止事项**：不要把任何 PEM、API v3 密钥、完整 `Authorization`、回调密文、支付者标识或证书文件放入 Git、`.env.example`、前端构建产物、错误页面、普通应用日志、工单截图或 Cloud 的配置表。若私钥或 API v3 密钥疑似泄露，应立即在商户平台轮换并吊销旧材料，同时审查该时间段的订单和访问日志。

## 3. 推荐架构

```text
浏览器
  | 1. 创建充值意图（业务金额由服务端确定）
  v
Relay API ----------------------> Relay 数据库
  | 创建本地 payment_order            | 原子状态迁移/额度账本
  | 2. API v3 签名下单                v
  +----------------------------> 微信支付 API v3
  | <--------------------------- code_url
  | 3. 返回二维码内容
  v
浏览器展示二维码
  |
  | 4. 用户在微信内付款
  v
微信支付 --- 5. POST 回调 ---> Relay 回调入口 ---> 验签/解密/核验 ---> 原子入账
                                      |                                  |
                                      | 204                              +--> 额度账本、支付审计
                                      v
                                    微信支付

浏览器 --- 6. 轮询本地订单状态 ---> Relay API（绝不直接查微信支付）
定时补偿任务 --- 7. 查单/关单/退款 ---> 微信支付 API v3
```

建议以一个独立的支付服务边界实现：`controller -> service -> model`。Controller 只负责认证、参数解析和 HTTP 响应；Service 负责状态机、签名调用、回调核验与额度账本事务；Model 负责数据库读写与跨 SQLite/MySQL/PostgreSQL 的兼容。微信支付 SDK 是基础设施适配，不应在前端、控制器或 Cloud 合同层散落签名逻辑。

推荐使用微信支付官方 Go API v3 SDK（`github.com/wechatpay-apiv3/wechatpay-go`）处理签名、验签与 AES-GCM 解密，锁定经评估的版本并跟踪其安全更新。若因审计要求自行实现密码学代码，必须逐字节与官方 SDK 的测试向量互验；不得自创签名字符串、证书验证或 AES 解密实现。

## 4. 本地数据模型与金额规则

### 4.1 核心表/实体

以下是建议的领域模型。实际表名可随项目风格调整，但唯一约束、不可变金额和状态含义不可削弱。

| 实体 | 关键字段 | 约束/说明 |
| --- | --- | --- |
| `payment_order` | `id`、`user_id`、`provider`、`out_trade_no`、`amount_fen`、`currency`、`status`、`expires_at` | `provider + out_trade_no` 唯一；金额为正整型分；创建后不得修改金额、用户或币种 |
| `payment_transaction` | `payment_order_id`、`wechat_transaction_id`、`trade_state`、`success_time`、`bank_type`、`payer_openid_hash` | `wechat_transaction_id` 唯一且可空直到成功；只保存满足业务需要的最少字段 |
| `payment_notification` | `wechat_event_id`、`received_at`、`headers_digest`、`body_digest`、`verification_status`、`processing_status` | `wechat_event_id` 唯一；可保留加密原文于受限审计存储，普通日志仅存摘要 |
| `payment_refund` | `payment_order_id`、`out_refund_no`、`wechat_refund_id`、`refund_amount_fen`、`status` | `out_refund_no` 唯一；退款累计额不得超过已支付金额 |
| `credit_ledger` | `idempotency_key`、`user_id`、`delta`、`reason`、`payment_order_id` | `idempotency_key` 唯一；支付成功入账与退款扣回均须可追溯 |

建议状态机如下。数据库状态更新必须使用带条件的原子更新或短事务内的行锁，避免并发回调、轮询补偿、人工操作和退款任务重复发放额度。

```text
CREATED -> PENDING -> PAID -> CREDITED
   |         |         |       |
   |         |         |       +-> REFUNDING -> REFUNDED
   |         |         +-> CREDIT_FAILED (待重试/人工处置，不得视为未支付)
   |         +-> CLOSED / EXPIRED
   +-> CANCELLED

只有 SUCCESS 回调或查单 SUCCESS 能进入 PAID。
只有本地额度账本成功且幂等键已落库，才能进入 CREDITED。
```

### 4.2 金额、商品与订单号

1. 金额只能由后端根据服务端价格表、套餐 SKU、优惠规则和币种计算。浏览器传来的金额、商品名称、`appid`、`mchid`、`notify_url`、`out_trade_no` 一律忽略或仅作为校验线索，绝不能直接转发给微信支付。
2. 微信支付 `amount.total` 的单位是**分**，为大于零的整数，币种固定传 `CNY`。应用内也统一用整数分或项目既有的安全整数额度转换规则，禁止 `float64` 参与金额结算。
3. `out_trade_no` 由 Relay 生成，6 至 32 个字符，只能使用数字、大小写字母、`_`、`-`、`|`、`*`，并在同一 `mchid` 下唯一。建议格式：`BRP` + 时间排序段 + 随机安全段；生成后写入数据库唯一索引，冲突时重试。
4. `description` 不超过 127 个字符，应如实描述，例如“BlackRain Relay 额度充值（100 元）”。不要记录用户邮箱、手机号、模型提示词、API key 或业务敏感数据。
5. `attach` 可选，最长 128 字符。推荐不放业务明文；如必须关联，写不可逆/无敏感含义的短标识。支付成功回调、查单和账单都会返回该字段。
6. `time_expire` 使用 RFC 3339（如 `2026-07-22T14:30:00+08:00`）。建议业务上设置 10 至 30 分钟，且后台到期后主动调用关单。微信支付未传时默认时效较长；即使 `time_expire` 到达，也不等于本地订单已安全关闭。

### 4.3 创建支付订单 API

建议内部 API：`POST /api/user/payments/wechat/native`。

请求只接受受控的 `sku_id`、可选优惠码和客户端幂等键；不得接收金额。响应只返回本地订单 ID、金额、币种、`expires_at`、二维码内容和本地状态查询 URL：

```json
{
  "payment_order_id": "pay_01J...",
  "amount_fen": 10000,
  "currency": "CNY",
  "code_url": "weixin://wxpay/bizpayurl?...",
  "expires_at": "2026-07-22T14:30:00+08:00",
  "status_url": "/api/user/payments/pay_01J..."
}
```

客户端只把 `code_url` 生成为二维码，不解析、不修改、不缓存到公共 CDN，也不使用它判断支付成功。建议二维码页面每 2 至 5 秒查询本地 `status_url`，最长不超过订单有效期；服务端完成入账后返回 `CREDITED`。轮询应有登录鉴权、订单归属校验、限速和退避。

## 5. API v3 请求签名与 Native 下单

### 5.1 API 基础信息

| 项目 | 值 |
| --- | --- |
| 请求方法 | `POST` |
| 路径 | `/v3/pay/transactions/native` |
| 主域名 | `https://api.mch.weixin.qq.com` |
| 备域名 | `https://api2.mch.weixin.qq.com`，仅按官方容灾指引使用 |
| Content-Type / Accept | `application/json` |
| 认证 | `WECHATPAY2-SHA256-RSA2048` |
| 成功响应 | JSON 中的 `code_url` |

签名串精确为下列五行，最后一行后也必须有换行：

```text
HTTP请求方法\n
请求URL（path + query）\n
时间戳（秒）\n
随机字符串\n
请求体原始字节\n
```

用商户 API 证书私钥按 RSA-SHA256 对该串签名，Base64 后组装请求头：

```text
Authorization: WECHATPAY2-SHA256-RSA2048 mchid="商户号",nonce_str="随机串",timestamp="Unix 秒",serial_no="商户证书序列号",signature="Base64签名"
```

不要对 JSON 进行二次格式化后再签名，不要把完整 URL（含域名）代入签名的第二行，不要重复使用 nonce，也不要在签名后让 HTTP 客户端改写 body。SDK 应接管这些易错细节。

### 5.2 下单请求示例

以下仅说明字段关系；生产代码应使用已初始化的官方 SDK 客户端，而不是手写 HTTP 和签名：

```json
{
  "appid": "wx1234567890abcdef",
  "mchid": "1900000001",
  "description": "BlackRain Relay 额度充值（100 元）",
  "out_trade_no": "BRP20260722A1B2C3D4",
  "time_expire": "2026-07-22T14:30:00+08:00",
  "attach": "pay_01J...",
  "notify_url": "https://relay.example.com/api/payment/wechat/notify",
  "amount": {
    "total": 10000,
    "currency": "CNY"
  }
}
```

处理下单请求的顺序：

1. 验证当前用户、SKU、购买限制、优惠资格和客户端幂等键。
2. 在数据库创建 `CREATED` 本地订单，固化服务器计算出的金额、商品、过期时间和唯一 `out_trade_no`。
3. 调用 Native 下单 API。网络超时不代表下单失败：在重试前，先按 `out_trade_no` 调用查单 API 确认是否已创建/支付，避免同一本地订单产生不确定的远端状态。
4. 成功取得 `code_url` 后更新为 `PENDING` 并返回；仅在短期受控存储中保留必要的二维码数据。
5. 若明确下单失败且未创建远端交易，可记录失败原因并允许用户创建新订单；不要重用失败/关闭订单的 `out_trade_no`。

对 HTTP 429、5xx、连接失败和超时使用受限指数退避，且每次重试前查单。4xx 业务错误应记录微信的 `code`、`message`、`detail`（经脱敏）并按错误类型处理，不能无限重试。生产监控应按错误码、商户号、接口、网络域名和时间段聚合。

## 6. 支付成功回调：验签、解密、核验、入账

### 6.1 回调绝对规则

微信支付会向下单时的 `notify_url` 发送 JSON `POST`。回调可能重复、乱序、延迟，可能在应用重启期间重发；同一订单还可能被查单任务同时确认。因此：

- **先验签，再解析业务内容；先解密，再信任资源字段。**
- **验签/解密失败时绝不入账、绝不返回成功确认。**
- **验签通过也不能直接入账，必须逐项匹配本地不可变订单。**
- 回调处理必须幂等。无论同一个通知到达多少次，最终最多产生一笔额度账本入账。
- 回调成功响应为 HTTP `204 No Content`。只有订单已安全处理或已经被幂等处理过，才返回 204；临时故障返回 5xx 以请求微信支付重试。

### 6.2 验证微信支付签名

从请求头读取：

```text
Wechatpay-Timestamp
Wechatpay-Nonce
Wechatpay-Signature
Wechatpay-Serial
```

拼接验签消息（`body` 必须是原始请求字节，不是重新序列化后的对象）：

```text
Wechatpay-Timestamp + "\n" + Wechatpay-Nonce + "\n" + body + "\n"
```

使用与 `Wechatpay-Serial` 对应、可信的**微信支付平台证书/平台公钥**验证 `Wechatpay-Signature`。不要使用自己的商户证书验证回调；不要从未验证的回调正文下载并信任证书；不要因为序列号未知而跳过验签。SDK 的证书/平台公钥验证器应负责平台材料缓存和受控更新。

另做合理的时间窗口检查（例如 5 分钟，具体以官方建议和时钟漂移策略为准），并对 `event.id` 做持久化去重。这些是抗重放的补充，不能替代密码学验签。

### 6.3 解密通知资源

验签通过后，外层通知的 `resource` 使用 AES-256-GCM 加密，典型字段为：

```json
{
  "id": "通知唯一ID",
  "event_type": "TRANSACTION.SUCCESS",
  "resource_type": "encrypt-resource",
  "resource": {
    "algorithm": "AEAD_AES_256_GCM",
    "ciphertext": "...",
    "associated_data": "transaction",
    "nonce": "..."
  }
}
```

使用 API v3 密钥以及 `nonce`、`associated_data`、`ciphertext` 解密 `resource`。解密后的交易对象应至少读取：`appid`、`mchid`、`out_trade_no`、`transaction_id`、`trade_type`、`trade_state`、`success_time`、`attach`、`amount.total`、`amount.currency`、`payer` 和退款/优惠相关字段（如业务需要）。API v3 密钥只用于对称解密资源，不能用于请求签名或替代平台签名验证。

### 6.4 业务核验清单

仅当全部条件成立时才可把订单推进到 `PAID`：

| 核验项 | 期望值 | 不匹配处理 |
| --- | --- | --- |
| `event_type` | 支付成功事件 | 记录并按事件类型处理，不入账 |
| `trade_state` | `SUCCESS` | 非成功不入账；按状态机/查单处理 |
| `trade_type` | `NATIVE` | 记录安全事件，不入账 |
| `out_trade_no` | 本地唯一订单 | 不存在或归属不符时进入人工差错队列 |
| `mchid` / `appid` | 当前受控配置 | 不入账，按高优先级安全告警 |
| `amount.total` | 本地固化金额（分） | 不入账，按高优先级安全告警 |
| `amount.currency` | `CNY` | 不入账 |
| `attach` | 若使用则精确匹配 | 不入账并审计 |
| `transaction_id` | 未绑定其他本地订单 | 冲突即冻结自动处理并人工复核 |
| 本地状态 | 可从 `PENDING`/补偿状态转入 `PAID` | 已入账时走幂等成功；关闭/退款状态须人工/补偿规则处理 |

在同一短事务中完成：锁定本地订单、创建/确认唯一 `payment_transaction`、插入带唯一幂等键的 `credit_ledger`、增加用户额度、更新订单至 `CREDITED`。任何一步失败时整体回滚并返回 5xx，或将订单标为可恢复的 `CREDIT_FAILED` 后由可靠任务重试；绝不能只更新订单为成功而未真正入账，也绝不能先加额度再丢失幂等记录。

`credit_ledger.idempotency_key` 建议使用稳定、来源明确的值，如 `wechat:transaction:{transaction_id}`。本地订单号也应保留唯一约束，用于抵抗同一用户的重复点击和回调重放。

### 6.5 Go 伪代码骨架

以下展示顺序，非可直接复制的生产实现。所有 JSON 编解码应遵循项目的 `common` JSON 包约束。

```go
func HandleWechatPayNotify(c *gin.Context) {
    rawBody, err := io.ReadAll(c.Request.Body)
    if err != nil {
        c.Status(http.StatusInternalServerError)
        return
    }

    // verifier 必须使用可信的平台证书/公钥；rawBody 不可被重编码。
    notification, err := verifier.VerifyAndDecrypt(c.Request.Header, rawBody, apiV3Key)
    if err != nil {
        common.SysError("wechat pay notification verification failed")
        c.Status(http.StatusUnauthorized)
        return
    }

    if err = paymentService.ConfirmWechatNativePayment(c.Request.Context(), notification); err != nil {
        // 可恢复错误返回 5xx，交由微信重试；错误细节不得返回给调用方。
        c.Status(http.StatusInternalServerError)
        return
    }
    c.Status(http.StatusNoContent)
}
```

生产实现还必须限制请求体大小、保留请求 ID、对敏感字段作日志脱敏，并确保错误分支不会泄漏密钥、签名、密文或用户信息。

## 7. 查单、关单与补偿

回调是主要通道，但不能是唯一通道。应实现受控的补偿任务：

1. 对 `PENDING` 且接近/超过业务有效期的订单，按商户订单号调用 `GET /v3/pay/transactions/out-trade-no/{out_trade_no}?mchid={mchid}`。
2. 若查到 `SUCCESS`，复用**同一个**核验和幂等入账服务，不要单独写一套“查单成功即加额度”的简化逻辑。
3. 若仍为未支付且到达本地过期策略，调用 `POST /v3/pay/transactions/out-trade-no/{out_trade_no}/close` 关单，并仅在微信确认/可证明不可支付后将本地状态推进为 `CLOSED`。
4. 关单、下单和查单之间存在竞态：关单失败或返回状态不确定时必须重新查单。任何时候看到 `SUCCESS`，都优先保护用户已付款事实并进入幂等入账路径。
5. 对长时间不确定、金额不符、未知订单、重复 `transaction_id`、账本失败等情况，写入持久化差错队列并告警，不要由定时任务无限重试或静默丢弃。

前端“支付成功”页面仅根据本地 `CREDITED` 显示；不得因二维码扫描、浏览器返回、前端定时器或客户端声称支付完成而显示成功。

## 8. 退款闭环

退款不是删除充值订单，也不是直接给用户“负额度”就结束。它必须有独立订单、微信退款结果与本地商业规则。

### 8.1 业务前置条件

1. 明确退款规则：可退款期限、是否允许已经消耗的额度退款、人工审核阈值、部分退款、拒绝退款和争议处理。
2. 先锁定支付订单和相关额度账本，计算可退的最大金额。累计已退款金额不得超过已成功支付金额。
3. 若额度已经被消耗，禁止盲目扣成负余额。应根据既定规则拒绝、人工处理，或建立受控欠款/补偿流程；不得借退款接口制造不受约束的负向账务。
4. `out_refund_no` 由 Relay 生成且全局唯一，按 `payment_order_id + 退款业务原因` 保持幂等。

### 8.2 API 与处理

| 场景 | API v3 |
| --- | --- |
| 发起退款 | `POST /v3/refund/domestic/refunds` |
| 按商户退款单号查退款 | `GET /v3/refund/domestic/refunds/{out_refund_no}` |
| 接收退款结果 | 使用退款结果回调通知并执行同样的验签/解密 |

退款请求传入原 `transaction_id` 或 `out_trade_no`、唯一 `out_refund_no`、退款金额（分）、原订单总金额（分）、原因和可选 `notify_url`。退款成功的最终依据是经验证的退款成功通知或主动查询到终态成功，不是退款请求返回 200。

退款成功时，用稳定的退款 ID/退款单号作为幂等键创建反向账本记录，更新 `payment_refund` 与支付订单退款聚合状态。若本地扣减额度失败，必须将退款订单进入待处理状态并立即告警；不能把用户资金已退回和本地服务权益仍可无限使用的差异隐藏掉。

## 9. 安全、隐私与运维控制

### 9.1 Secret 管理

- 按环境分离商户号、证书私钥、证书序列号、API v3 密钥、平台证书/公钥缓存和回调地址；开发、测试、生产不能共用生产私钥。
- 生产 Secret 仅授予支付服务运行身份及必要的值班轮换流程；禁止管理后台、普通 API 服务、前端构建和 Cloud 读取。
- 为证书、私钥和 API v3 密钥设置到期/轮换提醒。轮换按“双材料并行验证/签名 -> 健康验证 -> 切换 -> 吊销旧材料”的受控流程实施，避免回调验签中断。
- 任何环境变量调试输出都必须过滤 `WECHATPAY`、`API_V3_KEY`、`PRIVATE_KEY`、`CERT`、`SIGNATURE` 等敏感键及其值。

### 9.2 日志、审计和隐私

- 记录：本地订单 ID、`out_trade_no`（可按权限展示）、`transaction_id`（最小化展示）、状态转换、金额分、错误码、通知 ID、处理时间、幂等结果和操作者。
- 不记录：完整回调正文、完整 `code_url`、完整签名、nonce、平台/商户证书、API v3 密钥、支付者 `openid`。若合规审计确需原始通知，进行字段加密、独立访问控制、保留期和访问审计。
- 用户可见页面只显示本人的订单信息；管理员查看支付记录需 RBAC、审计轨迹和按需脱敏。
- 支付 API 及回调日志使用关联 ID（本地订单 ID、请求 ID），不要用用户数据或密钥作为关联键。

### 9.3 防滥用与可用性

- 创建支付订单必须绑定登录用户、SKU 白名单、频率限制、并发未支付订单上限和客户端幂等键；避免攻击者批量创建二维码造成对账噪声。
- 状态查询必须校验订单归属，防止枚举订单 ID 或交易状态。
- 回调路径可允许匿名访问，但只接受预期的 POST/Content-Type/请求体大小；安全性来自微信签名验签，不是简单的来源 IP 信任。IP 规则只能作为额外运营控制，不能作为唯一认证。
- 为下单、回调、验签失败、解密失败、查单补偿、关单、入账、退款、账本失败分别建立指标和告警；关注成功率、P95 延迟、回调积压、未入账成功单、未知订单和金额不符。
- API 域名访问失败时按官方容灾文档处理；不要自行轮询未知 IP 或用未经批准的第三方代理转发支付请求。

## 10. 实施计划

当前实现使用以下运行时环境变量，真实值只允许进入部署 Secret：`WECHAT_PAY_APP_ID`、`WECHAT_PAY_MCH_ID`、`WECHAT_PAY_MERCHANT_SERIAL_NO`、`WECHAT_PAY_MERCHANT_PRIVATE_KEY_PATH`、`WECHAT_PAY_PUBLIC_KEY_ID`、`WECHAT_PAY_PUBLIC_KEY_PATH`、`WECHAT_PAY_API_V3_KEY`、`WECHAT_PAY_NOTIFY_URL`。证书路径必须指向容器或主机内的只读挂载文件，不得指向仓库文件。

### 阶段 A：配置与基础设施

- [ ] 由运营主体确认普通商户、Native 产品、`appid` 绑定及结算状态。
- [ ] 建立各环境 Secret、证书挂载、平台证书/公钥更新机制和回调 HTTPS 域名。
- [ ] 引入并锁定官方 API v3 Go SDK，完成证书私钥加载、出站客户端和响应验签的基础封装。
- [ ] 在 `.env.example` 仅添加不含秘密的键名及说明；真实值只进入部署 Secret。

### 阶段 B：创建订单与二维码

- [ ] 设计/迁移支付订单、交易、通知、退款及额度账本的表、唯一索引和状态枚举，验证 SQLite、MySQL、PostgreSQL 兼容性。
- [ ] 实现服务器定价、幂等创建本地订单、Native 下单、`code_url` 返回和本地状态查询。
- [ ] 实现过期订单查单/关单补偿，并对不确定网络结果先查单再重试。
- [ ] 实现前端二维码、加载/过期/重新创建状态及本地状态轮询；所有新增用户文案接入现有 i18n。

### 阶段 C：回调、入账与退款

- [ ] 实现原始 body 读取、微信平台签名验证、AES-256-GCM 解密、字段核验和 204 响应。
- [ ] 实现数据库事务内的幂等入账；覆盖重复通知、查单与通知并发、账本失败重试和交易 ID 冲突。
- [ ] 实现退款申请、退款查单、退款回调和额度/账本反向处理。
- [ ] 建立受控人工差错队列和支付运营后台的最小查询能力。

### 阶段 D：发布与运营

- [ ] 使用独立小额测试订单完成真实环境 E2E：下单、扫码、回调、入账、重复通知、查单补偿、关单、全额/部分退款。
- [ ] 压测仅针对自有创建订单/状态查询接口；不得对微信支付生产 API 做无授权压力测试。
- [ ] 完成失败演练：回调 5xx、应用重启、数据库回滚、微信 API 超时、证书轮换、重复回调和金额不符。
- [ ] 完成财务日对账、告警值班、密钥轮换 Runbook、退款审批流程和回滚方案后再逐步放量。

## 11. 验收测试矩阵

| 用例 | 期望结果 |
| --- | --- |
| 正常下单 | 服务端金额正确、成功返回 `code_url`、本地为 `PENDING` |
| 用户完成扫码支付 | 有效回调经核验后仅生成一笔额度账本，订单为 `CREDITED` |
| 同一回调重复投递 | 每次均安全返回 204，额度和账本只增加一次 |
| 回调在订单状态轮询前到达 | 前端随后从本地状态读到 `CREDITED` |
| 回调验签失败/序列号未知 | 非 2xx、无订单状态变化、产生安全告警 |
| 回调解密失败 | 非 2xx、无入账、记录脱敏诊断 |
| `amount.total`/`appid`/`mchid` 不匹配 | 无入账、订单冻结/差错队列、最高级别告警 |
| 微信 API 下单超时 | 先查单再决定重试，不产生重复远端订单或重复本地订单 |
| 未收到回调但查单 `SUCCESS` | 复用统一确认逻辑，准确入账一次 |
| 到期未支付 | 关单后本地为 `CLOSED`，二维码不再显示可支付 |
| 关单与支付竞态 | 最终以查单 `SUCCESS` 为准，绝不漏发用户额度 |
| 多次创建同一客户端请求 | 客户端幂等键返回同一本地订单，不重复下单 |
| 全额/部分退款成功 | 退款金额受上限约束，反向账本仅一次，订单聚合状态准确 |
| 退款回调重复 | 不重复扣减额度或重复记录退款 |
| 数据库暂时不可用 | 回调不返回 204；恢复后重试/补偿可完成，账本不丢失 |
| 证书轮换 | 新旧材料切换期间下单、回调验签和查单均连续可用 |

测试应使用确定性断言保护真实合同：签名/验签失败不可入账、金额核验、幂等键唯一性、状态机非法迁移、回调与补偿并发、跨数据库迁移兼容性。不得把真实私钥、真实 API v3 密钥或生产交易样本提交进测试夹具。

## 12. 运行手册要点

### 12.1 用户已付款但未到账

1. 用本地订单号定位 `out_trade_no`、订单状态、通知处理记录和账本幂等键。
2. 检查是否存在已验签成功但账本失败的记录；优先修复/重放同一幂等确认任务。
3. 若没有有效通知，按商户订单号查单；仅查到 `SUCCESS` 后进入统一确认流程。
4. 若金额、商户号、应用 ID 或交易 ID 不匹配，停止自动入账，保留审计证据并由支付值班人员人工处理。
5. 不根据用户截图、前端提示或口头陈述直接加额度；可将其作为工单线索。

### 12.2 回调验签大量失败

检查平台证书/平台公钥是否过期或未更新、反向代理是否改写 body/编码、系统时钟是否漂移、是否误用商户证书验签、`Wechatpay-Serial` 是否被未知材料冒用，以及是否存在攻击流量。不要为恢复业务临时关闭验签。

### 12.3 证书或 API v3 密钥泄露

立即按商户平台流程轮换对应材料，更新 Secret 并滚动重启受控工作负载，撤销旧材料，检查访问审计与交易异常，评估是否需要暂停下单/退款。泄露处置期间不得在聊天、工单或日志中再次传播秘密值。

## 13. 官方资料索引

以下链接在编写本文时有效；实施、升级和故障处理前必须再次核对最新官方版本与商户平台公告。

- [Native 下单](https://pay.weixin.qq.com/doc/v3/merchant/4012791877)：`POST /v3/pay/transactions/native`、请求字段、`code_url`。
- [Native 支付开发指引](https://pay.weixin.qq.com/doc/v3/merchant/4012791891)：完整用户支付流程与查单/关单说明。
- [微信支付订单号查询订单](https://pay.weixin.qq.com/doc/v3/merchant/4012791879) 与 [商户订单号查询订单](https://pay.weixin.qq.com/doc/v3/merchant/4012791880)。
- [关闭订单](https://pay.weixin.qq.com/doc/v3/merchant/4012791881)。
- [支付成功回调通知](https://pay.weixin.qq.com/doc/v3/merchant/4012791882)。
- [退款申请](https://pay.weixin.qq.com/doc/v3/merchant/4012791883)、[查询单笔退款](https://pay.weixin.qq.com/doc/v3/merchant/4012791884)、[退款结果回调通知](https://pay.weixin.qq.com/doc/v3/merchant/4012791886)。
- [签名认证](https://pay.weixin.qq.com/doc/v3/merchant/4012365342)：后端请求签名、响应/通知验签。
- [证书密钥概览](https://pay.weixin.qq.com/doc/v3/merchant/4024350132)。
- [通用参数说明](https://pay.weixin.qq.com/doc/v3/merchant/4012068676)：订单号字符集等约束。
