# 点击热力图：实施与验收

本轮在三个仓库的 `feat/click-heatmap` 分支实现第一版。功能默认关闭，未操作生产环境。

## 第一版范围

- 独立点击采集、页面访问级采样、白名单、限流、有界队列、写入重试和质量监控。
- 按自然日查询，选择页面版本、布局变体和手机/平板/桌面设备组。
- 上传同布局的 PNG/JPEG 全页截图；Canvas 网格热力图、透明度、元素点击排名。
- 原始点击默认保留 7 天，聚合默认 90 天；支持原始保留期内补算。
- 不提供点击 UV、曝光分母、点击率、会话回放或 DOM 快照；自动截图由独立 worker 提供。

热力点击不进入 events/session 管道，不生成用户标识，不触发 Session 恢复，也不占用普通业务事件配额。
热力图不改变 PV、UV、业务事件和 Session 统计口径。

## 安装与采样

1. 站点设置 → 热力图设置，在列表新增热力图，在采集设置保存开关及页面白名单。
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
+ device_class (mobile / tablet / desktop)
```

坐标单位为 CSS 像素，采集 pageX/pageY、尺寸、显式 data-zs-id、元素内相对坐标及定位类型。
视口尺寸限制 100～8192，文档最长边 100000。版本缺失归入 `unknown` 并在界面提示。
按视口宽度区分手机（＜768）、平板（768～1023）、桌面（≥1024），单位 CSS 像素，不按 UA 判断。
同设备组内不同宽高和页面长度会合并，不再增加精确尺寸选项。不同主机、版本、变体仍隔离。

采用 40×80 比例网格，点击按 x/文档宽、y/文档高归一化，并在底图对应网格中心绘制。为让升级前后的重试落在同一网格，先使用原16px网格中心再归一化。
上线前统一将网格调整为40×80，不兼容此前开发版20×40小时聚合；已有开发测试聚合应重新生成，不能直接混用或仅通过Replay转换已有小时聚合。
这是大致区域展示：不同页面长度、动态内容和响应式细节可能导致位置偏移；不提供元素精确定位。
自动截图用设备标准视口（390×844、768×1024、1440×900），保存实际文档尺寸；手动上传按图片比例推导文档尺寸。
底图按主机、路径/标识、版本、变体和设备匹配，已有不同精确尺寸的同设备底图可继续使用。
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
- 布局列表最多 200，每个布局热力网格最多 3200（40×80），元素排名前 100；点击总数包含全部网格。
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

## 页面聚合标识和自动底图（第二阶段）

### 配置与查询

站点设置新增独立的「热力图设置」菜单。设置页默认只展示已保存的热力图列表，尚无点击数据也会显示。点击「新增热力图」进入独立表单，点击名称或「详情」进入单条信息页，可编辑或删除该条配置。表单保存后跳到对应详情，取消则不写入；列表、详情、新增、编辑状态保存在 URL，刷新和返回可恢复相应视图。采样率、保留周期、总开关及安装代码位于单独的「采集设置」入口。新增或编辑会同步维护采集白名单，修改聚合规则保留原先的匹配顺序。删除配置不删除历史数据，移除最后一条白名单路径时关闭总采集开关。列表可直接查看对应热力图，统计页域名下拉不再放置热力图入口。热力查看页支持「具体 URL / 聚合标识」两种方式，并有设置入口。配置示例：

```json
{
  "enabled": true,
  "paths": ["/soft/*.html"],
  "groups": [
    {"path": "/soft/*.html", "key": "software-detail", "example_url": "https://example.com/soft/example.html"}
  ],
  "sample_rate": 0.1,
  "max_clicks": 100,
  "raw_days": 7,
  "aggregate_days": 90,
  "timezone": "Asia/Shanghai"
}
```

`*` 匹配单层路径，不跨 `/`。按顺序首条聚合规则优先于脚本 `data-heatmap-key`；规则不会扩大 `paths` 采集白名单。没有命中规则时使用脚本声明的标识，未声明则为空。服务端最终确定标识。标识仅使用模板名称，不含用户 ID 或个人信息。规则最多 50 条，标识最长 120 字符。

```html
<script defer src="https://stats.example.com/js/script.js" data-domain="example.com"
  data-heatmap="true" data-heatmap-key="software-detail"
  data-heatmap-version="release-1" data-heatmap-variant="default"></script>
```

仍保留具体路径。`GET /api/sites/:domain/heatmap/pages` 和 `/data` 可携带 `scope=key`；默认 `scope=url`。按标识查询的 Page `path` 为虚拟 `/`，实际 SQL 按 `aggregation_key` 查询。列表跳转可通过 `key` 或 `path` 筛选指定标识 / 路径规则（单层 `*`），数据库在 200 条布局限制前先执行筛选，避免目标被其他页面挤出。页面列表上限 200，需留意截断提示。快照保存真实来源路径，不保存虚拟路径。

跨 URL 合并相同 **主机、标识、版本、变体、设备组** 的数据。不同文档宽高按百分比投影，允许大致区域偏移，操作者应选择同模板并及时更新版本。没有元素位置重投影，也不跨 hostname 聚合。元素榜与未映射点击沿用现有语义。

### 数据迁移与兼容性

原始表、小时聚合表新增 `aggregation_key LowCardinality(String)`，在一次 ALTER 中把该列追加到各自 ORDER BY，随后 `ALTER TABLE heatmap_clicks_mv MODIFY QUERY` 更新物化视图。迁移是元数据操作，不新建镜像聚合表，不扫描重写 90 天数据。可重复执行，原有行的标识为空；重试与 Replay 继续合并 UUID 精确集合。

旧数据不会按新规则自动回填。旧 API 实例仍可写入，标识为空；新 Tracker 通过公开配置中的 `groups` 判断服务端能力，面对旧 API 时省略新增字段，避免严格 JSON 校验返回 400。完成所有实例和 Tracker 更新后，新点击按新规则归类。规则变化不重新解释历史标识。配置及快照 PostgreSQL 字段通过 ent schema 生成和迁移维护，`groups` 使用数据库默认 `[]` 兼容已有配置行。

### 自动截图

`POST /api/sites/:domain/heatmap/capture`：

```json
{
  "url": "https://example.com/soft/example.html",
  "scope": "key",
  "page": {
    "aggregation_key": "software-detail", "hostname": "example.com", "path": "/",
    "version": "release-1", "variant": "default",
    "viewport_width": 1440, "viewport_height": 900,
    "document_width": 1440, "document_height": 2400
  }
}
```

主账号的站点 owner/admin 可生成，普通成员和子账号只读。具体 URL 模式默认填写页面地址；标识模式预填规则的范例 URL，也可手动输入。来源必须属于所选主机与本站，HTTP(S)，不含用户密码和查询参数。浏览器报告的实际路径、版本、变体及设备组必须匹配，文档尺寸可以不同，重定向不能更换主机。只有无 Tracker 的 `unknown` 版本可匹配 `unknown`，不会伪造历史版本。

独立 Node/Playwright 进程生成全页 PNG；API 不启动 Chromium。截图任务单并发，超载返回 429；浏览器任务 30 秒截止，API 调用 40 秒截止。PNG 最多 5MiB / 25MP，继续复用既有快照存储、成员权限、过期清理。快照记录实际来源 URL。页面高度或同设备内窗口尺寸变化不再拒绝；设备组、版本或变体不一致仍明确报错。可在具体 URL 模式手动上传历史底图，匹配标识的单页底图也可用于标识视图。

截图使用独立无登录 cookie 的 context，在脚本执行前设置 `zenstats_ignore`，屏蔽本服务的事件/热力点击接口，避免自访问污染。仅适合公开页面；登录页、验证码和依赖交互的页面请手工上传。截图包含页面可见内容，底图接口仍须鉴权。

### 浏览器服务部署

在 `zenstats-web/screenshot` 中：

```sh
npm ci
npx playwright install chromium
SCREENSHOT_TOKEN='<random token at least 32 chars>' npm start
```

API 环境变量：

```sh
ZENSTATS_HEATMAP_SCREENSHOT_URL=http://heatmap-screenshot:8090/capture
ZENSTATS_HEATMAP_SCREENSHOT_TOKEN=<same internal token>
```

令牌仅在内部服务间使用，不进入 Tracker 或管理面板响应。未配置服务时，查看页提示部署方式并保留手动上传。截图服务不应发布到公网端口。

Worker 强制 Chromium 经本地 HTTP/CONNECT 代理访问网络；代理校验 DNS 全部结果，拒绝私有/保留 IP、loopback、link-local、metadata、IPv4-mapped IPv6 等，并直接连接校验后的 IP，避免 DNS 重绑定。此边界覆盖重定向及子资源；浏览器另限制跨主机主框架导航、Service Worker、弹窗、WebSocket、QUIC、WebRTC API 及非代理 UDP。默认仅允许目的端口 80/443。生产不配置私网例外。

仅管理员控制的本地开发可用 `SCREENSHOT_ALLOW_PRIVATE_HOSTS=127.0.0.1,localhost` 和 `SCREENSHOT_ALLOW_PORTS=5173` 放行指定目的主机/端口。没有网页参数可改变这两个限制。Compose/K8s 的可选部署及独立内存/CPU 限制见 `zenstats-deploy/docs/heatmap-screenshot.md`。

## 区域热力图升级

目的为观察大致点击区域，避免真实窗口尺寸造成布局选项和小时聚合高基数。

- 采集协议保持 v1，Tracker 无需更新，原始表继续保留实际坐标与尺寸，PV/UV/Session 链路不变。
- 原始表增加 `grid_version UInt8`（隐式默认0），新 API 写1；小时表同样加列并追加到 ORDER BY。默认0兼容既有数据和旧实例写入。迁移不执行历史回填、不新建镜像表，可以重复运行。
- 物化视图与 Replay 共用投影 SQL：版本0保持原16px网格和实际尺寸；版本1写40×80网格索引，并把四个尺寸列归为三组标准视口占位值。占位值不代表实际文档大小，只有原始点击和快照中的尺寸是实际值。
- 查询时把历史16px网格中心转换为比例网格，并合并同设备的所有尺寸。保留 UUID 的 `uniqExactState/uniqExactMerge` 去重，包括同一点击跨新旧API重试；不会把两个网格状态简单相加。Replay保留每条原始记录的网格版本，重复补算不增加计数。
- `/pages` 返回设备标准 Page 尺寸；`/data` 返回 `coordinate_space="normalized"`、`grid_columns=40`、`grid_rows=80`，point.x/y为0起的网格索引（0～39、0～79），`grid_size=0`。Canvas使用 `(x+0.5)/40`、`(y+0.5)/80` 投影到所选底图。
- 新界面可兼容旧CSS坐标响应，但新API响应需要新版界面解释。发布时先迁移并更新API，紧接更新管理界面，避免长期混用新旧API/界面。旧API对新小时数据的查询口径不兼容；回退到旧应用时须关闭热力图查看和采集，保留表和文件，不能仅回退旧查询代码后继续查看新数据。
- 旧历史聚合仍保留已有尺寸基数，随原TTL到期；新 API 写入从源头收敛到三组。按路径、元素、小时、采样率等原有维度仍保留，UUID状态仍随点击数增长，不能把3200个网格理解为整个表最多800行。

验收包含：旧版本真实表和数据升级；8种不同窗口合并为3组；归一化网格和边缘点击；跨版本重试和重复补算不重计；同设备不同尺寸底图兼容；不同版本/设备底图拒用；标准视口截图与当前文档长度不同仍成功。
