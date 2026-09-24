---
title: 后端 API · 模块设计
type: design
status: draft
updated: 2026-09-23
---

# 后端 API · 模块设计

本文把 [DESIGN.md](DESIGN.md) 的整体方案拆到包级：每个模块的职责边界、对外接口、依赖方向、事务与并发约束、错误产出。需求口径见 [PRD.md](PRD.md)，待办登记见 [ROADMAP.md](ROADMAP.md)。

## 1. 目标与范围

给出可直接开工的包结构与接口签名，使多人并行开发时模块边界不靠口头约定。

覆盖：包划分与依赖规则、12 个业务与基础设施模块的契约、错误模型、事务边界、迁移与配置。不覆盖：具体 SQL 语句、`api/openapi.yaml` 的逐字段定义（契约以该文件为准）。

## 2. 现状与约束

| 项 | 现状 |
|---|---|
| 代码 | M0 脚手架：`config` / `db` / `handler` 可用，`model` / `service` / `repository` 仅有 `doc.go` |
| 契约 | `api/openapi.yaml` 仅 4 个系统端点（`/healthz`、`/api`、`/openapi.{yaml,json}`） |
| 依赖 | Echo v4、GORM + pgx、kin-openapi；无 Redis、无消息队列 |
| 部署 | 单实例 Docker Compose + Nginx，SSE 关缓冲 |

约束来自 DESIGN §2：服务端裁决围栏与抽奖；写接口幂等；所有查询强制带 `org_id` / `event_id`。单实例是本阶段的硬前提，SSE 广播与限流都按进程内实现。

## 3. 原则

1. **依赖单向**：`handler → service → repository → db`，反向引用禁止
2. **DTO 不下沉**：`api.*` 生成类型只出现在 `handler`；`service` 只认 `model`
3. **实体不上浮**：数据库实体不直接作为响应体，字段映射写在 `handler`
4. **规则集中**：围栏判定、奖池构造、配额扣减只有一处实现，不在 handler 里复制条件判断
5. **副作用后置**：SSE 推送、审计落盘都在事务提交之后，事务内只做数据库读写
6. **基础设施无业务**：`internal/platform` 下的包不 import 任何业务包

## 4. 包结构

```text
internal/
  config/                 环境变量装配（扩展：微信、JWT TTL、围栏默认值、限流、保留期）
  db/                     连接、事务管理器、迁移执行
    migrations/           嵌入式 SQL 迁移
  handler/                HTTP 适配层，实现 api.StrictServerInterface
    middleware/           JWT 解析、身份与租户上下文、限流、SSE 心跳
  service/                业务规则
    auth/ event/ attendee/ checkin/ lottery/ quota/ export/
  repository/             持久化实现，按聚合分文件
  model/                  领域模型、枚举、领域错误
  platform/               无业务依赖的基础能力
    geo/ idgen/ token/ passwd/ random/ sse/ ratelimit/ sheet/
```

`handler` 按 OpenAPI tag 分文件（`system.go` / `auth.go` / `attendee.go` / `checkin.go` / `host.go` / `console.go` / `platform.go`），与 `api/openapi.yaml` 的分组一一对应，避免单文件膨胀。

依赖规则用 `depguard` 固化：`internal/service/**` 禁止 import `github.com/labstack/echo/v4` 与 `golottery/api/api`；`internal/platform/**` 禁止 import `golottery/api/internal/{model,service,repository,handler}`。`.golangci.yml` 当前为 `default: none` 且只启用 4 个 linter，落地 API-001 时需一并启用 `depguard` 并写入上述规则，否则依赖方向只是文字约定。

## 5. 领域模型与错误

### 5.1 模型分层

`model` 只放领域结构与规则常量，不带 GORM 标签；持久化结构体放 `repository`，两者在 repository 内转换。理由：`attendee` 等实体在签到与抽奖路径上读写形态差异大，共用一个带标签的结构会把存储细节泄到服务层。代价是多一层转换函数，字段数可控（单表均在 15 列内），可接受。

聚合与归属模块：

| 聚合 | 主要实体 | 归属模块 |
|---|---|---|
| 组织 | `Org`、`OrgQuota`、`OrgUser`、`CreditLedger` | quota |
| 活动 | `Event`、`EventMember`、`HostUser`、`Prize` | event |
| 参会人 | `Attendee` | attendee |
| 签到 | `CheckinAttempt`、`ManualRequest` | checkin |
| 抽奖 | `DrawResult`、`DrawLog` | lottery |

所有实体带 `bigint generated always as identity` 内部主键。`public_id`（nanoid 21 字符）只给 DESIGN §3 列举的 `event`、`event_member`、`host_user`、`platform_user`，用于活动码等不可枚举入口；其余实体（`attendee`、`prize`、`draw_result` 等）不设 `public_id`，对外用内部 ID，且只在已鉴权的活动上下文内有效 —— 它们的访问始终经过 `event_id` scope 过滤，枚举 ID 无法跨活动取数。

### 5.2 领域错误

`model/errors.go` 定义结构化错误类型（不是 `errors.New` 哨兵值，因为错误码、HTTP 状态与消息需要随错误一起传递），`service` 返回，`handler` 统一映射：

```go
type Error struct {
    Code    string // 对外错误码，进 api.Error.code
    Message string
    Status  int    // HTTP 状态
    cause   error
}

func (e *Error) Error() string { return e.Message }
func (e *Error) Unwrap() error { return e.cause }
```

| 错误码 | 状态 | 产出模块 | 触发条件 |
|---|---|---|---|
| `UNAUTHENTICATED` | 401 | auth | 无 JWT、签名或有效期失败 |
| `FORBIDDEN_EVENT` | 403 | auth | JWT 中 `event_id` / `org_id` 与目标资源不符 |
| `NOT_BOUND` | 409 | checkin | openid 未绑定本活动名单 |
| `WINDOW_CLOSED` | 409 | checkin | 不在签到时间窗内 |
| `LOW_ACCURACY` | 422 | checkin | 定位精度 > 500 米 |
| `OUT_OF_RANGE` | 422 | checkin | `d - min(accuracy, 200m) > radius` |
| `PRIZE_LOCKED` | 409 | event | 奖项已有中奖结果，名额下调被拒 |
| `POOL_EMPTY` | 409 | lottery | 本轮奖池无可抽人员 |
| `PRIZE_EXHAUSTED` | 409 | lottery | 奖项剩余名额为 0 |
| `QUOTA_EXHAUSTED` | 409 | quota | 场次配额不足 |
| `IMPORT_INVALID` | 422 | attendee | 名单格式错误或超出人数档位 |
| `RATE_LIMITED` | 429 | middleware | 触发 openid / 账号维度限流 |

重复签到不进本表：它不是错误，签到接口对重复请求返回 200 与既有签到状态、签到时间，由响应体的 `status` 字段区分首次与重复，保证弱网重试幂等。

本表中各错误码的 HTTP 状态是本文新增决策，DESIGN §5 只规定了错误码名称。定稿前若与小程序、Web 端的处理分支冲突，以本表为准并回写 DESIGN。

`handler/errors.go` 提供唯一映射函数，把 `*model.Error` 转为 `api.Error`，并填入 Echo `RequestID` 中间件生成的 `requestId`；非 `*model.Error` 一律记日志后返回 `INTERNAL_ERROR` 500，不把内部细节透传给客户端。

## 6. 模块契约

每个模块给出对外接口与关键约束。接口定义在调用方一侧（`service` 定义所需的 repository 接口），实现放 `repository`，便于用内存实现做服务层单测。

### 6.1 auth

职责：微信 `code2session`、四类凭证登录、JWT 签发与解析、活动上下文建立。

```go
type Service interface {
    EventContext(ctx context.Context, publicID string) (model.EventContext, error)
    GuestLogin(ctx context.Context, eventPublicID, wxCode string) (model.Session, error)
    ConsoleLogin(ctx context.Context, email, password string) (model.Session, error)
    HostLogin(ctx context.Context, eventPublicID, secret string) (model.Session, error)
    PlatformLogin(ctx context.Context, email, password string) (model.Session, error)
}
```

- JWT 载荷：`sub`、`role`（`guest` / `staff` / `console` / `host` / `platform`）、`org_id`、`event_id`、`openid`、`exp`、`jti`
- 口令用 bcrypt（cost 10）；微信 `code2session` 调用超时 3 秒，失败返回 `UNAUTHENTICATED` 并记日志
- 令牌有效期：guest / host 12 小时（覆盖单场活动），console / platform 2 小时
- 工作人员权限不进 JWT：`role=staff` 只表示身份，具体活动授权每次查 `event_member`，避免授权变更后令牌未过期仍可操作

### 6.2 middleware

职责：把 JWT 解出的身份放进 `context`，并在进入 handler 前完成租户与限流校验。

| 中间件 | 作用 |
|---|---|
| `RequireRole(roles...)` | 校验 `role`，不符返回 `UNAUTHENTICATED` / `FORBIDDEN_EVENT` |
| `TenantScope` | 把 `org_id` / `event_id` / `openid` 写入 `context`，供 repository 强制过滤 |
| `RateLimit(keyFn, rate)` | 进程内令牌桶，按 openid 计：签到 10 次/分；绑定 5 次/分，且绑定连续失败 5 次锁定 10 分钟 |

租户值只从 `context` 读取，禁止从请求体或查询参数取 `org_id` / `event_id`。repository 的每个查询方法都要求显式传入 scope 参数，由 `service` 从 `context` 取出后传递 —— 强制在编译期出现，而不是依赖 GORM 全局钩子。

### 6.3 event

职责：活动 CRUD、围栏与时间窗配置、活动码与短链、奖项配置、活动成员授权。

```go
type Service interface {
    Update(ctx context.Context, scope model.OrgScope, id int64, in model.EventInput) (model.Event, error)
    SetGeofence(ctx context.Context, scope model.OrgScope, id int64, g model.Geofence) error
    ListPrizes(ctx context.Context, scope model.EventScope) ([]model.Prize, error)
    UpsertPrizes(ctx context.Context, scope model.EventScope, in []model.PrizeInput) error
}
```

- 围栏半径校验 100~1000 米，保存即生效；活动进行中修改写审计
- 本接口不含 `Create`：创建活动必须与配额扣减同事务，入口统一在 quota 的 `CreateEventWithCredit`（§6.8），event 模块只对外提供事务内的插入函数 `InsertTx(ctx, orgID, in)` 供其调用。保留两个创建入口会让配额被绕过
- 奖项已有中奖结果时，名额只允许上调；下调返回 `PRIZE_LOCKED`

### 6.4 attendee

职责：名单导入与校验、姓名 + 手机号后四位绑定、导出。

```go
type Service interface {
    Import(ctx context.Context, scope model.EventScope, r io.Reader) (model.ImportReport, error)
    Bind(ctx context.Context, scope model.EventScope, openid, name, phoneLast4 string) (model.Attendee, error)
    Get(ctx context.Context, scope model.EventScope, openid string) (model.Attendee, error)
}
```

- 导入用 excelize 流式读取，逐行校验并累积 `ImportReport{Total, Inserted, Skipped, Errors[]}`，整批在一个事务内提交；行数超过套餐人数上限直接拒绝
- 绑定靠数据库唯一约束兜底：`(event_id, openid)` 与 `(event_id, name, phone_last4)` 均唯一，冲突时转换为业务错误而非 500
- 绑定失败计数由 middleware 的限流器承担，attendee 模块不自行计数

### 6.5 geo

职责：纯函数，无状态。

```go
func Distance(a, b Point) float64                      // Haversine，米
func Within(d, accuracy, radius float64) bool          // d - min(accuracy, 200) <= radius
func ValidPoint(p Point, accuracy float64) bool        // 排除 (0,0)、越界经纬度、负精度
```

放 `platform/geo`，不依赖任何业务包，边界值单测覆盖：恰好等于半径、精度上限 500、精度截断 200、跨 180 度经线。

### 6.6 checkin

职责：签到主路径与人工确认，是并发最密集的模块。

```go
type Service interface {
    Status(ctx context.Context, scope model.EventScope, openid string) (model.CheckinStatus, error)
    Checkin(ctx context.Context, scope model.EventScope, in model.CheckinInput) (model.CheckinResult, error)
    RequestManual(ctx context.Context, scope model.EventScope, in model.ManualInput) (model.ManualRequest, error)
    ResolveManual(ctx context.Context, scope model.EventScope, operatorID, reqID int64, approve bool) error
}
```

执行顺序按 DESIGN §5，固定为：绑定检查 → 时间窗 → 坐标合法性 → 已签到短路 → 距离判定 → 落盘。

- **无论成败都写 `checkin_attempt`**，且写入不受主路径失败影响；判定失败时 attempt 记录 `reason` 后正常返回业务错误
- 置为已签到用条件更新 `UPDATE attendee SET status='checked_in', checked_in_at=now() WHERE id=? AND status='pending'`，`RowsAffected=0` 表示并发下已被置位，按重复签到返回
- 事务只包含 attempt 插入与 attendee 更新；SSE 人数事件在提交后发送，且由 sse 模块做每秒至多一次的合并
- 人工确认：每人最多一条 pending 申请，用 `(event_id, attendee_id) WHERE status='pending'` 的部分唯一索引保证；通过后记录处理人与 `method='manual'`

### 6.7 lottery

职责：奖池快照、抽取、结果落库、作废补抽，要求强幂等与强串行。

```go
type Service interface {
    Pool(ctx context.Context, scope model.EventScope, prizeID int64) (model.Pool, error)
    Draw(ctx context.Context, scope model.EventScope, in model.DrawInput) (model.DrawRound, error)
    Void(ctx context.Context, scope model.EventScope, resultID int64, operatorID int64) error
    Results(ctx context.Context, scope model.EventScope) ([]model.DrawResult, error)
}
```

`Draw` 的事务内步骤：

1. 按 `request_id` 查 `draw_log`，命中则直接返回既有结果（幂等出口）
2. `SELECT pg_advisory_xact_lock(event_id)` 串行化本活动的抽奖
3. 校验奖项余量；构造奖池：已签到（含人工确认）∩ 满足中奖约束；`allow_multi_win=false` 时排除全部已中奖者，为 true 时仅排除本奖项已中奖者；补抽额外排除本奖项已作废者
4. 用 `crypto/rand` 做部分 Fisher-Yates 洗牌取前 N 人（`platform/random`）
5. 写 `draw_result` 与 `draw_log`（`request_id` 唯一）

提交后发送 `draw` SSE。唯一约束是最后一道防线：并发双窗口即使绕过 advisory lock，也会在有效中奖唯一索引上失败。作废只置 `voided_at` 与操作人，不删除记录；已确认结果不提供直接改写接口。

### 6.8 quota

职责：组织配额、场次增减流水，唯一拥有跨聚合事务的模块。

```go
type Service interface {
    Balance(ctx context.Context, orgID int64) (model.Quota, error)
    CreateEventWithCredit(ctx context.Context, orgID int64, in model.EventInput) (model.Event, error)
    Adjust(ctx context.Context, orgID int64, delta int, operatorID int64, memo string) error
}
```

`CreateEventWithCredit` 在一个事务内：`SELECT ... FOR UPDATE` 锁 `org_quota` → 校验余量 → 扣减 → 插入 `event` → 写 `credit_ledger`。余量不足返回 `QUOTA_EXHAUSTED`。活动取消不自动退还，由运营通过 `Adjust` 手工调账并留痕。

### 6.9 sse

职责：进程内事件广播，`platform/sse` 提供通用 Hub，`handler/host.go` 负责 HTTP 协议细节。

```go
type Hub interface {
    Subscribe(topic string, lastID string) (<-chan Event, func())
    Publish(topic string, ev Event)
}
```

- topic 为 `event:<event_id>`；事件类型 `stats` / `draw` / `void`
- 每 topic 保留最近 50 条事件的环形缓冲，客户端带 `Last-Event-ID` 重连时补发，缺口超出缓冲则返回提示客户端全量拉取
- 订阅者 channel 带缓冲 16，写阻塞即丢弃该订阅并关闭连接，防止慢客户端拖垮广播
- 心跳 15 秒一次注释行；`stats` 事件在 Hub 内按 topic 做 1 秒合并
- 大屏的正确性不依赖 SSE：断线后用 `/api/host/results` 与统计接口可重建完整状态

### 6.10 export

职责：签到名单、尝试明细、中奖名单、抽奖日志导出。

- 统一 `Export(ctx, scope, kind) (io.Reader, filename, error)`，用 excelize 流式写入，避免大名单全量驻留内存
- 坐标明细仅 `console` 角色可导出，`staff` 与 `host` 请求返回 403
- 导出动作记审计（操作人、类型、时间）

### 6.11 retention

职责：活动结束后按保留策略清理原始坐标，保留距离与判定结果。

进程内 `time.Ticker` 每日执行一次，用 `pg_advisory_lock` 做任务互斥（为多实例预留）。默认 30 天可由配置或组织设置缩短。清理只置空 `lat` / `lng` 字段，不删除 attempt 行，保证审计链完整。

### 6.12 platform 基础包

| 包 | 内容 |
|---|---|
| `idgen` | nanoid 21 字符 `public_id`；字符集排除易混字符 |
| `token` | JWT 签发与解析，密钥来自配置，禁止硬编码 |
| `passwd` | bcrypt 包装，统一 cost |
| `random` | `crypto/rand` 洗牌与抽样，注入接口以便测试确定化 |
| `ratelimit` | 令牌桶 + LRU 淘汰，键为 openid 或账号 |
| `sheet` | excelize 读写包装，统一表头与时间格式 |

## 7. 事务与时间

事务管理器放 `db`，通过 `context` 传递会话，repository 不感知自己是否在事务中：

```go
type TxManager interface {
    InTx(ctx context.Context, fn func(ctx context.Context) error) error
}
```

`repository` 内部统一用 `db.From(ctx)` 取 `*gorm.DB`：在事务内返回事务句柄，否则返回根连接。事务边界只由 `service` 划定，`handler` 不得开启事务。

时间统一走注入的 `Clock` 接口（默认 `time.Now().UTC()`），签到时间窗与保留期测试据此确定化。数据库时间列一律 `timestamptz`。

## 8. 迁移与配置

### 8.1 迁移

用 goose + 嵌入式 SQL（`internal/db/migrations/*.sql`，`embed.FS`），而非 GORM `AutoMigrate`。理由：本项目依赖部分唯一索引（pending 申请唯一、有效中奖唯一）、`generated always as identity` 与 advisory lock 相关约束，`AutoMigrate` 无法表达且会在字段变更时产生不可预期的 DDL。代价是每次改表要手写 SQL，在表数量约 15 张、变更频率低的前提下可接受。触发重新评估的条件：需要支持多套租户库并行升级。

迁移在进程启动时执行并加 advisory lock，失败即退出，不允许带着未完成迁移提供服务。

### 8.2 配置扩展

`config.Config` 在现有三项外增加，全部有默认值，缺失不阻断启动，但生产必填项（`JWT_SECRET`、微信凭证）在非开发环境校验为空时直接退出：

| 变量 | 默认 | 用途 |
|---|---|---|
| `WECHAT_APPID` / `WECHAT_SECRET` | 空 | code2session |
| `JWT_GUEST_TTL` / `JWT_CONSOLE_TTL` | `12h` / `2h` | 令牌有效期 |
| `GEOFENCE_DEFAULT_RADIUS` | `400` | 默认围栏半径（米） |
| `CHECKIN_RATE_PER_MIN` | `10` | 按 openid 限流 |
| `LOCATION_RETENTION_DAYS` | `30` | 坐标保留期 |
| `APP_ENV` | `dev` | 控制必填项校验与日志级别 |

## 9. 测试策略

| 层次 | 对象 | 方式 |
|---|---|---|
| 单元 | `geo`、`random`、错误映射、时间窗判定 | 纯函数表驱动，无数据库 |
| 服务 | checkin、lottery、quota 规则 | repository 接口用内存实现替身，注入固定 Clock 与随机源 |
| 集成 | 租户隔离、幂等、并发抽奖、迁移 | 真实 Postgres（compose 实例），每用例独立 schema |
| 契约 | `api/*.gen.go` 与 `openapi.yaml` 同步 | `go generate ./...` 后 `git diff --exit-code` |

并发抽奖用例：同一 `request_id` 并发 20 次只应产生一条 `draw_log`；不同 `request_id` 并发抽同一奖项，有效中奖总数不得超过奖项名额。

## 10. 影响面与落地顺序

本文不改变已交付行为，`/healthz` 等 4 个端点保持不变。新增模块均为增量；`config`、`db` 会扩展但保持现有字段与默认值兼容。

落地顺序按依赖排列，每步可独立交付并有可验证出口：

| 步骤 | 内容 | 出口 |
|---|---|---|
| 1 | 迁移框架 + 全部建表 + 领域错误与错误映射 + 启用 depguard | 迁移可重复执行，未完成迁移时进程拒绝启动；不动 `api/openapi.yaml` |
| 2 | auth + middleware + 租户 scope | 四类登录可用，跨租户访问被拒 |
| 3 | event + quota | 配额扣减与活动创建在同一事务 |
| 4 | attendee 导入与绑定 | 导入报告可返回，重复绑定被约束拦截 |
| 5 | geo + checkin + 人工确认 | 距离边界单测通过，重复签到幂等 |
| 6 | lottery + sse | 并发抽奖用例通过，大屏可断线重建 |
| 7 | export + retention | 导出鉴权生效，坐标按期清理 |

## 11. 明确不做

| 项 | 原因 |
|---|---|
| Redis / 消息队列 | 单实例下进程内 Hub 与令牌桶已满足 100 req/s 目标（推断，依据 DESIGN §2 的部署形态），引入即增加运维面 |
| 多实例 SSE 广播 | 需要外部 broker，与「不引入消息基础设施」冲突；触发条件：单实例无法承载并发连接 |
| 组织独立分库 | 首发为共享库，分库会让迁移与配额统计成本陡增 |
| GORM 全局租户钩子 | 隐式过滤在遗漏 scope 时静默放行，改为显式传参以便编译期暴露 |
| 在线支付与发票 | PRD 列为 P2 |
| 指定中奖人 | PRD 明确排除，抽取结果只由服务端随机源决定 |

## 12. 开放项

| 编号 | 问题 | 现状 |
|---|---|---|
| O-1 | 工作人员授权的粒度（活动级 vs 操作级） | 当前按活动级授权，若需区分「可代签」与「可改围栏」需扩展 `event_member` |
| O-2 | 短链服务归属（自建 vs 复用 Web 端） | 未定，影响 event 模块是否需要额外表 |
| O-3 | 压测目标未实测 | DESIGN §7 的 P95 ≤ 1 秒为目标值，需在步骤 5 完成后补压测 |
| O-4 | host 凭证的轮换方式 | 活动级 secret 泄露后的失效路径未定义 |
