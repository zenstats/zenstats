// Package docs contains the root Swagger annotations for the Zenstats HTTP API.
//
// This file is the swagger entrypoint used by `make swagger` and may be safely
// kept under version control. Generated artifacts are docs.go, swagger.json and
// swagger.yaml.
package docs

// @title           Zenstats API
// @version         1.0
// @description     Zenstats — 自托管网站分析服务，提供隐私友好的 Web 分析 API。
// @description
// @description     ## 认证方式
// @description     - **BearerAuth**: 在 Authorization 头中传入 `Bearer <JWT Token>`，用于管理类 API
// @description     - **APIKeyAuth**: 在 Authorization 头中传入 API Key，用于统计查询 API
// @description
// @description     ## 统一响应格式
// @description     成功: `{ "code": 200, "message": "success", "data": ... }`
// @description     失败: `{ "code": <status>, "message": "...", "error": "..." }`
// @description
// @description     ## API 分组
// @description     - **认证** — 登录、注册、令牌刷新、密码重置、邮箱验证、系统初始化
// @description     - **站点管理** — 站点 CRUD、域名验证、屏蔽规则（IP/域名/国家）
// @description     - **统计分析** — 聚合指标、时间序列、维度细分、实时访客、CSV 导出
// @description     - **漏斗分析** — 漏斗 CRUD + 转化率分析
// @description     - **目标管理** — 转化目标 CRUD
// @description     - **事件采集** — 前端埋点 SDK 上报端点
// @description     - **数据导入** — GA4 历史数据导入与查询
// @description     - **用户** — 自定义搜索引擎、子账号管理、配额查询、个人资料
// @description     - **API Key** — API Key 创建、列表、删除
// @description     - **管理员** — 用户/站点/套餐管理、系统配置、系统统计
// @description     - **健康检查** — 服务及依赖连通性检查
// @termsOfService  https://github.com/zenstats/zenstats

// @contact.name   Zenstats
// @contact.url    https://github.com/zenstats/zenstats
// @contact.email  wrpota@gmail.com

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /api

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description 在 Authorization 头中传入 `Bearer <JWT Token>`。获取方式：调用 /auth/login 或 /auth/register。

// @securityDefinitions.apikey APIKeyAuth
// @in header
// @name Authorization
// @description 在 Authorization 头中传入 API Key（无需 Bearer 前缀）。在站点管理页面创建 API Key。

// @externalDocs.description  OpenAPI 规范
// @externalDocs.url          https://swagger.io/resources/open-api/
