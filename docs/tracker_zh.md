# ZenStats 追踪脚本

ZenStats 追踪脚本是一个轻量级（约 3KB）的 JavaScript 代码片段，可自动追踪页面浏览、用户参与时间、滚动深度和自定义事件。它适用于任何网站框架，包括 React、Vue、Angular 和纯 HTML。

## 目录

- [安装](#安装)
- [自动追踪](#自动追踪)
- [自定义事件](#自定义事件)
- [事件属性](#事件属性)
- [目标与转化](#目标与转化)
- [漏斗分析](#漏斗分析)
- [隐私与数据](#隐私与数据)
- [集成指南](#集成指南)
- [常见问题](#常见问题)
- [API 参考](#api-参考)

---

## 安装

### 基本设置

在 HTML 页面的 `<head>` 部分添加追踪脚本：

```html
<script defer data-domain="yourdomain.com" src="https://your-zenstats-domain.com/js/script.js"></script>
```

将 `yourdomain.com` 替换为您在 ZenStats 账户中添加的域名，将 `your-zenstats-domain.com` 替换为您 ZenStats 实例的域名。

### 脚本属性

| 属性 | 必填 | 说明 |
|------|------|------|
| `data-domain` | 是 | 您在 ZenStats 中注册的域名 |
| `data-api` | 否 | 自定义 API 端点 URL（默认为 `/api/event`） |
| `event-*` | 否 | 附加到所有事件的自定义属性（参见[自定义属性](#自定义属性)） |

### 包含所有属性的示例

```html
<script
  defer
  data-domain="example.com"
  data-api="https://stats.example.com/api/event"
  data-author="John"
  data-version="1.0"
  src="https://stats.example.com/js/script.js">
</script>
```

### 代理设置（推荐）

为了绕过广告拦截器并提高数据准确性，建议从您的域名提供追踪脚本作为第一方连接。配置您的 Web 服务器将 `/js/script.js` 的请求代理到您的 ZenStats 实例。

**Nginx 示例：**

```nginx
location /js/script.js {
    proxy_pass https://your-zenstats-domain.com/js/script.js;
    proxy_set_header Host your-zenstats-domain.com;
}
```

**Caddy 示例：**

```
example.com {
    handle /js/script.js {
        reverse_proxy your-zenstats-domain.com
    }
}
```

---

## 自动追踪

安装脚本后，ZenStats 会自动追踪：

### 页面浏览

每次页面加载和导航事件都会被自动追踪，包括：

- 初始页面加载
- 浏览器前进/后退导航
- SPA 路由变化（React Router、Vue Router 等）

### 用户参与（页面停留时间与滚动深度）

ZenStats 追踪访客与内容的互动情况：

- **页面停留时间**：测量实际活跃时间（排除标签页切换和后台标签页）
- **滚动深度**：追踪达到的最大滚动百分比
- **参与事件**：在滚动深度增加或 3 秒活跃时间后发送

参与数据帮助您了解：

- 哪些内容能留住访客
- 访客在哪里失去兴趣
- 访客滚动到页面的什么位置

### SPA 导航

脚本会自动检测并追踪 SPA 导航：

- **基于 History 的路由**（React Router、Vue Router、Angular Router）：检测 `pushState` 和 `popstate` 事件
- **基于 Hash 的路由**：检测 `hashchange` 事件

大多数 SPA 框架无需额外配置。

---

## 自定义事件

### 使用 CSS 类名（无代码方式）

追踪自定义事件最简单的方法是为 HTML 元素添加 CSS 类名。

**格式：** `event-name=事件名称`

**示例：**

```html
<!-- 追踪按钮点击 -->
<button class="event-name=Signup+Click">注册</button>

<!-- 追踪链接点击 -->
<a href="/pricing" class="event-name=Pricing+Click">查看定价</a>

<!-- 追踪表单提交 -->
<form class="event-name=Contact+Form+Submit">
  <input type="email" />
  <button type="submit">提交</button>
</form>
```

**说明：**

- 使用 `+` 表示事件名称中的空格（例如，`Button+Click` 变为 "Button Click"）
- 如果您的 CMS 将 `=` 替换为 `-`，请使用 `event-name--事件名称`
- 类名可以放在元素或任何父元素上

### 使用 JavaScript（手动追踪）

需要更多控制时，直接使用 `zenstats()` 函数：

```javascript
// 基本自定义事件
zenstats('Signup')

// 带属性的事件
zenstats('Purchase', {
  props: {
    plan: 'Business',
    amount: 99
  }
})

// 带回调的事件
zenstats('Download', {
  callback: function(result) {
    if (result.status === 200) {
      console.log('事件追踪成功')
    } else {
      console.log('事件失败:', result.error)
    }
  }
})
```

### 非交互事件

默认情况下，自定义事件会影响跳出率计算。要从跳出率中排除某事件：

```javascript
zenstats('Scroll Depth', { interactive: false })
```

---

## 事件属性

### 自定义属性

为事件附加自定义属性以进行详细分析：

```javascript
zenstats('Purchase', {
  props: {
    product: 'Pro Plan',
    amount: 99,
    currency: 'USD'
  }
})
```

### 脚本标签属性

使用脚本标签属性为所有事件添加属性：

```html
<script
  defer
  data-domain="example.com"
  data-author="John"
  data-version="1.0"
  src="https://stats.example.com/js/script.js">
</script>
```

通过脚本属性设置的属性可以在每个事件中被覆盖：

```javascript
// 使用脚本属性 'author=John'
zenstats('Page Load')

// 覆盖为 'author=Jane'
zenstats('Page Load', { props: { author: 'Jane' } })
```

---

## 目标与转化

从网站发送事件后，需要在 ZenStats 中创建目标以追踪转化。

### 设置目标

1. 转到 **站点设置** → **转化** → **目标**
2. 点击 **添加目标**
3. 选择目标类型：
   - **自定义事件**：追踪自定义事件（如 "Signup"、"Purchase"）
   - **页面浏览**：追踪特定页面的访问（如 "/thank-you"）
4. 输入显示名称
5. 点击 **创建目标**

### 页面浏览目标

追踪特定页面的访问：

```html
<!-- 目标：访问 /thank-you -->
<!-- 无需代码 - 只需在设置中创建页面浏览目标 -->
```

### 自定义事件目标

追踪自定义操作：

```javascript
// 1. 从网站发送事件
zenstats('Signup')

// 2. 在站点设置 → 目标中创建名为 "Signup" 的目标
```

---

## 漏斗分析

漏斗帮助您追踪多步骤序列的转化率。

### 创建漏斗

1. 转到 **站点设置** → **转化** → **漏斗**
2. 点击 **添加漏斗**
3. 输入名称（如 "注册流程"）
4. 通过选择目标添加 2-8 个步骤
5. 点击 **创建漏斗**

### 漏斗示例

**注册流程：**
1. 访问 /pricing（页面浏览目标）
2. 点击注册（自定义事件目标）
3. 完成注册（自定义事件目标）

### 分析漏斗结果

1. 转到 **漏斗分析**（页面顶部链接）
2. 从下拉菜单中选择漏斗
3. 选择时间段
4. 查看结果：
   - **总访客数**：进入漏斗的人数
   - **逐步数据**：每一步的访客数和流失率
   - **转化率**：完成所有步骤的百分比

---

## 隐私与数据

### 机器人过滤

ZenStats 自动过滤以下内容：

- 无头浏览器（Puppeteer、Playwright 等）
- 网络爬虫和机器人
- 自动化测试框架（Cypress、Selenium）

### 忽略追踪（开发环境）

在开发期间忽略您自己的访问：

```javascript
// 在浏览器控制台或开发代码中
localStorage.setItem('zenstats_ignore', 'true')
```

要重新启用追踪：

```javascript
localStorage.removeItem('zenstats_ignore')
```

### 数据收集

ZenStats 收集：

- **页面 URL**：完整页面 URL
- **来源**：访客来源
- **浏览器**：浏览器类型和版本
- **操作系统**：操作系统信息
- **屏幕尺寸**：设备屏幕分辨率
- **位置**：国家、城市（基于 IP 地址）
- **UTM 参数**：营销活动追踪

ZenStats **不** 收集：

- 个人信息
- IP 地址（哈希处理用于识别）
- Cookie（无 Cookie 追踪）

---

## 集成指南

### React / Next.js

**React Router：**

```jsx
// 无需特殊设置 - 自动 SPA 追踪
import { BrowserRouter } from 'react-router-dom'

function App() {
  return (
    <BrowserRouter>
      {/* 您的路由 */}
    </BrowserRouter>
  )
}
```

**Next.js（App Router）：**

在 `app/layout.tsx` 中添加：

```tsx
import Script from 'next/script'

export default function RootLayout({ children }) {
  return (
    <html>
      <head>
        <Script
          defer
          data-domain="yourdomain.com"
          src="https://stats.example.com/js/script.js"
        />
      </head>
      <body>{children}</body>
    </html>
  )
}
```

### Vue / Nuxt

**Vue Router：**

```javascript
// 无需特殊设置 - 自动 SPA 追踪
import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [...]
})
```

**Nuxt：**

在 `nuxt.config.ts` 中添加：

```javascript
export default defineNuxtConfig({
  app: {
    head: {
      script: [
        {
          defer: true,
          'data-domain': 'yourdomain.com',
          src: 'https://stats.example.com/js/script.js'
        }
      ]
    }
  }
})
```

### WordPress

在主题的 `header.php` 中添加脚本：

```php
<head>
  <script defer data-domain="yourdomain.com" src="https://stats.example.com/js/script.js"></script>
</head>
```

或使用 **Insert Headers and Footers** 等插件。

### 纯 HTML

```html
<!DOCTYPE html>
<html>
<head>
  <script defer data-domain="yourdomain.com" src="https://stats.example.com/js/script.js"></script>
</head>
<body>
  <!-- 您的内容 -->
</body>
</html>
```

---

## 常见问题

### 事件未显示

1. **检查脚本是否加载**：打开浏览器开发者工具 → 网络选项卡。查找 `script.js` 请求。
2. **验证域名匹配**：`data-domain` 属性必须与您在 ZenStats 中注册的域名完全一致。
3. **检查控制台**：查看浏览器控制台中的错误。
4. **验证目标已创建**：自定义事件只有在站点设置 → 目标中创建匹配目标后才会显示。

### 参与数据缺失

- 参与事件在页面变为隐藏或 3 秒活跃时间后发送
- 如果访客快速离开（< 3 秒），则不会发送参与数据
- 检查脚本是否被广告拦截器或 CSP 策略阻止

### SPA 导航未追踪

- 脚本自动追踪基于 History 和 Hash 的路由
- 如果使用自定义路由解决方案，手动触发页面浏览：

```javascript
// 路由变化后
zenstats('pageview')
```

### 调试模式

在 window 上添加 `__zenstats` 以查看调试日志：

```javascript
window.__zenstats = true
```

---

## API 参考

### window.zenstats(eventName, options?)

触发追踪事件。

**参数：**

| 参数 | 类型 | 说明 |
|------|------|------|
| `eventName` | `string` | 事件名称（如 "pageview"、"engagement" 或自定义名称） |
| `options` | `object` | 可选配置 |

**选项：**

| 选项 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `props` | `object` | `{}` | 附加到事件的自定义属性 |
| `callback` | `function` | `undefined` | 事件发送后调用 |
| `interactive` | `boolean` | `true` | 事件是否影响跳出率 |

**回调响应：**

```javascript
{
  status: 200,      // HTTP 状态码（失败时为 0）
  error: null,       // 请求失败时的错误对象
  ignored: false     // 事件被忽略时为 true（机器人、localStorage 标志）
}
```

**示例：**

```javascript
// 页面浏览（通常自动触发）
zenstats('pageview')

// 自定义事件
zenstats('Signup')

// 带属性的自定义事件
zenstats('Purchase', {
  props: {
    plan: 'Business',
    amount: 99
  }
})

// 非交互事件
zenstats('Scroll Depth', { interactive: false })

// 带回调的事件
zenstats('Download', {
  callback: function(result) {
    console.log('状态:', result.status)
  }
})
```

### 事件负载结构

```javascript
{
  n: 'event_name',           // 事件名称
  v: '1',                    // 脚本版本
  u: 'https://...',          // 页面 URL
  d: 'yourdomain.com',       // 域名
  r: 'https://...',          // 来源
  p: { key: 'value' },       // 自定义属性
  m: '{"key":"value"}',      // 元数据（JSON 字符串）
  i: true,                   // 交互标志
  e: 15000,                  // 参与时间（毫秒）
  sd: 85                     // 滚动深度（%）
}
```

### 事件类型

| 类型 | 说明 | 发送时机 |
|------|------|----------|
| `pageview` | 页面浏览事件 | 页面加载和导航时 |
| `engagement` | 参与指标 | 标签页隐藏或 3 秒活跃时间后 |
| `batch` | 批量事件 | 多个事件排队时 |
| `*`（任意） | 自定义事件 | 通过 `zenstats()` 手动触发 |

---

## 最佳实践

1. **使用描述性事件名称**："Signup" 而不是 "click1"
2. **保持属性一致性**：在不同事件中使用相同的属性名称
3. **先创建目标**：目标必须存在，转化才会显示在仪表板中
4. **使用控制台测试**：检查浏览器控制台中的错误
5. **使用代理设置**：从您的域提供脚本以避免广告拦截器
6. **尊重隐私**：只追踪分析所需的内容
