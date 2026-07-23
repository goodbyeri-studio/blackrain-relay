# BlackRain Relay 生产基础设施

## 状态

2026-07-23 已在 DigitalOcean 和 Cloudflare 建立 Relay 的第一版生产基础设施。资源只属于 `blackrain-relay`，与 `2049-agent` 没有资源、数据库、缓存、网络或部署依赖关系。

当前是“基础设施已就绪、应用尚未发布”状态：App Droplet 仍是干净的 Ubuntu 24.04 节点，Relay 镜像、production Secret、TLS 终止、模型渠道和真实流量压测尚未完成。因此不能把当前资源状态描述为 Relay 已经上线。

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
| App | 2 x `s-2vcpu-4gb` Ubuntu 24.04 | 无状态 API、控制台和 Relay 数据面 |
| PostgreSQL | `db-s-2vcpu-4gb` / PostgreSQL 15 / 1 node / 60 GiB | `blackrain_relay` 数据库和业务账务 |
| Valkey | `db-s-1vcpu-2gb` / Valkey 8 / 1 node | 缓存、限流、会话和分布式状态 |
| Load Balancer | `lb-small` / `sgp1` | 两台 App 节点的入口和健康检查 |
| Firewall | `blackrain-relay-prod-fw` | SSH 管理出口和私网 App 端口限制 |
| Container Registry | `blackrain-relay` / Basic | 存储固定 Git SHA 的生产镜像 |
| DNS | `relay.goodbyeri.cc` A/AAAA | 指向 Load Balancer，当前 DNS-only |

数据库和 Valkey 均使用 Relay VPC 的私网 TLS endpoint。PostgreSQL 已建立 `blackrain_relay` 和普通用户 `blackrain_relay_app`；密码不记录在仓库或本文档中。

Load Balancer 当前配置为 `HTTP:80 -> App:3000`，健康检查为 `/api/status`，HTTP idle timeout 为 600 秒。HTTPS `443` 监听和有效证书仍需在应用发布前完成。`relay.goodbyeri.cc` 当前为 DNS-only，因此 Cloudflare Zone 的 SSL Strict、Always HTTPS 和最低 TLS 1.2 不会保护该主机名；正式流量接入前必须由 Load Balancer 终止 TLS，并将 `80` 改为跳转 HTTPS 而不是继续转发到 App。后续若启用 Cloudflare proxy，需先验证 SSE、WebSocket 和长请求。

Firewall 当前只允许创建时的管理出口访问 SSH `22`，App `3000` 仅允许 `10.200.0.0/20`。管理出口变化后必须更新 Firewall，不能为了临时登录重新开放全网 SSH。

## 首月容量方案

目标是首月最多约 500 人同时使用，按“可能存在 500 个活跃流式请求”做保守估算。两台 App 保留横向分担和滚动发布能力；PostgreSQL、Valkey 先采用单节点，牺牲自动故障切换来降低首月固定成本。

预计固定成本约为 `$155/月`，不含额外流量、超额存储或后续备份 bucket：

- App Droplet：约 `$48/月`
- Load Balancer：约 `$12/月`
- PostgreSQL：约 `$60/月`
- Valkey：约 `$30/月`
- Container Registry：约 `$5/月`

500 个在线用户不等于 500 个数据库连接。发布前必须设置合理的连接池，例如每个 App 节点先从 `SQL_MAX_OPEN_CONNS=50`、`SQL_MAX_IDLE_CONNS=20` 开始，通过真实流式压测再调整，不能沿用默认的 1000 个数据库连接上限。

## 多节点运行约束

- 两台 App 使用相同的固定 Git SHA 镜像，但必须使用不同、稳定的 `NODE_NAME`。
- 只能有一个节点设置 `NODE_TYPE=master`；另一个节点必须设置 `NODE_TYPE=slave`。主节点负责数据库 migration、订阅重置、日志保留和其它后台任务，不允许两台节点同时以默认 master 身份启动。
- 首次部署和包含 migration 的升级必须先启动 master，确认 migration 完成且 `/api/status` 健康后再启动 slave。故障切换时先确认旧 master 已退出，再显式提升 slave，避免双主。
- 两台节点必须共享相同的 `SQL_DSN`、`REDIS_CONN_STRING`、`SESSION_SECRET` 和 `CRYPTO_SECRET`；节点角色和 `NODE_NAME` 单独配置。

## 上线前检查

- 构建并推送固定 Git SHA 镜像，禁止生产使用 `latest`。
- 在两台 App 节点安装运行时、部署相同镜像，并按“一个 master、一个 slave”配置 `NODE_TYPE` 和稳定的 `NODE_NAME`。
- 写入两台节点共享的 production Secret：`SQL_DSN`、`REDIS_CONN_STRING`、`SESSION_SECRET`、`CRYPTO_SECRET`、模型渠道、支付和 webhook 凭据。
- 设置 `SESSION_COOKIE_SECURE=true`、`SESSION_COOKIE_TRUSTED_URL=https://relay.goodbyeri.cc`、`SQL_MAX_OPEN_CONNS=50` 和 `SQL_MAX_IDLE_CONNS=20`。
- migration 前完成备份；先发布 master 并确认 migration 完成，再发布 slave，最后验证健康检查和回滚流程。
- 为 Load Balancer 增加 HTTPS `443` 和有效证书，将 HTTP `80` 配置为跳转 HTTPS，并验证 SSE、WebSocket、支付回调和长请求。
- 部署后创建 `https://relay.goodbyeri.cc/api/status` Uptime Check。
- 使用 500 活跃流式请求做压测，观察 App CPU/内存、PostgreSQL 连接与 CPU、Valkey 内存、P95 延迟和中断率。
- 完成 PostgreSQL restore、迁移回滚、支付回调、计费、限流、流式和 usage 对账 smoke。

## 升级触发条件

满足以下任一条件时，再将 PostgreSQL 或 Valkey 调整为双节点：

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
doctl databases list
doctl compute load-balancer get 652fa540-ae89-4e87-a8b5-a58306dacfc8
doctl compute firewall get cd9336cc-d295-40cb-bf87-5bfa47a674c7
```
