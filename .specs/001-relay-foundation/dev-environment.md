# Dev Environment

## 目标

仓库保留两种互不替代的开发方式：

1. **标准本地模式**：所有贡献者均可使用。PostgreSQL 与 Redis 在本地 Docker 中运行，Go 后端与 `web/default` 前端可在宿主机运行并保留热更新；也可以用现有 `Dockerfile.dev` 启动完整本地 API dev stack。
2. **Personal dev 模式**：仅供拥有个人云端开发 VPS 的开发者选择。Go API、PostgreSQL 和 Valkey 在个人 VPS 的隔离 Compose project 中运行，本地 Docker 只运行 default 前端，通过 Tailscale SSH 同步源码和转发 API。

两种模式都不用于 staging、production 或对外售卖。Personal dev 是附加能力，不改变或废弃任何现有本地 Compose、Make target 和微信支付本地 override。

## 标准本地模式

### 一次性初始化

```bash
make dev-bootstrap
make dev-infra-up
```

`dev-bootstrap` 会生成本地 `.env.dev`，构建一次 classic/default 前端产物，并保留 default 前端的开发依赖。由于两个 workspace 依赖不同的 `date-fns` 主版本，classic 与 default 必须按该命令隔离安装和构建。

### 日常启动

混合模式分别打开两个终端：

```bash
make dev-backend
```

```bash
make dev-frontend
```

默认地址：

- 后端与内嵌页面：`http://127.0.0.1:3000`
- default 前端开发服务器：`http://127.0.0.1:3001`
- PostgreSQL：`127.0.0.1:54329`
- Redis：`127.0.0.1:63799`

前端开发服务器将 `/api`、`/mj` 和 `/pg` 代理到 `VITE_REACT_APP_SERVER_URL`。

PostgreSQL、Redis、容器后端和前端端口均绑定 `127.0.0.1`。上游 Go 服务在宿主机运行时默认监听所有网卡，因此混合模式只适合受信任的本地网络；需要严格回环隔离时使用容器后端模式。

需要运行容器化后端时：

```bash
make dev-api-rebuild
make dev-frontend
```

Go 代码未变化时可用 `make dev-api` 直接复用本地 dev 镜像。

### 本地完整镜像验证

需要从当前检出源码构建包含前后端产物的完整镜像时，必须同时加载基础 Compose 文件和本地构建 override；`docker-compose.local.yml` 不是独立的 Compose 项目：

```bash
docker compose -f docker-compose.yml -f docker-compose.local.yml config
docker compose -f docker-compose.yml -f docker-compose.local.yml up -d --build
```

默认路径不读取任何支付 Secret，可在 Windows、macOS 和 Linux 上使用。它会复用 `docker-compose.yml` 中的端口、PostgreSQL、Redis、网络和持久化卷。不要与 `docker-compose.dev.yml` 管理的开发容器同时启动，避免容器名和端口冲突。

需要联调微信支付时，再显式加载可选 override。`WECHAT_PAY_ENV_FILE` 必须指向支付环境变量文件，`WECHAT_PAY_SECRET_DIR` 必须指向包含商户私钥和微信支付公钥的目录；两者都应使用当前宿主机可解析的绝对路径：

```bash
export WECHAT_PAY_ENV_FILE=/absolute/path/to/wechatpay.env
export WECHAT_PAY_SECRET_DIR=/absolute/path/to/wechatpay-secrets

docker compose \
  -f docker-compose.yml \
  -f docker-compose.local.yml \
  -f docker-compose.wechat-pay.local.yml \
  config

docker compose \
  -f docker-compose.yml \
  -f docker-compose.local.yml \
  -f docker-compose.wechat-pay.local.yml \
  up -d --build
```

PowerShell 使用 `$env:WECHAT_PAY_ENV_FILE` 和 `$env:WECHAT_PAY_SECRET_DIR` 设置相同变量。支付 override 未被加载时无需配置这两个变量；加载后若变量缺失，Compose 会在启动前给出明确错误。

### 常用命令

```bash
make dev-infra-status
make dev-down
```

需要删除本地开发数据库与 Redis 数据时才执行：

```bash
make dev-reset
```

### 安全边界

- `.env.dev` 被 Git 忽略；提交的是 `.env.dev.example`。
- 示例密码只允许本机开发，不得复制到云端环境。
- production 必须使用独立 Secret、托管数据库、TLS、备份、监控和发布流水线；本项目暂不维护长期 staging 环境。
- 首次 production 发布只允许内部账户与 BlackRain Cloud 测试租户小流量验证，通过计费、限流、流式和对账 smoke 后再开放销售。

## Personal dev 模式（可选）

Personal dev 的实现位于 [`deploy/personal-dev/`](../../deploy/personal-dev/)，完整本地模式仍使用仓库根目录原有 Compose 文件。初始化并启动：

```bash
make personal-dev-init
make personal-dev-up
```

默认职责边界：

- 本地 OrbStack 只运行 `meimei-api-personal-web`，默认地址 `http://127.0.0.1:3002`。
- 本地源码仍是唯一编辑源；前端通过 bind mount 热更新。
- 当前 worktree 通过 Tailscale SSH 增量同步到 `/opt/dev/projects/meimei-api/source`。
- VPS 使用 `meimei-api-fpsmeimei` Compose project 独立运行 API、PostgreSQL 和 Valkey。
- 远端 API 只绑定 `127.0.0.1:3100`，本地通过 `127.0.0.1:3310` SSH tunnel 访问。
- runtime env 保存在本地 `~/.config/goodbyeri/personal-dev/meimei-api.env` 和远端项目 runtime 目录，不进入仓库。

Go 后端变化后运行 `make personal-dev-rebuild`；前端变化由本地开发服务器直接热更新。`make personal-dev-down` 只停止 personal dev containers 和 tunnel，不删除 volumes，也不会操作标准本地开发 stack。

Personal dev 与同一 VPS 上的其他项目必须使用不同的目录、Compose project、network、volumes、数据库、缓存、端口和 Secret。个人 VPS 公网 SSH 必须由云 Firewall 阻断，管理连接只通过 Tailscale；PostgreSQL、Valkey 和开发 API 均不得直接暴露到公网。
