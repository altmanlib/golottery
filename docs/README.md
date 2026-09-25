---
title: 文档索引
type: guide
status: published
updated: 2026-09-25
---

# 文档索引

本仓库文档描述「定位签到 + 现场抽奖」的多租户 SaaS：活动引擎可复用，SaaS 壳先做薄层。

## 1. 文档入口

| 文档 | 说明 |
| --- | --- |
| [PRD.md](./PRD.md) | 产品需求 |
| [DESIGN.md](./DESIGN.md) | 技术方案 |
| [phases/README.md](./phases/README.md) | 实施阶段、门禁与状态 |
| [ROADMAP.md](./ROADMAP.md) | 待办与已知缺口的唯一登记处 |

## 2. 文档约定

- 适用范围：`docs/` 下所有 Markdown 文档
- YAML front matter 包含 `title`、`type`、`status`、`updated`；`title` 与一级标题一致；相对链接必须可解析
- `type` 取 `guide` / `runbook` / `reference` / `design` / `record`，本地追加 `prd`（仅 [PRD.md](./PRD.md)）
- `status` 取 `draft` / `published` / `deprecated`；`updated` 格式为 `YYYY-MM-DD`
- 阶段方案的标题统一为「阶段 N：名称」；未决问题写入该阶段的「开放项」表，并注明须在哪个阶段开工前定
- `reference/` 存放第三方手册原文，`plan/` 存放阶段内部临时笔记，二者不属于本约定的适用范围
- 章节默认编号
- `status` 只描述文档是否可依赖，不表示代码交付进度
- 阶段进度只在 [phases/README.md](./phases/README.md) 的状态表中维护
- 待办与已知缺口只在 [ROADMAP.md](./ROADMAP.md) 登记
- 上述可机器检查的条款（front matter、标题一致、相对链接与锚点、阶段文件名与标题、阶段状态取值、`P-` 编号不重复）由仓库根目录的 `scripts/check-docs.ts` 校验，CI 执行
