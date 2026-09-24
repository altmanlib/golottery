---
title: 控制台用户认证
type: design
status: published
updated: 2026-09-24
---

# 控制台用户认证

## 1. 目标与范围

在工程基线上交付 Web 控制台的账号认证。凭证继续用已有的 Bearer API Token：明文只返回一次，库内只存 SHA-256，吊销即删行。

本阶段只做**平台运营账号**这一类主体，`principal_type = platform`。它是控制台的第一道门，后续组织、活动、配额都挂在这个会话后面。

做：

1. `platform_users` 表与启动时播种一个运营账号
2. `POST /api/console/login`、`POST /api/console/logout`、`GET /api/console/me`、`POST /api/console/password`
3. `/api/console/*` 除登录外都要求 platform 令牌
4. 登录失败限速，账号不存在时仍做一次 argon2 校验
5. 控制台登录页接上真实接口；`401` 沿用 `src/api.ts` 的跳转

不做：

- 组织管理员、主持人、宾客微信登录
- 多运营账号、注册、找回口令、验证码
- 审计日志
- 组织、活动、名单、奖项

## 2. 现状与约束

| 项 | 现状 |
| --- | --- |
| 令牌 | `auth.TokenIssuer` 已能签发、查找、删除 `platform` 令牌 |
| 口令 | argon2id PHC；`golottery hash-password` 可生成哈希 |
| 限速表 | `login_attempts` 已迁移，没有判定逻辑 |
| 契约 | 业务接口只写在 `api/openapi.yaml`，前后端都从生成物引用 |
| 前端 | `gl.token`、Bearer 拦截器、`401` 跳 `/login` 已接好；登录页是占位 |

配置沿用现有键：`PLATFORM_USER`、`PLATFORM_PASSWORD_HASH` 本阶段新增为 ScopeInfra 必填；`PLATFORM_SESSION_TTL`、`LOGIN_MAX_FAILURES`、`LOGIN_WINDOW` 已存在。

## 3. 原则

1. 登录成功才签发令牌；失败只追加 `login_attempts`，不创建会话
2. 三种 `principal_type` 继续不可互换。本阶段中间件只接受 `platform`
3. 登出只删除当前 Bearer 对应的那一行
4. 改密成功后删除该主体全部令牌，再签发一把新令牌返回给当前客户端
5. 错误体继续用 `bizerr`，不新增平行错误格式
6. 账号是否存在不通过响应时间或文案区分

## 4. 方案

### 4.1 数据

新增迁移 `002_platform_users.sql`：

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| `id` | uuid | 主键，应用生成 |
| `username` | varchar(64) | 唯一，非空 |
| `password_hash` | text | argon2id |
| `created_at` / `updated_at` | timestamptz | |

启动时若表为空，用 `PLATFORM_USER` 与 `PLATFORM_PASSWORD_HASH` 插入一行。表非空时不再改已有口令，避免重启覆盖人工改密。

`auth.LoginLimiter` 放在 `internal/auth`：按 `platform:<username>` 计数，窗口与阈值取配置。达到阈值返回 `429 E_TOO_MANY_ATTEMPTS`，分钟数向上取整且至少为 1。成功登录删除该键的失败记录。

### 4.2 接口

全部写入 `openapi.yaml`，由 `apihttp` 实现。

| 方法 | 路径 | 鉴权 | 说明 |
| --- | --- | --- | --- |
| POST | `/api/console/login` | 无 | `{username, password}` → `{token, expires_at}` |
| POST | `/api/console/logout` | platform | 删除当前令牌，`204` |
| GET | `/api/console/me` | platform | `{username}` |
| POST | `/api/console/password` | platform | `{current_password, new_password}` → `{token, expires_at}` |

登录失败、令牌缺失、令牌类型不符、令牌过期，统一 `401 E_UNAUTHORIZED` 或 `401 E_INVALID_CREDENTIALS`。口令错误用 `E_INVALID_CREDENTIALS`；未带或无效令牌用 `E_UNAUTHORIZED`。新口令少于 8 个字符返回 `400 E_PASSWORD_TOO_SHORT`；与当前口令相同返回 `400 E_PASSWORD_UNCHANGED`。当前口令不对返回 `401 E_CURRENT_PASSWORD_WRONG`，本阶段补这个错误码。

### 4.3 前端

- `/login` 提交用户名与口令，成功后 `setToken` 并进入 `/console`
- 登录失败展示接口返回的 `message`
- `/console` 在没有令牌时直接去 `/login`；有令牌时请求 `GET /api/console/me`，失败交给现有 `401` 处理
- 不在本阶段做改密页面

### 4.4 测试

| 对象 | 用例 |
| --- | --- |
| limiter | 未达阈值允许；达到阈值返回剩余等待；成功后计数清零 |
| login | 正确口令返回可校验的 platform 令牌；错误口令与不存在用户都是 `401`；连续失败触发 `429` |
| logout | 当前令牌删除后再次访问 `401`；其他主体的令牌不受影响 |
| password | 成功后旧令牌失效、响应中的新令牌有效；短口令与相同口令被拒绝 |
| me | 无令牌、host 令牌、过期令牌均为 `401` |

前端只测登录表单的提交参数与错误展示条件，不测样式。

## 5. 明确不做

- 自助注册与邀请
- 按 IP 限流
- 记住我、刷新令牌、多设备列表
- 运营后台的账号管理界面

## 6. 完成定义

- `make generate` 后生成物与契约一致
- `make fmt && make lint && make test` 全绿
- `bun run gen:api && bun run test:run && bun run typecheck` 全绿
- 用播种账号可以登录、看到 `/console`、登出后回到 `/login`
