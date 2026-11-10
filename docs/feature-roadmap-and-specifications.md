# ZenStats 高价值新增特性与演进设计方案 (Feature Roadmap & Specifications)

本文档系统梳理了 ZenStats 在完成「点击热力图 (Click Heatmaps)」与「用户行为流向 (User Flows / Sankey DAG)」后，基于 **自托管、隐私优先（Cookieless）、高性能列式存储（Go 1.25 + ClickHouse 25 + PostgreSQL 18）** 架构，最值得添加落地的 **5 大高价值新增特性**。包含完整的**业务价值、技术实现方案、数据模型、API 契约、前端交互设计与实施排期建议**。

> 2026-11-10调研补充：滚动热力图和Web Vitals的具体实现以[调研及实施规划](scroll-depth-and-web-vitals-plan.md)为准，其中修正了SDK体积估计，并明确浏览级分母、可更新报告去重、分位数汇总和SPA导航边界。下文为初始路线图，不代表功能已实现。

---

## 目录
1. [特性一：用户留存热力矩阵 (Cohort & Retention Analysis)](#特性一用户留存热力矩阵-cohort--retention-analysis)
2. [特性二：Web 核心性能指标监控 (Core Web Vitals & Performance)](#特性二web-核心性能指标监控-core-web-vitals--performance)
3. [特性三：页面滚动深度热力图 (Scroll Depth Heatmap)](#特性三页面滚动深度热力图-scroll-depth-heatmap)
4. [特性四：多渠道即时与突发流量告警 (Multi-Channel Webhooks & Spike Alerts)](#特性四多渠道即时与突发流量告警-multi-channel-webhooks--spike-alerts)
5. [特性五：AI 智能数据分析副驾 (ZenStats AI Copilot / Text-to-Analytics)](#特性五ai-智能数据分析副驾-zenstats-ai-copilot--text-to-analytics)
6. [综合实施排期与演进建议](#综合实施排期与演进建议)

---

## 特性一：用户留存热力矩阵 (Cohort & Retention Analysis)

### 1. 核心价值与业务痛点
- **痛点**：当前系统能清晰展示过去 30 天的访问量走势，但无法回答企业运营与产品经理的核心问题：**“上周获取的 1,000 个新访客，在后续第 1 周、第 2 周、第 3 周还有多少人会主动回访？”**。
- **价值**：
  - **产品粘性评估**：衡量产品功能迭代是否真正提升了用户回访频次；
  - **获客质量判定**：对比不同获客渠道（SEO 自然搜索 vs 社交媒体推广 vs 付费投放）的长期留存质量，避免把推广预算浪费在“次日即走”的长尾渠道上。

### 2. 底层架构与 ClickHouse 纯 SQL 聚合方案
> **最大优势**：**无需任何数据库表迁移（Zero Migration），无需客户端 Tracker SDK 做任何改动**，现有的 `events` 表（已包含加盐哈希后的访客指纹/会话标识）完全支持留存计算。

#### 计算逻辑：
1. **定义基准期 (Cohort Period)**：按**周 (Weekly)** 或 **月 (Monthly)** 对访客的首次访问时间进行分桶；
2. **计算回访周期**：统计同一访客标识在后续 `W+1`, `W+2`, `W+3`, `W+4`... 的访问事件；
3. **聚合 SQL 示例**（周留存计算）：
```sql
WITH
    -- 步骤 1：计算站点在时间范围内每个独立访客的最早访问周（Cohort）
    first_visits AS (
        SELECT
            user_id,
            toStartOfWeek(timestamp) AS cohort_week
        FROM zenstats.events
        WHERE site_id = {site_id:UInt64}
          AND timestamp >= {start_time:DateTime}
          AND timestamp <= {end_time:DateTime}
        GROUP BY user_id
    ),
    -- 步骤 2：统计每个访客在后续各周的回访情况
    activity AS (
        SELECT
            e.user_id,
            f.cohort_week,
            intDiv(datediff('week', f.cohort_week, toStartOfWeek(e.timestamp)), 1) AS period_offset
        FROM zenstats.events AS e
        INNER JOIN first_visits AS f ON e.user_id = f.user_id
        WHERE e.site_id = {site_id:UInt64}
          AND e.timestamp >= f.cohort_week
          AND e.timestamp <= {end_time:DateTime}
        GROUP BY e.user_id, f.cohort_week, period_offset
    )
-- 步骤 3：按 cohort_week 与回访步长分组聚合访客人数
SELECT
    cohort_week,
    period_offset,
    count(DISTINCT user_id) AS active_users
FROM activity
GROUP BY cohort_week, period_offset
ORDER BY cohort_week ASC, period_offset ASC;
```

### 3. API 接口契约
- **请求方式**：`GET /api/stats/:domain/retention`
- **查询参数**：
  - `granularity`: `weekly`（默认） | `monthly`
  - `period`: `p30` | `p90` | `custom`
  - `filters`: 选填，支持按国家、渠道进行留存切片（如只看 `visit:channel=organic` 的留存率）
- **响应格式**：
```json
{
  "code": 0,
  "data": {
    "granularity": "weekly",
    "cohorts": [
      {
        "cohort_date": "2026-10-06",
        "total_users": 1250,
        "retention": [
          { "period": 0, "users": 1250, "rate": 100.0 },
          { "period": 1, "users": 480,  "rate": 38.4 },
          { "period": 2, "users": 350,  "rate": 28.0 },
          { "period": 3, "users": 290,  "rate": 23.2 },
          { "period": 4, "users": 240,  "rate": 19.2 }
        ]
      },
      {
        "cohort_date": "2026-10-13",
        "total_users": 1420,
        "retention": [
          { "period": 0, "users": 1420, "rate": 100.0 },
          { "period": 1, "users": 610,  "rate": 42.9 },
          { "period": 2, "users": 440,  "rate": 30.9 },
          { "period": 3, "users": 380,  "rate": 26.7 }
        ]
      }
    ]
  }
}
```

### 4. 前端交互设计
- **位置**：Stats 页面 Section Tabs 新增「留存 (Retention)」标签页；
- **展示形态**：
  - **留存率热力矩阵表 (Cohort Table)**：左侧为群组日期与首期访客总数，右侧为阶梯状单元格，单元格背景色采用从浅绿到深翡翠绿的渐变色阶（颜色深度对应留存百分比），视觉直观；
  - **鼠标 Hover 浮层**：显示该群组在第 N 周的准确回访人数、流失人数与流失率；
  - **顶部切片器**：一键切换「按周留存」与「按月留存」，以及「渠道对比下拉框」。

---

## 特性二：Web 核心性能指标监控 (Core Web Vitals & Performance)

### 1. 核心价值与业务痛点
- **痛点**：网站分析不仅要统计流量来源，站长与前端工程师迫切需要知道**页面加载与交互性能如何**。页面卡顿、白屏时间过长是导致跳出率奇高、SEO 降权的核心原因。
- **价值**：
  - **SEO 评分守护**：Google 搜索引擎已将 Core Web Vitals 纳入核心排名因子；
  - **跳出率精准归因**：直接定位“跳出率高达 65% 的页面，其 LCP 是否超过了 4.2 秒”。

### 2. 核心指标与 Google 标准阈值
| 指标名称 | 含义 | 优秀 (Good) | 需改进 (Needs Improvement) | 较差 (Poor) |
| :--- | :--- | :--- | :--- | :--- |
| **LCP** (Largest Contentful Paint) | 最大内容绘制时间（核心主视觉白屏时间） | ≤ 2.5 秒 | 2.5 秒 ~ 4.0 秒 | > 4.0 秒 |
| **INP** (Interaction to Next Paint) | 交互响应延迟（点击/输入后的渲染跟手度） | ≤ 200 毫秒 | 200 ~ 500 毫秒 | > 500 毫秒 |
| **CLS** (Cumulative Layout Shift) | 累计布局偏移（元素忽上忽下跳动程度） | ≤ 0.1 | 0.1 ~ 0.25 | > 0.25 |
| **TTFB** (Time to First Byte) | 首字节到达时间（服务器与网络响应） | ≤ 800 毫秒 | 800 ~ 1800 毫秒 | > 1800 毫秒 |
| **FCP** (First Contentful Paint) | 首次内容绘制时间（首次渲染任何文本或图） | ≤ 1.8 秒 | 1.8 ~ 3.0 秒 | > 3.0 秒 |

### 3. 数据采集与 Tracker 接入设计
- **采集端**：基于浏览器原生标准 `PerformanceObserver` API（零第三方外挂依赖，压缩后仅增加 ~0.8 KB 体积）；
- **上报时机**：
  - 页面首屏渲染完成后或页面进入后台（`visibilitychange: hidden`）时，通过 `navigator.sendBeacon` 或异步 `POST` 批量上报；
- **上报结构**：
```json
{
  "site_id": "demo.site",
  "pathname": "/pricing",
  "metrics": {
    "lcp": 1820,
    "inp": 140,
    "cls": 0.04,
    "ttfb": 420,
    "fcp": 980
  }
}
```

### 4. 存储与接口设计
- ClickHouse 新增 `web_vitals` 表（或在 `events` 表的 custom props 中紧凑存储）：
  - 包含 `site_id`, `timestamp`, `pathname`, `device`, `country`, `lcp`, `inp`, `cls`, `ttfb`, `fcp`；
- API：`GET /api/stats/:domain/vitals?period=p30`
  - 返回全站及单页维度的 P75（75分位数）核心指标评分及健康度比例。

### 5. 前端展示面板
- **全站健康度总览卡片**：LCP、INP、CLS 的大号 P75 统计与红黄绿达标环形进度条；
- **慢速页面排行表 (Slowest Pages Table)**：按 LCP 耗时降序排列，清晰标明每个页面的首字节延迟与跳出率对照；
- **网络与设备切片**：直观展示移动端 vs 桌面端的性能断层差距。

---

## 特性三：页面滚动深度热力图 (Scroll Depth Heatmap)

### 1. 核心价值与业务痛点
- **痛点**：现有的热力图是「元素点击热力图」，能够看清用户点了哪里。但对于长图文、产品介绍页与定价页，很多访客**根本没有滚动到第二屏或页面底部**。点击率低可能是因为“大部分人压根没看到这个按钮”。
- **价值**：
  - 精确标记页面的 **黄金折叠线（The Fold Line）**；
  - 统计访客在垂直方向上的流失阶梯（例如：100% 访问首屏，但 50% 深度处流失 40%，仅 18% 用户触及底部购买 CTA 按钮）。

### 2. 客户端轻量采样方案
- **节流监听**：Tracker 内部使用高频滚动节流（Throttle 200ms），在访客停留或离站时上报 `max_scroll_percent`（例如 25%、50%、75%、100% 水位）；
- **视口自适应**：按照设备断点（Desktop / Tablet / Mobile）分别计算相对高度百分比，避免因不同分辨率屏幕产生混淆。

### 3. 可视化集成方案
- **无缝复用现有热力图工作台**（[`heatmap.tsx`](file:///Volumes/Workspace/Developer/zenstats/zenstats-web/src/pages/sites/heatmap/heatmap.tsx)）：
  - 在热力图顶部工具栏增加模式切换：`[点击热力图 (Clicks)]` / `[滚动深度 (Scroll Depth)]`；
  - 在已有的页面截图底图之上，叠加纵向平滑彩虹渐变色带（从顶部的鲜绿色逐渐过渡到红色的流失区域）；
  - 悬浮参考线：鼠标在截图上下滑动时，实时显示光标位置对应的滚动深度百分比以及**“触达该深度的访客比例（如 78.4%）”**。

---

## 特性四：多渠道即时与突发流量告警 (Multi-Channel Webhooks & Spike Alerts)

### 1. 核心价值与业务痛点
- **痛点**：目前 ZenStats 的流量告警与通知主要依赖 SMTP 邮件。很多自建技术团队与企业重度依赖即时通讯（IM）工作群，且常常遇到两类极端情况：
  1. **流量异常爆发 (Spike)**：被 HackerNews / Twitter / 知名博主转载带火，需第一时间知晓并准备扩容；
  2. **流量断崖跌零 (Drop-off)**：DNS 故障、证书过期、CDN 拦截或站点崩溃，需紧急告警。

### 2. 渠道支持体系
在现有的 `traffic_alerts` 表与通知逻辑中，扩充渠道类型（`channel_type`）：
1. **通用 Webhook (Generic JSON Webhook)**：
   - 向用户配置的外部 URL 发送 `POST`，携带事件类型、站点域名、触发时间、当前指标与基线偏差；
2. **Telegram Bot**：
   - 配置 `bot_token` 与 `chat_id`，发送富文本 Markdown 告警消息；
3. **飞书 (Lark) 自定义机器人**：
   - 适配飞书标准 Webhook 协议与签名验证机制，生成高颜值的飞书消息卡片（Interactive Card）；
4. **钉钉 (DingTalk) 机器人**：
   - 适配钉钉 Webhook 与加签安全设置，发送 ActionCard；
5. **Slack / Discord**：
   - 适配 Incoming Webhook 标准格式，展示带颜色状态条的 Embed 消息。

### 3. 智能突发与归零检测算法 (3-Sigma 动态基线)
- 不再仅依赖僵硬的手工固定数值；
- 后端 Cron 任务每 5 分钟计算该站点过去 7 天同一时间窗口的平均流量 $\mu$ 与标准差 $\sigma$：
  - **尖峰预警**：当当前 5 分钟流量 $> \mu + 3\sigma$（超过 3 倍标准差）且实际增幅超预期时，触发「流量暴增告警」；
  - **归零预警**：当正常时段流量突然连续 15 分钟为 0，触发「网站可用性疑似中断告警」。

---

## 特性五：AI 智能数据分析副驾 (ZenStats AI Copilot / Text-to-Analytics)

### 1. 核心价值与业务痛点
- **痛点**：对于非专业数据分析师或管理层，手动在前端配置过滤条件、多维度交叉分组与排序门槛较高。
- **价值**：
  - 用户只需在控制台对话框输入一句话：*“帮我看看上周来自 Twitter 的移动端访客，转化率最高的着陆页是哪几个？”*；
  - 系统自动解析生成图表并给出数据解读总结，极大提升产品现代化科技感。

### 2. 架构设计与安全沙箱
```
[用户自然语言提问]
       ↓
[ZenStats 后端 Copilot Agent]
       ↓ (携带 Schema 元数据与 V2 Query 协议 Prompt)
[LLM 引擎 (支持 OpenAI / DeepSeek / 本地 Ollama)]
       ↓ (输出受控 JSON Query 参数，杜绝 SQL 注入)
[调用 internal/service/stats 的 QueryV2 引擎]
       ↓ (获取 ClickHouse 精确查询结果)
[LLM 生成结构化见解 (Summary Insights) + 前端自适应渲染图表]
```

#### 优势：
- **安全可控**：LLM 绝不直接操作原始 SQL 数据库，而是受控生成现有的 `QueryV2Request` JSON 参数，继承所有的租户隔离与权限校验，零安全风险。

---

## 综合实施排期与演进建议

根据“实现成本、架构依赖与用户感知价值”，建议采用**三期敏捷落地**：

### 阶段一：高价值速赢（建议首选立即实施）
- **目标**：**用户留存热力矩阵 (Cohort & Retention)**
- **原因**：后端无需数据库 Schema 变更，无需 SDK 变更，纯基于 ClickHouse 现有事件聚合；前端矩阵表格视觉冲击力强，直接补齐 SaaS 核心指标。
- **预估周期**：1 ~ 2 个迭代。

### 阶段二：体验闭环与深度洞察
- **目标**：**Core Web Vitals 性能监控** + **页面滚动深度热力图**
- **原因**：将热力图（点击 + 滚动）完善为完整闭环，同时引入 Web Vitals 性能监控，打通“流量统计 + 用户行为 + 页面加载性能”全链路。
- **预估周期**：2 ~ 3 个迭代。

### 阶段三：自动化与智能化飞跃
- **目标**：**多渠道告警 (飞书/钉钉/Telegram/Webhook)** + **AI 数据副驾 (Copilot)**
- **原因**：打造自托管生态的极客利器与智能化交互体验，全面超越竞品。
- **预估周期**：2 ~ 3 个迭代。
