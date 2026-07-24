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

生产发布由 `.github/workflows/deploy-production.yml` 在 `production` Environment 中读取上述配置：

- 单行标识和 URL 使用 GitHub Variables，API v3 密钥与 PEM 使用 GitHub Secrets。
- Runner 先验证变量、32 字符 API v3 密钥和 PEM 格式，再通过临时目录上传到生产机。
- 生产机将 PEM 安装到 `/etc/meimei-api/secrets/wechatpay`，容器只读挂载到 `/run/secrets/wechatpay`；`production.env` 只保存容器内路径和必要变量。
- 安装脚本保留 `production.env` 中所有非 `WECHAT_PAY_*` 配置，并替换旧微信配置；日志不得输出密钥或 PEM 内容。

## 客户展示规则

- 钱包、套餐购买和计费详情中的额度金额统一显示为人民币符号 `¥`，数值按 `1:1` 展示。
- 生产后台 `USDExchangeRate` 设置为 `1`；该运行时选项保存在生产数据库中，不由镜像或部署脚本重置。
- 客户套餐卡片和购买弹窗不显示“有效期：1 个月”，但套餐持续时间和到期结算逻辑保持不变。
- 客户计费详情只查询消费日志（`type=2`）；登录和管理日志仍写入数据库并保留审计用途。

## 验证

- Go：`go test ./model ./controller ./setting`
- 前端：`tsgo -b`、变更文件 `oxlint`、受保护头格式检查、`rsbuild build`
- 部署：`bash -n deploy/production/install-wechat-pay.sh`、`bash -n deploy/production/deploy.sh`、production workflow 检查
- 支付回调必须在微信商户平台配置为 `WECHAT_PAY_NOTIFY_URL` 的精确地址。
