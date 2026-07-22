# Verification

## 验证矩阵

| 日期 | 范围 | 命令/方式 | 结果 | 备注 |
|---|---|---|---|---|
| 2026-07-12 | 上游锁定 | `git describe --tags --exact-match HEAD`; `git rev-parse HEAD` | 通过 | `v1.0.0-rc.21` / `bde9b2f44887d34ec54799ae191d50f97914359e` |
| 2026-07-12 | License/署名 | 静态核对 `LICENSE`、`NOTICE`、README Section 7 和 AGENTS protected metadata | 通过 | 只证明文件保留，不代表法律审查完成 |
| 2026-07-12 | BlackRain foundation | `git diff --cached --check`; spec 文件静态核对 | 通过 | 不代表服务可运行 |
| 2026-07-12 | 上游 Go tests | `go test ./...` | 通过 | default/classic 产物生成后根包与全部测试包通过 |
| 2026-07-12 | default 前端 | `npx --yes bun@1.3.14 run build:check` | 通过 | TypeScript 检查与 Rsbuild production build 通过 |
| 2026-07-12 | classic 前端 | classic filtered install + `npx --yes bun@1.3.14 run build` | 通过 | 必须与 default workspace 隔离安装，避免 `date-fns` peer hoist 冲突 |
| 2026-07-12 | production 镜像 | `docker build -t blackrain-relay:foundation-smoke .` | 通过 | image `sha256:5e8b175203a963ead7c8496eb45300d4cf48cb19979ac23e300d22b8a3a91a6a` |
| 2026-07-12 | SQLite smoke | 本地二进制启动、自动迁移、`GET /api/status` | 通过 | 生成 31 类业务表，首页资源返回 200 |
| 2026-07-12 | PostgreSQL 15 smoke | 容器数据库 + 本地镜像启动/重启 + `GET /api/status` | 通过 | 31 张 public 表；重启后健康 |
| 2026-07-12 | MySQL 8.0 smoke | 容器数据库 + 本地镜像启动/重启 + `GET /api/status` | 通过 | 31 张表；重启后健康 |
| 2026-07-12 | 本地 dev 环境 | `make dev-bootstrap`; `make dev-infra-up`; `make dev-backend`; `make dev-api-rebuild`; `make dev-frontend` | 通过 | 宿主机/容器后端均通过；PostgreSQL/Redis healthy，API 直连与 `3001` proxy 成功 |
| 2026-07-12 | default 页面 smoke | Browser：`/setup` 首屏、console、下一步交互 | 通过 | 检测 PostgreSQL，进入管理员账户步骤，无 console error/warn |
| 2026-07-13 | 直销 token 生命周期 | `go test ./controller -count=1` | 通过 | 覆盖签发、额度、过期、模型白名单、禁用和删除 |
| 2026-07-23 | DeepKey 渠道与目录审计 | 42 把 Key 的 `/v1/models`、公开定价目录、渠道模型和 `abilities` 交叉核对 | 通过 | 42/42 Key 成功；230 个模型全部定价；模型清单和能力不匹配均为 0 |
| 2026-07-23 | DeepKey 真实渠道调用 | Relay 全量渠道测试 + 失败分组多模型、双端点复测 | 部分通过 | 修正后 37 个分组已验证可调用；5 个分组仍由 DeepKey 返回 403/429/503/服务错误 |
| 2026-07-23 | 渠道测试回归 | `go test ./controller -count=1` | 通过 | 覆盖测试提示词，并验证完整 controller 测试包 |
| 2026-07-23 | 本地生产镜像 | `docker compose -p blackrain-relay --env-file .env.dev -f docker-compose.dev.yml -f docker-compose.local.yml up -d --build new-api` | 通过 | `http://127.0.0.1:3010/api/status` 返回成功 |
| YYYY-MM-DD | Cloud/Relay contract | token + usage integration tests | 未跑 | 尚无 BlackRain 实现 |
| YYYY-MM-DD | WORK/CODE E2E | 真实授权模型渠道 | 未跑 | 发布门槛 |

## 已验证

- New API release 源码、历史、AGPLv3、NOTICE 和上游开发规则已导入。
- `origin` 与 `upstream` 已分离，当前基线可精确定位。
- default/classic production build、完整 `go test ./...` 与 production Docker image build 已通过。
- SQLite、PostgreSQL 15、MySQL 8.0 均完成自动迁移和健康检查；PostgreSQL/MySQL 重启正常。
- 本地 dev 环境已使用 PostgreSQL、Redis、宿主机 Go 后端和 default 前端 dev server 跑通。

## 未验证风险

- 尚未测试最低支持版本 PostgreSQL 9.6 与 MySQL 5.7.8，也未执行 migration rollback/backup restore。
- 尚未配置任何生产数据库、Redis、域名、Secret、模型渠道、备份或监控。
- Cloud 企业客户、scoped token、usage 对账和 BlackRain 双引擎 E2E 尚未实现。
- AGPL、模型厂商转售条款、支付、税务、备案、内容安全和日志留存尚未正式审查。
