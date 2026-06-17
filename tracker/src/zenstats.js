(function(){
  'use strict';

  var location = window.location
  var document = window.document
  var scriptEl = document.currentScript || {};

  var endpoint = scriptEl.getAttribute && scriptEl.getAttribute('data-api') || defaultEndpoint()
  var dataDomain = scriptEl.getAttribute && scriptEl.getAttribute('data-domain')

  function defaultEndpoint() {
    var src = scriptEl.src || ''
    return new URL(src).origin + '/api/event'
  }

  function onIgnoredEvent(eventName, reason, options) {
    if (reason) console.warn('Ignoring Event: ' + reason);
    options && options.callback && options.callback({ status: 0, ignored: true })
    if (eventName === 'pageview') currentEngagementIgnored = true
  }

  var currentEngagementIgnored
  var currentEngagementURL = location.href
  var currentEngagementProps = {}
  var currentEngagementMaxScrollDepth = -1
  var listeningOnEngagement = false
  var runningEngagementStart = null
  var currentEngagementTime = 0

  var BATCH_INTERVAL = 2000
  var MAX_BATCH_SIZE = 10
  var eventQueue = []
  var batchTimer = null

  function getDocumentHeight() {
    var body = document.body || {}
    var el = document.documentElement || {}
    return Math.max(
      body.scrollHeight || 0, body.offsetHeight || 0, body.clientHeight || 0,
      el.scrollHeight || 0, el.offsetHeight || 0, el.clientHeight || 0
    )
  }

  function getCurrentScrollDepthPx() {
    var body = document.body || {}
    var el = document.documentElement || {}
    var vh = window.innerHeight || el.clientHeight || 0
    var st = window.scrollY || el.scrollTop || body.scrollTop || 0
    return currentDocumentHeight <= vh ? currentDocumentHeight : st + vh
  }

  function getEngagementTime() {
    return runningEngagementStart ? currentEngagementTime + (Date.now() - runningEngagementStart) : currentEngagementTime
  }

  var currentDocumentHeight = getDocumentHeight()
  var maxScrollDepthPx = getCurrentScrollDepthPx()
  var scrollTicking = false

  function onScroll() {
    if (!scrollTicking) {
      window.requestAnimationFrame(function() {
        currentDocumentHeight = getDocumentHeight()
        var d = getCurrentScrollDepthPx()
        if (d > maxScrollDepthPx) maxScrollDepthPx = d
        scrollTicking = false
      })
      scrollTicking = true
    }
  }

  window.addEventListener('load', function () {
    currentDocumentHeight = getDocumentHeight()
    var c = 0
    var iv = setInterval(function () {
      currentDocumentHeight = getDocumentHeight()
      if (++c === 15) clearInterval(iv)
    }, 200)
  })
  document.addEventListener('scroll', onScroll)

  function triggerEngagement() {
    var et = getEngagementTime()
    if (!currentEngagementIgnored && (currentEngagementMaxScrollDepth < maxScrollDepthPx || et > 2000)) {
      currentEngagementMaxScrollDepth = maxScrollDepthPx
      sendRequest(endpoint, {
        n: 'engagement', sd: Math.round((maxScrollDepthPx / currentDocumentHeight) * 100),
        d: dataDomain, u: currentEngagementURL, p: currentEngagementProps, e: et,
        v: '{{TRACKER_SCRIPT_VERSION}}'
      })
      runningEngagementStart = null
      currentEngagementTime = 0
    }
  }

  function onVisibilityChange() {
    if (document.visibilityState === 'visible' && document.hasFocus() && runningEngagementStart === null) {
      runningEngagementStart = Date.now()
    } else if (document.visibilityState === 'hidden' || !document.hasFocus()) {
      currentEngagementTime = getEngagementTime()
      runningEngagementStart = null
      triggerEngagement()
    }
  }

  var engagementTimer = null
  function startEngagementTimer() {
    if (engagementTimer) clearInterval(engagementTimer)
    engagementTimer = setInterval(function() {
      if (!currentEngagementIgnored && runningEngagementStart !== null) triggerEngagement()
    }, 10000)
  }

  function registerEngagementListener() {
    if (!listeningOnEngagement) {
      document.addEventListener('visibilitychange', onVisibilityChange)
      window.addEventListener('blur', onVisibilityChange)
      window.addEventListener('focus', onVisibilityChange)
      startEngagementTimer()
      listeningOnEngagement = true
    }
  }

  {{#if (any COMPILE_TAGGED_EVENTS COMPILE_OUTBOUND_LINKS COMPILE_FILE_DOWNLOADS)}}
  var PARENTS_TO_SEARCH_LIMIT = 3
  var MIDDLE_MOUSE_BUTTON = 1
  {{/if}}

  {{#if COMPILE_OUTBOUND_LINKS}}
  function isOutboundLink(link) {
    return link && typeof link.href === 'string' && link.host && link.host !== location.host && !link.href.match(/^(javascript:|mailto:|tel:)/i)
  }
  {{/if}}

  {{#if COMPILE_FILE_DOWNLOADS}}
  var DEFAULT_FILE_TYPES = 'pdf,xlsx,docx,txt,rtf,csv,exe,key,pps,ppt,pptx,7z,pkg,rar,gz,zip,avi,mov,mp4,mpeg,wmv,midi,mp3,wav,wma,dmg'
  var fileTypes = (scriptEl.getAttribute && scriptEl.getAttribute('data-file-types') || DEFAULT_FILE_TYPES).split(',')

  function isDownloadToTrack(url) {
    if (!url) return false
    var ext = url.split('.').pop()
    for (var i = 0; i < fileTypes.length; i++) {
      if (fileTypes[i] === ext) return true
    }
    return false
  }
  {{/if}}

  {{#if (any COMPILE_TAGGED_EVENTS COMPILE_OUTBOUND_LINKS COMPILE_FILE_DOWNLOADS)}}
  function isLink(el) {
    return el && el.tagName && el.tagName.toLowerCase() === 'a'
  }

  function getLinkEl(link) {
    while (link && (typeof link.tagName === 'undefined' || !isLink(link) || !link.href) && link !== document.body)
      link = link.parentNode
    return link
  }
  {{/if}}

  {{#if COMPILE_TAGGED_EVENTS}}
  function isTagged(el) {
    var cl = el && el.classList
    if (!cl) return false
    for (var i = 0; i < cl.length; i++) {
      if (cl.item(i).match(/event-name(=|--)(.+)/)) return true
    }
    return false
  }


  function getTaggedEventAttributes(htmlEl) {
    var te = isTagged(htmlEl) ? htmlEl : htmlEl && htmlEl.parentNode
    var attrs = { name: null, props: {} }
    var cl = te && te.classList
    if (!cl) return attrs
    for (var i = 0; i < cl.length; i++) {
      var m = cl.item(i).match(/event-(.+)(=|--)(.+)/)
      if (m) {
        var k = m[1], v = m[3].replace(/\+/g, ' ')
        if (k.toLowerCase() === 'name') attrs.name = v
        else attrs.props[k] = v
      }
    }
    return attrs
  }

  function isForm(el) {
    return el && el.tagName && el.tagName.toLowerCase() === 'form'
  }
  {{/if}}

  {{#if (any COMPILE_OUTBOUND_LINKS COMPILE_FILE_DOWNLOADS COMPILE_TAGGED_EVENTS)}}
  function handleLinkClickEvent(event) {
    if (event.type === 'auxclick' && event.button !== MIDDLE_MOUSE_BUTTON) return

    {{#if COMPILE_TAGGED_EVENTS}}
    // Single DOM walk: check tagged elements + links in one pass
    var clicked = event.target
    var foundLink
    for (var i = 0; i <= PARENTS_TO_SEARCH_LIMIT; i++) {
      if (!clicked) break
      if (isForm(clicked)) return
      if (isLink(clicked)) foundLink = clicked
      if (isTagged(clicked)) {
        var ea = getTaggedEventAttributes(clicked)
        if (foundLink) ea.props.url = foundLink.href
        if (ea.name) return trigger(ea.name, { props: ea.props })
        return  // Tagged element → suppress outbound / download
      }
      clicked = clicked.parentNode
    }
    {{else}}
    var foundLink = getLinkEl(event.target)
    {{/if}}

    {{#if (any COMPILE_OUTBOUND_LINKS COMPILE_FILE_DOWNLOADS)}}
    var hrefWO = foundLink && typeof foundLink.href === 'string' && foundLink.href.split('?')[0]
    {{/if}}
    {{#if COMPILE_OUTBOUND_LINKS}}
    if (foundLink && isOutboundLink(foundLink)) return trigger('Outbound Link: Click', { props: { url: foundLink.href } })
    {{/if}}
    {{#if COMPILE_FILE_DOWNLOADS}}
    if (hrefWO && isDownloadToTrack(hrefWO)) return trigger('File Download', { props: { url: hrefWO } })
    {{/if}}
  }

  function handleFormSubmitEvent(event) {
    {{#if COMPILE_TAGGED_EVENTS}}
    var ea = getTaggedEventAttributes(event.target)
    if (ea.name) return trigger(ea.name, { props: ea.props })
    {{/if}}
    trigger('Form: Submission')
  }
  {{/if}}

  function trigger(eventName, options) {
    var isPageview = eventName === 'pageview'

    if (isPageview && listeningOnEngagement) {
      triggerEngagement()
      currentDocumentHeight = getDocumentHeight()
      maxScrollDepthPx = getCurrentScrollDepthPx()
    }

    if ((window._phantom || window.__nightmare || window.navigator.webdriver || window.Cypress) && !window.__zenstats) {
      return onIgnoredEvent(eventName, null, options)
    }
    try {
      if (window.localStorage.zenstats_ignore === 'true') return onIgnoredEvent(eventName, 'localStorage flag', options)
    } catch (e) {}

    {{#if COMPILE_EXCLUSIONS}}
    var exclusionRegexCache = {}
    function pathMatches(wildcardPath) {
      var actualPath = location.pathname
      {{#if COMPILE_HASH}}
      actualPath += location.hash
      {{/if}}
      var regex = exclusionRegexCache[wildcardPath]
      if (!regex) {
        regex = exclusionRegexCache[wildcardPath] = new RegExp(
          '^' +
            wildcardPath
              .trim()
              .replace(/\*\*/g, '.*')
              .replace(/([^\.])\*/g, '$1[^\\s/]*') +
            '\/?$'
        )
      }
      return actualPath.match(regex)
    }

    if (isPageview) {
      var dataIncludeAttr = scriptEl.getAttribute && scriptEl.getAttribute('data-include')
      var dataExcludeAttr = scriptEl.getAttribute && scriptEl.getAttribute('data-exclude')

      var isIncluded =
        !dataIncludeAttr ||
        (dataIncludeAttr && dataIncludeAttr.split(',').some(pathMatches))
      var isExcluded =
        dataExcludeAttr && dataExcludeAttr.split(',').some(pathMatches)

      if (!isIncluded || isExcluded)
        return onIgnoredEvent(eventName, 'exclusion rule', options)
    }
    {{/if}}

    {{#if COMPILE_MANUAL}}
    var customURL = options && (options.u || options.url)
    {{/if}}
    var payload = {
      n: eventName, v: '{{TRACKER_SCRIPT_VERSION}}',
      u: {{#if COMPILE_MANUAL}}(customURL || location.href){{else}}location.href{{/if}}, d: dataDomain, r: document.referrer || null
    }
    if (options && options.meta) payload.m = JSON.stringify(options.meta)
    if (options && options.props) payload.p = options.props
    if (options && options.interactive === false) payload.i = false

    {{#if COMPILE_PAGEVIEW_PROPS}}
    var attrs = scriptEl.getAttributeNames && scriptEl.getAttributeNames().filter(function(n) { return n.substring(0,6) === 'event-' }) || []
    var props = payload.p || {}
    for (var i = 0; i < attrs.length; i++) {
      var k = attrs[i].replace('event-', ''), v = scriptEl.getAttribute(attrs[i])
      props[k] = props[k] || v
    }
    payload.p = props
    {{/if}}

    if (isPageview) {
      currentEngagementIgnored = false
      currentEngagementURL = payload.u
      currentEngagementProps = payload.p
      currentEngagementMaxScrollDepth = -1
      currentEngagementTime = 0
      if (document.visibilityState === 'visible') runningEngagementStart = Date.now()
      else runningEngagementStart = null
      registerEngagementListener()
      startEngagementTimer()
    }
    addToQueue(payload, options)
  }

  {{#if (any COMPILE_TAGGED_EVENTS COMPILE_OUTBOUND_LINKS COMPILE_FILE_DOWNLOADS)}}
  document.addEventListener('click', handleLinkClickEvent)
  document.addEventListener('auxclick', handleLinkClickEvent)
  document.addEventListener('submit', handleFormSubmitEvent)
  {{/if}}

  function addToQueue(payload, options) {
    eventQueue.push({ payload: payload, options: options })
    if (eventQueue.length >= MAX_BATCH_SIZE) flushQueue()
    else if (!batchTimer) batchTimer = setTimeout(flushQueue, BATCH_INTERVAL)
  }

  function flushQueue() {
    if (batchTimer) { clearTimeout(batchTimer); batchTimer = null }
    if (eventQueue.length === 0) return
    var items = eventQueue.splice(0, MAX_BATCH_SIZE)
    var cbs = []
    for (var i = 0; i < items.length; i++) {
      if (items[i].options && items[i].options.callback) cbs.push(items[i].options.callback)
    }
    if (items.length === 1) {
      sendRequest(endpoint, items[0].payload, items[0].options)
    } else {
      sendRequest(endpoint, {
        n: 'batch', e: items.map(function(x) { return x.payload }), v: '{{TRACKER_SCRIPT_VERSION}}'
      }, cbs.length > 0 ? { callback: function(r) { for (var j = 0; j < cbs.length; j++) cbs[j](r) } } : null)
    }
    if (eventQueue.length > 0) batchTimer = setTimeout(flushQueue, BATCH_INTERVAL)
  }

  window.addEventListener('beforeunload', function() {
    triggerEngagement()
    if (eventQueue.length > 0) {
      // Use sendBeacon for reliable delivery during page unload
      if (navigator.sendBeacon) {
        var items = eventQueue.splice(0, eventQueue.length)
        if (items.length === 1) {
          navigator.sendBeacon(endpoint, JSON.stringify(items[0].payload))
        } else {
          navigator.sendBeacon(endpoint, JSON.stringify({
            n: 'batch', e: items.map(function(x) { return x.payload }), v: '{{TRACKER_SCRIPT_VERSION}}'
          }))
        }
      } else {
        flushQueue()
      }
    }
  })

  function sendRequest(endpoint, payload, options) {
    // Use sendBeacon when no callback is expected (fire-and-forget)
    if (navigator.sendBeacon && !(options && options.callback)) {
      navigator.sendBeacon(endpoint, JSON.stringify(payload))
      return
    }
    if (window.fetch) {
      fetch(endpoint, {
        method: 'POST', headers: { 'Content-Type': 'text/plain' },
        keepalive: true, body: JSON.stringify(payload)
      }).then(function(r) { options && options.callback && options.callback({ status: r.status }) })
        .catch(function(e) { options && options.callback && options.callback({ status: 0, error: e }) })
    } else if (window.XMLHttpRequest) {
      try {
        var xhr = new XMLHttpRequest()
        xhr.open('POST', endpoint, true)
        xhr.setRequestHeader('Content-Type', 'text/plain')
        xhr.onreadystatechange = function() {
          if (xhr.readyState === 4) options && options.callback && options.callback({ status: xhr.status })
        }
        xhr.onerror = function() { options && options.callback && options.callback({ status: 0, error: 'Network error' }) }
        xhr.send(JSON.stringify(payload))
      } catch(e) { options && options.callback && options.callback({ status: 0, error: e }) }
    } else {
      options && options.callback && options.callback({ status: 0, error: 'No transport available' })
    }
  }

  // Initialize
  var queue = (window.zenstats && window.zenstats.q) || []
  window.zenstats = trigger
  for (var i = 0; i < queue.length; i++) trigger.apply(this, queue[i])

  // SPA pageview tracking
  var lastPage;
  function page(isSPANav) {
    {{#unless COMPILE_HASH}}
    if (isSPANav && lastPage === location.pathname) return;
    {{/unless}}
    lastPage = location.pathname
    trigger('pageview')
  }
  var onSPANav = function() { page(true) }

  {{#if COMPILE_HASH}}
  window.addEventListener('hashchange', onSPANav)
  {{else}}
  var his = window.history
  if (his.pushState) {
    var orig = his.pushState
    his.pushState = function() { orig.apply(this, arguments); onSPANav() }
    window.addEventListener('popstate', onSPANav)
  }
  {{/if}}

  function visChange() {
    if (!lastPage && document.visibilityState === 'visible') page()
  }
  if (document.visibilityState === 'hidden' || document.visibilityState === 'prerender') {
    document.addEventListener('visibilitychange', visChange)
  } else {
    page()
  }
  window.addEventListener('pageshow', function(e) { if (e.persisted) page() })
})();
