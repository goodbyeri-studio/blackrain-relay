# Tasks

## 阶段 0：上游基线

- [x] 建立公开仓库和 `origin`/`upstream` remotes
- [x] 锁定 New API `v1.0.0-rc.21` / `bde9b2f44887d34ec54799ae191d50f97914359e`
- [x] 保留 AGPLv3、NOTICE、Section 7 署名和原项目链接
- [x] 建立 BlackRain 文档、AGENTS、上游纪律和 foundation spec
- [x] 运行锁定上游的 Go 测试并记录前端 `dist` 前置条件
- [x] 跑锁定上游的前端构建、完整 Go 测试和 SQLite/PostgreSQL/MySQL 基础测试
- [x] 建立本地 PostgreSQL/Redis dev 环境和前后端开发入口

## 阶段 1：直销闭环与企业客户

- [x] 冻结直销客户复用 New API `User`/`Group`/`Token` 的边界
- [x] 验证 token 签发、模型白名单、额度、过期、禁用和删除生命周期
- [x] 冻结 Cloud service account 与管理 API 权限
- [x] 冻结 enterprise tenant、external subject 和 token 数据模型
- [ ] 实现/验证 enterprise token 的客户/subject 归属与速率策略
- [ ] 禁止 Desktop token 访问管理 API

## 阶段 2：Usage 与结算

- [ ] 冻结稳定 request/usage id 和事件 schema
- [ ] 提供增量 usage API/Webhook 和重放语义
- [ ] 验证 Cloud 幂等入账、日终对账、退款和差错补偿

## 阶段 3：模型数据面

- [ ] 配置首批合法授权模型渠道和价格
- [ ] WORK Chat 流式、工具调用、错误、限流和计量 E2E
- [ ] CODE Responses 经本地翻译网关 E2E
- [ ] New API 原生 Responses 的 codex 严格协议探针

## 阶段 4：生产运营

- [x] 本地开发环境和仅限开发的 Secret 模板
- [ ] production 环境和独立 Secret 管理
- [ ] 内部账户与 BlackRain Cloud 测试租户小流量发布
- [ ] 数据库 migration/rollback、备份、恢复和灾难演练
- [ ] 日志脱敏、监控、告警、容量和成本看板
- [ ] AGPL 源码提供、上游授权、支付/税务和国内运营合规审查
