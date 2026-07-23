# BlackRain Relay

BlackRain Relay 是独立运营的公开模型中转产品，当前以 [QuantumNous/New API](https://github.com/QuantumNous/new-api) 为底座，完整遵守 AGPLv3、Section 7、署名和原项目链接义务。

## 产品关系

```text
BlackRain Desktop
  -> BlackRain Cloud：身份、套餐、权益、商业 credit ledger
  -> BlackRain Relay：model token、模型路由、usage、限流和中转
  -> DeepSeek / GLM / Qwen / Kimi / 其他授权模型渠道
```

- Relay 是独立产品，可以服务 BlackRain Cloud 和其他合法客户。
- BlackRain Cloud 是 Relay 的企业客户，不与 Relay 共享数据库。
- Desktop 只持可撤销、可限额、可过期的 model token，不持 Relay 管理凭据或模型厂商 key。
- Relay 不复制 BlackRain Cloud 的私有账号、支付、工作台市场或商业账本逻辑。

## 当前基线

| 项目 | 值 |
|---|---|
| 上游 | `https://github.com/QuantumNous/new-api.git` |
| 锁定版本 | `v1.0.0-rc.21` |
| 锁定 commit | `bde9b2f44887d34ec54799ae191d50f97914359e` |
| License | AGPLv3 + README 中的 Section 7 additional terms |
| 初始化日期 | 2026-07-12 |

仓库保留 `upstream` remote。升级纪律见 [UPSTREAM.md](UPSTREAM.md)，BlackRain 自有工作计划见 [.specs/001-relay-foundation/](.specs/001-relay-foundation/)。

## 当前状态

- New API 上游 release 源码、License、NOTICE 和历史已导入。
- 已建立 BlackRain Relay 独立生产基础设施：专用 DigitalOcean Project、VPC、单台 4 vCPU/8 GB App、Reserved IP、托管 PostgreSQL、托管 Valkey、Firewall、Container Registry，以及 `relay.goodbyeri.cc` DNS；首期不使用 Load Balancer。
- Caddy/TLS、Reserved IP 和 DNS 已完成；当前尚未发布 Relay 应用镜像，也尚未配置 production Secret、模型渠道、Cloud 企业客户和真实流量压测。基础设施存在不等于中转服务已经上线或可以商业运营。
- Relay 与 `2049-agent` 完全独立，不共享其 Droplet、数据库、Valkey、Spaces、Secret 或部署节点。
- 仓库存在和上游代码可构建，不等于中转服务已部署或可商业运营。

## 下一步

1. 按上游文档完成本地无 Secret build/test 基线。
2. 冻结 Cloud↔Relay 管理 API、scoped token 和 usage 对账合同。
3. 按 [.specs/001-relay-foundation/production-infrastructure.md](.specs/001-relay-foundation/production-infrastructure.md) 完成镜像、Secret、TLS 和可回滚发布。
4. 配置合法授权模型渠道、倍率、限流、日志和备份。
5. 跑通 WORK Chat 与 CODE Responses 两条 BlackRain 产品链路，并完成约 500 活跃流式请求压测。
6. 完成 Cloud↔Relay 管理 API、scoped token 和 usage 对账合同。
7. 完成 AGPL 源码提供、模型厂商转售条款和国内运营合规审查。
