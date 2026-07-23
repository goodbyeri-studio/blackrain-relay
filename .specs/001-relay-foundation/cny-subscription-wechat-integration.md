# CNY 套餐与微信支付闭环

## 决策

- DeepKey 当前账户页没有独立套餐目录，只有按量充值档位。因此 MeiMei API 使用现有 `amount_options` 的人民币档位初始化套餐：10、20、50、100、200、500、1000、2000 元。
- 仅在 `subscription_plans` 为空时幂等创建默认套餐；管理员已经配置过套餐时不覆盖、不追加。
- 套餐微信 Native 扫码订单使用独立的 `subscription_wechat_pay_orders` 表，回调先按本地订单查找业务，再区分套餐和钱包充值，禁止仅凭订单号前缀判断。
- 微信回调和主动查单都调用幂等结算逻辑，金额、币种、商户身份和用户订单归属必须匹配。

## 运行时配置

以下 GitHub Secrets/Variables 需要在部署步骤注入运行中的容器。仓库中的 Secrets 不会自动进入已经运行的服务器容器：

- `WECHAT_PAY_APP_ID`
- `WECHAT_PAY_MCH_ID`
- `WECHAT_PAY_MERCHANT_SERIAL_NO`
- `WECHAT_PAY_API_V3_KEY`
- `WECHAT_PAY_NOTIFY_URL`（必须是 `https` 公网地址）
- `WECHAT_PAY_PUBLIC_KEY_ID`
- `WECHAT_PAY_MERCHANT_PRIVATE_KEY_PEM` 或 `WECHAT_PAY_MERCHANT_PRIVATE_KEY_PATH`
- `WECHAT_PAY_PUBLIC_KEY_PEM` 或 `WECHAT_PAY_PUBLIC_KEY_PATH`

PEM Secret 适合 GitHub Actions 或平台环境变量直接注入；文件路径方式适合只读 Secret 卷。应用两种方式都支持，且不记录密钥内容。

## 验证

- Go：`go test ./model ./controller ./setting`
- 前端：`tsgo -b`、`oxlint`、`rsbuild build`
- 支付回调必须在微信商户平台配置为 `WECHAT_PAY_NOTIFY_URL` 的精确地址。
