---
title: 阶段 5：活动配置
type: design
status: published
updated: 2026-09-24
---

# 阶段 5：活动配置

## 1. 目标与范围

组织管理员用 `console` 令牌准备一场活动：创建活动、配置围栏与奖项、导入并维护名单、生成活动码，并在结束后导出数据。

做：

1. `events`、`attendees`、`prizes` 三张表
2. 活动首次进入 `ready` 时扣 1 个场次，并写配额流水；草稿不收费
3. 签到方式、围栏、签到时间窗、是否允许兼中
4. 奖项配置、Excel 名单导入与单条增删改
5. 活动级人数上限；运营可单独调高
6. 活动公开码与小程序码图片
7. 导出名单

不做：

- 宾客签到与人工确认。见下一份现场签到方案
- 抽奖、作废、大屏
- 复制活动模板
- 品牌装修。见 [阶段 8](phase-8-branding.md)
- 现场工作人员授权
- 活动结束后自动清理坐标。保留策略单列，不在本模块执行删除

## 2. 现状与约束

| 项 | 现状 |
| --- | --- |
| 身份 | 组织管理员认证提供 `console` 令牌和 `org_id` |
| 配额 | 组织与配额提供 `event_credits` 和组织默认 `max_attendees` |
| 坐标系 | GCJ-02 |
| 半径 | 默认 400 米，允许 100～1000 米 |
| 名单上限 | 活动名单总人数不得超过 `events.max_attendees`；导入与单条新增都校验 |
| 时区 | 库内 `timestamptz`；控制台输入、页面展示与 Excel 导出一律按 `Asia/Shanghai` |

所有查询都从令牌解出 `org_id`。路径里的活动 ID 必须属于该组织，否则返回 `404 E_NOT_FOUND`。

## 3. 原则

1. 首次就绪、扣场次、写流水在同一个事务里；同一活动只扣一次，退回草稿或结束都不退还
2. 配额不足时可以创建和配置草稿，但不能就绪
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
| `checkin_mode` | varchar(16) | `geo` / `direct`，默认 `geo` |
| `center_lat` / `center_lng` | numeric(9,6) | 可空；`geo` 模式就绪前必填 |
| `radius_m` | integer | 默认 400 |
| `checkin_start` / `checkin_end` | timestamptz | 可空；就绪前必填 |
| `allow_multi_win` | boolean | 默认 false |
| `max_attendees` | integer | 非空，`> 0`；创建时复制组织默认值 |
| `credit_consumed_at` | timestamptz | 可空；首次就绪扣场次时写入 |
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

创建活动要求组织为 `active`，不扣场次。`credit_consumed_at` 为空的活动进入 `ready` 时，锁定组织配额行，要求 `event_credits > 0`；成功后余额减 1，写入 `credit_consumed_at`，流水 `delta = -1`、`event_id` 为该活动、原因固定为 `event ready`。余额为 0 时返回 `409 E_NO_EVENT_CREDITS`，补进 `bizerr`。

状态迁移：

| 迁移 | 条件 |
| --- | --- |
| `draft` → `ready` | 已有时间窗和至少一名名单人员；`geo` 模式还需要中心点与半径；首次就绪还需要有剩余场次 |
| `ready` → `draft` | 没有现场数据（微信绑定、签到、人工确认、中奖记录，由阶段 6、7 引入）。试跑后先按[阶段 6](phase-6-checkin.md) 重置现场数据 |
| `ready` → `closed` | 随时；`closed` 是终态 |

`ready` 下仍可改签到方式、围栏、时间窗、奖项和追加名单，保存后立即生效；`ready` 活动从 `direct` 切到 `geo` 时必须已有中心点与半径；`allow_multi_win` 在产生中奖记录后不可改。宾客签到与抽奖只接受 `ready` 活动。`closed` 后拒绝修改配置与导入。

### 4.2 接口

全部要求 `console` 令牌，挂在 `/api/organization`。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET · POST | `/api/organization/events` | 列表 · 创建 |
| GET · PATCH | `/api/organization/events/:id` | 详情 · 修改名称、签到方式、时间窗、围栏、兼中开关、状态 |
| GET | `/api/organization/events/:id/entry` | `{public_id, path}`，`path` 为小程序路径 `pages/index/index?e=<public_id>` |
| GET | `/api/organization/events/:id/entry/qrcode` | 小程序码 PNG |
| POST | `/api/organization/events/:id/attendees/import` | `multipart` xlsx |
| GET · POST | `/api/organization/events/:id/attendees` | 名单分页 · 单条新增 `{name, dept, phone}` |
| PATCH · DELETE | `/api/organization/events/:id/attendees/:attendeeId` | 修改姓名、部门、后四位 · 删除 |
| GET · POST | `/api/organization/events/:id/prizes` | 奖项列表 · 新增 |
| PATCH · DELETE | `/api/organization/events/:id/prizes/:prizeId` | 修改 · 删除 |
| GET | `/api/organization/events/:id/exports/attendees` | 名单 xlsx。签到列由阶段 6 追加 |

小程序码由后端调用微信「获取不限制的小程序码」接口生成：`scene` 为 `public_id`（21 位 nanoid 字符在微信允许的字符集内，不超过 32 位上限），`page` 为 `pages/index/index`。该接口的 `page` 不能带参数，小程序从 `decodeURIComponent(query.scene)` 取活动码；`path` 中的 `e` 参数用于开发者工具与复制链接。`check_path` 默认要求页面已在正式版发布，开发期用 `env_version` 指向 `develop` 或 `trial`。组织客户没有平台小程序的管理后台权限，所以码图必须由后端生成。

本阶段引入 ScopeInfra 配置 `WECHAT_APP_ID`、`WECHAT_APP_SECRET` 与 `internal/wechat`（access_token、小程序码），阶段 6 复用它做 `code` 换 openid。

access_token 在多实例间共用：

| 项 | 设定 |
| --- | --- |
| 获取接口 | 微信「获取稳定版接口调用凭据」（`/cgi-bin/stable_token`）普通模式。官方说明：有效期内重复调用不会更新凭据，并会提前 5 分钟更新。各实例并发获取不会互相作废 |
| 缓存 | Redis 键 `gl:wechat:access_token`，过期时间取微信返回的 `expires_in` 减 300 秒 |
| 强制刷新 | 不使用。官方限制每天 20 次且间隔 30 秒，并会作废旧凭据。微信返回凭据失效时只删除缓存并按普通模式重取一次 |
| Redis 不可用 | 直接按普通模式获取，不缓存 |

本阶段同时引入 Redis（归属规则见[技术方案 §4.5](../DESIGN.md#45-共享状态与多实例)）：

- `internal/redisx`：客户端装配、`Ping`、测试辅助 `OpenTest`（连 db 15 并 `FLUSHDB`）
- ScopeInfra 配置 `REDIS_URL`（必填，Secret），本机为 `redis://127.0.0.1:57379/0`
- 启动顺序在 `store.Ping` 之后增加 `redisx.Ping`，不可达时拒绝启动；`GET /healthz` 增加 `redis` 字段，`/readyz` 不变
- 落地时同步 [技术方案 §9 启动与关闭](../DESIGN.md#9-启动与关闭) 与 [§10 配置](../DESIGN.md#10-配置)

导入文件第一行是表头：`姓名`、`部门`、`手机号`。服务端只取手机号后四位。上传文件不超过 5 MB。导入后活动名单总数超过 `max_attendees` 返回 `400 E_BAD_REQUEST`。Excel 读写使用 `github.com/xuri/excelize/v2`，落地时写入 [技术方案 §4.1](../DESIGN.md#41-技术选型)。

导入错误体包含行号与原因：空姓名、手机号不足四位、文件内重复、与已有名单重复。任一错误都回滚。多次导入只追加，不提供覆盖导入：覆盖会冲掉已绑定的宾客。

单条维护：

| 操作 | 规则 |
| --- | --- |
| 新增 | 校验同导入；超过 `events.max_attendees` 返回 `400 E_BAD_REQUEST`；与已有 `(name, phone_last4)` 重复返回 `409 E_CONFLICT` |
| 修改 | 姓名、部门、后四位随时可改；已绑定的微信保持不变 |
| 删除 | 只删未绑定、未签到、未中奖的人，否则返回 `409 E_CONFLICT`。绑定与签到校验由阶段 6 补上，中奖校验由阶段 7 补上 |

运营调整活动上限（要求 `platform` 令牌）：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/platform/orgs/:id/events` | 该组织的活动列表，含状态、名单人数、上限 |
| PATCH | `/api/platform/orgs/:id/events/:eventId` | `{max_attendees}`；不能低于当前名单人数；不影响场次余额 |

奖项在已有中奖记录后只允许改名称、奖品和顺序，不允许把 `quota` 改到小于已抽出的有效人数，也不允许删除。该约束由 [阶段 7](phase-7-draw.md) 在建中奖表时实现。

### 4.3 前端

`/organization` 进入活动列表：

- 创建时填写名称，不受场次限制
- 未扣过场次的活动在就绪前提示「将消耗 1 场次」；剩余场次为 0 时就绪按钮不可用并提示联系开通
- 名单区块支持单条新增、编辑、删除
- 详情分签到设置（签到方式、围栏、时间窗）、名单、奖项三个区块；`direct` 模式隐藏围栏输入但保留已填值
- 选择 `direct` 时提示「宾客不在现场也能签到，奖池可能包含缺席者，需依靠现场缺席重抽」
- 导入后展示成功行数或逐行错误
- 提供小程序码下载与活动路径复制

### 4.4 测试

| 对象 | 用例 |
| --- | --- |
| 创建 | 不扣场次；复制组织人数上限；组织停用时拒绝 |
| 扣场次 | 首次就绪扣 1 次并写带 `event_id` 的流水；退回草稿再就绪不再扣；余额为 0 时返回 `409` 且状态不变；并发就绪两场活动不能扣成负数 |
| 名单维护 | 新增超限或重复被拒绝；修改后唯一约束仍生效 |
| 上限调整 | 运营调高后可继续导入；低于当前人数被拒绝；`console` 令牌调用返回 `401` |
| 隔离 | 其他组织的活动 ID 返回 `404` |
| 导入 | 合法文件写入；重复或非法行整批拒绝；多次导入后总数超过人数上限拒绝 |
| 状态 | `geo` 模式缺少围栏或缺少名单时拒绝就绪；`direct` 模式无围栏可就绪；就绪后切到 `geo` 但缺围栏被拒绝；非法迁移拒绝；`closed` 后拒绝修改 |
| 微信凭据 | 两个实例共用 Redis 中的同一份；缓存过期后重取；微信返回凭据失效时只重取一次；Redis 不可达时仍能生成小程序码 |
| 小程序码 | 微信接口用测试替身；`scene` 等于 `public_id`；微信错误转为 `E_INTERNAL` 并记录原始错误码 |
| 奖项 | `quota <= 0` 拒绝；活动内 `sort_no` 重复拒绝 |

## 5. 明确不做

- 在线地图组件的供应商选型。前端可先用经纬度输入
- 复制上场配置
- 工作人员白名单
- 导出签到尝试明细。由阶段 6 交付
- 导出中奖名单。由阶段 7 交付
- 短链。已降为 P1，登记在 [ROADMAP](../ROADMAP.md)

## 6. 完成定义

- 迁移可重复执行，测试用真实 PostgreSQL
- 生成物与 `openapi.yaml` 一致
- API 与 Web 的格式、lint、测试全绿
- 管理员可以完成创建、导入、配置奖项，并下载可扫码进入小程序的活动码
