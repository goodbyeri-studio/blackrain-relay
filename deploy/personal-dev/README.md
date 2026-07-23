# MeiMei API Personal Dev

本目录提供一套可选的个人 VPS 开发方式，不替代仓库根目录已有的完整本地 Docker Compose 和本地混合开发方式。没有个人 VPS 的贡献者继续使用 `make dev-*` 即可。

Personal dev 将职责拆分为：

- 本地 OrbStack：只运行 `docker-compose.web.yml` 中的 default 前端开发服务器。
- Tailscale SSH：同步源码、执行远端 Compose，并为 API 建立本地端口转发。
- 个人 VPS：独立运行 `docker-compose.yml` 中的 Go API、PostgreSQL 和 Valkey。

每个项目必须使用独立的远端目录、Compose project、Docker network、volumes、数据库、缓存和 runtime env。不得复用其他项目或 production 的数据与 Secret。

## 初始化和启动

```bash
make personal-dev-init
make personal-dev-up
```

`personal-dev-init` 只在文件不存在时创建：

- `.env.personal-dev`：SSH、端口和目录等非敏感元数据，已被 Git 忽略。
- `~/.config/goodbyeri/personal-dev/meimei-api.env`：随机生成的个人开发 Secret，权限为 `0600`。

`personal-dev-up` 会同步当前 worktree 到 VPS，构建并启动远端 backend stack，建立 SSH tunnel，再启动本地前端。默认访问地址为 `http://127.0.0.1:3002`。

Go 代码变化后运行：

```bash
make personal-dev-rebuild
```

前端源码通过本地 bind mount 热更新，无需同步到 VPS。常用检查命令：

```bash
make personal-dev-status
make personal-dev-doctor
make personal-dev-logs
make personal-dev-web-status
```

停止 personal dev 不会删除任何 volume：

```bash
make personal-dev-down
```

该模式要求 VPS 的 API 端口只绑定 `127.0.0.1`，并通过 Tailscale SSH tunnel 访问。VPS 公网 SSH 应由云 Firewall 阻断；开发数据库和 Valkey 不得发布宿主机端口。
