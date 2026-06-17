# ZenStats Tracking Script

The ZenStats tracking script is a lightweight (~3KB) JavaScript snippet that automatically tracks pageviews, engagement time, scroll depth, and custom events. It works with any website framework including React, Vue, Angular, and plain HTML.

## Table of Contents

- [Installation](#installation)
- [Automatic Tracking](#automatic-tracking)
- [Custom Events](#custom-events)
- [Event Properties](#event-properties)
- [Goals & Conversions](#goals--conversions)
- [Funnels](#funnels)
- [Funnels](#funnels)
- [Privacy & Data](#privacy--data)
- [Integration Guides](#integration-guides)
- [Troubleshooting](#troubleshooting)
- [API Reference](#api-reference)

---

## Installation

### Basic Setup

Add the tracking script to the `<head>` section of your HTML page:

```html
<script defer data-domain="yourdomain.com" src="https://your-zenstats-domain.com/js/script.js"></script>
```

Replace `yourdomain.com` with the domain you added to your ZenStats account, and `your-zenstats-domain.com` with the domain where your ZenStats instance is hosted.

### Script Attributes

| Attribute | Required | Description |
|-----------|----------|-------------|
| `data-domain` | Yes | The domain name you registered in ZenStats |
| `data-api` | No | Custom API endpoint URL (defaults to `/api/event`) |
| `data-outbound-links` | No | Set to `true` to automatically track outbound link clicks |
| `data-file-downloads` | No | Set to `true` to automatically track file downloads (pdf, zip, etc.) |
| `data-file-types` | No | Comma-separated file extensions for download tracking (default: `pdf,xlsx,docx,txt,rtf,csv,...`) |
| `event-*` | No | Custom properties to attach to all events (see [Custom Properties](#custom-properties)) |

### Example with All Attributes

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

### Proxy Setup (Recommended)

To bypass adblockers and improve data accuracy, we recommend serving the tracking script as a first-party connection from your domain. Configure your web server to proxy requests from `/js/script.js` to your ZenStats instance.

**Nginx example:**

```nginx
location /js/script.js {
    proxy_pass https://your-zenstats-domain.com/js/script.js;
    proxy_set_header Host your-zenstats-domain.com;
}
```

**Caddy example:**

```
example.com {
    handle /js/script.js {
        reverse_proxy your-zenstats-domain.com
    }
}
```

---

## Automatic Tracking

Once the script is installed, ZenStats automatically tracks:

### Pageviews

Every page load and navigation event is tracked automatically. This includes:

- Initial page load
- Browser back/forward navigation
- SPA route changes (React Router, Vue Router, etc.)

### Engagement (Time on Page & Scroll Depth)

ZenStats tracks how long visitors engage with your content:

- **Time on page**: Measures actual active time (excludes tab switches and background tabs)
- **Scroll depth**: Tracks the maximum scroll percentage reached
- **Engagement events**: Sent when scroll depth increases or after 3 seconds of active time

Engagement data helps you understand:
- Which content keeps visitors engaged
- Where visitors lose interest
- How far down the page visitors scroll

### SPA Navigation

The script automatically detects and tracks SPA navigation:

- **History-based routing** (React Router, Vue Router, Angular Router): Detects `pushState` and `popstate` events
- **Hash-based routing**: Detects `hashchange` events

No additional configuration is needed for most SPA frameworks.

---

## Custom Events

### Using CSS Class Names (No-Code Approach)

The easiest way to track custom events is by adding CSS class names to HTML elements.

**Format:** `event-name=EventName`

**Examples:**

```html
<!-- Track button clicks -->
<button class="event-name=Signup+Click">Sign Up</button>

<!-- Track link clicks -->
<a href="/pricing" class="event-name=Pricing+Click">View Pricing</a>

<!-- Track form submissions -->
<form class="event-name=Contact+Form+Submit">
  <input type="email" />
  <button type="submit">Submit</button>
</form>
```

**Additional Properties via Classes:**

Add extra properties to events using additional classes:

```html
<button class="event-name=Purchase event-plan=Pro event-amount=99">
  Buy Pro
</button>
```

This sends an event with name `Purchase` and properties `{ plan: "Pro", amount: "99" }`.

**Notes:**
- Use `+` to represent spaces in event names (e.g., `Button+Click` becomes "Button Click")
- The class can be on the element or any parent element (up to 3 levels)
- For links, the `url` property is automatically set to the link's `href`

### Using JavaScript (Manual Tracking)

For more control, use the `zenstats()` function directly:

```javascript
// Basic custom event
zenstats('Signup')

// Event with properties
zenstats('Purchase', {
  props: {
    plan: 'Business',
    amount: 99
  }
})

// Event with callback
zenstats('Download', {
  callback: function(result) {
    if (result.status === 200) {
      console.log('Event tracked successfully')
    } else {
      console.log('Event failed:', result.error)
    }
  }
})
```

### Non-Interactive Events

By default, custom events affect bounce rate calculations. To exclude an event from bounce rate:

```javascript
zenstats('Scroll Depth', { interactive: false })
```

### Outbound Link Clicks

Enable automatic tracking of outbound link clicks by adding `data-outbound-links="true"` to the script tag:

```html
<script defer data-domain="yourdomain.com" data-outbound-links="true" src="..."></script>
```

When enabled, clicks on links pointing to external domains are tracked as `Outbound Link: Click` events with a `url` property.

### File Downloads

Enable automatic file download tracking by adding `data-file-downloads="true"`:

```html
<script defer data-domain="yourdomain.com" data-file-downloads="true" src="..."></script>
```

This tracks clicks on links ending with common file extensions (pdf, zip, docx, etc.). Customize file types:

```html
<script defer data-domain="yourdomain.com" data-file-downloads="true" data-file-types="pdf,zip,csv" src="..."></script>
```

### Form Submissions

Form submissions are automatically tracked as `Form: Submission` events. If a form has a `event-name=*` class, it will be tracked with that custom name instead.

---

## Event Properties

### Custom Properties

Attach custom properties to events for detailed analytics:

```javascript
zenstats('Purchase', {
  props: {
    product: 'Pro Plan',
    amount: 99,
    currency: 'USD'
  }
})
```

### Script Tag Properties

Add properties to all events using script tag attributes:

```html
<script
  defer
  data-domain="example.com"
  data-author="John"
  data-version="1.0"
  src="https://stats.example.com/js/script.js">
</script>
```

Properties set via script attributes can be overridden per event:

```javascript
// Uses script attribute 'author=John'
zenstats('Page Load')

// Overrides to 'author=Jane'
zenstats('Page Load', { props: { author: 'Jane' } })
```

---

## Goals & Conversions

After sending events from your site, create goals in ZenStats to track conversions.

### Setting Up Goals

1. Go to **Site Settings** → **Conversions** → **Goals**
2. Click **Add Goal**
3. Choose goal type:
   - **Custom Event**: Track custom events (e.g., "Signup", "Purchase")
   - **Page View**: Track visits to specific pages (e.g., "/thank-you")
4. Enter a display name
5. Click **Create Goal**

### Pageview Goals

Track visits to specific pages:

```html
<!-- Goal: Visit /thank-you -->
<!-- No code needed - just create a pageview goal in settings -->
```

### Custom Event Goals

Track custom actions:

```javascript
// 1. Send the event from your site
zenstats('Signup')

// 2. Create a goal with name "Signup" in Site Settings → Goals
```

---

## Funnels

Funnels let you track conversion rates through multi-step sequences.

### Creating Funnels

1. Go to **Site Settings** → **Conversions** → **Funnels**
2. Click **Add Funnel**
3. Enter a name (e.g., "Signup Flow")
4. Add 2-8 steps by selecting goals
5. Click **Create Funnel**

### Example Funnel

**Signup Flow:**
1. Visit /pricing (Page View goal)
2. Click Signup (Custom Event goal)
3. Complete Registration (Custom Event goal)

### Analyzing Funnel Results

1. Go to **Funnel Analysis** (link in header)
2. Select a funnel from the dropdown
3. Choose a time period
4. View results:
   - **Total Visitors**: Number who entered the funnel
   - **Step-by-step**: Visitors and drop-off at each step
   - **Conversion Rate**: Percentage who completed all steps

---

## Privacy & Data

### Bot Filtering

ZenStats automatically filters out:
- Headless browsers (Puppeteer, Playwright, etc.)
- Web crawlers and bots
- Automated testing frameworks (Cypress, Selenium)

### Ignore Tracking (Development)

To ignore your own visits during development:

```javascript
// In your browser console or development code
localStorage.setItem('zenstats_ignore', 'true')
```

To re-enable tracking:

```javascript
localStorage.removeItem('zenstats_ignore')
```

### Data Collection

ZenStats collects:
- **Page URL**: Full page URL
- **Referrer**: Where visitors came from
- **Browser**: Browser type and version
- **OS**: Operating system
- **Screen Size**: Device type (Desktop, Mobile, Tablet)
- **Location**: Country, city (from IP address)
- **UTM Parameters**: Marketing campaign tracking

ZenStats does **not** collect:
- Personal information
- IP addresses (hashed for identification)
- Cookies (cookieless tracking)

---

## Integration Guides

### React / Next.js

**React Router:**

```jsx
// No special setup needed - automatic SPA tracking
import { BrowserRouter } from 'react-router-dom'

function App() {
  return (
    <BrowserRouter>
      {/* Your routes */}
    </BrowserRouter>
  )
}
```

**Next.js (App Router):**

Add to `app/layout.tsx`:

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

**Vue Router:**

```javascript
// No special setup needed - automatic SPA tracking
import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [...]
})
```

**Nuxt:**

Add to `nuxt.config.ts`:

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

Add the script to your theme's `header.php`:

```php
<head>
  <script defer data-domain="yourdomain.com" src="https://stats.example.com/js/script.js"></script>
</head>
```

Or use a plugin like **Insert Headers and Footers**.

### Plain HTML

```html
<!DOCTYPE html>
<html>
<head>
  <script defer data-domain="yourdomain.com" src="https://stats.example.com/js/script.js"></script>
</head>
<body>
  <!-- Your content -->
</body>
</html>
```

---

## Troubleshooting

### Events Not Showing Up

1. **Check the script is loading**: Open browser DevTools → Network tab. Look for `script.js` request.
2. **Verify domain matches**: The `data-domain` attribute must match exactly what you registered in ZenStats.
3. **Check the console**: Look for errors in the browser console.
4. **Verify goal is created**: Custom events only appear after creating a matching goal in Site Settings → Goals.

### Engagement Data Missing

- Engagement events are sent when the page becomes hidden or after 3 seconds of active time
- If visitors leave quickly (< 3 seconds), no engagement data is sent
- Check that the script is not being blocked by adblockers or CSP policies

### SPA Navigation Not Tracking

- The script automatically tracks history-based and hash-based routing
- If using a custom routing solution, manually trigger pageviews:

```javascript
// After route change
zenstats('pageview')
```

### Debug Mode

Add `__zenstats` to window to see debug logs:

```javascript
window.__zenstats = true
```

---

## API Reference

### window.zenstats(eventName, options?)

Trigger a tracking event.

**Parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `eventName` | `string` | Event name (e.g., "pageview", "engagement", or custom name) |
| `options` | `object` | Optional configuration |

**Options:**

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `props` | `object` | `{}` | Custom properties to attach to the event |
| `callback` | `function` | `undefined` | Called after event is sent |
| `interactive` | `boolean` | `true` | Whether event affects bounce rate |

**Callback Response:**

```javascript
{
  status: 200,      // HTTP status code (0 if failed)
  error: null,       // Error object if request failed
  ignored: false     // true if event was ignored (bot, localStorage flag)
}
```

**Examples:**

```javascript
// Pageview (usually automatic)
zenstats('pageview')

// Custom event
zenstats('Signup')

// Custom event with properties
zenstats('Purchase', {
  props: {
    plan: 'Business',
    amount: 99
  }
})

// Non-interactive event
zenstats('Scroll Depth', { interactive: false })

// Event with callback
zenstats('Download', {
  callback: function(result) {
    console.log('Status:', result.status)
  }
})
```

### Event Payload Structure

```javascript
{
  n: 'event_name',           // Event name
  v: '1',                    // Script version
  u: 'https://...',          // Page URL
  d: 'yourdomain.com',       // Domain
  r: 'https://...',          // Referrer
  p: { key: 'value' },       // Custom properties
  m: '{"key":"value"}',      // Meta (JSON string)
  i: true,                   // Interactive flag
  e: 15000,                  // Engagement time (ms)
  sd: 85                     // Scroll depth (%)
}
```

### Event Types

| Type | Description | When Sent |
|------|-------------|-----------|
| `pageview` | Page view event | On page load and navigation |
| `engagement` | Engagement metrics | On tab hide or after 3s active time |
| `batch` | Batched events | When multiple events are queued |
| `*` (any) | Custom event | Manual trigger via `zenstats()` |

---

## Best Practices

1. **Use descriptive event names**: "Signup" instead of "click1"
2. **Keep properties consistent**: Use the same property names across events
3. **Create goals first**: Goals must exist before conversions appear in dashboard
4. **Test with console**: Check browser console for errors
5. **Use proxy setup**: Serve script from your domain to avoid adblockers
6. **Respect privacy**: Only track what you need for analytics
