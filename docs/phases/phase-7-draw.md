---
title: 阶段 7：现场抽奖
type: design
status: published
updated: 2026-09-26
---

# 阶段 7：现场抽奖

## 1. 目标与范围

主持人在大屏完成抽奖。结果由服务端随机产生，先落库，再通过 SSE 展示。缺席人员可以作废并重抽。

做：

1. 主持人登录
2. 抽奖事务、一人一奖、作废与重抽
3. SSE 和刷新后的状态恢复
4. 大屏控制页与签到大屏
5. 中奖名单与抽奖日志导出
6. 奖项在有中奖记录后的修改约束、中奖人员不可删除（[阶段 5 §4.2](phase-5-event-setup.md#42-接口)）
7. 重置现场数据追加清空 `draw_results` 与 `draw_logs`（[阶段 6 §4.3](phase-6-checkin.md#43-现场管理接口)）

不做：

- 指定中奖人、权重和内定
- 订阅消息
- 离线单机抽奖页。上线方案另行交付
- 品牌装修内容。见 [阶段 8](phase-8-branding.md)

## 2. 现状与约束

| 项 | 现状 |
| --- | --- |
| 奖项 | 活动配置提供奖项名额与顺序 |
| 奖池 | 现场签到把名单写成 `checked_in` |
| 兼中 | `events.allow_multi_win` 决定同一人能否中多个奖项 |
| 随机 | `crypto/rand` |

大屏动画不参与选人。

## 3. 原则

1. 抽奖请求相同 `request_id` 重复提交返回第一次的结果；作废天然幂等，已是 `void` 时直接返回当前记录
2. 同一活动的抽奖、作废、重抽用事务咨询锁串行
3. 中奖记录只追加。作废把状态改为 `void`，不删除
4. 抽奖提交成功后才发布 SSE
5. 主持人只能操作一把 host 令牌对应的那一个活动

## 4. 方案

### 4.1 数据

新增迁移 `007_draw.sql`。

`host_users`

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| `id` | uuid | 主键 |
| `org_id` / `event_id` | uuid | `event_id` 唯一 |
| `password_hash` | text | argon2id |
| `created_at` / `updated_at` | timestamptz | |

每场活动只有一个主持人凭证，不设用户名，与 PRD「活动专属链接 + 主持人口令」一致。组织管理员用 `POST /api/organization/events/:id/host` 创建或重置，临时口令只返回一次；重置时删除该主持人全部 `host` 令牌。主持人登录按 `host:<event_public_id>` 限速。

`draw_results`

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| `id` | uuid | 主键 |
| `org_id` / `event_id` / `prize_id` / `attendee_id` | uuid | 非空 |
| `status` | varchar(16) | `valid` / `void` |
| `exclusive` | boolean | 写入时复制 `NOT events.allow_multi_win` |
| `void_reason` | varchar(200) | 可空 |
| `voided_at` | timestamptz | 可空 |
| `created_at` | timestamptz | |

部分唯一索引不能引用其他表的列，因此建两条：

| 索引 | 范围 |
| --- | --- |
| `(prize_id, attendee_id) WHERE status = 'valid'` | 始终生效：同一奖项同一人只有一条有效记录 |
| `(event_id, attendee_id) WHERE status = 'valid' AND exclusive` | 禁止兼中时：同一活动同一人只有一条有效记录 |

活动产生中奖记录后 `allow_multi_win` 不可改，保证同一活动内 `exclusive` 取值一致。

`draw_logs`：活动、奖项、奖池人数、抽取人数、操作人、`request_id`、创建时间。`request_id` 全局唯一。

### 4.2 接口

主持人登录返回 `host` 令牌，`principal_id` 保存 `host_users.id`。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/host/login` | `{event_public_id, password}` |
| GET | `/api/host/snapshot` | 奖项、剩余名额、签到人数、有效中奖记录 |
| GET | `/api/host/pool` | 当前可抽取人员的姓名与部门 |
| GET | `/api/host/stream` | SSE，事件为 `snapshot`、`draw`、`void`、`stats` |
| POST | `/api/host/draws` | `{prize_id, count, request_id}` |
| POST | `/api/host/results/:id/void` | `{reason}` |
| GET | `/api/organization/events/:id/exports/winners` | 中奖名单，作废单独标注 |
| GET | `/api/organization/events/:id/exports/draw-log` | 抽奖日志 |

SSE 约定：

- 浏览器 `EventSource` 不能设置 `Authorization` 头。大屏用 `fetch` 读取流式响应并自行解析 SSE，沿用 Bearer；不把令牌放进 URL，避免进入访问日志
- `stats` 每 5 秒推送一次签到人数与 `draw_version`，兼作心跳，满足 PRD D1 签到大屏。每个实例只为本地有连接的活动各跑一个定时器，每轮查一次库，不按连接数查询
- 响应头带 `X-Accel-Buffering: no`
- 跨实例广播走 Redis Pub/Sub：频道 `gl:event:<event_id>`。事务提交后发布 `draw` / `void` 事件；每个实例在本地出现该活动的第一个连接时订阅、最后一个连接断开时退订，收到消息后转发给本地连接。负载均衡不需要会话保持
- Pub/Sub 不保证送达（订阅断开期间的消息会丢）。`events.draw_version` 在每次抽奖或作废的事务内加 1，随 `snapshot`、`draw`、`void`、`stats` 下发。客户端发现版本跳号或与 `stats` 不一致时重拉 `snapshot`，最多 5 秒补齐
- 发布失败只记错误日志，不影响抽奖响应；发起抽奖的大屏直接使用 HTTP 响应里的结果
- `http.Server` 不设 `WriteTimeout`；关闭时用 `RegisterOnShutdown` 主动结束所有流，否则 `Shutdown` 会等到 5 秒超时

抽取在锁内完成：

1. 命中 `request_id` 时返回旧结果
2. 校验奖项剩余名额
3. 奖池取已签到人员，排除当前规则下不能再中的人，也排除该奖项已作废过的人
4. 用 `crypto/rand` 取 `count` 人
5. 写中奖记录和抽奖日志
6. `events.draw_version` 加 1，提交后发布到 Redis

活动非 `ready`、名额不足、奖池不足返回 `409 E_CONFLICT`。`count` 小于 1 返回 `400 E_BAD_REQUEST`。作废只改状态，补抽由主持人再发一次 `count = 1` 的抽奖请求。

### 4.3 大屏

`/host/:publicId` 先输入主持人口令，再进入该活动。令牌存 `gl.token.host`：

- 抽奖前展示活动小程序码与实时签到人数

- 展示剩余名额、签到人数和奖池
- 开始抽奖后只播放服务端返回的名单
- 断线显示提示，重连后先拉 `snapshot`；连接正常但版本号不一致时也重拉
- 刷新页面不重新抽奖

一次抽多人时分页展示。键盘快捷键只触发按钮，不单独发请求。

大屏按 1920 × 1080 设计，布局从一开始预留固定品牌位：左上角 Logo、顶部主标题与副标题、全屏背景层与半透明遮罩。所有颜色取自 CSS 主题变量，不写死。本阶段用平台默认外观填充，[阶段 8](phase-8-branding.md) 只替换数据，不改布局。

### 4.4 测试

| 对象 | 用例 |
| --- | --- |
| 随机 | 只从奖池中选择，且数量正确 |
| 幂等 | 同一 `request_id` 不产生第二组结果 |
| 并发 | 两个请求不能抽出超过名额的人数，也不能让同一人在禁止兼中时中两次 |
| 作废 | 原记录保留；重抽不再抽到该奖项已作废的人 |
| 兼中 | `allow_multi_win = false` 时同一人不能中两个奖项；为 true 时同一奖项仍不能中两次；有记录后改开关被拒绝 |
| 奖项 | 有中奖记录后删除或把 `quota` 改到小于有效人数被拒绝 |
| 重置 | 试跑抽奖后重置，中奖记录与日志清空，奖项名额恢复 |
| SSE | 数据库提交失败时不发布抽中事件；服务关闭时流在 5 秒内结束 |
| 多实例 | 两个实例连同一 Redis：连在实例 B 的大屏收到实例 A 产生的抽奖事件；Redis 不可用时抽奖仍成功，客户端靠 `draw_version` 补齐 |

## 5. 明确不做

- 前端本地随机兜底
- 修改已有中奖人
- 跨活动主持人账号

## 6. 完成定义

- 真实 PostgreSQL 测试覆盖幂等、并发和作废
- 大屏刷新后能恢复有效结果
- API 与 Web 的格式、lint、测试全绿
