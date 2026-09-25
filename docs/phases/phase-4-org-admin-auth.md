---
title: 阶段 4：组织管理员认证
type: design
status: published
updated: 2026-09-25
---

# 阶段 4：组织管理员认证

## 1. 目标与范围

给组织一张自己的控制台入口。运营在 `platform` 会话里为组织创建管理员；管理员用邮箱和口令登录，拿到只属于该组织的 `console` 令牌。

做：

1. `org_users` 表
2. 运营创建管理员、重置口令、停用与启用账号
3. 管理员登录、登出、查看自己、修改口令
4. `console` 令牌绑定一个 `org_id`，不能访问其他组织

不做：

- 开通组织、调整场次。见 [phase-3-org-quota.md](phase-3-org-quota.md)
- 活动、围栏、奖项、名单
- 主持人与宾客微信登录
- 自助注册、邀请链接、找回口令
- 一个账号管理多个组织
- 审计日志

## 2. 现状与约束

| 项 | 现状 |
| --- | --- |
| 令牌 | `auth.TokenIssuer` 已能签发 `console` 类型，但还没有登录入口 |
| 运营认证 | [阶段 2](phase-2-platform-auth.md) 提供 `platform` 令牌与登录限速 |
| 组织 | [阶段 3](phase-3-org-quota.md) 提供 `orgs.id` 与 `orgs.status` |
| 隔离 | 管理员只能看见令牌里的 `org_id` |

创建管理员的接口依赖组织表，因此排在组织与配额之后实现。方案可以先定。

## 3. 原则

1. `org_users` 与 `platform_users` 分开，不共用账号行
2. `console` 与 `platform` 令牌不可互换
3. 令牌里的组织以 `api_tokens.principal_id` 保存的 `org_user_id` 为准，服务端再查出 `org_id`，不接受客户端另传组织
4. 停用账号或停用组织后，该账号已签发的 `console` 令牌立即失效。停用账号时删除其令牌；停用组织不批量删令牌，由中间件在每次请求校验 `org_users.status` 与 `orgs.status`
5. 重置口令或修改口令时删除该账号全部令牌
6. 账号是否存在不通过响应时间或文案区分

## 4. 方案

### 4.1 数据

新增迁移 `004_org_users.sql`。编号排在组织配额迁移之后；前置迁移号变化时顺延，不复用。

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| `id` | uuid | 主键，应用生成 |
| `org_id` | uuid | 非空，外键 → `orgs`，索引 |
| `email` | varchar(254) | 非空；全局唯一，保存前转小写并去掉首尾空白 |
| `name` | varchar(100) | 非空 |
| `password_hash` | text | argon2id |
| `status` | varchar(16) | `active` / `disabled` |
| `created_at` / `updated_at` | timestamptz | |

一个邮箱全局只能属于一个组织。首发不做同一人切换多个组织。

临时口令由服务端生成，10 位，去掉容易混淆的字符。明文只在创建和重置的响应里出现一次，库内只存哈希。

### 4.2 运营接口

全部要求 `platform` 令牌。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/platform/orgs/:id/users` | 该组织的管理员列表（按创建时间），不含口令哈希 |
| POST | `/api/platform/orgs/:id/users` | `{name, email}` → `{user: {id, name, email, status, created_at}, password}` |
| POST | `/api/platform/orgs/:id/users/:userId/reset-password` | 返回一次性新口令 |
| POST | `/api/platform/orgs/:id/users/:userId/disable` | 停用并删除该账号全部令牌 |
| POST | `/api/platform/orgs/:id/users/:userId/enable` | 启用，不改口令 |

组织不存在返回 `404 E_NOT_FOUND`。邮箱重复返回 `409 E_CONFLICT`。邮箱格式不合法或姓名为空返回 `400 E_BAD_REQUEST`。用户不属于路径中的组织时返回 `404 E_NOT_FOUND`，不暴露该用户属于别的组织。

### 4.3 管理员接口

| 方法 | 路径 | 鉴权 | 说明 |
| --- | --- | --- | --- |
| POST | `/api/organization/login` | 无 | `{email, password}` → `{token, expires_at, org_id}` |
| POST | `/api/organization/logout` | console | 删除当前令牌，`204` |
| GET | `/api/organization/me` | console | `{name, email, org_id, org_name}`；`org_name` 用于控制台显示本组织名称 |
| POST | `/api/organization/password` | console | `{current_password, new_password}` → `{token, expires_at}` |

登录按 `org:<email>` 限速，规则与运营登录相同。邮箱不存在、口令错误、账号停用、所属组织停用，对外都返回 `401 E_INVALID_CREDENTIALS`。新口令少于 8 个字符返回 `400 E_PASSWORD_TOO_SHORT`；与当前口令相同返回 `400 E_PASSWORD_UNCHANGED`。

后续组织侧业务接口统一挂在 `/api/organization/*`，中间件只接受 `console` 令牌，并把解析出的 `org_id` 放进请求上下文。

### 4.4 前端

Web 按令牌类型分两条入口。令牌按主体分开存储：`gl.token.platform`、`gl.token.console`，阶段 7 再加 `gl.token.host`。同一浏览器同时登录运营、组织控制台或大屏时互不覆盖。`src/api.ts` 按请求路径前缀选择令牌，`gl.token` 在本阶段删除：

- `/platform/login` 继续是运营登录
- `/organization/login` 是组织管理员登录
- 运营的组织详情里可以创建管理员，并一次性展示临时口令
- 管理员登录后进入 `/organization`，只看到自己的组织

`401` 时按请求 URL 清除对应令牌并回对应登录页：`/api/organization/*` 清 `gl.token.console` 并回 `/organization/login`，`/api/platform/*` 清 `gl.token.platform` 并回 `/platform/login`。只在当前页面仍属于该入口时跳转，避免一个迟到的 `401` 把用户从另一个入口拉走；跳转经路由器执行，不直接改 `location.hash`。登录接口自身的 `401` 不触发跳转，由表单展示错误。本阶段不在 `/organization` 做活动页面。

### 4.5 测试

| 对象 | 用例 |
| --- | --- |
| 创建 | 临时口令只出现在响应里；库内哈希可校验；重复邮箱返回 `409` |
| 登录 | 正确口令得到 `console` 令牌；该令牌不能访问 `/api/platform/me` |
| 隔离 | 令牌只能解析出自己的 `org_id` |
| 停用 | 停用账号或停用组织后，旧令牌访问 `/api/organization/me` 返回 `401` |
| 重置与改密 | 旧令牌失效；响应里的新口令或新令牌可用 |
| 限速 | 同一邮箱连续失败达到阈值返回 `429` |

前端只测登录提交值、临时口令只展示一次的状态，以及 `401` 跳转目标。

## 5. 明确不做

- 管理员邀请邮件
- 一个邮箱加入多个组织
- 组织内角色分级
- 管理员自行创建其他管理员。PRD O5（P1）的「邀请组织管理员」首发由运营代建替代，登记在 [ROADMAP](../ROADMAP.md)
- 主持人口令

## 6. 完成定义

- 迁移可重复执行，测试用真实 PostgreSQL
- `make generate` 后生成物与契约一致
- `make fmt && make lint && make test` 全绿
- `bun run gen:api && bun run test:run && bun run typecheck` 全绿
- 运营可以创建管理员；管理员可以登录并只能访问自己的组织
