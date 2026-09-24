---
title: 阶段 8：品牌装修
type: design
status: published
updated: 2026-09-24
---

# 阶段 8：品牌装修

## 1. 目标与范围

让每场活动在大屏和小程序上呈现组织自己的品牌。首发只做「固定位置换图、改文字、换主题色」，不做自由排版。年会的焦点在大屏，品牌呈现直接影响成交，因此排在上线之前。

做：

1. 对象存储接入与图片素材上传
2. 活动品牌配置：显示名、副标题、主题色、Logo、大屏背景、小程序封面
3. 大屏与小程序按配置渲染
4. 运营下架违规素材

不做：

- 自由排版、页面编辑器、自定义字体
- 视频或动图背景
- 组织级品牌库（多场活动共用一套品牌）。数据结构已预留，见 §4.1
- 自有小程序名称与图标。走微信第三方平台，见 [PRD §2](../PRD.md)
- 图片裁剪、压缩与内容自动审核

## 2. 现状与约束

| 项 | 现状 |
| --- | --- |
| 对象存储 | 本机 Compose 提供 RustFS（S3 兼容），`127.0.0.1:57800`，控制台 `57801` |
| 大屏 | [阶段 7](phase-7-draw.md) 已按主题变量与固定品牌位搭好布局，颜色不写死 |
| 小程序 | [阶段 6](phase-6-checkin.md) 活动页已预留封面位，标题栏显示活动名 |
| 安全响应头 | 所有 API 响应带 `Cache-Control: no-store`（[技术方案 §6.3](../DESIGN.md#63-安全响应头)） |
| `<img>` 与小程序 `<image>` | 不能携带 `Authorization` 头 |
| 定价 | 所有档位可用，不单独收费 |

## 3. 原则

1. 只依赖 S3 协议，不依赖 RustFS 特有接口；生产可换任意 S3 兼容存储
2. 客户端只使用服务端返回的 `url`，不自行拼接地址。以后改走 CDN 只改配置
3. 素材归属组织，活动只引用素材 ID。组织级品牌库以后直接复用素材表
4. 对象键包含素材 ID，素材 ID 永不复用，因此素材地址可以永久缓存
5. 不接受 SVG，避免脚本注入；格式由服务端读取文件头判定，不信任扩展名与 `Content-Type`
6. 对象存储不可用不影响签到与抽奖；`/readyz` 不检查对象存储

## 4. 方案

### 4.1 数据

新增迁移 `008_branding.sql`。前置迁移号变化时顺延。

`assets`

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| `id` | uuid | 主键 |
| `org_id` | uuid | 非空，外键 → `orgs`，索引 |
| `object_key` | varchar(200) | 唯一；`orgs/<org_id>/assets/<id>.<ext>` |
| `mime` | varchar(32) | `image/png` / `image/jpeg` / `image/webp` |
| `bytes` | integer | 非空 |
| `width` / `height` | integer | 非空 |
| `created_by` | varchar(64) | 上传者 `principal_id` |
| `created_at` | timestamptz | |

`event_branding`，与 `events` 一对一

| 字段 | 类型 | 约束 |
| --- | --- | --- |
| `event_id` | uuid | 主键，外键 → `events` |
| `org_id` | uuid | 非空 |
| `display_name` | varchar(60) | 可空；为空时用活动名 |
| `subtitle` | varchar(100) | 默认空串 |
| `theme_color` | char(7) | 可空；`#RRGGBB`，为空时用平台默认色 |
| `logo_asset_id` / `screen_bg_asset_id` / `cover_asset_id` | uuid | 可空，外键 → `assets` |
| `updated_at` | timestamptz | |

组织级品牌库以后新增 `org_branding`，字段与本表相同；活动字段为空时回落到组织值。本阶段不建该表。

### 4.2 素材规格

| 品牌位 | 使用位置 | 建议尺寸 | 大小上限 |
| --- | --- | --- | --- |
| Logo | 大屏左上角 | 高 ≥ 120 px，透明底 PNG | 1 MB |
| 大屏背景 | 大屏全屏，按 `cover` 铺满 | 1920 × 1080 | 5 MB |
| 小程序封面 | 小程序活动页顶部 | 750 × 400 | 2 MB |

上传时统一校验：PNG、JPEG、WebP 之一，单文件 ≤ 5 MB，宽高均 ≤ 4096 px。按品牌位的大小上限在保存品牌配置时校验。不合格返回 `400 E_INVALID_IMAGE`，补进 `bizerr`。宽高用 `image.DecodeConfig` 读取，WebP 依赖 `golang.org/x/image/webp`。

主题色只存一个值。前端据此生成 10 阶色板，并按亮度自动选择黑色或白色文字。大屏背景图上叠一层固定的半透明遮罩，保证任何背景下文字可读。

### 4.3 接口

| 方法 | 路径 | 鉴权 | 说明 |
| --- | --- | --- | --- |
| POST | `/api/organization/assets` | console | `multipart` 单文件 → `{id, url, width, height}` |
| GET · PUT | `/api/organization/events/:id/branding` | console | 读取 · 整体保存品牌配置；引用其他组织的素材返回 `404` |
| GET | `/api/assets/:id` | 无 | 读取素材 |
| POST | `/api/platform/orgs/:id/events/:eventId/branding/clear` | platform | 下架：清空该活动全部品牌配置 |

`GET /api/assets/:id` 从对象存储流式读出，响应头 `Cache-Control: public, max-age=31536000, immutable`。这是 §2 `no-store` 规则的唯一例外。素材 ID 是随机 uuid，不可枚举；品牌素材本身会在大屏与小程序上公开展示，不做鉴权。

品牌配置随现有接口下发，客户端不单独请求：

| 接口 | 追加字段 |
| --- | --- |
| `GET /api/guest/checkin` | `branding`：显示名、主题色、封面 `url` |
| `GET /api/host/snapshot` | `branding`：显示名、副标题、主题色、Logo 与背景 `url` |
| 离线兜底 JSON（[阶段 9](phase-9-launch.md)） | 显示名与主题色；不含图片 |

### 4.4 对象存储

新增 `internal/objectstore`，只暴露 `Put`、`Get`、`Delete`、`Ping`，内部使用 `github.com/minio/minio-go/v7`。选它而不选 `aws-sdk-go-v2`：只需要基础对象读写，minio-go 依赖更少，并兼容所有 S3 协议存储。

新增 ScopeInfra 配置，落地时写入 [技术方案 §10](../DESIGN.md#10-配置)：

| 键 | 默认 | Secret | 说明 |
| --- | --- | --- | --- |
| `S3_ENDPOINT` | （必填） | | 如 `127.0.0.1:57800` |
| `S3_USE_SSL` | `false` | | |
| `S3_REGION` | `us-east-1` | | |
| `S3_BUCKET` | `golottery` | | |
| `S3_ACCESS_KEY` | （必填） | ✅ | |
| `S3_SECRET_KEY` | （必填） | ✅ | |
| `ASSET_PUBLIC_BASE_URL` | 空 | | 为空时 `url` 为 `/api/assets/<id>`；设置后改为 `<base>/<object_key>`，用于以后接 CDN |

桶由部署步骤创建，API 不自动建桶。本机与测试用 `golottery storage init` 建桶，命令可重复执行。启动时 `Ping` 失败只记录错误日志，不拒绝启动。

素材回收：未被任何活动引用且创建超过 7 天的素材，由 [阶段 9](phase-9-launch.md) 的 `golottery purge` 删除对象与记录。先删记录、再删对象；删对象失败时下次重试。

### 4.5 前端与小程序

- 控制台活动详情新增「品牌」区块：各品牌位上传与预览、显示名、副标题、主题色选择，旁边给出大屏缩略预览
- 大屏按 `branding` 注入主题变量与图片；缺省时使用平台默认外观
- 小程序活动页用 `wx.setNavigationBarTitle` 设置显示名，顶部展示封面
- 运营后台的活动列表提供「下架品牌」按钮，二次确认后调用清空接口

### 4.6 测试

| 对象 | 用例 |
| --- | --- |
| 上传 | PNG、JPEG、WebP 通过；SVG、改扩展名的非图片、超 5 MB、超 4096 px 被拒绝 |
| 隔离 | 引用其他组织的素材返回 `404` |
| 保存 | 品牌位超出各自大小上限被拒绝；主题色格式错误被拒绝 |
| 读取 | 公开读取成功且带 `immutable` 缓存头；不存在的 ID 返回 `404` |
| 下架 | 清空后宾客与大屏接口不再返回品牌；素材记录保留，等待回收 |
| 存储 | 对象存储不可达时签到与抽奖正常，上传返回 `503 E_STORE_UNAVAILABLE` |

`objectstore` 测试连接真实 RustFS（`127.0.0.1:57800`，桶 `golottery-test`），不使用内存替身证明存储路径。

## 5. 明确不做

- 素材库管理界面（列出、删除组织全部素材）
- 按品牌位自动裁剪或压缩
- 图片内容自动审核。首发依靠运营下架；素材只对持有活动码的宾客与现场大屏可见

## 6. 完成定义

- 迁移可重复执行，测试用真实 PostgreSQL 与 RustFS
- 生成物与 `openapi.yaml` 一致
- API、Web、小程序的格式、lint、测试全绿
- 组织管理员能配置一场活动的全部品牌位，大屏与小程序按配置呈现；清空配置后恢复默认外观
