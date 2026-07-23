# MeiMei API 生产部署

本目录只承载 MeiMei API 单节点生产部署，不包含任何 production Secret。生产发布由 `.github/workflows/deploy-production.yml` 手动触发，应用镜像固定到 DigitalOcean Container Registry digest。

## 服务器前置条件

- Ubuntu 24.04、Docker Engine、Docker Compose plugin、Caddy 和 `curl` 已安装。
- 建立独立 `meimei-deploy` 用户；它需要执行 Docker Compose，但不允许密码登录。
- `/opt/meimei-api` 由 deploy 用户管理；`/var/lib/meimei-api/data` 和 `/var/log/meimei-api` 可由容器写入。
- `/etc/meimei-api/production.env` 由管理员创建，权限 `0600`，不通过 GitHub Actions 上传。
- deploy 用户的 Docker 配置只保存 Registry 只读凭据；GitHub Actions 的推送凭据不写入生产机。
- `deploy/production/Caddyfile` 和 `docker-daemon.json` 已经由管理员安装并验证。

`production.env` 至少包含：

```dotenv
SQL_DSN=postgresql://...
REDIS_CONN_STRING=rediss://...
SESSION_SECRET=...
CRYPTO_SECRET=...
SESSION_COOKIE_SECURE=true
SESSION_COOKIE_TRUSTED_URL=https://meimeiapi.com,https://api.meimeiapi.com
SQL_MAX_OPEN_CONNS=40
SQL_MAX_IDLE_CONNS=10
SQL_MAX_LIFETIME=300
BATCH_UPDATE_ENABLED=false
```

真实值不得提交到仓库、PR、Issue、Actions 日志或部署摘要。

## GitHub production Environment

Secrets：

- `DOCR_PUSH_TOKEN`：只允许向 MeiMei API Registry 推送镜像。
- `DO_FIREWALL_TOKEN`：只用于创建和删除一次性部署 Firewall。
- `PROD_SSH_PRIVATE_KEY`：deploy 用户的独立 SSH 私钥。
- `PROD_SSH_KNOWN_HOSTS`：预先核验的生产机 host key，不运行 `ssh-keyscan` 动态信任。

Variables：

- `PROD_HOST`：Reserved IP 或固定主机名。
- `PROD_DROPLET_ID`：DigitalOcean Droplet ID，只保存在 Environment Variable。
- `PROD_DEPLOY_USER`：建议为 `meimei-deploy`。
- `PROD_APP_ROOT`：固定为 `/opt/meimei-api`。

GitHub Free 不配置 required reviewer 或 required status check。团队约定 CI 通过后再合并，生产发布仍要求手动输入固定 SHA 和 `DEPLOY_PRODUCTION`。

## 发布和回滚

发布时在 GitHub Actions 选择 `Deploy MeiMei API production`：

1. `operation` 选择 `deploy`。
2. `target_sha` 填入 `main` 上的完整 40 位 SHA。
3. `confirm` 填入 `DEPLOY_PRODUCTION`。
4. `reason` 记录发布原因。

Workflow 会重新执行代码验证、构建并推送 digest 镜像、为当前 GitHub Runner 创建一次性 SSH Firewall、上传 Compose/脚本、检查内网和公网 readiness，然后删除临时 Firewall。

回滚时选择 `operation=rollback`，仍填写 `DEPLOY_PRODUCTION`。服务器只在 `.deploy/current.env` 和 `.deploy/previous.env` 保存非敏感的 SHA/digest；回滚不会重新构建镜像。

首次发布前必须完成数据库备份和恢复演练。高风险 migration、计费、支付、权限、缓存或路由变更应在低峰发布，并在发布前确认可用的数据库恢复点。
