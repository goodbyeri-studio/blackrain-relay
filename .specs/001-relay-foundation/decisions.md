# Decisions

## 2026-07-12：Relay 基于锁定 New API release

- 决策：初始 `main` 基于 New API `v1.0.0-rc.21`，commit `bde9b2f44887d34ec54799ae191d50f97914359e`；保留 `upstream` remote，只按评估过的 tag/commit 升级。
- 原因：直接获得渠道、协议、token、计量和运营能力，同时避免跟随每日 HEAD 导致不可复现发布。
- 替代方案：只引用官方 Docker 镜像；从零自研；跟随上游 main。
- 后续复查条件：每次升级必须重跑 License、migration、billing、protocol 和 BlackRain 三方 E2E。

## 2026-07-12：接受并履行 AGPL 与 Section 7

- 决策：Relay 公开运营并提供对应修改源码，保留 New API / QuantumNous 署名、原项目链接、LICENSE、NOTICE 和受保护元数据。
- 原因：Relay 本身就是公开独立产品，AGPL 不是该项目的商业阻塞；私有 Desktop/Cloud 仅通过 HTTP 合同使用服务。
- 替代方案：购买商业许可；使用 Apache/MIT 网关；Relay 私有闭源。
- 后续复查条件：上线前法律复核实际部署、修改和源码提供方式；License 变化时停止升级并重审。

## 2026-07-12：Cloud 是 Relay 企业客户

- 决策：Cloud 使用管理面企业凭据申请 scoped model token；Desktop 使用数据面 token；Relay 不直接依赖 Supabase 或 Cloud 私有表。
- 原因：三个产品独立运营、独立故障域和独立账本，避免把 BlackRain 身份模型写死在公开中转站。
- 替代方案：Relay 直接验证 Supabase JWT；共享数据库；Cloud 代理模型正文。
- 后续复查条件：实现前冻结管理 API、token、usage 和撤销合同。

## 2026-07-13：直销复用 New API 原生账户能力

- 决策：普通直销客户使用现有 `User`、`Group`、`Token`、充值和请求级计费；仅为需要自动化下游发行与对账的客户增加 enterprise tenant 映射层。
- 原因：现有能力已经覆盖开户、授额、模型白名单、过期、撤销和 usage，重复建设会增加迁移、账务与安全成本。
- 替代方案：所有客户统一迁移到新 tenant 系统；为 BlackRain Cloud 定制专用账户模型。
- 后续复查条件：原生账户无法满足多成员组织、统一账单或企业 RBAC 时，再扩展 enterprise tenant 能力。

## 2026-07-13：采用 dev 到 production 发布路径

- 决策：小团队暂不维护长期 staging 环境；开发验证通过后，以内部账户和 BlackRain Cloud 测试租户小流量发布到 production。
- 原因：减少环境维护成本，同时保留真实生产链路验证。
- 替代方案：长期 staging；直接全量开放 production。
- 发布门槛：独立 Secret、TLS、迁移前备份、健康检查、监控告警、快速回滚以及计费/限流/流式/对账 smoke 全部通过。

## 2026-07-23：首期采用单 App 生产架构

- 决策：首期使用单台 `s-4vcpu-8gb` App、Reserved IP、Caddy HTTPS、托管 PostgreSQL 和托管 Valkey；不使用长期 staging、Load Balancer 或双 App。
- 原因：首月目标是约 500 人同时在线，DeepKey 上游可用性不属于 Relay 容量边界。单 App 更容易控制连接池、后台任务、日志、发布和回滚；垂直容量比双 App 更符合小团队效率和成本优先。
- 运行边界：先通过 500 在线连接与 100/250/500 并发流式请求压测验证 Relay 自身的 CPU、内存、数据库、缓存、计费和连接稳定性。
- 替代方案：双 App + Load Balancer；仅在单 App 压测无法达标、需要无中断发布或出现明确生产 SLA 时启用。
- 后续复查条件：App CPU 持续超过 70%、内存超过 75%、Relay 错误率/P95 不达标，或需要应用级自动故障切换时，重新评估第二 App；启用前必须为所有 master 后台任务增加数据库租约或单主提升机制。

## 2026-07-13：企业控制面使用独立凭据域

- 决策：企业自动化接口固定在 `/api/enterprise/v1`，使用独立 service credential；每个 tenant 绑定一个内部执行用户，external subject 关联现有 scoped token。
- 原因：复用成熟的 quota、group、token 和请求级计费，同时阻止 Cloud credential 获得 Relay 管理员或模型数据面权限。
- 替代方案：复用 `User.AccessToken`；为每个 Cloud 用户创建 Relay 登录账户；单独重写企业计费路径。
- 后续复查条件：企业需要多成员后台、企业 RBAC 或独立终端用户账本时，再扩展 tenant 模型。
