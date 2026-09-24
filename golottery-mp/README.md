# golottery-mp

微信小程序宾客端 / 管理入口骨架

技术栈：原生小程序 · TypeScript · 微信开发者工具

需求与技术方案见仓库根目录 [docs/README.md](../docs/README.md)

## 快速开始

1. 安装并打开[微信开发者工具](https://developers.weixin.qq.com/miniprogram/dev/devtools/download.html)
2. 选择「导入项目」，目录指向本仓库根目录
3. AppID 当前为 `touristappid`（游客模式）；正式环境替换为平台小程序 AppID（见 `project.config.json`）

无需 `npm install`：当前脚手架未引入 npm 依赖。后续若启用 npm 构建，在本目录执行安装后，于开发者工具中「工具 → 构建 npm」

## 活动上下文

平台单小程序 + 活动码入场。启动参数 `e` 携带 `event publicId`，在 `app.ts` 的 `onLaunch` 写入 `globalData.eventPublicId`：

```text
# 示例路径（开发者工具「编译模式」可模拟）
pages/index/index?e=<event_public_id>
```

首页会读取并展示该 ID，无活动上下文时，后续业务页应视为不可用

## 目录结构

```text
app.ts / app.json / app.wxss   小程序入口与全局样式
pages/index/                   占位首页（展示活动上下文）
typings/                       全局 TypeScript 声明（如 IAppOption）
project.config.json            开发者工具项目配置
sitemap.json                   索引配置
```

## 权限与隐私

`app.json` 已声明定位相关能力，用于现场签到：

| 项 | 说明 |
| --- | --- |
| `requiredPrivateInfos` | `getLocation`、`chooseLocation` |
| `permission.scope.userLocation` | 用途说明：活动现场定位签到 |
