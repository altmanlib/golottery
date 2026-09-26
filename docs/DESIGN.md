---
title: 定位签到抽奖 · 技术方案
type: design
status: published
updated: 2026-09-26
---

# 定位签到抽奖 · 技术方案

## 1. 目标与范围

把「定位围栏签到 + 服务端抽奖」做成可复用的活动引擎，并加上薄多租户壳。产品需求见 [PRD.md](PRD.md)。

**范围内**

- 共享部署下的组织 / 活动行级隔离
- 平台单小程序 + 活动码入场
- Web 控制台、大屏页、签到与抽奖核心链路
- 手动开通场次配额（在线支付后置）

**范围外**

- 微信第三方平台 / 独立小程序白标
- 按组织分库分实例
- 会务套件（票务、议程、直播）
- 自动计费与开票

工程基线的交付范围见 [phases/phase-1-baseline.md](phases/phase-1-baseline.md)。业务表与业务接口不在基线内。

## 2. 约束

| 约束 | 影响 |
| --- | --- |
| 微信正式版小程序需备案域名与定位权限 | 平台主体一次性申请，所有租户共用 |
| 年会峰值约单场 800 人、签到设计目标 100 req/s | 首发部署一个实例足够；所有设计按多实例成立，容量不足时横向加实例；用限流保护共享资源 |
| 季节性售卖、按场次收费 | 配额表比完整计费系统优先 |
| 位置与名单属个人信息 | 导出权限、保留策略、审计必须按组织隔离 |
| 前后端契约已用 OpenAPI 生成 | 业务接口只改 `openapi.yaml`，禁止手写重复描述 |

## 3. 原则

1. **活动引擎与 SaaS 壳分离**：签到判定、抽奖事务、SSE 不感知计费；壳只负责组织、配额、入口鉴权。
2. **服务端裁决**：围栏与中奖结果只由服务端决定；客户端只上报与展示。
3. **写路径幂等**：弱网重试不产生重复签到或重复抽奖。
4. **先落库再展示**：抽奖结果提交成功后再推大屏。
5. **默认共享库行级隔离**：所有业务 SQL 经强制条件注入 `org_id` / `event_id`；分库是触发条件后的事。
6. **安全边界在接口**：前端隐藏入口不算鉴权。
7. **契约优先**：HTTP 业务面以 `api/openapi.yaml` 为唯一来源；生成代码禁止手改。
8. **实例无状态**：进程内不保存跨请求共享的状态，任意实例可处理任意请求。共享状态放 PostgreSQL 或 Redis，归属见 §4.5。

## 4. 系统架构

```text
        平台小程序          Web 控制台 / 大屏
              \                 /
               \   HTTPS       /
                ▼             ▼
              Nginx（生产，阶段 9）
           golottery.ioclub.cn
                     │
                     ▼
        golottery-api (Go, echo)  127.0.0.1:5568
          ├── /healthz /readyz          httpapi
          ├── /openapi.json /openapi.yaml
          └── /api/*                    OpenAPI strict handler
                │ gorm (pgx)       │ go-redis        │ S3 协议（内网写）
                ▼                  ▼                 ▼
          PostgreSQL 18         Redis 7        对象存储（RustFS）
                                                     │
                                                     ▼ 公网读
                                          golottery-oss.ioclub.cn
```

生产对外主机名：`golottery.ioclub.cn` 承载 Web 与 API；`golottery-oss.ioclub.cn` 只提供品牌素材的公网读取。Nginx 后可以挂多个 API 实例，不需要会话保持。

对象存储只存品牌素材，访问只经过 S3 协议，生产可换任意 S3 兼容存储，见 [阶段 8](phases/phase-8-branding.md)。API 经内网 `S3_ENDPOINT` 读写；对外素材 URL 由 `ASSET_PUBLIC_BASE_URL` 指向 `https://golottery-oss.ioclub.cn`。对象存储不可用时只影响素材上传与读取，不影响签到与抽奖。

没有进程内异步任务。签到与抽奖都是短事务。大屏推送用 SSE，跨实例广播走 Redis。定期清理做成 `golottery` 子命令，由宿主机定时器调用。

### 4.1 技术选型

| 层 | 选型 | 说明 |
| --- | --- | --- |
| 语言 | Go 1.27 | `CGO_ENABLED=0` |
| HTTP | `github.com/labstack/echo/v4` | 显式 `http.Server`；自研 `requestLogger` / `recovery`，统一走 `slog` |
| 契约 | `api/openapi.yaml` + `oapi-codegen` | `models.yaml` / `server.yaml` 生成 `api/*.gen.go`；`go generate ./...` |
| 持久化 | `gorm.io/gorm` + `gorm.io/driver/postgres`（pgx） | 显式 SQL 迁移（`internal/store/migrations` + `schema_migrations`） |
| 数据库 | PostgreSQL 18（`postgres:18-alpine`） | 本机 Compose；生产与 api 同机 |
| 共享状态 | Redis 7（`redis:7-alpine`）+ `github.com/redis/go-redis/v9` | 限流、SSE 广播、微信凭据缓存；封装在 `internal/redisx`，阶段 5 引入 |
| 对象存储 | RustFS（`rustfs/rustfs:1.0.0`）+ `github.com/minio/minio-go/v7` | 只用 S3 协议；封装在 `internal/objectstore`，阶段 8 引入 |
| Excel | `github.com/xuri/excelize/v2` | 名单导入导出；导入读单元格原始值，避免数字格式改写手机号 |
| 令牌 | Bearer API Token，库内只存 SHA-256 | 标准库 `crypto/sha256`；明文只在签发时返回一次 |
| 配置 | `github.com/joho/godotenv` + 自有 registry | 分层解析见 §10 |
| 口令 | `golang.org/x/crypto/argon2` | argon2id，PHC 字符串 |
| 门禁 | `make fmt` + `make lint` + `make test` | golangci-lint：errcheck / govet / ineffassign / staticcheck |
| 前端 | Bun + Vite + React 19 + TypeScript + Mantine 9 | TanStack Query、React Router、Vitest、Biome |
| API 客户端 | `@hey-api/openapi-ts` → `src/api-gen` | 同一份 `openapi.yaml`；运行时在 `src/api.ts` 注入令牌与 `401` |

### 4.2 目录

```text
golottery/
  golottery-api/                Go module `golottery/api`（二进制 golottery）
    cmd/golottery/
      main.go                   装配：config → store → settings → httpapi → OpenAPI handler
      run.go                    http.Server、信号、优雅关闭
      settings_cmd.go           golottery settings list|set|unset
      password_cmd.go           golottery hash-password（从 stdin 读口令）
    internal/
      config/                   配置 registry 与分层解析（Bootstrap / Apply）
      settings/                 settings 表实体与仓储
      store/                    gorm 句柄、连接池、显式 SQL 迁移、测试辅助
      store/migrations/         embed 的 NNN_*.sql
      bizerr/                   错误码与中文文案
      httpapi/                  echo 根路由、中间件、/readyz、安全响应头、请求关联 ID
      auth/                     API Token、argon2id、LoginAttempt 实体、登录限速
      platform/                 平台运营账号：实体、播种、登录 / 登出 / 改密
      org/                      组织、配额、场次流水；组织管理员账号与 console 会话
      redisx/                   Redis 客户端装配、Ping、测试辅助
      event/                    活动、名单、奖项、Excel 导入导出
      wechat/                   微信 access_token 与小程序码；wechattest 为测试替身
      apihttp/                  实现 api.StrictServerInterface，注册生成路由
    api/                        openapi.yaml、生成配置、*.gen.go（禁止手改生成物）
    Makefile  Dockerfile  compose.yml  .env.example  .golangci.yml  VERSION
  golottery-web/                控制台与大屏（独立项目）
    src/api-gen/                openapi-ts 生成物，禁止手改
    src/api.ts                  按路径前缀附加令牌、401、ApiError
    src/platform/               运营后台（路由前缀 /platform，懒加载）
    src/organization/           组织控制台（路由前缀 /organization，懒加载）
    src/components/             跨入口复用的视图壳：TableSkeleton、EmptyState
  golottery-mp/                 微信小程序，不在工程基线交付内
  docs/
```

### 4.3 依赖规则

```text
main ──→ 全部

apihttp ──→ api  auth  bizerr  platform  org  event  wechat  echo
platform ──→ auth  bizerr  gorm
org ──→ auth  bizerr  gorm
event ──→ org  bizerr  gorm  excelize
wechat ──→ go-redis  net/http
auth / settings ──→ gorm
store ──→ gorm                 （只被 main 与测试辅助 import）
redisx ──→ go-redis           （客户端装配、Ping、测试辅助；阶段 5 起）
ratelimit ──→ go-redis        （阶段 6 起）
config ──→ os  godotenv        （不依赖 settings；覆盖值以 map 传入）
httpapi ──→ echo               （不依赖 gorm、store 与任何领域包；就绪探针以函数注入）
bizerr ──→ 标准库
```

- 不设 `repo` / `services` 层：实体与仓储放在拥有该概念的包内
- `httpapi` 不 import `api` 生成包；OpenAPI 路由由 `apihttp` 挂到已建好的 echo 上
- 业务领域包在后续阶段加入，不得反向依赖 `apihttp`

### 4.4 本机开发约定

| 服务 | 地址 |
| --- | --- |
| API | `127.0.0.1:5568` |
| PostgreSQL | `127.0.0.1:15436`，开发库 `golottery`、测试库 `golottery_test`，账号 `postgres/secret` |
| Adminer | `127.0.0.1:58033`，数据库管理页，默认连接 `postgres` 服务 |
| RustFS | S3 接口 `127.0.0.1:57800`，控制台 `127.0.0.1:57801`，账号 `rustfsadmin/rustfsadmin` |
| Redis | `127.0.0.1:57379`。开发用 db 0，测试用 db 15 |
| Web dev（Vite） | `localhost:3000`，`/api`、`/healthz`、`/readyz`、`/openapi.json`、`/openapi.yaml` 代理到 `5568` |

本机依赖用 `golottery-api/compose.yml` 启动。测试代码不负责 `CREATE DATABASE`（`init-db.sql` 建 `golottery_test`）。

### 4.5 共享状态与多实例

划分规则：需要持久、需要审计或参与业务事务的状态放 PostgreSQL；高频、短命、丢失后可自动重建的状态放 Redis。

| 状态 | 位置 | 说明 |
| --- | --- | --- |
| 令牌与宾客会话 | PostgreSQL | `api_tokens`、`guest_sessions` |
| 登录与绑定失败锁定 | PostgreSQL | `login_attempts`；安全相关、频率低，需要留痕 |
| 抽奖串行与幂等 | PostgreSQL | 事务咨询锁 + `draw_logs.request_id` |
| 迁移与清理命令互斥 | PostgreSQL | `pg_advisory_lock` / `pg_try_advisory_lock` |
| 签到请求限流 | Redis | 按 openid 计数，高频且不需要留痕，见[阶段 6](phases/phase-6-checkin.md) |
| SSE 广播 | Redis Pub/Sub | 按活动分频道，见[阶段 7](phases/phase-7-draw.md) |
| 微信 access_token | Redis | 全部实例共用一份，见[阶段 5](phases/phase-5-event-setup.md) |

Redis 约定：

- 键统一前缀 `gl:`，所有键带过期时间；不开启持久化，数据丢失只造成限流计数归零或凭据重新获取
- 运行中 Redis 不可用时降级：限流放行并记录错误日志，SSE 靠客户端版本号比对补漏；签到与抽奖不中断
- 启动时 Redis 不可达则拒绝启动，尽早暴露配置错误；`/readyz` 只检查 PostgreSQL，避免 Redis 故障让全部实例被摘除
- 测试连接真实 Redis 的 db 15，每个测试前 `FLUSHDB`；不用内存替身证明共享状态路径

进程内只允许保存与当前连接绑定的状态，例如本实例持有的 SSE 连接列表。

## 5. 数据模型与不变量

业务表（`org`、`event`、`attendee`、`prize`、`draw_result` 等）在活动引擎阶段按产品需求落地，不在工程基线建表。基线只建运行所需的三张表。时间字段 `timestamptz`。

### 5.1 `settings`（`settings.Setting`）

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| `Key` | varchar(64) | 主键 |
| `Value` | text | 非空 |
| `Group` | varchar(32) | 列名 `category` |
| `UpdatedBy` | varchar(64) | |
| `UpdatedAt` | timestamptz | |

只允许写入 registry 中的 ScopeApp 键。ScopeInfra 键拒绝写入。

### 5.2 `api_tokens`（`auth.APIToken`）

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| `ID` | uuid | 主键，应用生成 |
| `PrincipalType` | varchar(32) | `console` / `host` / `platform` |
| `PrincipalID` | varchar(64) | 主体标识 |
| `Name` | varchar(64) | 如 `login` |
| `TokenHash` | bytea | SHA-256，唯一 |
| `ExpiresAt` | timestamptz | |
| `LastUsedAt` | timestamptz NULL | |
| `CreatedAt` | timestamptz | |

明文令牌不落库。过期行在校验时视为不存在。

### 5.3 `login_attempts`（`auth.LoginAttempt`）

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| `ID` | bigint | 主键，自增 |
| `Key` | varchar(128) | 与 `CreatedAt` 联合索引 |
| `CreatedAt` | timestamptz | |

限速判定在 `auth.LoginLimiter`：窗口内失败次数达到阈值即拒绝，等待分钟数向上取整且至少为 1；登录成功删除该键的记录。键的格式为 `<主体类型>:<用户名>`，如 `platform:ops`。

### 5.4 `platform_users`（`platform.User`）

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| `ID` | uuid | 主键，应用生成；即 platform 令牌的 `PrincipalID` |
| `Username` | varchar(64) | 唯一 |
| `PasswordHash` | text | argon2id PHC |
| `CreatedAt` / `UpdatedAt` | timestamptz | |

启动时表为空则用 `PLATFORM_USER` 与 `PLATFORM_PASSWORD_HASH` 播种一行；表为空且缺任一键，或哈希格式无效时拒绝启动。表非空时不改已有行。

### 5.5 组织与配额（`org.Org`、`org.Quota`、`org.LedgerEntry`）

表结构见 [阶段 3 §4.1](phases/phase-3-org-quota.md#41-数据)。不变量：

- `org_quotas.event_credits >= 0`、`max_attendees > 0`、`credit_ledger.delta <> 0`、`balance_after >= 0` 由数据库 `CHECK` 兜底
- 调整场次用一条带条件的 `UPDATE ... WHERE event_credits + delta >= 0 RETURNING`，与流水写入同一事务；并发扣减不会把余额打成负数
- 流水只追加；`operator_type` / `operator_id` 取自操作令牌

### 5.6 `org_users`（`org.User`）

表结构见 [阶段 4 §4.1](phases/phase-4-org-admin-auth.md#41-数据)。邮箱保存前去首尾空白并转小写，全局唯一；一个邮箱只属于一个组织。临时口令由服务端生成（10 位，去掉 `0 O o 1 l I`），明文只在创建与重置的响应里出现一次。

## 6. 认证与会话

### 6.1 令牌

- 签发：32 字节 `crypto/rand`，十六进制明文返回一次；库内保存 `sha256`
- `typ`：`console`（组织管理员）、`host`（主持人）、`platform`（平台运营）
- 宾客令牌存在独立的 `guest_sessions`，按 `(event_id, openid)` 一行，重新登录即轮换；有效期 `GUEST_SESSION_TTL`，契约里对应 `guestBearer`
- 校验按 `typ` 分开，三种令牌不可互换
- 有效期来自 `CONSOLE_SESSION_TTL` / `HOST_SESSION_TTL` / `PLATFORM_SESSION_TTL`
- 需要令牌的接口在 `openapi.yaml` 中用 `security` 声明（`platformBearer` → `platform`，`consoleBearer` → `console`）；`apihttp` 启动时从内嵌契约读出这些接口，校验 `Authorization: Bearer` 且类型匹配，失败统一 `401 E_UNAUTHORIZED`
- `console` 令牌每次请求再查 `org_users` 与 `orgs`：账号或组织已停用即 `401`。组织以账号行的 `org_id` 为准，放进请求上下文，不接受客户端传入
- 登出只删除当前令牌；改密删除该主体全部令牌并签发一把新令牌

### 6.2 口令

- argon2id，PHC 字符串：`m=65536,t=3,p=2`
- `golottery hash-password` 从 stdin 读口令，stdout 只打印哈希
- 登录限速使用 `login_attempts`，阈值 `LOGIN_MAX_FAILURES`，窗口 `LOGIN_WINDOW`

### 6.3 安全响应头

所有经 `httpapi` 的响应：

| 头 | 值 |
| --- | --- |
| `X-Content-Type-Options` | `nosniff` |
| `Referrer-Policy` | `no-referrer` |
| `Cache-Control` | `no-store` |
| `Content-Security-Policy` | `default-src 'none'; frame-ancestors 'none'` |

## 7. HTTP 表面

### 7.1 路由与挂载

| 路由 | 鉴权 | 归属 |
| --- | --- | --- |
| `GET /healthz` | 无；JSON，含数据库与 Redis 状态；`ok` 只看数据库 | `apihttp`（OpenAPI `getHealthz`） |
| `GET /readyz` | 无；数据库可达 `200 ok`，否则空体 `503` | `httpapi` |
| `GET /api` | 无；服务元数据 | `apihttp` |
| `GET /openapi.json` · `GET /openapi.yaml` | 无 | `apihttp` |
| `POST /api/platform/login` | 无 | `apihttp` → `platform` |
| `POST /api/platform/logout` · `GET /api/platform/me` · `POST /api/platform/password` | platform 令牌 | `apihttp` → `platform` |
| `/api/platform/orgs` 下的组织与配额接口（[阶段 3 §4.2](phases/phase-3-org-quota.md#42-接口)） | platform 令牌 | `apihttp` → `org` |
| `/api/platform/orgs/:id/users` 下的管理员接口（[阶段 4 §4.2](phases/phase-4-org-admin-auth.md#42-运营接口)） | platform 令牌 | `apihttp` → `org` |
| `POST /api/organization/login` | 无 | `apihttp` → `org` |
| `/api/organization/*` 其余接口（[阶段 4 §4.3](phases/phase-4-org-admin-auth.md#43-管理员接口)） | console 令牌 | `apihttp` → `org` |
| `/api/organization/events` 下的活动、名单、奖项、导入导出、活动码（[阶段 5 §4.2](phases/phase-5-event-setup.md#42-接口)） | console 令牌 | `apihttp` → `event`、`wechat` |
| `/api/organization/events/:id` 下的工作人员邀请、重置现场数据、签到明细导出（[阶段 6 §4.1](phases/phase-6-checkin.md#41-数据)、[§4.3](phases/phase-6-checkin.md#43-现场管理接口)） | console 令牌 | `apihttp` → `guest` |
| `POST /api/guest/session` | 无 | `apihttp` → `guest` |
| `/api/guest/*` 其余接口：绑定、签到、现场求助与工作人员接口（[阶段 6 §4.2](phases/phase-6-checkin.md#42-宾客接口)、[§4.3](phases/phase-6-checkin.md#43-现场管理接口)） | guest 令牌 | `apihttp` → `guest` |

列表接口统一用 `offset` / `limit` 查询参数（`limit` 默认 40，最大 100），返回 `{items, total}`；越界值按边界处理，不报错。
| 其余路径 | 空体 `404` | `httpapi` |

`httpapi` 使用静默错误处理器，不把框架默认错误页暴露给调用方。OpenAPI handler 返回的错误实现 `bizerr.Error` 时，按 §7.2 写成 JSON。

### 7.2 错误体与错误码

```json
{ "code": "E_UNAUTHORIZED", "message": "登录已失效，请重新登录" }
```

| HTTP | Code | 文案 |
| --- | --- | --- |
| 400 | `E_BAD_REQUEST` | 请求格式不正确 |
| 400 | `E_NAME_REQUIRED` | 请填写名称 |
| 400 | `E_PASSWORD_TOO_SHORT` | 密码至少 8 位 |
| 400 | `E_PASSWORD_UNCHANGED` | 新口令不能与当前口令相同 |
| 400 | `E_CURRENT_PASSWORD_WRONG` | 当前口令不正确 |
| 401 | `E_UNAUTHORIZED` | 登录已失效，请重新登录 |
| 401 | `E_INVALID_CREDENTIALS` | 账号或口令错误 |
| 403 | `E_FORBIDDEN` | 无权执行此操作 |
| 404 | `E_NOT_FOUND` | 内容不存在或无权访问 |
| 409 | `E_CONFLICT` | 操作与当前状态冲突 |
| 429 | `E_TOO_MANY_ATTEMPTS` | 尝试次数过多，请 %d 分钟后再试 |
| 500 | `E_INTERNAL` | 系统出错了，请稍后重试 |
| 503 | `E_STORE_UNAVAILABLE` | 系统暂时不可用，请稍后重试 |
| 409 | `E_NO_EVENT_CREDITS` | 剩余场次不足，请联系运营开通 |
| 400 | `E_EVENT_INCOMPLETE` | 就绪前请补全：%s |
| 400 | `E_ROSTER_FULL` | 名单不能超过人数上限 %d 人 |
| 400 | `E_IMPORT_FILE` | 无法读取文件，请上传 5 MB 以内、表头为姓名、部门、手机号的 xlsx |
| 400 | `E_IMPORT_INVALID` | 有 %d 行需要修正，整份文件未导入 |
| 503 | `E_WECHAT_NOT_CONFIGURED` | 微信小程序尚未配置，暂时无法生成小程序码 |
| 400 | `E_ATTENDEE_NOT_MATCHED` | 姓名或手机后四位与名单不符 |
| 409 | `E_ATTENDEE_TAKEN` | 该名单人员已被其他设备绑定，请联系现场工作人员 |
| 400 | `E_NOT_BOUND` | 请先核对姓名与手机后四位 |
| 409 | `E_EVENT_NOT_OPEN` | 活动尚未开放签到 |
| 409 | `E_WINDOW_CLOSED` | 当前不在签到时间内 |
| 400 | `E_LOW_ACCURACY` | 定位精度不足，请到开阔处重试或联系现场工作人员 |
| 400 | `E_OUT_OF_RANGE` | 不在签到范围内，距离约 %d 米 |
| 400 | `E_INVITE_INVALID` | 邀请链接无效、已使用或已过期 |

面向用户的文案句尾不用中文句号 `。`。带 `%d` / `%s` 的文案由调用方传入参数。名单导入的 400 响应另带 `rows: [{row, reason}]`，行号按表格计，表头为第 1 行。

5xx 错误的原始原因只写日志（含操作名与错误码，例如微信 `errcode`），不出现在响应里。

### 7.3 请求关联 ID

入站 `X-Request-Id` 只在来源属于 `TRUSTED_PROXIES`、长度 ≤ 64、字符集为 `[A-Za-z0-9-]` 时沿用，否则丢弃并重新生成。响应头与访问日志都带该值。

## 8. 前端结构

一个 SPA，运营后台、组织端、宾客网页与大屏四条入口，各自懒加载。组织端在阶段 4 加入，大屏在阶段 7 替换占位。

| 路由 | 页面 |
| --- | --- |
| `/` | 重定向到 `/platform` |
| `/platform/login` | 运营登录；已有令牌时直接进入 `/platform` |
| `/platform` | 运营后台外壳；无令牌时去 `/platform/login`，有令牌时请求 `GET /api/platform/me`；首页重定向到 `/platform/orgs` |
| `/platform/orgs` | 组织列表（页码写在 `?page=`）与开通弹窗 |
| `/platform/orgs/:orgId` | 组织详情：停用 / 启用、调整场次、人数上限、管理员（创建、重置口令、停用 / 启用）、最近 20 条流水 |
| `/organization/login` | 组织管理员登录；已有令牌时直接进入 `/organization` |
| `/organization` | 组织控制台外壳，页头显示本组织名称；无令牌时去 `/organization/login`；首页重定向到 `/organization/events` |
| `/organization/events` | 活动列表（页码写在 `?page=`）、剩余场次与创建弹窗 |
| `/organization/events/:eventId` | 活动详情：状态操作（就绪前提示场次消耗，余额为 0 时禁用）、签到设置（时间按北京时间输入）、签到入口（网页地址与二维码、小程序码下载）、奖项、现场工作人员（一次性邀请链接与二维码、撤销）、现场数据（签到明细导出、签到开始前重置）、名单（导入逐行报错、导出） |
| `/m/:publicId` | 宾客签到页（手机）：确认身份、签到、现场求助；求助待处理时每 10 秒刷新 |
| `/m/:publicId/staff` | 现场工作台（手机）：`?invite=` 兑换邀请后去掉参数；签到进度与求助列表每 10 秒轮询、代签，`admin` 另有签到方式与围栏设置 |
| `/host` | 大屏占位 |

- 视觉 token 定义在 `src/theme.ts`：`brand` 第 6 阶 `#1E4544`，`forceColorScheme="light"`
- 字体 `@fontsource/roboto`（400/500/700）、`roboto-condensed`（700）、`roboto-mono`（500），自托管
- 样式用 CSS Modules；结构用 Mantine 布局组件
- 业务请求只从 `#/api-gen/sdk.gen` 与 `#/api-gen/types.gen` 引用
- `src/api.ts` 按路径前缀附加 Bearer：`/api/platform/*` 用 `gl.token.platform`，`/api/organization/*` 用 `gl.token.console`，`/api/host/*` 用 `gl.token.host`，`/api/guest/*` 用 `gl.token.guest`
- 宾客网页没有登录页：浏览器生成的设备号存在 `gl.device`，令牌只属于 `gl.guest.event` 记录的活动；并发调用共用一次登录，`401` 时重新登录并重试一次，不走全局跳转
- `401` 删除对应令牌，只清该入口的查询缓存；当前页面仍在该入口内时，经 `router.navigate` 跳到它的登录页（`/platform/login`、`/organization/login`），已离开该入口则不跳。登录接口自身的 `401` 只展示错误，不跳转
- 同一浏览器可同时登录运营后台与组织控制台，令牌与缓存互不影响
- 临时口令只放在组件状态里展示一次，关闭即丢弃，不进查询缓存
- 前端只请求相对路径；开发时由 Vite 代理

## 9. 启动与关闭

```text
启动：
  config.Bootstrap（.env / 环境变量 / 默认值）→ 校验必填项
  store.Open → store.Migrate → store.Ping
  redisx.Open（连接并 Ping，不可达则拒绝启动）
  settings.Snapshot → config.Apply（ScopeApp 覆盖）
  platform.Seed（platform_users 为空时播种）
  httpapi.NewRouter → apihttp.Register（从内嵌契约读出受保护接口）
  启动 http.Server（ReadHeaderTimeout 5s）

关闭（SIGINT / SIGTERM）：
  server.Shutdown（5s）→ Redis Close → store.Close
```

`DATABASE_URL` 或 `REDIS_URL` 缺失、`SESSION_SECRET` 不足 32 字符、数据库或 Redis 不可达、`platform_users` 为空却缺播种配置时拒绝启动。

## 10. 配置

ScopeInfra 只来自 `.env` / 环境变量 / 默认值。ScopeApp 额外可由 `golottery settings` 写入 `settings` 表覆盖，优先级 `settings > .env > 环境变量 > 默认值`，改动后重启生效。

`.env` 用 `godotenv.Overload` 加载，会覆盖同名环境变量。生产只用环境变量注入，工作目录不放 `.env`。`make dev` 用 shell `.` 读取 `.env`，含 `$` 的值（如 argon2id PHC 哈希）必须用单引号包裹。

| 键 | Scope | 默认 | Secret | 说明 |
| --- | --- | --- | --- | --- |
| `DATABASE_URL` | Infra | （必填） | ✅ | PostgreSQL URL |
| `REDIS_URL` | Infra | （必填） | ✅ | Redis URL，本机 `redis://127.0.0.1:57379/0` |
| `APP_HOST` | Infra | `127.0.0.1` | | 监听地址 |
| `APP_PORT` | Infra | `5568` | | 监听端口 |
| `SESSION_SECRET` | Infra | （必填） | ✅ | 预留给后续签名，≥ 32 字符 |
| `TRUSTED_PROXIES` | Infra | 空 | | 信任其 `X-Forwarded-For` / `X-Request-Id` 的 CIDR，逗号分隔 |
| `PLATFORM_USER` | Infra | 空 | | 播种的运营账号；仅 `platform_users` 为空时必填 |
| `PLATFORM_PASSWORD_HASH` | Infra | 空 | ✅ | 播种账号的 argon2id 哈希，由 `golottery hash-password` 生成；仅表为空时必填 |
| `WECHAT_APP_ID` | Infra | 空 | | 平台小程序 AppID；为空时活动码接口返回 `503 E_WECHAT_NOT_CONFIGURED` |
| `WECHAT_APP_SECRET` | Infra | 空 | ✅ | 平台小程序 AppSecret |
| `WECHAT_ENV_VERSION` | Infra | `release` | | 小程序码打开的版本：`release` / `trial` / `develop`；非 `release` 时不校验页面是否已发布 |
| `WECHAT_API_BASE` | Infra | `https://api.weixin.qq.com` | | 微信接口地址；测试与本机联调可指向替身 |
| `CONSOLE_SESSION_TTL` | App | `12h` | | 控制台令牌有效期 |
| `HOST_SESSION_TTL` | App | `12h` | | 主持人令牌有效期 |
| `PLATFORM_SESSION_TTL` | App | `8h` | | 运营令牌有效期 |
| `LOGIN_MAX_FAILURES` | App | `5` | | 限速阈值 |
| `LOGIN_WINDOW` | App | `15m` | | 限速窗口 |
| `GUEST_LOGIN_MODE` | Infra | `wechat` | | 宾客登录方式：`wechat`（小程序 `code`）或 `web`（浏览器设备号，首发使用） |
| `GUEST_SESSION_TTL` | App | `24h` | | 宾客令牌有效期 |

仅宿主机使用、不进容器的键：`POSTGRES_PASSWORD`、`POSTGRES_PORT`、`POSTGRES_DB`、`POSTGRES_USER`。

## 11. 测试边界

| 层 | 做法 |
| --- | --- |
| `config` | 必填项缺失时报出键名；`SESSION_SECRET` 过短拒绝；ScopeInfra 不接受 settings 覆盖；优先级 `settings > .env > 环境变量 > 默认值` |
| `settings` | 读写删；未知键与 ScopeInfra 键拒绝 |
| `redisx` | 真实 Redis db 15 的 `Open` / `Ping`；空地址、非法地址、不可达端口都报错 |
| `store` | 真实库 `Ping` / `Migrate` / `Reset`；默认 `postgres://postgres:secret@127.0.0.1:15436/golottery_test?sslmode=disable` |
| `bizerr` | 每个 Code 都有文案；文案句尾无中文句号 |
| `auth` | 令牌签发与按哈希查找；`typ` 不可互换；过期拒绝；argon2id 往返；限速阈值、等待分钟数与清零 |
| `platform` | 播种：表空时写入、表非空不覆盖、缺配置或哈希无效拒绝 |
| `org`（账号） | 临时口令只在响应里且哈希可校验、重复邮箱 `409`、跨组织操作 `404`、`console` 令牌不能当 `platform` 用、停用账号删令牌、停用组织令牌立即失效、重置与改密后旧令牌失效、邮箱限速 |
| `org` | 开通的三行同事务（含回滚）、初始场次为 0 不写流水、调整与 `balance_after` 一致、扣成负数拒绝且余额不变、并发扣减不为负、停用启用幂等且不动配额、分页顺序 |
| `httpapi` | `/readyz` 成功 200、失败空体 503；未知路径空体 404；安全响应头；不可信 `X-Request-Id` 被丢弃 |
| `apihttp` | `/healthz` 在库可达时 `ok=true, db=up`，不可达时 503；`redis` 字段为 `up` / `down` / `skipped` 且不影响 `ok`；运营登录、限速、登出、`me`、改密的 HTTP 行为；受保护接口与契约的 `security` 一致 |
| `guest` | 围栏判定与精度抵扣、WGS-84 换算、限流、幂等签到、绑定占用与锁定、邀请单次使用、三种求助处理、代签、现场切换签到方式与移动围栏、重置只在签到开始前 |
| 前端 | `ApiError` 解析；路径前缀到令牌的映射；`401` 清令牌但登录接口除外；`401` 跳转目标只在本入口内；登录表单的提交参数与错误展示；临时口令只展示一次；宾客会话按活动隔离、并发共用一次登录、过期后重登重试 |

`make test` 即 `go test -p=1 ./...`。禁止用 SQLite 证明持久化路径。

## 12. 迁移与兼容

结构由显式 SQL 迁移维护：`internal/store/migrations` 下的 `NNN_*.sql` 由 `embed` 打包，`store.Migrate` 按文件名顺序应用，已应用版本记录在 `schema_migrations`，不再重复执行。并发进程用 `pg_advisory_lock` 串行化。

| 迁移 | 内容 |
| --- | --- |
| `001_init.sql` | `schema_migrations`、`settings`、`api_tokens`、`login_attempts` |
| `002_platform_users.sql` | `platform_users` |
| `003_org_quota.sql` | `orgs`、`org_quotas`、`credit_ledger` |
| `004_org_users.sql` | `org_users` |
| `005_event_setup.sql` | `events`、`attendees`、`prizes`；`credit_ledger.event_id` 外键与每场只扣一次的唯一索引 |
| `006_checkin.sql` | `guest_sessions`、`checkin_attempts`、`manual_requests`、`event_staff`、`staff_invites`；`attendees` 追加绑定与签到字段 |

仓库尚无生产数据。业务表以新增迁移追加，不改已发布迁移文件。

## 13. 明确不做

- 第三方平台生成客户独立小程序
- 组织级独立数据库 / 独立部署 SKU
- 指定中奖人、调整中奖概率
- 持续定位追踪与轨迹存储
- 首发在线支付、发票、合同电子签
- MQ
