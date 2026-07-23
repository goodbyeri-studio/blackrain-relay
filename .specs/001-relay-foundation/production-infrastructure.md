# BlackRain Relay 生产基础设施

## 状态

2026-07-23 已在 DigitalOcean 和 Cloudflare 建立 Relay 的第一版生产基础设施。资源只属于 `blackrain-relay`，与 `2049-agent` 没有资源、数据库、缓存、网络或部署依赖关系。

当前是“基础设施已按单 App 方案收敛、应用尚未发布”状态：App Droplet 仍未运行 Relay，production Secret、Caddy/TLS、模型渠道和真实流量压测尚未完成。因此不能把当前资源状态描述为 Relay 已经上线。

## 命名和所有权

- DigitalOcean Project：`BlackRain Relay`
- Project description：`BlackRain / blackrain-relay`
- 所有资源使用 `blackrain-relay-*` 前缀，不使用泛化的 `blackrain-prod-*`。
- `BlackRain` 是品牌/产品集合；`blackrain-relay`、`blackrain-cloud`、`blackrain-desktop` 是独立子项目。
- `2049-agent` 是另一个完全独立的项目，不得复用其 Droplet、数据库、Valkey、Spaces、Secrets 或部署节点。

## 已创建资源

| 类型 | 配置 | 用途 |
|---|---|---|
| VPC | `blackrain-relay-prod-sgp1` / `10.200.0.0/20` / `sgp1` | Relay 私有网络；不使用 `2049-agent` 的 `10.104.0.0/20` |
| App | `blackrain-relay-prod-app-sgp1` / `s-4vcpu-8gb` / 80 GiB / Ubuntu 24.04 | API、控制台和 Relay 数据面 |
| Reserved IP | `146.190.193.211` / `sgp1` | 绑定 App，作为 DNS 和故障重建时的稳定入口 |
| PostgreSQL | `db-s-2vcpu-4gb` / PostgreSQL 15 / 1 node / 60 GiB | `blackrain_relay` 数据库和业务账务 |
| Valkey | `db-s-1vcpu-2gb` / Valkey 8 / 1 node | 缓存、限流、会话和分布式状态 |
| Firewall | `blackrain-relay-prod-fw` | SSH 仅允许管理出口；公网只开放 HTTP/HTTPS |
| Container Registry | `blackrain-relay` / Basic | 存储固定 Git SHA 的生产镜像 |
| DNS | `relay.goodbyeri.cc` A | 指向 Reserved IP，保持 DNS-only |

数据库和 Valkey 均使用 Relay VPC 的私网 TLS endpoint。PostgreSQL 已建立 `blackrain_relay` 和普通用户 `blackrain_relay_app`；密码不记录在仓库或本文档中。

首期不使用 Load Balancer。Caddy 在 App 上终止 HTTPS 并反向代理到 `127.0.0.1:3000`；`relay.goodbyeri.cc` 保持 DNS-only，避免 Cloudflare proxy 给长时间 AI 请求引入额外超时。未来引入第二 App 时再创建 Load Balancer，并先补齐真正的 readiness endpoint。

Firewall 当前只允许管理出口访问 SSH `22`，公网开放 `80/443`，不开放 App `3000`。管理出口变化后必须更新 Firewall，不能为了临时登录重新开放全网 SSH。

## 首月容量方案

目标是首月约 500 人同时在线。在 DeepKey 上游正常的前提下，Relay 必须稳定承载 500 个在线连接，并通过 100、250、500 个并发流式请求的分级压测验证认证、路由、流转、计费和日志不会成为瓶颈。DeepKey 的模型可用性和生成速度不计入 Relay 容量结论。

首期采用单台 `s-4vcpu-8gb`，以垂直容量换取更简单的发布、后台任务、日志和连接池管理。PostgreSQL、Valkey 先采用单节点，牺牲自动故障切换来降低首月固定成本；不为“看起来更标准”提前维护双 App。

预计固定成本约为 `$143/月`，不含额外流量、超额存储或后续备份 bucket：

- App Droplet：约 `$48/月`
- PostgreSQL：约 `$60/月`
- Valkey：约 `$30/月`
- Container Registry：约 `$5/月`

500 个在线用户不等于 500 个数据库连接。单 App 先使用 `SQL_MAX_OPEN_CONNS=40`、`SQL_MAX_IDLE_CONNS=10`、`SQL_MAX_LIFETIME=300`，通过真实流式压测再调整，不能沿用默认的 1000 个数据库连接上限。

首期设置 `BATCH_UPDATE_ENABLED=false`，优先保证计费和使用量及时落库；只有数据库写入指标证明需要时才启用进程内批量更新。`SESSION_SECRET`、`CRYPTO_SECRET` 必须显式配置为不同的 production 随机值，`NODE_NAME=blackrain-relay-prod-app-sgp1`、`NODE_TYPE=master`。

全局 API 限流当前按 `ClientIP` 计数，默认 `360` 次/`180` 秒。上线压测必须覆盖多用户共享 NAT/企业出口的情况，并按真实客户端 IP、用户和 Token 的组合策略校准，避免 Relay 自己产生误限流。

## 环境和发布路径

- `dev`：本地 Docker Compose，使用仅限开发的 PostgreSQL、Valkey 和 Secret。
- CI：临时 PostgreSQL/Valkey，执行构建、迁移和定向回归测试。
- `prod`：唯一长期云环境，使用固定 Git SHA 镜像和可回滚部署。
- 不维护长期 staging，不做常态灰度。支付、计费、权限、migration、路由和缓存等高风险变更在 CI 完成定向验证后低峰发布，并在发布前确认备份和回滚点。

## 上线前检查

- 构建并推送固定 Git SHA 镜像，禁止生产使用 `latest`。
- 在单台 App 安装 Docker、Caddy，并部署固定 SHA 镜像。
- 写入独立 production Secret：`SQL_DSN`、`REDIS_CONN_STRING`、`SESSION_SECRET`、`CRYPTO_SECRET`、模型渠道、支付和 webhook 凭据。
- 通过 migration 前备份、健康检查和可回滚发布流程。
- 配置 Caddy HTTPS，验证 SSE、支付回调和长请求不会被入口层提前中断。
- 新增无需认证的 `/healthz/live` 和 `/healthz/ready`；现有 `/api/status` 是控制台配置接口，不作为 readiness。完成前只使用进程和端口监控，不能把 `/api/status` 当作依赖健康证明。
- 部署后创建 HTTPS、进程、PostgreSQL 和 Valkey Uptime/Alert Check。
- 使用 500 活跃流式请求做压测，观察 App CPU/内存、PostgreSQL 连接与 CPU、Valkey 内存、P95 延迟和中断率。
- 完成 PostgreSQL restore、迁移回滚、支付回调、计费、限流、流式和 usage 对账 smoke。

容量验收要求：Relay 自身 5xx 低于 `0.1%`，Relay 增加的 p95 延迟低于 `100ms`，App CPU 持续低于 `70%`、内存低于 `75%`，PostgreSQL 连接数低于上限的 `70%`，且无 OOM、连接泄漏、流式中断或计费记录丢失。

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
doctl projects resources list 56be4c2c-33f7-4035-a228-46b5d4259d46
doctl vpcs get 6e3f3ff1-472e-4238-b48d-fc7c3a73a1ab
doctl databases list --output json | jq '[.[] | {id, name, engine, version, num_nodes, size, region, status, project_id}]'
doctl compute droplet get 586801498
doctl compute reserved-ip list
doctl compute firewall get cd9336cc-d295-40cb-bf87-5bfa47a674c7
```
