# 支付宝扫码预创建支付集成

## 范围

- Relay 通过支付宝开放平台 `alipay.trade.precreate` 生成付款二维码。
- 前端钱包在内置页面弹窗展示二维码并轮询 Relay 订单状态。
- Relay 负责本地充值订单、支付宝异步通知验签、主动查单补偿、幂等入账和充值日志。
- 不接入 BlackRain Cloud 数据表、商业账本或私有业务逻辑。

## 配置

运行时通过部署 Secret 提供：

- `ALIPAY_APP_ID`
- `ALIPAY_APP_PRIVATE_KEY_PATH`
- `ALIPAY_PUBLIC_KEY_PATH`
- `ALIPAY_NOTIFY_URL`
- `ALIPAY_GATEWAY_URL`，留空时使用 `https://openapi.alipay.com/gateway.do`
- `ALIPAY_APP_AUTH_TOKEN`，仅 ISV 代调用场景需要

私钥和支付宝公钥只允许以只读文件挂载，不得提交到仓库。

## 支付流

1. 用户选择 `alipay_precreate` 并确认充值。
2. Relay 创建 `topups` 和 `alipay_orders` 本地待支付订单。
3. Relay 请求支付宝 `alipay.trade.precreate`，保存 `qr_code`。
4. 前端展示二维码并轮询 `/api/user/alipay/precreate/order/:trade_no`。
5. 支付宝异步通知 `/api/payment/alipay/notify`，Relay 验签并核对应用、金额、币种、订单状态。
6. Relay 使用 `alipay_notifications.event_id` 去重，成功后只入账一次。
7. 轮询查询会调用 `alipay.trade.query` 作为回调补偿。

## 安全要求

- 所有支付宝通知必须使用支付宝公钥验签。
- `app_id` 必须与当前配置一致。
- 只允许 `TRADE_SUCCESS` 和 `TRADE_FINISHED` 入账。
- `total_amount` 转分后必须与本地订单金额完全一致。
- 本地订单状态必须为 `pending` 才能入账。
- `CreditQuota` 在建单时已用 `common.QuotaFromDecimalChecked` 饱和检查，入账时再次确认正数且不超过 `common.MaxQuota`。

## 验证

- `controller/topup_alipay_test.go` 覆盖金额转分和签名串排序。
- `model/alipay_test.go` 覆盖回调幂等和金额不一致拒绝入账。
- `setting/payment_alipay_test.go` 覆盖配置 URL 与密钥路径校验。
