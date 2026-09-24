---
title: 阶段 2：运营认证
type: design
status: published
updated: 2026-09-24
---

# 阶段 2：运营认证

## 1. 目标与范围

在工程基线上交付平台运营后台的账号认证。凭证继续用已有的 Bearer API Token：明文只返回一次，库内只存 SHA-256，吊销即删行。

本阶段只做**平台运营账号**这一类主体，`principal_type = platform`。后续开通组织、调整配额、创建组织管理员都挂在这个会话后面。

命名约定：`platform` 专指平台运营，API 前缀 `/api/platform`，页面前缀 `/platform`；「控制台」专指组织管理员使用的 Web 控制台，API 前缀 `/api/organization`，令牌类型 `console`。

做：

1. `platform_users` 表与启动时播种一个运营账号
2. `POST /api/platform/login`、`POST /api/platform/logout`、`GET /api/platform/me`、`POST /api/platform/password`
3. `/api/platform/*` 除登录外都要求 platform 令牌
4. 登录失败限速，账号不存在时仍做一次 argon2 校验
5. 运营登录页接上真实接口；基线占位路由 `/login`、`/console` 改为 `/platform/login`、`/platform`

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

配置：本阶段新增 ScopeInfra 键 `PLATFORM_USER`、`PLATFORM_PASSWORD_HASH`，写入 [技术方案 §10](../DESIGN.md#10-配置)。两者只在 `platform_users` 为空时必填，表非空时可以不配。`PHC` 哈希含 `$`，写进 `.env` 时用单引号包裹。`PLATFORM_SESSION_TTL`、`LOGIN_MAX_FAILURES`、`LOGIN_WINDOW` 已存在。

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

启动时若表为空，用 `PLATFORM_USER` 与 `PLATFORM_PASSWORD_HASH` 插入一行；表为空且两键缺任一时拒绝启动。表非空时不再改已有口令，避免重启覆盖人工改密。

`auth.LoginLimiter` 放在 `internal/auth`：按 `platform:<username>` 计数，窗口与阈值取配置。达到阈值返回 `429 E_TOO_MANY_ATTEMPTS`，分钟数向上取整且至少为 1。成功登录删除该键的失败记录。超出窗口的失败记录与过期令牌由 [阶段 9](phase-9-launch.md) 的清理命令删除。

限速键用用户名原文，达到阈值后正确口令也返回 `429`。这会让他人故意输错来锁住运营账号；运营账号只有一个且入口不对外公布，首发接受这个风险。

### 4.2 接口

全部写入 `openapi.yaml`，由 `apihttp` 实现。

| 方法 | 路径 | 鉴权 | 说明 |
| --- | --- | --- | --- |
| POST | `/api/platform/login` | 无 | `{username, password}` → `{token, expires_at}` |
| POST | `/api/platform/logout` | platform | 删除当前令牌，`204` |
| GET | `/api/platform/me` | platform | `{username}` |
| POST | `/api/platform/password` | platform | `{current_password, new_password}` → `{token, expires_at}` |

登录失败、令牌缺失、令牌类型不符、令牌过期，统一 `401 E_UNAUTHORIZED` 或 `401 E_INVALID_CREDENTIALS`。口令错误用 `E_INVALID_CREDENTIALS`；未带或无效令牌用 `E_UNAUTHORIZED`。新口令少于 8 个字符返回 `400 E_PASSWORD_TOO_SHORT`；与当前口令相同返回 `400 E_PASSWORD_UNCHANGED`；当前口令不对返回 `400 E_CURRENT_PASSWORD_WRONG`。当前口令错误不用 `401`：前端遇到 `401` 会清除令牌并跳登录页，输错一次就被登出。

本阶段向 `bizerr` 与 [技术方案 §7.2](../DESIGN.md#72-错误体与错误码) 补充：

| HTTP | Code | 文案 |
| --- | --- | --- |
| 400 | `E_PASSWORD_UNCHANGED` | 新口令不能与当前口令相同 |
| 400 | `E_CURRENT_PASSWORD_WRONG` | 当前口令不正确 |

### 4.3 前端

- `/platform/login` 提交用户名与口令，成功后 `setToken` 并进入 `/platform`
- 登录失败展示接口返回的 `message`
- `/platform` 在没有令牌时直接去 `/platform/login`；有令牌时请求 `GET /api/platform/me`，失败交给 `401` 处理
- `401` 跳转目标改为 `/platform/login`；根路径 `/` 暂时重定向到 `/platform`
- 不在本阶段做改密页面

### 4.4 测试

| 对象 | 用例 |
| --- | --- |
| limiter | 未达阈值允许；达到阈值返回剩余等待；成功后计数清零 |
| login | 正确口令返回可校验的 platform 令牌；错误口令与不存在用户都是 `401`；连续失败触发 `429` |
| logout | 当前令牌删除后再次访问 `401`；其他主体的令牌不受影响 |
| password | 成功后旧令牌失效、响应中的新令牌有效；短口令与相同口令被拒绝；当前口令错误返回 `400` 且令牌仍有效 |
| seed | 表为空时按配置播种；表非空时不覆盖口令；表为空且缺配置时拒绝启动 |
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
- 用播种账号可以登录、看到 `/platform`、登出后回到 `/platform/login`
- [技术方案 §8](../DESIGN.md#8-前端结构) 的路由表与测试边界同步更新
