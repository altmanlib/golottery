---
title: 后端 API · 技术方案
type: design
status: draft
updated: 2026-09-21
---

# 后端 API · 技术方案

## 1. 目标与原则

后端以 Go + Echo + GORM + PostgreSQL 实现多租户活动引擎与薄配额层。需求见 [PRD.md](PRD.md)。围栏与抽奖由服务端裁决；写接口幂等，结果先提交数据库再推送；所有查询带组织/活动范围，入口隐藏不能替代鉴权。

## 2. 契约与组件

- `api/openapi.yaml` 是接口契约唯一来源；`models.yaml` / `server.yaml` 生成 `api/*.gen.go`，使用 `go generate ./...`
- 后端负责 JWT、微信 code2session、活动会话、名单绑定、签到、人工确认、抽奖、SSE 与导出
- Excel 使用 excelize；Web 可从运行中 API 的 OpenAPI JSON 生成客户端；数据库为 PostgreSQL
- 共享实例部署使用 Docker Compose + Nginx；SSE 反向代理关闭缓冲，发送 15 秒心跳

## 3. 租户、凭证与数据

| 身份 | 认证与范围 |
| --- | --- |
| 宾客/工作人员 | 微信 `code` 换 openid；JWT 携带当前 `event_id`；工作人员操作再查活动成员授权 |
| 组织管理员 | 邮箱口令登录；JWT 携带 `org_id` |
| 主持人 | 活动级 host 登录；JWT 携带 `event_id` 和 host 角色 |
| 平台运营 | 平台账号登录；运营组织与配额 |

活动码携带不可枚举 `event_public_id`，入场时建立活动上下文；业务数据访问层强制使用 `org_id` / `event_id`。按 openid 限流签到每分钟 ≤ 10 次；绑定每分钟 ≤ 5 次、连续错误 5 次锁定 10 分钟。

| 数据 | 关键约束 |
| --- | --- |
| org、org_quota、org_user、credit_ledger | 组织配额、邮箱账号及场次增减流水 |
| event、event_member、host_user、platform_user | 唯一 `public_id`、活动成员及分端凭证 |
| attendee | 唯一 `(event_id, openid)` 与 `(event_id, name, phone_last4)` |
| checkin_attempt、manual_request | 签到尝试只追加；每人最多一条 pending 申请 |
| prize、draw_result、draw_log | 有效中奖唯一约束；`draw_log.request_id` 唯一 |

`allow_multi_win` 为 true 时，中奖唯一约束按活动、奖项、人员生效；`public_id` 使用 nanoid 21 字符。口令使用 bcrypt；密钥仅留服务端，备份不进仓库。

## 4. 接口分组

统一 JSON 错误 `{ code, message }`；以下为方案草案，实施时以 `api/openapi.yaml` 为准。

| 分组 | 主要操作 |
| --- | --- |
| 活动与身份 | `POST /api/auth/event-context`、`/api/auth/login`、`/api/console/login`、`/api/host/login`、`/api/platform/login` |
| 宾客 | `POST /api/attendee/bind`、`GET /api/checkin/status`、`POST /api/checkin`、`POST /api/checkin/manual-request`、`GET /api/prize/my` |
| 现场管理 | `/api/admin/stats`、`/api/admin/manual-requests`、`/api/admin/checkin/proxy`、`/api/admin/event/geofence` |
| 主持人 | `/api/host/stream`、`/api/host/pool`、`/api/host/draw`、`/api/host/results`、`/api/host/redraw` |
| 控制台 | `/api/console/quota`、`/api/console/events`、`/api/console/attendees/import`、`/api/console/prizes`、`/api/console/export/:type` |
| 运营 | `/api/platform/orgs`、`/api/platform/orgs/:id/credits`、`/api/platform/usage` |

## 5. 签到路径

1. 从 JWT 获取 openid 和活动，查已绑定参会人；未绑定返回 `NOT_BOUND`
2. 校验活动签到时间窗、有限经纬度、排除 (0,0) 且精度非负；已签到返回既有签到时间
3. Haversine 计算距离 `d`；精度超过 500 米返回 `LOW_ACCURACY`，否则 `d - min(accuracy, 200m) ≤ radius` 时通过，超出返回 `OUT_OF_RANGE`
4. 无论成败写 `checkin_attempt`；通过时按 `event_id` 和 pending 状态条件更新参会人，成功后推送人数事件

窗口外返回 `WINDOW_CLOSED`，活动越权返回 `FORBIDDEN_EVENT`。GCJ-02 坐标由客户端上报；人工确认由授权工作人员处理，记录操作人与签到方式。

## 6. 抽奖、事件与配额

- 以 `requestId` 查询既有抽奖日志实现幂等；事务中用 `pg_advisory_xact_lock(event_id)` 串行化，校验奖项余量
- 从已签到且符合中奖约束的人员中生成本轮奖池；补抽排除该奖项已作废者；使用 `crypto/rand` 部分洗牌选择人员
- 同一事务写 `draw_result` 与 `draw_log`；提交后发送 `draw` SSE。`stats` 至多每秒一次，作废发送 `void`
- 断线后重新拉取统计及中奖结果；数据库唯一约束兜底重复中奖
- 创建活动的事务同时检查并扣减场次配额、插入活动与配额流水；取消不自动退还，由运营手工调账

## 7. 安全与验证

- 入参校验字段与导入行数；跨组织/跨活动读写必须拒绝；人工确认、代签、作废、改围栏和改配额均记录操作者
- 位置只在签到时采集；尝试明细仅管理员可导出；活动结束默认 30 天后清除原始坐标，保留距离与结果
- 单测覆盖距离边界、精度上限与抽奖随机逻辑；集成测试覆盖租户隔离、幂等和并发抽奖
- 压测目标：单场 800 人/10 分钟及 100 req/s 冲击，签到 P95 ≤ 1 秒、错误率 < 1%（目标，尚未实测）

## 8. 落地范围

先建多租户表与演示组织，再做名单/签到、抽奖/SSE、配额与导出。仓库尚无生产数据，不提供无组织单场模式。不引入独立小程序、独立租户库、Redis/MQ 或在线支付；多实例 SSE 和限流无法满足时再评估消息基础设施
