# 点击热力图：实施与验收

本轮在三个仓库的 `feat/click-heatmap` 分支实现第一版。功能默认关闭，未操作生产环境。

## 第一版范围

- 独立点击采集、页面访问级采样、白名单、限流、有界队列、写入重试和质量监控。
- 按自然日查询，选择页面版本、布局变体、实际视口和文档尺寸。
- 上传同布局的 PNG/JPEG 全页截图；Canvas 网格热力图、透明度、元素点击排名。
- 原始点击默认保留 7 天，聚合默认 90 天；支持原始保留期内补算。
- 不提供点击 UV、曝光分母、点击率、会话回放、DOM 快照或自动截图。

热力点击不进入 events/session 管道，不生成用户标识，不触发 Session 恢复，也不占用普通业务事件配额。
热力图不改变 PV、UV、业务事件和 Session 统计口径。

## 安装与采样

1. 站点统计页域名菜单 → 点击热力图，保存开关及页面白名单。
2. 在原有 Tracker 标签加入 `data-heatmap="true"` 和显式 `data-heatmap-version`。
3. Hash 路由继续使用 `script.hash.js`，模块沿用其路由模式。
4. 可选 `data-heatmap-variant` 标记登录态/实验组等不同布局；默认为 `default`。

```html
<script defer data-domain="example.com" src="https://stats.example.com/js/script.js"
  data-heatmap="true" data-heatmap-version="release-2026-11"
  data-heatmap-variant="default"></script>
<button data-zs-id="pricing.signup">Sign up</button>
<div data-zs-ignore>Private content</div>
```

普通 Tracker 不加载热力模块。启用后从同目录加载 `heatmap.js`，每 30 秒刷新公开配置。
`data-api` 以 `/event` 结尾时推导同前缀 `/heatmap`；其他自定义代理路径用
`data-heatmap-api="https://.../custom/heatmap"` 显式指定；模块地址可用 `data-heatmap-src` 指定。
服务端配置缓存 10 秒，公开配置允许缓存 30 秒；跨实例关闭和浏览器停止监听存在短暂延迟。

白名单仅支持精确路径和单层 `*`，不跨 `/`；不支持 `?`、`[]`、转义及任意查询参数。
例如 `/pricing`、`/blog/*`、`/#/pricing`。页面路径去除普通及 Hash 路由查询参数。
采样决定和采样率在一次页面访问内固定；配置刷新不会重新抽样或改变旧点击的采样标签。
计数为实际采集且按 ID 去重的点击，不进行采样率放大。

只处理可信鼠标/触摸产生的左键 click，排除键盘和程序触发点击。
排除表单、输入框、contenteditable 和 `data-zs-ignore` 区域。
不上传文字、表单值、DOM、CSS selector、链接属性、IP、UA 或用户/session 标识。
路径本身仍可能含业务敏感信息，应选择稳定公开页面，避免个人路径、订单详情等页面。
现有 IP/hostname/country 屏蔽规则在服务端继续生效；Bot 和威胁 IP 被过滤。

## 定位协议与底图

页面匹配键为：

```text
site_id + hostname + canonical_path + version + variant
+ viewport_width + viewport_height + document_width + document_height
```

坐标单位为 CSS 像素，采集 pageX/pageY、尺寸、显式 data-zs-id、元素内相对坐标及定位类型。
视口尺寸限制 100～8192，文档最长边 100000。版本缺失归入 `unknown` 并在界面提示。
不同桌面宽度、文档高度、发布版本和布局不能混画；页面高度变化会生成新的可选布局。
第一版以准确匹配为优先，不自动推断动态布局相似性。

网格为 16 CSS px。截图等比缩放，热力点绘制在网格中心，边缘点限制在文档边界。
上传时根据完整文档宽高验证图片比例，支持 DPR 等比例截图；不拉伸纵轴来凑页面高度。
截图头部尺寸验证为 PNG/JPEG、≤5 MiB、≤2500 万像素，每站点最多 100 张。
用户需保证截图的实际版本和布局正确，尺寸校验无法判断截图内容是否属于该页面；上传前遮盖个人信息。

fixed、sticky、嵌套滚动及 pinch zoom 点击归入未投影数量，仍可参与显式元素排名。
第一版不采集截图对应 DOM 边界，不尝试通过元素标识重新定位历史点击。

底图元数据存 PostgreSQL，文件存 `${data_path}/heatmaps`，沿用 `/app/data` 持久卷。
文件只经站点鉴权接口读取，文件名不暴露给客户端，响应 private/no-store。
仅站点 owner/admin 的主账号可修改配置、上传/删除底图或补算；所有成员及子账号可读。
每小时追加的维护任务清理过期底图和一小时以上无元数据的孤儿文件；删除站点级联删除新增元数据。
多副本 API 必须共享同一个可写 data 卷；跨节点副本需共享文件存储，不能各用不同本地目录。

## 存储与故障恢复

| 对象 | 引擎/用途 | 默认保留 |
|---|---|---|
| heatmap_clicks | ReplacingMergeTree 原始点击，click_id/batch_id | 7 天 |
| heatmap_clicks_hourly | AggregatingMergeTree 小时网格/元素维度，uniqExactState(click_id) | 90 天 |
| heatmap_clicks_mv | 每次插入生成增量聚合状态 | 随表创建 |

同一条聚合记录同时带网格与显式元素维度，不额外生成第二份元素记录。
按网格查询或按元素查询分别 merge 精确 UUID 状态，总数独立 merge，避免双计数。
查询不依赖后台已合并，不使用全范围 FINAL。
原始表关闭插入块去重，每次重试均重跑物化视图；UUID 状态使已计入点击不会再次计数。
源表已提交、视图失败时，重复提交或补算可以修复缺失聚合。

精确 UUID 状态随点击数增长，聚合不是固定大小计数，也不承诺固定存储压缩倍数。
后台原始表替换重复行；查询按 ID 去重。保留期通过 TTL 异步清理，查询主动过滤过期聚合。
修改保留期只影响新点击，不改写历史记录的 expires。
热力时区在首次保存时采用站点时区并固定，时间戳存 UTC。
小时桶和日期边界覆盖半小时/45 分钟时区及夏令时。

```text
POST /api/sites/:domain/heatmap/replay
Authorization: Bearer <access token>
{"from":"2026-11-09T00:00:00Z","to":"2026-11-09T01:00:00Z"}
```

补算一次最多 24 小时，需在当前原始保留期内，可重复执行，超过原始保留期不保证可重建。
写入失败后依据日志、采集时间及质量指标选取时间窗口进行补算；第一版未实现自动差异核对和自动补算。

## 资源边界与质量监控

- 浏览器：最多 20 条/批、16 KiB/请求、2 秒刷新；待发队列 200，默认每页面访问 100 次点击。
- 客户端重试最多 3 次，click_id/batch_id 和请求正文保持不变，遵守限流等待；202 后不自动重发。
- 服务端：单站点每实例 6000 点击/分钟；队列 128 批，单 worker，256 行或 1 秒写缓冲。
- 写入最多 3 次，每次 2 秒超时；队列满 503/Retry-After，超配额 429/Retry-After。
- 单独的 ClickHouse 两连接池，查询并发 2；查询单线程、5 秒、128 MiB 内存限制。
- 布局列表最多 200，热力网格最多 10000（截断提示），元素排名前 100；点击总数包含全部网格。
- HTTP 停止后排空点击队列再关连接，Compose/K8s 关闭宽限期 120 秒。

管理员“采集质量”页面展示独立热力计数：received/accepted/rejected/filtered/persisted/failed、
pending、最早积压、重试、最近成功/失败、实例 ID 和启动时间。重启清零，多实例非全局聚合。
received/accepted/persisted 等含客户端重试，不能直接与去重后的报表点击数相减计算丢失率。
202 只代表有界内存队列接收，进程崩溃或重试耗尽仍可能丢失；failed 也可能有部分源表写入。
该方案不属于持久消息队列，未实现零丢失保证。

## 验证与发布

已纳入自动化：真实 PG/CH 接口、站点鉴权/子账号写限制、快照比例/文件读取/级联清理、
重复上报、真实物化视图失败、缺失聚合补算与重复补算、合并前后结果；时区及字段验证。
Tracker Playwright 覆盖 opt-in、真实点击、滚动/DPR、隐私过滤、Hash 切页、稳定重试 ID、
配置关闭/恢复和采样标签；管理界面测试覆盖 Canvas 坐标、底图匹配及子账号只读。

```sh
# 本地隔离数据库
cd ../zenstats-deploy && docker compose -f docker-compose.test.yml up -d --wait
# 后端
cd ../zenstats
go build ./... && go vet ./...
APP_ENV=test go test -race ./internal/api/router ./internal/heatmap ./internal/store/clickhouse/repository -run 'Test(Heatmap|Click|Canonical|Hour)' -count=1
APP_ENV=test go test ./internal/event ./internal/session ./internal/ingestion ./internal/api/external -count=1
# Web
cd ../zenstats-web && npm run build
# Tracker（界面测试需要单独运行前端 npm run dev）
cd tracker && npm test
ZENSTATS_WEB_TEST_URL=http://127.0.0.1:5173 npx playwright test test/heatmap-ui.spec.js
```

发布顺序：后端幂等迁移 → 网关/关闭宽限期 → 前端及 Tracker → 单站点手动启用。
首次建议仅 1～2 个稳定公开页面、10% 页面访问采样。关闭开关可停止新点击接收，回退应用保留新增表和文件。

上线灰度仍需实测：相同流量下关闭/开启热力图的主采集 p95、API/CH 内存、积压、每小时磁盘增长和合并压力。
以 p95 增幅 10%、API 增量 64 MiB 为初始观察门槛（低基线同时看绝对值），
结合压缩后字节/点击评估 7/90 天空间；本地功能测试不替代生产容量验证。
后续可扩展辅助截图/元素边界、对象存储、滚动热力图及有明确分母的点击率。
