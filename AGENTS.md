# golottery

定位签到 + 现场抽奖的多租户 SaaS。本文件是 AI 助手在本仓库工作的总约定；子项目细则见各自的 `AGENTS.md`。

## 1. 入口

| 位置 | 内容 |
| --- | --- |
| [docs/README.md](docs/README.md) | 文档索引与文档约定 |
| [docs/ROADMAP.md](docs/ROADMAP.md) | 待办与已知缺口；开工前先读 |
| [docs/phases/README.md](docs/phases/README.md) | 阶段门禁与状态 |
| [golottery-api/AGENTS.md](golottery-api/AGENTS.md) | 后端约定 |
| [golottery-web/AGENTS.md](golottery-web/AGENTS.md) | 前端约定 |
| [golottery-mp/AGENTS.md](golottery-mp/AGENTS.md) | 小程序约定 |

## 2. Git 工作流

- AI 助手只在长期分支 `agent/develop` 上开发，不在 `main` 上直接提交
- 仓库所有者也会直接推送 `main` 与 `agent/develop`，所以开工前、合并前都先拉取最新代码
- 不 rebase、不 amend 已推送的提交、不强制推送、不删除远程分支；需要同步时用 merge
- 不走 PR。合并到 `main`、打 tag、发布、部署都要所有者明确同意
- 提交信息沿用约定式前缀：`feat:`、`fix:`、`docs:`、`chore:`、`style:`、`ci:` 等，可带范围如 `feat(api):`

### 2.1 开工

```bash
git fetch origin
git checkout agent/develop
git merge origin/agent/develop
git merge origin/main
```

### 2.2 开发中

分支上可以随时提交，并推送到远程：`git push -u origin agent/develop`。每次推送都会触发 CI。

### 2.3 合并到 main

功能完成、[§3 完成定义](#3-完成定义)全部满足、所有者确认后：

```bash
git fetch origin
git merge origin/agent/develop                 # 在 agent/develop 上，先把两边最新改动合进来并重跑检查
git merge origin/main
git checkout main
git merge --ff-only origin/main
git merge --squash agent/develop
git commit                                     # 一个功能一个提交，信息概括整体改动
git push -u origin main
git checkout agent/develop
git merge main
git push -u origin agent/develop
```

squash 之后 `main` 上的提交号才确定，[ROADMAP](docs/ROADMAP.md) 的提交号用一个单独的 `docs:` 提交回填。

`main` 合回 `agent/develop` 时，git 不认为 squash 提交与分支上的提交同源，回填提交号的行会冲突。此时两边内容本应一致，冲突处一律以 `main` 为准（`git checkout --theirs <文件>`），合并后 `git diff main` 应为空。

## 3. 完成定义

提交合并前，以下检查必须全绿；CI（`.github/workflows/ci.yml`）执行同样的检查。

| 范围 | 命令（在对应目录执行） |
| --- | --- |
| api | `make fmt && make lint && make check-generate && make test && make smoke` |
| api 镜像 | `docker build -t golottery-api:dev golottery-api`（仓库根目录） |
| web | `bun run format && bun run test:run && bun run typecheck && bun run build` |
| web SDK | `bun run gen:api` 后 `src/api-gen/` 无改动 |
| docs | `bun scripts/check-docs.ts`（仓库根目录） |

- 验证真实产物，不只跑测试：Go 二进制要真实构建并启动后请求 `/readyz`（`make smoke`），前端要跑 `vite build`，Dockerfile 改动要真实构建镜像
- 写完测试后故意改坏实现，确认测试会失败，再恢复；不会失败的测试不算数
- 本机依赖用 `golottery-api/compose.yml` 启动；`make test` 与 `make smoke` 需要可达的 PostgreSQL 与 Redis

## 4. 文档与路线图

- 实现与设计不一致时，先改 [DESIGN.md](docs/DESIGN.md) 或 [PRD.md](docs/PRD.md)，再改代码
- 计划有变（新增、推迟、决定不做）时，在同一个提交里更新 [ROADMAP.md](docs/ROADMAP.md)，文档与实际打算保持一致
- 文档改动须通过 `bun scripts/check-docs.ts`
