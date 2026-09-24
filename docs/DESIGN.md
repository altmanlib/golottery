---
title: 定位签到抽奖 · 技术方案
type: design
status: published
updated: 2026-09-24
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
- 对象存储、封面、Redis

工程基线的交付范围见 [phases/phase-1-baseline.md](phases/phase-1-baseline.md)。业务表与业务接口不在基线内。

## 2. 约束

| 约束 | 影响 |
| --- | --- |
| 微信正式版小程序需备案域名与定位权限 | 平台主体一次性申请，所有租户共用 |
| 年会峰值约单场 800 人、签到设计目标 100 req/s | 首发用单体 + PostgreSQL 足够；用租户限流保护共享资源 |
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

## 4. 系统架构

```text
        平台小程序          Web 控制台 / 大屏
              \                 /
               \   HTTPS       /
                ▼             ▼
              Nginx（生产，阶段 4）
                     │
                     ▼
        golottery-api (Go, echo)  127.0.0.1:5568
          ├── /healthz /readyz          httpapi
          ├── /openapi.json /openapi.yaml
          └── /api/*                    OpenAPI strict handler
                     │ gorm (pgx)
                     ▼
               PostgreSQL 18
```

没有异步任务，没有对象存储。签到与抽奖都是短事务。大屏推送用 SSE，在业务阶段挂到同一进程。

### 4.1 技术选型

| 层 | 选型 | 说明 |
| --- | --- | --- |
| 语言 | Go 1.27 | `CGO_ENABLED=0` |
| HTTP | `github.com/labstack/echo/v4` | 显式 `http.Server`；自研 `requestLogger` / `recovery`，统一走 `slog` |
| 契约 | `api/openapi.yaml` + `oapi-codegen` | `models.yaml` / `server.yaml` 生成 `api/*.gen.go`；`go generate ./...` |
| 持久化 | `gorm.io/gorm` + `gorm.io/driver/postgres`（pgx） | 显式 SQL 迁移（`internal/store/migrations` + `schema_migrations`） |
| 数据库 | PostgreSQL 18（`postgres:18-alpine`） | 本机 Compose；生产与 api 同机 |
| 令牌 | Bearer API Token，库内只存 SHA-256 | 标准库 `crypto/sha256`；明文只在签发时返回一次 |
| 配置 | `github.com/joho/godotenv` + 自有 registry | 分层解析见 §11 |
| 口令 | `golang.org/x/crypto/argon2` | argon2id，PHC 字符串 |
| 门禁 | `make fmt` + `make lint` + `make test` | golangci-lint：errcheck / govet / ineffassign / staticcheck |
| 前端 | Bun + Vite + React 19 + TypeScript + Mantine 8 | TanStack Query、React Router、Vitest、Biome |
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
      auth/                     API Token、argon2id、LoginAttempt 实体
      apihttp/                  实现 api.StrictServerInterface，注册生成路由
    api/                        openapi.yaml、生成配置、*.gen.go（禁止手改生成物）
    Makefile  Dockerfile  compose.yml  .env.example  .golangci.yml  VERSION
  golottery-web/                控制台与大屏（独立项目）
    src/api-gen/                openapi-ts 生成物，禁止手改
    src/api.ts                  令牌、401、ApiError
  golottery-mp/                 微信小程序，不在工程基线交付内
  docs/
```

### 4.3 依赖规则

```text
main ──→ 全部

apihttp ──→ api  auth  bizerr  echo
auth / settings ──→ gorm
store ──→ gorm                 （只被 main 与测试辅助 import）
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
| Web dev（Vite） | `localhost:3000`，`/api`、`/healthz`、`/readyz`、`/openapi.json`、`/openapi.yaml` 代理到 `5568` |

本机依赖用 `golottery-api/compose.yml` 启动。测试代码不负责 `CREATE DATABASE`（`init-db.sql` 建 `golottery_test`）。

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

基线只建表。限速判定在登录接口阶段实现。

## 6. 认证与会话

### 6.1 令牌

- 签发：32 字节 `crypto/rand`，十六进制明文返回一次；库内保存 `sha256`
- `typ`：`console`（组织管理员）、`host`（主持人）、`platform`（平台运营）
- 校验按 `typ` 分开，三种令牌不可互换
- 有效期来自 `CONSOLE_SESSION_TTL` / `HOST_SESSION_TTL` / `PLATFORM_SESSION_TTL`
- 基线只提供签发、按哈希查找、删除；登录路由在后续阶段挂上

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
| `GET /healthz` | 无；JSON，含数据库状态 | `apihttp`（OpenAPI `getHealthz`） |
| `GET /readyz` | 无；数据库可达 `200 ok`，否则空体 `503` | `httpapi` |
| `GET /api` | 无；服务元数据 | `apihttp` |
| `GET /openapi.json` · `GET /openapi.yaml` | 无 | `apihttp` |
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
| 401 | `E_UNAUTHORIZED` | 登录已失效，请重新登录 |
| 401 | `E_INVALID_CREDENTIALS` | 账号或口令错误 |
| 403 | `E_FORBIDDEN` | 无权执行此操作 |
| 404 | `E_NOT_FOUND` | 内容不存在或无权访问 |
| 409 | `E_CONFLICT` | 操作与当前状态冲突 |
| 429 | `E_TOO_MANY_ATTEMPTS` | 尝试次数过多，请 %d 分钟后再试 |
| 500 | `E_INTERNAL` | 系统出错了，请稍后重试 |
| 503 | `E_STORE_UNAVAILABLE` | 系统暂时不可用，请稍后重试 |

面向用户的文案句尾不用中文句号 `。`。带 `%d` 的文案由调用方传入数字。

### 7.3 请求关联 ID

入站 `X-Request-Id` 只在来源属于 `TRUSTED_PROXIES`、长度 ≤ 64、字符集为 `[A-Za-z0-9-]` 时沿用，否则丢弃并重新生成。响应头与访问日志都带该值。

## 8. 前端结构

一个 SPA，控制台与大屏两条入口。业务页在后续阶段替换占位内容。

| 路由 | 页面 |
| --- | --- |
| `/login` | 组织登录占位 |
| `/console` | 控制台占位，展示 `GET /healthz` |
| `/host` | 大屏占位 |

- 视觉 token 定义在 `src/theme.ts`：`brand` 第 6 阶 `#1E4544`，`forceColorScheme="light"`
- 字体 `@fontsource/roboto`（400/500/700）、`roboto-condensed`（700）、`roboto-mono`（500），自托管
- 样式用 CSS Modules；结构用 Mantine 布局组件
- 业务请求只从 `#/api-gen/sdk.gen` 与 `#/api-gen/types.gen` 引用
- `src/api.ts` 按路径附加 Bearer；`401` 删除 `gl.token` 并跳 `/login`
- 前端只请求相对路径；开发时由 Vite 代理

## 9. 启动与关闭

```text
启动：
  config.Bootstrap（.env / 环境变量 / 默认值）→ 校验必填项
  store.Open → store.Migrate → store.Ping
  settings.Snapshot → config.Apply（ScopeApp 覆盖）
  httpapi.NewRouter → apihttp.Register
  启动 http.Server（ReadHeaderTimeout 5s）

关闭（SIGINT / SIGTERM）：
  server.Shutdown（5s）→ store.Close
```

`DATABASE_URL` 缺失、`SESSION_SECRET` 不足 32 字符、数据库不可达时拒绝启动。

## 10. 配置

ScopeInfra 只来自 `.env` / 环境变量 / 默认值。ScopeApp 额外可由 `golottery settings` 写入 `settings` 表覆盖，优先级 `settings > .env > 环境变量 > 默认值`，改动后重启生效。

| 键 | Scope | 默认 | Secret | 说明 |
| --- | --- | --- | --- | --- |
| `DATABASE_URL` | Infra | （必填） | ✅ | PostgreSQL URL |
| `APP_HOST` | Infra | `127.0.0.1` | | 监听地址 |
| `APP_PORT` | Infra | `5568` | | 监听端口 |
| `SESSION_SECRET` | Infra | （必填） | ✅ | 预留给后续签名，≥ 32 字符 |
| `TRUSTED_PROXIES` | Infra | 空 | | 信任其 `X-Forwarded-For` / `X-Request-Id` 的 CIDR，逗号分隔 |
| `CONSOLE_SESSION_TTL` | App | `12h` | | 控制台令牌有效期 |
| `HOST_SESSION_TTL` | App | `12h` | | 主持人令牌有效期 |
| `PLATFORM_SESSION_TTL` | App | `8h` | | 运营令牌有效期 |
| `LOGIN_MAX_FAILURES` | App | `5` | | 限速阈值 |
| `LOGIN_WINDOW` | App | `15m` | | 限速窗口 |

仅宿主机使用、不进容器的键：`POSTGRES_PASSWORD`、`POSTGRES_PORT`、`POSTGRES_DB`、`POSTGRES_USER`。

## 11. 测试边界

| 层 | 做法 |
| --- | --- |
| `config` | 必填项缺失时报出键名；`SESSION_SECRET` 过短拒绝；ScopeInfra 不接受 settings 覆盖；优先级 `settings > 环境变量 > 默认值` |
| `settings` | 读写删；未知键与 ScopeInfra 键拒绝 |
| `store` | 真实库 `Ping` / `Migrate` / `Reset`；默认 `postgres://postgres:secret@127.0.0.1:15436/golottery_test?sslmode=disable` |
| `bizerr` | 每个 Code 都有文案；文案句尾无中文句号 |
| `auth` | 令牌签发与按哈希查找；`typ` 不可互换；过期拒绝；argon2id 往返 |
| `httpapi` | `/readyz` 成功 200、失败空体 503；未知路径空体 404；安全响应头；不可信 `X-Request-Id` 被丢弃 |
| `apihttp` | `/healthz` 在库可达时 `ok=true, db=up`，不可达时 503 |
| 前端 | `ApiError` 解析；`401` 清令牌并给出 `/login` |

`make test` 即 `go test -p=1 ./...`。禁止用 SQLite 证明持久化路径。

## 12. 迁移与兼容

结构由显式 SQL 迁移维护：`internal/store/migrations` 下的 `NNN_*.sql` 由 `embed` 打包，`store.Migrate` 按文件名顺序应用，已应用版本记录在 `schema_migrations`，不再重复执行。并发进程用 `pg_advisory_lock` 串行化。

| 迁移 | 内容 |
| --- | --- |
| `001_init.sql` | `schema_migrations`、`settings`、`api_tokens`、`login_attempts` |

仓库尚无生产数据。业务表以新增迁移追加，不改已发布迁移文件。

## 13. 明确不做

- 第三方平台生成客户独立小程序
- 组织级独立数据库 / 独立部署 SKU
- 指定中奖人、调整中奖概率
- 持续定位追踪与轨迹存储
- 首发在线支付、发票、合同电子签
- Redis / MQ / 对象存储
