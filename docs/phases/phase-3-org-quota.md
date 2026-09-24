---
title: 阶段 3：组织与配额
type: design
status: published
updated: 2026-09-24
---

# 阶段 3：组织与配额

## 1. 目标与范围

运营登录后可以开通组织，并手工增减场次配额。这是活动数据的租户边界：后续活动、名单、奖项都挂 `org_id`，创建活动时再扣场次。

做：

1. `orgs`、`org_quotas`、`credit_ledger` 三张表
2. 开通组织、停用与启用、调整场次与人数上限
3. 每次配额变化写一条只追加流水
4. 控制台列出组织与剩余场次

不做：

- 组织管理员账号与 `console` 令牌
- 活动、围栏、奖项、名单、导入
- 在线支付、发票、自动扣场次
- 用量统计（活动数、峰值签到）
- 审计日志

## 2. 现状与约束

| 项 | 现状 |
| --- | --- |
| 身份 | 控制台认证计划只提供 `platform` 令牌，本模块全部接口使用它 |
| 配额规则 | 创建活动消耗 1 个场次；用尽后不可创建。创建动作不在本模块 |
| 人数档位 | 体验 100、标准 800、加量 2000。首发由运营写入，不在线购买 |
| 隔离 | 共享库，业务行带 `org_id`。本模块的行本身就是组织 |

人数上限在这里只保存。名单导入阶段读取它并拒绝超限，本模块不实现导入。

## 3. 原则

1. 场次余额以 `org_quotas.event_credits` 为准，流水只解释余额为什么变化
2. 余额与流水在同一个事务里写。任一方失败则整笔回滚
3. 流水不提供修改和删除
4. 余额扣到负数时拒绝，不留下失败流水
5. 停用组织的效果：管理员无法登录且已有 `console` 令牌失效（[阶段 4](phase-4-org-admin-auth.md)），不能创建或修改活动（[阶段 5](phase-5-event-setup.md)）。已就绪活动的宾客签到与主持人抽奖不受影响，避免运营误操作中断现场。不删除组织、不回收已写流水
6. 接口继续走 `openapi.yaml`，错误继续走 `bizerr`

## 4. 方案

### 4.1 数据

新增迁移 `003_org_quota.sql`。编号排在 `platform_users` 之后；若认证迁移号变化，本文件号顺延，不复用。

`orgs`

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| `id` | uuid | 主键，应用生成 |
| `name` | varchar(100) | 非空 |
| `contact` | varchar(100) | 默认空串 |
| `status` | varchar(16) | `active` / `disabled` |
| `created_at` / `updated_at` | timestamptz | |

`org_quotas`

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| `org_id` | uuid | 主键，外键 → `orgs`，删除组织时级联删除 |
| `event_credits` | integer | 非空，`>= 0` |
| `max_attendees` | integer | 非空，`> 0` |
| `updated_at` | timestamptz | |

一个组织一行配额。

`credit_ledger`

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| `id` | uuid | 主键 |
| `org_id` | uuid | 外键 → `orgs`，索引 |
| `delta` | integer | 非零 |
| `balance_after` | integer | 非空，`>= 0` |
| `reason` | varchar(200) | 非空 |
| `event_id` | uuid | 可空；创建活动扣场次时填写，外键在活动迁移中补加 |
| `operator_type` | varchar(32) | `platform` / `console`，即操作令牌的 `principal_type` |
| `operator_id` | varchar(64) | 操作令牌的 `principal_id` |
| `created_at` | timestamptz | |

开通组织时在同一事务插入三行：组织、配额、一条 `delta = event_credits` 的流水。`event_credits = 0` 时不写流水。

### 4.2 接口

全部在 `/api/console` 下，要求 `platform` 令牌。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/console/orgs` | 列表。项含名称、状态、剩余场次、人数上限 |
| POST | `/api/console/orgs` | `{name, contact, event_credits, max_attendees}` |
| GET | `/api/console/orgs/:id` | 组织、配额、最近 20 条流水 |
| POST | `/api/console/orgs/:id/disable` | 停用 |
| POST | `/api/console/orgs/:id/enable` | 启用 |
| POST | `/api/console/orgs/:id/credits` | `{delta, reason}` |

校验：

| 条件 | 错误 |
| --- | --- |
| 名称为空 | `400 E_NAME_REQUIRED` |
| `event_credits < 0` 或 `max_attendees <= 0` | `400 E_BAD_REQUEST` |
| `delta = 0` 或 `reason` 为空 | `400 E_BAD_REQUEST` |
| 扣减后余额小于 0 | `409 E_CONFLICT` |
| 组织不存在 | `404 E_NOT_FOUND` |

停用与启用是幂等的：状态已经是目标值时返回 `204`，不写配额流水。

### 4.3 前端

控制台在 `/console` 下增加组织列表与开通表单：

- 列表显示名称、状态、剩余场次、人数上限
- 开通时填写名称、联系人、初始场次、人数上限
- 详情里可以停用、启用、按正负整数调整场次并填写原因
- 流水只读

不新增登录和账号管理界面。

### 4.4 测试

| 对象 | 用例 |
| --- | --- |
| 开通 | 组织、配额、首条流水在同一事务；初始场次为 0 时没有流水 |
| 调整 | 增加后余额与 `balance_after` 一致；扣成负数被拒绝且余额不变 |
| 并发 | 两笔扣减不能把余额打成负数 |
| 状态 | 停用后再启用，配额数字不变 |
| 鉴权 | 无令牌或其他 `principal_type` 返回 `401` |

前端只测表单提交值与错误码分支，不测样式。

## 5. 明确不做

- 创建活动时扣场次。活动模块消费 `event_credits`，并再写一条负数流水
- 删除组织
- 修改历史流水
- 按人数档位自动换算场次价格
- 跨组织汇总报表

## 6. 开放项

| 问题 | 现状 | 需要在哪个阶段前定 |
| --- | --- | --- |
| 人数档位挂在组织还是场次 | `max_attendees` 是组织级。PRD 的「按场次开通，可叠加人数档位」无法表达同一组织同时持有 800 人与 2000 人两种场次 | 阶段 3 开工前。至少要在创建活动时把上限快照到 `events.max_attendees`，运营后续调整不影响已建活动 |
| 何时扣场次 | 创建活动即扣。试跑或误建都会消耗场次，只能靠运营手工补回 | 阶段 5 开工前。备选：首次进入 `ready` 时扣，用 `events.credit_consumed_at` 保证只扣一次 |

## 7. 完成定义

- 迁移可重复执行，测试用真实 PostgreSQL
- `make generate` 后生成物与契约一致
- `make fmt && make lint && make test` 全绿
- `bun run gen:api && bun run test:run && bun run typecheck` 全绿
- 运营账号可以开通组织、调整场次，并在列表看到剩余场次
