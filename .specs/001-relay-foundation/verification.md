# Verification

## 验证矩阵

| 日期 | 范围 | 命令/方式 | 结果 | 备注 |
|---|---|---|---|---|
| 2026-07-12 | 上游锁定 | `git describe --tags --exact-match HEAD`; `git rev-parse HEAD` | 通过 | `v1.0.0-rc.21` / `bde9b2f44887d34ec54799ae191d50f97914359e` |
| 2026-07-12 | License/署名 | 静态核对 `LICENSE`、`NOTICE`、README Section 7 和 AGENTS protected metadata | 通过 | 只证明文件保留，不代表法律审查完成 |
| 2026-07-12 | foundation | `git diff --cached --check`; spec 文件静态核对 | 通过 | 不代表服务可运行 |
| 2026-07-12 | 上游 Go tests | `go test ./...` | 通过 | default/classic 产物生成后根包与全部测试包通过 |
| 2026-07-12 | default 前端 | `npx --yes bun@1.3.14 run build:check` | 通过 | TypeScript 检查与 Rsbuild production build 通过 |
| 2026-07-12 | classic 前端 | classic filtered install + `npx --yes bun@1.3.14 run build` | 通过 | 必须与 default workspace 隔离安装，避免 `date-fns` peer hoist 冲突 |
| 2026-07-12 | production 镜像 | `docker build` | 通过 | image `sha256:5e8b175203a963ead7c8496eb45300d4cf48cb19979ac23e300d22b8a3a91a6a`；当时使用的旧内部标签已淘汰 |
| 2026-07-12 | SQLite smoke | 本地二进制启动、自动迁移、`GET /api/status` | 通过 | 生成 31 类业务表，首页资源返回 200 |
| 2026-07-12 | PostgreSQL 15 smoke | 容器数据库 + 本地镜像启动/重启 + `GET /api/status` | 通过 | 31 张 public 表；重启后健康 |
| 2026-07-12 | MySQL 8.0 smoke | 容器数据库 + 本地镜像启动/重启 + `GET /api/status` | 通过 | 31 张表；重启后健康 |
| 2026-07-12 | 本地 dev 环境 | `make dev-bootstrap`; `make dev-infra-up`; `make dev-backend`; `make dev-api-rebuild`; `make dev-frontend` | 通过 | 宿主机/容器后端均通过；PostgreSQL/Redis healthy，API 直连与 `3001` proxy 成功 |
| 2026-07-12 | default 页面 smoke | Browser：`/setup` 首屏、console、下一步交互 | 通过 | 检测 PostgreSQL，进入管理员账户步骤，无 console error/warn |
| 2026-07-13 | 直销 token 生命周期 | `go test ./controller -count=1` | 通过 | 覆盖签发、额度、过期、模型白名单、禁用和删除 |
| 2026-07-23 | Relay production infrastructure | `doctl` Project、Droplet、Reserved IP、VPC、database、firewall 只读检查 | 通过 | `MeiMei API` Project；独立 VPC；单台 4 vCPU/8 GB App + Reserved IP；PostgreSQL/Valkey 单节点；无 Load Balancer；未记录 Secret 或资源 ID |
| 2026-07-23 | MeiMei API production edge | App 安装 Docker/Caddy；`caddy validate`；Caddy systemd；Let's Encrypt certificate；Cloudflare DNS API | 通过 | `meimeiapi.com`、`api.meimeiapi.com` A 指向 Reserved IP；Caddy 监听 `80/443`；Admin API 仅监听 localhost；边缘压缩待 SSE 回归后再启用；MeiMei API 镜像尚未部署，所以后端请求当前返回 `502` |
| 2026-07-23 | MeiMei API 域名 | Spaceship nameserver；Cloudflare Zone/DNS API；Cloudflare DoH；Let's Encrypt certificate；Caddy route | 通过 | Zone `active`；`meimeiapi.com`、`api.meimeiapi.com` 指向 Reserved IP；`www.meimeiapi.com` CNAME 到根域名并永久重定向；新域名证书已签发；旧域名已从入口和 DNS 清单移除 |
| 2026-07-23 | 独立命名迁移 | GitHub API；`doctl` Project/VPC/Droplet/database/firewall/registry/monitoring；PostgreSQL/Valkey 空数据检查；Cloudflare DNS API；Caddy 配置与 hostname/tag 检查 | 通过 | 仓库和运行资源统一为 `MeiMei API` / `meimei-api`；旧 PostgreSQL 业务表和 Valkey key 均为 `0` 后删除；Reserved IP 和 Droplet 保留；应用仍未部署 |
| 2026-07-23 | 支付事务 PostgreSQL 15 | `PAYMENT_TEST_DB=postgres PAYMENT_TEST_DSN=... go test ./model -run 'TestCompleteWechatPayTopUp|TestCreditUserTopUpQuota' -count=1` | 通过 | 独立 `payment_test` 数据库，容器端口 54329 |
| 2026-07-23 | 支付事务 MySQL 8.0 | `PAYMENT_TEST_DB=mysql PAYMENT_TEST_DSN=... go test ./model -run 'TestCompleteWechatPayTopUp|TestCreditUserTopUpQuota' -count=1` | 通过 | 临时 MySQL 8 容器，端口 33306；测试后销毁 |
| 2026-07-23 | 支付事务 SQLite | `go test ./model -run 'TestCompleteWechatPayTopUp|TestCreditUserTopUp' -count=1` | 通过 | 默认内存 SQLite |
| 2026-07-23 | 支付三数据库事务 | `PAYMENT_TEST_DIALECT=... PAYMENT_TEST_DSN=... go test ./model -run TestPaymentTransactionDatabaseMatrix -count=1` | 自动化矩阵 | GitHub Actions 覆盖 SQLite、MySQL 8.0、PostgreSQL 15；保护并发通知、额度上限、邀请奖励和状态一致性 |
| 2026-07-23 | API 密钥名称唯一性 | `go test ./model ./controller ./relay/helper -count=1`；GitHub 数据库矩阵 | 通过 | 覆盖 SQLite、MySQL 5.7.44、PostgreSQL 9.6；数据库唯一索引阻止并发重名，软删除后允许复用名称 |
| 2026-07-23 | default 用量日志界面 | `tsgo -b`; `npx tsx --tsconfig tsconfig.app.json --test ...`; `rsbuild build` | 通过 | 覆盖普通用户/管理员列、实际 quota、订阅扣费标记与移动端字段恢复；4 个回归场景通过 |
| 2026-07-23 | 本地完整镜像 Compose | Windows Docker Compose 与 Linux `docker:27-cli` 执行基础/微信支付 override 的 `config --quiet` | 通过 | 基础组合无需支付 Secret；支付组合显式要求 `WECHAT_PAY_ENV_FILE` 和 `WECHAT_PAY_SECRET_DIR` |
| 2026-07-23 | 本地完整镜像构建 | `docker compose -f docker-compose.yml -f docker-compose.local.yml build new-api` | 通过 | default/classic 前端产物与 Go 后端均构建完成，生成 `meimei-api:local` |
| 2026-07-24 | CI 与生产发布 workflow | `actionlint`; `bash -n`; production Compose `config --quiet`; `go test ./...`; default/classic build | 通过 | Action 固定到 commit SHA；PR CI 不读取 Secret；production 仅手动固定 SHA/digest；安全审计为非阻塞告警 |
| 2026-07-24 | 生产镜像与健康端点 | `docker buildx build --platform linux/amd64 --load`; 临时容器请求 `/healthz/live`、`/healthz/ready` | 通过 | 镜像 `sha256:4c1bdc9b027e96a6997d4e6d5780e4bc2590b1c8047186d27d6df305e8453fb3`；SQLite/Redis-disabled 验证环境中两个端点均返回 `200` |
| 2026-07-24 | 渠道分组倍率生产加固 | `go test ./model ./controller -count=1`；`bun test src/features/channels/lib/group-ratios.test.ts`；`bun run typecheck`；相关文件 `oxlint`、`oxfmt --check`；`bun run build`；`git diff --check` | 通过 | 单分组倍率使用事务和行锁合并更新，避免并发管理员以旧整表覆盖；配置读取失败或结构异常时禁止前端编辑；default production build 通过 |
| 2026-07-24 | Personal dev 隔离环境 | `make personal-dev-up`；`make personal-dev-doctor`；远端 Compose/health 检查；本地前端 proxy smoke；DigitalOcean Firewall 与 SSH 路径检查 | 通过 | 标准本地 Compose 未改变；本地仅运行 default 前端；个人 VPS 独立运行 API/PostgreSQL/Valkey；与 `2049-agent` 的目录、Compose project、network、volume、端口和 Secret 隔离；公网 SSH 阻断，Tailscale SSH 正常 |
| 2026-07-24 | 微信支付生产注入与人民币计费展示 | `bash -n`（安装/部署脚本）；`tsgo -b`；`npx tsx --test` 用量金额回归；变更文件 `oxlint`/`oxfmt --check`；`rsbuild build`；生产后台页面回读 | 通过 | workflow 注入微信 Variables/Secrets 并只读挂载 PEM；人民币金额回归 2/2；钱包/计费金额按 `¥` 1:1 展示；生产 `USDExchangeRate=1`；完整 workflow 仍需 PR CI 验证，真实微信预下单需部署后验证 |
| YYYY-MM-DD | Cloud/Relay contract | token + usage integration tests | 未跑 | 尚无企业合同实现 |
| YYYY-MM-DD | WORK/CODE E2E | 真实授权模型渠道 | 未跑 | 发布门槛 |

## 已验证

- New API release 源码、历史、AGPLv3、NOTICE 和上游开发规则已导入。
- `origin` 与 `upstream` 已分离，当前基线可精确定位。
- default/classic production build、完整 `go test ./...` 与 production Docker image build 已通过。
- SQLite、PostgreSQL 15、MySQL 8.0 均完成自动迁移和健康检查；PostgreSQL/MySQL 重启正常。
- 本地 dev 环境已使用 PostgreSQL、Redis、宿主机 Go 后端和 default 前端 dev server 跑通。

## API 密钥名称唯一约束

- 同一用户的有效 API 密钥名称按去除首尾空格后的 Unicode 字符串唯一；名称限制为最多 50 个字符，不同用户可以使用相同名称。
- 数据库使用 `name_fingerprint` 保存带命名空间的 SHA-256 指纹，并通过 `(user_id, name_fingerprint)` 唯一索引防止并发创建或改名绕过 controller 检查。
- 软删除记录改用按 Token ID 生成的墓碑指纹，因此原名称可以复用。
- 启动迁移按用户分批整理空名称、首尾空格、重复名称和超长名称；生产滚动发布必须先完成迁移并确认旧实例不再写入 Token 表。

## 未验证风险

- 尚未执行名称唯一性 migration rollback/backup restore。
- 生产基础设施已按单 App 方案收敛，DNS、Caddy/TLS 和 Docker 基础运行时已完成；尚未完成 production Secret、固定 SHA 镜像、模型渠道、应用部署、独立健康检查、SSE 压缩回归、restore/PITR 演练和真实流量压测。
- Cloud 企业客户、scoped token、usage 对账和 BlackRain 双引擎 E2E 尚未实现。
- AGPL、模型厂商转售条款、支付、税务、备案、内容安全和日志留存尚未正式审查。
