(function(){
  'use strict';

  var location = window.location
  var document = window.document
  var scriptEl = document.currentScript;

  var endpoint = scriptEl.getAttribute('data-api') || defaultEndpoint(scriptEl)
  var dataDomain = scriptEl.getAttribute('data-domain')

  function defaultEndpoint(el) {
    return new URL(el.src).origin + '/api/event'
  }

  function onIgnoredEvent(eventName, reason, options) {
    if (reason) console.warn('Ignoring Event: ' + reason);
    options && options.callback && options.callback({ status: 0, ignored: true })

    if (eventName === 'pageview') {
      currentEngagementIgnored = true
    }
  }

  var currentEngagementIgnored
  var currentEngagementURL = location.href
  var currentEngagementProps = {}
  var currentEngagementMaxScrollDepth = -1

  // Multiple pageviews might be sent by the same script when the page
  // uses client-side routing (e.g. hash or history-based). This flag
  // prevents registering multiple listeners in those cases.
  var listeningOnEngagement = false

  // Timestamp indicating when this particular page last became visible.
  // Reset during pageviews, set to null when page is closed.
  var runningEngagementStart = null

  // When page is hidden, this 'engaged' time is saved to this variable
  var currentEngagementTime = 0

  // Event batching configuration
  var BATCH_INTERVAL = 2000  // Flush every 2 seconds
  var MAX_BATCH_SIZE = 10    // Max events per batch
  var eventQueue = []
  var batchTimer = null

  function getDocumentHeight() {
    var body = document.body || {}
    var el = document.documentElement || {}
    return Math.max(
      body.scrollHeight || 0,
      body.offsetHeight || 0,
      body.clientHeight || 0,
      el.scrollHeight || 0,
      el.offsetHeight || 0,
      el.clientHeight || 0
    )
  }

  function getCurrentScrollDepthPx() {
    var body = document.body || {}
    var el = document.documentElement || {}
    var viewportHeight = window.innerHeight || el.clientHeight || 0
    var scrollTop = window.scrollY || el.scrollTop || body.scrollTop || 0

    return currentDocumentHeight <= viewportHeight ? currentDocumentHeight : scrollTop + viewportHeight
  }

  function getEngagementTime() {
    if (runningEngagementStart) {
      return currentEngagementTime + (Date.now() - runningEngagementStart)
    } else {
      return currentEngagementTime
    }
  }

  var currentDocumentHeight = getDocumentHeight()
  var maxScrollDepthPx = getCurrentScrollDepthPx()

  // Scroll handler with requestAnimationFrame throttle
  var scrollTicking = false
  function onScroll() {
    if (!scrollTicking) {
      window.requestAnimationFrame(function() {
        currentDocumentHeight = getDocumentHeight()
        var currentScrollDepthPx = getCurrentScrollDepthPx()

        if (currentScrollDepthPx > maxScrollDepthPx) {
          maxScrollDepthPx = currentScrollDepthPx
        }
        scrollTicking = false
      })
      scrollTicking = true
    }
  }

  window.addEventListener('load', function () {
    currentDocumentHeight = getDocumentHeight()

    // Update the document height again after every 200ms during the
    // next 3 seconds. This makes sure dynamically loaded content is
    // also accounted for.
    var count = 0
    var interval = setInterval(function () {
      currentDocumentHeight = getDocumentHeight()
      if (++count === 15) {clearInterval(interval)}
    }, 200)

  })

  document.addEventListener('scroll', onScroll)

  function triggerEngagement() {
    var engagementTime = getEngagementTime()

    /*
    We send engagements if there's new relevant engagement information to share:
    - If the user has scrolled more than the previously sent max scroll depth.
    - If the user has been engaged for more than 3 seconds since the last engagement event.

    The first engagement event is always sent due to containing at least the initial scroll depth.

    Also, we don't send engagements if the current pageview is ignored (onIgnoredEvent)
    */
    if (!currentEngagementIgnored && (currentEngagementMaxScrollDepth < maxScrollDepthPx || engagementTime >= 3000)) {
      currentEngagementMaxScrollDepth = maxScrollDepthPx

      var payload = {
        n: 'engagement',
        sd: Math.round((maxScrollDepthPx / currentDocumentHeight) * 100),
        d: dataDomain,
        u: currentEngagementURL,
        p: currentEngagementProps,
        e: engagementTime,
        v: '{{TRACKER_SCRIPT_VERSION}}'
      }

      // Reset current engagement time metrics. They will restart upon when page becomes visible or the next SPA pageview
      runningEngagementStart = null
      currentEngagementTime = 0

      {{#if hash}}
      payload.h = 1
      {{/if}}

      sendRequest(endpoint, payload)
    }
  }

  function onVisibilityChange() {
    if (document.visibilityState === 'visible' && document.hasFocus() && runningEngagementStart === null) {
      runningEngagementStart = Date.now()
    } else if (document.visibilityState === 'hidden' || !document.hasFocus()) {
      // Tab went back to background or lost focus. Save the engaged time so far
      currentEngagementTime = getEngagementTime()
      runningEngagementStart = null

      triggerEngagement()
    }
  }

  function registerEngagementListener() {
    if (!listeningOnEngagement) {
      // Only register visibilitychange listener only after initial page load and pageview
      document.addEventListener('visibilitychange', onVisibilityChange)
      window.addEventListener('blur', onVisibilityChange)
      window.addEventListener('focus', onVisibilityChange)
      listeningOnEngagement = true
    }
  }

  function trigger(eventName, options) {
    var isPageview = eventName === 'pageview'

    if (isPageview && listeningOnEngagement) {
      // If we're listening on engagement already, at least one pageview
      // has been sent by the current script (i.e. it's most likely a SPA).
      // Trigger an engagement marking the "exit from the previous page".
      triggerEngagement()
      currentDocumentHeight = getDocumentHeight()
      maxScrollDepthPx = getCurrentScrollDepthPx()
    }

    // if (/^localhost$|^127(\.[0-9]+){0,2}\.[0-9]+$|^\[::1?\]$/.test(location.hostname) || location.protocol === 'file:') {
    //   return onIgnoredEvent(eventName, 'localhost', options)
    // }
    if ((window._phantom || window.__nightmare || window.navigator.webdriver || window.Cypress) && !window.__zenstats) {
      return onIgnoredEvent(eventName, null, options)
    }
    try {
      if (window.localStorage.zenstats_ignore === 'true') {
        return onIgnoredEvent(eventName, 'localStorage flag', options)
      }
    } catch (e) {

    }

    var payload = {}
    payload.n = eventName
    payload.v = '{{TRACKER_SCRIPT_VERSION}}'

    payload.u = location.href
    payload.d = dataDomain
    payload.r = document.referrer || null
    if (options && options.meta) {
      payload.m = JSON.stringify(options.meta)
    }
    if (options && options.props) {
      payload.p = options.props
    }
    if (options && options.interactive === false) {
      payload.i = false
    }


    var propAttributes = scriptEl.getAttributeNames().filter(function (name) {
      return name.substring(0, 6) === 'event-'
    })

    var props = payload.p || {}

    propAttributes.forEach(function(attribute) {
      var propKey = attribute.replace('event-', '')
      var propValue = scriptEl.getAttribute(attribute)
      props[propKey] = props[propKey] || propValue
    })

    payload.p = props
    {{#if hash}}
    payload.h = 1
    {{/if}}

    if (isPageview) {
      currentEngagementIgnored = false
      currentEngagementURL = payload.u
      currentEngagementProps = payload.p
      currentEngagementMaxScrollDepth = -1
      currentEngagementTime = 0
      runningEngagementStart = Date.now()
      registerEngagementListener()
    }

    addToQueue(payload, options)
  }

  // Event batching: queue events and flush periodically
  function addToQueue(payload, options) {
    eventQueue.push({ payload: payload, options: options })

    if (eventQueue.length >= MAX_BATCH_SIZE) {
      flushQueue()
    } else if (!batchTimer) {
      batchTimer = setTimeout(flushQueue, BATCH_INTERVAL)
    }
  }

  function flushQueue() {
    if (batchTimer) {
      clearTimeout(batchTimer)
      batchTimer = null
    }

    if (eventQueue.length === 0) return

    var eventsToSend = eventQueue.splice(0, MAX_BATCH_SIZE)
    var callbacks = []

    // Collect all callbacks
    for (var i = 0; i < eventsToSend.length; i++) {
      if (eventsToSend[i].options && eventsToSend[i].options.callback) {
        callbacks.push(eventsToSend[i].options.callback)
      }
    }

    // Send batch if multiple events, otherwise send single
    if (eventsToSend.length === 1) {
      sendRequest(endpoint, eventsToSend[0].payload, eventsToSend[0].options)
    } else {
      var batchPayload = {
        n: 'batch',
        e: eventsToSend.map(function(item) { return item.payload }),
        v: '{{TRACKER_SCRIPT_VERSION}}'
      }
      sendRequest(endpoint, batchPayload, callbacks.length > 0 ? { callback: function(result) {
        for (var j = 0; j < callbacks.length; j++) {
          callbacks[j](result)
        }
      }} : null)
    }

    // If there are more events in queue, schedule another flush
    if (eventQueue.length > 0) {
      batchTimer = setTimeout(flushQueue, BATCH_INTERVAL)
    }
  }

  // Flush remaining events on page unload
  window.addEventListener('beforeunload', function() {
    flushQueue()
  })

  function sendRequest(endpoint, payload, options) {
    if (window.fetch) {
      fetch(endpoint, {
        method: 'POST',
        headers: {
          'Content-Type': 'text/plain'
        },
        keepalive: true,
        body: JSON.stringify(payload)
      }).then(function(response) {
        options && options.callback && options.callback({ status: response.status })
      }).catch(function(error) {
        options && options.callback && options.callback({ status: 0, error: error })
      })
    } else if (window.XMLHttpRequest) {
      // Fallback to XMLHttpRequest for older browsers
      try {
        var xhr = new XMLHttpRequest()
        xhr.open('POST', endpoint, true)
        xhr.setRequestHeader('Content-Type', 'text/plain')
        xhr.onreadystatechange = function() {
          if (xhr.readyState === 4) {
            options && options.callback && options.callback({ status: xhr.status })
          }
        }
        xhr.onerror = function() {
          options && options.callback && options.callback({ status: 0, error: 'Network error' })
        }
        xhr.send(JSON.stringify(payload))
      } catch (e) {
        options && options.callback && options.callback({ status: 0, error: e })
      }
    } else {
      // Last resort: image pixel (limited payload, no response)
      options && options.callback && options.callback({ status: 0, error: 'No transport available' })
    }
  }

  var queue = (window.zenstats && window.zenstats.q) || []
  window.zenstats = trigger
  for (var i = 0; i < queue.length; i++) {
    trigger.apply(this, queue[i])
  }

  var lastPage;

  function page(isSPANavigation) {
    {{#unless hash}}
    if (isSPANavigation && lastPage === location.pathname) return;
    {{/unless}}

    lastPage = location.pathname
    trigger('pageview')
  }

  var onSPANavigation = function() {page(true)}

  {{#if hash}}
  window.addEventListener('hashchange', onSPANavigation)
  {{else}}
  var his = window.history
  if (his.pushState) {
    var originalPushState = his['pushState']
    his.pushState = function() {
      originalPushState.apply(this, arguments)
      onSPANavigation();
    }
    window.addEventListener('popstate', onSPANavigation)
  }
  {{/if}}

  function handleVisibilityChange() {
    if (!lastPage && document.visibilityState === 'visible') {
      page()
    }
  }

  if (document.visibilityState === 'hidden' || document.visibilityState === 'prerender') {
    document.addEventListener('visibilitychange', handleVisibilityChange);
  } else {
    page()
  }

  window.addEventListener('pageshow', function(event) {
    if (event.persisted) {
      // Page was restored from bfcache - trigger a pageview
      page();
    }
  })
})();
