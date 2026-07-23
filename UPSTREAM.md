# Upstream Policy

## Remotes

```text
origin   https://github.com/goodbyeri-studio/meimei-api.git
upstream https://github.com/QuantumNous/new-api.git
```

`origin/main` 是 MeiMei API 可发布基线；`upstream` 只用于评估和同步 New API。

## 当前锁定

- Tag：`v1.0.0-rc.21`
- Commit：`bde9b2f44887d34ec54799ae191d50f97914359e`
- Release：2026-07-11
- License：AGPLv3；README 声明 Section 7 additional terms，修改版 UI 必须保留指定署名和原项目链接。

## 升级流程

1. `git fetch upstream --tags --prune`。
2. 阅读目标 release notes、License、NOTICE、依赖、migration、API/计费/鉴权变化。
3. 在短分支上把目标 tag 合入或重放 MeiMei API patch；禁止直接跟随 `upstream/main`。
4. 解决冲突时优先保留上游品牌、署名、License 和安全修复；MeiMei API 改动应尽量位于独立文件或稳定扩展边界。
5. 跑上游 Go、前端、数据库兼容、billing、relay protocol 和 migration 测试。
6. 跑 BlackRain Cloud 与 MeiMei API 的 token/usage 合同、WORK Chat、CODE Responses、限流、余额和故障矩阵。
7. 在 `.specs/001-relay-foundation/verification.md` 记录旧锁、新锁、命令、结果和未验证风险。
8. 通过 PR 合并，不在升级过程中顺手修改无关产品行为。

## 禁止事项

- 删除、隐藏或替换 New API / QuantumNous 署名、原项目链接、License、NOTICE 或受保护元数据。
- 把 Cloud 私有源码、Supabase service-role、支付密钥、模型 key、用户 token 或生产数据带入公开仓库。
- 把上游测试数量当作 MeiMei API 与 BlackRain Cloud/Desktop E2E 已通过。
- 在未验证 migration 和回滚前更新生产镜像。
