# 滚动深度热力图与 Web Vitals：调研及实施规划

调研日期：2026-11-10。状态：滚动深度第一阶段与 Core Web Vitals 首版已实现并在本地验证；历史封账汇总与生产规模压测尚未完成。

本文补充 `feature-roadmap-and-specifications.md`，作为这两项功能的实施依据。Web 任务见 [Web 实施规划](../../zenstats-web/docs/scroll-depth-and-web-vitals-plan.md)。

## 已实现范围：滚动深度第一阶段

- 热力图总开关下新增独立 `clicks_enabled`（默认 true）和 `scroll_enabled`（默认 false）。升级需执行 PostgreSQL `migrate`；ClickHouse 启动时幂等创建 `heatmap_scroll_reports`。
- Tracker 沿用 `data-heatmap="true"`，更新自托管 `heatmap.js` 后即可根据新能力字段启用滚动。未开启滚动时不发送滚动报告；页面未点击、未滚动也记录初始可见样本。
- `POST /api/heatmap/scroll` 使用独立有界队列、原子批校验、站点／Origin／屏蔽校验、有限重试和管理员质量计数。**202 表示进入内存队列，不能视为落库成功**；质量计数记录报告条数，去重后的浏览数由查询产生。
- `GET /api/sites/:domain/heatmap/scroll/pages` 与 `/scroll/data` 只允许查询最近 `min(raw_days, 7)` 个自然日，使用冻结站点时区。沿用读权限，跨站成员不可访问；`GET /api/admin/scroll-quality` 仅管理员可读。
- 浏览 ID 只存内存；报告最多12个 revision。查询先按浏览解析最新整条快照，再按维度过滤；不依赖后台合并，也不会把重试或每次增量上报累加为新浏览。
- 页面最大高度增长超过初始高度25%、报告截断、不支持布局的浏览不计入有效分母。标记可能重叠；延迟初始化保留在样本中并单独提示。比例表示实际采样浏览的几何触达，不能外推为独立访客、阅读完成率或元素曝光。
- Web 提供独立滚动页面列表、80段色带、25/50/75/100%触达卡、平均初始首屏线、质量标记与无截图查看，复用聚合标识、设备、版本及截图。
- 点击／滚动复用热力图专用两连接池及一个查询槽；查询继续限制单线程、5秒、128MiB。保留PV、UV、Session及原有 engagement 滚动指标口径。
- 当前不支持滚动的90天历史聚合；`aggregate_days` 仍只控制原有点击聚合。后续任务按下文分阶段推进，不应把以下设计视为全部已交付。

## 已实现范围：Core Web Vitals 首版

- 独立站点配置默认关闭；Tracker 需显式设置 `data-performance="true"`。锁定官方 `web-vitals@6.2.3`，采集 LCP、INP、CLS，并提供 TTFB、FCP 辅助指标；不增加普通 PV、UV、Session 或业务事件计数。
- PostgreSQL `site_performance_configs` 保存采样率、路径白名单、1～7天保留期及配置版本；执行 `migrate` 更新。ClickHouse 创建独立 `web_vitals_reports`，采用 TTL 清理原始报告。
- `GET /api/performance/config` 与 `POST /api/performance/reports` 提供公开能力配置和独立采集；单批最多20条、16KiB，原子校验、独立限流和有界队列。**202 仅表示进入内存队列，不能推断落库成功**。管理员在采集质量页面及 `/api/admin/performance-quality` 查看接收、待写、持久化、重试和丢弃计数。
- `GET/PUT /api/sites/:domain/performance/config` 沿用站点读权限及所有者/管理员写权限，子账号只读；`GET /api/sites/:domain/performance/data` 提供 P75、分级、覆盖率、每日趋势和慢页面排行，支持设备、路径、主机、版本、浏览器、国家及导航类型筛选。
- 按文档导航采集，SPA 路由切换保留初始文档 URL；BFCache 恢复新建生命周期。浏览身份仅保存在内存，不采集 DOM、资源 URL 或持久化用户 ID；不启用 soft navigation、attribution 或 reportAllChanges。
- 查询先按指标实例解析最新整条 revision，再过滤和计算原始样本 P75；不取最大值、不平均每日 P75。缺失 INP 保持空，CLS=0 有效；后台初始化和截断指标排除有效分母，延迟初始化单列提示；三项核心指标任一不足100有效样本时整体显示“数据不足”（产品展示门槛）。
- 仅查询冻结站点时区下最近 `raw_days` 个自然日（最多7天）；混合采样率与配置版本会提示。历史封账、30/90天聚合及生产规模压测尚未交付。
- 与点击/滚动共用专用两连接池及一个查询槽，查询限制单线程、5秒、128MiB；结果缓存5秒、最多100组条件。
- Web 入口：**统计页面 → Core Web Vitals**；**站点设置 → 性能监控设置 → 查看性能监控**。接入步骤见 Web 文档的性能监控章节。

## 1. 调研结论

| 参考实现／规范 | 核实结果 | ZenStats 的选择 |
| --- | --- | --- |
| Plausible 滚动深度 | 提供平均滚动深度、1%～100%深度和滚动目标，支持页面分组；强调按设备、内容类型比较 | 保留既有平均指标；新增按浏览去重的最大深度分布，避免直接把心跳平均值绘制成触达率 |
| Microsoft Clarity Scroll maps | 提供截图叠加、深度触达比例、25%／50%／75%参考线、平均首屏线 | 复用当前热力图工作台，增加纵向色带、参考线及平均首屏线 |
| Google Web Vitals | 核心指标为 LCP／INP／CLS，推荐按移动／桌面分别评估 P75 | 首版提供三项核心指标，TTFB／FCP作为辅助；报告实际样本数与覆盖率 |
| 官方 `web-vitals` | 本次 npm 查询稳定版为 **6.2.3**，发布于2026-10-05；文档给出的标准库体积约3KB Brotli，非原路线图的0.8KB | 锁定核验后的稳定版本，自托管构建产物；实际打包、压缩体积以本项目构建测量为准 |
| 生命周期 | INP可能因没有交互而缺失；指标可能多次回调；BFCache恢复产生新指标实例 | 缺失不补0，按指标实例取最新版本；恢复页面作为新生命周期 |
| SPA | 官方6.2.3文档已描述 Chromium 151+ 的可选 soft navigation 支持，其他浏览器行为不同 | MVP使用默认文档导航口径；软导航单独作为后续实验功能，不能声称所有SPA均支持路由级标准CWV |
| ClickHouse增量物化视图 | 处理新插入的数据块；ReplacingMergeTree后台合并不会撤回已写入的聚合贡献 | 可更新报告先去重，再产生分布／分位数汇总；不能直接对每次重试和更新执行countState／quantileState |

## 2. 当前项目基础与边界

- `zenstats-web/tracker/src/zenstats.js` 已采集 engagement 的最大可视底部和文档高度，滚动监听使用 requestAnimationFrame；活跃时每10秒发送心跳，隐藏／pagehide时补报。
- `internal/service/stats/sql/fragments.go` 当前 `scroll_depth` 为 engagement 事件的平均值，既不是按页面浏览计算的最大深度均值，也不是某深度的触达率。
- 当前常规事件没有独立 `pageview_id`，同一用户、会话和URL可能包含重复浏览／多个标签页，不能可靠恢复逐次浏览。
- 点击热力图已有40×80比例网格、mobile／tablet／desktop分组、路径／标识聚合、截图、日期选择、配置权限及独立有界队列。
- 可复用 `internal/api/external/heatmap.go` 的站点、Origin、Bot、屏蔽规则和限流设计；业务类型、统计计数和故障状态分开。
- 热力图当前单独连接池和并发槽并不能限制整个ClickHouse进程内存。新增功能需要共同的可选分析资源预算，不能为每项功能都另开两个并发查询。
- 保持PV、UV、Session、业务事件数口径。新功能不通过普通自定义事件模拟，不引入跨访问持久化访客标识。
- 两项功能均默认关闭。页面配置与站点脚本接入共同控制启用；设置开关不能使未安装可选SDK的站点自动产生数据。

## 3. 滚动深度热力图

### 3.1 第一版范围

- 在现有热力图工作台增加“点击／滚动”模式；保持设置列表→新建／详情的操作方式。
- 复用路径白名单、聚合标识、hostname、version、variant、设备组、日期范围及截图匹配。
- 输出80个纵向区域的最深可视边界触达率；与点击图80行对应，但没有横向网格，不需要3200个滚动区块。
- 提供25%、50%、75%、100%触达卡片、浏览级平均最大深度和平均首屏线。
- UI使用“采样页面浏览”“到达该深度的浏览比例”。首版不称为独立访客人数、阅读完成率或元素曝光率。
- 常规统计里的旧平均滚动指标先保持原口径；新版标为“浏览级平均最大深度”，不能静默替换历史指标。

### 3.2 统计单位与算法

单位为一次**可见页面浏览**。进入新页面／SPA页面切换／BFCache恢复时，在内存生成随机 `page_view_id`。刷新、重复打开同一URL及不同标签页均为不同浏览。ID不写Cookie或localStorage。

每次有效采样记录：

```text
visible_bottom_px = min(scroll_y + viewport_height, document_height)
max_visible_bottom_px = max(历次有效visible_bottom_px)
max_document_height_px = max(历次有效document_height)
depth = clamp(max_visible_bottom_px / max_document_height_px, 0, 1)
到达深度d的比例 = 最大深度>=d的有效采样浏览数 / 有效采样浏览数
```

- 短页可在未滚动时达到100%，因为整页已处于可视区域。
- 没有点击、没有滚动的采样浏览也要上报初始记录，不能只从有点击的样本构造分母。
- 该指标表示“最深可视边界到达”，跳转锚点或快速滚动不证明中间内容已被阅读；不设置停留时长条件。
- 动态增高时分母随观测到的最大文档高度更新，因此深度可能下降。按最新revision重算，不能取历次百分比最大值，也不能仅保留最高命中档位。
- 记录初始视口高度与初始文档高度，首屏线按有效浏览的初始视口／文档高度比例平均后投影。页面已在深处恢复时仍需说明是几何参考线。
- 无限滚动、显著高度变化、超长页面、未得到有效尺寸分别标记原因。首版将显著动态布局从静态截图投影中排除，展示排除量；动态变化门槛在第0阶段固定并版本化。
- 首版只统计文档根滚动，不统计嵌套滚动容器、横向滚动与缩放后的精确位置。
- 文档比例坐标与截图比例对应，沿用“大致区域”定位；不同长度、内容顺序的聚合页仍可能产生偏移。

### 3.3 采集与传输

- 与点击图共用同次浏览的采样抽签与固定sample_rate，刷新配置不重新抽签。点击、滚动分别有启用开关，滚动不能依赖发生点击。
- 已存在engagement监听可以复用生命周期通知；独立模块维护自己的尺寸／浏览状态，不把新的滚动记录塞进普通事件队列。
- 滚动与尺寸变化只更新本地状态，节流执行，避免每次scroll都进行网络请求或整页DOM扫描。
- 上报初始快照、发生变化后的检查点、隐藏／页面切换／pagehide时的最新快照。建议检查点最小间隔10秒、每浏览最多12个快照，保留尾部预算，预算耗尽记录截断状态。
- 监听文档尺寸变化时合并任务并读取根尺寸，禁止为页面每个节点安装Observer；移动端可视区域变化、图片懒加载和旋转屏幕都纳入测试。
- `report_id`、`page_view_id`、`revision`、采样配置及归属时间在重试时保持不变；同revision内容不得变化。
- 配置或可选模块加载失败、页面过早关闭均可能造成漏采。面板只对已收到的有效采样浏览计算比例，展示延迟初始化、截断等质量信息。

建议报告字段：

```text
protocol_version, domain, report_id, page_view_id, revision,
page_started_at, reported_at, config_version, sample_rate,
hostname, path, aggregation_key, version, variant, device_class,
viewport_width/height, document_height_at_start,
max_visible_bottom_px, max_document_height_px,
initial_visible, late_init, dynamic_layout, clipped, end_reason
```

初始身份和设备组在浏览期间冻结；跨断点resize结束旧样本，下一段作为新样本并标记resize，不把同一浏览同时归入多个设备组。

## 4. Core Web Vitals 性能监控

### 4.1 指标和展示口径

| 指标 | 单位 | Good | Needs improvement | Poor |
| --- | --- | --- | --- | --- |
| LCP | ms | ≤2500 | >2500且≤4000 | >4000 |
| INP | ms | ≤200 | >200且≤500 | >500 |
| CLS | 无量纲 | ≤0.1 | >0.1且≤0.25 | >0.25 |
| TTFB（辅助） | ms | ≤800 | >800且≤1800 | >1800 |
| FCP（辅助） | ms | ≤1800 | >1800且≤3000 | >3000 |

- 总览提供每项P75、三档占比、有效样本数和覆盖率；列表提供慢页面排名和趋势。
- CWV整体通过要求LCP、INP、CLS各自P75均达到Good；任一项不足时显示“数据不足”，不能将现有两项优秀视为整体通过。
- 各指标独立计算样本分母，不要求每个页面浏览都有五项数据。INP缺失不能填0、不能按Good计入。
- 性能设备类型沿用服务端UA解析，与既有Stats设备口径一致；热力图设备组依旧按视口断点。缩小桌面窗口不因此变成性能面板的移动设备，两种口径在帮助信息中区分。
- CLS允许有效的0值。性能耗时不使用采样倍率外推，样本数标为真实采样量。
- 缺失原因仅报告能够观测的内容，如API不支持、无交互、后台初始化；无法判断的统一“未收到”，不猜测浏览器内部原因。
- 第一版建议每项少于100个有效样本时标记“小样本”，仍可查看数值但不作稳定达标判断。100是产品展示门槛，不是Google或CrUX的官方要求。
- 不宣称ZenStats结果等同CrUX、Search Console或PageSpeed实验室数据，设备、地区、样本及浏览器覆盖均有差异。
- 性能与跳出率可并列展示，不能据此断言性能造成跳出。

### 4.2 官方库接入

- 新增独立 `vitals.js`，编译打包官方 `web-vitals` 标准构建，不手写INP或简单累加CLS。
- 锁定稳定依赖及lockfile，不使用浏览器运行时第三方CDN、`latest`或`soft-navs`标签。
- 每份文档只注册一次onLCP／onINP／onCLS／onTTFB／onFCP；SPA路由切换不反复创建Observer。
- 使用默认报告模式，生产环境首版不启用 `reportAllChanges`；即使如此，hidden后重新可见仍可能产生后续报告。
- 配置建议由轻量引导模块预先获取；被抽中的可见导航尽早加载监控模块。虽支持buffered，也不能假定晚加载能补齐全部交互细节；记录初始化延迟并验证对INP的影响。
- 默认不加载attribution构建，不采集DOM文本、CSS选择器、资源完整URL或脚本堆栈。后续若增加诊断，优先显式 `data-zs-id` 和脱敏资源路径。

### 4.3 生命周期、路径归属和更新

- 给性能测量生成文档生命周期临时ID；BFCache恢复新建生命周期，官方库的新metric.id进入新样本。
- 以 `(site_id, lifecycle_id, metric_name, metric.id)` 为指标实例键，每次报告递增revision；乱序抵达时取最新revision，重复请求不增加权重。
- INP等统计值可能因后续交互而变化，不能通过取max(value)代替最新值。
- MVP保持官方默认完整文档导航口径。首次导航URL冻结，后续SPA变更不能把迟到的LCP／INP归到当前地址。
- 若库提供navigationURL，使用该值并做同站点和规范化校验；建立与生命周期的一致映射。
- 后续soft navigation作为独立导航类型、独立开关及浏览器能力分组；需核验6.2.3的reportSoftNavs行为。不得用路由耗时冒充标准LCP，soft navigation的TTFB=0也不能混入硬导航TTFB分布。
- 同生命周期的context和全部指标统一按 `navigation_started_at` 分桶，使覆盖率的分子／分母对齐；不按每次report接收时间跨桶重复归属。`metric_started_at`用于诊断，不能单独改变统计归属。
- 发送一个轻量生命周期上下文记录，用于测量采样导航量、初始化状态和API支持能力；没有指标回调的浏览也能反映覆盖情况。

建议报告字段：

```text
protocol_version, domain, report_id, lifecycle_id, revision,
metric_id, metric_name, value, metric_started_at, reported_at,
navigation_started_at, navigation_type, hostname, path,
release_version, device_class, browser_family, country_code,
sample_rate, config_version, init_delay_ms, support_status
```

country_code由服务端定位；不信任客户端站点ID、国家或权限字段。context记录为独立record_kind，不用虚构数值指标表示。

## 5. 存储、聚合与资源控制

### 5.1 建议新增的表

| 数据库 | 建议表 | 用途 |
| --- | --- | --- |
| PostgreSQL | 扩展现有heatmap_configs | 点击／滚动开关及滚动报告限制；沿用路径和分组规则 |
| PostgreSQL | site_performance_configs | 性能开关、采样率、保留期、允许路径及版本配置 |
| ClickHouse | heatmap_scroll_reports | 滚动快照、revision和质量标记；默认原始保留7天 |
| ClickHouse | heatmap_scroll_rollups | 去重后的最大深度直方图、有效浏览数和首屏比例汇总；默认90天 |
| ClickHouse | web_vitals_reports | context和指标更新记录；默认原始保留7天 |
| ClickHouse | web_vitals_rollups | 指标分位数状态、三档计数、样本数及必要覆盖统计；默认90天 |

- 先改ent schema，再生成代码和迁移；不手改ent CRUD。
- 原始更新可使用ReplacingMergeTree(revision)。滚动按站点／浏览开始日／page_view_id，性能按站点／导航开始日／lifecycle_id／metric_name／metric.id确定实例身份；report_id及更新时刻不放入替换键。查询不能等待后台合并，显式argMax去重，避免全表FINAL。
- 滚动按0～80的最大深度桶保存直方图，查询时累加深度尾部，返回80行触达率。桶0包含未达到第一行阈值的有效浏览，不得从分母丢弃。
- 性能使用 `quantilesTDigestState` 保存所需分位数状态，范围合并后求P75；不平均小时P75。三档占比通过有效样本计数合并。
- 汇总采用UTC **15分钟桶**，对外可再合并为小时／天，以覆盖当前常见的半小时、45分钟时区偏移。桶数相对小时汇总最多增加4倍，需要纳入压测；不增加浏览实例数。
- sample_rate和config_version属于固定样本属性，保留到聚合层。首版展示实际采样分布，不宣称采样比例变化后仍是无偏的全量分布；混合采样率时显示提示，完整加权方案另立验收。
- 首版控制分组维度：滚动沿用现有布局身份；性能为站点／主机／路径／发布版本／设备／浏览器大类／国家／导航类型。禁止任意props、完整UA、精确视口尺寸进入聚合键。
- 性能路径聚合独立于热力图截图模板配置；可以复用路径匹配函数，但不因为修改热力图标识而重解释历史性能数据。

### 5.2 可更新数据的聚合策略

**禁止直接使用当前点击MV的套路对滚动或性能更新求和。** 点击UUID代表不可变事件；这两项功能的同一实例会更新，计数和分布必须先按revision选最新值。

推荐先落实以下策略，第0阶段用真实ClickHouse完成原型验证：

1. 最近数据以原始报告去重后查询；短时缓存相同条件查询。初始只开放7天以内，先验证采集和统计正确性。
2. 开放30／90天查询前，落地“可变近期数据＋已封账历史汇总”。建议晚到／更新窗口48小时，按UTC日封账，查询明确使用已提交watermark分界。
3. 已封账部分查询15分钟状态，未封账部分查询raw并产生相同类型状态，先合并状态再计算最终P75及触达率；两者时间区间不得交叠。
4. 后台从去重raw构建临时汇总分区，核验样本数后原子替换正式分区并推进watermark。重跑、崩溃恢复不能追加第二份分布。
5. 封账发布前必须确认相关写队列已处理到边界；多实例、配置缓存、跨日队列和迟到重试需共同遵守封账规则。不能只等待固定时间就假设队列排空。
6. 超过已封账更新窗口的长开页面或延迟报告明确拒绝更新并记录too_late，不新建虚假指标样本；界面说明该截断限制。业务要求接收更晚更新时，先扩展窗口及重算协议。
7. raw TTL需要晚于封账、补算及故障恢复窗口。未完成汇总不得静默让raw过期；通过任务状态和数据缺口告警暴露风险。

该方案的封账协调、合并状态SQL和异常恢复是必须通过的设计验证项；若原型未通过，保留“7天raw查询”的明确限制，不能提前宣称支持90天正确汇总。

### 5.3 上报质量与资源预算

- 两项业务使用独立有界队列及质量计数，复用经过验证的实现模式；设服务级共享写入和查询预算，避免新增模块成倍放大并发。
- `202`只表示接受进入内存队列；sendBeacon返回true只表示浏览器已排队，均不等于落库成功。本期不承诺浏览器到数据库零丢失。
- 质量指标至少包括received、accepted、filtered、rejected、persisted、failed、retries、pending、oldest_age及最后成功／失败时间。
- 增加invalid、too_late、unsupported、late_init、clipped、aggregation_lag及封账失败原因。累计计数应说明实例和重启范围；persisted报告数不等于去重浏览数。
- 首版不扩展持久消息队列；需可靠投递时单独评估WAL／持久队列，不能通过无限重试伪装可靠。
- 浏览器单批建议≤16KiB，限制条数与队列长度；foreground用fetch以获得状态，离站优先sendBeacon，失败回退fetch keepalive；两种方式均不绕过浏览器共享的keepalive队列预算。
- 限制重试次数及退避时间；同报告不变ID和revision；429遵守Retry-After，永久性校验失败不重试。禁止用unload／同步XHR阻塞导航。
- 服务端复用Origin／域名／Bot／屏蔽规则，独立配置限流，拒绝超长路径、NaN、Infinity、未知指标和超范围值；不静默将错误数值修正成Good。
- 所有新增分析和汇总工作建议共享最大1个可选重查询槽，单查询1线程、≤128MiB、5秒初始预算；超限返回可理解的资源错误，不阻塞基础事件／Session写入。
- 批写初始参考1秒／256条；队列容量、请求限制及性能预算均作为压测起点，不是已验证容量承诺。

## 6. API与配置草案

端点均包含现有 `/api` 前缀；请求和响应按项目统一约定，下面是拟新增契约。

| 方法与路径 | 认证 | 用途 |
| --- | --- | --- |
| POST /api/heatmap/scroll | 公开，Origin等校验 | 滚动报告批次 |
| GET /api/sites/:domain/heatmap/scroll/pages | JWT＋站点成员 | 仅有滚动、没有点击的布局也可选 |
| GET /api/sites/:domain/heatmap/scroll/data | JWT＋站点成员 | 日期、设备、路径／标识的滚动分布 |
| GET /api/performance/config | 公开，只返回采集必需字段 | 性能能力版本与开关 |
| POST /api/performance/reports | 公开，Origin等校验 | context及性能报告批次 |
| GET/PUT /api/sites/:domain/performance/config | 读：成员；写：owner/admin且非子账号 | 性能配置 |
| GET /api/stats/:domain/web-vitals | 沿用统计JWT／ApiKey权限 | 性能总览和趋势 |
| GET /api/stats/:domain/web-vitals/pages | 同上 | 页面排行和分页 |
| GET /api/admin/collection-quality | 管理员 | 新功能质量分项，可整合已有质量面板 |

- 现有heatmap config扩展clicks_enabled／scroll_enabled等能力；enabled作为总开关，首次部署默认不启用新增采集。
- 性能配置与热力图配置分开：性能可覆盖全站，不要求截图，不受热力图max_clicks影响。
- 滚动查询响应包含sampled_views、valid_views、excluded_views_by_reason、coverage_flags、80个阈值及count／rate、average_max_depth、average_fold和aggregation_watermark。
- 性能响应包含每指标value／unit／valid_samples／good／needs_improvement／poor、context样本量、可解释的覆盖信息、navigation_type及aggregation_watermark。
- 滚动默认全部有效采样浏览；性能默认全部有效指标实例。所有过滤必须同样作用于分子和分母，不能把全站总PV作为某页面的采样分母。
- 时间范围采用站点时区的[start,end)并转UTC。首版日期查询边界必须与15分钟桶对齐，支持常见整小时、半小时和45分钟偏移及其DST；其他精细自定义时间范围在raw保留期内补算，超出raw范围则明确拒绝，不四舍五入、不返回伪精确结果。
- 首版性能筛选为时间、主机、具体路径、设备、浏览器大类、国家、发布版本及导航类型；不假定能直接支持全部现有业务props和转化条件。
- 共享链接不自动获得新功能；只有明确授权并测试后再开放。采集接口不依赖用户JWT，查询接口不能信任客户端site_id绕过成员校验。

## 7. 实施阶段与验收门槛

| 阶段 | 工作 | 完成标准 |
| --- | --- | --- |
| 0：口径与技术原型 | 浏览临时ID和生命周期映射；同实例乱序更新；封账／去重／分位数合并；时区边界；标准库本地打包 | 重试／更新不增加样本权重，最新版本生效；故障重跑不重复聚合；明确首版导航和时区范围 |
| 1：滚动采集及7天图 | PG开关、raw表、采集端点、SDK、80行截图图层、质量信息 | 没有点击的浏览在分母；SPA不串页；短页／长页／动态高度／重复上报／设备切换口径正确 |
| 2：滚动历史汇总 | 幂等封账、15分钟直方图、watermark、90天日期查询 | raw与rollup交界不重叠；多实例、DST／非整点时区、TTL和补算通过 |
| 3：性能采集及面板 | 自托管web-vitals、性能配置、context／metric去重、P75／分级／页面排行 | 无交互INP显示缺失；BFCache独立样本；SPA归属正确；每指标样本数可核验 |
| 4：性能历史汇总及压测 | 可合并分位数、覆盖计数、30／90天、资源门槛、故障演练 | 合并状态与全量去重基准一致到预设误差；内存／写入不拖累核心统计 |
| 后续：可选增强 | soft navigation实验支持、资源归因、滚动目标、版本对比／告警 | 单独开关、单独导航口径、数据脱敏和迁移验收 |

建议分为两个功能迭代，并将第0阶段作为共同前置。未跑完原型与压测前不承诺具体天数；历史聚合是两项功能的主要后端工作量，不能当作“增加两张图表”。

### 必测场景

1. 浏览器：短页、首屏无交互、锚点进入、懒加载、无限滚动、SPA前进／后退／同URL重复浏览、BFCache、隐藏后返回、prerender激活、断网／重试、脚本重复加载、配置关闭、私有模式存储不可用。
2. 性能指标：CLS=0、INP缺失、指标迟到、同metric多次回调且值下降、unsupported API、buffered晚加载、后台初始化、页面URL在报告前变化。
3. ClickHouse：乱序revision、重复块、后台未合并、写入部分失败、补算两次、封账中断、atomic替换前后、跨小时同metric、队列延迟越过封账边界。
4. 权限：跨站点报告、非法Origin、路径／hostname伪造、管理员／成员／子账号／ApiKey／共享链接边界。
5. 压测：固定硬件与现有业务数据基线，分别测试新增功能关闭、10%采样、100%采样及浏览器重试风暴；记录事件写入吞吐／延迟、队列积压、CH峰值内存、查询P95和后台任务耗时。

### 容量估算方式

以100万次页面浏览、10%采样为例：滚动约10万有效浏览实例，性能最多约50万个五指标实例，另有约10万个context实例；原始更新次数通常更多，并受浏览器支持、活跃时长、隐藏次数和限额影响。

80个滚动区块不代表表只有80行，UUID／revision记录和页面、设备、版本等维度仍随流量增长。必须用实际压缩字节／实例、报告次数／浏览和聚合维度基数估算7天／90天磁盘与查询成本。

## 8. 代码落点

- Tracker：`tracker/src/zenstats.js`提供最小生命周期通知；`tracker/src/heatmap.js`接入滚动状态；新增`tracker/src/vitals.js`；`tracker/compile.js`增加独立性能资产构建。
- 后端：扩展`internal/heatmap`滚动模型；新增`internal/performance`领域模型／采集器；外部入口位于`internal/api/external`，查询与配置经service→repository。
- CH：表与迁移在`internal/store/clickhouse`，读取和幂等汇总在repository；不把两项数据塞入events或sessions。
- PG：heatmap配置schema扩展及performance配置schema；`make ent-generate`后迁移。
- Web：热力图模式和滚动图层位于`src/pages/sites/heatmap`；性能面板与设置独立组件，沿用统计日期和权限入口。
- 任务：新增cron只能追加到`internal/bootstrap/cron.go`末尾，沿用PG advisory lock；任务结果需有持久watermark和失败状态，不能只靠内存计数。
- 校验：Go构建／vet、相关DB集成测试、Tracker及页面Playwright、Web TypeScript构建；随后执行固定基线压测。

## 9. 参考资料

以下为本次实际查阅的一手资料。外部文档和npm版本会变化，实施时再次核对所锁定版本。

1. [Plausible：Scroll depth tracking](https://plausible.io/docs/scroll-depth)，本次页面标注更新于2026-11-05。
2. [Microsoft Clarity：Scroll maps](https://learn.microsoft.com/en-us/clarity/heatmaps/scroll-maps)，本次页面标注更新于2026-10-06。
3. [Google：Web Vitals](https://web.dev/articles/vitals)，核心指标、P75和RUM／CrUX区别。
4. [GoogleChrome/web-vitals v6.2.3 README](https://github.com/GoogleChrome/web-vitals/blob/v6.2.3/README.md)，回调、缺失、重复报告、BFCache及soft navigation。
5. [npm web-vitals元数据](https://registry.npmjs.org/web-vitals)，本次latest=6.2.3；不建议运行时依赖latest。
6. [MDN：Navigator.sendBeacon](https://developer.mozilla.org/en-US/docs/Web/API/Navigator/sendBeacon)，排队语义及64KiB预算。
7. [ClickHouse：Incremental materialized views](https://clickhouse.com/docs/materialized-view/incremental-materialized-view)，增量视图处理插入块的语义。
8. [ClickHouse：quantileTDigest](https://clickhouse.com/docs/sql-reference/aggregate-functions/reference/quantiletdigest)，近似分位数及多分位数建议。
