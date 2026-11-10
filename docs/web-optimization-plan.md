# ZenStats Web 前端与分析能力优化实施指南

本文档对标业界顶尖开源与商业产品（**Plausible Analytics**、**Umami**、**PostHog**），梳理出 ZenStats Web 系统的深度优化方案，供后续使用 AI 智能体（如 Codex）或工程团队逐项实施、自测与验收。

---

## 目录
1. [实施原则与代码规范](#实施原则与代码规范)
2. [分期任务推进矩阵](#分期任务推进矩阵)
3. [任务一：Tracker 即时连通性调试器（Live DebugView）](#任务一tracker-即时连通性调试器live-debugview)
4. [任务二：全局 Command Palette（Cmd+K 快捷命令面板）](#任务二全局-command-palettecmdk-快捷命令面板)
5. [任务三：实时动态访客地图与活动流（Live Visitors Map & Stream）](#任务三实时动态访客地图与活动流live-visitors-map--stream)
6. [任务四：Core Web Vitals 网站体验与性能看板](#任务四core-web-vitals-网站体验与性能看板)
7. [任务五：页面跳转流向与流失分析（User Flows / Sankey）](#任务五页面跳转流向与流失分析user-flows--sankey)
8. [任务六：用户留存队列分析（Retention Cohorts）](#任务六用户留存队列分析retention-cohorts)
9. [Codex 实施执行提示词与 Checklist](#codex-实施执行提示词与-checklist)

---

## 实施原则与代码规范

1. **分层清晰与最小改动**：
   - 前端遵循 `Page -> Component -> API Client -> Store` 模式；
   - 严格维护 TypeScript 类型，禁止新增 `any`；修改后必须确保 `npm run build` (`tsc -b && vite build`) 0 报错。
2. **样式风格全局统一**：
   - 统一遵循 Shadcn UI + Tailwind CSS 设计规范；
   - 容器底色 `bg-gray-50 dark:bg-gray-950`，卡片 `bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-800 rounded-lg shadow-sm`；
   - 主色调以 Indigo/Violet 为强调色，状态色使用 Emerald (成功)、Amber (告警)、Red (危险)。
3. **国际化完整支持**：
   - 新增的所有文案均需同步在 `src/i18n/locales/zh-CN.json` 和 `src/i18n/locales/en.json` 中配置。
4. **测试兼容性保护**：
   - 保留各组件的关键测试选择器（如热力图的 `aria-label` 与指定测试标签），实施后运行回归测试。

---

## 分期任务推进矩阵

| 序号 | 任务名称 | 影响模块 | 前端重点 | 后端/CH 依赖 | 预估工期 |
| :---: | :--- | :--- | :--- | :--- | :---: |
| **P1-1** | **Tracker 实时调试器 (DebugView)** | `sites/install.tsx`, `sites/stats` | 轮询/SSE 动态检测提示卡片 | 已有 `/stats/:domain/current-visitors` | 1 天 |
| **P1-2** | **全局 Command Palette (Cmd+K)** | `components/command-palette.tsx` | 快捷键监听、站点与时间跳转 | 无（纯前端） | 1 天 |
| **P2-1** | **实时访客地图与动态流 (Live Map)** | `sites/stats/components/live-map` | D3/TopoJSON 轻量点阵世界地图 | 需输出国家经纬度或 ISO 代码聚合 | 2~3 天 |
| **P2-2** | **网站性能看板 (Web Vitals)** | `tracker.js`, `sites/stats` | CWV 绿/黄/红评分柱状分布图 | events 表扩展或专用性能指标字段 | 3~4 天 |
| **P2-3** | **页面跳转流向 (User Flows)** | `sites/stats/components/user-flows` | 桑基图 (Sankey) 或树状跳转路径 | ClickHouse `neighbor()` 路径聚合 | 3~5 天 |
| **P3-1** | **用户留存分析 (Retention Cohorts)**| `sites/stats/components/retention` | 留存热力表格矩阵、衰减折线 | ClickHouse `retention()` 函数 | 3~4 天 |

---

## 任务一：Tracker 即时连通性调试器（Live DebugView）

### 1.1 业务背景
站长在复制并在网站上安装 Tracker 脚本后，最渴望立即确认“脚本是否生效”。目前需要站长自行打开浏览器 Network 或在统计页面等待数据刷新，体验有断层。

### 1.2 实施方案
在 `/sites/:domain/install` 页面下方，增加一个带有脉冲波动的「**实时连通性验证器 (Live Connection Verifier)**」：
1. 页面打开后，自动以 2 秒间隔轻量请求 `/stats/:domain/current-visitors` 或专用 ping 端点；
2. 提示用户：“在另一个标签页打开您的网站”；
3. 一旦探测到有活跃心跳或新事件入库：
   - 状态由“等待首条数据”变为绿色的“🎉 成功连接！检测到来自您的站点的事件”；
   - 动态打出该事件的基本信息：受访 URL、访客所在城市、设备与浏览器；
   - 提供「进入数据看板」直达按钮。

### 1.3 核心文件
- 修改：`src/pages/sites/install.tsx`
- 新增：`src/pages/sites/install/components/live-ping-card.tsx`
- 国际化：`install.pingWaiting`, `install.pingSuccess`, `install.openWebsiteHint`

---

## 任务二：全局 Command Palette（Cmd+K 快捷命令面板）

### 1.1 业务背景
当用户管理多个站点、或在分析页面频繁切换时间周期（今天/昨日/近30天）、跳转到热力图/漏斗/设置时，鼠标点击路径长。成熟的现代 SaaS（如 Raycast、Linear、PostHog）标配 `Cmd+K` / `Ctrl+K` 全局快捷操作。

### 1.2 实施方案
利用 `@/components/ui/command`（基于 `cmdk`）：
1. 监听全局快捷键 `Meta+K` / `Ctrl+K` 打开弹窗；
2. 模块分类：
   - **Sites 站点跳转**：模糊搜索站名/域名，Enter 快速切换；
   - **Date Range 时间切换**：输入 `today`、`7d`、`30d`、`yesterday` 一键更新当前看板日期；
   - **Navigation 导航跳转**：直达「点击热力图」、「转化目标」、「漏斗分析」、「站点设置」、「安装代码」；
   - **Theme 主题切换**：切换浅色 / 深色 / 跟随系统模式。

### 1.3 核心文件
- 新增：`src/components/command-palette.tsx`
- 修改：`src/App.tsx`（在根布局注册命令面板）
- 国际化：`command.searchPlaceholder`, `command.sites`, `command.actions`, `command.periods`

---

## 任务三：实时动态访客地图与活动流（Live Visitors Map & Stream）

### 1.1 业务背景
对标 Umami 的明星功能：全屏/大卡片**点阵世界地图**与**实时活动流**。相比冷冰冰的数字列表，跳动的实时地图在大屏展示、日常监控时极具吸引力。

### 1.2 实施方案
1. **数据端**：
   - 端点：`/stats/:domain/current-visitors` 增加返回字段：每个活跃 Session 的 `{ country_code, city, latitude, longitude, current_page, referrer }`。
2. **前端轻量地图组件**：
   - 依赖：采用无需庞大地图包的 `d3-geo` + 世界简易 TopoJSON（压缩后 < 20KB）；
   - 视觉：灰色点阵陆地轮廓，活跃访客位置使用带涟漪发光动画的圆点（`<circle className="animate-ping" />`）；
   - 交互：鼠标 Hover 访客圆点展示 Tooltip：“美国 纽约 · 访问了 /pricing · 来自 Google”；
   - 布局：嵌入在 Stats 面板的「实时访客」扩展卡片或独立大屏弹窗。

### 1.3 核心文件
- 新增：`src/pages/sites/stats/components/live-map/world-map.tsx`
- 新增：`src/pages/sites/stats/components/live-map/activity-feed.tsx`
- 修改：`src/pages/sites/stats/components/current-visitors.tsx`

---

## 任务四：Core Web Vitals 网站体验与性能看板

### 1.1 业务背景
Google SEO 与用户体验对页面性能高度敏感。成熟站长急需了解真实用户（RUM）在不同地区、设备下的加载与交互性能。

### 1.2 核心指标定义
- **LCP (Largest Contentful Paint)**：最大内容绘制（目标 ≤ 2.5s）
- **INP (Interaction to Next Paint)**：下次绘制交互延迟（目标 ≤ 200ms）
- **CLS (Cumulative Layout Shift)**：累积布局偏移（目标 ≤ 0.1）
- **TTFB (Time to First Byte)**：首字节到达时间（目标 ≤ 800ms）

### 1.3 实施方案
1. **Tracker SDK 自动采集**：
   - 脚本通过标准 `PerformanceObserver` 捕获指标；
   - 在页面离开前通过 `navigator.sendBeacon` 发送给 `/api/event`，事件名为 `zs:perf`；
   - Props 包含：`{ metric: 'LCP', value: 1820, rating: 'good' }`。
2. **Web 呈现看板**：
   - 在 Stats 的 Section Tabs 中新增「网页性能 (Performance)」面板；
   - 顶栏汇总：三色比例条（绿: 良好 / 黄: 需要改进 / 红: 较差）及 P75 分位数数值；
   - 下方细分表：各受访页面的性能均值与合格率排名。

### 1.4 核心文件
- 修改：`tracker/src/zenstats.js`
- 新增：`src/pages/sites/stats/components/performance-panel.tsx`
- 新增：`src/pages/sites/stats/components/vitals-gauge.tsx`

---

## 任务五：页面跳转流向与流失分析（User Flows / Sankey）

### 1.1 业务背景
漏斗只能查看单向预设转化，用户旅程流向（User Journey）可以直观回答：“用户从落地页进来之后，绝大多数去了哪个页面？从哪里跳出？”。

### 1.2 实施方案
1. **数据查询（ClickHouse）**：
   - 使用窗口函数 `neighbor(pathname, 1) OVER (PARTITION BY session_id ORDER BY timestamp)` 聚合 `source_page -> target_page` 的跳转频次；
   - 标记跳转为 `(exit)` 的跳出事件。
2. **前端可视化**：
   - 使用 Recharts 的 `<Sankey />` 或基于 Canvas 的桑基图；
   - 节点表示页面，连线宽度表示跳转流量大小；
   - 支持点击节点进行聚焦过滤（以该节点为核心查看后续 3 步跳转）。

### 1.3 核心文件
- 新增：`src/pages/sites/stats/components/user-flows-panel.tsx`
- 新增：`src/pages/sites/stats/components/sankey-chart.tsx`

---

## 任务六：用户留存队列分析（Retention Cohorts）

### 1.1 业务背景
衡量网站或产品黏性的终极指标。统计某周/某月首次访问的用户，在后续第 1~8 周的复访比率。

### 1.2 实施方案
1. **数据查询**：
   - 后端利用 ClickHouse `retention()` 函数，基于已有的 `user_id`（持久化 Salt 散列）计算留存组；
2. **前端矩阵表格**：
   - 纵轴：初次访问周期（Cohort，例如“2026 第 40 周”）；
   - 横轴：+1 周、+2 周、+3 周...
   - 单元格：百分比数字 + 渐变蓝色背景填充（百分比越高颜色越深）；
   - 点击某一周支持查看该留存群组的画像（国家、来源渠道分布）。

### 1.3 核心文件
- 新增：`src/pages/sites/stats/components/retention-panel.tsx`
- 新增：`src/pages/sites/stats/components/cohort-heatmap-table.tsx`

---

## Codex 实施执行提示词与 Checklist

在调用 Codex 实施某一阶段时，可直接复用以下结构化 Prompt：

```markdown
请按照 docs/web-optimization-plan.md 中的规范，实施 [任务编号: 如 P1-1 Tracker 即时连通性调试器]。

要求：
1. 严格遵守 docs/web-optimization-plan.md 的设计方案与文件规范；
2. 保持全站 Tailwind 样式与暗黑模式适配；
3. 文案统一在 src/i18n/locales/zh-CN.json 与 en.json 中双语配置；
4. 实施完成后执行 npm run build 确保 0 报错并通过类型检查。
```

### 实施自测验收 Checklist
- [ ] 页面在明亮/暗黑模式下无样式异常或刺眼色差。
- [ ] 路由切换、URL Query 同步工作正常，刷新页面状态不丢失。
- [ ] 国际化文案完整，无缺失的硬编码中文或英文。
- [ ] 移动端/响应式窄屏下控件自动折行，不破坏排版与横向滚动条。
- [ ] 执行 `npm run build`，TypeScript 与 Vite 编译打包正常通过。
