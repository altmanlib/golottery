# golottery-mp

微信小程序宾客端 / 管理入口骨架

需求与技术方案只读本项目 `docs/`：[docs/PRD.md](docs/PRD.md)、[docs/DESIGN.md](docs/DESIGN.md)。

## 开发

用微信开发者工具打开本目录。`appid` 当前为 `touristappid`，正式环境替换为平台小程序 AppID。

活动上下文通过启动参数 `e`（`event publicId`）传入，见 `app.ts`。
