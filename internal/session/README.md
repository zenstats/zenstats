# Session 聚合模块

## 职责

接收 event 包处理后的 Events 对象，进行 Session 聚合与去重。

## 处理流程

1. **OnEvent(event)** — 通过 Balancer 按 user_id 分发到对应 shard
2. **findSession(event)** — 从 LRU 缓存（30min TTL）查找活跃 session
3. **handleEvent(event, foundSession)**:
   - **engagement 事件**: 仅更新 session（duration/timestamp/events），不创建新 session
   - **pageview/自定义事件**:
     - 缓存命中（且 30min 内活跃）→ update：写 sign=-1（旧） + sign=+1（新）
     - 缓存未命中 → 从 ClickHouse 加载最近的活跃 session
     - 无活跃 session → new：创建 sign=+1 的新 session
   - **is_bounce 判断**: 使用更新后的 pageviews 值（非旧值），当 pageviews>1 或非 pageview 的交互事件到达时设为 0

## 幂等写入

Session 表使用 sign 列实现版本化：
- sign=+1 表示生效记录
- sign=-1 表示废弃记录
- 查询时取 sign 之和最大的版本

## 缓存

- 使用 `hashicorp/golang-lru/v2/expirable` LRU 缓存
- 容量：1000 个 session
- TTL：30 分钟
- 每次事件到达刷新缓存中的 timestamp

## 涉及文件

- `session.go` — 核心逻辑（OnEvent / handleEvent / newSession / updateSession）
- `balancer.go` — 按 user_id 分片 + 互斥锁保障同用户串行处理
- `buffer.go` — 异步批写 ClickHouse（5s 间隔或满 batch 触发）
- `cache/` — 缓存接口定义
