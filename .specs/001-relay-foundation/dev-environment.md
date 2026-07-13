# Dev Environment

## 目标

日常开发默认采用混合模式：PostgreSQL 与 Redis 在 Docker 中运行，Go 后端与 `web/default` 前端在宿主机运行并保留热更新。需要验证容器后端时，也可以使用现有 `Dockerfile.dev` 启动完整 API dev stack。该环境不用于 staging、production 或对外售卖。

## 一次性初始化

```bash
make dev-bootstrap
make dev-infra-up
```

`dev-bootstrap` 会生成本地 `.env.dev`，构建一次 classic/default 前端产物，并保留 default 前端的开发依赖。由于两个 workspace 依赖不同的 `date-fns` 主版本，classic 与 default 必须按该命令隔离安装和构建。

## 日常启动

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

## 常用命令

```bash
make dev-infra-status
make dev-down
```

需要删除本地开发数据库与 Redis 数据时才执行：

```bash
make dev-reset
```

## 安全边界

- `.env.dev` 被 Git 忽略；提交的是 `.env.dev.example`。
- 示例密码只允许本机开发，不得复制到云端环境。
- staging/production 必须使用独立 Secret、托管数据库、TLS、备份、监控和发布流水线。
