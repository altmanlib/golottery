---
title: 阶段 5：活动配置
type: design
status: published
updated: 2026-09-24
---

# 阶段 5：活动配置

## 1. 目标与范围

组织管理员用 `console` 令牌准备一场活动：创建活动、配置围栏与奖项、导入名单、生成活动码，并在结束后导出数据。

做：

1. `events`、`attendees`、`prizes` 三张表
2. 创建活动时扣 1 个场次，并写配额流水
3. 围栏、签到时间窗、是否允许兼中
4. 奖项配置与 Excel 名单导入
5. 活动公开码与小程序码图片
6. 导出名单

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
| 导入上限 | 活动名单总人数不得超过该组织的 `max_attendees` |
| 时区 | 库内 `timestamptz`；控制台输入、页面展示与 Excel 导出一律按 `Asia/Shanghai` |

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

状态迁移：

| 迁移 | 条件 |
| --- | --- |
| `draft` → `ready` | 已有中心点、半径、时间窗和至少一名名单人员 |
| `ready` → `draft` | 尚无签到记录 |
| `ready` → `closed` | 随时；`closed` 是终态 |

`ready` 下仍可改围栏、时间窗、奖项和追加名单，保存后立即生效；`allow_multi_win` 在产生中奖记录后不可改。宾客签到与抽奖只接受 `ready` 活动。`closed` 后拒绝修改配置与导入。

### 4.2 接口

全部要求 `console` 令牌，挂在 `/api/organization`。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET · POST | `/api/organization/events` | 列表 · 创建 |
| GET · PATCH | `/api/organization/events/:id` | 详情 · 修改名称、时间窗、围栏、兼中开关、状态 |
| GET | `/api/organization/events/:id/entry` | `{public_id, path}`，`path` 为小程序路径 `pages/index/index?e=<public_id>` |
| GET | `/api/organization/events/:id/entry/qrcode` | 小程序码 PNG |
| POST | `/api/organization/events/:id/attendees/import` | `multipart` xlsx |
| GET | `/api/organization/events/:id/attendees` | 名单分页 |
| GET · POST | `/api/organization/events/:id/prizes` | 奖项列表 · 新增 |
| PATCH · DELETE | `/api/organization/events/:id/prizes/:prizeId` | 修改 · 删除 |
| GET | `/api/organization/events/:id/exports/attendees` | 名单 xlsx。签到列由阶段 6 追加 |

小程序码由后端调用微信「获取不限制的小程序码」接口生成：`scene` 为 `public_id`（21 位 nanoid 字符在微信允许的字符集内，不超过 32 位上限），`page` 为 `pages/index/index`。该接口的 `page` 不能带参数，小程序从 `decodeURIComponent(query.scene)` 取活动码；`path` 中的 `e` 参数用于开发者工具与复制链接。`check_path` 默认要求页面已在正式版发布，开发期用 `env_version` 指向 `develop` 或 `trial`。组织客户没有平台小程序的管理后台权限，所以码图必须由后端生成。

本阶段引入 ScopeInfra 配置 `WECHAT_APP_ID`、`WECHAT_APP_SECRET` 与 `internal/wechat`（access_token 缓存、小程序码），阶段 6 复用它做 `code` 换 openid。

导入文件第一行是表头：`姓名`、`部门`、`手机号`。服务端只取手机号后四位。上传文件不超过 5 MB。导入后活动名单总数超过 `max_attendees` 返回 `400 E_BAD_REQUEST`。Excel 读写使用 `github.com/xuri/excelize/v2`，落地时写入 [技术方案 §4.1](../DESIGN.md#41-技术选型)。

导入错误体包含行号与原因：空姓名、手机号不足四位、文件内重复、与已有名单重复。任一错误都回滚。

奖项在已有中奖记录后只允许改名称、奖品和顺序，不允许把 `quota` 改到小于已抽出的有效人数，也不允许删除。该约束由 [阶段 7](phase-7-draw.md) 在建中奖表时实现。

### 4.3 前端

`/organization` 进入活动列表：

- 创建时填写名称；剩余场次为 0 时按钮不可用
- 详情分围栏、时间窗、名单、奖项四个区块
- 导入后展示成功行数或逐行错误
- 提供小程序码下载与活动路径复制

### 4.4 测试

| 对象 | 用例 |
| --- | --- |
| 创建 | 成功扣 1 个场次并写流水；余额为 0 或组织停用时不产生活动 |
| 隔离 | 其他组织的活动 ID 返回 `404` |
| 导入 | 合法文件写入；重复或非法行整批拒绝；多次导入后总数超过人数上限拒绝 |
| 状态 | `ready` 缺少围栏或名单时拒绝；非法迁移拒绝；`closed` 后拒绝修改 |
| 小程序码 | 微信接口用测试替身；`scene` 等于 `public_id`；微信错误转为 `E_INTERNAL` 并记录原始错误码 |
| 奖项 | `quota <= 0` 拒绝；活动内 `sort_no` 重复拒绝 |

## 5. 明确不做

- 在线地图组件的供应商选型。前端可先用经纬度输入
- 复制上场配置
- 工作人员白名单
- 导出签到尝试明细。由阶段 6 交付
- 导出中奖名单。由阶段 7 交付

## 6. 开放项

| 问题 | 现状 | 需要在哪个阶段前定 |
| --- | --- | --- |
| 名单单条增删改 | 只有整批导入，导入错字或现场临时加人无法处理 | 阶段 5 开工前 |
| 试跑数据清理 | PRD 要求活动前用测试账号跑通签到与抽奖，试跑产生的签到和中奖记录会留在正式活动里 | 阶段 6 开工前。备选：`ready` → `draft` 时允许清空签到、人工确认与中奖数据 |
| 短链 | PRD O4 要求短链，本阶段只交付小程序码与页面路径 | 阶段 8 前，需先核实微信官方链接能力的限制 |

## 7. 完成定义

- 迁移可重复执行，测试用真实 PostgreSQL
- 生成物与 `openapi.yaml` 一致
- API 与 Web 的格式、lint、测试全绿
- 管理员可以完成创建、导入、配置奖项，并下载可扫码进入小程序的活动码
