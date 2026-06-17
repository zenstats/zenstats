# ZenStats 追踪脚本

ZenStats 追踪脚本是一个轻量级（根据所选功能 4–6KB）JavaScript 代码片段，可追踪页面浏览、停留时间、滚动深度、自定义事件、站外链接、文件下载和表单提交。支持 React、Vue、Angular 和纯 HTML 等所有框架。

## 目录

- [安装](#安装)
- [脚本版本（功能选择）](#脚本版本功能选择)
- [脚本标签属性](#脚本标签属性)
- [自动追踪](#自动追踪)
- [自定义事件（Tagged Events）](#自定义事件tagged-events)
- [JavaScript 手动追踪](#javascript-手动追踪)
- [SPA 路由](#spa-路由)
- [目标与转化](#目标与转化)
- [漏斗分析](#漏斗分析)
- [隐私与数据](#隐私与数据)
- [集成指南](#集成指南)
- [常见问题](#常见问题)
- [API 参考](#api-参考)

---

## 安装

### 快速开始

1. 在 ZenStats 后台进入站点的 **安装统计代码** 页面
2. 勾选需要的功能
3. 复制生成的 `<script>` 代码片段
4. 粘贴到每个页面的 `<head>` 中：

```html
<script defer crossorigin="anonymous" data-domain="yourdomain.com" src="https://your-zenstats-domain.com/js/script.js"></script>
```

将 `yourdomain.com` 替换为在 ZenStats 中注册的域名，`your-zenstats-domain.com` 替换为 ZenStats 实例的域名。

---

## 脚本版本（功能选择）

ZenStats 根据你选择的功能编译优化的脚本变体。与传统分析脚本打包所有功能不同，ZenStats 只包含你实际需要的代码——保持网站加载速度。

在 **安装统计代码** 页面勾选功能复选框即可。代码片段中的脚本 URL 会自动更新为对应的预编译文件。

| 功能 | 勾选框 | 脚本 URL 示例 | 说明 |
|------|--------|-------------|------|
| **自定义事件** | ☑ 自定义事件 | `script.te.js` | 通过 CSS 类名（`event-name=...`）追踪点击和表单 |
| **站外链接** | ☑ 站外链接 | `script.ol.js` | 自动追踪指向外部域名的链接点击 |
| **文件下载** | ☑ 文件下载 | `script.fd.js` | 自动追踪文档/媒体/压缩包链接点击 |
| **Hash 路由** | ☑ Hash 路由模式 | `script.ha.js` | 使用 `hashchange` 而非 History API 追踪 SPA 导航 |
| **页面排除规则** | ☑ 页面排除规则 | `script.ex.js` | 按路径模式包含/排除页面追踪 |
| **手动 Pageview URL** | ☑ 手动 Pageview URL | `script.ma.js` | 通过 `options.u` / `options.url` 覆盖追踪的 URL |

### 脚本文件命名

```
script.js                            ← 全部功能（向后兼容别名）
script.{ex}.{fd}.{ha}.{ma}.{ol}.{te}.js   ← 任意组合，按字母排序
```

**缩写对照：** `ex` = 页面排除, `fd` = 文件下载, `ha` = Hash 路由, `ma` = 手动 URL, `ol` = 站外链接, `te` = 自定义事件。

### 文件体积

| 变体 | 典型大小 |
|------|---------|
| 裸版（仅页面浏览） | ~4.1 KB |
| 单一功能 | ~4.5–5.2 KB |
| 全部 6 个功能 | ~6.0 KB |

---

## 脚本标签属性

| 属性 | 必填 | 对应功能 | 说明 |
|------|------|---------|------|
| `data-domain` | 是 | — | 在 ZenStats 中注册的域名 |
| `data-api` | 否 | — | 自定义 API 端点（默认：脚本同源的 `/api/event`） |
| `data-include` | 否 | 页面排除规则 | 逗号分隔的路径模式。设置后 **仅** 匹配页面被追踪。支持 `*`（单段）和 `**`（多段）。 |
| `data-exclude` | 否 | 页面排除规则 | 逗号分隔的路径模式。匹配页面 **排除** 追踪。支持 `*` 和 `**`。 |
| `data-file-types` | 否 | 文件下载 | 逗号分隔的文件扩展名（默认：`pdf,xlsx,docx,…`）。 |

> **注意：** 旧文档中的 `data-outbound-links` 和 `data-file-downloads` 属性不再存在——这些功能通过安装页面的勾选框启用，由脚本文件名决定包含哪些代码。

### 页面排除规则示例

```html
<!-- 只追踪 /blog/ 和 /docs/ 子页面 -->
<script defer data-domain="example.com" data-include="/blog/**, /docs/**" src="...script.ex.js"></script>

<!-- 追踪除 /admin/ 和 /internal/ 之外的所有页面 -->
<script defer data-domain="example.com" data-exclude="/admin/**, /internal/**" src="...script.ex.js"></script>

<!-- 通配符：排除所有 .php 页面 -->
<script defer data-domain="example.com" data-exclude="*.php" src="...script.ex.js"></script>
```

排除检查在每个 pageview 事件发生时运行。不匹配的页面会被静默忽略（回调中 `ignored: true`）。

### 自定义文件类型

```html
<script defer data-domain="example.com" data-file-types="pdf,zip,csv" src="...script.fd.js"></script>
```

### 代理设置（推荐）

从你的域名提供脚本，绕过广告拦截器：

**Caddy：**

```
example.com {
    handle /js/* {
        reverse_proxy your-zenstats-domain.com
    }
}
```

**Nginx：**

```nginx
location /js/ {
    proxy_pass https://your-zenstats-domain.com/js/;
    proxy_set_header Host your-zenstats-domain.com;
}
```

---

## 自动追踪

安装脚本后，ZenStats 会自动追踪：

### 页面浏览

- 初始页面加载
- 浏览器前进/后退导航
- SPA 路由变化（History API 或 Hash 路由，取决于脚本版本）

### 用户参与（停留时间与滚动深度）

- **活跃时间**：仅在标签页可见且有焦点时计算
- **滚动深度**：达到的最大滚动百分比
- 参与事件在标签页隐藏或活跃 10 秒后发送

### 站外链接点击

启用 **站外链接** 功能后，指向外部域名的 `<a>` 元素点击会被追踪为 `Outbound Link: Click`，附带 `url` 属性。伪协议（`javascript:`、`mailto:`、`tel:`）自动排除。

### 文件下载

启用 **文件下载** 功能后，结尾为已知文件扩展名的链接点击被追踪为 `File Download`，附带 `url` 属性。

### 表单提交

启用 **自定义事件**、**站外链接** 或 **文件下载** 功能时，`<form>` 提交会被追踪为 `Form: Submission`。如果表单带有 `event-name=…` CSS 类名，则使用该自定义名称。

---

## 自定义事件（Tagged Events）

"自定义事件"功能通过 CSS 类名实现 **零代码事件追踪**。

**格式：** `event-name=事件名称`

```html
<!-- 追踪按钮点击 -->
<button class="event-name=注册">注册</button>

<!-- 追踪链接点击 -->
<a href="/pricing" class="event-name=定价+点击">查看定价</a>

<!-- 追踪表单提交 -->
<form class="event-name=联系+表单">
  <input type="email" />
  <button type="submit">提交</button>
</form>
```

### 通过类名添加附加属性

```html
<button class="event-name=购买 event-plan=Pro event-amount=99">
  购买专业版
</button>
```

发送事件名 `购买`，属性 `{ plan: "Pro", amount: "99" }`。

**规则：**
- 用 `+` 表示空格（`按钮+点击` → "按钮 点击"）
- 在点击元素及其最多 3 层父元素上检查类名
- 对链接，`url` 属性自动设为链接的 `href`

---

## JavaScript 手动追踪

```javascript
// 基础自定义事件
zenstats('Signup')

// 带属性的事件
zenstats('Purchase', {
  props: { plan: 'Business', amount: 99 }
})

// 带回调的事件
zenstats('Download', {
  callback: function(result) {
    console.log('状态:', result.status)
  }
})

// 非交互事件（不影响跳出率）
zenstats('Scroll Depth', { interactive: false })

// 手动覆盖 pageview URL（需要启用 Manual URL 功能）
zenstats('pageview', {
  u: '/virtual-path',
  props: { section: 'docs' }
})
// 或使用 options.url：
zenstats('pageview', { url: '/virtual-path' })
```

---

## SPA 路由

默认情况下，脚本使用 **History API**（PushState / popstate）追踪 SPA 页面浏览。

如果 SPA 使用 **Hash 路由**（`#/path`），在安装页面勾选 **Hash 路由模式**。这会将脚本改为监听 `hashchange` 事件，并在 pageview 负载中包含 `h: 1` 标记。

---

## 目标与转化

在 ZenStats 界面中创建目标来追踪转化：

1. **站点设置** → **转化** → **目标**
2. 点击 **添加目标**
3. 选择类型：
   - **自定义事件** — 匹配事件名称如 `Signup`、`Purchase`
   - **页面浏览** — 匹配 URL 路径如 `/thank-you`
4. 输入显示名称并创建

创建目标后，匹配的事件将显示在仪表板中。

---

## 漏斗分析

追踪多步骤转化流程：

1. **站点设置** → **转化** → **漏斗** → **添加漏斗**
2. 命名漏斗（如 "注册流程"）
3. 选择已有目标，添加 2–8 个步骤
4. 在 **漏斗分析** 中查看结果：每步访客数、流失率、转化率

---

## 隐私与数据

### 机器人过滤

ZenStats 自动忽略：

- 无头浏览器（`_phantom`、`__nightmare`、`webdriver`、`Cypress`）
- `localStorage.zenstats_ignore = 'true'` 的页面

### 忽略自己的访问

```javascript
// 在浏览器控制台中执行
localStorage.setItem('zenstats_ignore', 'true')

// 重新启用
localStorage.removeItem('zenstats_ignore')
```

### 数据收集说明

| 收集 | 不收集 |
|------|--------|
| 页面 URL | 个人信息 |
| 来源（Referrer） | 原始 IP 地址（哈希处理） |
| 浏览器与操作系统 | Cookie |
| 屏幕尺寸 | |
| 国家/城市（基于 IP） | |
| UTM 参数 | |

---

## 集成指南

### React / Next.js

**React Router** — 无需设置，基于 History 的 SPA 追踪自动生效。

**Next.js（App Router）：**

```tsx
// app/layout.tsx
import Script from 'next/script'

export default function RootLayout({ children }) {
  return (
    <html>
      <head>
        <Script defer crossorigin="anonymous"
          data-domain="yourdomain.com"
          src="https://stats.example.com/js/script.js" />
      </head>
      <body>{children}</body>
    </html>
  )
}
```

### Vue / Nuxt

**Vue Router** — History 模式下自动生效。

**Nuxt**（`nuxt.config.ts`）：

```ts
export default defineNuxtConfig({
  app: {
    head: {
      script: [{
        defer: true,
        crossorigin: 'anonymous',
        'data-domain': 'yourdomain.com',
        src: 'https://stats.example.com/js/script.js'
      }]
    }
  }
})
```

### WordPress

在 `header.php` 中添加：

```php
<script defer crossorigin="anonymous" data-domain="yourdomain.com" src="https://stats.example.com/js/script.js"></script>
```

或使用 **Insert Headers and Footers** 插件。

### 纯 HTML

```html
<!DOCTYPE html>
<html>
<head>
  <script defer crossorigin="anonymous" data-domain="yourdomain.com"
          src="https://stats.example.com/js/script.js"></script>
</head>
<body><!-- 你的内容 --></body>
</html>
```

---

## 常见问题

### 事件未显示

1. **检查脚本是否加载**：开发者工具 → 网络 → 查找脚本文件
2. **域名匹配**：`data-domain` 必须与注册的站点域名完全一致
3. **正确的变体**：确认脚本 URL 与功能选择匹配
4. **控制台错误**：检查 CSP、CORS 或网络错误
5. **目标已创建**：自定义事件需先创建匹配的目标

### SPA 导航未追踪

- 默认版本使用 History API；如果 SPA 使用 Hash 路由，请在安装页面勾选 **Hash 路由模式**
- 自定义路由方案可手动调用 `zenstats('pageview')`

### 参与数据缺失

- 参与事件在标签页隐藏或活跃 10 秒后触发
- 少于 2 秒的访问不会产生参与事件

### 调试模式

```javascript
window.__zenstats = true  // 测试时绕过机器人过滤
```

---

## API 参考

### `window.zenstats(eventName, options?)`

| 参数 | 类型 | 说明 |
|------|------|------|
| `eventName` | `string` | 事件名称（"pageview"、"engagement" 或自定义） |
| `options` | `object` | 可选配置 |

**选项：**

| 选项 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `props` | `object` | `{}` | 附加到事件的自定义属性 |
| `callback` | `function` | — | 请求完成后调用 |
| `interactive` | `boolean` | `true` | `false` 表示不影响跳出率 |
| `u` / `url` | `string` | — | 手动 URL 覆盖（需启用 **手动 Pageview URL** 功能） |
| `meta` | `any` | — | 任意元数据（JSON 序列化） |

**回调响应：**

```javascript
{ status: 200, ignored: false }
// status: HTTP 状态码（网络失败时为 0）
// ignored: 事件被过滤时为 true（机器人、排除规则、localStorage 标志）
```

### 事件负载（发往 `/api/event`）

```javascript
{
  n: "pageview",              // 事件名称
  v: "1",                     // 脚本版本
  u: "https://example.com/",  // 完整 URL
  d: "example.com",           // 域名
  r: "https://google.com/",   // 来源（直接访问为 null）
  p: { key: "value" },        // 自定义属性
  m: "{\"key\":\"value\"}",   // 元数据（JSON 字符串，可选）
  e: 15000,                   // 停留时间（毫秒，仅 engagement）
  sd: 85,                     // 滚动深度（百分比，仅 engagement）
  i: false,                   // 交互标志
  h: 1                        // Hash 路由标记（启用 Hash 路由功能时）
}
```

### 内置事件名称

| 事件 | 来源 |
|------|------|
| `pageview` | 页面加载/导航时自动发送 |
| `engagement` | 自动发送（滚动深度 + 活跃时间） |
| `Outbound Link: Click` | 站外链接功能 |
| `File Download` | 文件下载功能 |
| `Form: Submission` | 表单提交自动追踪 |
| `batch` | 多个排队事件合并发送 |

