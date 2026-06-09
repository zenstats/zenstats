# ZenStats 数据统计 API 文档

## 概述

**Base URL**: `/api/stats`

**认证方式**: `Authorization: Bearer <token>`

---

## 通用请求参数

所有统计接口共享以下查询参数：

| 参数       | 类型     | 必填 | 说明                                                         |
|-----------|----------|------|--------------------------------------------------------------|
| `period`  | string   | ✅   | 时间周期：`realtime` / `day` / `p7` / `p14` / `p30` / `custom` |
| `date`    | string   | 否   | 日期，格式 `YYYY-MM-DD`（`realtime` 和 `custom` 时不需要）    |
| `from`    | string   | 否   | 自定义起始日期（仅 `period=custom` 时必填）                   |
| `to`      | string   | 否   | 自定义结束日期（仅 `period=custom` 时必填）                   |
| `interval`| string   | 否   | 时间间隔：`minute` / `hourly` / `daily` / `weekly` / `monthly` |
| `filters` | string   | 否   | 过滤条件，JSON 数组格式（见「过滤条件」章节）                 |
| `limit`   | int      | 否   | 分页每页条数                                                  |
| `page`    | int      | 否   | 页码，从 1 开始                                               |

### 支持的指标 (Metrics)

| 指标名            | 说明              | 适用表     |
|-------------------|-------------------|-----------|
| `visitors`        | 独立访客数        | events    |
| `pageviews`       | 页面浏览量        | sessions  |
| `bounce_rate`     | 跳出率 (%)        | sessions  |
| `visit_duration`  | 平均访问时长 (秒)  | sessions  |
| `views_per_visit` | 每次访问页面数     | sessions  |
| `events`          | 事件总数          | sessions  |

### 支持的维度 (Dimensions / Property)

| 维度名                    | 说明            |
|--------------------------|-----------------|
| `visit:source`           | 流量来源        |
| `visit:country`          | 国家            |
| `visit:region`           | 地区            |
| `visit:city`             | 城市            |
| `visit:browser`          | 浏览器          |
| `visit:browser_version`  | 浏览器版本      |
| `visit:os`               | 操作系统        |
| `visit:os_version`       | 操作系统版本    |
| `visit:device`           | 设备类型        |
| `visit:entry_page`       | 入口页面        |
| `visit:exit_page`        | 退出页面        |
| `event:page`             | 页面路径        |
| `event:name`             | 事件名称        |
| `event:hostname`         | 主机名          |
| `event:browser`          | 浏览器          |
| `event:os`               | 操作系统        |
| `event:device`           | 设备类型        |
| `event:country`          | 国家            |
| `event:props:<key>`      | 自定义事件属性  |

### 过滤条件 (Filters)

过滤条件以 JSON 数组格式传递：

```
# 简单过滤
[["is", "visit:country", ["US", "CN"]]]

# AND 组合
["and", [["is", "visit:country", ["US"]], ["is", "visit:browser", ["Chrome"]]]]

# OR 组合
["or", [["is", "visit:source", ["Google"]], ["is", "visit:source", ["Baidu"]]]]
```

支持的操作符：`is` / `is_not` / `contains` / `contains_not` / `matches` / `matches_not`

---

## API 端点

### 1. 总览指标 (Aggregate)

获取聚合统计数据。

**接口**: `GET /stats/:domain/aggregate`

**额外参数**:

| 参数       | 类型   | 说明                          | 默认值                                  |
|-----------|--------|-------------------------------|-----------------------------------------|
| `metrics` | string | 逗号分隔的指标列表             | `visitors,pageviews,bounce_rate,visit_duration,views_per_visit` |

**示例请求**:
```
GET /api/stats/example.com/aggregate?period=p30&date=2024-01-31&metrics=visitors,pageviews
```

**响应示例**（含对比数据，自动与上一同期对比）:
```json
{
  "success": true,
  "data": {
    "results": {
      "visitors": {
        "value": 12345,
        "comparison_value": 10000,
        "change": 23.45
      },
      "pageviews": {
        "value": 45678,
        "comparison_value": 40000,
        "change": 14.20
      },
      "bounce_rate": {
        "value": 35.67,
        "comparison_value": 40.12,
        "change": -11.09
      },
      "visit_duration": {
        "value": 185.42,
        "comparison_value": 170.50,
        "change": 8.75
      },
      "views_per_visit": {
        "value": 3.70,
        "comparison_value": 4.00,
        "change": -7.50
      }
    }
  }
}
```

**对比逻辑说明**:
- `day` → 对比前一天
- `p7` → 对比前 7 天
- `p14` → 对比前 14 天
- `p30` → 对比前 30 天
- `custom` → 对比相同天数的上一周期
- `change` = (当前值 - 对比值) / 对比值 × 100，保留两位小数
- 当对比值为 0 时，`change` 为 null

---

### 2. 主图表 (Main Graph / Time Series)

获取时序图表数据。

**接口**: `GET /stats/:domain/main-graph`

**额外参数**:

| 参数       | 类型   | 说明                          | 默认值                |
|-----------|--------|-------------------------------|-----------------------|
| `metrics` | string | 逗号分隔的指标列表             | `visitors,pageviews`  |

**示例请求**:
```
GET /api/stats/example.com/main-graph?period=p7&date=2024-01-31&interval=daily&metrics=visitors,pageviews
```

**响应示例**:
```json
{
  "success": true,
  "data": [
    {
      "timestamp": "2024-01-25",
      "metrics": {
        "visitors": 1500,
        "pageviews": 3200
      }
    },
    {
      "timestamp": "2024-01-26",
      "metrics": {
        "visitors": 1800,
        "pageviews": 4100
      }
    }
  ]
}
```

**响应说明**:
- 时间戳格式随 `interval` 变化：
  - `minute` → `2024-01-25 14:30`
  - `hourly` → `2024-01-25 14`
  - `daily` → `2024-01-25`
  - `weekly` → `2024-01-22`
  - `monthly` → `2024-01`
- 自动填补缺失时间点（值为 0）

---

### 3. 维度细分 (Breakdown)

按维度获取排行数据。

**接口**: `GET /stats/:domain/breakdown`

**额外参数**:

| 参数       | 类型   | 必填 | 说明                          | 默认值    |
|-----------|--------|------|-------------------------------|-----------|
| `property`| string | ✅   | 细分维度（见上表）             | -         |
| `metrics` | string | 否   | 逗号分隔的指标列表             | `visitors`|

**示例请求**:

```bash
# 来源排行
GET /api/stats/example.com/breakdown?period=p30&date=2024-01-31&property=visit:source&metrics=visitors&pageviews

# 国家排行
GET /api/stats/example.com/breakdown?period=p30&date=2024-01-31&property=visit:country

# 页面排行
GET /api/stats/example.com/breakdown?period=p30&date=2024-01-31&property=event:page&metrics=visitors,pageviews

# 浏览器排行
GET /api/stats/example.com/breakdown?period=p30&date=2024-01-31&property=visit:browser

# 入口页面
GET /api/stats/example.com/breakdown?period=p30&date=2024-01-31&property=visit:entry_page
```

**响应示例**:
```json
{
  "success": true,
  "data": {
    "columns": ["referrer_source", "visitors", "pageviews"],
    "data": [
      {
        "referrer_source": "Google",
        "visitors": 5000,
        "pageviews": 12000
      },
      {
        "referrer_source": "Direct / None",
        "visitors": 3000,
        "pageviews": 8000
      }
    ]
  }
}
```

---

### 4. 实时访客 (Current Visitors)

获取当前在线访客数。

**接口**: `GET /stats/:domain/current-visitors`

**无需额外参数**。默认查询最近 5 分钟内的数据。

**示例请求**:
```
GET /api/stats/example.com/current-visitors
```

**响应示例**:
```json
{
  "success": true,
  "data": {
    "total": 42,
    "visitors": 42,
    "sessions": 58,
    "last_updated": "2024-01-31T12:00:00Z"
  }
}
```

---

### 5. 来源排行 (Source Rank)

获取流量来源排行（旧版兼容）。

**接口**: `GET /stats/:domain/source_rank` 或 `GET /stats/:domain/sources`

**示例请求**:
```
GET /api/stats/example.com/source_rank?period=p30&date=2024-01-31
```

**响应示例**:
```json
{
  "success": true,
  "data": [
    {
      "key": "Google",
      "visits": 5000,
      "percentage": 45.23
    },
    {
      "key": "Direct",
      "visits": 3000,
      "percentage": 27.14
    }
  ]
}
```

---

### 6. 设备排行 (Device Rank)

获取设备类型排行（旧版兼容）。

**接口**: `GET /stats/:domain/device_rank`

**示例请求**:
```
GET /api/stats/example.com/device_rank?period=p30&date=2024-01-31
```

**响应示例**:
```json
{
  "success": true,
  "data": [
    {
      "key": "Desktop",
      "visits": 6000,
      "percentage": 54.32
    },
    {
      "key": "Mobile",
      "visits": 4500,
      "percentage": 40.72
    }
  ]
}
```

---

### 7. 页面排行 (Page Rank)

获取热门页面排行（旧版兼容）。

**接口**: `GET /stats/:domain/page_rank`

**示例请求**:
```
GET /api/stats/example.com/page_rank?period=p30&date=2024-01-31
```

**响应示例**:
```json
{
  "success": true,
  "data": [
    {
      "key": "/blog/post-1",
      "visits": 2000,
      "percentage": 18.10
    }
  ]
}
```

---

### 8. 顶部概览 (Top Stats)

获取 PV/UV/Sessions/平均时长/跳出率等核心指标及环比数据（旧版兼容）。

**接口**: `GET /stats/:domain/top_stats`

**示例请求**:
```
GET /api/stats/example.com/top_stats?period=p7&date=2024-01-31
```

**响应示例**:
```json
{
  "success": true,
  "data": {
    "pv": 45678,
    "uv": 12345,
    "sessions": 15678,
    "avg_duration": 185.42,
    "prev_pv": 40000,
    "prev_uv": 10000,
    "prev_sessions": 13000,
    "prev_avg_duration": 170.50,
    "pv_change": 14.20,
    "uv_change": 23.45,
    "session_change": 20.60,
    "avg_duration_change": 8.75,
    "avg_duration_format": "3M 5.42S",
    "bounce_rate": 35.67
  }
}
```

---

### 9. 曲线图 (Curve)

获取按时间维度的 UV 曲线图数据（旧版兼容）。

**接口**: `GET /stats/:domain/curve`

**示例请求**:
```
GET /api/stats/example.com/curve?period=p7&date=2024-01-31&interval=1%20DAY
```

**响应示例**:
```json
{
  "success": true,
  "data": [
    { "time": "2024-01-25", "uv": 1500 },
    { "time": "2024-01-26", "uv": 1800 },
    { "time": "2024-01-27", "uv": 1600 }
  ]
}
```

---

## 高级查询示例

### 自定义事件属性查询

当追踪脚本通过 `zenstats('event', { props: { author: '张三', category: '科技' }})` 发送自定义事件时，可以使用 `event:props:<key>` 维度进行查询。

**按自定义属性分组**:
```bash
# 按文章作者统计访客数
GET /api/stats/example.com/breakdown?period=p30&date=2024-01-31&property=event:props:author&metrics=visitors,events

# 按文章分类统计
GET /api/stats/example.com/breakdown?period=p30&date=2024-01-31&property=event:props:category&metrics=visitors
```

**过滤自定义属性**:
```bash
# 只统计 category 为 "科技" 的页面浏览
GET /api/stats/example.com/breakdown?period=p30&date=2024-01-31&property=event:page&metrics=visitors,pageviews&filters=[["is","event:props:category",["科技"]]]

# 排除特定作者的事件
GET /api/stats/example.com/breakdown?period=p30&date=2024-01-31&property=event:name&metrics=visitors,events&filters=[["is_not","event:props:author",["测试用户"]]]
```

**入口页面自定义属性**:
```bash
# 按入口页面的自定义属性分组
GET /api/stats/example.com/breakdown?period=p30&date=2024-01-31&property=visit:entry_props:variant&metrics=visitors
```

### 自定义事件 UV/PV 统计

```bash
# 统计所有自定义事件（非 pageview 和 engagement）的数量
GET /api/stats/example.com/aggregate?period=p30&date=2024-01-31&metrics=events

# 按事件名称查看各事件的 UV 和触发次数
GET /api/stats/example.com/breakdown?period=p30&date=2024-01-31&property=event:name&metrics=visitors,events

# 过滤特定事件名称
GET /api/stats/example.com/aggregate?period=p30&date=2024-01-31&metrics=visitors,events&filters=[["is","event:name",["purchase","signup"]]]
```

### 网站页面流量查询

```bash
# 热门页面排行（按独立访客排序）
GET /api/stats/example.com/breakdown?period=p30&date=2024-01-31&property=event:page&metrics=visitors,pageviews&limit=20

# 按主机名查看流量分布
GET /api/stats/example.com/breakdown?period=p30&date=2024-01-31&property=event:hostname&metrics=visitors,pageviews

# 入口页面排行
GET /api/stats/example.com/breakdown?period=p30&date=2024-01-31&property=visit:entry_page&metrics=visitors,visit_duration,bounce_rate&limit=20

# 退出页面排行
GET /api/stats/example.com/breakdown?period=p30&date=2024-01-31&property=visit:exit_page&metrics=visitors&pageviews&limit=20

# 特定页面的流量时序趋势
GET /api/stats/example.com/main-graph?period=p30&date=2024-01-31&interval=daily&metrics=visitors,pageviews&filters=[["is","event:page",["/blog/my-post"]]]

# 页面流量 + 来源交叉分析
GET /api/stats/example.com/breakdown?period=p30&date=2024-01-31&property=visit:source&metrics=visitors&pageviews&filters=[["is","event:page",["/pricing"]]]
```

### 多条件组合查询

```bash
# 美国 Chrome 浏览器用户的页面访问排行
GET /api/stats/example.com/breakdown?period=p30&date=2024-01-31&property=event:page&metrics=visitors,pageviews&filters=[["and",[["is","visit:country",["US"]],["is","visit:browser",["Chrome"]]]]]

# 来自 Google 或 Baidu 的访客的热门页面
GET /api/stats/example.com/breakdown?period=p30&date=2024-01-31&property=event:page&metrics=visitors&pageviews&filters=[["or",[["is","visit:source",["Google"]],["is","visit:source",["Baidu"]]]]]

# 移动端用户的来源分布
GET /api/stats/example.com/breakdown?period=p30&date=2024-01-31&property=visit:source&metrics=visitors&filters=[["is","visit:device",["Mobile"]]]
```

---

## 错误响应

所有接口在出错时返回统一格式：

```json
{
  "success": false,
  "error": {
    "message": "error description"
  }
}
```

常见 HTTP 状态码：
- `200` - 成功
- `400` - 请求参数错误
- `401` - 未认证
- `403` - 无权限访问该站点
- `500` - 服务器内部错误