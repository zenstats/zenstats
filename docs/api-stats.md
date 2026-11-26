# ZenStats Statistics & Query API

> **Base URL**: `/api`  
> **Swagger UI** (local): `http://localhost:8080/swagger/index.html`

---

## Authentication

All stats endpoints require authentication and site access verification.

| Method | Header | Scope |
|---|---|---|
| **JWT Token** | `Authorization: Bearer <jwt_token>` | All user-accessible endpoints (`stats`, `v2/query`, `segments`, `heatmap`, `performance`) |
| **API Key** | `Authorization: Bearer <api_key>` or `X-API-Key: <api_key>` | Read-only stats endpoints (`/api/stats/:domain/*` and `POST /api/v2/query/:domain`) |

---

## Response Format

All JSON API endpoints return a unified response envelope:

```json
{
  "code": 200,
  "message": "success",
  "data": { ... }
}
```

Error responses include `code`, `message`, and optional `error` details:

```json
{
  "code": 400,
  "message": "Invalid query parameter",
  "error": "..."
}
```

---

## Common Time & Query Parameters

### `period` Values

`realtime` · `day` · `7d` · `28d` · `30d` · `91d` · `month` · `6mo` · `12mo` · `year` · `all` · `custom`

- `date`: Anchor date in `YYYY-MM-DD` (defaults to today in the site's timezone)
- `from` / `to`: Required when `period=custom` (ISO date `YYYY-MM-DD`)
- `filters`: URL-encoded JSON filter expression (see [Filter Syntax](#filter-syntax))
- `compare`: Period comparison (`previous_period`, `year_over_year`, or `custom` with `compare_from` & `compare_to`)

---

## Statistics API v1 (`/api/stats/:domain`)

> Accepts **JWT** or **API Key**

### 1. Aggregate Summary

```http
GET /api/stats/:domain/aggregate
```

Query params:
- `period`, `date`, `from`, `to`
- `metrics` (comma-separated, e.g. `visitors,pageviews,bounce_rate,visit_duration,views_per_visit`)
- `filters`
- `compare`, `compare_from`, `compare_to`

**Example:**

```bash
curl -H "Authorization: Bearer zs_live_xxx" \
  "https://analytics.example.com/api/stats/example.com/aggregate?period=30d&metrics=visitors,pageviews,bounce_rate,visit_duration,views_per_visit&compare=previous_period"
```

---

### 2. Main Graph (Time Series)

```http
GET /api/stats/:domain/main-graph
GET /api/stats/:domain/time_series
```

(`/time_series` is a backwards-compatible alias of `/main-graph`.)

Query params:
- `period`, `date`, `from`, `to`
- `metric` (e.g. `visitors`, `pageviews`, `visits`, `bounce_rate`, `visit_duration`) or `metrics`
- `interval` (`minute`, `hour`, `day`, `week`, `month`)
- `filters`
- `compare`, `compare_from`, `compare_to`
- `with_imported` (`true` / `false`)

---

### 3. Dimensional Breakdown

```http
GET /api/stats/:domain/breakdown
```

Query params:
- `property` (required) — dimension key (e.g. `visit:source`, `event:page`, `visit:country`, `visit:browser`, `event:name`)
- `period`, `date`, `from`, `to`
- `metrics` (comma-separated)
- `filters`
- `limit` (integer) & `page` (integer)
- `search` (search filter on breakdown values)
- `order_by`

**Example:**

```bash
curl -H "Authorization: Bearer zs_live_xxx" \
  "https://analytics.example.com/api/stats/example.com/breakdown?property=event:page&period=7d&metrics=visitors,pageviews,bounce_rate,time_on_page&limit=20&page=1"
```

---

### 4. CSV Export

```http
GET /api/stats/:domain/export
```

Streams a CSV report (`text/csv`) for a site and period. Accepts `period`, `date`, `from`, `to`, `metrics`, `property`, `filters`, `limit`, `with_imported`.

---

### 5. Current Visitors (Realtime)

```http
GET /api/stats/:domain/current-visitors
```

Returns active visitors/sessions in the live realtime window:

```json
{
  "code": 200,
  "message": "success",
  "data": {
    "total": 12,
    "visitors": 12,
    "sessions": 14,
    "last_updated": "2026-01-15T08:30:00Z"
  }
}
```

---

### 6. Filter Autocomplete Suggestions

```http
GET /api/stats/:domain/suggestions
```

Query params:
- `property` (required) — dimension key
- `q` — search prefix
- `period`, `date`, `from`, `to`, `limit`

---

### 7. User Flows (Sankey Path Graph)

```http
GET /api/stats/:domain/user-flows
```

Query params:
- `period`, `date`, `from`, `to`
- `entry_path` (optional starting path filter)
- `filters` (JSON filter string)
- `steps` (integer `2`..`4`, default `3`)
- `limit` (integer `3`..`12`, top nodes per step)

**Response `data`:**

```json
{
  "nodes": [
    { "id": "step_0_/", "step": 0, "name": "/", "sessions": 1240 }
  ],
  "links": [
    { "source": "step_0_/", "target": "step_1_/pricing", "value": 420 }
  ],
  "total_sessions": 1240,
  "drop_off_sessions": 310,
  "drop_off_rate": 25.0
}
```

---

## Flexible Query API v2 (`/api/v2/query`)

### Multi-Dimensional Query

```http
POST /api/v2/query/:domain
```

Accepts **JWT** or **API Key**.

**Request body (`QueryV2Request`):**

| Field | Type | Description |
|---|---|---|
| `period` | `string` | Preset period (`7d`, `30d`, `day`, `custom`, etc.) |
| `date_range` | `[string, string]` \| `string` | ISO date/timestamp range tuple or preset string |
| `from` / `to` | `string` | Explicit start / end bounds (`YYYY-MM-DD` or RFC3339) |
| `date` | `string` | Reference date (`YYYY-MM-DD`) |
| `metrics` | `string[]` | **Required.** Metric list (at least 1) |
| `dimensions` | `string[]` | Breakdown / time-bucket dimensions (e.g. `["time:day", "visit:source"]`) |
| `filters` | `any[]` | Recursive filter tree (see [Filter Syntax](#filter-syntax)) |
| `order_by` | `[string, "asc" \| "desc"][]` | Multi-column sort rules |
| `limit` / `offset` | `number` | Pagination (`limit` / `offset`) |
| `sample_threshold` | `number` | Optional query sampling threshold |
| `compare` | `string` | Period comparison mode (`previous_period`, `year_over_year`, `custom`) |
| `compare_from` / `compare_to` | `string` | Bounds when `compare=custom` |

**Example:**

```bash
curl -X POST "https://analytics.example.com/api/v2/query/example.com" \
  -H "Authorization: Bearer zs_live_xxx" \
  -H "Content-Type: application/json" \
  -d '{
    "period": "30d",
    "metrics": ["visitors", "pageviews", "bounce_rate", "visit_duration"],
    "dimensions": ["visit:country", "visit:device"],
    "filters": [["is", "visit:source", ["Google"]]],
    "order_by": [["visitors", "desc"]],
    "limit": 20,
    "offset": 0
  }'
```

### Segment Query Preview

```http
POST /api/v2/query/:domain/segments
```

Requires **JWT Auth** (`segments:write` site permission). Evaluates audience segment filter definitions on the target site.

---

## Site Heatmap & Core Web Vitals Endpoints

> Requires **JWT Auth** + Site membership (not accessible via public stats API keys).

- **Click & Scroll Heatmaps** (`/api/sites/:domain/heatmap`):
  - `GET /config`, `PUT /config`
  - `GET /pages`, `GET /data`
  - `GET /scroll/pages`, `GET /scroll/data`
  - `GET /snapshots`, `POST /capture` (proxies to screenshot worker), `GET /replay`
- **Core Web Vitals** (`/api/sites/:domain/performance`):
  - `GET /config`, `PUT /config`
  - `GET /data` (returns `LCP`, `INP`, `CLS`, `FCP`, `TTFB` percentiles and distributions)

---

## Metrics & Dimensions Reference

### Metrics

| Metric | Description |
|---|---|
| `visitors` | Unique visitors (daily cryptographic hash) |
| `pageviews` | Total pageview events |
| `visits` | Total sessions / visits |
| `bounce_rate` | Percentage of single-page visits (%) |
| `visit_duration` | Average session duration (seconds) |
| `time_on_page` | Average time spent on page (seconds) |
| `scroll_depth` | Average page scroll depth (%) |
| `exit_rate` | Percentage of visits exiting on the page (%) |
| `views_per_visit` | Average pageviews per session |
| `events` | Total event count (custom events / goals) |
| `conversions` | Goal conversion count |
| `conversion_rate` | Percentage of visitors who converted (%) |
| `percentage` | Share of total metric (%) |
| `total_revenue` | Total goal conversion revenue |
| `average_revenue` | Average revenue per conversion |

### Dimensions

| Group | Dimensions |
|---|---|
| **Time** | `time`, `time:hour`, `time:day`, `time:week`, `time:month` |
| **Traffic & UTM** | `visit:source`, `visit:channel`, `visit:referrer`, `visit:utm_source`, `visit:utm_medium`, `visit:utm_campaign`, `visit:utm_content`, `visit:utm_term` |
| **Pages** | `event:page`, `event:hostname`, `visit:entry_page`, `visit:entry_page_hostname`, `visit:exit_page`, `visit:exit_page_hostname` |
| **Geography** | `visit:country`, `visit:country_name`, `visit:region`, `visit:city`, `visit:city_name` |
| **Client & Tech** | `visit:browser`, `visit:browser_version`, `visit:os`, `visit:os_version`, `visit:device`, `visit:screen_size` |
| **Events & Props** | `event:name`, `event:goal`, `event:props:<key>`, `visit:entry_props:<key>` |

---

## Filter Syntax

Filters use prefix Polish-notation arrays that can be nested arbitrarily:

### Leaf Filters

`[operator, dimension, values]` (with optional 4th modifier object `{"case_sensitive": false}`):

- `is` / `is_not`
- `contains` / `not_contains` (`contains_not` alias)
- `matches` / `matches_not` (regular expression)
- `matches_wildcard` / `matches_wildcard_not` (glob pattern, e.g. `/blog/*`)

### Logical & Behavioral Combinators

- `["and", filter1, filter2, ...]`
- `["or", filter1, filter2, ...]`
- `["not", filter]`
- `["has_done", inner_filter]` / `["has_not_done", inner_filter]` (session/visitor behavioral filters)

**Example:**

```json
[
  "and",
  ["is", "visit:country", ["US", "DE"]],
  [
    "or",
    ["matches_wildcard", "event:page", ["/docs/*"]],
    ["is", "event:goal", ["Signup"]]
  ]
]
```
