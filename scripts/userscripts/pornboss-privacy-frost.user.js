// ==UserScript==
// @name         PornBoss Privacy Frost
// @namespace    https://github.com/pornboss
// @version      0.1.0
// @description  Adds a frosted privacy blur to PornBoss thumbnails, covers, and screenshots while leaving small icons untouched.
// @author       PornBoss
// @match        http://localhost/*
// @match        http://127.0.0.1/*
// @match        http://0.0.0.0/*
// @include      /^https?:\/\/(localhost|127\.0\.0\.1|0\.0\.0\.0)(:\d+)?\/.*$/
// @run-at       document-idle
// @grant        GM_getValue
// @grant        GM_setValue
// @grant        GM_registerMenuCommand
// ==/UserScript==

(function () {
  'use strict'

  const CONFIG = {
    blurPx: 16,
    brightness: 0.74,
    saturate: 0.82,
    scale: 1.06,
    minRenderedWidth: 72,
    minRenderedHeight: 72,
    minRenderedArea: 9000,
    rescanDelayMs: 120,
    storageKey: 'pornboss-privacy-frost-enabled',
  }

  const CONTENT_IMAGE_URLS = [
    /\/videos\/\d+\/thumbnail(?:[?#].*)?$/i,
    /\/videos\/\d+\/screenshots\/[^/?#]+(?:[?#].*)?$/i,
    /\/jav\/[^/?#]+\/cover(?:[?#].*)?$/i,
  ]

  const ICON_URLS = [
    /\/ico\//i,
    /favicon/i,
    /(?:^|[/._-])icon(?:[/.?#_-]|$)/i,
    /\.ico(?:[?#].*)?$/i,
    /\.svg(?:[?#].*)?$/i,
  ]

  const CLASS_NAME = 'pb-privacy-frosted'
  const BACKGROUND_CLASS_NAME = 'pb-privacy-frosted-bg'
  const DISABLED_CLASS_NAME = 'pb-privacy-frost-disabled'

  let enabled = readEnabled()
  let scanTimer = 0

  injectStyle()
  setEnabled(enabled)
  scan()
  observe()
  registerControls()

  function readEnabled() {
    try {
      if (typeof GM_getValue === 'function') {
        return GM_getValue(CONFIG.storageKey, true)
      }
    } catch (_) {
      // Fall back to localStorage below.
    }

    return localStorage.getItem(CONFIG.storageKey) !== 'false'
  }

  function writeEnabled(value) {
    try {
      if (typeof GM_setValue === 'function') {
        GM_setValue(CONFIG.storageKey, value)
        return
      }
    } catch (_) {
      // Fall back to localStorage below.
    }

    localStorage.setItem(CONFIG.storageKey, String(value))
  }

  function injectStyle() {
    if (document.getElementById('pb-privacy-frost-style')) return

    const style = document.createElement('style')
    style.id = 'pb-privacy-frost-style'
    style.textContent = `
      :root {
        --pb-privacy-frost-blur: ${CONFIG.blurPx}px;
        --pb-privacy-frost-scale: ${CONFIG.scale};
        --pb-privacy-frost-brightness: ${CONFIG.brightness};
        --pb-privacy-frost-saturate: ${CONFIG.saturate};
      }

      .${CLASS_NAME} {
        filter:
          blur(var(--pb-privacy-frost-blur))
          saturate(var(--pb-privacy-frost-saturate))
          brightness(var(--pb-privacy-frost-brightness)) !important;
        transform: scale(var(--pb-privacy-frost-scale));
        transform-origin: center center;
        transition: filter 120ms ease, transform 120ms ease;
      }

      .${BACKGROUND_CLASS_NAME} {
        filter:
          blur(var(--pb-privacy-frost-blur))
          saturate(var(--pb-privacy-frost-saturate))
          brightness(var(--pb-privacy-frost-brightness)) !important;
        transform: scale(var(--pb-privacy-frost-scale));
        transform-origin: center center;
        transition: filter 120ms ease, transform 120ms ease;
      }

      .${DISABLED_CLASS_NAME} .${CLASS_NAME},
      .${DISABLED_CLASS_NAME} .${BACKGROUND_CLASS_NAME} {
        filter: none !important;
        transform: none !important;
      }
    `
    document.head.appendChild(style)
  }

  function setEnabled(nextEnabled) {
    enabled = Boolean(nextEnabled)
    document.documentElement.classList.toggle(DISABLED_CLASS_NAME, !enabled)
    writeEnabled(enabled)
  }

  function registerControls() {
    if (typeof GM_registerMenuCommand === 'function') {
      GM_registerMenuCommand('PornBoss 隐私毛玻璃：开启/关闭', () => {
        setEnabled(!enabled)
        scan()
      })
      GM_registerMenuCommand('PornBoss 隐私毛玻璃：重新扫描', () => scan())
    }

    window.addEventListener('keydown', (event) => {
      if (event.altKey && event.shiftKey && event.code === 'KeyB') {
        setEnabled(!enabled)
        scan()
      }
    })
  }

  function observe() {
    const observer = new MutationObserver((mutations) => {
      for (const mutation of mutations) {
        if (mutation.type === 'childList' && mutation.addedNodes.length > 0) {
          scheduleScan()
          return
        }
        if (mutation.type === 'attributes') {
          inspectElement(mutation.target)
        }
      }
    })

    observer.observe(document.documentElement, {
      childList: true,
      subtree: true,
      attributes: true,
      attributeFilter: ['src', 'srcset', 'style', 'class'],
    })
  }

  function scheduleScan() {
    window.clearTimeout(scanTimer)
    scanTimer = window.setTimeout(scan, CONFIG.rescanDelayMs)
  }

  function scan(root = document) {
    const scope = root.nodeType === Node.ELEMENT_NODE ? root : document

    if (scope instanceof HTMLImageElement || scope instanceof HTMLElement) {
      inspectElement(scope)
    }

    scope.querySelectorAll('img, [style*="background"], [role="img"]').forEach(inspectElement)
  }

  function inspectElement(element) {
    if (!(element instanceof HTMLElement)) return

    if (element instanceof HTMLImageElement) {
      updateImageElement(element)
      return
    }

    updateBackgroundElement(element)
  }

  function updateImageElement(image) {
    const shouldProtect = shouldProtectImage(image)
    image.classList.toggle(CLASS_NAME, shouldProtect)

    if (!image.complete && !image.dataset.pbPrivacyLoadBound) {
      image.dataset.pbPrivacyLoadBound = '1'
      image.addEventListener('load', () => updateImageElement(image), { once: true })
    }
  }

  function updateBackgroundElement(element) {
    const shouldProtect = shouldProtectBackground(element)
    element.classList.toggle(BACKGROUND_CLASS_NAME, shouldProtect)
  }

  function shouldProtectImage(image) {
    const url = normalizeURL(image.currentSrc || image.src || image.getAttribute('src') || '')
    if (!url || isExplicitIconURL(url) || isIgnoredElement(image)) return false

    const rect = image.getBoundingClientRect()
    const naturalWidth = image.naturalWidth || 0
    const naturalHeight = image.naturalHeight || 0

    if (isSmallRenderedBox(rect) && !isContentImageURL(url)) return false
    if (naturalWidth > 0 && naturalHeight > 0 && isSmallNaturalImage(naturalWidth, naturalHeight)) return false

    return isContentImageURL(url) || isLargeRenderedBox(rect)
  }

  function shouldProtectBackground(element) {
    if (isIgnoredElement(element)) return false

    const backgroundImage = window.getComputedStyle(element).backgroundImage || ''
    const url = extractBackgroundURL(backgroundImage)
    if (!url || isExplicitIconURL(url)) return false

    const rect = element.getBoundingClientRect()
    if (isSmallRenderedBox(rect) && !isContentImageURL(url)) return false

    const hasMeaningfulChildren = Array.from(element.children).some((child) => {
      if (!(child instanceof HTMLElement)) return false
      const childRect = child.getBoundingClientRect()
      return childRect.width > 8 && childRect.height > 8
    })

    return !hasMeaningfulChildren && (isContentImageURL(url) || isLargeRenderedBox(rect))
  }

  function isIgnoredElement(element) {
    if (element.closest('[data-pb-privacy-ignore]')) return true
    if (element.closest('button, [role="button"], .MuiIconButton-root')) return true
    if (element.classList.contains('MuiSvgIcon-root')) return true
    return false
  }

  function isContentImageURL(url) {
    return CONTENT_IMAGE_URLS.some((pattern) => pattern.test(url))
  }

  function isExplicitIconURL(url) {
    return ICON_URLS.some((pattern) => pattern.test(url))
  }

  function isSmallRenderedBox(rect) {
    return rect.width < CONFIG.minRenderedWidth || rect.height < CONFIG.minRenderedHeight
  }

  function isLargeRenderedBox(rect) {
    return (
      rect.width >= CONFIG.minRenderedWidth &&
      rect.height >= CONFIG.minRenderedHeight &&
      rect.width * rect.height >= CONFIG.minRenderedArea
    )
  }

  function isSmallNaturalImage(width, height) {
    return width <= 64 && height <= 64
  }

  function extractBackgroundURL(backgroundImage) {
    const match = backgroundImage.match(/url\((["']?)(.*?)\1\)/i)
    return match ? normalizeURL(match[2]) : ''
  }

  function normalizeURL(url) {
    return String(url).trim().replace(/^["']|["']$/g, '')
  }
})()
