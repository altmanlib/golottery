# golottery-mp

微信小程序宾客端 / 管理入口骨架

需求与技术方案见仓库根目录 `docs/`：[PRD.md](../docs/PRD.md)、[DESIGN.md](../docs/DESIGN.md)。小程序相关的阶段方案是 [phase-6-checkin.md](../docs/phases/phase-6-checkin.md)。

## 开发

用微信开发者工具打开本目录。`appid` 当前为 `touristappid`，正式环境替换为平台小程序 AppID。

活动上下文（`event publicId`）扫小程序码时从 `decodeURIComponent(query.scene)` 取，按路径打开时从启动参数 `e` 取。当前 `app.ts` 只处理 `e`，`scene` 在阶段 6 补上。
