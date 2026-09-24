---
title: 活动配置
type: design
status: published
updated: 2026-09-24
---

# 活动配置

## 1. 目标与范围

组织管理员用 `console` 令牌准备一场活动：创建活动、配置围栏与奖项、导入名单、生成活动码，并在结束后导出数据。

做：

1. `events`、`attendees`、`prizes` 三张表
2. 创建活动时扣 1 个场次，并写配额流水
3. 围栏、签到时间窗、是否允许兼中
4. 奖项配置与 Excel 名单导入
5. 活动公开码与短链
6. 导出签到名单与中奖名单

不做：

- 宾客签到与人工确认。见下一份现场签到方案
- 抽奖、作废、大屏
- 复制活动模板、品牌封面与主题色
- 现场工作人员授权
- 活动结束后自动清理坐标。保留策略单列，不在本模块执行删除

## 2. 现状与约束

| 项 | 现状 |
| --- | --- |
| 身份 | 组织管理员认证提供 `console` 令牌和 `org_id` |
| 配额 | 组织与配额提供 `event_credits` 和 `max_attendees` |
| 坐标系 | GCJ-02 |
| 半径 | 默认 400 米，允许 100～1000 米 |
| 导入上限 | 名单行数不得超过该组织的 `max_attendees` |

所有查询都从令牌解出 `org_id`。路径里的活动 ID 必须属于该组织，否则返回 `404 E_NOT_FOUND`。

## 3. 原则

1. 创建活动、扣场次、写流水在同一个事务里
2. 配额不足时不创建活动
3. 停用组织不能创建或修改活动
4. `public_id` 使用 21 位 nanoid，不暴露自增 ID
5. 导入整批校验。有错误时不写入任何名单行
6. 手机号只保存后四位

## 4. 方案

### 4.1 数据

新增迁移 `005_event_setup.sql`。前置迁移号变化时顺延。

`events`

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| `id` | uuid | 主键 |
| `org_id` | uuid | 非空，外键 → `orgs`，索引 |
| `public_id` | varchar(21) | 唯一 |
| `name` | varchar(100) | 非空 |
| `status` | varchar(16) | `draft` / `ready` / `closed` |
| `center_lat` / `center_lng` | numeric(9,6) | 可空；就绪前必填 |
| `radius_m` | integer | 默认 400 |
| `checkin_start` / `checkin_end` | timestamptz | 可空；就绪前必填 |
| `allow_multi_win` | boolean | 默认 false |
| `created_at` / `updated_at` | timestamptz | |

`attendees`

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| `id` | uuid | 主键 |
| `org_id` / `event_id` | uuid | 非空，外键 |
| `name` | varchar(100) | 非空 |
| `dept` | varchar(100) | 默认空串 |
| `phone_last4` | char(4) | 非空 |
| `status` | varchar(16) | 本阶段只有 `pending` |
| `created_at` | timestamptz | |

唯一约束：`(event_id, name, phone_last4)`。

`prizes`

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| `id` | uuid | 主键 |
| `org_id` / `event_id` | uuid | 非空 |
| `name` | varchar(100) | 非空 |
| `gift` | varchar(200) | 默认空串 |
| `quota` | integer | `> 0` |
| `sort_no` | integer | 活动内唯一 |

创建活动时锁定组织配额行，要求组织为 `active` 且 `event_credits > 0`。成功后余额减 1，流水 `delta = -1`，原因固定为 `create event`。

活动从 `draft` 变为 `ready` 前必须已有中心点、半径、时间窗和至少一名名单人员。`closed` 后拒绝修改配置与导入。

### 4.2 接口

全部要求 `console` 令牌，挂在 `/api/organization`。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET · POST | `/api/organization/events` | 列表 · 创建 |
| GET · PATCH | `/api/organization/events/:id` | 详情 · 修改名称、时间窗、围栏、兼中开关、状态 |
| GET | `/api/organization/events/:id/entry` | `{public_id, path}`，`path` 为小程序路径 `pages/index/index?e=<public_id>` |
| POST | `/api/organization/events/:id/attendees/import` | `multipart` xlsx |
| GET | `/api/organization/events/:id/attendees` | 名单分页 |
| GET · POST | `/api/organization/events/:id/prizes` | 奖项列表 · 新增 |
| PATCH · DELETE | `/api/organization/events/:id/prizes/:prizeId` | 修改 · 删除 |
| GET | `/api/organization/events/:id/exports/:type` | `attendees` 或 `winners` |

导入文件第一行是表头：`姓名`、`部门`、`手机号`。服务端只取手机号后四位。单文件最多 `max_attendees` 行，超过返回 `400 E_BAD_REQUEST`。

导入错误体包含行号与原因：空姓名、手机号不足四位、文件内重复、与已有名单重复。任一错误都回滚。

奖项在已有中奖记录后只允许改名称、奖品和顺序，不允许把 `quota` 改到小于已抽出的有效人数，也不允许删除。中奖表由抽奖方案建立，本方案只预留这个约束。

`winners` 导出在抽奖记录不存在时返回只有表头的文件。

### 4.3 前端

`/organization` 进入活动列表：

- 创建时填写名称；剩余场次为 0 时按钮不可用
- 详情分围栏、时间窗、名单、奖项四个区块
- 导入后展示成功行数或逐行错误
- 提供活动路径复制。小程序码图片由微信侧生成，不在后端渲染

### 4.4 测试

| 对象 | 用例 |
| --- | --- |
| 创建 | 成功扣 1 个场次并写流水；余额为 0 或组织停用时不产生活动 |
| 隔离 | 其他组织的活动 ID 返回 `404` |
| 导入 | 合法文件写入；重复或非法行整批拒绝；超过人数上限拒绝 |
| 状态 | `ready` 缺少围栏或名单时拒绝；`closed` 后拒绝修改 |
| 奖项 | `quota <= 0` 拒绝；活动内 `sort_no` 重复拒绝 |

## 5. 明确不做

- 在线地图组件的供应商选型。前端可先用经纬度输入
- 复制上场配置
- 工作人员白名单
- 导出签到尝试坐标。签到方案确定记录内容后再开放

## 6. 完成定义

- 迁移可重复执行，测试用真实 PostgreSQL
- 生成物与 `openapi.yaml` 一致
- API 与 Web 的格式、lint、测试全绿
- 管理员可以完成创建、导入、配置奖项和复制活动路径
