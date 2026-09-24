---
title: 现场抽奖
type: design
status: published
updated: 2026-09-24
---

# 现场抽奖

## 1. 目标与范围

主持人在大屏完成抽奖。结果由服务端随机产生，先落库，再通过 SSE 展示。缺席人员可以作废并重抽。

做：

1. 主持人登录
2. 抽奖事务、一人一奖、作废与重抽
3. SSE 和刷新后的状态恢复
4. 大屏控制页
5. 抽奖日志导出

不做：

- 指定中奖人、权重和内定
- 订阅消息
- 离线单机抽奖页。上线方案另行交付
- 品牌主题色

## 2. 现状与约束

| 项 | 现状 |
| --- | --- |
| 奖项 | 活动配置提供奖项名额与顺序 |
| 奖池 | 现场签到把名单写成 `checked_in` |
| 兼中 | `events.allow_multi_win` 决定同一人能否中多个奖项 |
| 随机 | `crypto/rand` |

大屏动画不参与选人。

## 3. 原则

1. 相同 `request_id` 重复提交返回第一次的结果
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
| `username` | varchar(64) | 活动内唯一 |
| `password_hash` | text | argon2id |

组织管理员在活动详情创建或重置主持人，临时口令只返回一次。

`draw_results`

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| `id` | uuid | 主键 |
| `org_id` / `event_id` / `prize_id` / `attendee_id` | uuid | 非空 |
| `status` | varchar(16) | `valid` / `void` |
| `void_reason` | varchar(200) | 可空 |
| `voided_at` | timestamptz | 可空 |
| `created_at` | timestamptz | |

`allow_multi_win = false` 时，部分唯一索引保证同一人只有一条 `valid`。为 true 时，唯一范围改为同一奖项下的同一人。

`draw_logs`：活动、奖项、奖池人数、抽取人数、操作人、`request_id`、创建时间。`request_id` 全局唯一。

### 4.2 接口

主持人登录返回 `host` 令牌，`principal_id` 保存 `host_users.id`。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/api/host/login` | `{event_public_id, username, password}` |
| GET | `/api/host/snapshot` | 奖项、剩余名额、签到人数、有效中奖记录 |
| GET | `/api/host/pool` | 当前可抽取人员的姓名与部门 |
| GET | `/api/host/stream` | SSE，事件为 `snapshot`、`draw`、`void`，15 秒心跳 |
| POST | `/api/host/draws` | `{prize_id, count, request_id}` |
| POST | `/api/host/results/:id/void` | `{reason, request_id}` |
| GET | `/api/organization/events/:id/exports/draw-log` | 组织管理员导出日志 |

抽取在锁内完成：

1. 命中 `request_id` 时返回旧结果
2. 校验奖项剩余名额
3. 奖池取已签到人员，排除当前规则下不能再中的人，也排除该奖项已作废过的人
4. 用 `crypto/rand` 取 `count` 人
5. 写中奖记录和抽奖日志
6. 提交后发 SSE

名额不足、奖池不足返回 `409 E_CONFLICT`。`count` 小于 1 返回 `400 E_BAD_REQUEST`。

### 4.3 大屏

`/host` 先登录，再进入单一活动：

- 展示剩余名额、签到人数和奖池
- 开始抽奖后只播放服务端返回的名单
- 断线显示提示，重连后先拉 `snapshot`
- 刷新页面不重新抽奖

一次抽多人时分页展示。键盘快捷键只触发按钮，不单独发请求。

### 4.4 测试

| 对象 | 用例 |
| --- | --- |
| 随机 | 只从奖池中选择，且数量正确 |
| 幂等 | 同一 `request_id` 不产生第二组结果 |
| 并发 | 两个请求不能抽出超过名额的人数，也不能让同一人在禁止兼中时中两次 |
| 作废 | 原记录保留；重抽不再抽到该奖项已作废的人 |
| SSE | 数据库提交失败时不发送抽中事件 |

## 5. 明确不做

- 前端本地随机兜底
- 修改已有中奖人
- 跨活动主持人账号

## 6. 完成定义

- 真实 PostgreSQL 测试覆盖幂等、并发和作废
- 大屏刷新后能恢复有效结果
- API 与 Web 的格式、lint、测试全绿
