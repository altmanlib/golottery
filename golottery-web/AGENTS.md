# golottery-web

控制台与大屏前端

需求与技术方案只读本项目 `docs/`：[docs/PRD.md](docs/PRD.md)、[docs/DESIGN.md](docs/DESIGN.md)。

## 常用命令

```bash
bun dev           # http://localhost:3000
bun run typecheck
bun run build
bun run format
bun run gen:api   # 需 golottery-api 已启动并提供 /openapi.json
```

## 约定

- 视觉 token 在 `src/theme.ts`；`brand` 第 6 阶为 `#1E4544`
- 样式用 CSS Modules；全局只留 `src/styles/base.css`
- 字体自托管：`@fontsource/roboto`（400/500/700）、`roboto-condensed`（700）、`roboto-mono`（500）
- 结构用 Mantine 布局组件，不手写 `display: flex/grid`
- 图标后续统一用 `@tabler/icons-react`
- 不写视觉测试；测试只覆盖纯函数与接口契约

## OpenAPI 客户端

- 配置：`openapi-ts.config.ts`
- 源契约：`golottery-api/api/openapi.yaml`
- 生成产物：`src/api-gen/`（勿手改）
- 运行时：入口 `import '#/api'`。`src/api.ts` 把 `baseUrl` 覆写为空字符串，按 `gl.token` 附加 Bearer，`401` 清令牌并跳 `/login`

业务代码只从 `#/api-gen/sdk.gen` / `#/api-gen/types.gen` 引用 API，禁止手写 fetch 封装重复描述同一接口。
