---
title: 后端 API 文档索引
type: guide
status: published
updated: 2026-09-23
---

# 后端 API 文档索引

| 文档 | 内容 |
| --- | --- |
| [PRD.md](PRD.md) | 多租户、签到、抽奖与接口侧产品需求 |
| [DESIGN.md](DESIGN.md) | 数据、鉴权、事务与 API 技术方案 |
| [MODULES.md](MODULES.md) | 包结构、模块契约、事务边界与落地顺序 |
| [ROADMAP.md](ROADMAP.md) | 后端待办与已知缺口登记 |

本目录文档以本项目为边界；跨端接口以 `api/openapi.yaml` 为准。文档采用 YAML front matter（`title`、`type`、`status`、`updated`），标题与 `title` 一致、相对链接可解析；本项目允许 `type: prd`，原有大写文件名保留
