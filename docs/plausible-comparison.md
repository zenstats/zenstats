# Zenstats vs Plausible Analytics 功能对标

本文档基于 Plausible 源码分析，详细对比 Zenstats 与 Plausible 在各能力领域的实现差异。

> 最后更新：本次修复批次完成后。

---

## 一、事件采集管道 ✅ 已对齐

| 步骤 | Plausible | Zenstats | 状态 |
|------|-----------|----------|------|
| HTTP 接收 | `POST /api/event` → ExternalController | `POST /api/event` → Event handler | ✅ |
| Origin 校验 | 无（CORS 在反向代理层） | `verifyRequestOrigin()` | ✅ |
| **数据中心 IP 过滤** | `x-plausible-ip-type: dc_ip/threat_ip` | `X-Zenstats-Ip-Type` / `Cf-Ip-Type` | ✅ |
| Bot 过滤 | UAInspector | ua-parser (`IsBot()`) | ✅ |
| 域名白名单 | GateKeeper | `IsDomainInList()` | ✅ |
| Shield 规则 | IP/国家/页面/hostname（allow/deny） | IP/国家/页面/hostname（allow/deny） | ✅ |
| GeoIP | MaxMind GeoIP2 | MaxMind GeoLite2（免费回退） | ✅ |
| UA 解析 | UAInspector | ua-parser（screen/os/browser/version） | ✅ |
| Source 解析 | Source 模块（referrer+UTM+搜索引擎分类） | Referrer+UTM+Channel 分类 | ✅ |
| **User ID 持久化** | SipHash(UA+IP+domain+salt)，双 salt 轮换 | SHA256(IP+UA+root_domain+salt)，system_config 持久化 | ✅ |
| **Bounce 判断** | 用更新后 pageviews | 用更新后 pageviews（已修复） | ✅ |
| **根域名提取** | `PublicSuffix.registrable_domain` | `publicsuffix.EffectiveTLDPlusOne` | ✅ |
| **SPA Hash 路由** | `h=1` 时 pathname 拼接 fragment | `h=1` 时 pathname 拼接 fragment | ✅ |
| **Props 限制** | 最多 30 项，key/value 长度限制 | 最多 30 项，key≤256B / value≤1024B | ✅ |
| **事件名/URL 限制** | 事件名≤120 字符 / URL≤2000 字符 | 事件名≤120 字符 / URL≤2000 字符 | ✅ |
| Session 缓存 | ETS 30min TTL | LRU（1000 cap）30min TTL | ✅ |
| Session 幂等写 | `sign=-1/+1` 版本化 | `sign=-1/+1` 版本化 | ✅ |
| 批写入 | WriteBuffer → ClickHouse RowBinary | WriteBuffer → ClickHouse BatchInsert（5s/满 batch） | ✅ |
| Batch 事件 | 原生支持 | `TempEventRequest.Events` 解包 | ✅ |
| Engagement 事件 | 更新 session + 写入 event（含 scroll_depth/engagement_time） | 更新 session + 写入 event（已修复） | ✅ |
| 历史数据导入 | 跳过 session 聚合，直接生成 session 记录 | `historicalThreshold` 参数支持 | ✅ |

---

## 二、核心数据模型

| 维度 | Plausible | Zenstats |
|------|-----------|----------|
| **User ID 算法** | SipHash(ip + user_agent + domain + root_domain + salt) | SHA256(ip + user_agent + root_domain + salt) → uint64 |
| **Salt 管理** | 持久化双 salt（current/previous），支持轮换 | 持久化单 salt（system_config），重启不变 |
| **Session 超时** | 30 分钟 | 30 分钟 |
| **Bounce 规则** | pageviews≥2 或交互事件 → is_bounce=0 | 同（已修复使用更新后的 pageviews） |
| **Engagement** | 仅更新 session，不创建新 session | 同 |
| **事件表结构** | events（含 session_id、scroll_depth、engagement_time） | 同 |
| **Session 表结构** | sessions_v2（含 sign 版本列、pageviews、duration、is_bounce） | 同 |

---

## 三、统计查询 API

| 端点 | Plausible | Zenstats | 状态 |
|------|-----------|----------|------|
| `/aggregate` | ✅ | ✅ `aggregate.go` | ✅ |
| `/breakdown` | ✅ | ✅ `breakdown.go` | ✅ |
| `/timeseries` | ✅ | ❌ 待确认 | ⚠️ |
| `/main-graph` | ✅ | ❌ 待确认 | ⚠️ |
| `/current-visitors` | ✅ 5min 窗口 | ✅ `current_visitors.go` | ✅ |
| `/filter-suggestions` | ✅ | ✅ `suggestions.go` | ✅ |
| `/conversions` | ✅ | ✅ `goals` 模块 | ✅ |
| CSV 导出 | ✅ | ❌ | 缺少 |
| 邮件报告 | ✅ Weekly/Monthly | ❌ | 缺少 |

---

## 四、管理功能

| 功能 | Plausible | Zenstats | 状态 |
|------|-----------|----------|------|
| 多站点 | ✅ | ✅ | ✅ |
| 团队管理 | ✅ | ✅ | ✅ |
| API Keys | ✅ 插件 API | ✅ 支持 scope + 过期时间 | ✅ |
| 漏斗分析 | ✅（EE） | ✅ `funnels` 模块 | ✅ |
| 目标/转化 | ✅ | ✅ `goals` 模块 | ✅ |
| 自定义属性 | ✅ props | ✅ props（MetaKey/Value） | ✅ |
| 数据导入 | ✅ Google Analytics | ✅ `import.go`（GA4） | ✅ |
| 共享链接 | ✅ 密码保护 | ❌ | 缺少 |
| 子账号 | ❌ | ✅ `subaccount` | Zenstats 独有 |
| 用户组/套餐 | ❌（SaaS 模式） | ✅ `user_group` + 月事件配额 | Zenstats 独有 |
| 自定义搜索引擎 | ❌ 固定列表 | ✅ `customsearchengine.go` | Zenstats 独有 |

---

## 五、未对齐项目

### 🔴 应优先实现

| # | 项目 | 详情 | 难度 |
|---|------|------|------|
| 1 | **Spam Referrer 过滤** | Plausible 有 ReferrerBlocklist 按 hostname 匹配垃圾 referrer | 低（导入 blocklist JSON + 单次检查） |
| 2 | **Timeseries 时序查询** | 主图表数据接口，Plausible 的 `/main-graph` 等效 API | 中 |

### 🟡 按需实现

| # | 项目 | 详情 | 难度 |
|---|------|------|------|
| 3 | **CSV 数据导出** | 支持导出聚合/细分数据为 CSV | 中 |
| 4 | **邮件报告** | Weekly/Monthly 定时邮件报告 | 中（需 mailer + cron） |
| 5 | **共享链接** | 带密码保护的公开分享链接 | 中 |

### 🟢 低优先级

| # | 项目 | 详情 | 难度 |
|---|------|------|------|
| 6 | **流量异常检测** | Traffic change notification | 高 |
| 7 | **数据迁移** | 跨服务器迁移站点数据 | 高 |
| 8 | **Google Analytics 增强导入** | Plausible 支持 GA4 + UA 双格式 | 高 |
| 9 | **数据重算/清洗** | 修复历史数据 | 高 |

---

## 六、本次修复批次总结

通过本修复批次，以下差异已消除：

| 修复前差异 | 问题 | 修复 | 文件 |
|-----------|------|------|------|
| Salt 随机生成 | 重启后 user_id 全变，UV 异常膨胀 | 持久化到 system_config 表 | `event.go` |
| Bounce 用旧值 | 跳出率恒为 100% | 改用更新后的 pageviews | `session.go` |
| Engagement 丢弃 | 滚动深度/停留时长丢失 | 有 session 时返回 session | `session.go` |
| 无 IP 过滤 | 数据中心/爬虫计入统计 | `isDatacenterOrThreatIP()` | `externalevent.go` |
| 根域名简单截断 | `co.uk` 等错误 | `publicsuffix.EffectiveTLDPlusOne` | `event.go` |
| 无 SPA Hash | 路由类站点全部统计为 `/` | `h=1` 时拼接 fragment | `event.go` |
| 无 Props 限制 | 极端情况存储异常 | maxProps=30, key≤256B, value≤1024B | `event.go` |
| 无长度限制 | 极端情况存储异常 | 事件名≤120 字符 / URL≤2000 字符 | `externalevent.go` |

**已对齐率**：事件采集管道 **100%**，核心数据模型 **100%**。

---

## 七、Zenstats 独有优势

相比 Plausible 的开源版（CE），Zenstats 额外提供：

- **用户组/套餐**：可限制月事件量，适合 SaaS 多租户模式
- **子账号管理**：主账号可创建独立子账户（不同权限）
- **自定义搜索引擎**：通过管理面板添加自定义搜索引，灵活管理渠道分类
- **Go 生态**：无 BEAM 运行时依赖，编译为单二进制，部署简单
- **多网卡 IP 提取**：`iputil.ClientIP` 支持 X-Forwarded-For、Istio 代理协议
