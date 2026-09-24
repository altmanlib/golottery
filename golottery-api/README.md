# golottery-api

定位签到抽奖后端

技术栈：Go · Echo · GORM · PostgreSQL

需求与技术方案见仓库根目录 [docs/README.md](../docs/README.md)

## 快速开始

```bash
# 1. 环境变量
cp .env.example .env

# 2. 本地 Postgres（默认 127.0.0.1:15436）
docker compose up -d

# 3. 启动 API（默认 :5568）
go run ./cmd/server
```

健康检查：

```bash
curl -s http://127.0.0.1:5568/healthz
```

## 常用命令

| 命令 | 说明 |
| --- | --- |
| `make generate` / `go generate ./...` | 根据 OpenAPI 重新生成 `api/*.gen.go` |
| `make test` / `go test ./...` | 跑测试 |
| `make build` / `go build ./...` | 编译 |
| `make run` / `go run ./cmd/server` | 启动服务 |
| `make tidy` / `go mod tidy` | 整理依赖 |

## 配置

见 `.env.example`：

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `DATABASE_URL` | `postgres://postgres:secret@127.0.0.1:15436/golottery?sslmode=disable` | Postgres 连接串 |
| `API_PORT` | `5568` | HTTP 监听端口 |
| `JWT_SECRET` | `change-me-in-production` | JWT 签名密钥 |

Compose 还会读取 `POSTGRES_DB` / `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_PORT`

## OpenAPI / oapi-codegen

- 契约唯一来源：`api/openapi.yaml`
- 生成配置：`api/models.yaml`、`api/server.yaml`
- 生成入口：`api/generate.go` → `go generate ./...`
- 生成产物：`api/models.gen.go`、`api/server.gen.go`（禁止手改）
- Handler：`internal/handler` 实现 `api.StrictServerInterface`
- 业务与持久化预留：`internal/service`、`internal/repository`、`internal/model`

修改契约后必须重新生成，并保证 `go generate ./...` 后工作区无 diff

当前脚手架已暴露：

| 路径 | 说明 |
| --- | --- |
| `GET /healthz` | 存活与数据库探测 |
| `GET /api` | 服务元信息 |
| `GET /openapi.yaml` | OpenAPI YAML |
| `GET /openapi.json` | OpenAPI JSON |

## 目录结构

```text
api/                 OpenAPI 契约与生成代码
cmd/server/          进程入口
internal/
  config/            环境变量配置
  db/                GORM / Postgres
  handler/           HTTP 路由与 StrictServer 实现
  model/             领域模型（预留）
  repository/        持久化（预留）
  service/           业务逻辑（预留）
compose.yml          本地 Postgres
Dockerfile           多阶段构建（distroless）
```
