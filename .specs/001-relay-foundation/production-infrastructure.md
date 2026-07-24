# 莓莓 API 生产基础设施

## 状态

2026-07-23 已在 DigitalOcean 和 Cloudflare 建立 MeiMei API 的第一版生产基础设施。资源只属于 `meimei-api`，与 `2049-agent` 没有资源、数据库、缓存、网络或部署依赖关系。

2026-07-24 已通过 GitHub Actions 首次成功部署 MeiMei API。部署 Run 为 [`30036622257`](https://github.com/goodbyeri-studio/meimei-api/actions/runs/30036622257)，目标 commit 为 `5575b8fa7b590a0b3e14202b5c6730ca206d5a0e`；流水线使用不可变镜像 digest 发布并完成公开 readiness 验证。`meimeiapi.com` 与 `api.meimeiapi.com` 的主页、`/healthz/live`、`/healthz/ready` 和 `/api/status` 均返回 HTTP 200。

当前是“生产应用已上线、真实付费流量尚待运营验收”状态。正式 Firewall 已恢复为 `succeeded`，Droplet 运行正常，PostgreSQL 和 Valkey 均为 `online`。模型渠道、微信支付生产凭据实测、备份恢复演练、监控告警和 100/250/500 流式并发压测仍需在真实运营前完成。

## 命名和所有权

- 公开产品名称：`莓莓 API`
- 官网：`https://meimeiapi.com`
- 推荐 API endpoint：`https://api.meimeiapi.com`
- DigitalOcean Project：`MeiMei API`
- Project description：`MeiMei API / meimei-api`
- 所有资源使用 `meimei-api-*` 前缀，不使用泛化的 `blackrain-prod-*`。
- `meimei-api` 是 MeiMei API 的公开仓库和基础设施 slug。
- `blackrain-cloud`、`blackrain-desktop` 是独立项目和 MeiMei API 的外部客户，不属于本项目的品牌边界。
- `2049-agent` 是另一个完全独立的项目，不得复用其 Droplet、数据库、Valkey、Spaces、Secrets 或部署节点。

## 已创建资源

| 类型 | 配置 | 用途 |
|---|---|---|
| VPC | `meimei-api-prod-sgp1` / `sgp1` | MeiMei API 独立私有网络；不复用其他项目网络 |
| App | `meimei-api-prod-app-sgp1` / `s-4vcpu-8gb` / 80 GiB / Ubuntu 24.04 | API、控制台和 MeiMei API 数据面 |
| Reserved IP | `sgp1` | 绑定 App，作为 DNS 和故障重建时的稳定入口 |
| PostgreSQL | `db-s-2vcpu-4gb` / PostgreSQL 15 / 1 node / 60 GiB | `meimei_api` 数据库和业务账务 |
| Valkey | `db-s-1vcpu-2gb` / Valkey 8 / 1 node | 缓存、限流、会话和分布式状态 |
| Firewall | `meimei-api-prod-fw` | SSH 仅允许管理出口；公网只开放 HTTP/HTTPS |
| Container Registry | `meimei-api` / Basic | 存储固定 Git SHA 的生产镜像 |
| DNS | `meimeiapi.com` A | 官网，指向 Reserved IP，保持 DNS-only |
| DNS | `api.meimeiapi.com` A | 推荐 API endpoint，指向 Reserved IP，保持 DNS-only |
| DNS | `www.meimeiapi.com` CNAME | 指向 `meimeiapi.com`，由 Caddy 永久重定向到官网 |

数据库和 Valkey 均使用 MeiMei API VPC 的私网 TLS endpoint。PostgreSQL 已建立 `meimei_api` 和普通用户 `meimei_api_app`；密码不记录在仓库或本文档中。具体资源 ID 只保存在私有运维清单，不写入公开仓库。

本次命名迁移发生在应用发布前。迁移前已确认旧 PostgreSQL 业务表数为 `0`、旧 Valkey key 数为 `0`，随后以 `meimei-api-*` 名称重建两个托管集群并删除旧集群；新 PostgreSQL 和 Valkey 复核结果同样为空。Reserved IP 和 Droplet 保留，Project、VPC、Droplet、Firewall、Registry、tag、hostname、数据库、监控告警、DNS 和 Caddy 日志名称均已迁移到 `MeiMei API` / `meimei-api` 命名。

首期不使用 Load Balancer。Caddy 使用 [`deploy/production/Caddyfile`](../../deploy/production/Caddyfile) 在 App 上终止 HTTPS 并反向代理到 `127.0.0.1:3000`；`meimeiapi.com`、`api.meimeiapi.com` 提供同一服务，`www.meimeiapi.com` 永久重定向到官网。Docker 使用 [`deploy/production/docker-daemon.json`](../../deploy/production/docker-daemon.json) 开启 `live-restore` 和日志轮转。Caddy Admin API 仅监听 `127.0.0.1:2019`，用于校验后的平滑 reload，不对公网开放。边缘压缩暂不启用，直到 SSE 回归验证证明不会缓冲 token。所有入口保持 DNS-only，避免 Cloudflare proxy 给长时间 AI 请求引入额外超时。未来引入第二 App 时再创建 Load Balancer，并先补齐真正的 readiness endpoint。

Firewall 当前状态为 `succeeded`，只允许管理出口访问 SSH `22`，公网开放 `80/443`，不开放 App `3000`。管理出口变化后必须更新 Firewall，不能为了临时登录重新开放全网 SSH。

## 首月容量方案

目标是首月约 500 人同时在线。在 DeepKey 上游正常的前提下，MeiMei API 必须稳定承载 500 个在线连接，并通过 100、250、500 个并发流式请求的分级压测验证认证、路由、流转、计费和日志不会成为瓶颈。DeepKey 的模型可用性和生成速度不计入 MeiMei API 容量结论。

首期采用单台 `s-4vcpu-8gb`，以垂直容量换取更简单的发布、后台任务、日志和连接池管理。PostgreSQL、Valkey 先采用单节点，牺牲自动故障切换来降低首月固定成本；不为“看起来更标准”提前维护双 App。

预计固定成本约为 `$143/月`，不含额外流量、超额存储或后续备份 bucket：

- App Droplet：约 `$48/月`
- PostgreSQL：约 `$60/月`
- Valkey：约 `$30/月`
- Container Registry：约 `$5/月`

500 个在线用户不等于 500 个数据库连接。单 App 先使用 `SQL_MAX_OPEN_CONNS=40`、`SQL_MAX_IDLE_CONNS=10`、`SQL_MAX_LIFETIME=300`，通过真实流式压测再调整，不能沿用默认的 1000 个数据库连接上限。

首期设置 `BATCH_UPDATE_ENABLED=false`，优先保证计费和使用量及时落库；只有数据库写入指标证明需要时才启用进程内批量更新。`SESSION_SECRET`、`CRYPTO_SECRET` 必须显式配置为不同的 production 随机值，`SESSION_COOKIE_SECURE=true`、`SESSION_COOKIE_TRUSTED_URL=https://meimeiapi.com,https://api.meimeiapi.com`，并设置 `NODE_NAME=meimei-api-prod-app-sgp1`、`NODE_TYPE=master`。部署前还需把 New API 的系统名称配置为“莓莓 API”，避免控制台继续显示技术标识。

全局 API 限流当前按 `ClientIP` 计数，默认 `360` 次/`180` 秒。上线压测必须覆盖多用户共享 NAT/企业出口的情况，并按真实客户端 IP、用户和 Token 的组合策略校准，避免 MeiMei API 自己产生误限流。

## 环境和发布路径

- `dev`：本地 Docker Compose，使用仅限开发的 PostgreSQL、Valkey 和 Secret。
- CI：临时 PostgreSQL/Valkey，执行构建、迁移和定向回归测试。
- `prod`：唯一长期云环境，使用固定 Git SHA 镜像和可回滚部署。
- 不维护长期 staging，不做常态灰度。支付、计费、权限、migration、路由和缓存等高风险变更在 CI 完成定向验证后低峰发布，并在发布前确认备份和回滚点。

### GitHub Actions 发布流水线

`.github/workflows/ci.yml` 负责 PR 和 `main` 的 Go、default 前端、按变更触发的 classic 前端、受影响的生产镜像构建和上游署名检查。它只使用只读权限，不读取生产 Secret；GitHub Free 不配置 required checks，失败结果作为团队合并依据而不是强制规则。

`.github/workflows/deploy-production.yml` 只接受从 `main` 手动触发的 40 位 commit SHA，并要求输入 `DEPLOY_PRODUCTION`。部署任务会重新运行 Go 和 default 前端验证，构建单架构 `linux/amd64` 镜像并推送到 MeiMei API Registry，生产机只使用镜像 digest。GitHub Runner 的 SSH 访问通过一次性 DigitalOcean Firewall 规则临时开放，任务结束时删除；不向公网永久开放 SSH，也不使用 self-hosted runner。

生产机的 `/etc/meimei-api/production.env` 不进入仓库或 Actions 日志。微信支付 Variables/Secrets 由生产 workflow 校验后安装到该环境文件和 `/etc/meimei-api/secrets/wechatpay`，PEM 通过只读卷挂载，流水线不输出密钥内容。`deploy/production/deploy.sh` 在拉取候选 digest 后才切换 `current.env`，部署健康检查失败时恢复 `previous.env`；`rollback` 可以直接复用上一版本，不重新构建镜像。GitHub `production` Environment 的部署凭据、服务器 deploy 用户和 Registry 只读凭据已经配置并完成首次发布验证；数据库备份恢复流程仍需演练。

`security-audit.yml` 每周运行 Go `govulncheck` 和 Bun 依赖审计，当前只作为告警，不阻塞 PR 或发布。

## 当前验收状态

- [x] 构建并推送固定 Git SHA 镜像，生产使用不可变镜像 digest，禁止使用 `latest`。
- [x] 在单台 App 安装 Docker、Caddy，并部署固定 SHA 镜像。
- [x] 配置 GitHub `production` Environment、服务器 deploy 用户、Registry 凭据和应用运行所需的 production Secret。
- [x] 通过 migration、健康检查和可回滚发布流程完成首次部署。
- [x] 配置 Caddy HTTPS；两个生产域名的主页和基本 API 均可访问。
- [x] 新增无需认证的 `/healthz/live` 和 `/healthz/ready` 并完成公开验证；`/api/status` 仍只作为控制台配置接口，不作为 readiness。
- [ ] 配置并实测模型渠道、微信支付生产凭据和正式 webhook 回调；验证 SSE、支付回调和长请求不会被入口层提前中断。
- [ ] 创建 HTTPS、进程、PostgreSQL 和 Valkey Uptime/Alert Check。
- [ ] 使用本地可控的模拟流式上游完成 100/250/500 并发压测，观察 App CPU/内存、PostgreSQL 连接与 CPU、Valkey 内存、P95 延迟和中断率；再使用 DeepKey 做小规模真实端到端 smoke，避免上游波动污染 MeiMei API 容量结论。
- [ ] 完成 PostgreSQL restore、迁移回滚、支付回调、计费、限流、流式和 usage 对账 smoke。
- [ ] 真实付费流量进入前，确认 PostgreSQL 自动备份/PITR 可用，完成 restore 演练并为数据库、Valkey、进程和 HTTPS 建立告警。

容量验收要求：MeiMei API 自身 5xx 低于 `0.1%`，MeiMei API 增加的 p95 延迟低于 `100ms`，App CPU 持续低于 `70%`、内存低于 `75%`，PostgreSQL 连接数低于上限的 `70%`，且无 OOM、连接泄漏、流式中断或计费记录丢失。

## 升级触发条件

满足以下任一条件时，再增加第二 App 和 Load Balancer：

- 500 并发流式压测无法满足容量验收要求。
- App CPU 持续超过 70%，内存持续超过 75%，且垂直扩容不再具备成本优势。
- 需要无中断滚动发布、应用自动故障切换或明确生产 SLA。

双 App 上线前必须为每个节点设置稳定且唯一的 `NODE_NAME`，共享固定 Secret，限制每节点连接池，接入集中日志，并把订阅重置、Codex 凭据刷新等所有 master 后台任务纳入数据库租约或明确的单主提升流程。

满足以下任一条件时，再将 PostgreSQL 或 Valkey调整为高可用节点：

- 需要正式的数据库自动故障切换或生产 SLA。
- 支付、账户或账务数据不能接受单节点维护窗口。
- PostgreSQL CPU 持续超过 60%，或连接池长期接近上限。
- Valkey 内存长期超过 70%，或出现淘汰、超时和连接错误。
- 压测证明单节点已经影响 P95 延迟或错误率。

## 审计命令

以下命令用于复核资源，不包含任何 Secret：

```bash
doctl projects resources list "$MEIMEI_API_PROJECT_ID"
doctl vpcs get "$MEIMEI_API_VPC_ID"
doctl databases list --output json | jq '[.[] | {id, name, engine, version, num_nodes, size, region, status, project_id}]'
doctl compute droplet get "$MEIMEI_API_DROPLET_ID"
doctl compute reserved-ip list
doctl compute firewall get "$MEIMEI_API_FIREWALL_ID"
```
