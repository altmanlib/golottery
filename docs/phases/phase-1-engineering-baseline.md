---
title: 阶段 1：工程基线
type: guide
status: published
updated: 2026-09-24
---

# 阶段 1：工程基线

## 1. 目标

建立可启动、可测试、边界清晰的 Go 后端与 Web 骨架。本阶段不交付签到、抽奖、组织与活动，但后续功能都在这个基线上增量实现。

## 2. 前置门禁

- [PRD](../PRD.md) 与 [技术方案](../DESIGN.md) 状态为 `published`
- 本机有 Go 1.27、golangci-lint、Bun
- `golottery-api/compose.yml` 能启动 PostgreSQL 18（`127.0.0.1:15436`，库 `golottery` / `golottery_test`，账号 `postgres/secret`）

## 3. 交付范围

后端按 [技术方案 §4.2](../DESIGN.md#42-目录) 建立：

- `cmd/golottery`：装配、优雅关闭、`settings`、`hash-password`
- `internal/config`：registry 与分层解析；必填项缺失或 `SESSION_SECRET` 不足 32 字符时拒绝启动
- `internal/settings`：实体与仓储；`golottery settings list|set|unset`
- `internal/store`：`Open` / `Ping` / `Migrate` / `Close`；连接池 `MaxOpenConns=10` / `MaxIdleConns=5` / `ConnMaxLifetime=1h`；测试辅助 `OpenTest` / `Reset`
- `internal/bizerr`：[技术方案 §7.2](../DESIGN.md#72-错误体与错误码) 的全部 Code 与文案
- `internal/httpapi`：echo、`slog` 中间件、`/readyz`、未知路径空体 404、安全响应头
- `internal/auth`：三种 `typ` 的 API Token、argon2id、`LoginAttempt` 实体
- `internal/apihttp`：现有 OpenAPI 路由（`/healthz`、`/api`、`/openapi.json`、`/openapi.yaml`）
- 迁移 `001_init.sql`：`settings`、`api_tokens`、`login_attempts`
- `Makefile`、`Dockerfile`、`AGENTS.md`

前端：

- 保留现有三条路由为占位页
- 主题 `brand` 第 6 阶 `#1E4544`，字体自托管
- `src/api.ts`：Bearer、`ApiError`、`401` 跳 `/login`
- 业务调用继续走 `#/api-gen`，不手写重复接口

## 4. 完成定义

- `make dev` 后 `GET /healthz` 返回 `db=up`，`GET /readyz` 返回 `200`
- `make fmt && make lint && make test` 全绿
- `bun run format && bun run test:run && bun run typecheck` 全绿
