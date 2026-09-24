# golottery-web

golottery 前端：控制台、组织端与大屏 SPA（Vite + React 19 + Mantine）

需求与技术方案见仓库 `docs/`。阶段方案在 `docs/phases/`。

## 常用命令

```bash
bun run dev        # http://localhost:3000；代理 /api /healthz /readyz /openapi.json /openapi.yaml → :5568
bun run format
bun run test:run
bun run typecheck
bun run build
bun run gen:api    # 需 golottery-api 已启动并提供 /openapi.json
```

## 约定

- 视觉 token 定义在 `src/theme.ts` 的 Mantine 主题（`colors` / `radius` / `fontSizes` / `fontFamily` / `other`）；Mantine 生成 `--mantine-*`，主题词汇覆盖不到的值经 `cssVariablesResolver` 输出，`brand` 第 6 阶为 `#1E4544`
- 样式用 CSS Modules：全局只留 `src/styles/base.css`（reset，`main.tsx` 引入）。组件视觉规则写在同目录 `*.module.css`，`import classes from './X.module.css'` 后用 `className={classes.x}`。Mantine 内部节点用 `classNames={classes}`（键名对齐 Styles API），不要用全局类名覆盖 `.mantine-*`。不新增全局样式文件，不跨组件复用类名；确实共用的壳做成组件再引用
- 字体自托管：`@fontsource/roboto`（400/500/700）、`roboto-condensed`（700）、`roboto-mono`（500）
- 不做运行时配置；只请求相对路径
- 图标统一用 `@tabler/icons-react`；控制台常见尺寸 `size={16}`，返回箭头 `14`
- 插画取自本机 unDraw 库（`~/code/fishx/illustration/undraw`），拷入 `public/illustrations/`，并把默认主色 `#6c63ff` 改成品牌青 `#55807E`（brand-4）；装饰性插画一律 `alt=""`，空状态插画宽度 `132`，登录页 `min(280px, 68vw)`
- 控制台与组织端密度按 Data-Dense：间距只用 `4/8/12/16/24`，卡片 padding `12`，主区 `16×24`，页面标题 `18`
- 列表页不出现整页滚动，表格区撑满剩余高度并内部滚动（表头 sticky）
- 列表走后端 `offset`/`limit` 分页（默认 40），页码写在 URL；底栏用 `Pagination`
- 动效只做 CSS 微交互：路由淡入写在对应壳的 CSS Module；弹层用 Mantine `Modal`（默认 portal 到 `body`）；尊重 `prefers-reduced-motion`
- 加载用 `TableSkeleton`；空列表用 `EmptyState` 并带 CTA
- toast 右下角
- 运营后台在 `src/platform/`，路由前缀 `/platform`；组织端在 `src/organization/`，路由前缀 `/organization`；大屏在 `src/host/`，路由前缀 `/host`。三块都懒加载
- 分层：查询、变更与表单状态放各自目录的 `hooks/`，可复用视图块放 `components/`，页面只做装配；query key 集中在该目录的 `queryKeys.ts`
- 筛选写入 URL
- 不写视觉/UI 测试：不为 token、主题、CSS 变量、类名、内联样式或计算样式写 `*.test.tsx`；前端测试只覆盖纯函数与接口契约
- 不对 UI 视觉做验收：不截图比对、不靠浏览器肉眼确认样式/遮罩/动效；功能正确性用 format / test / typecheck / build 与接口行为验证，视觉由人确认
- 结构用 Mantine 布局组件（映射见下），不手写 `display: flex/grid`；颜色、边框、阴影、圆角、动效由 CSS 与 Mantine 主题（`--mantine-*`）承担

## OpenAPI 客户端

- 配置：`openapi-ts.config.ts`
- 源契约：`golottery-api/api/openapi.yaml`
- 生成产物：`src/api-gen/`（勿手改）
- 运行时：入口 `import '#/api'`。`src/api.ts` 把 `baseUrl` 覆写为空字符串，按请求路径前缀附加对应令牌：`/api/platform/*` 用 `gl.token.platform`，`/api/organization/*` 用 `gl.token.console`，`/api/host/*` 用 `gl.token.host`
- `401` 删除对应令牌：`/api/platform/*` 跳 `/platform/login`，`/api/organization/*` 跳 `/organization/login`，`/api/host/*` 回当前活动的 `/host/:publicId`；登录接口自身的 `401` 不跳转

业务代码只从 `#/api-gen/sdk.gen` / `#/api-gen/types.gen` 引用 API，禁止手写 fetch 封装重复描述同一接口。

## 布局组件

完整手册见仓库根目录 `docs/reference/mantine-ui-library.txt`（按组件名检索章节）

| 场景 | 组件 |
| --- | --- |
| 垂直排列 | `Stack gap` |
| 水平排列 | `Group gap`；需要非居中时显式传 `align` |
| 需控制 `wrap` / 响应式 `direction` | `Flex` |
| 等宽网格 | `SimpleGrid cols spacing verticalSpacing` |
| 不等宽网格 | `Grid` + `Grid.Col span` |
| 粘性侧栏 | `Box pos="sticky" top={12}` |
| 留白占位 | `Space h` |
| 居中 | `Center` |
| 语义容器 / 透传样式 | `Box` |
| 卡片面板 | `Paper withBorder radius="md" p` |

- 数值 1:1 映射：Mantine 把数字按像素转 rem，`gap={12}` 即 12px
- `SimpleGrid` 的 `spacing` 是水平间距、`verticalSpacing` 是垂直间距，不要写反
- `Group` 默认 `align="center"`，替换手写行容器时行为一致
- 布局类 CSS 只在承载响应式或视觉规则时保留，放进对应组件的 CSS Module；纯结构部分用 Mantine 布局组件，不留死规则
