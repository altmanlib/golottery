# golottery-web

控制台与大屏前端

技术栈：Vite · React 19 · Mantine 8 · TanStack Query · Zustand

需求与技术方案见 [docs/README.md](docs/README.md)

## 快速开始

```bash
# 依赖
bun install

# 需先启动 golottery-api（默认 :5568）
bun dev   # http://localhost:3000
```

开发态通过 Vite 代理转发后端：

| 前缀 | 目标 |
| --- | --- |
| `/api` | `http://127.0.0.1:5568` |
| `/healthz` | 同上 |
| `/openapi.json` / `/openapi.yaml` | 同上 |

## 常用命令

| 命令 | 说明 |
| --- | --- |
| `bun dev` | 本地开发（端口 `3000`） |
| `bun run build` | 类型检查 + 生产构建 |
| `bun run typecheck` | 仅 TypeScript 检查 |
| `bun run preview` | 预览构建产物 |
| `bun run format` | Biome 格式化 / lint 修复 |
| `bun run gen:api` | 从运行中的 API 重新生成客户端 |
| `bun test` / `bun run test:run` | Vitest 监听 / 单次 |
| `bun run test:coverage` | 覆盖率报告 |

## 页面路由

使用 Hash Router：

| 路径 | 说明 |
| --- | --- |
| `#/login` | 登录 |
| `#/console` | 组织方控制台 |
| `#/host` | 现场大屏 / 主持人 |

## OpenAPI 客户端

- 配置：`openapi-ts.config.ts`
- 输入：运行中 API 的 `http://127.0.0.1:5568/openapi.json`
- 产物：`src/api-gen/`（禁止手改）
- 运行时：`src/lib/client.ts` 将 `baseUrl` 覆写为空字符串，请求走 Vite 代理

业务代码只从 `#/api-gen/sdk.gen` / `#/api-gen/types.gen` 引用 API，禁止手写 fetch 封装重复描述同一接口

```bash
# API 已启动后
bun run gen:api
```

## 目录结构

```text
src/
  api-gen/       OpenAPI 生成客户端（勿手改）
  assets/        全局样式
  components/    可复用 UI
  hooks/         自定义 hooks
  layouts/       布局（控制台壳）
  lib/           客户端、主题、QueryClient 等
  router/        路由定义
  stores/        Zustand store
  test/          Vitest setup
  views/         页面视图
```

路径别名：`#/*` → `src/*`
