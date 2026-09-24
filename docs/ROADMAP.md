---
title: Roadmap
type: guide
status: published
updated: 2026-09-24
---

# Roadmap

设计文档索引见 [README.md](README.md)

## 1. 怎么用

- 本文件是唯一登记待办与已知缺口的文档，其余文档只记录当前事实
- **开始工作前**先读本文件，从「计划中」中取最靠前、依赖已满足的一项
- **开工时**把条目移到「现在」，并确认对应的 `docs/phases/phase-<n>-<slug>.md` 已定稿
- **完成时**移到「已完成」并写上提交号；roadmap 的改动与对应工作放在同一个提交里
- **发现新工作**登记到「之后」
- 「现在」最多两项
- 编号只增不复用：`P-` 编号作为编号前缀

## 2. 状态判定

| 状态 | 标准 |
| --- | --- |
| 已完成 | 代码合入，且设计文档与现实一致 |
| 现在 | 设计文档已定稿或代码已开工，但尚未全量交付 |
| 计划中 | 已登记、依赖已满足，可被取用 |
| 之后 | 已发现，但依赖未满足或尚未排期 |
| 暂缓 | 已评估过，但当前不做；需记录触发条件 |

## 3. 现在

| 编号 | 范围 | 内容 |
| --- | --- | --- |
| P-5 | api, web | 阶段 2 运营认证。见 [phases/phase-2-platform-auth.md](phases/phase-2-platform-auth.md) |

## 4. 计划中

| 编号 | 范围 | 内容 |
| --- | --- | --- |
| P-13 | ops | 微信与域名资质：平台小程序主体与类目、`wx.getLocation` 接口权限、隐私保护指引、ICP 备案域名与 HTTPS。阻塞阶段 5 小程序码、阶段 6 真机签到、阶段 8 上线 |

## 5. 之后

| 编号 | 范围 | 内容 |
| --- | --- | --- |
| P-2 | api, web | 阶段 3 组织与配额。见 [phases/phase-3-org-quota.md](phases/phase-3-org-quota.md) |
| P-6 | api, web | 阶段 4 组织管理员认证。见 [phases/phase-4-org-admin-auth.md](phases/phase-4-org-admin-auth.md) |
| P-7 | api, web | 阶段 5 活动配置。见 [phases/phase-5-event-setup.md](phases/phase-5-event-setup.md) |
| P-3 | api, mp | 阶段 6 现场签到。见 [phases/phase-6-checkin.md](phases/phase-6-checkin.md) |
| P-8 | api, web | 阶段 7 现场抽奖。见 [phases/phase-7-draw.md](phases/phase-7-draw.md) |
| P-4 | ops | 阶段 8 上线与现场兜底。见 [phases/phase-8-launch.md](phases/phase-8-launch.md) |
| P-15 | mp, api | 宾客端中奖结果展示（PRD G6，P1）。依赖阶段 7 |
| P-16 | api | 多场并行时按活动限流（PRD §9）。依赖阶段 8 压测结果；单场达标且无并行活动冲突时转暂缓 |
| P-18 | api, web | 活动短链（PRD O8，P1）。依赖阶段 5；实施前先核实微信官方链接能力的有效期与调用额度 |

## 6. 暂缓

| 编号 | 范围 | 结论与触发条件 |
| --- | --- | --- |
| P-9 | web | 复制活动模板。触发条件：同一组织需要重复办活动 |
| P-10 | mp, api | 中奖订阅消息、我的活动列表。触发条件：首发现场流程稳定 |
| P-11 | api | 运营用量统计。触发条件：手动开通组织超过电子表格可维护的规模 |
| P-12 | ops | 在线支付与发票。触发条件：不再由运营手工开通场次 |
| P-17 | api, web | 组织管理员邀请其他管理员（PRD O5），首发由运营代建。触发条件：运营代建管理员成为经常性工作 |

## 7. 已完成

| 编号 | 提交 | 范围 | 内容 |
| --- | --- | --- | --- |
| P-14 | f374059 | docs | 删除三个子项目下的旧版文档，子项目 README 与 AGENTS.md 改为指向根 `docs/` |
| P-1 | 33d5163 | api, web | 阶段 1 工程基线：config / store / httpapi / bizerr / auth、Web 令牌客户端 |
