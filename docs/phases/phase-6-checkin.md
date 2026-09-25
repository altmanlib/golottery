---
title: 阶段 6：现场签到
type: design
status: published
updated: 2026-09-25
---

# 阶段 6：现场签到

## 1. 目标与范围

宾客从活动码进入小程序，绑定名单后在地理围栏内签到。定位失败或名单匹配不上时提交现场求助，现场工作人员在小程序里处理。

做：

1. 宾客微信登录与活动上下文
2. 名单绑定
3. 按签到方式判定（`geo` 围栏判定、`direct` 直接签到）与签到流水
4. 现场求助：定位失败的人工确认，以及名单匹配不上时的关联或新增
5. 工作人员邀请与身份、签到进度、代签到、小程序内签到方式与围栏设置
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
| 签到方式 | 活动配置提供 `checkin_mode`：`geo` / `direct`，随时可切换 |
| 判定 | `geo`：距离减去最多 200 米精度后，仍小于等于半径才通过；`direct`：不判断位置 |
| 精度 | `geo` 模式下大于 500 米拒绝自动签到 |
| 限流 | 同一 openid 每分钟最多 10 次签到请求，不按 IP 限流 |
| 微信与 Redis | 阶段 5 已提供 `internal/wechat`、`internal/redisx` 与相应配置 |

签到是否通过只由服务端决定。

## 3. 原则

1. 每次签到尝试都写 `checkin_attempts`，无论成败
2. 签到更新使用条件更新，重复请求不改变第一次签到时间
3. 定位通过、直接签到与人工通过最终都写成 `checked_in`，用 `checkin_method` 区分
4. 签到方式以服务端当前配置为准；`direct` 模式不保存客户端传来的坐标
5. 工作人员权限按活动授权，不复用组织管理员的 Web 口令
6. 坐标只在导出给组织管理员时可见

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
| `checkin_method` | `geo` / `direct` / `manual` / `proxy`，可空 |
| `checkin_by` | 可空；人工通过或代签到的工作人员 openid |

`checkin_attempts` 只追加：活动、名单人员、openid、经纬度、精度、距离、结果、原因、时间。只记录宾客自己点击的签到；`direct` 模式下经纬度、精度、距离为空；人工通过与代签到没有坐标，只写 `attendees`。

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
| GET | `/api/guest/checkin` | 当前签到状态与活动当前 `checkin_mode` |
| POST | `/api/guest/checkin` | `geo`：`{lat, lng, accuracy}` 必填；`direct`：空请求体，带了坐标也忽略 |
| POST | `/api/guest/manual-requests` | 已绑定：`{reason}`；未绑定：`{name, phone_last4, reason}`。绑定被锁定时仍可提交 |

`GET /api/guest/checkin` 同时返回当前求助的状态，未绑定的宾客也能看到处理结果。

签到限流由新增的 `internal/ratelimit` 实现，计数放 Redis，多实例共享：

| 项 | 设定 |
| --- | --- |
| 算法 | 固定 60 秒窗口计数。同一个 Lua 脚本内执行 `INCR`，首次计数时设 `EXPIRE 60`，保证原子 |
| 键 | `gl:rl:checkin:<event_id>:<openid>` |
| 超限 | 返回 `429 E_TOO_MANY_ATTEMPTS`，分钟数取 1 |
| Redis 不可用 | 放行并记录错误日志，限流失效不阻塞签到 |

`internal/ratelimit` 只暴露 `Allow(ctx, key, limit, window)`，[ROADMAP](../ROADMAP.md) P-16 的按活动限流直接复用。

绑定限速：手机号后四位只有 1 万种组合，姓名又容易猜到。同一 openid 在同一活动内连续匹配失败 5 次后锁定 10 分钟，返回 `429 E_TOO_MANY_ATTEMPTS`。计数复用 `login_attempts`，键为 `bind:<event_id>:<openid>`。

判定顺序：未绑定、活动非 `ready`、已签到、时间窗外；`direct` 模式到此通过，`geo` 模式继续判断坐标非法、精度过差、围栏外，全部通过才签到。已签到排在时间窗之前，窗口关闭后再点签到仍看到已签到。错误码分别为 `E_NOT_BOUND`、`E_EVENT_NOT_OPEN`、（已签到返回 `200` 和原签到时间，不写第二次成功状态）、`E_WINDOW_CLOSED`、`E_BAD_REQUEST`、`E_LOW_ACCURACY`、`E_OUT_OF_RANGE`。新增的码与 `E_ATTENDEE_NOT_MATCHED` 一并补进 `bizerr` 与 [技术方案 §7.2](../DESIGN.md#72-错误体与错误码)。

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
| PATCH | `/api/guest/staff/checkin-settings` | `{checkin_mode, center_lat, center_lng, radius_m}`，字段可部分提交，仅 `admin`，PRD M5。切到 `geo` 时缺围栏返回 `400 E_BAD_REQUEST` |

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
- 正式版服务器域名：`request` 合法域名为 `golottery.ioclub.cn`；品牌封面启用后另将 `golottery-oss.ioclub.cn` 加入 `downloadFile` 合法域名（见 [阶段 8](phase-8-branding.md)）
- 在公众平台「用户隐私保护指引」声明位置信息；`app.json` 已声明 `getLocation`、`chooseLocation`
- 工作人员邀请码走独立页面，扫码后先建立会话再调用 `join`
- 管理页签到进度每 10 秒轮询一次
- 宾客页显示未签到、已签到、待确认
- 绑定失败或被锁定时提示「联系现场工作人员」并提供求助表单
- 管理页处理未绑定者的求助时，先按申报姓名搜索名单，再选「关联」或「新增」
- 点签到时先读取当前 `checkin_mode`：`geo` 才调用 `wx.getLocation`，`direct` 不调用也不弹授权；签到请求失败后重新读取状态，应对现场切换
- `geo` 模式下定位拒绝时引导打开设置
- 管理页对 `admin` 展示签到方式切换，切到 `direct` 前展示与控制台相同的代价提示
- 管理页只在 staff 接口返回成功时显示

活动页标题栏用 `wx.setNavigationBarTitle` 显示活动名，顶部预留封面位（无封面时显示默认样式）；颜色取自主题变量。[阶段 8](phase-8-branding.md) 只需填入品牌数据，不改页面结构。

微信 `appSecret` 只放服务端。

### 4.5 测试

| 对象 | 用例 |
| --- | --- |
| 距离 | 边界内通过、边界外拒绝、精度抵扣最多 200 米、精度超过 500 米拒绝 |
| 限流 | 同一 openid 第 11 次返回 `429`；窗口过后恢复；两个 `ratelimit` 实例共用计数；Redis 不可达时放行 |
| 直接签到 | 无坐标通过且方式为 `direct`；带坐标也不落库；未绑定、时间窗外仍被拒绝 |
| 切换 | `geo` 切 `direct` 后原先围栏外的宾客可签到；`direct` 切 `geo` 后已签到记录不变、无坐标请求被拒绝；`staff` 角色不能切换 |
| 幂等 | 并发两次签到只有一条 `checked_in` |
| 绑定 | 第二个 openid 绑定同一名单人员被拒绝；连续失败 5 次返回 `429` |
| 邀请 | 邀请码过期或已使用被拒绝；其他活动的邀请码无效 |
| 状态 | `draft` / `closed` 活动拒绝签到 |
| 求助 | 同一 openid 不能有两条 pending；三种通过方式都得到绑定且 `manual` 签到；关联到已被他人绑定的人员返回 `409`；新增超限被拒绝；通过后绑定锁定解除 |
| 重置 | `checkin_start` 之前清空现场数据且名单保留；之后拒绝；确认名不符拒绝；重置后可退回 `draft` |
| 名单删除 | 已绑定或已签到的人员不能删除（补上阶段 5 的预留校验） |
| 权限 | 普通宾客访问 staff 接口返回 `404` |

### 4.6 首发的网页宾客端

首发不等小程序上线：由 `golottery-web` 里的手机网页扮演小程序客户端，后端接口按本方案实现，不另开一套。小程序接入登记为 [ROADMAP P-25](../ROADMAP.md)，届时只换客户端与登录方式。

| 项 | 网页宾客端 |
| --- | --- |
| 入口 | `/m/:publicId`；活动码与路径同时提供网页地址，印刷物使用网页地址的二维码 |
| 登录 | 新增 ScopeInfra 配置 `GUEST_LOGIN_MODE`：`wechat`（默认，`code` 换 openid）或 `web`。`web` 模式下 `POST /api/guest/session` 接受 `{public_id, device_id}`，`device_id` 由浏览器生成并存在本地，服务端以 `web:<device_id>` 作为 openid。两种模式不同时开启 |
| 坐标 | 浏览器定位为 WGS-84。签到请求增加 `coord_type`（`gcj02` 默认 / `wgs84`），服务端先换算成 GCJ-02 再判定与落库 |
| 绑定限速 | 网页身份可以随意更换，按 openid 的锁定挡不住穷举。`web` 模式另按客户端 IP 限速：同一活动同一 IP 每 10 分钟最多 20 次绑定失败，超出返回 `429 E_TOO_MANY_ATTEMPTS`，计数放 Redis，复用 `internal/ratelimit` |
| 工作人员 | 邀请、现场求助处理、代签到、签到进度与签到方式切换做成网页，路由 `/m/:publicId/staff`；邀请以网页链接发出 |
| 定位权限 | 浏览器定位要求 HTTPS；被拒绝时引导使用现场求助 |

网页身份没有微信背书，同一个人换浏览器即是新身份，绑定仍是「名单 + 后四位」一次性占用：一个名单人员只能被一个身份绑定，冒名者抢先绑定时，本人通过现场求助由工作人员处理。

## 5. 明确不做

- 持续定位
- 人脸、蓝牙或 Wi-Fi 校验
- 工作人员自行注册

## 6. 完成定义

- 真实 PostgreSQL 测试覆盖判定、幂等和权限
- 网页宾客端能用活动参数完成绑定、签到和现场求助（含名单匹配不上的情形）；工作人员网页能处理求助与代签到
- 小程序页面与真机验证移到 [ROADMAP P-25](../ROADMAP.md)，不阻塞本阶段
- API 格式、lint、测试全绿

## 7. 开放项

| 问题 | 现状 | 需要在哪个阶段前定 |
| --- | --- | --- |
| OpenAPI 对外冻结前评审 | 契约版本 `0.1.0`，web 与 API 同步部署可随时调整；小程序一经提审，旧版本会在用户手机上长期存在，接口改名、字段语义变化都会变成不兼容改动。见 [ROADMAP P-23](../ROADMAP.md) | 小程序首次提审前（[ROADMAP P-25](../ROADMAP.md)）。首发客户端是与 API 同步部署的网页，接口仍可调整。评审路由与字段命名、错误码、分页参数、默认值语义、可空性、枚举取值，收回不该对外的内部开关；结论写回 [DESIGN.md](../DESIGN.md)，并提升 `info.version` |
