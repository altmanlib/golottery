# golottery-api

定位签到抽奖后端。工程基线对齐 [docs/DESIGN.md](../docs/DESIGN.md)。

## Module

- 模块路径：`golottery/api`
- 二进制：`golottery`
- Go 版本：`go 1.27`

## 目录结构

```text
cmd/golottery/         # 装配、优雅关闭、settings / hash-password
internal/
  config/              # 配置 registry 与分层解析
  settings/            # settings 表
  store/               # gorm、连接池、显式 SQL 迁移、测试辅助
  store/migrations/    # embed 的 NNN_*.sql
  bizerr/              # 错误码与中文文案
  httpapi/             # echo、中间件、/readyz、安全响应头
  auth/                # API Token、argon2id、LoginAttempt、LoginLimiter
  platform/            # 平台运营账号：实体、播种、登录 / 登出 / 改密
  apihttp/             # OpenAPI strict handler
api/                   # openapi.yaml 与生成物，禁止手改 *.gen.go
```

## 依赖规则

```text
main ──→ 全部
apihttp ──→ api  auth  bizerr  platform  echo
platform ──→ auth  bizerr  gorm
auth / settings ──→ gorm
store ──→ gorm
config ──→ os  godotenv
httpapi ──→ echo
```

- 不设 `repo` / `services` 层
- `httpapi` 不 import 生成包；OpenAPI 路由由 `apihttp` 挂载
- 业务领域包不得反向依赖 `apihttp`

## 常用命令

```bash
make tidy
make fmt
make lint
make test           # 需可达的 PostgreSQL；默认 127.0.0.1:15436/golottery_test
make run
make dev            # 加载 .env 后运行
make build
make generate       # 根据 api/openapi.yaml 生成 api/*.gen.go
make check-generate # 生成物与契约不一致时失败（会顺带重新生成）
make smoke          # 构建二进制、在临时目录启动并请求 /readyz、/healthz、/openapi.json
```

本机依赖：`docker compose up -d`（见 `compose.yml`）

## OpenAPI

- 契约唯一来源：`api/openapi.yaml`
- 生成配置：`api/models.yaml`、`api/server.yaml`
- 生成入口：`api/generate.go`
- 修改契约后必须 `make generate`，并保证工作区里生成物与契约一致
- 需要令牌的接口在契约里写 `security`（如 `platformBearer`），`apihttp` 据此校验；新增安全方案时同步 `apihttp/security.go` 的 `schemeTokenTypes`，否则启动失败
- 业务错误由处理函数返回 `bizerr`，契约里用 `components/responses/Error` 声明

## 配置分层

- ScopeInfra：`DATABASE_URL`、`APP_HOST`、`APP_PORT`、`SESSION_SECRET`、`TRUSTED_PROXIES`、`PLATFORM_USER`、`PLATFORM_PASSWORD_HASH`
- `platform_users` 为空时必须配 `PLATFORM_USER` 与 `PLATFORM_PASSWORD_HASH`（哈希含 `$`，`.env` 里用单引号）
- ScopeApp：`CONSOLE_SESSION_TTL`、`HOST_SESSION_TTL`、`PLATFORM_SESSION_TTL`、`LOGIN_MAX_FAILURES`、`LOGIN_WINDOW`
- 优先级：`settings > .env > 环境变量 > 默认值`（仅 ScopeApp）
- 必填项缺失或 `SESSION_SECRET` 不足 32 字符时拒绝启动

## 错误码

面向用户的文案集中在 `internal/bizerr`；句尾不用中文句号 `。`

## 质量门禁

完成需求前必须 `make fmt && make lint && make check-generate && make test && make smoke` 全绿；改动 `Dockerfile` 或依赖时再真实构建一次镜像。完整清单见仓库根目录 `AGENTS.md`

`golangci-lint` 须用不低于 `go.mod` 的 Go 版本编译：`go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0`
