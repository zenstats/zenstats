# Zenstats 优化路线图

基于 [Plausible vs Zenstats 功能对标](./plausible-comparison.md)，按优先级列出剩余优化项及实施计划。

> ✅ 已完成的差异修复（本批次）见 [plausible-comparison.md#六](./plausible-comparison.md#六本次修复批次总结)。

---

## 一、当前状态总览

| 模块 | 对齐度 | 说明 |
|------|--------|------|
| 事件采集管道 | **100%** | 22 个步骤全部对齐（含 SPA Hash、Props 限制、长度限制等本次修复项） |
| 核心数据模型 | **100%** | Salt 持久化、Bounce 判断、根域名提取、Engagement 入库等全部对齐 |
| 统计查询 API | **~90%** | aggregate / breakdown / timeseries / main-graph / current-visitors / suggestions / CSV 导出均已实现 |
| 管理功能 | **~80%** | 多站点/团队/API Keys/漏斗/目标/导入 已对齐；共享链接/邮件报告 待补充 |
| 安全/反滥用 | **~85%** | Bot/IP/Geo/Shields 对齐；Spam Referrer 过滤待补充 |

---

## 二、优化项目（按优先级）

### 🟠 P1 — Spam Referrer 过滤

**对标**: Plausible `ReferrerBlocklist` 模块，按 hostname 匹配已知垃圾 referrer。

**当前状态**: 未实现。所有 referrer 来源计入渠道统计，垃圾 referrer 污染数据。

**实施方案**:

```
文件: internal/event/spam_referrer.go （新建）
功能: 加载 Plausible referrer-spam-list JSON → 内存 Set
调用: processEvent() 中 PutSourceInfo() 之后，dropSpamReferrer()

文件: internal/event/event.go
改动: processEvent() 中新增 1 行调用
```

**代码量**: ~30 行（加载 + 匹配函数）+ 1 行调用

**数据源**: [github.com/plausible/referrer-spam-list](https://github.com/plausible/referrer-spam-list) 的 `spammers.json`，编译时 `//go:embed` 内嵌。

**验证**:
```bash
# 模拟垃圾 referrer
curl -X POST http://localhost:8080/api/event \
  -H "Content-Type: application/json" \
  -d '{"n":"pageview","u":"https://mysite.com/","d":"mysite.com","r":"https://known-spammer.com/"}'
# 预期: HTTP 202, 事件被丢弃, 日志: "dropping spam referrer"
```

---

### 🟡 P2 — 邮件报告

**对标**: Plausible Weekly/Monthly Email Reports。

**当前状态**: Email 服务（`internal/service/email.go`）和 Cron 调度器（`pkg/scheduler/`）已就绪，缺少报告内容生成。

**实施方案**:

```
新建: internal/service/email_report.go    — 报告生成（每周聚合、月度摘要）
新建: internal/api/admin/email_report.go  — 管理端订阅/取消订阅接口
改动: internal/bootstrap/cron.go          — 注册周报/月报定时任务
```

**Cron 表达式**:
- 周报: `0 8 * * 1`（每周一 8:00）
- 月报: `0 9 1 * *`（每月 1 日 9:00）

**报告内容**: 总 PV/UV、Top 5 页面、Top 5 来源、与上周/上月环比变化率。

---

### 🟡 P2 — 共享链接

**对标**: Plausible Shared Links（带密码保护的公开分享链接）。

**当前状态**: 未实现。

**实施方案**:

```
新建: internal/store/postgresql/ent/schema/shared_link.go  — Schema
      （字段: slug, password_hash, site_id, expires_at, created_by）
生成: make ent-generate                                      — 生成 CRUD
新建: internal/api/sharedlink/create.go                     — 创建分享链接
新建: internal/api/sharedlink/public.go                     — 公开访问（无需登录）
新建: internal/service/sharedlink.go                        — 业务逻辑
改动: internal/api/router/router.go                         — 注册路由
```

**路由设计**:
```
POST   /api/sites/:domain/shared-links       → 创建分享链接
GET    /api/sites/:domain/shared-links       → 列出分享链接
DELETE /api/sites/:domain/shared-links/:id   → 删除分享链接

GET   /share/:slug                           → 公开统计页（JWT-free）
POST  /share/:slug/authenticate              → 密码验证，返回临时 token
GET   /share/:slug/data                      → 公开统计数据（需临时 token）
```

---

### 🟢 P3 — 流量异常检测

**对标**: Plausible `traffic_change_notifier`。

**当前状态**: 未实现。

**实施方案**:

```
新建: internal/service/anomaly.go  — 对比当前时段与上一同期的 PV/UV 变化
改动: internal/bootstrap/cron.go   — 注册检测定时任务（每小时）
```

**检测逻辑**: 当前小时 PV 与上周同小时 PV 对比，变化超过 50% 且绝对值 > 阈值时触发通知（Email 或 Webhook）。

---

## 三、分阶段路线图

```
阶段 1（1 天）   ───  P1 Spam Referrer 过滤
                       ≈ 30 行代码 + 1 行调用，零风险
                       完成即达事件采集 100% 对齐

阶段 2（3-5 天） ───  P2 邮件报告 + 共享链接
                       完整功能模块，中等复杂度

阶段 3（2-3 天） ───  P3 流量异常检测
                       数据对比 + 通知逻辑

阶段 4（按需）   ───  Google Analytics 增强导入 / 数据迁移
                       低频需求，按用户反馈决定
```

---

## 四、验证方法

每个阶段完成后执行：

```bash
cd /Volumes/Workspace/Developer/zenstats/zenstats

# 编译验证
go build ./...

# 相关模块测试
go test ./internal/event/... ./internal/service/... ./internal/api/...

# 全量集成测试
make test-integration
```

---

## 五、不纳入优化（设计取舍）

| 项目 | 原因 |
|------|------|
| **双 salt 轮换** | 单 salt + 持久化已足够；轮换增加复杂度，无实质收益 |
| **手动数据清洗/重算** | 数据一致性由 sign 版本化保障，无需额外工具 |
| **跨服务器数据迁移** | 低频率需求，暂不投入；后续按用户反馈决定 |
