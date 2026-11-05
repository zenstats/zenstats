# 流量统计准确性评估与改进

评估日期：2026-11-04。范围：本地 `zenstats` 后端、`zenstats-web/tracker` 和隔离测试数据库；没有读取生产流量，也没有部署本次修改。行业口径参考下方官方文档。

ZenStats 已具备事件采集、会话聚合、分群、目标、漏斗、收入事件和历史导入等基础能力。当前优先级应是统一采集与报表口径、减少漏计与误归因，然后建设采集质量监控。不能用“数据接近 GA4”直接证明准确：平台之间的访客识别、跳出率、过滤规则和归因窗口本来就不同。

本项目更适合沿着 Plausible 一类轻量、自托管分析的方向完善。需要同时明确：**无 Cookie 不等于没有可关联数据**。当前访客 ID 使用固定持久化 salt，ClickHouse 的事件/会话还保存 IP 和原始 User-Agent；这与 Plausible 的每日 salt、无原始 IP/UA 存储有明显差别。

## 与主流平台的关键区别

| 维度 | ZenStats 当前实现 | 主流平台参照 | 对准确性的含义 |
| --- | --- | --- | --- |
| 访客 UV | `hash(IP + UA + 注册域名 + 持久化 salt)` | GA4 通常结合客户端标识、User-ID 等报告身份；Matomo 默认使用访客 Cookie，也支持其他模式；Plausible 使用每日轮换 salt 的匿名标识 | 同一 NAT 下相同 UA 可能合并；换网络或 UA 可能拆分；跨日 UV 不能直接与 Plausible 对比 |
| 会话 | 事件时间驱动、30 分钟不活跃超时；进程内按用户串行更新、缓存 miss 回读数据库 | GA4 和 Plausible 默认也是 30 分钟不活跃，GA4 超时可配置 | 超时相同不代表会话数必然相同；识别方式、并发实例与事件顺序同样影响结果 |
| 跳出率 | 单页且没有交互事件仍为跳出，engagement 心跳不解除跳出 | GA4 跳出率是非参与会话比例；参与会话满足超过 10 秒、发生关键事件或至少 2 次页面/屏幕浏览之一 | 单页阅读 60 秒在两个平台可能得到不同跳出结论；应另加 `engaged_sessions` / `engagement_rate`，不要静默替换原指标 |
| 页面停留 | 本次改为活跃时长增量总和 / PV；无参与上报的 PV 按 0 秒 | GA4 的参与度相关指标有自己的分母；不同平台的访问时长也可能采用首末事件差 | `time_on_page` 与 `visit_duration` 含义不同，跨时间窗口的心跳也存在边界误差 |
| 来源归因 | 会话首次事件保存来源；本次让 `visit:source` 优先使用 UTM，并让筛选与分组都读取会话来源 | GA4 区分用户、会话、事件归因，还支持更复杂广告与归因模型 | 当前是会话获客来源分析，不等于多触点转化归因；不能承诺与广告平台转化数一致 |
| 去重计算 | 本次主查询引擎的访客/事件侧会话去重改为 `uniqExact`；显式采样仍为估计 | GA4 官方说明部分用户/会话指标采用 HLL++ 估算 | 精确去重消除该查询的算法估算误差，但无法纠正访客识别、漏报或重复上报 |
| 机器人 | 浏览器自动化标记、UA 识别、规则过滤、代理 IP 类型头 | 成熟平台综合已知机器人规则、网络信息和自定义过滤 | UA 可伪造，未知设备也不一定是机器人；不应仅靠扩大屏蔽范围提高“纯净度” |
| 隐私与留存 | 固定 salt；保存 IP、原始 UA；普通 URL 参数移除 | Plausible 每 24 小时轮换并删除 salt，不保存原始 IP/UA | 需设计匿名化、保留期限和可解释的访客口径，避免把长期识别包装成每日匿名统计 |

## 本次已实现的修复

| 问题与根因 | 修复后的行为 | 主要实现位置 |
| --- | --- | --- |
| 首次 10 秒心跳后把计时起点设为 `null`，持续阅读不再累计 | 每次心跳上报增量后继续计时；退出时补记末尾不足一个周期的部分 | `zenstats-web/tracker/src/zenstats.js` |
| 仅依赖 `beforeunload` 刷队列，短访问和移动端生命周期容易漏报 | 页面隐藏或 `pagehide` 刷出队列；心跳与 PV 使用同一队列；BFCache 恢复重新建立页面计时 | 同上 |
| Hash 变体实际没有传 `h`，不同页面可能全部归到 `/` | PV、自定义事件与 engagement 均传 `h: 1` | 同上 |
| Hash 内查询参数进入 pathname，同页面被拆为多个维度，还可能包含 token | Hash 路径去查询参数；空首页路径统一为 `/`；仅 Hash 查询参数变化不新增 PV | Tracker 与 `internal/event/event.go` |
| 默认 Tracker 不捕获 `replaceState` 导航 | 捕获实际路径变化，重复路径和普通 query-only 更新不额外增加 PV | Tracker |
| 站内或子域跳转被记作外部 Referral | 通过公共后缀列表比较注册域名排除自引荐，同时保留 UTM；不同 `github.io` 租户保持区分 | `internal/event/event.go` |
| 搜索/社交来源靠子串匹配，`google.com.evil.test` 也可能被归为搜索 | 使用完整域名或子域边界；增加 `x.com`、`t.co` 等常见社交来源；支持 `organic` / `referral` medium | 事件渠道分类与统计来源 CASE |
| 有 UTM、没有 Referrer 的活动被显示为 Direct；来源筛选与分组可能使用不同的数据范围 | UTM source 优先；`visit:source` 按会话首次来源分组和筛选；明确限定查询字段作用域 | `internal/service/stats/sql/` |
| `avg(engagement_time)` 实际统计的是平均心跳时长 | 对时长增量求和后除以 PV；2 次 PV、其中一次停留 25 秒，结果为 12.5 秒 | `fragments.go` |
| 采样时把滚动百分比也乘以采样系数，可能超过 100% | 比例不乘采样放大系数；Tracker 上报滚动深度限定在 0–100 | Tracker 与 `fragments.go` |
| 事件表排除 engagement，但会话的 Events 仍累加它 | 新的会话更新中，心跳只更新活动时间等状态，不增加业务事件数 | `internal/session/session.go` |
| 不采样查询仍使用近似 `uniq` | 主查询引擎改为精确访客/会话去重；采样模式仍明确属于估计 | `fragments.go` |

## 仍需继续建设的部分

下面是源码检查确认的设计限制或风险，没有生产数据证明它们的实际发生比例。

| 优先级 | 差距、证据与影响 | 建议与验收方式 |
| --- | --- | --- |
| P0 | `pkg/iputil/iputil.go` 直接采用最左侧 X-Forwarded-For / X-Real-IP，未核验请求是否来自受信代理；IP 会影响 UV、GeoIP 和屏蔽 | 将代理 CIDR 与部署拓扑统一配置，从可信链解析 IP；直连伪造头不能改变访客地址，CDN→反向代理→应用仍能解析真实地址 |
| P1 | API 202 不表示落库：可能已入内存队列，也可能被过滤。队列/写缓冲没有持久化，写入重试耗尽会丢弃 | 保留异步批写，先完善失败批次保留、容量控制、关闭顺序和分阶段计数。若要求进程崩溃后恢复已接收事件，再增加返回 202 前的持久化接收层；详见下文 Plausible 对照 |
| P1 | 事件请求没有 event_id。现有 ClickHouse 批次 dedup token 仅防同一批次写重试，无法识别客户端重复请求 | 增加每次事件唯一 ID，重试复用 ID，设去重有效期；同一事件重复发送 10 次只计一次，真实两次刷新仍计两次 |
| P1 | `processWorker` 并发预处理，之后才进入会话 balancer；同用户任务会串行执行，但不保证原始事件时间顺序。多个实例的缓存与 balancer 也不共享 | 使用稳定用户分区、单一会话状态所有者或可重算的会话聚合；测试倒序批次、缓存淘汰、实例重启、跨实例交错与午夜边界 |
| P1 | UV 依赖 IP/UA，固定 salt 可跨日关联；不能识别“真实人数” | 保持默认隐私模式，明确 UV 是估计访客；如需回访率/跨设备，单独设计经用户选择的第一方标识模式，配套隐私与保留策略 |
| P2 | `Other` 设备类型直接被当机器人；数据中心 IP 类型取自请求头 | 将“已知机器人”和“未知设备”分开；建立真实浏览器、内嵌 WebView、机器人样本回归集；代理必须覆盖而非透传不可信分类头 |
| P2 | 每个心跳携带累计最大滚动深度，报表当前仍按心跳平均，长时间访问权重更高 | 增加 pageview_id，先按单次浏览取最大深度，再在浏览之间平均；多标签页和重复访问同 URL 不应混为一次浏览 |
| P2 | 渠道域名表与可配置来源名称是两套逻辑；付费社交、广告 click ID、引荐排除、多域站点归因尚不完整 | 建立统一且可版本化的来源规则；优先明确 UTM 与广告参数的覆盖顺序，校验分组与过滤使用相同表达式 |
| P2 | 页面不可见时的 SPA 导航、prerender、真实浏览器 BFCache/移动端终止仍需更广泛验证；当前生命周期回归包含人工派发的浏览器事件 | 补充真实导航的 Chromium/WebKit/Firefox 测试与 Safari 移动端抽样；不要把 `keepalive` 视为必达保证 |

采集质量计数现已落地（见下方 2026-11-05 实施记录）。后续优先完成可信代理、失败批次保留、事件去重及多实例会话一致性；是否增加持久化接收层取决于故障恢复目标。

## Plausible 对照：202 与写入可靠性（2026-11-05 补充）

核对 Plausible 公开仓库提交 `06a435ef177f695ca7bc6321fbd3e066bf517ab0` 后，需要澄清：202 与异步内存缓冲本身是合理取舍，不能仅因此认定 ZenStats 必须引入持久化消息中间件。

该版本配置默认选择 `embedded` persistor。接收接口调用 `build_and_buffer` 后返回 202；某些被过滤的事件也返回 202，并附带 `x-plausible-dropped`。Embedded 实现先做会话处理，再向事件写缓冲投递；缓冲用 GenServer 内存状态保存 RowBinary，按时间或字节阈值写 ClickHouse。默认刷新间隔为 5 秒、触发刷新阈值为 100,000 字节；后者是刷新条件，不等于整条采集链的硬内存上限。缓冲还实现了关闭时刷新，并通过 telemetry 区分 buffered 与 dropped。

这些证据只描述公开源码的 Embedded 路径。仓库另有 Remote / EmbeddedWithRelay 实现，不能据此推断 Plausible Cloud 的全部基础设施，也不能把 buffered 计数解释为成功持久化计数。

**初始评估与轻量方案建议（实际实施范围见下方）：**

| 顺序 | 当前具体问题 | 建议 |
| --- | --- | --- |
| 1 | `processEvent` 的过滤和处理错误都可能以 nil 结束；写缓冲也有丢弃分支，只有日志很难计算损失 | 区分 filtered、rejected、processing_failed、buffered、persisted、dropped；增加 pending/inflight 的条数、字节数、最老等待时间。按终态事件核对，不能直接相减 HTTP 请求数与含多个版本行的 session 写入行数 |
| 2 | events 尝试写入 5 次、sessions 尝试 3 次后，失败批次仍被释放 | 批次保持独立所有权，失败后进入有容量和时限约束的待重试集合；使用退避与抖动。保留同一批次的行内容和 token，避免重新执行会话加一逻辑。永久数据错误单独隔离和计数，不能反复阻塞有效批次 |
| 3 | 每次 flush 会取走整个缓冲，而非严格限制批次大小；session 缓冲无显式容量上限 | 对入口、待处理、待重试及在途批次一起预算内存；设置单批最大行数/字节数和写入超时。容量不足时在接受前返回 503 + Retry-After；站点配额/限频继续使用 429。当前 batch API 可能部分入队后返回 503，需增加容量预留或子事件结果/去重，不能直接整包重发 |
| 4 | HTTP Shutdown 与队列排空实际并发执行，尽管注释写着先停止 HTTP；容器终止时间也未给全部收尾留出余量 | 先停止接收并等待在途请求完成，再关闭入口队列、排空处理任务和两路写缓冲，最后关闭数据库。统一配置总期限，使部署的终止宽限期大于应用收尾预算 |
| 5 | events、sessions 分别确认写入；只保住一边仍会产生指标不一致 | 同时跟踪两路写入结果；session 失败状态保持可恢复、可查询，不能因批次失败立即丢失 pending 状态。相关容量预留应在会话状态变更前完成 |

当前 ClickHouse `non_replicated_deduplication_window = 1000` 是最近块的窗口，不是 1000 秒。延长重试时必须核对实际块产生速率和去重窗口；重建 token、改变批次内容，或超过去重窗口后重放，都可能重复计数。现有 token 只在单次 `writeBatch` 调用内复用，不能直接假定跨轮次/重启安全。

轻量方案的验收目标是：在配置的容量与故障时间预算内，ClickHouse 短暂不可用后能够补写，且不重复计数；正常发布能够排空；达到预算上限时有明确拒绝或丢弃原因。它仍不能承诺 SIGKILL、OOM 或主机故障后恢复所有已返回 202 的事件。

**需要更强保证时再增加持久化接收：** 对准备接纳的事件先写入本地 WAL 并完成同步落盘，或提交 PostgreSQL inbox，再返回 202。消费者批量处理，确认所需输出完成后才推进 checkpoint。仅在 ClickHouse 写失败后才写磁盘不能保护“已返回 202、尚未进入写缓冲”的窗口。

持久化回放必须覆盖会话状态：重新调用现有 `OnEvent` 会生成或累加会话状态，可能重复计算 PV/事件数。需要设计幂等的处理记录、会话更新与确认进度，保留稳定 ID/token；events 与 sessions 也没有跨表事务。WAL 需要持久卷且只能保证对应存储仍可用的故障范围。具体选型应结合单实例/多实例、故障恢复目标和写入吞吐测试确定。

建议故障用例：ClickHouse 暂停后恢复、写成功但响应丢失、缓冲满、batch 部分接收、坏数据行、SIGTERM 时有在途 HTTP 请求、仅一张表写失败；启用持久化模式后再验证各个确认边界的 SIGKILL 恢复。

**2026-11-05 实施记录：** 已加入管理员采集质量页面/API、事件阶段和终态计数、60 分钟趋势、两路写入指标与失败原因；统一单/批校验、整批原子入队与失败配额回退；增加有限批次、session 缓冲容量上限、成对入缓冲、单次写入超时、坏行整批失败及正确的 HTTP→队列关闭顺序，部署宽限期调整为 60 秒。已完成真实数据库与并发回归。**尚未实现**持久化、跨轮次失败批次保留、统一字节预算、多实例指标汇总或跨表一致性恢复。202 的接收语义不变。完整口径、排查方法与验收范围见[采集质量监控](ingestion-quality.zh-CN.md)。

源码参考（固定提交）：[接收接口](https://github.com/plausible/analytics/blob/06a435ef177f695ca7bc6321fbd3e066bf517ab0/lib/plausible_web/controllers/api/external_controller.ex)、[Embedded persistor](https://github.com/plausible/analytics/blob/06a435ef177f695ca7bc6321fbd3e066bf517ab0/lib/plausible/ingestion/persistor/embedded.ex)、[内存写缓冲](https://github.com/plausible/analytics/blob/06a435ef177f695ca7bc6321fbd3e066bf517ab0/lib/plausible/ingestion/write_buffer.ex)、[默认配置](https://github.com/plausible/analytics/blob/06a435ef177f695ca7bc6321fbd3e066bf517ab0/config/runtime.exs)、[采集计数](https://github.com/plausible/analytics/blob/06a435ef177f695ca7bc6321fbd3e066bf517ab0/lib/plausible/ingestion/counters/telemetry_handler.ex)。

## 指标和升级影响

- PV 统计 `pageview`，events 包括 PV 和自定义事件，不包括 engagement。`visitors` 精确去重的是生成的 user_id，不是现实中的自然人。
- `time_on_page` 表示本查询时间窗口内收到的活跃时长 / PV；没有 PV 的窗口返回 0。跨窗口心跳及上报失败会影响结果。该指标不应与“最后一个页面事件减第一个页面事件”的会话时长混用。
- 旧 Tracker 漏掉的活跃时长和 Hash 路径无法从数据库补回；旧会话中的 Events 也不会自动扣除历史心跳。报表公式修复会影响历史数据的查询结果，应记录发布时间作为口径分界。
- `uniqExact` 的内存开销随去重基数增长。大型站点应压测最长日期范围、高基数分组，并在需要时显式启用采样、展示采样状态。未进行生产规模性能测试。
- 本次没有数据库字段迁移。发布时需同时更新后端和重新编译的 Tracker，并刷新脚本缓存；仅替换后端不能修复旧 SDK 的漏报。
- 仍采用已有的单页/交互事件跳出口径。将来增加 GA4 式参与率时，应增加独立指标和说明，保留历史可比性。

## 验证方法

后端使用真实 PostgreSQL 18 和 ClickHouse 25.11 的独立测试容器；新增 `TestTrafficAccuracyIntegration` 验证 100,000 个访客和 50,000 个会话精确去重、25 秒活跃时长在两次 PV 间平均、采样下百分比不放大、UTM 分组/筛选与会话首次来源一致。新增事件处理回归覆盖首页、Hash 参数、自引荐、多级公共后缀和仿冒来源域名。

Tracker 使用编译后的脚本在 Chromium 中运行 7 个回归场景：连续 25 秒、500ms 短访问、后台时间排除、Hash、History replaceState、BFCache 生命周期、重复安装。测试使用受控时钟；隐藏与 BFCache 场景验证处理程序，不等于完整的移动端浏览器认证。

本次结果：上述 7 个浏览器场景通过；事件、会话及 SQL 包相关测试通过，包含真实数据库集成用例；64 个 Tracker 变体编译通过；`go build ./...`、`go vet ./...` 和 `npx tsc -b` 通过。隔离测试容器在验证结束后停止。

```sh
# 从后端仓库启动隔离数据库
make test-up
APP_ENV=test go test ./internal/event ./internal/session ./internal/service/stats/sql -count=1
go build ./...
go vet ./...

# 从 zenstats-web/tracker 执行
npx playwright install chromium
npm test

# 从 zenstats-web 执行
npx tsc -b
```

本报告的验证范围是本次修改及相关回归，不是全站生产验收或完整故障注入。对齐外部平台时，应先统一站点、时区、日期范围、PV/UV/会话定义、UTM 规则、机器人过滤和同意状态，再比较差异原因，不要增加人为补偿系数。

## 官方参考

- [GA4：会话、默认超时与估算计数](https://support.google.com/analytics/answer/9191807?hl=en)
- [GA4：参与率与跳出率](https://support.google.com/analytics/answer/12195621?hl=en)
- [Plausible：指标定义](https://plausible.io/docs/metrics-definitions)
- [Plausible：访客标识和数据政策](https://plausible.io/data-policy)
- [Matomo：默认与可选追踪 Cookie](https://matomo.org/faq/general/faq_146/)

代码证据入口：[事件处理](../internal/event/event.go)、[会话管理](../internal/session/session.go)、[统计查询](../internal/service/stats/sql/query_builder.go)、[IP 解析](../pkg/iputil/iputil.go)、[事件写缓冲](../internal/event/buffer.go)、[Tracker](../../zenstats-web/tracker/src/zenstats.js)。
