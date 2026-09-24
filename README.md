# golottery

面向线下活动的多租户 SaaS：组织方自助开活动，宾客用微信小程序在地理围栏内签到入池，主持人在电脑大屏完成可追溯的现场抽奖。

文档索引：[docs/README.md](docs/README.md) · 进度：[docs/ROADMAP.md](docs/ROADMAP.md)

## 仓库结构

| 目录 | 说明 |
| --- | --- |
| `golottery-api` | 后端：Go + Echo + GORM + PostgreSQL，OpenAPI / oapi-codegen |
| `golottery-web` | 控制台与大屏：Vite + React 19 + Mantine 8 |
| `golottery-mp` | 微信小程序（原生 TypeScript） |
| `docs` | 产品与技术方案 |

## 前置条件

- Go 1.27、golangci-lint、Bun
- Docker（本机数据库用 `golottery-api/compose.yml`）

## 本机依赖

```bash
cd golottery-api
docker compose up -d
```

| 服务 | 地址 |
| --- | --- |
| PostgreSQL | `127.0.0.1:15436`，库 `golottery` / `golottery_test`，账号 `postgres/secret` |
| Adminer | http://127.0.0.1:58033 |
| RustFS（S3） | 接口 `127.0.0.1:57800`，控制台 http://127.0.0.1:57801，账号 `rustfsadmin/rustfsadmin` |
| Redis | `127.0.0.1:57379` |

## 后端

```bash
cd golottery-api
cp .env.example .env   # 填 SESSION_SECRET（≥32）
make dev               # 加载 .env，监听 127.0.0.1:5568
curl -s 127.0.0.1:5568/healthz
curl -s 127.0.0.1:5568/readyz

make fmt && make lint && make test
make build
```

口令哈希：`printf '%s' 'your-password' | go run ./cmd/golottery hash-password`

## 前端

```bash
cd golottery-web
bun install
bun run dev            # http://localhost:3000
bun run format && bun run test:run && bun run typecheck
```

Vite 把 `/api`、`/healthz`、`/readyz`、`/openapi.json`、`/openapi.yaml` 代理到 `127.0.0.1:5568`。

小程序：用微信开发者工具打开 `golottery-mp/`。
