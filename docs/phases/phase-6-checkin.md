---
title: 阶段 6：现场签到
type: design
status: published
updated: 2026-09-24
---

# 阶段 6：现场签到

## 1. 目标与范围

宾客从活动码进入小程序，绑定名单后在地理围栏内签到。定位失败或名单匹配不上时提交现场求助，现场工作人员在小程序里处理。

做：

1. 宾客微信登录与活动上下文
2. 名单绑定
3. 服务端围栏判定与签到流水
4. 现场求助：定位失败的人工确认，以及名单匹配不上时的关联或新增
5. 工作人员邀请与身份、签到进度、代签到、小程序内围栏设置
6. 小程序宾客页与管理页
7. 名单导出追加签到列；签到尝试明细导出
8. 重置现场数据，清理试跑结果

不做：

- 抽奖与中奖展示
- 订阅消息、我的活动列表
- 大屏签到人数。抽奖方案消费签到结果
- 活动码图片生成。由阶段 5 交付

## 2. 现状与约束

| 项 | 现状 |
| --- | --- |
| 活动 | 活动配置提供 `public_id`、围栏、时间窗和 `pending` 名单 |
| 坐标系 | GCJ-02 |
| 判定 | 距离减去最多 200 米精度后，仍小于等于半径才通过 |
| 精度 | 大于 500 米拒绝自动签到 |
| 限流 | 同一 openid 每分钟最多 10 次签到请求，不按 IP 限流。进程内计数，依赖[单实例前提](../DESIGN.md#4-系统架构) |
| 微信 | 阶段 5 已提供 `internal/wechat` 与 `WECHAT_APP_ID` / `WECHAT_APP_SECRET` |

签到是否通过只由服务端决定。

## 3. 原则

1. 每次签到尝试都写 `checkin_attempts`，无论成败
2. 签到更新使用条件更新，重复请求不改变第一次签到时间
3. 人工通过与定位通过最终都写成 `checked_in`，用 `checkin_method` 区分
4. 工作人员权限按活动授权，不复用组织管理员的 Web 口令
5. 坐标只在导出给组织管理员时可见

## 4. 方案

### 4.1 数据

新增迁移 `006_checkin.sql`。

`guest_sessions`

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| `id` | uuid | 主键 |
| `event_id` | uuid | 非空 |
| `openid` | varchar(64) | 非空 |
| `token_hash` | bytea | 唯一 |
| `expires_at` | timestamptz | |
| `created_at` | timestamptz | |

唯一约束：`(event_id, openid)`。宾客令牌是独立表，不进入 `api_tokens` 的三种后台类型。

`attendees` 追加：

| 字段 | 说明 |
| --- | --- |
| `openid` | 可空；活动内唯一 |
| `status` | `pending` / `checked_in` |
| `checkin_at` | 可空 |
| `checkin_method` | `geo` / `manual` / `proxy`，可空 |
| `checkin_by` | 可空；人工通过或代签到的工作人员 openid |

`checkin_attempts` 只追加：活动、名单人员、openid、经纬度、精度、距离、结果、原因、时间。只记录宾客自己的定位签到；人工通过与代签到没有坐标，只写 `attendees`。

`manual_requests`（现场求助）：

| 字段 | 说明 |
| --- | --- |
| `event_id` | 非空 |
| `openid` | 非空；提交人 |
| `attendee_id` | 可空。已绑定者提交时即填；未绑定者由工作人员处理时填入 |
| `claimed_name` / `claimed_phone_last4` | 未绑定者申报的姓名与后四位 |
| `reason` | 备注 |
| `status` | `pending` / `approved` / `rejected` |
| `handled_by` / `handled_at` | 处理人 openid 与时间 |

同一活动同一 openid 同时只能有一条 `pending`，用 `(event_id, openid) WHERE status = 'pending'` 部分唯一索引保证；被拒绝后可以再次提交。

`event_staff`：`event_id + openid` 唯一，`role` 为 `staff` 或 `admin`。`admin` 额外可以在小程序里改围栏。组织管理员不自动拥有现场权限。

`staff_invites`：活动、角色、邀请码哈希、过期时间、使用者 openid、使用时间。组织管理员拿不到工作人员的 openid，授权改为邀请：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET · POST | `/api/organization/events/:id/staff` | 工作人员列表 · `{role}` 生成邀请，返回一次性邀请码与小程序码 |
| DELETE | `/api/organization/events/:id/staff/:staffId` | 撤销授权 |
| POST | `/api/guest/staff/join` | `{invite}`，把当前宾客会话的 openid 写入 `event_staff` |

邀请码 24 小时有效、只能使用一次，库内只存 SHA-256。

### 4.2 宾客接口

小程序从 `query.scene`（扫码）或 `query.e`（路径）取 `public_id`。宾客令牌有效期由新增 ScopeApp 配置 `GUEST_SESSION_TTL` 决定，默认 `24h`；同一 `(event_id, openid)` 重新登录时轮换令牌。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/guest/session` | `{public_id, code}`，服务端用 `code` 换 openid 并签发宾客令牌 |
| POST | `/api/guest/bind` | `{name, phone_last4}`；匹配失败返回 `400 E_ATTENDEE_NOT_MATCHED` |
| GET | `/api/guest/checkin` | 当前签到状态 |
| POST | `/api/guest/checkin` | `{lat, lng, accuracy}` |
| POST | `/api/guest/manual-requests` | 已绑定：`{reason}`；未绑定：`{name, phone_last4, reason}`。绑定被锁定时仍可提交 |

`GET /api/guest/checkin` 同时返回当前求助的状态，未绑定的宾客也能看到处理结果。

绑定限速：手机号后四位只有 1 万种组合，姓名又容易猜到。同一 openid 在同一活动内连续匹配失败 5 次后锁定 10 分钟，返回 `429 E_TOO_MANY_ATTEMPTS`。计数复用 `login_attempts`，键为 `bind:<event_id>:<openid>`。

判定顺序：未绑定、活动非 `ready`、已签到、时间窗外、坐标非法、精度过差、围栏外、通过。已签到排在时间窗之前，窗口关闭后再点签到仍看到已签到。错误码分别为 `E_NOT_BOUND`、`E_EVENT_NOT_OPEN`、（已签到返回 `200` 和原签到时间，不写第二次成功状态）、`E_WINDOW_CLOSED`、`E_BAD_REQUEST`、`E_LOW_ACCURACY`、`E_OUT_OF_RANGE`。新增的码与 `E_ATTENDEE_NOT_MATCHED` 一并补进 `bizerr` 与 [技术方案 §7.2](../DESIGN.md#72-错误体与错误码)。

### 4.3 现场管理接口

要求宾客令牌，且 openid 在该活动 `event_staff` 中。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/guest/staff/summary` | 应到、已签到、待确认 |
| GET | `/api/guest/staff/manual-requests` | `pending` 列表 |
| POST | `/api/guest/staff/manual-requests/:id/approve` | 通过并签到，入参见下表 |
| POST | `/api/guest/staff/manual-requests/:id/reject` | 拒绝 |
| POST | `/api/guest/staff/checkins/proxy` | `{attendee_id}` 代签到 |
| GET | `/api/guest/staff/attendees?q=` | 按姓名搜索，供代签到选人 |
| PATCH | `/api/guest/staff/fence` | `{center_lat, center_lng, radius_m}`，仅 `admin`，PRD M5 |

无现场权限统一返回 `404 E_NOT_FOUND`。

通过求助时按申请类型处理，一步完成「绑定微信 + 签到」。工作人员当面处理即证明本人在现场：

| 申请 | 入参 | 结果 |
| --- | --- | --- |
| 已绑定者 | `{}` | 签到 |
| 未绑定，名单有错字 | `{attendee_id}` | 关联已有人员并绑定 openid；该人员已绑定其他 openid 时返回 `409 E_CONFLICT` |
| 未绑定，不在名单 | `{create: {name, dept, phone_last4}}` | 新增名单人员并绑定；受 `events.max_attendees` 约束 |

三种结果都写 `checkin_method = manual` 与 `checkin_by`，并删除该 openid 的绑定失败计数。

重置现场数据：`POST /api/organization/events/:id/reset`，要求 `console` 令牌，入参 `{confirm_name}` 必须等于活动名称。

| 项 | 规则 |
| --- | --- |
| 可用时间 | 仅 `checkin_start` 之前；之后返回 `409 E_CONFLICT`。这一时间点之前的数据按定义都是试跑数据 |
| 清空 | 微信绑定、签到状态、`checkin_attempts`、`manual_requests`、`guest_sessions`；阶段 7 追加中奖记录与抽奖日志 |
| 保留 | 名单、奖项、围栏、工作人员、主持人、场次扣减记录 |
| 原子性 | 同一事务完成；写运行日志，记录操作人与各表删除行数 |

组织管理员导出：名单导出追加签到状态、时间、方式与操作人；新增 `GET /api/organization/events/:id/exports/checkin-attempts`，含坐标、精度与距离。

### 4.4 小程序

- 没有活动码参数时只显示扫码说明，不调用签到
- 在公众平台「用户隐私保护指引」声明位置信息；`app.json` 已声明 `getLocation`、`chooseLocation`
- 工作人员邀请码走独立页面，扫码后先建立会话再调用 `join`
- 管理页签到进度每 10 秒轮询一次
- 宾客页显示未签到、已签到、待确认
- 绑定失败或被锁定时提示「联系现场工作人员」并提供求助表单
- 管理页处理未绑定者的求助时，先按申报姓名搜索名单，再选「关联」或「新增」
- 定位拒绝时引导打开设置
- 管理页只在 staff 接口返回成功时显示

微信 `appSecret` 只放服务端。

### 4.5 测试

| 对象 | 用例 |
| --- | --- |
| 距离 | 边界内通过、边界外拒绝、精度抵扣最多 200 米、精度超过 500 米拒绝 |
| 幂等 | 并发两次签到只有一条 `checked_in` |
| 绑定 | 第二个 openid 绑定同一名单人员被拒绝；连续失败 5 次返回 `429` |
| 邀请 | 邀请码过期或已使用被拒绝；其他活动的邀请码无效 |
| 状态 | `draft` / `closed` 活动拒绝签到 |
| 求助 | 同一 openid 不能有两条 pending；三种通过方式都得到绑定且 `manual` 签到；关联到已被他人绑定的人员返回 `409`；新增超限被拒绝；通过后绑定锁定解除 |
| 重置 | `checkin_start` 之前清空现场数据且名单保留；之后拒绝；确认名不符拒绝；重置后可退回 `draft` |
| 名单删除 | 已绑定或已签到的人员不能删除（补上阶段 5 的预留校验） |
| 权限 | 普通宾客访问 staff 接口返回 `404` |

## 5. 明确不做

- 持续定位
- 人脸、蓝牙或 Wi-Fi 校验
- 工作人员自行注册

## 6. 完成定义

- 真实 PostgreSQL 测试覆盖判定、幂等和权限
- 小程序能用活动参数完成绑定、签到和现场求助（含名单匹配不上的情形）
- API 格式、lint、测试全绿
